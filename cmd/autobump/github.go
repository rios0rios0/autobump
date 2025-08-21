package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"
)

// TODO: this should be better using an Adapter pattern
//
//	(interface with many providers and implementing the methods)
//
// createGitHubPullRequest creates a new pull request on GitHub
func createGitHubPullRequest(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating GitHub pull request")

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitHubAccessToken
	}

	ctx := context.Background()
	client := github.NewClient(nil).WithAuthToken(accessToken)

	// Get the repository owner and name
	owner, repoName, err := getGitHubRepoInfo(repo)
	if err != nil {
		return err
	}

	prTitle := "chore(bump): bumped version to " + newVersion
	targetBranch := "main"
	maintainerCanModify := true

	pullRequestOptions := &github.NewPullRequest{
		Title:               &prTitle,
		Head:                &sourceBranch,
		Base:                &targetBranch,
		MaintainerCanModify: &maintainerCanModify,
	}

	_, _, err = client.PullRequests.Create(ctx, owner, repoName, pullRequestOptions)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	log.Info("Successfully created GitHub pull request")
	return nil
}

// getGitHubRepoInfo extracts owner and repository name from the remote URL
func getGitHubRepoInfo(repo *git.Repository) (string, string, error) {
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return "", "", err
	}

	// Remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	var owner, repoName string
	switch {
	case strings.HasPrefix(trimmedURL, "git@github.com:"):
		// SSH format: git@github.com:owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "git@github.com:"), "/")
		if len(parts) == 2 {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid SSH GitHub URL format: %s", remoteURL)
		}
	case strings.HasPrefix(trimmedURL, "https://github.com/"):
		// HTTPS format: https://github.com/owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "https://github.com/"), "/")
		if len(parts) >= 2 {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid HTTPS GitHub URL format: %s", remoteURL)
		}
	default:
		return "", "", fmt.Errorf("unsupported GitHub URL format: %s", remoteURL)
	}

	if owner == "" || repoName == "" {
		return "", "", fmt.Errorf("could not parse owner and repository name from URL: %s", remoteURL)
	}

	return owner, repoName, nil
}
