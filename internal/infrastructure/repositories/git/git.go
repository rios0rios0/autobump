package git

import (
	"context"
	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	domainRepos "github.com/rios0rios0/autobump/internal/domain/repositories"
	forgeEntities "github.com/rios0rios0/gitforge/domain/entities"
	forgeRepos "github.com/rios0rios0/gitforge/domain/repositories"
	forgeGit "github.com/rios0rios0/gitforge/infrastructure/git"
)

// AdapterFinder provides adapter lookup capabilities without circular dependencies.
type AdapterFinder interface {
	GetAdapterByServiceType(serviceType entities.ServiceType) domainRepos.GitServiceAdapter
	GetAdapterByURL(url string) domainRepos.GitServiceAdapter
}

const (
	DefaultGitTag               = forgeGit.DefaultGitTag
	MaxAcceptableInitialCommits = forgeGit.MaxAcceptableInitialCommits
)

var (
	ErrNoAuthMethodFound  = forgeGit.ErrNoAuthMethodFound
	ErrAuthNotImplemented = forgeGit.ErrAuthNotImplemented
	ErrNoRemoteURL        = forgeGit.ErrNoRemoteURL
	ErrNoTagsFound        = forgeGit.ErrNoTagsFound
)

// SetAdapterFinder sets the adapter finder used by git utilities.
// It initializes the bridge to gitforge's adapter finder with nil config.
// For full config support, use SetupAdapterFinderBridge instead.
func SetAdapterFinder(finder AdapterFinder) {
	SetupAdapterFinderBridge(finder, nil, nil)
}

// GetGlobalGitConfig reads the global git configuration file.
func GetGlobalGitConfig() (*config.Config, error) {
	return forgeGit.GetGlobalGitConfig()
}

// GetOptionFromConfig gets a Git option from local and global Git config.
func GetOptionFromConfig(cfg, globalCfg *config.Config, section string, option string) string {
	return forgeGit.GetOptionFromConfig(cfg, globalCfg, section, option)
}

// OpenRepo opens a git repository at the given path.
func OpenRepo(projectPath string) (*git.Repository, error) {
	return forgeGit.OpenRepo(projectPath)
}

// CheckBranchExists checks if a given Git branch exists (local or remote).
func CheckBranchExists(repo *git.Repository, branchName string) (bool, error) {
	return forgeGit.CheckBranchExists(repo, branchName)
}

// CreateAndSwitchBranch creates a new branch and switches to it.
func CreateAndSwitchBranch(
	repo *git.Repository,
	workTree *git.Worktree,
	branchName string,
	hash plumbing.Hash,
) error {
	return forgeGit.CreateAndSwitchBranch(repo, workTree, branchName, hash)
}

// CheckoutBranch switches to the given branch.
func CheckoutBranch(w *git.Worktree, branchName string) error {
	return forgeGit.CheckoutBranch(w, branchName)
}

// CommitChanges commits the changes in the given worktree.
func CommitChanges(
	workTree *git.Worktree,
	commitMessage string,
	signKey *openpgp.Entity,
	name string,
	email string,
) (plumbing.Hash, error) {
	return forgeGit.CommitChanges(workTree, commitMessage, signKey, name, email)
}

// PushChangesSSH pushes the changes to the remote repository over SSH.
func PushChangesSSH(repo *git.Repository, refSpec config.RefSpec) error {
	return forgeGit.PushChangesSSH(repo, refSpec)
}

// GetRemoteRepoURL returns the URL of the remote repository.
func GetRemoteRepoURL(repo *git.Repository) (string, error) {
	return forgeGit.GetRemoteRepoURL(repo)
}

// GetAmountCommits returns the number of commits in the repository.
func GetAmountCommits(repo *git.Repository) (int, error) {
	return forgeGit.GetAmountCommits(repo)
}

// GetLatestTag finds the latest tag in the Git history.
func GetLatestTag(repo *git.Repository) (*entities.LatestTag, error) {
	return forgeGit.GetLatestTag(repo)
}

// GetRemoteServiceType returns the type of the remote service.
func GetRemoteServiceType(repo *git.Repository) (entities.ServiceType, error) {
	return forgeGit.GetRemoteServiceType(repo)
}

// GetServiceTypeByURL returns the service type by URL.
func GetServiceTypeByURL(remoteURL string) entities.ServiceType {
	return forgeGit.GetServiceTypeByURL(remoteURL)
}

// PushChangesHTTPS pushes the changes to the remote repository over HTTPS.
func PushChangesHTTPS(
	repo *git.Repository,
	repoCfg *config.Config,
	refSpec config.RefSpec,
	_ *entities.GlobalConfig,
	_ *entities.ProjectConfig,
) error {
	return forgeGit.PushChangesHTTPS(repo, repoCfg.User.Name, refSpec)
}

