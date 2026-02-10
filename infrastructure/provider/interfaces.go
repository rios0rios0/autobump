package provider

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/rios0rios0/autobump/config"
	"github.com/rios0rios0/autobump/domain"
)

// GitServiceAdapter defines the interface for Git hosting service adapters.
// Each adapter handles authentication, URL processing, and pull request creation
// for a specific Git hosting service (GitHub, GitLab, Azure DevOps, etc.).
type GitServiceAdapter interface {
	// GetServiceType returns the service type identifier for this adapter.
	GetServiceType() domain.ServiceType

	// MatchesURL returns true if the given URL belongs to this service.
	MatchesURL(url string) bool

	// PrepareCloneURL processes the URL before cloning (e.g., stripping embedded credentials).
	PrepareCloneURL(url string) string

	// ConfigureTransport configures any transport-level settings required by this service.
	ConfigureTransport()

	// GetAuthMethods returns the authentication methods for this service.
	GetAuthMethods(
		username string,
		globalConfig *config.GlobalConfig,
		projectConfig *config.ProjectConfig,
	) []transport.AuthMethod

	// CreatePullRequest creates a pull/merge request on this service.
	CreatePullRequest(
		globalConfig *config.GlobalConfig,
		projectConfig *config.ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
		newVersion string,
	) error

	// PullRequestExists checks if a pull request already exists for the given source branch.
	PullRequestExists(
		globalConfig *config.GlobalConfig,
		projectConfig *config.ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
	) (bool, error)
}

// PullRequestProvider defines the interface for creating pull/merge requests across different Git hosting providers.
// This interface is implemented by GitServiceAdapter, allowing adapters to serve as PR providers.
type PullRequestProvider interface {
	CreatePullRequest(
		globalConfig *config.GlobalConfig,
		projectConfig *config.ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
		newVersion string,
	) error

	// PullRequestExists checks if a pull request already exists for the given source branch.
	// Returns true if a PR exists, false otherwise.
	PullRequestExists(
		globalConfig *config.GlobalConfig,
		projectConfig *config.ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
	) (bool, error)
}
