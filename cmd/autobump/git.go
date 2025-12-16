package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"
)

type ServiceType int

type LatestTag struct {
	Tag  *semver.Version
	Date time.Time
}

const (
	UNKNOWN ServiceType = iota
	GITHUB
	GITLAB
	AZUREDEVOPS
	BITBUCKET
	CODECOMMIT
)

const (
	defaultGitTag               = "0.1.0"
	maxAcceptableInitialCommits = 5
)

var (
	ErrNoAuthMethodFound  = errors.New("no authentication method found")
	ErrAuthNotImplemented = errors.New("authentication method not implemented")
	ErrNoRemoteURL        = errors.New("no remote URL found for repository")
	ErrNoTagsFound        = errors.New("no tags found in Git history")
)

// getGlobalGitConfig reads the global git configuration file and returns a config.Config object.
func getGlobalGitConfig() (*config.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user home directory: %w", err)
	}

	globalConfigPath := filepath.Join(homeDir, ".gitconfig")
	configBytes, err := os.ReadFile(globalConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not read global git config: %w", err)
	}

	cfg := config.NewConfig()

	// Recover from panics in go-git's Config.Unmarshal (known bug with certain git configs)
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("go-git panicked while parsing git config (known bug), using minimal config: %v", r)
		}
	}()

	if err = cfg.Unmarshal(configBytes); err != nil {
		return nil, fmt.Errorf("could not unmarshal global git config: %w", err)
	}

	return cfg, nil
}

// openRepo opens a git repository at the given path.
func openRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return nil, fmt.Errorf("could not open repository: %w", err)
	}
	return repo, nil
}

// checkBranchExists checks if a given Git branch exists (local or remote).
func checkBranchExists(repo *git.Repository, branchName string) (bool, error) {
	refs, err := repo.References()
	if err != nil {
		return false, fmt.Errorf("could not get repo references: %w", err)
	}

	branchExists := false
	remoteBranchName := "origin/" + branchName
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		refName := ref.Name().String()
		shortName := ref.Name().Short()

		// Check local branch
		if ref.Name().IsBranch() && shortName == branchName {
			branchExists = true
		}
		// Check remote branch (refs/remotes/origin/branchName)
		if strings.HasPrefix(refName, "refs/remotes/") && shortName == remoteBranchName {
			branchExists = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("could not check if branch exists: %w", err)
	}
	return branchExists, nil
}

// createAndSwitchBranch creates a new branch and switches to it.
func createAndSwitchBranch(
	repo *git.Repository,
	workTree *git.Worktree,
	branchName string,
	hash plumbing.Hash,
) error {
	log.Infof("Creating and switching to new branch '%s'", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), hash)
	err := repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("could not create branch: %w", err)
	}

	return checkoutBranch(workTree, branchName)
}

// checkoutBranch switches to the given branch.
func checkoutBranch(w *git.Worktree, branchName string) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}

// commitChanges commits the changes in the given worktree.
func commitChanges(
	workTree *git.Worktree,
	commitMessage string,
	signKey *openpgp.Entity,
	name string,
	email string,
) (plumbing.Hash, error) {
	log.Info("Committing changes")

	// add DCO sign-off
	signoff := fmt.Sprintf("\n\nSigned-off-by: %s <%s>", name, email)
	commitMessage += signoff

	commit, err := workTree.Commit(commitMessage, &git.CommitOptions{SignKey: signKey})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not commit changes: %w", err)
	}
	return commit, nil
}

