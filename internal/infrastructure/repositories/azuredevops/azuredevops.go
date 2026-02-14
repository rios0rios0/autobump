package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gohttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitutil "github.com/rios0rios0/autobump/internal/infrastructure/repositories/git"
	"github.com/rios0rios0/autobump/internal/support"
)

const (
	contextTimeout = 60 * time.Second

	// minSSHURLParts is the minimum number of parts expected when splitting an SSH URL by "/".
	// SSH format: git@ssh.dev.azure.com:v3/{org}/{project}/{repo}
	// Split result: ["git@ssh.dev.azure.com:v3", "{org}", "{project}", "{repo}"].
	minSSHURLParts = 4

	// minHTTPSURLParts is the minimum number of parts expected when splitting an HTTPS URL by "/".
	// HTTPS format: https://dev.azure.com/{org}/{project}/_git/{repo}
	// Split result: ["https:", "", "dev.azure.com", "{org}", "{project}", "_git", "{repo}"].
	minHTTPSURLParts = 7

	// defaultBranchMain is the default branch name for most repositories.
	defaultBranchMain = "main"

	// defaultBranchMaster is the legacy default branch name.
	defaultBranchMaster = "master"
)

var (
	ErrUnknownURLType            = errors.New("unknown remote URL type")
	ErrFailedToCreatePullRequest = errors.New("failed to create pull request")
)

// Info struct to hold organization, project, and repo info.
type Info struct {
	OrganizationName string
	ProjectName      string
	RepositoryID     string
	DefaultBranch    string
}

// RepoInfo struct to hold repository info from Azure DevOps API.
type RepoInfo struct {
	ID            string `json:"id"`
	DefaultBranch string `json:"defaultBranch"`
}

// PullRequestListResponse represents the response from Azure DevOps PR list API.
type PullRequestListResponse struct {
	Value []struct {
		PullRequestID int    `json:"pullRequestId"`
		Status        string `json:"status"`
		SourceRefName string `json:"sourceRefName"`
	} `json:"value"`
	Count int `json:"count"`
}

// Adapter implements GitServiceAdapter for Azure DevOps.
type Adapter struct{}

// NewAdapter creates a new Azure DevOps adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) GetServiceType() entities.ServiceType {
	return entities.AZUREDEVOPS
}

func (a *Adapter) MatchesURL(url string) bool {
	return strings.Contains(url, "dev.azure.com")
}

func (a *Adapter) PrepareCloneURL(url string) string {
	// Strip embedded username from Azure DevOps URLs to avoid conflicts with BasicAuth
	// Example: https://user@dev.azure.com/org/project -> https://dev.azure.com/org/project
	return support.StripUsernameFromURL(url)
}

func (a *Adapter) ConfigureTransport() {
	// Azure DevOps requires capabilities multi_ack / multi_ack_detailed,
	// which are not fully implemented in go-git and by default are included in
	// transport.UnsupportedCapabilities. By replacing (not appending!) the list
	// with only ThinPack, we allow go-git to use multi_ack for initial clones.
	// See: https://github.com/go-git/go-git/blob/master/_examples/azure_devops/main.go
	transport.UnsupportedCapabilities = []capability.Capability{ //nolint:reassign // required for Azure DevOps
		capability.ThinPack,
	}
}

func (a *Adapter) GetAuthMethods(
	_ string, // username not used for Azure DevOps
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) []transport.AuthMethod {
	var authMethods []transport.AuthMethod

	// Project access token (highest priority)
	if projectConfig.ProjectAccessToken != "" {
		log.Infof("Using project access token to authenticate")
		authMethods = append(authMethods, &gohttp.BasicAuth{
			Username: "pat",
			Password: projectConfig.ProjectAccessToken,
		})
	}

	// Azure DevOps personal access token
	if globalConfig.AzureDevOpsAccessToken != "" {
		log.Infof("Using Azure DevOps access token to authenticate")
		authMethods = append(authMethods, &gohttp.BasicAuth{
			Username: "pat",
			Password: globalConfig.AzureDevOpsAccessToken,
		})
	}

	return authMethods
}

