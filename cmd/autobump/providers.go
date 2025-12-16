package main

import (
	"github.com/go-git/go-git/v5"
)

// PullRequestProvider defines the interface for creating pull/merge requests across different Git hosting providers.
// This interface is implemented by GitServiceAdapter, allowing adapters to serve as PR providers.
type PullRequestProvider interface {
	CreatePullRequest(
		globalConfig *GlobalConfig,
		projectConfig *ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
		newVersion string,
	) error

	// PullRequestExists checks if a pull request already exists for the given source branch.
	// Returns true if a PR exists, false otherwise.
	PullRequestExists(
		globalConfig *GlobalConfig,
		projectConfig *ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
	) (bool, error)
}

// NewPullRequestProvider creates the appropriate provider based on the service type.
// It uses the adapter registry to find the correct adapter.
func NewPullRequestProvider(serviceType ServiceType) PullRequestProvider {
	adapter := GetAdapterByServiceType(serviceType)
	if adapter != nil {
		return adapter
	}
	return nil
}
