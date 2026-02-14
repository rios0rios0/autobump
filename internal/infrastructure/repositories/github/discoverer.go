package github

import (
	"context"
	"fmt"
	"strconv"

	gogithub "github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

const (
	providerName = "github"
	perPage      = 100
)

// Discoverer implements entities.RepositoryDiscoverer for GitHub.
type Discoverer struct {
	client *gogithub.Client
}

// NewDiscoverer creates a new GitHub repository discoverer authenticated with the given token.
func NewDiscoverer(token string) entities.RepositoryDiscoverer {
	client := gogithub.NewClient(nil).WithAuthToken(token)
	return &Discoverer{client: client}
}

func (d *Discoverer) Name() string {
	return providerName
}

// DiscoverRepositories lists all repositories in a GitHub organization,
// falling back to user repositories if the org listing fails.
func (d *Discoverer) DiscoverRepositories(
	ctx context.Context,
	org string,
) ([]entities.Repository, error) {
	repos, err := d.discoverOrgRepos(ctx, org)
	if err != nil {
		log.Warnf("Failed to list org repos for %q, falling back to user repos: %v", org, err)
		return d.discoverUserRepos(ctx, org)
	}
	return repos, nil
}

func (d *Discoverer) discoverOrgRepos(
	ctx context.Context,
	org string,
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
	opts := &gogithub.RepositoryListByOrgOptions{
		ListOptions: gogithub.ListOptions{PerPage: perPage},
	}

	for {
		repos, resp, err := d.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list org repos: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, githubRepoToDomain(r, org))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (d *Discoverer) discoverUserRepos(
	ctx context.Context,
	user string,
) ([]entities.Repository, error) {
	var allRepos []entities.Repository
	opts := &gogithub.RepositoryListByUserOptions{
		ListOptions: gogithub.ListOptions{PerPage: perPage},
	}

	for {
		repos, resp, err := d.client.Repositories.ListByUser(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repos for %q: %w", user, err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, githubRepoToDomain(r, user))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func githubRepoToDomain(r *gogithub.Repository, org string) entities.Repository {
	defaultBranch := "main"
	if r.DefaultBranch != nil {
		defaultBranch = *r.DefaultBranch
	}
	return entities.Repository{
		ID:            strconv.FormatInt(r.GetID(), 10),
		Name:          r.GetName(),
		Organization:  org,
		DefaultBranch: "refs/heads/" + defaultBranch,
		RemoteURL:     r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		ProviderName:  providerName,
	}
}
