package commands

import "github.com/rios0rios0/autobump/internal/domain/entities"

// FilterRepositories exports filterRepositories for testing.
func FilterRepositories(
	repos []entities.Repository,
	globalConfig *entities.GlobalConfig,
) []entities.Repository {
	return filterRepositories(repos, globalConfig)
}

// CommitChanges exports the commitChanges function for testing.
var CommitChanges = commitChanges //nolint:gochecknoglobals // test export

// ResolveConfigKey exports resolveConfigKey for testing.
var ResolveConfigKey = resolveConfigKey //nolint:gochecknoglobals // test export

// BuildGitforgeRepo exports buildGitforgeRepo for testing.
var BuildGitforgeRepo = buildGitforgeRepo //nolint:gochecknoglobals // test export

// ResolveToken exports resolveToken for testing.
var ResolveToken = resolveToken //nolint:gochecknoglobals // test export

// CollectTokens exports collectTokens for testing.
var CollectTokens = collectTokens //nolint:gochecknoglobals // test export

// GetForgeProvider exports getForgeProvider for testing.
var GetForgeProvider = getForgeProvider //nolint:gochecknoglobals // test export

// RepoToProjectConfig exports repoToProjectConfig for testing.
var RepoToProjectConfig = repoToProjectConfig //nolint:gochecknoglobals // test export

// LoadProjectConfigOverrides exports loadProjectConfigOverrides for testing.
var LoadProjectConfigOverrides = loadProjectConfigOverrides //nolint:gochecknoglobals // test export

// CollectSSHAuthMethods exports collectSSHAuthMethods for testing.
var CollectSSHAuthMethods = collectSSHAuthMethods //nolint:gochecknoglobals // test export

// DetectSSHAgentSockets exports detectSSHAgentSockets for testing.
var DetectSSHAgentSockets = detectSSHAgentSockets //nolint:gochecknoglobals // test export

// AddCurrentVersion exports addCurrentVersion for testing.
var AddCurrentVersion = addCurrentVersion //nolint:gochecknoglobals // test export

// SetupChangelog exports setupChangelog for testing.
var SetupChangelog = setupChangelog //nolint:gochecknoglobals // test export

// UpdateVersion exports updateVersion for testing.
var UpdateVersion = updateVersion //nolint:gochecknoglobals // test export

// GeneratePRDescription exports generatePRDescription for testing.
var GeneratePRDescription = generatePRDescription //nolint:gochecknoglobals // test export

// ShouldBumpProject exports shouldBumpProject for testing.
var ShouldBumpProject = shouldBumpProject //nolint:gochecknoglobals // test export

// EnsureProjectLanguage exports ensureProjectLanguage for testing.
var EnsureProjectLanguage = ensureProjectLanguage //nolint:gochecknoglobals // test export

// UpdateChangelogFile exports updateChangelogFile for testing.
var UpdateChangelogFile = updateChangelogFile //nolint:gochecknoglobals // test export

// GetNextVersion exports getNextVersion for testing.
var GetNextVersion = getNextVersion //nolint:gochecknoglobals // test export

// CreateChangelogIfNotExists exports createChangelogIfNotExists for testing.
var CreateChangelogIfNotExists = createChangelogIfNotExists //nolint:gochecknoglobals // test export

// SetupRepo exports setupRepo for testing.
var SetupRepo = setupRepo //nolint:gochecknoglobals // test export

// CreateBumpBranch exports createBumpBranch for testing.
var CreateBumpBranch = createBumpBranch //nolint:gochecknoglobals // test export

// AddFilesToWorktree exports addFilesToWorktree for testing.
var AddFilesToWorktree = addFilesToWorktree //nolint:gochecknoglobals // test export

// CheckoutToMainBranch exports checkoutToMainBranch for testing.
var CheckoutToMainBranch = checkoutToMainBranch //nolint:gochecknoglobals // test export

// ResolveDefaultBranch exports resolveDefaultBranch for testing.
var ResolveDefaultBranch = resolveDefaultBranch //nolint:gochecknoglobals // test export

// UpdateChangelogAndVersionFiles exports updateChangelogAndVersionFiles for testing.
var UpdateChangelogAndVersionFiles = updateChangelogAndVersionFiles //nolint:gochecknoglobals // test export

// HostKeyCallback exports hostKeyCallback for testing.
var HostKeyCallback = hostKeyCallback //nolint:gochecknoglobals // test export

// IterateProjects is already exported (public), no need to re-export.

// ProcessVersionFile exports processVersionFile for testing.
var ProcessVersionFile = processVersionFile //nolint:gochecknoglobals // test export

// GetVersionFiles exports getVersionFiles for testing.
var GetVersionFiles = getVersionFiles //nolint:gochecknoglobals // test export

// CloneRepo exports cloneRepo for testing.
var CloneRepo = cloneRepo //nolint:gochecknoglobals // test export

// CloneRepoIfNeeded exports cloneRepoIfNeeded for testing.
var CloneRepoIfNeeded = cloneRepoIfNeeded //nolint:gochecknoglobals // test export

// PushChanges exports pushChanges for testing.
var PushChanges = pushChanges //nolint:gochecknoglobals // test export

// CommitAndPushChanges exports commitAndPushChanges for testing.
var CommitAndPushChanges = commitAndPushChanges //nolint:gochecknoglobals // test export

// CreatePullRequest exports createPullRequest for testing.
var CreatePullRequest = createPullRequest //nolint:gochecknoglobals // test export

// CreateAndCheckoutPullRequest exports createAndCheckoutPullRequest for testing.
var CreateAndCheckoutPullRequest = createAndCheckoutPullRequest //nolint:gochecknoglobals // test export

// HandleExistingBranchWithoutPR exports handleExistingBranchWithoutPR for testing.
var HandleExistingBranchWithoutPR = handleExistingBranchWithoutPR //nolint:gochecknoglobals // test export

// CheckPullRequestExists exports checkPullRequestExists for testing.
var CheckPullRequestExists = checkPullRequestExists //nolint:gochecknoglobals // test export

// CollectAuthMethods exports collectAuthMethods for testing.
var CollectAuthMethods = collectAuthMethods //nolint:gochecknoglobals // test export

// SSHAgentAuthFromSocket exports sshAgentAuthFromSocket for testing.
var SSHAgentAuthFromSocket = sshAgentAuthFromSocket //nolint:gochecknoglobals // test export

// CommitAndPushInitialChangelog exports commitAndPushInitialChangelog for testing.
var CommitAndPushInitialChangelog = commitAndPushInitialChangelog //nolint:gochecknoglobals // test export

// DetectBySpecialPatterns exports detectBySpecialPatterns for testing.
var DetectBySpecialPatterns = detectBySpecialPatterns //nolint:gochecknoglobals // test export

// DetectByExtensions exports detectByExtensions for testing.
var DetectByExtensions = detectByExtensions //nolint:gochecknoglobals // test export

// GetLanguageInterface exports getLanguageInterface for testing.
var GetLanguageInterface = getLanguageInterface //nolint:gochecknoglobals // test export
