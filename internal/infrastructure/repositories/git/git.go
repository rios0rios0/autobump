package git

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

	"github.com/rios0rios0/autobump/internal/domain/entities"
	domainRepos "github.com/rios0rios0/autobump/internal/domain/repositories"
)

// AdapterFinder provides adapter lookup capabilities without circular dependencies.
type AdapterFinder interface {
	GetAdapterByServiceType(serviceType entities.ServiceType) domainRepos.GitServiceAdapter
	GetAdapterByURL(url string) domainRepos.GitServiceAdapter
}

// adapterFinder is the package-level adapter finder, set by the application at startup.
var adapterFinder AdapterFinder //nolint:gochecknoglobals // required to break import cycle

// SetAdapterFinder sets the adapter finder used by git utilities.
func SetAdapterFinder(finder AdapterFinder) {
	adapterFinder = finder
}

const (
	DefaultGitTag               = "0.1.0"
	MaxAcceptableInitialCommits = 5
)

var (
	ErrNoAuthMethodFound  = errors.New("no authentication method found")
	ErrAuthNotImplemented = errors.New("authentication method not implemented")
	ErrNoRemoteURL        = errors.New("no remote URL found for repository")
	ErrNoTagsFound        = errors.New("no tags found in Git history")
)

// GetGlobalGitConfig reads the global git configuration file and returns a config.Config object.
func GetGlobalGitConfig() (*config.Config, error) {
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

// GetOptionFromConfig gets a Git option from local and global Git config.
func GetOptionFromConfig(cfg, globalCfg *config.Config, section string, option string) string {
	opt := cfg.Raw.Section(section).Option(option)
	if opt == "" {
		opt = globalCfg.Raw.Section(section).Option(option)
	}
	return opt
}

// OpenRepo opens a git repository at the given path.
func OpenRepo(projectPath string) (*git.Repository, error) {
	log.Infof("Opening repository at %s", projectPath)
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return nil, fmt.Errorf("could not open repository: %w", err)
	}
	return repo, nil
}

// CheckBranchExists checks if a given Git branch exists (local or remote).
func CheckBranchExists(repo *git.Repository, branchName string) (bool, error) {
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

// CreateAndSwitchBranch creates a new branch and switches to it.
func CreateAndSwitchBranch(
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

	return CheckoutBranch(workTree, branchName)
}

// CheckoutBranch switches to the given branch.
func CheckoutBranch(w *git.Worktree, branchName string) error {
	log.Infof("Switching to branch '%s'", branchName)
	err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %w", err)
	}
	return nil
}

// CommitChanges commits the changes in the given worktree.
func CommitChanges(
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

// PushChangesSSH pushes the changes to the remote repository over SSH.
func PushChangesSSH(repo *git.Repository, refSpec config.RefSpec) error {
	log.Info("Pushing local changes to remote repository through SSH")
	err := repo.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// PushChangesHTTPS pushes the changes to the remote repository over HTTPS.
func PushChangesHTTPS(
	repo *git.Repository,
	repoCfg *config.Config,
	refSpec config.RefSpec,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) error {
	log.Info("Pushing local changes to remote repository through HTTPS")
	pushOptions := &git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: "origin",
	}

	service, err := GetRemoteServiceType(repo)
	if err != nil {
		return err
	}
	authMethods, err := GetAuthMethods(service, repoCfg.User.Name, globalConfig, projectConfig)
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

// GetAuthMethods returns the authentication method to use for cloning/pushing changes.
// It delegates to the appropriate adapter based on the service type.
func GetAuthMethods(
	service entities.ServiceType,
	username string,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) ([]transport.AuthMethod, error) {
	adapter := adapterFinder.GetAdapterByServiceType(service)
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

// GetRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab).
func GetRemoteServiceType(repo *git.Repository) (entities.ServiceType, error) {
	cfg, err := repo.Config()
	if err != nil {
		return entities.UNKNOWN, fmt.Errorf("could not get repository config: %w", err)
	}

	var firstRemote string
	for _, remote := range cfg.Remotes {
		firstRemote = remote.URLs[0]
		break
	}

	return GetServiceTypeByURL(firstRemote), nil
}

// GetServiceTypeByURL returns the type of the remote service (e.g. GitHub, GitLab) by URL.
// It uses the adapter registry to determine the service type.
func GetServiceTypeByURL(remoteURL string) entities.ServiceType {
	adapter := adapterFinder.GetAdapterByURL(remoteURL)
	if adapter != nil {
		return adapter.GetServiceType()
	}

	// Fallback for services without adapters (Bitbucket, CodeCommit)
	switch {
	case strings.Contains(remoteURL, "bitbucket.org"):
		return entities.BITBUCKET
	case strings.Contains(remoteURL, "git-codecommit"):
		return entities.CODECOMMIT
	default:
		return entities.UNKNOWN
	}
}

// GetRemoteRepoURL returns the URL of the remote repository.
func GetRemoteRepoURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("could not get remote: %w", err)
	}

	if len(remote.Config().URLs) > 0 {
		return remote.Config().URLs[0], nil // return the first URL configured for the remote
	}

	return "", ErrNoRemoteURL
}

// GetAmountCommits returns the number of commits in the repository.
func GetAmountCommits(repo *git.Repository) (int, error) {
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

// GetLatestTag find the latest tag in the Git history.
func GetLatestTag(repo *git.Repository) (*entities.LatestTag, error) {
	tags, err := repo.Tags()
	if err != nil {
		log.Fatal(err)
	}

	var latestTag *plumbing.Reference
	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		latestTag = tag
		return nil
	})

	numCommits, _ := GetAmountCommits(repo)
	if latestTag == nil {
		// if the project is already started with no tags in the history
		// TODO: review this section
		if numCommits >= MaxAcceptableInitialCommits {
			log.Warnf("No tags found in Git history, falling back to '%s'", DefaultGitTag)
			version, _ := semver.NewVersion(DefaultGitTag)
			return &entities.LatestTag{
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
	if err != nil {
		log.Fatal(err)
	}
	latestTagDate := commit.Committer.When

	version, _ := semver.NewVersion(latestTag.Name().Short())
	return &entities.LatestTag{
		Tag:  version,
		Date: latestTagDate,
	}, nil
}