// determineFallbackBranch determines the target branch by checking if main or master exist.
func determineFallbackBranch(repo *git.Repository) (string, error) {
	// Try main first
	mainExists, err := gitutil.CheckBranchExists(repo, defaultBranchMain)
	if err != nil {
		return "", fmt.Errorf("failed to check if '%s' branch exists: %w", defaultBranchMain, err)
	}
	if mainExists {
		return defaultBranchMain, nil
	}

	// Try master as fallback
	masterExists, err := gitutil.CheckBranchExists(repo, defaultBranchMaster)
	if err != nil {
		return "", fmt.Errorf("failed to check if '%s' branch exists: %w", defaultBranchMaster, err)
	}
	if masterExists {
		return defaultBranchMaster, nil
	}

	// Neither main nor master exist or both checks failed
	return "", errors.New("neither 'main' nor 'master' branch exists in repository")
}

// determineTargetBranch determines the target branch for a pull request.
// It uses the default branch from Azure DevOps API, falls back to repository HEAD,
// or tries main/master as a last resort.
func determineTargetBranch(repo *git.Repository, defaultBranch string) (string, error) {
	// If we have a default branch from the API, use it (strip refs/heads/ prefix if present)
	if defaultBranch != "" {
		return strings.TrimPrefix(defaultBranch, "refs/heads/"), nil
	}

	// Try to get default branch from repository HEAD
	head, err := repo.Head()
	if err != nil {
		// Failed to get HEAD, try main/master fallback
		return determineFallbackBranch(repo)
	}

	// Extract branch name from ref (e.g., "refs/heads/main" -> "main")
	refName := head.Name().String()
	if strings.HasPrefix(refName, "refs/heads/") {
		return strings.TrimPrefix(refName, "refs/heads/"), nil
	}

	// HEAD doesn't point to a branch, try main/master fallback
	return determineFallbackBranch(repo)
}

// BuildPullRequestPayload constructs the payload for creating a pull request.
func BuildPullRequestPayload(sourceBranch, targetBranch, newVersion string) map[string]interface{} {
	targetRefName := targetBranch
	if !strings.HasPrefix(targetRefName, "refs/heads/") {
		targetRefName = "refs/heads/" + targetRefName
	}

	prTitle := "chore(bump): bumped version to " + newVersion
	prDescription := fmt.Sprintf(
		"Automated version bump to %s\n\nThis PR was automatically created by AutoBump.",
		newVersion,
	)

	return map[string]interface{}{
		"sourceRefName": "refs/heads/" + sourceBranch,
		"targetRefName": targetRefName,
		"title":         prTitle,
		"description":   prDescription,
	}
}

// sendPullRequestRequest sends the HTTP request to create a pull request and handles the response.
func sendPullRequestRequest(
	ctx context.Context,
	url string,
	payloadBytes []byte,
	personalAccessToken string,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(":"+personalAccessToken)),
	)

	log.Infof("POST %s", url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return parseErrorResponse(body, resp.StatusCode)
	}

	return nil
}

// parseErrorResponse extracts error message from Azure DevOps API response.
func parseErrorResponse(body []byte, statusCode int) error {
	var errorResponse map[string]interface{}
	if json.Unmarshal(body, &errorResponse) == nil {
		if message, ok := errorResponse["message"].(string); ok {
			return fmt.Errorf(
				"%w: %d - %s",
				ErrFailedToCreatePullRequest,
				statusCode,
				message,
			)
		}
	}
	return fmt.Errorf(
		"%w: %d - %s",
		ErrFailedToCreatePullRequest,
		statusCode,
		string(body),
	)
}

