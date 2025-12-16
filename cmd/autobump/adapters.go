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
}

// gitServiceRegistry holds all registered Git service adapters.
var gitServiceRegistry []GitServiceAdapter

// RegisterGitServiceAdapter registers a new Git service adapter.
func RegisterGitServiceAdapter(adapter GitServiceAdapter) {
	gitServiceRegistry = append(gitServiceRegistry, adapter)
}

// GetAdapterByURL returns the appropriate adapter for the given URL.
func GetAdapterByURL(url string) GitServiceAdapter {
	for _, adapter := range gitServiceRegistry {
		if adapter.MatchesURL(url) {
			return adapter
		}
	}
	return nil
}

// GetAdapterByServiceType returns the adapter for the given service type.
func GetAdapterByServiceType(serviceType ServiceType) GitServiceAdapter {
	for _, adapter := range gitServiceRegistry {
		if adapter.GetServiceType() == serviceType {
			return adapter
		}
	}
	return nil
}

// init registers all available adapters.
func init() {
	RegisterGitServiceAdapter(&gitLabServiceAdapter{})
	RegisterGitServiceAdapter(&azureDevOpsServiceAdapter{})
	RegisterGitServiceAdapter(&gitHubServiceAdapter{})
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

