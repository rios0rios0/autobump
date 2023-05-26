package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	log "github.com/sirupsen/logrus"
)

func processRepo(globalConfig *GlobalConfig, config *ProjectsConfig) error {
	adapter := getAdapterByName(config.Language)
	if adapter == nil {
		return fmt.Errorf("invalid adapter: %s", config.Language)
	}

	projectPath := config.Path

	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		fmt.Printf("No version found in CHANGELOG.md for project at %s\n", config.Path)
		return err
	}

	config.NewVersion = version.String()
	err = adapter.UpdateVersion(projectPath, config)
	if err != nil {
		return err
	}

	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	changelogRelativePath, err := filepath.Rel(changelogPath, projectPath)
	if err != nil {
		return err
	}

	result, err := w.Add(changelogRelativePath)
	if err != nil {
		log.Errorf("Result not expected: %v", result)
		return err
	}

	commit, err := w.Commit("Bump version to "+config.NewVersion, &git.CommitOptions{
		Author: &object.Signature{
			Name:  globalConfig.GitLabConfig.UserName,
			Email: globalConfig.GitLabConfig.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	_, err = repo.CommitObject(commit)
	if err != nil {
		return err
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}

	ref := plumbing.NewHashReference("refs/heads/chore/bump", head.Hash())
	err = repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	serviceType, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	if serviceType == "GitLab" {
		err = createGitLabMergeRequest(globalConfig, projectPath, repo)
		if err != nil {
			return err
		}
	}

	return nil
}

func iterateProjects(globalConfig *GlobalConfig) error {
	for _, project := range globalConfig.ProjectsConfig {
		err := processRepo(globalConfig, &project)
		if err != nil {
			fmt.Printf("Error processing project at %s: %v\n", project.Path, err)
		}
	}
	return nil
}
