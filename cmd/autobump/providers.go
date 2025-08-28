package main

import (
	"github.com/go-git/go-git/v5"
)

// PullRequestProvider defines the interface for creating pull/merge requests across different Git hosting providers
type PullRequestProvider interface {
	CreatePullRequest(
		globalConfig *GlobalConfig,
		projectConfig *ProjectConfig,
		repo *git.Repository,
		sourceBranch string,
		newVersion string,
	) error
}

// NewPullRequestProvider creates the appropriate provider based on the service type
func NewPullRequestProvider(serviceType ServiceType) PullRequestProvider {
	switch serviceType {
	case GITHUB:
		return &GitHubAdapter{}
	case GITLAB:
		return &GitLabAdapter{}
	case AZUREDEVOPS:
		return &AzureDevOpsAdapter{}
	default:
		return nil
	}
}
