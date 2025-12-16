package main

import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

// GitServiceAdapter defines the interface for Git hosting service adapters.
// Each adapter handles authentication, URL processing, and pull request creation
// for a specific Git hosting service (GitHub, GitLab, Azure DevOps, etc.).
type GitServiceAdapter interface {
	// GetServiceType returns the service type identifier for this adapter.
	GetServiceType() ServiceType

	// MatchesURL returns true if the given URL belongs to this service.
	MatchesURL(url string) bool

	// PrepareCloneURL processes the URL before cloning (e.g., stripping embedded credentials).
	PrepareCloneURL(url string) string

	// ConfigureTransport configures any transport-level settings required by this service.
	ConfigureTransport()

	// GetAuthMethods returns the authentication methods for this service.
	GetAuthMethods(username string, globalConfig *GlobalConfig, projectConfig *ProjectConfig) []transport.AuthMethod

	// CreatePullRequest creates a pull/merge request on this service.
	CreatePullRequest(
		globalConfig *GlobalConfig,
		projectConfig *ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
		newVersion string,
	) error

	// PullRequestExists checks if a pull request already exists for the given source branch.
	PullRequestExists(
		globalConfig *GlobalConfig,
		projectConfig *ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
	) (bool, error)
}

// GitServiceRegistry manages all registered Git service adapters.
type GitServiceRegistry struct {
	adapters []GitServiceAdapter
}

// NewGitServiceRegistry creates a new registry with all available adapters pre-registered.
func NewGitServiceRegistry() *GitServiceRegistry {
	return &GitServiceRegistry{
		adapters: []GitServiceAdapter{
			&gitLabServiceAdapter{},
			&azureDevOpsServiceAdapter{},
			&gitHubServiceAdapter{},
		},
	}
}

// Register adds a new Git service adapter to the registry.
func (r *GitServiceRegistry) Register(adapter GitServiceAdapter) {
	r.adapters = append(r.adapters, adapter)
}

// GetAdapterByURL returns the appropriate adapter for the given URL.
func (r *GitServiceRegistry) GetAdapterByURL(url string) GitServiceAdapter {
	for _, adapter := range r.adapters {
		if adapter.MatchesURL(url) {
			return adapter
		}
	}
	return nil
}

// GetAdapterByServiceType returns the adapter for the given service type.
func (r *GitServiceRegistry) GetAdapterByServiceType(serviceType ServiceType) GitServiceAdapter {
	for _, adapter := range r.adapters {
		if adapter.GetServiceType() == serviceType {
			return adapter
		}
	}
	return nil
}

// defaultRegistry is the default registry instance used by global functions.
// It is lazily initialized on first use.
var defaultRegistry *GitServiceRegistry //nolint:gochecknoglobals // required for backward compatibility

// getDefaultRegistry returns the default registry, initializing it if needed.
func getDefaultRegistry() *GitServiceRegistry {
	if defaultRegistry == nil {
		defaultRegistry = NewGitServiceRegistry()
	}
	return defaultRegistry
}

// GetAdapterByURL returns the appropriate adapter for the given URL using the default registry.
func GetAdapterByURL(url string) GitServiceAdapter {
	return getDefaultRegistry().GetAdapterByURL(url)
}

// GetAdapterByServiceType returns the adapter for the given service type using the default registry.
func GetAdapterByServiceType(serviceType ServiceType) GitServiceAdapter {
	return getDefaultRegistry().GetAdapterByServiceType(serviceType)
}

// =============================================================================
// GitLab Adapter
// =============================================================================

type gitLabServiceAdapter struct {
	GitLabAdapter // Embed existing PR functionality
}

func (a *gitLabServiceAdapter) GetServiceType() ServiceType {
	return GITLAB
}

func (a *gitLabServiceAdapter) MatchesURL(url string) bool {
	return strings.Contains(url, "gitlab.com")
}

func (a *gitLabServiceAdapter) PrepareCloneURL(url string) string {
	return url // GitLab doesn't need URL modification
}

func (a *gitLabServiceAdapter) ConfigureTransport() {
	// GitLab doesn't need special transport configuration
}

func (a *gitLabServiceAdapter) GetAuthMethods(
	username string,
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "oauth2",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// GitLab personal access token
	if globalConfig.GitLabAccessToken != "" {
		log.Infof("Using GitLab access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: username,
			Password: globalConfig.GitLabAccessToken,
		})
	}

	// CI job token
	if globalConfig.GitLabCIJobToken != "" {
		log.Infof("Using GitLab CI job token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "gitlab-ci-token",
			Password: globalConfig.GitLabCIJobToken,
		})
	}

	return authMethods
}

// =============================================================================
// Azure DevOps Adapter
// =============================================================================

type azureDevOpsServiceAdapter struct {
	AzureDevOpsAdapter // Embed existing PR functionality
}

func (a *azureDevOpsServiceAdapter) GetServiceType() ServiceType {
	return AZUREDEVOPS
}

func (a *azureDevOpsServiceAdapter) MatchesURL(url string) bool {
	return strings.Contains(url, "dev.azure.com")
}

func (a *azureDevOpsServiceAdapter) PrepareCloneURL(url string) string {
	// Strip embedded username from Azure DevOps URLs to avoid conflicts with BasicAuth
	// Example: https://user@dev.azure.com/org/project -> https://dev.azure.com/org/project
	return stripUsernameFromURL(url)
}

func (a *azureDevOpsServiceAdapter) ConfigureTransport() {
	// Azure DevOps requires capabilities multi_ack / multi_ack_detailed,
	// which are not fully implemented in go-git and by default are included in
	// transport.UnsupportedCapabilities. By replacing (not appending!) the list
	// with only ThinPack, we allow go-git to use multi_ack for initial clones.
	// See: https://github.com/go-git/go-git/blob/master/_examples/azure_devops/main.go
	transport.UnsupportedCapabilities = []capability.Capability{ //nolint:reassign // required for Azure DevOps
		capability.ThinPack,
	}
}

func (a *azureDevOpsServiceAdapter) GetAuthMethods(
	_ string, // username not used for Azure DevOps
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "pat",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// Azure DevOps personal access token
	if globalConfig.AzureDevOpsAccessToken != "" {
		log.Infof("Using Azure DevOps access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "pat",
			Password: globalConfig.AzureDevOpsAccessToken,
		})
	}

	return authMethods
}

// =============================================================================
// GitHub Adapter
// =============================================================================

type gitHubServiceAdapter struct {
	GitHubAdapter // Embed existing PR functionality
}

func (a *gitHubServiceAdapter) GetServiceType() ServiceType {
	return GITHUB
}

func (a *gitHubServiceAdapter) MatchesURL(url string) bool {
	return strings.Contains(url, "github.com")
}

func (a *gitHubServiceAdapter) PrepareCloneURL(url string) string {
	return url // GitHub doesn't need URL modification
}

func (a *gitHubServiceAdapter) ConfigureTransport() {
	// GitHub doesn't need special transport configuration
}

func (a *gitHubServiceAdapter) GetAuthMethods(
	_ string, // username not used for GitHub
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "x-access-token",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// GitHub personal access token
	if globalConfig.GitHubAccessToken != "" {
		log.Infof("Using GitHub access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "x-access-token",
			Password: globalConfig.GitHubAccessToken,
		})
	}

	return authMethods
}