// GetAuthMethods returns the authentication methods for the given service type.
func GetAuthMethods(
	service entities.ServiceType,
	username string,
	_ *entities.GlobalConfig,
	_ *entities.ProjectConfig,
) ([]transport.AuthMethod, error) {
	return forgeGit.GetAuthMethods(service, username)
}

// SetupAdapterFinderBridge creates a bridge from autobump's AdapterFinder to gitforge's.
func SetupAdapterFinderBridge(
	finder AdapterFinder,
	gc *entities.GlobalConfig,
	pc *entities.ProjectConfig,
) {
	bridge := &adapterFinderBridge{
		autobumpFinder: finder,
		globalConfig:   gc,
		projectConfig:  pc,
	}
	forgeGit.SetAdapterFinder(bridge)
}

// adapterFinderBridge bridges autobump's AdapterFinder to gitforge's AdapterFinder.
type adapterFinderBridge struct {
	autobumpFinder AdapterFinder
	globalConfig   *entities.GlobalConfig
	projectConfig  *entities.ProjectConfig
}

func (b *adapterFinderBridge) GetAdapterByServiceType(
	serviceType entities.ServiceType,
) forgeRepos.LocalGitAuthProvider {
	adapter := b.autobumpFinder.GetAdapterByServiceType(serviceType)
	if adapter == nil {
		return nil
	}
	return &gitServiceAdapterWrapper{
		adapter:       adapter,
		globalConfig:  b.globalConfig,
		projectConfig: b.projectConfig,
	}
}

func (b *adapterFinderBridge) GetAdapterByURL(url string) forgeRepos.LocalGitAuthProvider {
	adapter := b.autobumpFinder.GetAdapterByURL(url)
	if adapter == nil {
		return nil
	}
	return &gitServiceAdapterWrapper{
		adapter:       adapter,
		globalConfig:  b.globalConfig,
		projectConfig: b.projectConfig,
	}
}

// gitServiceAdapterWrapper wraps autobump's GitServiceAdapter to satisfy
// gitforge's LocalGitAuthProvider interface.
type gitServiceAdapterWrapper struct {
	adapter       domainRepos.GitServiceAdapter
	globalConfig  *entities.GlobalConfig
	projectConfig *entities.ProjectConfig
}

func (w *gitServiceAdapterWrapper) Name() string               { return "" }
func (w *gitServiceAdapterWrapper) AuthToken() string          { return "" }
func (w *gitServiceAdapterWrapper) MatchesURL(url string) bool { return w.adapter.MatchesURL(url) }

func (w *gitServiceAdapterWrapper) GetServiceType() entities.ServiceType {
	return w.adapter.GetServiceType()
}

func (w *gitServiceAdapterWrapper) PrepareCloneURL(url string) string {
	return w.adapter.PrepareCloneURL(url)
}

func (w *gitServiceAdapterWrapper) ConfigureTransport() {
	w.adapter.ConfigureTransport()
}

func (w *gitServiceAdapterWrapper) GetAuthMethods(username string) []transport.AuthMethod {
	gc := w.globalConfig
	if gc == nil {
		gc = &entities.GlobalConfig{}
	}
	pc := w.projectConfig
	if pc == nil {
		pc = &entities.ProjectConfig{}
	}
	return w.adapter.GetAuthMethods(username, gc, pc)
}

func (w *gitServiceAdapterWrapper) CloneURL(_ forgeEntities.Repository) string { return "" }

func (w *gitServiceAdapterWrapper) DiscoverRepositories(
	_ context.Context, _ string,
) ([]forgeEntities.Repository, error) {
	return nil, errors.New("not implemented")
}

func (w *gitServiceAdapterWrapper) CreatePullRequest(
	_ context.Context, _ forgeEntities.Repository, _ forgeEntities.PullRequestInput,
) (*forgeEntities.PullRequest, error) {
	return nil, errors.New("not implemented")
}

func (w *gitServiceAdapterWrapper) PullRequestExists(
	_ context.Context, _ forgeEntities.Repository, _ string,
) (bool, error) {
	return false, errors.New("not implemented")
}

// Ensure compile-time type compatibility.
var _ forgeGit.AdapterFinder = (*adapterFinderBridge)(nil)
var _ forgeRepos.LocalGitAuthProvider = (*gitServiceAdapterWrapper)(nil)

// Ensure function signature compatibility at compile time.
var _ func(*git.Repository) (*entities.LatestTag, error) = GetLatestTag

var _ func(entities.ServiceType, string, *entities.GlobalConfig, *entities.ProjectConfig) ([]transport.AuthMethod, error) = GetAuthMethods
var _ *semver.Version
