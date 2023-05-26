package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/xanzy/go-gitlab"
	"net/url"
	"path/filepath"
	"strings"
)

func createGitLabMergeRequest(globalConfig *GlobalConfig, projectPath string, repo *git.Repository) error {
	gitlabClient, err := gitlab.NewClient(globalConfig.GitLabConfig.GitLabAccessToken)
	if err != nil {
		return err
	}

	remoteURL, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	remoteURLParsed, err := url.Parse(remoteURL)
	if err != nil {
		return err
	}

	namespace, project := filepath.Split(remoteURLParsed.Path)
	namespace = strings.TrimSuffix(namespace, "/")

	projectID := url.PathEscape(fmt.Sprintf("%s/%s", namespace, project))
	mrTitle := "Bump version"

	mergeRequestOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.String("chore/bump"),
		TargetBranch:       gitlab.String("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gitlab.Bool(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	if err != nil {
		return err
	}

	return nil
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
