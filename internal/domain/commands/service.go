package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	logger "github.com/sirupsen/logrus"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/python"
	"github.com/rios0rios0/autobump/internal/support"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
	gitHelpers "github.com/rios0rios0/gitforge/pkg/git/infrastructure/helpers"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
	signingInfra "github.com/rios0rios0/gitforge/pkg/signing/infrastructure"
	langEntities "github.com/rios0rios0/langforge/pkg/domain/entities"
	langRegistry "github.com/rios0rios0/langforge/pkg/infrastructure/registry"
)

var (
	ErrBranchExists                 = errors.New("branch already exists")
	ErrProjectPathDoesNotExist      = errors.New("project path does not exist")
	ErrProjectLanguageNotRecognized = errors.New("project language not recognized")
	ErrUnsupportedRemoteURL         = errors.New("unsupported remote URL")
	ErrNoVersionFileFound           = errors.New("no version file found")
	ErrLanguageNotFoundInConfig     = errors.New("language not found in config")
)

// loadProjectConfigOverrides searches for a per-project .autobump.yaml in the given
// project directory. If found, it reads the file and merges its languages section
// into the provided globalConfig, returning a new GlobalConfig without mutating the original.
// If no per-project config is found, the original globalConfig is returned unchanged.
func loadProjectConfigOverrides(
	globalConfig *entities.GlobalConfig,
	projectPath string,
) *entities.GlobalConfig {
	configPath := entities.FindProjectConfigFile(projectPath)
	if configPath == "" {
		return globalConfig
	}

	logger.Infof("Found per-project config: %s", configPath)

	projectOverrides, err := entities.ReadProjectConfig(configPath)
	if err != nil {
		logger.Warnf("Failed to read per-project config %s: %v, using global config", configPath, err)
		return globalConfig
	}

	if len(projectOverrides.LanguagesConfig) == 0 {
		return globalConfig
	}

	logger.Infof("Merging %d language override(s) from per-project config", len(projectOverrides.LanguagesConfig))
	return entities.CopyGlobalConfigWithLanguageOverrides(globalConfig, projectOverrides.LanguagesConfig)
}

// providerRegistry is set by the application at startup via SetProviderRegistry.
var providerRegistry *infraRepos.ProviderRegistry //nolint:gochecknoglobals // required for provider access

// SetProviderRegistry sets the provider registry for the commands package.
func SetProviderRegistry(reg *infraRepos.ProviderRegistry) {
	providerRegistry = reg
}

// gitOps is set by the application at startup via SetGitOperations.
var gitOps *gitInfra.GitOperations //nolint:gochecknoglobals // required for git operations

// SetGitOperations sets the GitOperations instance for the commands package.
func SetGitOperations(ops *gitInfra.GitOperations) {
	gitOps = ops
}

// RepoContext holds the context for processing a repository.
type RepoContext struct {
	GlobalConfig    *entities.GlobalConfig
	ProjectConfig   *entities.ProjectConfig
	GlobalGitConfig *gitconfig.Config
	Repo            *git.Repository
	Worktree        *git.Worktree
	Head            *plumbing.Reference
}

//nolint:gochecknoglobals // read-only lookup table mapping langforge Language constants to common config key aliases
var langforgeAliases = map[langEntities.Language][]string{
	langEntities.LanguageGo:         {"golang"},
	langEntities.LanguageNode:       {"typescript", "javascript"},
	langEntities.LanguageJava:       {"java"},
	langEntities.LanguageJavaGradle: {"java"},
	langEntities.LanguageJavaMaven:  {"java"},
	langEntities.LanguageCSharp:     {"cs"},
	langEntities.LanguagePython:     {},
	langEntities.LanguageTerraform:  {},
	langEntities.LanguageYAML:       {},
	langEntities.LanguageDockerfile: {},
	langEntities.LanguagePipeline:   {},
	langEntities.LanguageUnknown:    {},
}

