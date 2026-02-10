package azuredevops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/domain"
)

const (
	discovererProviderName = "azuredevops"
	discovererPerPage      = 100
)

// Discoverer implements domain.RepositoryDiscoverer for Azure DevOps.
type Discoverer struct {
	token string
}

// NewDiscoverer creates a new Azure DevOps repository discoverer authenticated with the given token.
func NewDiscoverer(token string) domain.RepositoryDiscoverer {
	return &Discoverer{token: token}
}

func (d *Discoverer) Name() string {
	return discovererProviderName
}

// DiscoverRepositories lists all repositories across all projects in an Azure DevOps organization.
// The org parameter can be an organization URL (https://dev.azure.com/MyOrg) or just the org name.
func (d *Discoverer) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]domain.Repository, error) {
	baseURL := normalizeOrgURL(org)

	projects, err := d.getProjects(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var repos []domain.Repository
	for _, proj := range projects {
		projRepos, repoErr := d.getRepositories(ctx, baseURL, proj.ID, proj.Name)
		if repoErr != nil {
			log.Warnf("Failed to list repos for project %q: %v", proj.Name, repoErr)
			continue
		}
		repos = append(repos, projRepos...)
	}

	return repos, nil
}

// adoProject represents an Azure DevOps project from the API response.
type adoProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// adoProjectListResponse is the API response for listing projects.
type adoProjectListResponse struct {
	Value             []adoProject `json:"value"`
	ContinuationToken string       `json:"continuationToken"`
}

// adoRepo represents an Azure DevOps repository from the API response.
type adoRepo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"defaultBranch"`
	RemoteURL     string `json:"remoteUrl"`
	SSHURL        string `json:"sshUrl"`
}

// adoRepoListResponse is the API response for listing repositories.
type adoRepoListResponse struct {
	Value []adoRepo `json:"value"`
	Count int       `json:"count"`
}

func (d *Discoverer) getProjects(ctx context.Context, baseURL string) ([]adoProject, error) {
	var allProjects []adoProject
	continuationToken := ""

	for {
		url := fmt.Sprintf(
			"%s/_apis/projects?api-version=7.1&$top=%d",
			baseURL, discovererPerPage,
		)
		if continuationToken != "" {
			url += "&continuationToken=" + continuationToken
		}

		body, err := d.doGet(ctx, url)
		if err != nil {
			return nil, err
		}

		var resp adoProjectListResponse
		if unmarshalErr := json.Unmarshal(body, &resp); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to unmarshal projects response: %w", unmarshalErr)
		}

		allProjects = append(allProjects, resp.Value...)

		if resp.ContinuationToken == "" {
			break
		}
		continuationToken = resp.ContinuationToken
	}

	return allProjects, nil
}

func (d *Discoverer) getRepositories(
	ctx context.Context,
	baseURL string,
	projectID string,
	projectName string,
) ([]domain.Repository, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/git/repositories?api-version=7.1",
		baseURL, projectID,
	)

	body, err := d.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var resp adoRepoListResponse
	if unmarshalErr := json.Unmarshal(body, &resp); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal repos response: %w", unmarshalErr)
	}

	orgName := extractOrgName(baseURL)
	var repos []domain.Repository
	for _, r := range resp.Value {
		repos = append(repos, domain.Repository{
			ID:            r.ID,
			Name:          r.Name,
			Organization:  orgName,
			Project:       projectName,
			DefaultBranch: r.DefaultBranch,
			CloneURL:      r.RemoteURL,
		})
	}

	return repos, nil
}

func (d *Discoverer) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(":"+d.token)),
	)

	log.Debugf("GET %s", url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// normalizeOrgURL ensures the org string is a full Azure DevOps base URL.
func normalizeOrgURL(org string) string {
	if strings.HasPrefix(org, "https://") {
		return strings.TrimRight(org, "/")
	}
	return "https://dev.azure.com/" + org
}

// extractOrgName extracts the organization name from a base URL.
func extractOrgName(baseURL string) string {
	parts := strings.Split(strings.TrimRight(baseURL, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return baseURL
}
