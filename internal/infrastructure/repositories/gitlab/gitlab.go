package gitlab

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitutil "github.com/rios0rios0/autobump/internal/infrastructure/repositories/git"
)

var (
	ErrInvalidSSHRepoURL  = errors.New("invalid SSH repository URL")
	ErrInvalidRepoURL     = errors.New("invalid repository URL")
	ErrCannotParseRepoURL = errors.New("unable to parse repository URL")
)

// Adapter implements GitServiceAdapter for GitLab.
type Adapter struct{}

// NewAdapter creates a new GitLab adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) GetServiceType() entities.ServiceType {
	return entities.GITLAB
}

func (a *Adapter) MatchesURL(url string) bool {
	return strings.Contains(url, "gitlab.com")
}

func (a *Adapter) PrepareCloneURL(url string) string {
	return url // GitLab doesn't need URL modification
}

func (a *Adapter) ConfigureTransport() {
	// GitLab doesn't need special transport configuration
}

func (a *Adapter) GetAuthMethods(
	username string,
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "oauth2",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// GitLab personal access token
	if globalConfig.GitLabAccessToken != "" {
		log.Infof("Using GitLab access token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: username,
			Password: globalConfig.GitLabAccessToken,
		})
	}

	// CI job token
	if globalConfig.GitLabCIJobToken != "" {
		log.Infof("Using GitLab CI job token to authenticate")
		authMethods = append(authMethods, &http.BasicAuth{
			Username: "gitlab-ci-token",
			Password: globalConfig.GitLabCIJobToken,
		})
	}

	return authMethods
}

// PullRequestExists checks if a merge request already exists for the given source branch.
func (a *Adapter) PullRequestExists(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
) (bool, error) {
	log.Infof("Checking if merge request exists for branch '%s'", sourceBranch)

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitLabAccessToken
	}

	gitlabClient, err := gogitlab.NewClient(accessToken)
	if err != nil {
		return false, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get the project owner and name
	projectName, err := getRemoteRepoFullProjectName(repo)
	if err != nil {
		return false, err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(projectName, &gogitlab.GetProjectOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get project ID: %w", err)
	}

	// List open merge requests for the source branch
	state := "opened"
	mrs, _, err := gitlabClient.MergeRequests.ListProjectMergeRequests(
		project.ID,
		&gogitlab.ListProjectMergeRequestsOptions{
			SourceBranch: &sourceBranch,
			State:        &state,
		},
	)
	if err != nil {
		return false, fmt.Errorf("failed to list merge requests: %w", err)
	}

	if len(mrs) > 0 {
		log.Infof("Found %d open merge request(s) for branch '%s'", len(mrs), sourceBranch)
		return true, nil
	}

	log.Infof("No open merge request found for branch '%s'", sourceBranch)
	return false, nil
}

// CreatePullRequest creates a new merge request on GitLab.
func (a *Adapter) CreatePullRequest(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating GitLab merge request")

	var accessToken string
	if projectConfig.ProjectAccessToken != "" {
		accessToken = projectConfig.ProjectAccessToken
	} else {
		accessToken = globalConfig.GitLabAccessToken
	}

	gitlabClient, err := gogitlab.NewClient(accessToken)
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get the project owner and name
	projectName, err := getRemoteRepoFullProjectName(repo)
	if err != nil {
		return err
	}

	// Get the project ID using the GitLab API
	project, _, err := gitlabClient.Projects.GetProject(projectName, &gogitlab.GetProjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get project ID: %w", err)
	}
	projectID := project.ID

	mrTitle := "chore(bump): bumped version to " + newVersion

	mergeRequestOptions := &gogitlab.CreateMergeRequestOptions{
		SourceBranch:       gogitlab.Ptr(sourceBranch),
		TargetBranch:       gogitlab.Ptr("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gogitlab.Ptr(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	if err != nil {
		return fmt.Errorf("failed to create merge request: %w", err)
	}
	return nil
}

// getRemoteRepoFullProjectName returns the full project name of the remote repository.
func getRemoteRepoFullProjectName(repo *git.Repository) (string, error) {
	remoteURL, err := gitutil.GetRemoteRepoURL(repo)
	if err != nil {
		return "", err
	}

	// remove .git if it exists
	trimmedURL := strings.TrimSuffix(remoteURL, ".git")

	var fullProjectName string
	switch {
	case strings.HasPrefix(trimmedURL, "git@"):
		parts := strings.Split(trimmedURL, ":")
		if len(parts) == 2 { //nolint:mnd // 2 is the minimum number of parts
			fullProjectName = parts[len(parts)-1]
		} else {
			return "", ErrInvalidSSHRepoURL
		}
	case strings.HasPrefix(trimmedURL, "https://"):
		parts := strings.Split(trimmedURL, "/")
		if len(parts) >= 4 { //nolint:mnd // 4 is the minimum number of parts
			fullProjectName = parts[len(parts)-1]
		} else {
			return "", ErrCannotParseRepoURL
		}
	default:
		return "", ErrInvalidRepoURL
	}

	return fullProjectName, nil
}