// resolveConfigKey maps a langforge Language constant to the corresponding
// configuration key present in the user's GlobalConfig.
func resolveConfigKey(globalConfig *entities.GlobalConfig, lang langEntities.Language) string {
	langName := string(lang)
	if _, ok := globalConfig.LanguagesConfig[langName]; ok {
		return langName
	}
	for _, alias := range langforgeAliases[lang] {
		if _, ok := globalConfig.LanguagesConfig[alias]; ok {
			return alias
		}
	}
	return ""
}

// DetectProjectLanguage detects the language of a project by looking at the files in the project.
func DetectProjectLanguage(globalConfig *entities.GlobalConfig, cwd string) (string, error) {
	logger.Info("Detecting project language")

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Primary: use langforge's registry detection (marker files like go.mod, package.json, etc.)
	registry := langRegistry.NewDefaultRegistry()
	provider, detectErr := registry.Detect(absPath)
	if detectErr == nil {
		configKey := resolveConfigKey(globalConfig, provider.Language())
		if configKey != "" {
			logger.Infof("Project language detected as %s via marker files", configKey)
			return configKey, nil
		}
	}

	// Fallback: config-driven special patterns (for languages langforge doesn't know)
	if language := detectBySpecialPatterns(globalConfig, absPath); language != "" {
		return language, nil
	}

	// Extension-based detection using langforge's classifier
	configKey, err := detectByExtensions(globalConfig, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to walk project directory: %w", err)
	}
	if configKey != "" {
		return configKey, nil
	}

	return "", ErrProjectLanguageNotRecognized
}

// detectBySpecialPatterns checks the project type using config-driven special file patterns
// as a fallback for languages not covered by langforge's registry.
func detectBySpecialPatterns(globalConfig *entities.GlobalConfig, absPath string) string {
	for language, cfg := range globalConfig.LanguagesConfig {
		for _, pattern := range cfg.SpecialPatterns {
			matches, _ := filepath.Glob(filepath.Join(absPath, pattern))
			if len(matches) > 0 {
				logger.Infof("Project language detected as %s via file pattern '%s'", language, pattern)
				return language
			}
		}
	}
	return ""
}

