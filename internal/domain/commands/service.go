package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	logger "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/python"
	"github.com/rios0rios0/autobump/internal/support"
	downloadHelpers "github.com/rios0rios0/gitforge/v4/pkg/config/infrastructure/helpers"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	gitHelpers "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure/helpers"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/v4/pkg/registry/infrastructure"
	signingInfra "github.com/rios0rios0/gitforge/v4/pkg/signing/infrastructure"
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

// loadProjectConfigOverrides folds the repository's own .autobump.yaml onto the resolved
// configuration and returns the result, leaving the one it was given untouched.
//
// This is the last of the four configuration layers, and the only one that is read out of
// the repository being released -- which in `run` mode is a repository AutoBump discovered
// rather than one anybody vetted. It is applied with ScopeRestricted, so the keys that are
// the operator's alone (credentials, `providers`, `projects`, the branch prefix that aims
// stale-branch deletion) have no field to land in. Everything else layers exactly as the
// operator's own file does.
//
// A missing file is not an error, and neither is an unparseable one: the release proceeds
// on the configuration resolved without it, which is what a project that cannot be read
// has always got.
func loadProjectConfigOverrides(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	projectPath string,
) *entities.GlobalConfig {
	configPath := entities.FindProjectConfigFile(projectPath)
	if configPath == "" {
		return globalConfig
	}

	logger.Infof("Found per-project config: %s", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		logger.Warnf("Failed to read per-project config %s: %v; ignoring it", configPath, err)
		return globalConfig
	}

	//nolint:exhaustruct // Strict is false for a restricted layer by construction
	merged, err := entities.ApplyProjectLayer(globalConfig, projectConfig, entities.ConfigLayer{
		Name:     entities.LayerProjectConfig,
		Origin:   configPath,
		Data:     data,
		Scope:    entities.ScopeRestricted,
		Optional: true,
	})
	if err != nil {
		logger.Warnf("Failed to apply the per-project config %s: %v; ignoring it", configPath, err)
		return globalConfig
	}

	return merged
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

	// GoModulePathChange is the module path rewrite the release made, nil when it made
	// none. It is kept so the pull request can say what moved.
	GoModulePathChange *GoModulePathChange

	// ToolingDir is where the lockfile refresh keeps its package managers' caches and
	// temporary files: the `tooling/` directory of a remote repository's workspace, which
	// is removed with the clone. It is empty for a local project, whose refresh runs in
	// the operator's own checkout with the operator's own caches.
	ToolingDir string

	// Cloned reports that the checkout is AutoBump's own clone, removed after the run,
	// rather than the operator's working tree. Only a clone is moved to another branch
	// with files written on it: the way back is a plain checkout, which go-git refuses
	// over uncommitted edits after it has already moved HEAD, leaving the new file staged
	// on the branch the run started on.
	Cloned bool
}

// langIDJava is the user-facing config key alias shared by all Java langforge variants
// (plain Java, Gradle, and Maven). Extracted to satisfy goconst.
const langIDJava = "java"