// pushChangesSSH pushes the changes to the remote repository over SSH.
func pushChangesSSH(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository through SSH")
	err := repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// pushChangesHTTPS pushes the changes to the remote repository over HTTPS.
func pushChangesHTTPS(
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

	service, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}
	authMethods, err := getAuthMethods(service, repoCfg.User.Name, globalConfig, projectConfig)
	if err != nil {
		return err
	}

	// try each authentication method
	for _, auth := range authMethods {
		pushOptions.Auth = auth
		err = repo.Push(pushOptions)

		// if action finished successfully, return
		if err == nil {
			return nil
		}
	}

	if err != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// getAuthMethods returns the authentication method to use for cloning/pushing changes.
// It delegates to the appropriate adapter based on the service type.
func getAuthMethods(
	service ServiceType,
	username string,
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) ([]transport.AuthMethod, error) {
	adapter := GetAdapterByServiceType(service)
	if adapter == nil {
		log.Errorf("No authentication mechanism implemented for service type '%v'", service)
		return nil, ErrAuthNotImplemented
	}

	// Configure transport settings (e.g., Azure DevOps multi_ack workaround)
	adapter.ConfigureTransport()

	// Get authentication methods from the adapter
	authMethods := adapter.GetAuthMethods(username, globalConfig, projectConfig)

	if len(authMethods) == 0 {
		log.Error("No authentication credentials found for any authentication method")
		return nil, ErrNoAuthMethodFound
	}

	return authMethods, nil
}

// getRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab).
func getRemoteServiceType(repo *git.Repository) (ServiceType, error) {
	cfg, err := repo.Config()
	if err != nil {
		return UNKNOWN, fmt.Errorf("could not get repository config: %w", err)
	}

	var firstRemote string
	for _, remote := range cfg.Remotes {
		firstRemote = remote.URLs[0]
		break
	}

	return getServiceTypeByURL(firstRemote), nil
}

// getServiceTypeByURL returns the type of the remote service (e.g. GitHub, GitLab) by URL.
// It uses the adapter registry to determine the service type.
func getServiceTypeByURL(remoteURL string) ServiceType {
	adapter := GetAdapterByURL(remoteURL)
	if adapter != nil {
		return adapter.GetServiceType()
	}

	// Fallback for services without adapters (Bitbucket, CodeCommit)
	switch {
	case strings.Contains(remoteURL, "bitbucket.org"):
		return BITBUCKET
	case strings.Contains(remoteURL, "git-codecommit"):
		return CODECOMMIT
	default:
		return UNKNOWN
	}
}

// getRemoteRepoURL returns the URL of the remote repository.
func getRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("could not get remote: %w", err)
	}

	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil // return the first URL configured for the remote
	}

	return "", ErrNoRemoteURL
}

// getAmountCommits returns the number of commits in the repository.
func getAmountCommits(repo *git.Repository) (int, error) {
	commits, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return 0, fmt.Errorf("could not get commits: %w", err)
	}

	amountCommits := 0
	err = commits.ForEach(func(_ *object.Commit) error {
		amountCommits++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("could not count commits: %w", err)
	}

	return amountCommits, nil
}

// getLatestTag find the latest tag in the Git history.
func getLatestTag(repo *git.Repository) (*LatestTag, error) {
	tags, err := repo.Tags()
	if err != nil {
		log.Fatal(err)
	}

	var latestTag *plumbing.Reference
	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		latestTag = tag
		return nil
	})

	numCommits, _ := getAmountCommits(repo)
	if latestTag == nil {
		// if the project is already started with no tags in the history
		// TODO: review this section
		if numCommits >= maxAcceptableInitialCommits {
			log.Warnf("No tags found in Git history, falling back to '%s'", defaultGitTag)
			version, _ := semver.NewVersion(defaultGitTag)
			return &LatestTag{
				Tag:  version,
				Date: time.Now(),
			}, nil
		}

		// if the project is new, we should not use any tag and just commit the file
		log.Warn("This project seems be a new project, the CHANGELOG should be committed by itself.")
		return nil, ErrNoTagsFound
	}

	// get the date time of the tag
	commit, err := repo.CommitObject(latestTag.Hash())
	latestTagDate := commit.Committer.When
	if err != nil {
		log.Fatal(err)
	}

	version, _ := semver.NewVersion(latestTag.Name().Short())
	return &LatestTag{
		Tag:  version,
		Date: latestTagDate,
	}, nil
}