// detectByExtensions checks the project type using langforge's file extension classifier.
func detectByExtensions(globalConfig *entities.GlobalConfig, absPath string) (string, error) {
	var detected string
	err := filepath.Walk(absPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || detected != "" {
			return nil
		}
		lang := langEntities.ClassifyFileByExtension(info.Name())
		if lang != langEntities.LanguageUnknown {
			configKey := resolveConfigKey(globalConfig, lang)
			if configKey != "" {
				detected = configKey
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

// cloneRepo clones a remote repository into a temporary directory.
func cloneRepo(ctx *RepoContext) (string, error) {
	tmpDir, err := os.MkdirTemp("", "autobump-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	serviceType := gitOps.GetServiceTypeByURL(ctx.ProjectConfig.Path)
	authMethods := collectAuthMethods(
		serviceType,
		ctx.GlobalGitConfig.Raw.Section("user").Option("name"),
		ctx.GlobalConfig,
		ctx.ProjectConfig,
	)

	repo, err := gitOps.CloneRepo(ctx.ProjectConfig.Path, tmpDir, authMethods)
	if err != nil {
		return "", err
	}

	ctx.Repo = repo
	ctx.ProjectConfig.Path = tmpDir
	return tmpDir, nil
}

func createPullRequest(
	ctx *RepoContext,
	repo *git.Repository,
	branchName string,
	serviceType entities.ServiceType,
) error {
	token := resolveToken(serviceType, ctx.GlobalConfig, ctx.ProjectConfig)
	if token == "" {
		logger.Warnf("No token found for service type '%v', cannot create pull request", serviceType)
		return nil
	}

	provider, err := getForgeProvider(serviceType, token)
	if err != nil {
		logger.Warnf("Service type '%v' not supported for PR creation: %v", serviceType, err)
		return nil
	}

	remoteURL, err := gitInfra.GetRemoteRepoURL(repo)
	if err != nil {
		return err
	}

	targetBranch := resolveDefaultBranch(repo)
	gitforgeRepo := buildGitforgeRepo(remoteURL, targetBranch)
	input := globalEntities.PullRequestInput{
		SourceBranch: branchName,
		TargetBranch: targetBranch,
		Title:        "chore(bump): bumped version to " + ctx.ProjectConfig.NewVersion,
		Description:  generatePRDescription(ctx),
	}

	pr, err := provider.CreatePullRequest(context.Background(), gitforgeRepo, input)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	logger.Infof("Created PR #%d: %s", pr.ID, pr.URL)
	return nil
}

func generatePRDescription(ctx *RepoContext) string {
	var sb strings.Builder
	sb.WriteString("## Summary\n\n")
	fmt.Fprintf(&sb, "This PR bumps the version to **%s** for project **%s**.\n\n",
		ctx.ProjectConfig.NewVersion, ctx.ProjectConfig.Name,
	)

	sb.WriteString("### Changes\n\n")
	sb.WriteString("- Updated `CHANGELOG.md` with the new version and date\n")

	versionFiles, _ := getVersionFiles(ctx.GlobalConfig, ctx.ProjectConfig)
	for _, vf := range versionFiles {
		fmt.Fprintf(&sb, "- Updated version in `%s`\n", filepath.Base(vf.Path))
	}

	sb.WriteString("\n### Review Checklist\n\n")
	sb.WriteString("- [ ] Verify build passes\n")
	sb.WriteString("- [ ] Verify tests pass\n")
	sb.WriteString("- [ ] Review changelog entries\n")
	if len(versionFiles) > 0 {
		sb.WriteString("- [ ] Verify version file updates\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*This PR was automatically created by [AutoBump](https://github.com/rios0rios0/autobump)*\n")
	return sb.String()
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
		logger.Infof("Bump is empty, skipping project %s", ctx.ProjectConfig.Name)
		return false, nil
	}
	return true, nil
}

func ensureProjectLanguage(ctx *RepoContext) {
	if ctx.ProjectConfig.Language == "" {
		projectLanguage, err := DetectProjectLanguage(ctx.GlobalConfig, ctx.ProjectConfig.Path)
		if err != nil {
			logger.Warnf("Could not detect project language: %v, will only update changelog", err)
			ctx.ProjectConfig.Language = ""
			return
		}
		ctx.ProjectConfig.Language = projectLanguage
	}
}

func setupRepo(ctx *RepoContext) error {
	if ctx.Repo == nil {
		var err error
		ctx.Repo, err = gitInfra.OpenRepo(ctx.ProjectConfig.Path)
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

	branchExists, err := gitInfra.CheckBranchExists(ctx.Repo, branchName)
	if err != nil {
		return "", entities.BranchCreated, err
	}
	if branchExists {
		logger.Warnf("Branch '%s' already exists (local or remote)", branchName)
		return branchName, entities.BranchExistsNoPR, nil // Return branch name for PR check
	}

	err = gitInfra.CreateAndSwitchBranch(ctx.Repo, ctx.Worktree, branchName, ctx.Head.Hash())
	if err != nil {
		return "", entities.BranchCreated, err
	}

	return branchName, entities.BranchCreated, nil
}

func updateChangelogAndVersionFiles(ctx *RepoContext, changelogPath string) error {
	logger.Info("Updating CHANGELOG.md file")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		logger.Errorf("No version found in CHANGELOG.md for project at %s\n", ctx.ProjectConfig.Path)
		return err
	}

	ctx.ProjectConfig.NewVersion = version.String()
	logger.Infof("Updating version to %s", ctx.ProjectConfig.NewVersion)
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

		logger.Infof("Adding version file %s", versionFileRelativePath)
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
	_, err := commitChanges(ctx)
	if err != nil {
		return err
	}

	err = pushChanges(ctx, branchName)
	if err != nil {
		if err.Error() == "object not found" {
			logger.Error("Got error object not found (remote branch already exists?)")
		}
		return err
	}

	return nil
}

// commitChanges commits the staged changes with optional GPG or SSH signing.
func commitChanges(ctx *RepoContext) (plumbing.Hash, error) {
	cfg, err := ctx.Repo.Config()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to get repo config: %w", err)
	}

	gpgSign := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "commit", "gpgsign")
	gpgFormat := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "gpg", "format")
	signingKey := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "signingkey")

	name := ctx.GlobalGitConfig.Raw.Section("user").Option("name")
	email := ctx.GlobalGitConfig.Raw.Section("user").Option("email")
	commitMessage := "chore(bump): bumped version to " + ctx.ProjectConfig.NewVersion

	sshProgram := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "gpg.ssh", "program")

	signer, err := signingInfra.ResolveSignerFromGitConfig(
		gpgSign, gpgFormat, signingKey,
		ctx.GlobalConfig.GpgKeyPath, ctx.GlobalConfig.GpgKeyPassphrase, "autobump", sshProgram,
	)
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to resolve commit signer: %w", err)
	}

	return gitInfra.CommitChanges(ctx.Repo, ctx.Worktree, commitMessage, signer, name, email)
}

