package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/rios0rios0/autobump/domain"
)

const (
	providerName = "gitlab"
	perPage      = 100
)

// Discoverer implements domain.RepositoryDiscoverer for GitLab.
type Discoverer struct {
	client *gogitlab.Client
}

// NewDiscoverer creates a new GitLab repository discoverer authenticated with the given token.
func NewDiscoverer(token string) domain.RepositoryDiscoverer {
	client, err := gogitlab.NewClient(token)
	if err != nil {
		log.Errorf("Failed to create GitLab client: %v", err)
		return &Discoverer{client: nil}
	}
	return &Discoverer{client: client}
}

func (d *Discoverer) Name() string {
	return providerName
}

// DiscoverRepositories lists all projects in a GitLab group (including sub-groups),
// falling back to user projects if the group listing fails.
func (d *Discoverer) DiscoverRepositories(
	ctx context.Context,
	group string,
) ([]domain.Repository, error) {
	if d.client == nil {
		return nil, errors.New("gitlab client not initialized")
	}

	repos, err := d.discoverGroupProjects(ctx, group)
	if err != nil {
		log.Warnf(
			"Failed to list group projects for %q, falling back to user projects: %v",
			group, err,
		)
		return d.discoverUserProjects(ctx, group)
	}
	return repos, nil
}

func (d *Discoverer) discoverGroupProjects(
	ctx context.Context,
	group string,
) ([]domain.Repository, error) {
	var allRepos []domain.Repository
	opts := &gogitlab.ListGroupProjectsOptions{
		ListOptions:      gogitlab.ListOptions{PerPage: perPage},
		IncludeSubGroups: gogitlab.Ptr(true),
	}

	for {
		projects, resp, err := d.client.Groups.ListGroupProjects(
			group, opts, gogitlab.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list group projects: %w", err)
		}

		for _, proj := range projects {
			allRepos = append(allRepos, gitlabProjectToDomain(proj, group))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (d *Discoverer) discoverUserProjects(
	ctx context.Context,
	user string,
) ([]domain.Repository, error) {
	var allRepos []domain.Repository
	opts := &gogitlab.ListProjectsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: perPage},
	}

	for {
		projects, resp, err := d.client.Projects.ListUserProjects(
			user, opts, gogitlab.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list user projects for %q: %w", user, err)
		}

		for _, proj := range projects {
			allRepos = append(allRepos, gitlabProjectToDomain(proj, user))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func gitlabProjectToDomain(proj *gogitlab.Project, org string) domain.Repository {
	defaultBranch := "main"
	if proj.DefaultBranch != "" {
		defaultBranch = proj.DefaultBranch
	}
	return domain.Repository{
		ID:            strconv.FormatInt(proj.ID, 10),
		Name:          proj.Path,
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		CloneURL:      proj.HTTPURLToRepo,
	}
}
