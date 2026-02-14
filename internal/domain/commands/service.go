package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitutil "github.com/rios0rios0/autobump/internal/infrastructure/repositories/git"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/python"
	"github.com/rios0rios0/autobump/internal/support"
)

var (
	ErrBranchExists                 = errors.New("branch already exists")
	ErrProjectPathDoesNotExist      = errors.New("project path does not exist")
	ErrProjectLanguageNotRecognized = errors.New("project language not recognized")
	ErrUnsupportedRemoteURL         = errors.New("unsupported remote URL")
	ErrNoVersionFileFound           = errors.New("no version file found")
	ErrLanguageNotFoundInConfig     = errors.New("language not found in config")
)

// RepoContext holds the context for processing a repository.
type RepoContext struct {
	GlobalConfig    *entities.GlobalConfig
	ProjectConfig   *entities.ProjectConfig
	GlobalGitConfig *gitconfig.Config
	Repo            *git.Repository
	Worktree        *git.Worktree
	Head            *plumbing.Reference
}

// DetectProjectLanguage detects the language of a project by looking at the files in the project.
func DetectProjectLanguage(globalConfig *entities.GlobalConfig, cwd string) (string, error) {
	log.Info("Detecting project language")

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check the project type by special files
	if language := detectBySpecialPatterns(globalConfig, absPath); language != "" {
		return language, nil
	}

	// Check the project type by file extensions
	language, err := detectByExtensions(globalConfig, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to walk project directory: %w", err)
	}
	if language != "" {
		return language, nil
	}

	return "", ErrProjectLanguageNotRecognized
}

// detectBySpecialPatterns checks the project type using special file patterns.
func detectBySpecialPatterns(globalConfig *entities.GlobalConfig, absPath string) string {
	for language, cfg := range globalConfig.LanguagesConfig {
		for _, pattern := range cfg.SpecialPatterns {
			matches, _ := filepath.Glob(filepath.Join(absPath, pattern))
			if len(matches) > 0 {
				log.Infof("Project language detected as %s via file pattern '%s'", language, pattern)
				return language
			}
		}
	}
	return ""
}

