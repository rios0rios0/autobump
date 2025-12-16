package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var (
	ErrInvalidSSHRepoURL  = errors.New("invalid SSH repository URL")
	ErrInvalidRepoURL     = errors.New("invalid repository URL")
	ErrCannotParseRepoURL = errors.New("unable to parse repository URL")
)

// GitLabAdapter implements PullRequestProvider for GitLab.
type GitLabAdapter struct{}

// PullRequestExists checks if a merge request already exists for the given source branch.
func (g *GitLabAdapter) PullRequestExists(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
) (bool, error) {
	log.Infof("Checking if merge request exists for branch '%s'", sourceBranch)

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitLabAccessToken
	}

	gitlabClient, err := gitlab.NewClient(accessToken)
	if err != nil {
		return false, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get the project owner and name
	projectName, err := getRemoteRepoFullProjectName(repo)
	if err != nil {
		return false, err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(projectName, &gitlab.GetProjectOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get project ID: %w", err)
	}

	// List open merge requests for the source branch
	state := "opened"
	mrs, _, err := gitlabClient.MergeRequests.ListProjectMergeRequests(project.ID, &gitlab.ListProjectMergeRequestsOptions{
		SourceBranch: &sourceBranch,
		State:        &state,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list merge requests: %w", err)
	}

	if len(mrs) > 0 {
		log.Infof("Found %d open merge request(s) for branch '%s'", len(mrs), sourceBranch)
		return true, nil
	}

	log.Infof("No open merge request found for branch '%s'", sourceBranch)
	return false, nil
}

// CreatePullRequest creates a new merge request on GitLab.
func (g *GitLabAdapter) CreatePullRequest(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating GitLab merge request")

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitLabAccessToken
	}

	gitlabClient, err := gitlab.NewClient(accessToken)
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get the project owner and name
	projectName, err := getRemoteRepoFullProjectName(repo)
	if err != nil {
		return err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(projectName, &gitlab.GetProjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get project ID: %w", err)
	}
	projectID := project.ID

	mrTitle := "chore(bump): bumped version to " + newVersion

	mergeRequestOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.Ptr(sourceBranch),
		TargetBranch:       gitlab.Ptr("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gitlab.Ptr(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	if err != nil {
		return fmt.Errorf("failed to create merge request: %w", err)
	}
	return nil
}

// getRemoteRepoFullProjectName returns the full project name of the remote repository.
func getRemoteRepoFullProjectName(repo *git.Repository) (string, error) {
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return "", err
	}

	// remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	var fullProjectName string
	switch {
	case strings.HasPrefix(trimmedURL, "git@"):
		parts := strings.Split(trimmedURL, ":")
		if len(parts) == 2 { //nolint:mnd // 2 is the minimum number of parts
			fullProjectName = parts[len(parts)-1]
		} else {
			return "", ErrInvalidSSHRepoURL
		}
	case strings.HasPrefix(trimmedURL, "https://"):
		parts := strings.Split(trimmedURL, "/")
		if len(parts) >= 4 { //nolint:mnd // 4 is the minimum number of parts
			fullProjectName = parts[len(parts)-1]
		} else {
			return "", ErrCannotParseRepoURL
		}
	default:
		return "", ErrInvalidRepoURL
	}

	return fullProjectName, nil
}
