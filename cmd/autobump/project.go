package main

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	log "github.com/sirupsen/logrus"
)

func openRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	return repo, err
}

func checkIfBranchExists(repo *git.Repository, branchName string) (bool, error) {
	refs, err := repo.References()
	if err != nil {
		return false, err
	}

	branchExists := false
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() && ref.Name().Short() == branchName {
			branchExists = true
		}
		return nil
	})
	return branchExists, err
}

func createAndSwitchBranch(repo *git.Repository, w *git.Worktree, branchName string, hash plumbing.Hash) error {
	log.Infof("Creating and switching to new branch `%s`", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), hash)
	err := repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	return err
}

func commitChanges(w *git.Worktree, commitMessage string, author *object.Signature) (plumbing.Hash, error) {
	log.Info("Committing changes")
	commit, err := w.Commit(commitMessage, &git.CommitOptions{})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return commit, nil
}

func pushChanges(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository")
	return repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
}

func processRepo(globalConfig *GlobalConfig, projectsConfig *ProjectsConfig) error {
	adapter := getAdapterByName(projectsConfig.Language)
	if adapter == nil {
		return fmt.Errorf("invalid adapter: %s", projectsConfig.Language)
	}

	projectPath := projectsConfig.Path
	repo, err := openRepo(projectPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}

	branchName := "chore/bump"
	branchExists, err := checkIfBranchExists(repo, branchName)
	if err != nil {
		return err
	}
	if branchExists {
		return fmt.Errorf("branch %s already exists", branchName)
	}

	err = createAndSwitchBranch(repo, w, branchName, head.Hash())
	if err != nil {
		return err
	}

	log.Info("Updating CHANGELOG.md file")
	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		fmt.Printf("No version found in CHANGELOG.md for project at %s\n", projectsConfig.Path)
		return err
	}

	projectsConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", projectsConfig.NewVersion)
	err = updateVersion(adapter, projectPath, projectsConfig)
	if err != nil {
		return err
	}
	log.Infof("Adding version file %s", adapter.VersionFile(projectsConfig))
	_, err = w.Add(adapter.VersionFile(projectsConfig))
	if err != nil {
		return err
	}

	changelogRelativePath, err := filepath.Rel(projectPath, changelogPath)
	if err != nil {
		return err
	}
	_, err = w.Add(changelogRelativePath)
	if err != nil {
		return err
	}

	commitMessage := "Bump version to " + projectsConfig.NewVersion
	commit, err := commitChanges(w, commitMessage, &object.Signature{})
	if err != nil {
		return err
	}

	_, err = repo.CommitObject(commit)
	if err != nil {
		return err
	}

	refSpec := config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)
	err = pushChanges(repo, refSpec)
	if err != nil {
		return err
	}

	serviceType, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	if serviceType == "GitLab" {
		err = createGitLabMergeRequest(globalConfig, repo, branchName)
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
			log.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}
	return nil
}