// detectByExtensions checks the project type using file extensions.
func detectByExtensions(globalConfig *entities.GlobalConfig, absPath string) (string, error) {
	var detected string
	err := filepath.Walk(absPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || detected != "" {
			return nil
		}
		for language, cfg := range globalConfig.LanguagesConfig {
			if HasMatchingExtension(info.Name(), cfg.Extensions) {
				detected = language
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk project directory: %w", err)
	}
	return detected, nil
}

// HasMatchingExtension checks if the file has one of the specified extensions.
func HasMatchingExtension(filename string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(filename, "."+ext) {
			return true
		}
	}
	return false
}

// cloneRepo clones a remote repository into a temporary directory.
func cloneRepo(ctx *RepoContext) (string, error) {
	// create a temporary directory
	tmpDir, err := os.MkdirTemp("", "autobump-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Get the adapter for this URL to handle service-specific logic
	adapter := infraRepos.GetAdapterByURL(ctx.ProjectConfig.Path)
	service := gitutil.GetServiceTypeByURL(ctx.ProjectConfig.Path)

	// Prepare the clone URL (adapters may strip embedded credentials, etc.)
	cloneURL := ctx.ProjectConfig.Path
	if adapter != nil {
		cloneURL = adapter.PrepareCloneURL(ctx.ProjectConfig.Path)
	}

	// setup the clone options
	log.Infof("Cloning %s into %s", cloneURL, tmpDir)
	cloneOptions := &git.CloneOptions{
		URL: cloneURL,
	}

	// get authentication methods
	var authMethods []transport.AuthMethod
	authMethods, err = gitutil.GetAuthMethods(
		service,
		ctx.GlobalGitConfig.Raw.Section("user").Option("name"),
		ctx.GlobalConfig,
		ctx.ProjectConfig,
	)
	if err != nil {
		return "", err
	}

	// try each authentication method
	clonedSuccessfully := false
	for _, auth := range authMethods {
		cloneOptions.Auth = auth
		ctx.Repo, err = git.PlainClone(tmpDir, false, cloneOptions)

		// if action finished successfully, return
		if err == nil {
			log.Infof("Successfully cloned %s", ctx.ProjectConfig.Path)
			ctx.ProjectConfig.Path = tmpDir
			clonedSuccessfully = true
			break
		}
	}

	// if all authentication methods failed, return the last error
	if !clonedSuccessfully {
		return "", fmt.Errorf("failed to clone %s: %w", ctx.ProjectConfig.Path, err)
	}

	return tmpDir, nil
}

func createPullRequest(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	repo *git.Repository,
	branchName string,
	serviceType entities.ServiceType,
) error {
	prProvider := infraRepos.NewPullRequestProvider(serviceType)
	if prProvider == nil {
		log.Warnf("Service type '%v' not supported yet...", serviceType)
		return nil
	}

	return prProvider.CreatePullRequest(
		globalConfig,
		projectConfig,
		repo,
		branchName,
		projectConfig.NewVersion,
	)
}

func cloneRepoIfNeeded(ctx *RepoContext) (string, error) {
	if strings.HasPrefix(ctx.ProjectConfig.Path, "https://") || strings.HasPrefix(ctx.ProjectConfig.Path, "git@") {
		return cloneRepo(ctx)
	}
	return "", nil
}

func setupChangelog(ctx *RepoContext, changelogPath string) error {
	exists, err := createChangelogIfNotExists(changelogPath)
	if err != nil {
		return err
	}
	if !exists {
		err = addCurrentVersion(ctx, changelogPath)
		if err != nil {
			return err
		}
		// TODO: commit and push the newly created file
	}
	return nil
}

func shouldBumpProject(ctx *RepoContext, changelogPath string) (bool, error) {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return false, err
	}

	bumpEmpty, err := entities.IsChangelogUnreleasedEmpty(lines)
	if err != nil {
		return false, err
	}
	if bumpEmpty {
		log.Infof("Bump is empty, skipping project %s", ctx.ProjectConfig.Name)
		return false, nil
	}
	return true, nil
}

func ensureProjectLanguage(ctx *RepoContext) {
	if ctx.ProjectConfig.Language == "" {
		projectLanguage, err := DetectProjectLanguage(ctx.GlobalConfig, ctx.ProjectConfig.Path)
		if err != nil {
			log.Warnf("Could not detect project language: %v, will only update changelog", err)
			ctx.ProjectConfig.Language = ""
			return
		}
		ctx.ProjectConfig.Language = projectLanguage
	}
}

func setupRepo(ctx *RepoContext) error {
	if ctx.Repo == nil {
		var err error
		ctx.Repo, err = gitutil.OpenRepo(ctx.ProjectConfig.Path)
		if err != nil {
			return err
		}
	}

	worktree, err := ctx.Repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	ctx.Worktree = worktree

	head, err := ctx.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get repo HEAD: %w", err)
	}
	ctx.Head = head

	return nil
}

func createBumpBranch(ctx *RepoContext, changelogPath string) (string, entities.BranchStatus, error) {
	nextVersion, err := getNextVersion(changelogPath)
	if err != nil {
		return "", entities.BranchCreated, err
	}

	branchName := "chore/bump-" + nextVersion.String()
	// Store the version for PR creation even if branch exists
	ctx.ProjectConfig.NewVersion = nextVersion.String()

	branchExists, err := gitutil.CheckBranchExists(ctx.Repo, branchName)
	if err != nil {
		return "", entities.BranchCreated, err
	}
	if branchExists {
		log.Warnf("Branch '%s' already exists (local or remote)", branchName)
		return branchName, entities.BranchExistsNoPR, nil // Return branch name for PR check
	}

	err = gitutil.CreateAndSwitchBranch(ctx.Repo, ctx.Worktree, branchName, ctx.Head.Hash())
	if err != nil {
		return "", entities.BranchCreated, err
	}

	return branchName, entities.BranchCreated, nil
}

func updateChangelogAndVersionFiles(ctx *RepoContext, changelogPath string) error {
	log.Info("Updating CHANGELOG.md file")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		log.Errorf("No version found in CHANGELOG.md for project at %s\n", ctx.ProjectConfig.Path)
		return err
	}

	ctx.ProjectConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", ctx.ProjectConfig.NewVersion)
	err = updateVersion(ctx.GlobalConfig, ctx.ProjectConfig)
	if err != nil {
		return err
	}

	return addFilesToWorktree(ctx, changelogPath)
}

