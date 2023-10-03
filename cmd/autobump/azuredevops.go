package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// AzureDevOpsInfo struct to hold organization, project, and repo info
type AzureDevOpsInfo struct {
	OrganizationName string
	ProjectName      string
	RepositoryID     string
}

// RepoInfo struct to hold repository id answer
type RepoInfo struct {
	Id string `json:"id"`
}

// TODO: this should be better using an Adapter pattern (interface with many providers and implementing the methods)
func createAzureDevOpsPullRequest(
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

	// TODO: refactor to use this library: https://github.com/microsoft/azure-devops-go-api
	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s/pullrequests?api-version=6.0",
		azureInfo.OrganizationName, azureInfo.ProjectName, azureInfo.RepositoryID)
	prTitle := fmt.Sprintf("chore(bump): bumped version to %s", newVersion)
	payload := map[string]interface{}{
		"sourceRefName": fmt.Sprintf("refs/heads/%s", sourceBranch),
		"targetRefName": "refs/heads/main",
		"title":         prTitle,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+personalAccessToken)))

	log.Infof("POST %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create pull request (status: %d), response body is: %s", resp.StatusCode, body)
	}

	log.Info("Successfully created Azure DevOps pull request")
	return nil
}

// GetAzureDevOpsInfo extracts organization, project, and repo information from the remote URL
func GetAzureDevOpsInfo(repo *git.Repository, personalAccessToken string) (info AzureDevOpsInfo, err error) {
	remoteURL, err := getRemoteRepoURL(repo)
	if err != nil {
		return info, err
	}

	var organizationName, projectName, repositoryName string
	parts := strings.Split(remoteURL, "/")
	if strings.HasPrefix(remoteURL, "git@") {
		organizationName = parts[1]
		projectName = parts[2]
		repositoryName = parts[3]
	} else if strings.HasPrefix(remoteURL, "https://") {
		organizationName = parts[3]
		projectName = parts[4]
		repositoryName = parts[5]
	} else {
		return info, fmt.Errorf("unknown URL format")
	}

	// fetch repositoryId using Azure DevOps API
	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s?api-version=6.0", organizationName, projectName, repositoryName)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return info, err
	}

	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+personalAccessToken)))

	log.Infof("GET %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}

	var repoInfo RepoInfo
	err = json.Unmarshal(bodyBytes, &repoInfo)
	if err != nil {
		return info, err
	}

	return AzureDevOpsInfo{
		OrganizationName: organizationName,
		ProjectName:      projectName,
		RepositoryID:     repoInfo.Id,
	}, nil
}
