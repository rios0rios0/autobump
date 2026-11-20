package commands_test

import (
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	gitforgeEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

func TestGitLabCIJobTokenDestinations(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name         string
		server       string
		destinations []string
		expected     []string
	}{
		{"the job runs on gitlab.com and the remote is there", "gitlab.com",
			[]string{"https://gitlab.com/group/project.git"}, []string{"ci-job-token"}},
		{"a self-managed runner pushes to its own server", "gitlab.example.com",
			[]string{"git@gitlab.example.com:group/project.git"}, []string{"ci-job-token"}},
		{"a self-managed runner pushes to gitlab.com", "gitlab.example.com",
			[]string{"https://gitlab.com/group/project.git"}, nil},
		{"one of the origin's URLs is on another server", "gitlab.example.com",
			[]string{"https://gitlab.example.com/group/project.git", "https://gitlab.com/group/project.git"}, nil},
		{"the server matches but the case differs", "GitLab.Example.com",
			[]string{"https://gitlab.example.com/group/project.git"}, []string{"ci-job-token"}},
		{"no CI server is known", "",
			[]string{"https://gitlab.com/group/project.git"}, nil},
		{"there is no destination", "gitlab.com", nil, nil},
	} {
		t.Run("should collect the job token only when "+scenario.name+" names its issuer", func(t *testing.T) {
			t.Parallel()

			// given
			globalConfig := entitybuilders.NewGlobalConfigBuilder().
				WithGitLabCIJobToken("ci-job-token").
				WithGitLabCIServerHost(scenario.server).
				BuildGlobalConfig()
			projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

			// when
			tokens := commands.CollectTokens(
				gitforgeEntities.GITLAB,
				globalConfig,
				projectConfig,
				scenario.destinations,
			)

			// then
			assert.Equal(t, scenario.expected, tokens)
		})
	}

	t.Run("should not call the GitLab API with a job token when the job runs on another server", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitLabCIJobToken("ci-job-token").
			WithGitLabCIServerHost("gitlab.example.com").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		assert.Empty(t, token)
	})

	t.Run("should not call the GitLab API with a job token when no CI server is known", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().WithGitLabCIJobToken("ci-job-token").BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		assert.Empty(t, token)
	})
}

func TestOriginURLs(t *testing.T) {
	t.Parallel()

	t.Run("should return every URL of origin when it fetches and pushes on different URLs", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(t.TempDir(), false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://gitlab.example.com/group/project.git", "https://gitlab.com/group/project.git"},
		})
		require.NoError(t, err)

		// when
		urls := commands.OriginURLs(repo)

		// then
		assert.Equal(t, []string{
			"https://gitlab.example.com/group/project.git", "https://gitlab.com/group/project.git",
		}, urls)
	})

	t.Run("should return nothing when the repository has no origin", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(t.TempDir(), false)
		require.NoError(t, err)

		// when
		urls := commands.OriginURLs(repo)

		// then
		assert.Empty(t, urls)
	})
}