func addFilesToWorktree(ctx *RepoContext, changelogPath string) error {
	versionFiles, err := getVersionFiles(ctx.GlobalConfig, ctx.ProjectConfig)
	if err != nil {
		return err
	}

	projectPath := ctx.ProjectConfig.Path

	for _, versionFile := range versionFiles {
		var versionFileRelativePath string
		versionFileRelativePath, err = filepath.Rel(projectPath, versionFile.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for version file: %w", err)
		}

		if _, err = os.Stat(versionFile.Path); os.IsNotExist(err) {
			continue
		}

		log.Infof("Adding version file %s", versionFileRelativePath)
		_, err = ctx.Worktree.Add(versionFileRelativePath)
		if err != nil {
			return fmt.Errorf("failed to add version file: %w", err)
		}
	}

	changelogRelativePath, err := filepath.Rel(projectPath, changelogPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path for changelog file: %w", err)
	}
	_, err = ctx.Worktree.Add(changelogRelativePath)
	if err != nil {
		return fmt.Errorf("failed to add changelog file: %w", err)
	}

	return nil
}

func commitAndPushChanges(ctx *RepoContext, branchName string) error {
	_, err := commitChangesWithGPG(ctx)
	if err != nil {
		return err
	}

	err = pushChanges(ctx, branchName)
	if err != nil {
		if err.Error() == "object not found" {
			log.Error("Got error object not found (remote branch already exists?)")
		}
		return err
	}

	return nil
}

func commitChangesWithGPG(ctx *RepoContext) (plumbing.Hash, error) {
	cfg, err := ctx.Repo.Config()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to get repo config: %w", err)
	}

	gpgSign := gitutil.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "commit", "gpgsign")
	gpgFormat := gitutil.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "gpg", "format")

	var signKey *openpgp.Entity
	if gpgSign == "true" && gpgFormat != "ssh" {
		log.Info("Signing commit with GPG key")
		gpgKeyID := gitutil.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "signingkey")

		var gpgKeyReader io.Reader
		gpgKeyReader, err = support.GetGpgKeyReader(context.Background(), gpgKeyID, ctx.GlobalConfig.GpgKeyPath)
		if err != nil {
			return plumbing.Hash{}, err
		}

		signKey, err = support.GetGpgKey(gpgKeyReader)
		if err != nil {
			return plumbing.Hash{}, err
		}
	}

	commitMessage := "chore(bump): bumped version to " + ctx.ProjectConfig.NewVersion
	return gitutil.CommitChanges(
		ctx.Worktree,
		commitMessage,
		signKey,
		ctx.GlobalGitConfig.Raw.Section("user").Option("name"),
		ctx.GlobalGitConfig.Raw.Section("user").Option("email"),
	)
}

func pushChanges(ctx *RepoContext, branchName string) error {
	refSpec := gitconfig.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)

	remoteCfg, err := ctx.Repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote origin: %w", err)
	}

	remoteURL := remoteCfg.Config().URLs[0]
	if strings.HasPrefix(remoteURL, "git@") {
		return gitutil.PushChangesSSH(ctx.Repo, refSpec)
	} else if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		var cfg *gitconfig.Config
		cfg, err = ctx.Repo.Config()
		if err != nil {
			return fmt.Errorf("failed to get repo config: %w", err)
		}
		return gitutil.PushChangesHTTPS(ctx.Repo, cfg, refSpec, ctx.GlobalConfig, ctx.ProjectConfig)
	}

	// If none of the conditions match, return an error
	return fmt.Errorf("%w: %s", ErrUnsupportedRemoteURL, remoteURL)
}

func createAndCheckoutPullRequest(ctx *RepoContext, branchName string) error {
	serviceType, err := gitutil.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		return err
	}

	err = createPullRequest(ctx.GlobalConfig, ctx.ProjectConfig, ctx.Repo, branchName, serviceType)
	if err != nil {
		return err
	}

	return checkoutToMainBranch(ctx)
}

func checkoutToMainBranch(ctx *RepoContext) error {
	err := gitutil.CheckoutBranch(ctx.Worktree, "main")
	if err != nil {
		return gitutil.CheckoutBranch(ctx.Worktree, "master")
	}
	return nil
}

