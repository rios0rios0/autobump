package commands

import (
	"net"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// FilterRepositories exports filterRepositories for testing.
func FilterRepositories(
	repos []entities.Repository,
	globalConfig *entities.GlobalConfig,
) []entities.Repository {
	return filterRepositories(repos, globalConfig)
}

// RemoveStaleBumpBranch exports removeStaleBumpBranch for testing, taking its side effects
// as plain functions; a nil closePullRequest stands for a run without a provider.
func RemoveStaleBumpBranch(
	branch string,
	deleteRemote, deleteLocal func(branch string) error,
	closePullRequest func(branch string) (bool, error),
) {
	removeStaleBumpBranch(branch, staleBranchActions{
		deleteRemote:     deleteRemote,
		deleteLocal:      deleteLocal,
		closePullRequest: closePullRequest,
	})
}

// CommitChanges exports the commitChanges function for testing.
var CommitChanges = commitChanges //nolint:gochecknoglobals // test export

// ResolveSSHProgram exports resolveSSHProgram for testing.
var ResolveSSHProgram = resolveSSHProgram //nolint:gochecknoglobals // test export

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

// UpdateChangelogFileString exports updateChangelogFileString for testing.
var UpdateChangelogFileString = updateChangelogFileString //nolint:gochecknoglobals // test export

// GetNextVersion exports getNextVersion for testing.
var GetNextVersion = getNextVersion //nolint:gochecknoglobals // test export

// GetNextVersionString exports getNextVersionString for testing.
var GetNextVersionString = getNextVersionString //nolint:gochecknoglobals // test export

// ProcessForkChangelog exports processForkChangelog for testing.
var ProcessForkChangelog = processForkChangelog //nolint:gochecknoglobals // test export

// RewriteUnreleasedAsForkRelease exports rewriteUnreleasedAsForkRelease for testing.
var RewriteUnreleasedAsForkRelease = rewriteUnreleasedAsForkRelease //nolint:gochecknoglobals // test export

// CreateChangelogIfNotExists exports createChangelogIfNotExists for testing.
var CreateChangelogIfNotExists = createChangelogIfNotExists //nolint:gochecknoglobals // test export

// ReadChangelogLines exports readChangelogLines for testing.
var ReadChangelogLines = readChangelogLines //nolint:gochecknoglobals // test export

// ResolveChangelogPath exports resolveChangelogPath for testing.
var ResolveChangelogPath = resolveChangelogPath //nolint:gochecknoglobals // test export

// ChlogSectionForKind exports chlogSectionForKind for testing.
var ChlogSectionForKind = chlogSectionForKind //nolint:gochecknoglobals // test export

// ConsumeChlogFragments exports consumeChlogFragments for testing.
var ConsumeChlogFragments = consumeChlogFragments //nolint:gochecknoglobals // test export

// SetupRepo exports setupRepo for testing.
var SetupRepo = setupRepo //nolint:gochecknoglobals // test export

// CreateBumpBranch exports createBumpBranch for testing.
var CreateBumpBranch = createBumpBranch //nolint:gochecknoglobals // test export

// AddFilesToWorktree exports addFilesToWorktree for testing.
var AddFilesToWorktree = addFilesToWorktree //nolint:gochecknoglobals // test export

// CheckoutToMainBranch exports checkoutToMainBranch for testing.
var CheckoutToMainBranch = checkoutToMainBranch //nolint:gochecknoglobals // test export

// RecordRemoteDefaultBranch exports recordRemoteDefaultBranch for testing.
var RecordRemoteDefaultBranch = recordRemoteDefaultBranch //nolint:gochecknoglobals // test export

// ResolveDefaultBranch exports resolveDefaultBranch for testing.
var ResolveDefaultBranch = resolveDefaultBranch //nolint:gochecknoglobals // test export

// UpdateChangelogAndVersionFiles exports updateChangelogAndVersionFiles for testing.
var UpdateChangelogAndVersionFiles = updateChangelogAndVersionFiles //nolint:gochecknoglobals // test export

// HostKeyCallback exports hostKeyCallback for testing.
var HostKeyCallback = hostKeyCallback //nolint:gochecknoglobals // test export

// BuildKnownHostsDB exports buildKnownHostsDB for testing.
var BuildKnownHostsDB = buildKnownHostsDB //nolint:gochecknoglobals // test export

// SSHConfigSource exports sshConfigSource for testing. An alias rather than a definition, so
// a test can name the files a resolution reads directly instead of arranging a home for them.
type SSHConfigSource = sshConfigSource

// SSHHostSettings exports sshHostSettings for testing.
type SSHHostSettings = sshHostSettings

// ResolveSSHHost loads the given ssh_config files, as the SSH transport loads the ones it
// discovers, and resolves alias against them.
func ResolveSSHHost(sources []SSHConfigSource, automountRoot, alias, remoteUser string) SSHHostSettings {
	return loadSSHConfigFiles(sources, automountRoot).resolve(alias, remoteUser)
}

// SSHHostTarget exports sshHostSettings.target for testing.
func SSHHostTarget(settings SSHHostSettings, endpoint *transport.Endpoint) *transport.Endpoint {
	return settings.target(endpoint)
}

// DiscoverSSHConfigSources exports discoverSSHConfigSources for testing.
var DiscoverSSHConfigSources = discoverSSHConfigSources //nolint:gochecknoglobals // test export

// WindowsSSHConfigSourcesUnder exports windowsSSHConfigSourcesUnder for testing.
var WindowsSSHConfigSourcesUnder = windowsSSHConfigSourcesUnder //nolint:gochecknoglobals // test export

// LoadSSHIdentity exports loadSSHIdentity for testing, flattening the unexported identity
// into its public key and whether the file yielded a key usable without the agent.
func LoadSSHIdentity(path string) (ssh.PublicKey, bool, error) {
	identity, err := loadSSHIdentity(path)
	return identity.publicKey, identity.signer != nil, err
}

// SelectSigners loads the identities the IdentityFile paths name and selects the keys to
// offer for them, as the SSH transport does for an alias.
func SelectSigners(agentSigners []ssh.Signer, identityFiles []string, identitiesOnly bool) []ssh.Signer {
	return selectSigners(agentSigners, loadSSHIdentities(identityFiles), identitiesOnly)
}

// GitSSHCommandInEffect exports gitSSHCommandInEffect for testing.
var GitSSHCommandInEffect = gitSSHCommandInEffect //nolint:gochecknoglobals // test export

// RunsWindowsSSH exports runsWindowsSSH for testing.
var RunsWindowsSSH = runsWindowsSSH //nolint:gochecknoglobals // test export

// WindowsProfilesOnPath exports windowsProfilesOnPath for testing.
var WindowsProfilesOnPath = windowsProfilesOnPath //nolint:gochecknoglobals // test export

// CurrentWindowsProfileSources exports currentWindowsProfileSources for testing.
var CurrentWindowsProfileSources = currentWindowsProfileSources //nolint:gochecknoglobals // test export

// LoadSSHConfigFiles loads the given ssh_config files and reports how many were usable, so a
// test can tell a skipped file from a loaded one.
func LoadSSHConfigFiles(sources []SSHConfigSource) int {
	return len(loadSSHConfigFiles(sources, "").files)
}

// NewSSHTransportWith builds the SSH transport over next with fixed ssh_config and
// known_hosts files in place of the ones the host offers.
func NewSSHTransportWith(
	next transport.Transport,
	sources []SSHConfigSource,
	automountRoot string,
	knownHostsFiles []string,
) transport.Transport {
	config := loadSSHConfigFiles(sources, automountRoot)
	hostKeys := buildKnownHostsDB(knownHostsFiles)
	return newSSHTransport(
		next,
		func() *sshConfigFiles { return config },
		func() *knownhosts.HostKeyDB { return hostKeys },
	)
}

// DescribeUnresolvedSSHHost exports describeUnresolvedSSHHost for testing.
var DescribeUnresolvedSSHHost = describeUnresolvedSSHHost //nolint:gochecknoglobals // test export

// DiscoverKnownHostsFiles exports discoverKnownHostsFiles for testing.
var DiscoverKnownHostsFiles = discoverKnownHostsFiles //nolint:gochecknoglobals // test export

// WindowsKnownHostsFilesUnder exports windowsKnownHostsFilesUnder for testing.
var WindowsKnownHostsFilesUnder = windowsKnownHostsFilesUnder //nolint:gochecknoglobals // test export

// ParseWSLAutomountRoot exports parseWSLAutomountRoot for testing.
var ParseWSLAutomountRoot = parseWSLAutomountRoot //nolint:gochecknoglobals // test export

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

// ListSSHAgentSigners exports listSSHAgentSigners for testing, taking the dialer as a plain
// function so a test can hand it connections that fail on cue.
func ListSSHAgentSigners(dial func() (net.Conn, error)) ([]ssh.Signer, error) {
	return listSSHAgentSigners(dial)
}

// CommitInitialChangelog exports commitInitialChangelog for testing.
var CommitInitialChangelog = commitInitialChangelog //nolint:gochecknoglobals // test export

// InitialChangelogBranch exports initialChangelogBranch for testing.
const InitialChangelogBranch = initialChangelogBranch

// DetectBySpecialPatterns exports detectBySpecialPatterns for testing.
var DetectBySpecialPatterns = detectBySpecialPatterns //nolint:gochecknoglobals // test export

// DetectByExtensions exports detectByExtensions for testing.
var DetectByExtensions = detectByExtensions //nolint:gochecknoglobals // test export

// GetLanguageInterface exports getLanguageInterface for testing.
var GetLanguageInterface = getLanguageInterface //nolint:gochecknoglobals // test export

// FilterStaleBumpBranches exports filterStaleBumpBranches for testing.
var FilterStaleBumpBranches = filterStaleBumpBranches //nolint:gochecknoglobals // test export

// RunRefreshCommands exports runRefreshCommands for testing.
var RunRefreshCommands = runRefreshCommands //nolint:gochecknoglobals // test export

// ResolveRefreshedFiles exports resolveRefreshedFiles for testing.
var ResolveRefreshedFiles = resolveRefreshedFiles //nolint:gochecknoglobals // test export

// RunRefreshRecipe exports runRefreshRecipe for testing, with a recipe built by the test
// rather than read from configuration -- which is the point of the redesign: production
// recipes are compile-time constants, so the bounds tests need a synthetic one.
func RunRefreshRecipe(
	projectPath, toolingDir string, run, files, env []string,
	timeout time.Duration, waitDelay time.Duration,
) error {
	return runRefreshRecipe(
		projectPath,
		toolingDir,
		refreshRecipe{Manager: "test", Run: run, Files: files, Env: env},
		timeout, waitDelay,
	)
}

// RefreshEnv exports refreshEnv for testing.
var RefreshEnv = refreshEnv //nolint:gochecknoglobals // test export

// NewRepositoryWorkspace exports newRepositoryWorkspace for testing.
var NewRepositoryWorkspace = newRepositoryWorkspace //nolint:gochecknoglobals // test export

// RemoveWorkspace exports removeWorkspace for testing.
var RemoveWorkspace = removeWorkspace //nolint:gochecknoglobals // test export

// WorkspaceRepoPath exports workspaceRepoPath for testing.
var WorkspaceRepoPath = workspaceRepoPath //nolint:gochecknoglobals // test export

// WorkspaceToolingPath exports workspaceToolingPath for testing.
var WorkspaceToolingPath = workspaceToolingPath //nolint:gochecknoglobals // test export

// IsWorkspaceName exports isWorkspaceName for testing.
var IsWorkspaceName = isWorkspaceName //nolint:gochecknoglobals // test export

// DetectNodeRecipe exports detectNodeRecipe for testing, flattening the unexported recipe
// into the manager name and argv a test can assert on.
func DetectNodeRecipe(projectPath string) (string, []string, []string, bool) {
	recipe, found := detectNodeRecipe(projectPath)
	return recipe.Manager, recipe.Run, recipe.Files, found
}

// RefreshRecipeEnv exports the environment a detected recipe adds to the child process.
func RefreshRecipeEnv(projectPath string) []string {
	recipe, _ := detectNodeRecipe(projectPath)
	return recipe.Env
}

// RefreshRecipeLanguages exports the languages that have a refresh recipe.
func RefreshRecipeLanguages() []string {
	languages := make([]string, 0, len(refreshRecipes))
	for language := range refreshRecipes {
		languages = append(languages, language)
	}
	return languages
}

// SSHEnvironment exports sshEnvironment for testing. An alias rather than a definition, so
// a test can build the host facts directly instead of arranging a host that exhibits them.
type SSHEnvironment = sshEnvironment

// DescribeMissingSSHCredential exports describeMissingSSHCredential for testing.
var DescribeMissingSSHCredential = describeMissingSSHCredential //nolint:gochecknoglobals // test export

// IsSSHRemote exports isSSHRemote for testing.
var IsSSHRemote = isSSHRemote //nolint:gochecknoglobals // test export

// OriginRemoteURL exports originRemoteURL for testing.
var OriginRemoteURL = originRemoteURL //nolint:gochecknoglobals // test export

// GitSSHCommand exports gitSSHCommand for testing.
var GitSSHCommand = gitSSHCommand //nolint:gochecknoglobals // test export

// KernelIsWSL exports kernelIsWSL for testing.
var KernelIsWSL = kernelIsWSL //nolint:gochecknoglobals // test export

// ExplainSSHPushFailure exports explainSSHPushFailure for testing.
var ExplainSSHPushFailure = explainSSHPushFailure //nolint:gochecknoglobals // test export

// DetectSSHEnvironment exports detectSSHEnvironment for testing.
var DetectSSHEnvironment = detectSSHEnvironment //nolint:gochecknoglobals // test export

// GoModulePathForMajor exports goModulePathForMajor for testing.
var GoModulePathForMajor = goModulePathForMajor //nolint:gochecknoglobals // test export

// RewriteGoImports exports rewriteGoImports for testing.
var RewriteGoImports = rewriteGoImports //nolint:gochecknoglobals // test export

// RewriteModuleDirective exports rewriteModuleDirective for testing.
var RewriteModuleDirective = rewriteModuleDirective //nolint:gochecknoglobals // test export

// RewriteModuleReferences exports rewriteModuleReferences for testing.
var RewriteModuleReferences = rewriteModuleReferences //nolint:gochecknoglobals // test export

// PrepareGoModulePathChange exports prepareGoModulePathChange for testing.
var PrepareGoModulePathChange = prepareGoModulePathChange //nolint:gochecknoglobals // test export

// ApplyGoModulePathChange exports GoModulePathChange.apply for testing.
func ApplyGoModulePathChange(change *GoModulePathChange) ([]string, error) {
	return change.apply()
}