//nolint:gochecknoglobals // read-only lookup table mapping langforge Language constants to common config key aliases
var langforgeAliases = map[langEntities.Language][]string{
	langEntities.LanguageGo:         {"golang"},
	langEntities.LanguageNode:       {"typescript", "javascript"},
	langEntities.LanguageJava:       {langIDJava},
	langEntities.LanguageJavaGradle: {langIDJava},
	langEntities.LanguageJavaMaven:  {langIDJava},
	langEntities.LanguageCSharp:     {"cs"},
	langEntities.LanguageDart:       {"flutter"},
	langEntities.LanguagePython:     {},
	langEntities.LanguageRuby:       {},
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

// cloneRepo clones a remote repository into a new workspace (see workspace.go) and
// returns the workspace's root.
//
// The root is returned with the error too, whenever the directory was created: a clone
// that fails halfway leaves the directory behind as surely as one that succeeds, so the
// caller removes it either way.
func cloneRepo(ctx *RepoContext) (string, error) {
	root, err := newRepositoryWorkspace()
	if err != nil {
		return root, err
	}

	serviceType := gitOps.GetServiceTypeByURL(ctx.ProjectConfig.Path)
	authMethods := collectAuthMethods(
		serviceType,
		ctx.GlobalGitConfig.Raw.Section("user").Option("name"),
		ctx.GlobalConfig,
		ctx.ProjectConfig,
	)

	repoDir := workspaceRepoPath(root)
	repo, err := gitOps.CloneRepo(ctx.ProjectConfig.Path, repoDir, authMethods)
	if err != nil {
		return root, err
	}
	recordRemoteDefaultBranch(repo)

	ctx.Repo = repo
	ctx.ProjectConfig.Path = repoDir
	ctx.ToolingDir = workspaceToolingPath(root)
	ctx.Cloned = true
	return root, nil
}

func createPullRequest(
	ctx *RepoContext,
	repo *git.Repository,
	branchName string,
	serviceType entities.ServiceType,
) error {
	return openPullRequest(
		ctx, repo, branchName, serviceType,
		"chore(bump): bumped version to "+ctx.ProjectConfig.NewVersion, generatePRDescription(ctx),
	)
}

// openPullRequest opens a pull request from branchName to the repository's default branch.
// A missing token or an unsupported forge is only logged, because a run without a token
// never meant to open one.
func openPullRequest(
	ctx *RepoContext,
	repo *git.Repository,
	branchName string,
	serviceType entities.ServiceType,
	title, description string,
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
	gitforgeRepo := buildGitforgeRepo(remoteURL, targetBranch, ctx.ProjectConfig.RepositoryID)
	input := globalEntities.PullRequestInput{
		SourceBranch: branchName,
		TargetBranch: targetBranch,
		Title:        title,
		Description:  description,
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
	changelogFile := ctx.ProjectConfig.ChangelogPath
	if changelogFile == "" {
		changelogFile = "CHANGELOG.md"
	}
	fmt.Fprintf(&sb, "- Updated `%s` with the new version and date\n", changelogFile)

	versionFiles, _ := getVersionFiles(ctx.GlobalConfig, ctx.ProjectConfig)
	for _, vf := range versionFiles {
		fmt.Fprintf(&sb, "- Updated version in `%s`\n", filepath.Base(vf.Path))
	}
	if change := ctx.GoModulePathChange; change != nil {
		fmt.Fprintf(&sb, "- Changed the Go module path from `%s` to `%s` in %d file(s)\n",
			change.From, change.To, len(change.Files))
	}

	sb.WriteString("\n### Review Checklist\n\n")
	sb.WriteString("- [ ] Verify build passes\n")
	sb.WriteString("- [ ] Verify tests pass\n")
	sb.WriteString("- [ ] Review changelog entries\n")
	if len(versionFiles) > 0 {
		sb.WriteString("- [ ] Verify version file updates\n")
	}
	if change := ctx.GoModulePathChange; change != nil {
		fmt.Fprintf(&sb, "- [ ] Plan the consumers' move to `%s`: their imports change with the module path\n",
			change.To)
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

// initialChangelogBranch is the branch a newly created changelog is proposed from. It sits
// outside the bump prefix, so stale-branch cleanup never takes it for a release.
const initialChangelogBranch = "chore/add-changelog"

// setupChangelog ensures the CHANGELOG file exists. It returns true when the changelog
// already existed, and false when it had to be created: until it is merged, the project has
// no changelog to release from.
//
// AutoBump's own clone proposes the new changelog through a pull request. The operator's
// checkout gets it committed to the branch they are on instead, because proposing it means
// switching branches and back. The switch away keeps their files, but the way back is a plain
// checkout that go-git refuses over their uncommitted edits, after it has already moved HEAD:
// they would be left on their branch with the proposed changelog staged for their next commit.
func setupChangelog(ctx *RepoContext, changelogPath string) (bool, error) {
	if _, err := os.Stat(changelogPath); !os.IsNotExist(err) {
		return true, nil
	}
	if ctx.Cloned {
		return false, proposeInitialChangelog(ctx, changelogPath)
	}
	return false, commitInitialChangelogInPlace(ctx, changelogPath)
}

// commitInitialChangelogInPlace creates the missing changelog in the operator's own checkout,
// commits it to the branch they are on and pushes that branch, without switching branches.
// The commit and the push are best-effort: a branch that refuses a direct push is the
// operator's to handle, and a failure is logged.
func commitInitialChangelogInPlace(ctx *RepoContext, changelogPath string) error {
	if _, err := createChangelogIfNotExists(changelogPath); err != nil {
		return err
	}
	if err := addCurrentVersion(ctx, changelogPath); err != nil {
		return err
	}

	if err := commitInitialChangelog(ctx, changelogPath); err != nil {
		logger.Warnf("Failed to commit the new changelog: %v", err)
		return nil
	}
	if err := pushChanges(ctx, ctx.Head.Name().Short()); err != nil {
		logger.Warnf("Failed to push the new changelog: %v", err)
	}
	return nil
}

// proposeInitialChangelog creates the missing changelog on initialChangelogBranch and opens a
// pull request for it, instead of committing it to the branch the clone started on.
//
// That branch is the default branch, and a default branch that accepts changes only through
// pull requests -- the usual state of a protected branch on every forge -- refused the direct
// push. The changelog was never persisted, so every run created the same file and lost it
// again. A branch an earlier run left behind means the changelog is already proposed: it gets
// a pull request when it has none open, and is otherwise left alone. The clone is fresh on
// every run, so such a branch is one the remote holds, never one an earlier push failed on.
//
// Only creating the file can fail the release. Proposing it is best-effort, as the direct push
// it replaces was: a failure is logged and the run moves on. The branch is created at the
// commit the clone is on, so CreateAndSwitchBranch only points HEAD at it, and the run returns
// to the branch it started on afterwards, whatever happened.
func proposeInitialChangelog(ctx *RepoContext, changelogPath string) error {
	proposed, err := gitInfra.CheckBranchExists(ctx.Repo, initialChangelogBranch)
	if err != nil {
		return err
	}
	if proposed {
		logger.Infof("The branch '%s' already proposes a changelog", initialChangelogBranch)
		ensureInitialChangelogPullRequest(ctx, changelogPath)
		return nil
	}

	if err = gitInfra.CreateAndSwitchBranch(
		ctx.Repo, ctx.Worktree, initialChangelogBranch, ctx.Head.Hash(),
	); err != nil {
		return err
	}
	defer returnToStartingBranch(ctx)

	if _, err = createChangelogIfNotExists(changelogPath); err != nil {
		return err
	}
	if err = addCurrentVersion(ctx, changelogPath); err != nil {
		return err
	}

	if err = commitInitialChangelog(ctx, changelogPath); err != nil {
		logger.Warnf("Failed to commit the new changelog: %v", err)
		return nil
	}
	if err = pushChanges(ctx, initialChangelogBranch); err != nil {
		logger.Warnf("Failed to push the branch '%s' proposing the new changelog: %v", initialChangelogBranch, err)
		return nil
	}

	ensureInitialChangelogPullRequest(ctx, changelogPath)
	return nil
}

// ensureInitialChangelogPullRequest opens the pull request proposing the new changelog, unless
// one is already open. Every failure is logged rather than returned, as for the proposal itself.
func ensureInitialChangelogPullRequest(ctx *RepoContext, changelogPath string) {
	open, err := checkPullRequestExists(ctx, initialChangelogBranch)
	if err != nil {
		logger.Warnf("Could not check for a pull request proposing the new changelog: %v", err)
		return
	}
	if open {
		logger.Infof("A pull request proposing the new changelog is already open")
		return
	}

	serviceType, err := gitOps.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		logger.Warnf("Could not open the pull request proposing the new changelog: %v", err)
		return
	}

	changelogFile := changelogRelativePath(ctx, changelogPath)
	if err = openPullRequest(
		ctx, ctx.Repo, initialChangelogBranch, serviceType,
		initialChangelogCommitMessage(changelogFile), generateInitialChangelogPRDescription(changelogFile),
	); err != nil {
		logger.Warnf("Could not open the pull request proposing the new changelog: %v", err)
	}
}

// returnToStartingBranch checks out the branch the run started on again -- the default branch,
// for a clone. A failure is only logged: nothing after it depends on the checkout.
func returnToStartingBranch(ctx *RepoContext) {
	var err error
	if ctx.Head.Name().IsBranch() {
		err = gitInfra.CheckoutBranch(ctx.Worktree, ctx.Head.Name().Short())
	} else {
		err = ctx.Worktree.Checkout(&git.CheckoutOptions{Hash: ctx.Head.Hash()})
	}
	if err != nil {
		logger.Warnf("Could not return to '%s': %v", ctx.Head.Name().Short(), err)
	}
}

// changelogRelativePath returns the changelog's path relative to the project root, falling
// back to its base name when the two cannot be related.
func changelogRelativePath(ctx *RepoContext, changelogPath string) string {
	relative, err := filepath.Rel(ctx.ProjectConfig.Path, changelogPath)
	if err != nil {
		return filepath.Base(changelogPath)
	}
	return relative
}

// initialChangelogCommitMessage is the subject of the commit adding the new changelog, and the
// title of the pull request proposing it.
func initialChangelogCommitMessage(changelogFile string) string {
	return "chore: added " + changelogFile
}

func generateInitialChangelogPRDescription(changelogFile string) string {
	var sb strings.Builder
	sb.WriteString("## Summary\n\n")
	fmt.Fprintf(&sb, "This PR adds `%s`, which AutoBump reads to decide when and how this project is released.\n\n",
		changelogFile)
	sb.WriteString("Once it is merged, record each change under `[Unreleased]`, and the next run releases them.\n")
	sb.WriteString("\n---\n")
	sb.WriteString("*This PR was automatically created by [AutoBump](https://github.com/rios0rios0/autobump)*\n")
	return sb.String()
}

// commitInitialChangelog stages and commits the freshly created changelog file, signed the way
// every other AutoBump commit is.
func commitInitialChangelog(ctx *RepoContext, changelogPath string) error {
	changelogFile := changelogRelativePath(ctx, changelogPath)
	if _, err := ctx.Worktree.Add(changelogFile); err != nil {
		return fmt.Errorf("failed to stage changelog: %w", err)
	}

	cfg, err := ctx.Repo.Config()
	if err != nil {
		return fmt.Errorf("failed to get repo config: %w", err)
	}

	gpgSign := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "commit", "gpgsign")
	gpgFormat := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "gpg", "format")
	signingKey := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "signingkey")
	sshProgram := resolveSSHProgram(cfg, ctx.GlobalGitConfig)

	name := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "name")
	email := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "email")

	signer, err := signingInfra.ResolveSignerFromGitConfig(
		gpgSign, gpgFormat, signingKey,
		ctx.GlobalConfig.GpgKeyPath, ctx.GlobalConfig.GpgKeyPassphrase, "autobump", sshProgram,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve commit signer: %w", err)
	}

	if _, err = gitInfra.CommitChanges(
		ctx.Repo, ctx.Worktree, initialChangelogCommitMessage(changelogFile), signer, name, email,
	); err != nil {
		return fmt.Errorf("failed to commit changelog: %w", err)
	}
	return nil
}

