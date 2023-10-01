package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

// getGlobalGitConfig reads the global git configuration file and returns a config.Config object
func getGlobalGitConfig() (*config.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	globalConfigPath := filepath.Join(homeDir, ".gitconfig")
	configBytes, err := os.ReadFile(globalConfigPath)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := cfg.Unmarshal(configBytes); err != nil {
		return nil, err
	}

	return cfg, nil
}

// openRepo opens a git repository at the given path
func openRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	return repo, err
}

// createAndSwitchBranch checks if a given Git branch exists
func checkBranchExists(repo *git.Repository, branchName string) (bool, error) {
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

// createAndSwitchBranch creates a new branch and switches to it
func createAndSwitchBranch(
	repo *git.Repository,
	w *git.Worktree,
	branchName string,
	hash plumbing.Hash,
) error {
	log.Infof("Creating and switching to new branch '%s'", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), hash)
	err := repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	return checkoutBranch(w, branchName)
}

// checkoutBranch switches to the given branch
func checkoutBranch(w *git.Worktree, branchName string) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	return err
}

// commitChanges commits the changes in the given worktree
func commitChanges(
	w *git.Worktree,
	commitMessage string,
	signKey *openpgp.Entity,
	name string,
	email string,
) (plumbing.Hash, error) {
	log.Info("Committing changes")

	// add DCO sign-off
	signoff := fmt.Sprintf("\n\nSigned-off-by: %s <%s>", name, email)
	commitMessage += signoff

	commit, err := w.Commit(commitMessage, &git.CommitOptions{SignKey: signKey})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return commit, nil
}

// pushChangesSsh pushes the changes to the remote repository over SSH
func pushChangesSsh(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository through SSH")
	return repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
}

// pushChangesHttps pushes the changes to the remote repository over HTTPS
func pushChangesHttps(
	repo *git.Repository,
	repoCfg *config.Config,
	refSpec config.RefSpec,
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) error {
	log.Info("Pushing local changes to remote repository through HTTPS")
	pushOptions := &git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: "origin",
	}

	// use the project access token if available
	if projectConfig.ProjectAccessToken != "" {
		pushOptions.Auth = &http.BasicAuth{
			Username: repoCfg.User.Name,
			Password: projectConfig.ProjectAccessToken,
		}
	} else {
		pushOptions.Auth = &http.BasicAuth{
			Username: repoCfg.User.Name,
			Password: globalConfig.GitLabAccessToken,
		}
	}

	return repo.Push(pushOptions)
}

// getRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab)
func getRemoteServiceType(repo *git.Repository) (string, error) {
	cfg, err := repo.Config()
	if err != nil {
		return "", err
	}

	// TODO: this could be better using Adapter pattern
	for _, remote := range cfg.Remotes {
		if strings.Contains(remote.URLs[0], "gitlab.com") {
			return "GitLab", nil
		} else if strings.Contains(remote.URLs[0], "github.com") {
			return "GitHub", nil
		} else if strings.Contains(remote.URLs[0], "bitbucket.org") {
			return "Bitbucket", nil
		} else if strings.Contains(remote.URLs[0], "git-codecommit") {
			return "CodeCommit", nil
		} else if strings.Contains(remote.URLs[0], "dev.azure.com") {
			return "AzureDevOps", nil
		}
	}

	return "Unknown", nil
}

// getRemoteRepoURL returns the URL of the remote repository
func getRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}

	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil // return the first URL configured for the remote
	}

	return "", fmt.Errorf("no URLs configured for the remote")
}