func pushChanges(ctx *RepoContext, branchName string) error {
	refSpec := gitconfig.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)

	serviceType, err := gitOps.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		return err
	}

	username := ctx.GlobalGitConfig.Raw.Section("user").Option("name")
	authMethods := collectAuthMethods(serviceType, username, ctx.GlobalConfig, ctx.ProjectConfig)

	return gitInfra.PushWithTransportDetection(ctx.Repo, refSpec, authMethods)
}

func createAndCheckoutPullRequest(ctx *RepoContext, branchName string) error {
	serviceType, err := gitOps.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		return err
	}

	err = createPullRequest(ctx, ctx.Repo, branchName, serviceType)
	if err != nil {
		return err
	}

	return checkoutToMainBranch(ctx)
}

func checkoutToMainBranch(ctx *RepoContext) error {
	err := gitInfra.CheckoutBranch(ctx.Worktree, "main")
	if err != nil {
		return gitInfra.CheckoutBranch(ctx.Worktree, "master")
	}
	return nil
}

// addCurrentVersion adds the current version to the CHANGELOG file.
func addCurrentVersion(ctx *RepoContext, changelogPath string) error {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return err
	}

	latestTag, err := gitInfra.GetLatestTag(ctx.Repo)
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
	ctx.GlobalGitConfig, err = gitHelpers.GetGlobalGitConfig()
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

	// Load per-project config overrides (must happen after clone so files are available)
	ctx.GlobalConfig = loadProjectConfigOverrides(ctx.GlobalConfig, ctx.ProjectConfig.Path)

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

	logger.Infof("Successfully processed project '%s'", ctx.ProjectConfig.Name)
	return nil
}

// handleExistingBranchWithoutPR handles the case where a bump branch exists but no PR was found.
func handleExistingBranchWithoutPR(ctx *RepoContext, branchName string) error {
	// Branch exists, check if PR exists
	prExists, prErr := checkPullRequestExists(ctx, branchName)
	if prErr != nil {
		logger.Warnf("Failed to check if PR exists: %v, skipping project", prErr)
		return nil
	}
	if prExists {
		logger.Infof("Pull request already exists for branch '%s', skipping project", branchName)
		return nil
	}
	// PR doesn't exist, create it
	logger.Infof("Branch exists but no PR found, creating pull request for branch '%s'", branchName)
	if err := createAndCheckoutPullRequest(ctx, branchName); err != nil {
		return err
	}
	logger.Infof("Successfully created PR for existing branch in project '%s'", ctx.ProjectConfig.Name)
	return nil
}