func shouldBumpProject(ctx *RepoContext, changelogPath string) (bool, error) {
	lines, err := readChangelogLines(ctx.GlobalConfig, ctx.ProjectConfig, changelogPath)
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
	nextVersion, err := getNextVersionString(ctx.GlobalConfig, ctx.ProjectConfig, changelogPath)
	if err != nil {
		return "", entities.BranchCreated, err
	}

	branchName := entities.ResolveBumpBranchPrefix(ctx.GlobalConfig) + nextVersion
	// Store the version for PR creation even if branch exists
	ctx.ProjectConfig.NewVersion = nextVersion

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
	// Prepared before the changelog is written: the release notes announce the rewrite, and
	// a rewrite that cannot be made has to stop the release before anything on disk moved.
	moduleChange, err := prepareGoModulePathChange(ctx, changelogPath)
	if err != nil {
		return err
	}

	logger.Infof("Updating changelog file %s", changelogPath)
	version, err := updateChangelogFileString(
		ctx.GlobalConfig, ctx.ProjectConfig, changelogPath, moduleChange.ReleaseNote()...,
	)
	if err != nil {
		logger.Errorf("No version found in changelog for project at %s\n", ctx.ProjectConfig.Path)
		return err
	}

	ctx.ProjectConfig.NewVersion = version
	mode := entities.ResolveVersioning(ctx.GlobalConfig, ctx.ProjectConfig)

	// Refresh commands regenerate what the version files derive; fork mode rewrites no
	// version file, so there is nothing derived for them to catch up with.
	var refreshedFiles []string
	if IsForkVersioning(mode) {
		logger.Infof(
			"Fork versioning mode %q is active; skipping language-specific version file updates for %s",
			mode, ctx.ProjectConfig.NewVersion,
		)
	} else {
		logger.Infof("Updating version to %s", ctx.ProjectConfig.NewVersion)
		err = updateVersion(ctx.GlobalConfig, ctx.ProjectConfig)
		if err != nil {
			return err
		}

		refreshedFiles, err = runRefreshCommands(ctx.GlobalConfig, ctx.ProjectConfig, ctx.ToolingDir)
		if err != nil {
			return err
		}
	}

	rewrittenFiles, err := moduleChange.apply()
	if err != nil {
		return err
	}
	ctx.GoModulePathChange = moduleChange

	// The fragments have been folded into the release section above, so they must go.
	// Leaving them behind would ship the same entries again on the next run.
	consumed, err := consumeChlogFragments(ctx)
	if err != nil {
		return err
	}

	if err = stageRewrittenFiles(ctx, rewrittenFiles); err != nil {
		return err
	}

	return addFilesToWorktree(ctx, changelogPath, refreshedFiles, consumed)
}