// PullRequestExists checks if a pull request already exists for the given source branch.
func (a *Adapter) PullRequestExists(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
) (bool, error) {
	log.Infof("Checking if pull request exists for branch '%s'", sourceBranch)

	personalAccessToken := globalConfig.AzureDevOpsAccessToken
	if projectConfig.ProjectAccessToken != "" {
		personalAccessToken = projectConfig.ProjectAccessToken
	}

	azureInfo, err := GetInfo(repo, personalAccessToken)
	if err != nil {
		return false, err
	}

	// Query for active PRs with the source branch
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.sourceRefName=refs/heads/%s&searchCriteria.status=active&api-version=7.1",
		azureInfo.OrganizationName,
		azureInfo.ProjectName,
		azureInfo.RepositoryID,
		sourceBranch,
	)

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(":"+personalAccessToken)),
	)

	log.Infof("GET %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check pull request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check pull request: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var prList PullRequestListResponse
	err = json.Unmarshal(bodyBytes, &prList)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if prList.Count > 0 {
		log.Infof("Found %d active pull request(s) for branch '%s'", prList.Count, sourceBranch)
		return true, nil
	}

	log.Infof("No active pull request found for branch '%s'", sourceBranch)
	return false, nil
}

// CreatePullRequest creates a new pull request on Azure DevOps.
func (a *Adapter) CreatePullRequest(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating Azure DevOps pull request")

	personalAccessToken := globalConfig.AzureDevOpsAccessToken
	if projectConfig.ProjectAccessToken != "" {
		personalAccessToken = projectConfig.ProjectAccessToken
	}

	azureInfo, err := GetInfo(repo, personalAccessToken)
	if err != nil {
		return err
	}

	targetBranch, err := determineTargetBranch(repo, azureInfo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to determine target branch: %w", err)
	}

	// TODO: refactor to use this library: https://github.com/microsoft/azure-devops-go-api
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/git/repositories/%s/pullrequests?api-version=7.1",
		azureInfo.OrganizationName,
		azureInfo.ProjectName,
		azureInfo.RepositoryID,
	)

	payload := BuildPullRequestPayload(sourceBranch, targetBranch, newVersion)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	if err = sendPullRequestRequest(ctx, url, payloadBytes, personalAccessToken); err != nil {
		return err
	}

	log.Info("Successfully created Azure DevOps pull request")
	return nil
}

// GetInfo extracts organization, project, and repo information from the remote URL.
func GetInfo(
	repo *git.Repository,
	personalAccessToken string,
) (Info, error) {
	var info Info
	remoteURL, err := gitutil.GetRemoteRepoURL(repo)
	if err != nil {
		return info, err
	}

	var organizationName, projectName, repositoryName string

	switch {
	case strings.HasPrefix(remoteURL, "git@"):
		// SSH format: git@ssh.dev.azure.com:v3/{org}/{project}/{repo}
		parts := strings.Split(remoteURL, "/")
		if len(parts) < minSSHURLParts {
			return info, fmt.Errorf("%w: invalid SSH URL format: %s", ErrUnknownURLType, remoteURL)
		}
		organizationName = parts[1]
		projectName = parts[2]
		repositoryName = parts[3]
	case strings.HasPrefix(remoteURL, "https://"):
		// HTTPS format: https://dev.azure.com/{org}/{project}/_git/{repo}
		// or with username: https://{user}@dev.azure.com/{org}/{project}/_git/{repo}
		cleanURL := support.StripUsernameFromURL(remoteURL)
		parts := strings.Split(cleanURL, "/")
		if len(parts) < minHTTPSURLParts {
			return info, fmt.Errorf("%w: invalid HTTPS URL format: %s", ErrUnknownURLType, remoteURL)
		}
		organizationName = parts[3]
		projectName = parts[4]
		repositoryName = parts[6]
	default:
		return info, fmt.Errorf("%w: %s", ErrUnknownURLType, remoteURL)
	}

	// fetch repositoryId using Azure DevOps API
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/git/repositories/%s?api-version=6.0",
		organizationName,
		projectName,
		repositoryName,
	)

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return info, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(":"+personalAccessToken)),
	)

	log.Infof("GET %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("failed to fetch repository info: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf(
			"failed to fetch repository info: %d - %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var repoInfo RepoInfo
	err = json.Unmarshal(bodyBytes, &repoInfo)
	if err != nil {
		return info, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if repoInfo.ID == "" {
		return info, errors.New("repository ID not found in response")
	}

	return Info{
		OrganizationName: organizationName,
		ProjectName:      projectName,
		RepositoryID:     repoInfo.ID,
		DefaultBranch:    repoInfo.DefaultBranch,
	}, nil
}
