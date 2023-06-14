package main

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

func createGitLabMergeRequest(globalConfig *GlobalConfig, repo *git.Repository, sourceBranch string) error {
	log.Info("Creating GitLab merge request")
	gitlabClient, err := gitlab.NewClient(globalConfig.GitLabAccessToken)
	if err != nil {
		return err
	}

	// Get the project owner and name
	owner, projectName, err := getRemoteRepoOwnerAndName(repo)
	if err != nil {
		return err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(fmt.Sprintf("%s/%s", owner, projectName), nil)
	if err != nil {
		return err
	}
	projectID := project.ID

	mrTitle := "Bump version"

	mergeRequestOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.String(sourceBranch),
		TargetBranch:       gitlab.String("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gitlab.Bool(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	return err
}

func getRemoteRepoOwnerAndName(repo *git.Repository) (owner, repoName string, err error) {
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return "", "", err
	}

	// remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	var repoURLParts []string

	// Check if the URL is an SSH URL
	if strings.HasPrefix(trimmedURL, "git@") {
		repoURLParts = strings.Split(trimmedURL, ":")
		if len(repoURLParts) != 2 {
			return "", "", fmt.Errorf("invalid SSH repository URL")
		}
		trimmedURL = repoURLParts[1]
	}

	// Extract owner and repo name
	repoURLParts = strings.Split(trimmedURL, "/")

	if len(repoURLParts) >= 2 {
		owner = repoURLParts[len(repoURLParts)-2]
		repoName = repoURLParts[len(repoURLParts)-1]
	} else {
		err = fmt.Errorf("unable to parse repository URL")
	}

	return owner, repoName, err
}

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
