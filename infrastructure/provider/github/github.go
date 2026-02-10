package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gogithub "github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/config"
	"github.com/rios0rios0/autobump/domain"
	gitutil "github.com/rios0rios0/autobump/infrastructure/git"
)

const expectedURLParts = 2

// Adapter implements GitServiceAdapter for GitHub.
type Adapter struct{}

// NewAdapter creates a new GitHub adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) GetServiceType() domain.ServiceType {
	return domain.GITHUB
}

func (a *Adapter) MatchesURL(url string) bool {
	return strings.Contains(url, "github.com")
}

func (a *Adapter) PrepareCloneURL(url string) string {
	return url // GitHub doesn't need URL modification
}

func (a *Adapter) ConfigureTransport() {
	// GitHub doesn't need special transport configuration
}

func (a *Adapter) GetAuthMethods(
	_ string, // username not used for GitHub
	globalConfig *config.GlobalConfig,
	projectConfig *config.ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "x-access-token",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// GitHub personal access token
	if globalConfig.GitHubAccessToken != "" {
		log.Infof("Using GitHub access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "x-access-token",
			Password: globalConfig.GitHubAccessToken,
		})
	}

	return authMethods
}

// PullRequestExists checks if a pull request already exists for the given source branch.
func (a *Adapter) PullRequestExists(
	globalConfig *config.GlobalConfig,
	projectConfig *config.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
) (bool, error) {
	log.Infof("Checking if pull request exists for branch '%s'", sourceBranch)

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitHubAccessToken
	}

	ctx := context.Background()
	client := gogithub.NewClient(nil).WithAuthToken(accessToken)

	// Get the repository owner and name
	owner, repoName, err := getGitHubRepoInfo(repo)
	if err != nil {
		return false, err
	}

	// List open pull requests for the source branch
	prs, _, err := client.PullRequests.List(ctx, owner, repoName, &gogithub.PullRequestListOptions{
		Head:  fmt.Sprintf("%s:%s", owner, sourceBranch),
		State: "open",
	})
	if err != nil {
		return false, fmt.Errorf("failed to list pull requests: %w", err)
	}

	if len(prs) > 0 {
		log.Infof("Found %d open pull request(s) for branch '%s'", len(prs), sourceBranch)
		return true, nil
	}

	log.Infof("No open pull request found for branch '%s'", sourceBranch)
	return false, nil
}

// CreatePullRequest creates a new pull request on GitHub.
func (a *Adapter) CreatePullRequest(
	globalConfig *config.GlobalConfig,
	projectConfig *config.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating GitHub pull request")

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitHubAccessToken
	}

	ctx := context.Background()
	client := gogithub.NewClient(nil).WithAuthToken(accessToken)

	// Get the repository owner and name
	owner, repoName, err := getGitHubRepoInfo(repo)
	if err != nil {
		return err
	}

	prTitle := "chore(bump): bumped version to " + newVersion
	targetBranch := "main"
	maintainerCanModify := true

	pullRequestOptions := &gogithub.NewPullRequest{
		Title:               &prTitle,
		Head:                &sourceBranch,
		Base:                &targetBranch,
		MaintainerCanModify: &maintainerCanModify,
	}

	_, _, err = client.PullRequests.Create(ctx, owner, repoName, pullRequestOptions)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	log.Info("Successfully created GitHub pull request")
	return nil
}

// getGitHubRepoInfo extracts owner and repository name from the remote URL.
func getGitHubRepoInfo(repo *git.Repository) (string, string, error) {
	remoteURL, err := gitutil.GetRemoteRepoURL(repo)
	if err != nil {
		return "", "", err
	}

	// Remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	var owner, repoName string
	switch {
	case strings.HasPrefix(trimmedURL, "git@github.com:"):
		// SSH format: git@github.com:owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "git@github.com:"), "/")
		if len(parts) == expectedURLParts {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid SSH GitHub URL format: %s", remoteURL)
		}
	case strings.HasPrefix(trimmedURL, "https://github.com/"):
		// HTTPS format: https://github.com/owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "https://github.com/"), "/")
		if len(parts) >= expectedURLParts {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid HTTPS GitHub URL format: %s", remoteURL)
		}
	default:
		return "", "", fmt.Errorf("unsupported GitHub URL format: %s", remoteURL)
	}

	if owner == "" || repoName == "" {
		return "", "", fmt.Errorf("could not parse owner and repository name from URL: %s", remoteURL)
	}

	return owner, repoName, nil
}
