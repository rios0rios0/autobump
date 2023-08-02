package main

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// createGitLabMergeRequest creates a new merge request on GitLab
func createGitLabMergeRequest(
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
	} else if globalConfig.GitLabCIJobToken != "" {
		accessToken = globalConfig.GitLabCIJobToken
	} else {
		accessToken = globalConfig.GitLabAccessToken
	}

	gitlabClient, err := gitlab.NewClient(accessToken)
	if err != nil {
		return err
	}

	// Get the project owner and name
	projectName, err := getRemoteRepoFullProjectName(repo)
	if err != nil {
		return err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(projectName, &gitlab.GetProjectOptions{})
	if err != nil {
		return err
	}
	projectID := project.ID

	mrTitle := fmt.Sprintf("chore(bump): bumped version to %s", newVersion)

	mergeRequestOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.String(sourceBranch),
		TargetBranch:       gitlab.String("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gitlab.Bool(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	return err
}

// getRemoteRepoFullProjectName returns the full project name of the remote repository
func getRemoteRepoFullProjectName(repo *git.Repository) (fullProjectName string, err error) {
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return "", err
	}

	// remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(trimmedURL, "git@") {
		parts := strings.Split(trimmedURL, ":")
		if len(parts) == 2 {
			fullProjectName = parts[1]
		} else {
			return "", fmt.Errorf("invalid SSH repository URL")
		}
	} else if strings.HasPrefix(trimmedURL, "https://") {
		parts := strings.SplitN(trimmedURL, "/", 4)
		if len(parts) >= 4 {
			fullProjectName = parts[3]
		} else {
			return "", fmt.Errorf("unable to parse repository URL")
		}
	} else {
		return "", fmt.Errorf("invalid repository URL: must be SSH or HTTPS")
	}

	return fullProjectName, nil
}

// getRemoteRepoURL returns the URL of the remote repository
func getRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}

	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil // Return the first URL configured for the remote
	}

	return "", fmt.Errorf("No URLs configured for the remote")
}

// getRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab)
func getRemoteServiceType(repo *git.Repository) (string, error) {
	cfg, err := repo.Config()
	if err != nil {
		return "", err
	}

	for _, remote := range cfg.Remotes {
		if strings.Contains(remote.URLs[0], "gitlab.com") {
			return "GitLab", nil
		} else if strings.Contains(remote.URLs[0], "github.com") {
			return "GitHub", nil
		}
	}

	return "Unknown", nil
}