// checkPullRequestExists checks if a PR exists for the given branch using the appropriate provider.
func checkPullRequestExists(ctx *RepoContext, branchName string) (bool, error) {
	serviceType, err := gitOps.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		return false, err
	}

	token := resolveToken(serviceType, ctx.GlobalConfig, ctx.ProjectConfig)
	if token == "" {
		logger.Warnf("No token found for service type '%v', cannot check PR", serviceType)
		return false, nil
	}

	provider, provErr := getForgeProvider(serviceType, token)
	if provErr != nil {
		logger.Warnf("Service type '%v' not supported for PR check: %v", serviceType, provErr)
		return false, nil
	}

	remoteURL, urlErr := gitInfra.GetRemoteRepoURL(ctx.Repo)
	if urlErr != nil {
		return false, urlErr
	}

	targetBranch := resolveDefaultBranch(ctx.Repo)
	gitforgeRepo := buildGitforgeRepo(remoteURL, targetBranch)
	return provider.PullRequestExists(context.Background(), gitforgeRepo, branchName)
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
				logger.Errorf("Project path does not exist: %s\n", project.Path)
				logger.Warn("Skipping project")
				err = ErrProjectPathDoesNotExist
				continue
			}
		}

		err = ProcessRepo(globalConfig, &project)
		if err != nil {
			logger.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}

	return err
}