// addCurrentVersion adds the current version to the CHANGELOG file.
func addCurrentVersion(ctx *RepoContext, changelogPath string) error {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return err
	}

	latestTag, err := gitutil.GetLatestTag(ctx.Repo)
	if err != nil {
		return err
	}

	// TODO: we should replace <LINK TO THE PLATFORM TO OPEN THE PULL REQUEST> with the actual link

	// add lines to the end of the file
	lines = append(lines, []string{
		fmt.Sprintf("\n## [%s] - %s\n", latestTag.Tag, latestTag.Date.Format("2006-01-02")),
		"The changes weren't tracked until this version.",
	}...)
	err = support.WriteLines(changelogPath, lines)
	if err != nil {
		return err
	}

	return nil
}

// ProcessRepo processes a repository:
// - clones the repository if it is a remote repository
// - creates the chore/bump branch
// - updates the CHANGELOG.md file
// - updates the version file
// - commits the changes
// - pushes the branch to the remote repository
// - creates a new merge request on GitLab.
func ProcessRepo(globalConfig *entities.GlobalConfig, projectConfig *entities.ProjectConfig) error {
	// Initialize RepoContext
	ctx := &RepoContext{
		GlobalConfig:  globalConfig,
		ProjectConfig: projectConfig,
	}

	// Get global Git config
	var err error
	ctx.GlobalGitConfig, err = gitutil.GetGlobalGitConfig()
	if err != nil {
		return err
	}

	// Clone repository if needed
	var tmpDir string
	tmpDir, err = cloneRepoIfNeeded(ctx)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	projectPath := ctx.ProjectConfig.Path
	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")

	// Setup repository and worktree
	err = setupRepo(ctx)
	if err != nil {
		return err
	}

	// Set up the changelog
	err = setupChangelog(ctx, changelogPath)
	if err != nil {
		return err
	}

	// Determine if bump is needed
	bumpNeeded, err := shouldBumpProject(ctx, changelogPath)
	if err != nil {
		return err
	}
	if !bumpNeeded {
		return nil
	}

	// Ensure the project language is detected (optional - will only update changelog if unknown)
	ensureProjectLanguage(ctx)

	// Create and switch to bump branch (or check if it already exists)
	branchName, branchStatus, err := createBumpBranch(ctx, changelogPath)
	if err != nil {
		return err
	}

	// Handle branch already exists case
	if branchStatus == entities.BranchExistsNoPR {
		return handleExistingBranchWithoutPR(ctx, branchName)
	}

	// Normal flow: branch was newly created
	// Update changelog and version files
	err = updateChangelogAndVersionFiles(ctx, changelogPath)
	if err != nil {
		return err
	}

	// Commit and push changes
	err = commitAndPushChanges(ctx, branchName)
	if err != nil {
		return err
	}

	// Create and checkout pull request
	err = createAndCheckoutPullRequest(ctx, branchName)
	if err != nil {
		return err
	}

	log.Infof("Successfully processed project '%s'", ctx.ProjectConfig.Name)
	return nil
}

// handleExistingBranchWithoutPR handles the case where a bump branch exists but no PR was found.
func handleExistingBranchWithoutPR(ctx *RepoContext, branchName string) error {
	// Branch exists, check if PR exists
	prExists, prErr := checkPullRequestExists(ctx, branchName)
	if prErr != nil {
		log.Warnf("Failed to check if PR exists: %v, skipping project", prErr)
		return nil
	}
	if prExists {
		log.Infof("Pull request already exists for branch '%s', skipping project", branchName)
		return nil
	}
	// PR doesn't exist, create it
	log.Infof("Branch exists but no PR found, creating pull request for branch '%s'", branchName)
	if err := createAndCheckoutPullRequest(ctx, branchName); err != nil {
		return err
	}
	log.Infof("Successfully created PR for existing branch in project '%s'", ctx.ProjectConfig.Name)
	return nil
}

// checkPullRequestExists checks if a PR exists for the given branch using the appropriate provider.
func checkPullRequestExists(ctx *RepoContext, branchName string) (bool, error) {
	serviceType, err := gitutil.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		return false, err
	}

	prProvider := infraRepos.NewPullRequestProvider(serviceType)
	if prProvider == nil {
		log.Warnf("Service type '%v' not supported for PR check", serviceType)
		return false, nil // Assume no PR exists if provider not supported
	}

	return prProvider.PullRequestExists(ctx.GlobalConfig, ctx.ProjectConfig, ctx.Repo, branchName)
}