// stageRewrittenFiles stages the files the module path rewrite changed. They are staged
// apart from the version files because no version_files rule names them: the rewrite
// found them, and only the rewrite knows they moved.
func stageRewrittenFiles(ctx *RepoContext, rewrittenPaths []string) error {
	for _, rewrittenPath := range rewrittenPaths {
		relativePath, err := filepath.Rel(ctx.ProjectConfig.Path, rewrittenPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for rewritten file: %w", err)
		}

		logger.Debugf("Adding rewritten file %s", relativePath)
		if _, err = ctx.Worktree.Add(relativePath); err != nil {
			return fmt.Errorf("failed to add rewritten file %s: %w", relativePath, err)
		}
	}

	return nil
}

// chlogConsumption is what a release did to the chlog fragment directory: the fragment
// files it removed, and the placeholder it left behind so the emptied directory survives
// the same commit. Both have to reach the worktree, one as a deletion and one as an
// addition, which is why they travel together.
type chlogConsumption struct {
	Removed []string
	Kept    string
}

// consumeChlogFragments deletes the chlog fragments that were just released and keeps
// their directory alive, returning both so the worktree can stage them. The fragments are
// re-read rather than threaded through the changelog writers: they are untouched on disk
// until this point, and re-reading keeps the I/O wrappers free of chlog state.
func consumeChlogFragments(ctx *RepoContext) (*chlogConsumption, error) {
	fragments, config, err := collectChlogFragments(ctx.GlobalConfig, ctx.ProjectConfig)
	if err != nil {
		return nil, err
	}
	if len(fragments) == 0 {
		return nil, nil //nolint:nilnil // nothing was consumed, so there is nothing to stage
	}

	deleted, err := DeleteChlogFragments(fragments)
	if err != nil {
		return nil, err
	}
	logger.Infof("Removed %d consumed chlog fragment(s)", len(deleted))

	// Removing the last fragment would otherwise remove the directory from the commit --
	// Git tracks files, not directories -- and with it the layout the next run detects
	// chlog by.
	kept, err := KeepChlogUnreleasedDirectory(ctx.ProjectConfig.Path, config)
	if err != nil {
		return nil, err
	}

	return &chlogConsumption{Removed: deleted, Kept: kept}, nil
}

