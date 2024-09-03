package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"
)

var (
	ErrBranchExists                 = errors.New("branch already exists")
	ErrProjectPathDoesNotExist      = errors.New("project path does not exist")
	ErrProjectLanguageNotRecognized = errors.New("project language not recognized")
	ErrUnsupportedRemoteURL         = errors.New("unsupported remote URL")
)

type RepoContext struct {
	globalConfig    *GlobalConfig
	projectConfig   *ProjectConfig
	globalGitConfig *config.Config
	repo            *git.Repository
	worktree        *git.Worktree
	head            *plumbing.Reference
}

// detectProjectLanguage detects the language of a project by looking at the files in the project
func detectProjectLanguage(globalConfig *GlobalConfig, cwd string) (string, error) {
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

// detectBySpecialPatterns checks the project type using special file patterns
func detectBySpecialPatterns(globalConfig *GlobalConfig, absPath string) string {
	for language, config := range globalConfig.LanguagesConfig {
		for _, pattern := range config.SpecialPatterns {
			matches, _ := filepath.Glob(filepath.Join(absPath, pattern))
			if len(matches) > 0 {
				log.Infof("Project language detected as %s via file pattern '%s'", language, pattern)
				return language
			}
		}
	}
	return ""
}

// detectByExtensions checks the project type using file extensions
func detectByExtensions(globalConfig *GlobalConfig, absPath string) (string, error) {
	var detected string
	err := filepath.Walk(absPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || detected != "" {
			return nil
		}
		for language, config := range globalConfig.LanguagesConfig {
			if hasMatchingExtension(info.Name(), config.Extensions) {
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

// hasMatchingExtension checks if the file has one of the specified extensions
func hasMatchingExtension(filename string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(filename, "."+ext) {
			return true
		}
	}
	return false
}

// getGlobalGitConfig gets a Git option from local and global Git config
func getOptionFromConfig(cfg, globalCfg *config.Config, section string, option string) string {
	opt := cfg.Raw.Section(section).Option(option)
	if opt == "" {
		opt = globalCfg.Raw.Section(section).Option(option)
	}
	return opt
}

// cloneRepo clones a remote repository into a temporary directory
func cloneRepo(ctx *RepoContext) (string, error) {
	// create a temporary directory
	tmpDir, err := os.MkdirTemp("", "autobump-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// setup the clone options
	log.Infof("Cloning %s into %s", ctx.projectConfig.Path, tmpDir)
	cloneOptions := &git.CloneOptions{
		URL:   ctx.projectConfig.Path,
		Depth: 1,
	}

	service := getServiceTypeByURL(ctx.projectConfig.Path)

	// get authentication methods
	var authMethods []transport.AuthMethod
	authMethods, err = getAuthMethods(
		service,
		ctx.globalGitConfig.Raw.Section("user").Option("name"),
		ctx.globalConfig,
		ctx.projectConfig,
	)
	if err != nil {
		return "", err
	}

	// try each authentication method
	clonedSuccessfully := false
	for _, auth := range authMethods {
		cloneOptions.Auth = auth
		ctx.repo, err = git.PlainClone(tmpDir, false, cloneOptions)

		// if action finished successfully, return
		if err == nil {
			log.Infof("Successfully cloned %s", ctx.projectConfig.Path)
			ctx.projectConfig.Path = tmpDir
			clonedSuccessfully = true
			break
		}
	}

	// if all authentication methods failed, return the last error
	if !clonedSuccessfully {
		return "", fmt.Errorf("failed to clone %s: %w", ctx.projectConfig.Path, err)
	}

	return tmpDir, nil
}

func createPullRequest(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
	repo *git.Repository,
	branchName string,
	serviceType ServiceType,
) error {
	var err error
	switch serviceType { //nolint:exhaustive // unsupported service types are handled by the default case
	case GITLAB:
		err = createGitLabMergeRequest(
			globalConfig,
			projectConfig,
			repo,
			branchName,
			projectConfig.NewVersion,
		)
		if err != nil {
			return err
		}
	case AZUREDEVOPS:
		err = createAzureDevOpsPullRequest(
			globalConfig,
			projectConfig,
			repo,
			branchName,
			projectConfig.NewVersion,
		)
		if err != nil {
			return err
		}
	default:
		log.Warnf("Service type '%v' not supported yet...", serviceType)
	}

	return nil
}

func cloneRepoIfNeeded(ctx *RepoContext) (string, error) {
	if strings.HasPrefix(ctx.projectConfig.Path, "https://") || strings.HasPrefix(ctx.projectConfig.Path, "git@") {
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
	lines, err := readLines(changelogPath)
	if err != nil {
		return false, err
	}

	bumpEmpty, err := isChangelogUnreleasedEmpty(lines)
	if err != nil {
		return false, err
	}
	if bumpEmpty {
		log.Infof("Bump is empty, skipping project %s", ctx.projectConfig.Name)
		return false, nil
	}
	return true, nil
}

func ensureProjectLanguage(ctx *RepoContext) error {
	if ctx.projectConfig.Language == "" {
		projectLanguage, err := detectProjectLanguage(ctx.globalConfig, ctx.projectConfig.Path)
		if err != nil {
			return err
		}
		ctx.projectConfig.Language = projectLanguage
	}
	return nil
}

func setupRepo(ctx *RepoContext) error {
	if ctx.repo != nil {
		var err error
		ctx.repo, err = openRepo(ctx.projectConfig.Path)
		if err != nil {
			return err
		}
	}

	worktree, err := ctx.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	ctx.worktree = worktree

	head, err := ctx.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get repo HEAD: %w", err)
	}
	ctx.head = head

	return nil
}

func createBumpBranch(ctx *RepoContext, changelogPath string) (string, error) {
	nextVersion, err := getNextVersion(changelogPath)
	if err != nil {
		return "", err
	}

	branchName := "chore/bump-" + nextVersion.String()

	branchExists, err := checkBranchExists(ctx.repo, branchName)
	if err != nil {
		return "", err
	}
	if branchExists {
		return "", fmt.Errorf("%w: %s", ErrBranchExists, branchName)
	}

	err = createAndSwitchBranch(ctx.repo, ctx.worktree, branchName, ctx.head.Hash())
	if err != nil {
		return "", err
	}

	return branchName, nil
}

func updateChangelogAndVersionFiles(ctx *RepoContext, changelogPath string) error {
	log.Info("Updating CHANGELOG.md file")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		log.Errorf("No version found in CHANGELOG.md for project at %s\n", ctx.projectConfig.Path)
		return err
	}

	ctx.projectConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", ctx.projectConfig.NewVersion)
	err = updateVersion(ctx.globalConfig, ctx.projectConfig)
	if err != nil {
		return err
	}

	return addFilesToWorktree(ctx, changelogPath)
}

func addFilesToWorktree(ctx *RepoContext, changelogPath string) error {
	versionFiles, err := getVersionFiles(ctx.globalConfig, ctx.projectConfig)
	if err != nil {
		return err
	}

	projectPath := ctx.projectConfig.Path

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
		_, err = ctx.worktree.Add(versionFileRelativePath)
		if err != nil {
			return fmt.Errorf("failed to add version file: %w", err)
		}
	}

	changelogRelativePath, err := filepath.Rel(projectPath, changelogPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path for changelog file: %w", err)
	}
	_, err = ctx.worktree.Add(changelogRelativePath)
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
	cfg, err := ctx.repo.Config()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to get repo config: %w", err)
	}

	gpgSign := getOptionFromConfig(cfg, ctx.globalGitConfig, "commit", "gpgsign")
	gpgFormat := getOptionFromConfig(cfg, ctx.globalGitConfig, "gpg", "format")

	var signKey *openpgp.Entity
	if gpgSign == "true" && gpgFormat != "ssh" {
		log.Info("Signing commit with GPG key")
		gpgKeyID := getOptionFromConfig(cfg, ctx.globalGitConfig, "user", "signingkey")

		var gpgKeyReader *io.Reader
		gpgKeyReader, err = getGpgKeyReader(gpgKeyID, ctx.globalConfig.GpgKeyPath)
		if err != nil {
			return plumbing.Hash{}, err
		}

		signKey, err = getGpgKey(*gpgKeyReader)
		if err != nil {
			return plumbing.Hash{}, err
		}
	}

	commitMessage := "chore(bump): bumped version to " + ctx.projectConfig.NewVersion
	return commitChanges(
		ctx.worktree,
		commitMessage,
		signKey,
		ctx.globalGitConfig.Raw.Section("user").Option("name"),
		ctx.globalGitConfig.Raw.Section("user").Option("email"),
	)
}

func pushChanges(ctx *RepoContext, branchName string) error {
	refSpec := config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)

	remoteCfg, err := ctx.repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote origin: %w", err)
	}

	remoteURL := remoteCfg.Config().URLs[0]
	if strings.HasPrefix(remoteURL, "git@") {
		return pushChangesSSH(ctx.repo, refSpec)
	} else if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		var cfg *config.Config
		cfg, err = ctx.repo.Config()
		if err != nil {
			return fmt.Errorf("failed to get repo config: %w", err)
		}
		return pushChangesHTTPS(ctx.repo, cfg, refSpec, ctx.globalConfig, ctx.projectConfig)
	}

	// If none of the conditions match, return an error
	return fmt.Errorf("%w: %s", ErrUnsupportedRemoteURL, remoteURL)
}

