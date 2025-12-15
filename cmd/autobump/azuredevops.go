package main

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

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
)

const contextTimeout = 10

var (
	ErrUnknownURLType            = errors.New("unknown remote URL type")
	ErrFailedToCreatePullRequest = errors.New("failed to create pull request")
)

// AzureDevOpsInfo struct to hold organization, project, and repo info
type AzureDevOpsInfo struct {
	OrganizationName string
	ProjectName      string
	RepositoryID     string
	DefaultBranch    string
}

// RepoInfo struct to hold repository info from Azure DevOps API
type RepoInfo struct {
	ID            string `json:"id"`
	DefaultBranch string `json:"defaultBranch"`
}

// AzureDevOpsAdapter implements PullRequestProvider for Azure DevOps
type AzureDevOpsAdapter struct{}

// CreatePullRequest creates a new pull request on Azure DevOps
func (a *AzureDevOpsAdapter) CreatePullRequest(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
	repo *git.Repository,
	sourceBranch string,
	newVersion string,
) error {
	log.Info("Creating Azure DevOps pull request")

	var personalAccessToken string
	if projectConfig.ProjectAccessToken != "" {
		personalAccessToken = projectConfig.ProjectAccessToken
	} else {
		personalAccessToken = globalConfig.AzureDevOpsAccessToken
	}

	azureInfo, err := GetAzureDevOpsInfo(repo, personalAccessToken)
	if err != nil {
		return err
	}

	// Determine target branch - use default branch from repository, fallback to main/master
	targetBranch := azureInfo.DefaultBranch
	if targetBranch == "" {
		// Try to get default branch from repository HEAD
		head, err := repo.Head()
		if err == nil {
			// Extract branch name from ref (e.g., "refs/heads/main" -> "main")
			refName := head.Name().String()
			if strings.HasPrefix(refName, "refs/heads/") {
				targetBranch = strings.TrimPrefix(refName, "refs/heads/")
			} else {
				targetBranch = "main" // fallback
			}
		} else {
			targetBranch = "main" // fallback
		}
	} else {
		// DefaultBranch from API is in format "refs/heads/main", extract just the branch name
		if strings.HasPrefix(targetBranch, "refs/heads/") {
			targetBranch = strings.TrimPrefix(targetBranch, "refs/heads/")
		}
	}

	// Ensure targetRefName is in correct format
	targetRefName := targetBranch
	if !strings.HasPrefix(targetRefName, "refs/heads/") {
		targetRefName = "refs/heads/" + targetRefName
	}

	// TODO: refactor to use this library: https://github.com/microsoft/azure-devops-go-api
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/git/repositories/%s/pullrequests?api-version=7.1",
		azureInfo.OrganizationName,
		azureInfo.ProjectName,
		azureInfo.RepositoryID,
	)
	prTitle := "chore(bump): bumped version to " + newVersion
	prDescription := fmt.Sprintf("Automated version bump to %s\n\nThis PR was automatically created by AutoBump.", newVersion)
	payload := map[string]interface{}{
		"sourceRefName": "refs/heads/" + sourceBranch,
		"targetRefName": targetRefName,
		"title":         prTitle,
		"description":   prDescription,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

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
		// Try to parse error response for better error message
		var errorResponse map[string]interface{}
		if json.Unmarshal(body, &errorResponse) == nil {
			if message, ok := errorResponse["message"].(string); ok {
				return fmt.Errorf(
					"%w: %d - %s",
					ErrFailedToCreatePullRequest,
					resp.StatusCode,
					message,
				)
			}
		}
		return fmt.Errorf(
			"%w: %d - %s",
			ErrFailedToCreatePullRequest,
			resp.StatusCode,
			string(body),
		)
	}

	log.Info("Successfully created Azure DevOps pull request")
	return nil
}

// GetAzureDevOpsInfo extracts organization, project, and repo information from the remote URL
func GetAzureDevOpsInfo(
	repo *git.Repository,
	personalAccessToken string,
) (AzureDevOpsInfo, error) {
	var info AzureDevOpsInfo
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return info, err
	}

	var organizationName, projectName, repositoryName string
	parts := strings.Split(remoteURL, "/")

	switch {
	case strings.HasPrefix(remoteURL, "git@"):
		organizationName = parts[1]
		projectName = parts[2]
		repositoryName = parts[3]
	case strings.HasPrefix(remoteURL, "https://"):
		organizationName = parts[3]
		projectName = parts[4]
		repositoryName = parts[5]
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
		return info, fmt.Errorf("repository ID not found in response")
	}

	return AzureDevOpsInfo{
		OrganizationName: organizationName,
		ProjectName:      projectName,
		RepositoryID:     repoInfo.ID,
		DefaultBranch:    repoInfo.DefaultBranch,
	}, nil
}