// DiscoverAndProcess discovers repositories from configured providers and processes each one.
func DiscoverAndProcess(
	ctx context.Context,
	globalConfig *entities.GlobalConfig,
	registry *infraRepos.ProviderRegistry,
) error {
	totalRepos := 0
	totalErrors := 0

	for _, provCfg := range globalConfig.Providers {
		discoverer, err := registry.GetDiscoverer(provCfg.Type, provCfg.Token)
		if err != nil {
			logger.Errorf("Failed to initialize provider %q: %v", provCfg.Type, err)
			totalErrors++
			continue
		}

		logger.Infof("Processing provider: %s", discoverer.Name())

		for _, org := range provCfg.Organizations {
			logger.Infof("Discovering repositories in %q...", org)

			repos, discoverErr := discoverer.DiscoverRepositories(ctx, org)
			if discoverErr != nil {
				logger.Errorf("Failed to discover repos in %q: %v", org, discoverErr)
				totalErrors++
				continue
			}

			logger.Infof("Found %d repositories in %q", len(repos), org)

			for _, repo := range repos {
				totalRepos++
				projectConfig := repoToProjectConfig(repo, provCfg)
				if processErr := ProcessRepo(globalConfig, projectConfig); processErr != nil {
					logger.Errorf(
						"Error processing %s/%s: %v",
						repo.Organization, repo.Name, processErr,
					)
					totalErrors++
				}
			}
		}
	}

	logger.Infof("Discovery complete: %d repos processed, %d errors", totalRepos, totalErrors)
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

// ---- Provider helpers ----

// resolveToken returns the best token for a given service type from the config.
func resolveToken(
	serviceType entities.ServiceType,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) string {
	// Project access token always takes priority
	if projectConfig.ProjectAccessToken != "" {
		return projectConfig.ProjectAccessToken
	}

	switch serviceType {
	case entities.GITHUB:
		return globalConfig.GitHubAccessToken
	case entities.GITLAB:
		if globalConfig.GitLabAccessToken != "" {
			return globalConfig.GitLabAccessToken
		}
		return globalConfig.GitLabCIJobToken
	case entities.AZUREDEVOPS:
		return globalConfig.AzureDevOpsAccessToken
	default:
		return ""
	}
}

// collectTokens returns all possible tokens for authentication, ordered by priority.
func collectTokens(
	serviceType entities.ServiceType,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) []string {
	var tokens []string

	if projectConfig.ProjectAccessToken != "" {
		tokens = append(tokens, projectConfig.ProjectAccessToken)
	}

	switch serviceType {
	case entities.GITHUB:
		if globalConfig.GitHubAccessToken != "" {
			tokens = append(tokens, globalConfig.GitHubAccessToken)
		}
	case entities.GITLAB:
		if globalConfig.GitLabAccessToken != "" {
			tokens = append(tokens, globalConfig.GitLabAccessToken)
		}
		if globalConfig.GitLabCIJobToken != "" {
			tokens = append(tokens, globalConfig.GitLabCIJobToken)
		}
	case entities.AZUREDEVOPS:
		if globalConfig.AzureDevOpsAccessToken != "" {
			tokens = append(tokens, globalConfig.AzureDevOpsAccessToken)
		}
	}

	return tokens
}

// collectAuthMethods creates providers with each available token and collects
// all authentication methods.
func collectAuthMethods(
	serviceType entities.ServiceType,
	username string,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) []transport.AuthMethod {
	tokens := collectTokens(serviceType, globalConfig, projectConfig)
	name := registryInfra.ServiceTypeToProviderName(serviceType)
	if name == "" || providerRegistry == nil {
		return nil
	}

	var authMethods []transport.AuthMethod
	for _, token := range tokens {
		provider, err := providerRegistry.Get(name, token)
		if err != nil {
			continue
		}
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		if !ok {
			continue
		}
		lgap.ConfigureTransport()
		methods := lgap.GetAuthMethods(username)
		authMethods = append(authMethods, methods...)
	}

	sshMethods := collectSSHAuthMethods(globalConfig)
	authMethods = append(authMethods, sshMethods...)

	return authMethods
}

// collectSSHAuthMethods builds SSH transport.AuthMethod instances from config.
// It tries explicit key/socket config first, then auto-detects common SSH agent sockets.
func collectSSHAuthMethods(globalConfig *entities.GlobalConfig) []transport.AuthMethod {
	var methods []transport.AuthMethod

	if globalConfig.SSHKeyPath != "" {
		auth, err := gitssh.NewPublicKeysFromFile("git", globalConfig.SSHKeyPath, globalConfig.SSHKeyPassphrase)
		if err != nil {
			logger.Warnf("Failed to load SSH key from %s: %v", globalConfig.SSHKeyPath, err)
		} else {
			auth.HostKeyCallback = hostKeyCallback()
			methods = append(methods, auth)
		}
	}

	if globalConfig.SSHAuthSock != "" {
		if method := sshAgentAuthFromSocket(globalConfig.SSHAuthSock); method != nil {
			methods = append(methods, method)
		}
	}

	if len(methods) > 0 {
		return methods
	}

	// Auto-detect common SSH agent sockets
	for _, sock := range detectSSHAgentSockets() {
		if method := sshAgentAuthFromSocket(sock); method != nil {
			methods = append(methods, method)
			break // use the first working socket
		}
	}

	return methods
}

// sshAgentAuthFromSocket returns an SSH agent auth method that dials the given Unix socket
// on each use and closes the connection after retrieving the available signers.
func sshAgentAuthFromSocket(socketPath string) transport.AuthMethod {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		logger.Debugf("Cannot connect to SSH agent at %s: %v", socketPath, err)
		return nil
	}
	if closeErr := conn.Close(); closeErr != nil {
		logger.Debugf("Failed to close SSH agent probe connection for %s: %v", socketPath, closeErr)
	}

	return &gitssh.PublicKeysCallback{
		User: "git",
		Callback: func() ([]ssh.Signer, error) {
			c, dialErr := dialer.DialContext(context.Background(), "unix", socketPath)
			if dialErr != nil {
				return nil, dialErr
			}
			defer func() {
				if closeErr := c.Close(); closeErr != nil {
					logger.Debugf("Failed to close SSH agent connection for %s: %v", socketPath, closeErr)
				}
			}()

			agentClient := agent.NewClient(c)
			return agentClient.Signers()
		},
		HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
			HostKeyCallback: hostKeyCallback(),
		},
	}
}

// detectSSHAgentSockets returns paths to common SSH agent sockets that exist on the filesystem.
func detectSSHAgentSockets() []string {
	var sockets []string

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if info, statErr := os.Stat(sock); statErr == nil && info.Mode().Type() == os.ModeSocket { //nolint:gosec // sock comes from SSH_AUTH_SOCK env var, trusted input
			sockets = append(sockets, sock)
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".1password", "agent.sock"),
		}
		for _, c := range candidates {
			if info, statErr := os.Stat(c); statErr == nil && info.Mode().Type() == os.ModeSocket {
				sockets = append(sockets, c)
			}
		}
	}

	return sockets
}