// IterateProjects iterates over the projects and processes them using the ProcessRepo function.
func IterateProjects(globalConfig *entities.GlobalConfig) error {
	var err error
	for _, project := range globalConfig.Projects {
		// verify if the project path exists
		if _, err = os.Stat(project.Path); os.IsNotExist(err) {
			// if the project path does not exist, check if it is a remote repository
			if !strings.HasPrefix(project.Path, "https://") &&
				!strings.HasPrefix(project.Path, "git@") {
				// if it is neither a local path nor a remote repository, skip the project
				log.Errorf("Project path does not exist: %s\n", project.Path)
				log.Warn("Skipping project")
				err = ErrProjectPathDoesNotExist
				continue
			}
		}

		err = ProcessRepo(globalConfig, &project)
		if err != nil {
			log.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}

	return err
}

// DiscoverAndProcess discovers repositories from configured providers and processes each one.
func DiscoverAndProcess(
	ctx context.Context,
	globalConfig *entities.GlobalConfig,
	discovererRegistry *infraRepos.DiscovererRegistry,
) error {
	totalRepos := 0
	totalErrors := 0

	for _, provCfg := range globalConfig.Providers {
		discoverer, err := discovererRegistry.Get(provCfg.Type, provCfg.Token)
		if err != nil {
			log.Errorf("Failed to initialize provider %q: %v", provCfg.Type, err)
			totalErrors++
			continue
		}

		log.Infof("Processing provider: %s", discoverer.Name())

		for _, org := range provCfg.Organizations {
			log.Infof("Discovering repositories in %q...", org)

			repos, discoverErr := discoverer.DiscoverRepositories(ctx, org)
			if discoverErr != nil {
				log.Errorf("Failed to discover repos in %q: %v", org, discoverErr)
				totalErrors++
				continue
			}

			log.Infof("Found %d repositories in %q", len(repos), org)

			for _, repo := range repos {
				totalRepos++
				projectConfig := repoToProjectConfig(repo, provCfg)
				if processErr := ProcessRepo(globalConfig, projectConfig); processErr != nil {
					log.Errorf(
						"Error processing %s/%s: %v",
						repo.Organization, repo.Name, processErr,
					)
					totalErrors++
				}
			}
		}
	}

	log.Infof("Discovery complete: %d repos processed, %d errors", totalRepos, totalErrors)
	return nil
}

// repoToProjectConfig converts a discovered Repository into a ProjectConfig
// that can be fed into the existing ProcessRepo pipeline.
func repoToProjectConfig(
	repo entities.Repository,
	provCfg entities.ProviderConfig,
) *entities.ProjectConfig {
	return &entities.ProjectConfig{
		Path:               repo.RemoteURL,
		Name:               repo.Name,
		ProjectAccessToken: provCfg.Token,
	}
}

// ---- Changelog I/O wrappers ----

// updateChangelogFile reads the changelog, processes it, and writes it back.
func updateChangelogFile(changelogPath string) (*semver.Version, error) {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return nil, err
	}

	version, newContent, err := entities.ProcessChangelog(lines)
	if err != nil {
		return nil, err
	}

	err = support.WriteLines(changelogPath, newContent)
	if err != nil {
		return nil, err
	}

	return version, nil
}

// getNextVersion reads the changelog and calculates the next version.
func getNextVersion(changelogPath string) (*semver.Version, error) {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return nil, err
	}

	// Check if this is a new changelog (no version found)
	_, err = entities.FindLatestVersion(lines)
	if errors.Is(err, entities.ErrNoVersionFoundInChangelog) {
		// For new changelogs, return 1.0.0 directly
		version, _ := semver.NewVersion(entities.InitialReleaseVersion)
		return version, nil
	}

	version, _, err := entities.ProcessChangelog(lines)
	if err != nil {
		return nil, err
	}

	return version, nil
}

// createChangelogIfNotExists create an empty CHANGELOG file if it doesn't exist.
func createChangelogIfNotExists(changelogPath string) (bool, error) {
	if _, err := os.Stat(changelogPath); os.IsNotExist(err) {
		log.Warnf("Creating empty CHANGELOG file at '%s'.", changelogPath)
		var fileContent []byte
		fileContent, err = support.DownloadFile(entities.DefaultChangelogURL)
		if err != nil {
			return false, fmt.Errorf("failed to download CHANGELOG template: %w", err)
		}

		err = os.WriteFile(changelogPath, fileContent, 0o644) //nolint:gosec // the CHANGELOG file is not sensitive
		if err != nil {
			return false, fmt.Errorf("failed to create CHANGELOG file: %w", err)
		}

		return false, nil
	}

	return true, nil
}