func createAndCheckoutPullRequest(ctx *RepoContext, branchName string) error {
	serviceType, err := getRemoteServiceType(ctx.repo)
	if err != nil {
		return err
	}

	err = createPullRequest(ctx.globalConfig, ctx.projectConfig, ctx.repo, branchName, serviceType)
	if err != nil {
		return err
	}

	return checkoutToMainBranch(ctx)
}

func checkoutToMainBranch(ctx *RepoContext) error {
	err := checkoutBranch(ctx.worktree, "main")
	if err != nil {
		return checkoutBranch(ctx.worktree, "master")
	}
	return nil
}

// addCurrentVersion adds the current version to the CHANGELOG file
func addCurrentVersion(ctx *RepoContext, changelogPath string) error {
	lines, err := readLines(changelogPath)
	if err != nil {
		return err
	}

	latestTag, err := getLatestTag(ctx.repo)
	if err != nil {
		return err
	}

	// TODO: we should replace <LINK TO THE PLATFORM TO OPEN THE PULL REQUEST> with the actual link

	// add lines to the end of the file
	lines = append(lines, []string{
		fmt.Sprintf("\n## [%s] - %s\n", latestTag.Tag, latestTag.Date.Format("2006-01-02")),
		"The changes weren't tracked until this version.",
	}...)
	err = writeLines(changelogPath, lines)
	if err != nil {
		return err
	}

	return nil
}

// processRepo:
// - clones the repository if it is a remote repository
// - creates the chore/bump branch
// - updates the CHANGELOG.md file
// - updates the version file
// - commits the changes
// - pushes the branch to the remote repository
// - creates a new merge request on GitLab
func processRepo(globalConfig *GlobalConfig, projectConfig *ProjectConfig) error {
	// Initialize RepoContext
	ctx := &RepoContext{
		globalConfig:  globalConfig,
		projectConfig: projectConfig,
	}

	// Get global Git config
	var err error
	ctx.globalGitConfig, err = getGlobalGitConfig()
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

	projectPath := ctx.projectConfig.Path
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

	// Ensure the project language is detected
	err = ensureProjectLanguage(ctx)
	if err != nil {
		return err
	}

	// Create and switch to bump branch
	branchName, err := createBumpBranch(ctx, changelogPath)
	if err != nil {
		return err
	}

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

	log.Infof("Successfully processed project '%s'", ctx.projectConfig.Name)
	return nil
}

// iterateProjects iterates over the projects and processes them using the processRepo function
func iterateProjects(globalConfig *GlobalConfig) error {
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

		err = processRepo(globalConfig, &project)
		if err != nil {
			log.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}

	return err
}