// hostKeyCallback returns an ssh.HostKeyCallback that validates against the user's
// known_hosts file. Falls back to InsecureIgnoreHostKey if no known_hosts is available.
func hostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("Cannot determine home directory, using insecure host key verification")
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // fallback when home dir unavailable
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		logger.Warnf("Cannot load known_hosts from %s: %v, using insecure host key verification", knownHostsPath, err)
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // fallback when known_hosts unavailable
	}

	return ssh.HostKeyCallback(cb)
}

// getForgeProvider creates a gitforge ForgeProvider with the given token.
func getForgeProvider(
	serviceType entities.ServiceType,
	token string,
) (globalEntities.ForgeProvider, error) {
	name := registryInfra.ServiceTypeToProviderName(serviceType)
	if name == "" || providerRegistry == nil {
		return nil, fmt.Errorf("unsupported service type: %v", serviceType)
	}
	return providerRegistry.Get(name, token)
}

// resolveDefaultBranch determines the default branch from the remote origin/HEAD reference.
func resolveDefaultBranch(repo *git.Repository) string {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), true)
	if err != nil {
		return "main"
	}
	branch := ref.Name().Short()
	branch = strings.TrimPrefix(branch, "origin/")
	if branch == "" {
		return "main"
	}
	return branch
}

// buildGitforgeRepo constructs a gitforge Repository entity from a remote URL.
func buildGitforgeRepo(remoteURL string, defaultBranch string) globalEntities.Repository {
	parsed, err := gitInfra.ParseRemoteURL(remoteURL)
	if err != nil {
		logger.WithField("remoteURL", remoteURL).Warn("could not parse organization or repository name from remote URL")
		return globalEntities.Repository{
			DefaultBranch: "refs/heads/" + defaultBranch,
			RemoteURL:     remoteURL,
		}
	}

	return globalEntities.Repository{
		Name:          parsed.RepoName,
		Organization:  parsed.Organization,
		Project:       parsed.Project,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     remoteURL,
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
		logger.Warnf("Creating empty CHANGELOG file at '%s'.", changelogPath)
		var fileContent []byte
		fileContent, err = downloadHelpers.DownloadFile(entities.DefaultChangelogURL)
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
		logger.Info("Language is unknown, skipping version file updates (only changelog will be updated)")
		return nil
	}

	versionFiles, err := getVersionFiles(globalConfig, projectConfig)
	if err != nil {
		// If language config not found, just warn and continue with changelog only
		if errors.Is(err, ErrLanguageNotFoundInConfig) {
			logger.Warnf("Language '%s' not found in config, skipping version file updates", projectConfig.Language)
			return nil
		}
		return err
	}

	// If no version files configured for this language, just continue
	if len(versionFiles) == 0 {
		logger.Warnf(
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
			logger.Warnf("Version file %s does not exist", versionFile.Path)
			continue
		}
		logger.Infof("Updating version file %s", versionFile.Path)

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

		//nolint:gosec // G703 false positive: path originates from filepath.Glob and is validated by os.Stat above
		err = os.WriteFile(versionFile.Path, []byte(updatedContent), originalFileMode)
		if err != nil {
			return fmt.Errorf("failed to write to file %s: %w", versionFile.Path, err)
		}
	}

	// If no version files exist, just warn and continue (don't fail)
	if !oneVersionFileExists {
		logger.Warnf("No version files found for language '%s', only changelog will be updated", projectConfig.Language)
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
			logger.Infof("Using project name '%s' from language interface", languageProjectName)
			projectName = strings.ReplaceAll(languageProjectName, "-", "_")
		}
	} else {
		logger.Infof("Language '%s' does not have a language interface", projectConfig.Language)
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