// ---- Versioning operations ----

// updateVersion updates the version in the version files.
func updateVersion(globalConfig *entities.GlobalConfig, projectConfig *entities.ProjectConfig) error {
	// If language is empty/unknown, skip version file updates
	if projectConfig.Language == "" {
		log.Info("Language is unknown, skipping version file updates (only changelog will be updated)")
		return nil
	}

	versionFiles, err := getVersionFiles(globalConfig, projectConfig)
	if err != nil {
		// If language config not found, just warn and continue with changelog only
		if errors.Is(err, ErrLanguageNotFoundInConfig) {
			log.Warnf("Language '%s' not found in config, skipping version file updates", projectConfig.Language)
			return nil
		}
		return err
	}

	// If no version files configured for this language, just continue
	if len(versionFiles) == 0 {
		log.Warnf(
			"No version files configured for language '%s', only changelog will be updated",
			projectConfig.Language,
		)
		return nil
	}

	oneVersionFileExists := false
	for _, versionFile := range versionFiles {
		// check if the file exists
		var info os.FileInfo
		info, err = os.Stat(versionFile.Path)
		if os.IsNotExist(err) {
			log.Warnf("Version file %s does not exist", versionFile.Path)
			continue
		}
		log.Infof("Updating version file %s", versionFile.Path)

		originalFileMode := info.Mode()
		oneVersionFileExists = true

		var content []byte
		content, err = os.ReadFile(versionFile.Path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", versionFile.Path, err)
		}

		updatedContent := string(content)
		for _, pattern := range versionFile.Patterns {
			re, compileErr := regexp.Compile(pattern)
			if compileErr != nil {
				return fmt.Errorf("invalid regex pattern %q in version file config: %w", pattern, compileErr)
			}
			updatedContent = re.ReplaceAllStringFunc(updatedContent, func(match string) string {
				return re.ReplaceAllString(match, "${1}"+projectConfig.NewVersion+"${2}")
			})
		}

		err = os.WriteFile(versionFile.Path, []byte(updatedContent), originalFileMode)
		if err != nil {
			return fmt.Errorf("failed to write to file %s: %w", versionFile.Path, err)
		}
	}

	// If no version files exist, just warn and continue (don't fail)
	if !oneVersionFileExists {
		log.Warnf("No version files found for language '%s', only changelog will be updated", projectConfig.Language)
	}

	return nil
}

// getVersionFiles returns the files in a project that contains the software's version number.
func getVersionFiles(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) ([]entities.VersionFile, error) {
	// If language is empty/unknown, return empty list
	if projectConfig.Language == "" {
		return []entities.VersionFile{}, nil
	}

	if projectConfig.Name == "" {
		projectConfig.Name = filepath.Base(projectConfig.Path)
	}
	projectName := strings.ReplaceAll(projectConfig.Name, "-", "_")
	var versionFiles []entities.VersionFile

	// try to get the project name from the language interface
	var languageInterface entities.Language
	getLanguageInterface(*projectConfig, &languageInterface)

	if languageInterface != nil {
		languageProjectName, err := languageInterface.GetProjectName()
		if err == nil && languageProjectName != "" {
			log.Infof("Using project name '%s' from language interface", languageProjectName)
			projectName = strings.ReplaceAll(languageProjectName, "-", "_")
		}
	} else {
		log.Infof("Language '%s' does not have a language interface", projectConfig.Language)
	}

	languageConfig, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLanguageNotFoundInConfig, projectConfig.Language)
	}

	for _, versionFile := range languageConfig.VersionFiles {
		matches, err := filepath.Glob(
			filepath.Join(
				projectConfig.Path,
				strings.ReplaceAll(versionFile.Path, "{project_name}", projectName),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get version files: %w", err)
		}
		for _, match := range matches {
			versionFiles = append(
				versionFiles, entities.VersionFile{
					Path:     match,
					Patterns: versionFile.Patterns,
				},
			)
		}
	}
	return versionFiles, nil
}

// getLanguageInterface returns the appropriate Language interface for the project.
func getLanguageInterface(projectConfig entities.ProjectConfig, languageInterface *entities.Language) {
	if projectConfig.Language == "python" {
		*languageInterface = &python.Python{ProjectConfig: projectConfig}
	}
}
