package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

func getGlobalGitConfig() (*config.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	globalConfigPath := filepath.Join(homeDir, ".gitconfig")
	configBytes, err := ioutil.ReadFile(globalConfigPath)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := cfg.Unmarshal(configBytes); err != nil {
		return nil, err
	}

	return cfg, nil
}

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

func commitChanges(w *git.Worktree, commitMessage string, signKey *openpgp.Entity) (plumbing.Hash, error) {
	log.Info("Committing changes")

	commit, err := w.Commit(commitMessage, &git.CommitOptions{SignKey: signKey})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return commit, nil
}

func pushChangesSsh(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository through SSH")
	return repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
}

func pushChangesHttps(repo *git.Repository, repoCfg *config.Config, refSpec config.RefSpec, globalConfig *GlobalConfig) error {
	log.Info("Pushing local changes to remote repository through HTTPS")
	return repo.Push(&git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: repoCfg.User.Name,
			Password: globalConfig.GitLabAccessToken,
		},
	})
}