func addFilesToWorktree(
	ctx *RepoContext,
	changelogPath string,
	refreshedPaths []string,
	consumed *chlogConsumption,
) error {
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

	// The refreshed files are staged separately from the version files because nothing
	// AutoBump rewrote points at them: they are whatever the refresh command declared,
	// and only the command knows they moved.
	for _, refreshedPath := range refreshedPaths {
		var refreshedRelativePath string
		refreshedRelativePath, err = filepath.Rel(projectPath, refreshedPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for refreshed file: %w", err)
		}

		logger.Infof("Adding refreshed file %s", refreshedRelativePath)
		if _, err = ctx.Worktree.Add(refreshedRelativePath); err != nil {
			return fmt.Errorf("failed to add refreshed file: %w", err)
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

	return stageChlogConsumption(ctx, consumed)
}

// stageChlogConsumption records the fragment directory's new state in the worktree: the
// consumed fragments as deletions, and the placeholder that keeps the directory in the
// tree as an addition.
//
// Staging a path that no longer exists on disk is what records its deletion, so the
// fragments disappear in the same commit that publishes their content.
func stageChlogConsumption(ctx *RepoContext, consumed *chlogConsumption) error {
	if consumed == nil {
		return nil
	}

	projectPath := ctx.ProjectConfig.Path

	for _, removedPath := range consumed.Removed {
		removedRelativePath, err := filepath.Rel(projectPath, removedPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for removed file: %w", err)
		}

		logger.Infof("Removing consumed file %s", removedRelativePath)
		if _, err = ctx.Worktree.Add(removedRelativePath); err != nil {
			return fmt.Errorf("failed to stage removal of %s: %w", removedRelativePath, err)
		}
	}

	if consumed.Kept == "" {
		return nil
	}

	keptRelativePath, err := filepath.Rel(projectPath, consumed.Kept)
	if err != nil {
		return fmt.Errorf("failed to get relative path for the chlog placeholder: %w", err)
	}

	logger.Infof("Adding chlog placeholder %s", keptRelativePath)
	if _, err = ctx.Worktree.Add(keptRelativePath); err != nil {
		return fmt.Errorf("failed to stage the chlog placeholder %s: %w", keptRelativePath, err)
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

// resolveSSHProgram reads gpg.ssh.program from the local or global git config.
// Git stores this value as the "program" option of the "ssh" subsection under the
// "gpg" section ([gpg "ssh"] program = ...), so it must be read with the
// subsection-aware helper. A flat section lookup for "gpg.ssh" never matches the
// subsection and silently returns "", which makes the SSH signer fall back to
// ssh-keygen instead of honoring a configured signer such as 1Password's
// op-ssh-sign-wsl.
func resolveSSHProgram(cfg, globalCfg *gitconfig.Config) string {
	return gitHelpers.GetSubsectionOptionFromConfig(cfg, globalCfg, "gpg", "ssh", "program")
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

	name := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "name")
	email := gitHelpers.GetOptionFromConfig(cfg, ctx.GlobalGitConfig, "user", "email")
	commitMessage := "chore(bump): bumped version to " + ctx.ProjectConfig.NewVersion

	sshProgram := resolveSSHProgram(cfg, ctx.GlobalGitConfig)

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

	err = gitInfra.PushWithTransportDetection(ctx.Repo, refSpec, authMethods)
	if err != nil {
		return explainSSHPushFailure(ctx, err)
	}

	return nil
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

// checkoutToMainBranch switches back to the repository's default branch once the pull
// request is open. The branch resolveDefaultBranch reports is tried before the
// conventional `main` and `master`, so a repository whose default branch is neither -- a
// fork working on `custom`, say -- is not reported as failing after its release opened.
func checkoutToMainBranch(ctx *RepoContext) error {
	branches := []string{fallbackDefaultBranch, "master"}
	if ctx.Repo != nil {
		defaultBranch := resolveDefaultBranch(ctx.Repo)
		branches = slices.DeleteFunc(branches, func(branch string) bool { return branch == defaultBranch })
		branches = append([]string{defaultBranch}, branches...)
	}

	var err error
	for _, branch := range branches {
		if err = gitInfra.CheckoutBranch(ctx.Worktree, branch); err == nil {
			return nil
		}
	}
	return err
}

// addCurrentVersion adds the current version to the CHANGELOG file.
// If the repository has no version tags or the tag cannot be resolved, the function logs a
// warning and returns nil so that the CHANGELOG creation still succeeds.
func addCurrentVersion(ctx *RepoContext, changelogPath string) error {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return err
	}

	latestTag, err := gitInfra.GetLatestTag(ctx.Repo)
	if err != nil {
		logger.Warnf("Could not determine latest tag, skipping historical version in CHANGELOG: %v", err)
		return nil
	}
	// gitforge before 4.3.0 returned a tag that is not a semantic version with a nil version
	// rather than an error. Formatting it wrote a literal `[<nil>]` header that no later run
	// could parse, which failed every release of the repository from then on.
	if latestTag.Tag == nil {
		logger.Warn("The latest tag is not a semantic version, skipping historical version in CHANGELOG")
		return nil
	}

	// TODO: we should replace <LINK TO THE PLATFORM TO OPEN THE PULL REQUEST> with the actual link

	// add lines to the end of the file
	lines = append(lines, []string{
		fmt.Sprintf("\n## [%s] - %s\n", latestTag.Tag, latestTag.Date.Format("2006-01-02")),
		"The changes weren't tracked until this version.",
	}...)
	return support.WriteLines(changelogPath, lines)
}

// resolveChangelogPath returns the absolute path of the project's changelog, defaulting
// to CHANGELOG.md. A chlog project that points changelogPath elsewhere in .chlog.yaml is
// honoured, so AutoBump writes where that project already keeps its changelog; an
// explicit changelog_path still wins over it.
//
// The project entry wins over the folded configuration, which is where a repository's own
// `changelog_path` and the operator's default both end up. Consulting the folded value is
// what makes a global `changelog_path` mean anything: it was decoded and never read
// before layering existed, so the setting the README documented did nothing.
//
// The configured path must stay inside the project root: an absolute or parent-escaping
// path would make the bumper read and rewrite a file outside the repository it was
// pointed at.
func resolveChangelogPath(ctx *RepoContext) (string, error) {
	changelogFile := ctx.ProjectConfig.ChangelogPath
	if changelogFile == "" && ctx.GlobalConfig != nil {
		changelogFile = ctx.GlobalConfig.ChangelogPath
	}
	if changelogFile == "" {
		var err error
		changelogFile, err = chlogChangelogPath(ctx)
		if err != nil {
			return "", err
		}
	}

	if !isPathInsideProject(changelogFile) {
		return "", fmt.Errorf("invalid changelog_path %q: must be relative to the project root", changelogFile)
	}

	return filepath.Join(ctx.ProjectConfig.Path, filepath.Clean(changelogFile)), nil
}

// chlogChangelogPath returns the changelog location a chlog project declares in
// .chlog.yaml, falling back to the plain CHANGELOG.md default for every other project.
//
// A broken or hostile .chlog.yaml is an error rather than a fallback: a project that
// commits one expects it to be honoured, and quietly using the default would look in the
// wrong place and release without the fragments.
func chlogChangelogPath(ctx *RepoContext) (string, error) {
	if !entities.ChlogEnabled(ctx.GlobalConfig, ctx.ProjectConfig) {
		return defaultChlogChangelogPath, nil
	}

	config, usesChlog, err := DetectChlog(ctx.ProjectConfig.Path)
	if err != nil {
		return "", err
	}
	if !usesChlog {
		return defaultChlogChangelogPath, nil
	}

	return config.ChangelogPath, nil
}

// ProcessRepo processes a repository:
// - clones the repository if it is a remote repository
// - creates the chore/bump branch
// - updates the changelog file
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

	// Clone repository if needed. The workspace is removed on every way out of this
	// function -- a release opened, nothing to release, a skip requested, or a failure,
	// the clone's own included -- so no repository leaves anything behind for the next.
	workspace, err := cloneRepoIfNeeded(ctx)
	defer removeWorkspace(workspace)
	if err != nil {
		return err
	}

	// Load per-project config overrides (must happen after clone so files are available)
	ctx.GlobalConfig = loadProjectConfigOverrides(ctx.GlobalConfig, ctx.ProjectConfig, ctx.ProjectConfig.Path)

	// A repository that opted out is left exactly as it was found. Everything below can
	// touch it: a missing changelog is created and pushed, stale bump branches are deleted
	// and their pull requests closed, and a release opens a branch and a pull request.
	if honourSkipRequest(ctx.ProjectConfig) {
		return nil
	}

	changelogPath, err := resolveChangelogPath(ctx)
	if err != nil {
		return err
	}

	logChlogDetection(ctx)

	// Setup repository and worktree
	err = setupRepo(ctx)
	if err != nil {
		return err
	}

	// Set up the changelog
	changelogExisted, err := setupChangelog(ctx, changelogPath)
	if err != nil {
		return err
	}
	if !changelogExisted {
		logger.Infof(
			"New %s was created for project '%s'; no unreleased content to bump",
			filepath.Base(changelogPath), ctx.ProjectConfig.Name,
		)
		return nil
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

	// Drop the bump branches left behind by earlier runs, closing their pull requests,
	// before the new one is created. This runs only once a bump is known to be needed,
	// so a pull request is never closed without a replacement being opened for it.
	if entities.CleanupEnabled(ctx.GlobalConfig) {
		cleanupStaleBumpBranches(ctx)
	}

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

// honourSkipRequest reports whether the repository's own .autobump.yaml asked to be left
// out of every release, and says so in the log when it did.
//
// The project name is empty when AutoBump was pointed at a path rather than handed a
// `projects[]` entry or a discovered repository, so the path's last element stands in for
// it, the way FinalizeGlobalConfig names an entry that has no name.
func honourSkipRequest(projectConfig *entities.ProjectConfig) bool {
	if !projectConfig.IsSkipped() {
		return false
	}

	name := projectConfig.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(projectConfig.Path), ".git")
	}

	if reason := projectConfig.SkipReason(); reason != "" {
		logger.Infof("Skipping project '%s': .autobump.yaml requested skip (%s)", name, reason)
	} else {
		logger.Infof("Skipping project '%s': .autobump.yaml requested skip", name)
	}

	return true
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
	gitforgeRepo := buildGitforgeRepo(remoteURL, targetBranch, ctx.ProjectConfig.RepositoryID)
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

// filterRepositories removes repositories that match the exclusion criteria
// defined in the settings (e.g. forks, archived repos).
func filterRepositories(
	repos []entities.Repository,
	globalConfig *entities.GlobalConfig,
) []entities.Repository {
	if !globalConfig.ExcludeForks && !globalConfig.ExcludeArchived {
		return repos
	}

	filtered := make([]entities.Repository, 0, len(repos))
	for _, repo := range repos {
		if globalConfig.ExcludeForks && repo.IsFork {
			logger.Debugf("Skipping fork: %s/%s", repo.Organization, repo.Name)
			continue
		}
		if globalConfig.ExcludeArchived && repo.IsArchived {
			logger.Debugf("Skipping archived repo: %s/%s", repo.Organization, repo.Name)
			continue
		}
		filtered = append(filtered, repo)
	}
	return filtered
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

			repos = filterRepositories(repos, globalConfig)
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
		RepositoryID:       repo.ID,
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
	case entities.UNKNOWN, entities.BITBUCKET, entities.CODECOMMIT, entities.CODEBERG:
		// These services have no global token of their own.
	}

	return ""
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
	case entities.UNKNOWN, entities.BITBUCKET, entities.CODECOMMIT, entities.CODEBERG:
		// These services have no global token of their own.
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
		HostKeyCallback: hostKeyCallback(),
	}
}

// detectSSHAgentSockets returns paths to common SSH agent sockets that exist on the filesystem.
func detectSSHAgentSockets() []string {
	var sockets []string

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		//nolint:gosec // sock comes from SSH_AUTH_SOCK env var, trusted input
		if info, statErr := os.Stat(sock); statErr == nil && info.Mode().Type() == os.ModeSocket {
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

// recordRemoteDefaultBranch points refs/remotes/origin/HEAD at the branch the clone checked
// out, which is the remote's default branch. `git clone` writes that reference and go-git
// does not, so resolveDefaultBranch found nothing on a clone and fell back to "main": a
// repository whose default branch is anything else -- a fork working on `custom`, say --
// had its release pull request opened against the wrong branch. A HEAD that is not on a
// branch has no default to record, and a failure to record one is only logged, which
// leaves resolveDefaultBranch's fallback where it was.
func recordRemoteDefaultBranch(repo *git.Repository) {
	head, err := repo.Storer.Reference(plumbing.HEAD)
	if err != nil || head.Type() != plumbing.SymbolicReference || !head.Target().IsBranch() {
		return
	}

	remoteHead := plumbing.NewSymbolicReference(
		plumbing.NewRemoteHEADReferenceName("origin"),
		plumbing.NewRemoteReferenceName("origin", head.Target().Short()),
	)
	if err = repo.Storer.SetReference(remoteHead); err != nil {
		logger.Warnf("Could not record the default branch of the remote: %v", err)
	}
}

// fallbackDefaultBranch is the branch assumed when the remote does not say which is its
// default.
const fallbackDefaultBranch = "main"

// resolveDefaultBranch determines the default branch from the remote origin/HEAD reference.
func resolveDefaultBranch(repo *git.Repository) string {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), true)
	if err != nil {
		return fallbackDefaultBranch
	}
	branch := ref.Name().Short()
	branch = strings.TrimPrefix(branch, "origin/")
	if branch == "" {
		return fallbackDefaultBranch
	}
	return branch
}

// buildGitforgeRepo constructs a gitforge Repository entity from a remote URL. The
// repository ID is the one discovery reported, or empty when the repository was not
// discovered through the provider API, in which case the API is addressed by name.
func buildGitforgeRepo(remoteURL, defaultBranch, repositoryID string) globalEntities.Repository {
	parsed, err := gitInfra.ParseRemoteURL(remoteURL)
	if err != nil {
		logger.WithField("remoteURL", remoteURL).Warn("could not parse organization or repository name from remote URL")
		return globalEntities.Repository{
			ID:            repositoryID,
			DefaultBranch: "refs/heads/" + defaultBranch,
			RemoteURL:     remoteURL,
		}
	}

	return globalEntities.Repository{
		ID:            repositoryID,
		Name:          parsed.RepoName,
		Organization:  parsed.Organization,
		Project:       parsed.Project,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     remoteURL,
	}
}

// ---- Changelog I/O wrappers ----

// readChangelogLines is the single boundary through which the changelog is read.
//
// It returns the file's own lines for an ordinary project. For a project using chlog
// (https://github.com/luizjhonata/chlog), whose pending changes live as YAML fragments
// under .changes/unreleased/ rather than in the file, it also splices those fragments
// into the [Unreleased] section. Everything downstream therefore sees plain Keep a
// Changelog content and needs no awareness of chlog -- including the emptiness check
// that decides whether a release is needed at all, the SemVer calculation, and fork mode.
//
// Entries already written by hand into [Unreleased] are kept: during a migration to
// chlog both sources can hold real work, and dropping either would lose a release note.
//
// The whole document is then unwrapped and the section normalised, which is what applies
// the changelog rules -- one physical line per entry, repaired headings, one spelling of the
// breaking-change marker, deduplication, verb-based reclassification, ordering -- to
// fragments as well as to hand-written entries. Doing it at the boundary rather than inside
// one release path is deliberate: fragments are the case that needs the rules most, since
// each one is written alone and nobody ever sees them side by side, and fork versioning
// rewrites the section without consulting the SemVer pipeline that used to be the only place
// the rules lived. Unwrapping specifically has to happen here and not only inside that
// pipeline, because fork versioning does not call it either.
func readChangelogLines(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
) ([]string, error) {
	lines, err := support.ReadLines(changelogPath)
	if err != nil {
		return nil, err
	}

	fragments, config, err := collectChlogFragments(globalConfig, projectConfig)
	if err != nil {
		return nil, err
	}
	if len(fragments) > 0 {
		lines = MergeChlogIntoUnreleased(lines, RenderChlogFragments(fragments, config))
	}

	return entities.NormalizeUnreleasedSection(entities.UnwrapChangelogEntries(lines)), nil
}

// collectChlogFragments reads the pending chlog fragments of a project, if it uses chlog
// and detection is enabled. It returns no fragments for every other project.
func collectChlogFragments(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) ([]ChlogFragment, *ChlogConfig, error) {
	if projectConfig == nil || !entities.ChlogEnabled(globalConfig, projectConfig) {
		return nil, nil, nil
	}

	config, usesChlog, err := DetectChlog(projectConfig.Path)
	if err != nil {
		return nil, nil, err
	}
	if !usesChlog {
		return nil, nil, nil
	}

	fragments, err := ReadChlogFragments(projectConfig.Path, config)
	if err != nil {
		return nil, nil, err
	}
	if len(fragments) == 0 {
		return nil, config, nil
	}

	// Debug, not Info: the changelog is read several times per run, and ProcessRepo
	// already announces the detection once via logChlogDetection.
	logger.Debugf("Read %d chlog fragment(s) from %s",
		len(fragments), config.UnreleasedPath(projectConfig.Path))

	return fragments, config, nil
}

// logChlogDetection announces once per run that a project keeps its pending changes as
// chlog fragments. Detection problems are only warned about here: the same read happens
// again on the path that actually needs the fragments, where the error is returned.
func logChlogDetection(ctx *RepoContext) {
	fragments, config, err := collectChlogFragments(ctx.GlobalConfig, ctx.ProjectConfig)
	if err != nil {
		logger.Warnf("Could not read the chlog fragments in %s: %v", ctx.ProjectConfig.Path, err)
		return
	}
	if len(fragments) == 0 {
		return
	}

	logger.Infof("Detected a chlog changelog with %d pending fragment(s) in %s",
		len(fragments), config.UnreleasedPath(ctx.ProjectConfig.Path))
}

// updateChangelogFile reads the changelog, processes it with the SemVer
// pipeline, and writes it back. Production callers should prefer
// updateChangelogFileString to honor the project's versioning mode.
//
// releaseNotes are Keep a Changelog lines ("### Fixed", "", "- fixed ...") that the
// release itself contributes, such as the announcement of a module path rewrite. They are
// merged into [Unreleased] here rather than at the readChangelogLines boundary, because
// that boundary also decides whether there is anything to release at all, and an entry
// AutoBump wrote must never be the reason a release happens.
func updateChangelogFile(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
	releaseNotes ...string,
) (*semver.Version, error) {
	lines, err := readChangelogLines(globalConfig, projectConfig, changelogPath)
	if err != nil {
		return nil, err
	}
	lines = MergeChlogIntoUnreleased(lines, releaseNotes)

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

// updateChangelogFileString rewrites the changelog using the versioning mode
// resolved from the global/project configuration and returns the next version
// as a plain string. Fork modes preserve the upstream X.Y.Z and only
// increment the trailing fork digit; SemVer mode delegates to gitforge.
// releaseNotes reach only the SemVer pipeline; see updateChangelogFile.
func updateChangelogFileString(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
	releaseNotes ...string,
) (string, error) {
	mode := entities.ResolveVersioning(globalConfig, projectConfig)
	if IsForkVersioning(mode) {
		lines, err := readChangelogLines(globalConfig, projectConfig, changelogPath)
		if err != nil {
			return "", err
		}
		nextVersion, newContent, err := processForkChangelog(lines, mode)
		if err != nil {
			return "", err
		}
		if err = support.WriteLines(changelogPath, newContent); err != nil {
			return "", err
		}
		return nextVersion, nil
	}

	version, err := updateChangelogFile(globalConfig, projectConfig, changelogPath, releaseNotes...)
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

// getNextVersion reads the changelog and calculates the next SemVer version
// without writing anything back.
func getNextVersion(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
) (*semver.Version, error) {
	lines, err := readChangelogLines(globalConfig, projectConfig, changelogPath)
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

// getNextVersionString reads the changelog and calculates the next version
// using the versioning mode resolved from the global/project configuration.
// The result is returned as a plain string so the caller does not have to
// know which mode produced it.
func getNextVersionString(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
) (string, error) {
	mode := entities.ResolveVersioning(globalConfig, projectConfig)
	if IsForkVersioning(mode) {
		lines, err := readChangelogLines(globalConfig, projectConfig, changelogPath)
		if err != nil {
			return "", err
		}
		current, err := FindLatestForkVersion(lines, mode)
		currentString := ""
		switch {
		case err == nil:
			currentString = current.String()
		case errors.Is(err, ErrNoForkVersionFound):
			// No prior fork release yet; NextForkVersion seeds the initial value.
		default:
			return "", err
		}
		return NextForkVersion(currentString, mode)
	}

	version, err := getNextVersion(globalConfig, projectConfig, changelogPath)
	if err != nil {
		return "", err
	}
	return version.String(), nil
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

	// If no version files are configured or none matched for this language, just continue.
	// This is an expected case (e.g. Go projects without Swagger annotations), so it is
	// logged at info level instead of warning.
	if len(versionFiles) == 0 {
		logger.Infof(
			"No version files applicable for language '%s', only changelog will be updated",
			projectConfig.Language,
		)
		return nil
	}

	oneVersionFileExists := false
	for _, versionFile := range versionFiles {
		exists, processErr := processVersionFile(versionFile, projectConfig.NewVersion)
		if processErr != nil {
			return processErr
		}
		if exists {
			oneVersionFileExists = true
		}
	}

	// If no version files exist, just warn and continue (don't fail)
	if !oneVersionFileExists {
		logger.Warnf("No version files found for language '%s', only changelog will be updated", projectConfig.Language)
	}

	return nil
}

// processVersionFile updates a single version file, replacing only the first match of each pattern.
func processVersionFile(versionFile entities.VersionFile, newVersion string) (bool, error) {
	info, err := os.Stat(versionFile.Path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Version file %s does not exist", versionFile.Path)
			return false, nil
		}
		return false, fmt.Errorf("failed to stat version file %s: %w", versionFile.Path, err)
	}
	logger.Infof("Updating version file %s", versionFile.Path)

	content, err := os.ReadFile(versionFile.Path)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", versionFile.Path, err)
	}

	// Rewrite exceptions (Maven's <parent> block, Flutter's build number) are
	// resolved by base name through versionFileHooks; see version_file_hooks.go.
	updatedContent, err := applyVersionPatterns(
		string(content),
		versionFile.Patterns,
		newVersion,
		versionFileHooks[filepath.Base(versionFile.Path)],
	)
	if err != nil {
		return false, err
	}

	//nolint:gosec // G703 false positive: path originates from filepath.Glob and is validated by os.Stat above
	err = os.WriteFile(versionFile.Path, []byte(updatedContent), info.Mode())
	if err != nil {
		return false, fmt.Errorf("failed to write to file %s: %w", versionFile.Path, err)
	}

	return true, nil
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
		// Only Python reads its project name from its own manifest. Every other language --
		// Go, Helm, TypeScript, Terraform alike -- uses the repository's name, which is the
		// normal case and not worth an info line on every version-file lookup: it read as if
		// the language were unsupported while its version files were being bumped.
		logger.Debugf(
			"Language '%s' has no project name reader, using '%s' for '{project_name}'",
			projectConfig.Language, projectName,
		)
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
			hasMatch, matchErr := versionFileHasPatternMatch(match, versionFile.Patterns)
			if matchErr != nil {
				return nil, matchErr
			}
			if !hasMatch {
				logger.Debugf("Skipping %s: no version pattern matches its content", match)
				continue
			}
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

// versionFileHasPatternMatch reports whether the file content matches at least one of
// the configured version patterns. A globbed file that carries no version marker
// (e.g. a main.go without a Swagger "@version" annotation) is not a version file for
// the project, so it must not be rewritten in place nor reported as updated.
func versionFileHasPatternMatch(path string, patterns []string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("failed to read version file %s: %w", path, err)
	}
	for _, pattern := range patterns {
		re, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			return false, wrapInvalidVersionPattern(pattern, compileErr)
		}
		if re.Match(content) {
			return true, nil
		}
	}
	return false, nil
}

// getLanguageInterface returns the appropriate Language interface for the project.
func getLanguageInterface(projectConfig entities.ProjectConfig, languageInterface *entities.Language) {
	if projectConfig.Language == "python" {
		*languageInterface = &python.Python{ProjectConfig: projectConfig}
	}
}
