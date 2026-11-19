package commands_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	logger "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	credentialEntities "github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	gitforgeEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
)

// The remotes the tests point origin at. Nothing is fetched from or pushed to them: the API
// token is resolved from the URL alone.
const (
	gitHubRemoteURL      = "https://github.com/fixture-owner/fixture-repo.git"
	azureDevOpsRemoteURL = "https://dev.azure.com/fixture-org/fixture-project/_git/fixture-repo"
)

func TestResolveAPIToken(t *testing.T) {
	t.Parallel()

	t.Run("should not ask the fallback when the forge's token is configured", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)
		ctx.GlobalConfig = entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("fixture-configured-token-placeholder").
			BuildGlobalConfig()

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-configured-token-placeholder", token)
		assert.Empty(t, fallback.Requests(), "a configured token must keep absolute priority")
	})

	t.Run("should not ask the fallback when the project carries its own token", func(t *testing.T) {
		t.Parallel()

		// given -- as every repository discovery finds does: it carries its provider's token
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)
		ctx.ProjectConfig = entitybuilders.NewProjectConfigBuilder().
			WithProjectAccessToken("fixture-provider-token-placeholder").
			BuildProjectConfig()

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-provider-token-placeholder", token)
		assert.Empty(t, fallback.Requests())
	})

	t.Run("should return the fallback's token when no token is configured for the forge", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-fallback-token-placeholder", token)
		assert.Equal(t,
			[]credentialEntities.CredentialRequest{{ServiceType: gitforgeEntities.GITHUB, Organization: ""}},
			fallback.Requests(),
		)
	})

	t.Run("should ask the fallback once when the same repository needs the token again", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)

		// when
		first, firstErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)
		second, secondErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.NoError(t, firstErr)
		require.NoError(t, secondErr)
		assert.Equal(t, first, second)
		assert.Len(t, fallback.Requests(), 1)
	})

	t.Run("should ask the fallback once when it found no token", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newDecliningFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)

		// when
		_, firstErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)
		_, secondErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.ErrorIs(t, firstErr, commands.ErrNoAPIToken)
		require.ErrorIs(t, secondErr, commands.ErrNoAPIToken)
		assert.Len(t, fallback.Requests(), 1)
	})

	t.Run("should ask the fallback again rather than reuse one forge's token for another", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)

		// when
		_, gitHubErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)
		_, gitLabErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITLAB)

		// then
		require.NoError(t, gitHubErr)
		require.NoError(t, gitLabErr)
		assert.Equal(t, []credentialEntities.CredentialRequest{
			{ServiceType: gitforgeEntities.GITHUB, Organization: ""},
			{ServiceType: gitforgeEntities.GITLAB, Organization: ""},
		}, fallback.Requests())
	})

	t.Run("should name the forge, setting and fallback's reasons when no token is found", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := newFallbackContext(t, gitHubRemoteURL, newDecliningFallback())

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.ErrorIs(t, err, commands.ErrNoAPIToken)
		require.ErrorIs(t, err, credentialEntities.ErrNoCredential)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "GitHub")
		assert.Contains(t, err.Error(), "github_access_token")
		assert.Contains(t, err.Error(), "GITHUB_TOKEN and GH_TOKEN are not set; the gh CLI is not installed")
	})

	t.Run("should name the forge's setting when there is no fallback", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := newFallbackContext(t, "https://gitlab.com/fixture-group/fixture-repo.git", nil)

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITLAB)

		// then
		require.ErrorIs(t, err, commands.ErrNoAPIToken)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "GitLab")
		assert.Contains(t, err.Error(), "gitlab_access_token")
	})

	t.Run("should not ask the fallback when AutoBump has no API client for the forge", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, "https://bitbucket.org/fixture-owner/fixture-repo.git", fallback)

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.BITBUCKET)

		// then
		require.ErrorIs(t, err, commands.ErrNoAPIToken)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "Bitbucket")
		assert.Empty(t, fallback.Requests())
	})

	t.Run("should treat an empty token from the fallback as no token", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := &doubles.CredentialResolverStub{
			Credential: credentialEntities.Credential{Token: "", Source: "gh CLI"},
		}
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)

		// when
		token, err := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.ErrorIs(t, err, commands.ErrNoAPIToken)
		require.ErrorIs(t, err, credentialEntities.ErrNoCredential)
		assert.Empty(t, token)
	})

	t.Run("should ask for the organization's token when the repository is on Azure DevOps", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, azureDevOpsRemoteURL, fallback)

		// when
		_, err := commands.ResolveAPIToken(ctx, gitforgeEntities.AZUREDEVOPS)

		// then
		require.NoError(t, err)
		assert.Equal(t, []credentialEntities.CredentialRequest{
			{ServiceType: gitforgeEntities.AZUREDEVOPS, Organization: "fixture-org"},
		}, fallback.Requests())
	})

	t.Run("should keep the Azure CLI's default tenant when origin is not a URL gitforge parses", func(t *testing.T) {
		t.Parallel()

		// given
		fallback := newSupplyingFallback()
		ctx := newFallbackContext(t, "https://fixture-org.visualstudio.com/fixture-project/_git/fixture-repo", fallback)

		// when
		_, err := commands.ResolveAPIToken(ctx, gitforgeEntities.AZUREDEVOPS)

		// then
		require.NoError(t, err)
		assert.Equal(t,
			[]credentialEntities.CredentialRequest{{ServiceType: gitforgeEntities.AZUREDEVOPS, Organization: ""}},
			fallback.Requests(),
		)
	})
}

// TestResolveAPITokenLogging is deliberately not parallel: it installs a hook on the standard
// logger, which every test in the package shares.
//
//nolint:paralleltest // test mutates global state or uses t.Setenv
func TestResolveAPITokenLogging(t *testing.T) {
	//nolint:paralleltest // test mutates global state or uses t.Setenv
	t.Run("should name where the token came from once and never log the token", func(t *testing.T) {
		// given
		logs := recordLogs(t)
		ctx := newFallbackContext(t, gitHubRemoteURL, newSupplyingFallback())

		// when
		_, firstErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)
		_, secondErr := commands.ResolveAPIToken(ctx, gitforgeEntities.GITHUB)

		// then
		require.NoError(t, firstErr)
		require.NoError(t, secondErr)
		infos := logs.messagesAt(logger.InfoLevel)
		require.Len(t, infos, 1, "the source is logged once per repository")
		assert.Contains(t, infos[0], "github_access_token")
		assert.Contains(t, infos[0], "GitHub")
		assert.Contains(t, infos[0], "gh CLI")
		assert.False(t, logs.mentions("fixture-fallback-token-placeholder"), "no log line may carry the token")
	})
}

// TestForgeAPICallsUseCredentialFallback is deliberately not parallel: it mutates
// package-level globals that other tests read.
//
//nolint:paralleltest // test mutates global state or uses t.Setenv
func TestForgeAPICallsUseCredentialFallback(t *testing.T) {
	registry := newForgeProviderRegistry(t)
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	//nolint:paralleltest // test mutates global state or uses t.Setenv
	t.Run("should close stale pull requests with the fallback's token when none is configured", func(t *testing.T) {
		// given
		ctx := newFallbackContext(t, gitHubRemoteURL, newSupplyingFallback())

		// when
		provider, forgeRepo := commands.ResolveCleanupProvider(ctx, gitforgeEntities.GITHUB, "main")

		// then
		require.NotNil(t, provider)
		assert.Equal(t, "fixture-fallback-token-placeholder", provider.AuthToken())
		assert.Equal(t, "fixture-owner", forgeRepo.Organization)
		assert.Equal(t, "fixture-repo", forgeRepo.Name)
	})

	//nolint:paralleltest // test mutates global state or uses t.Setenv
	t.Run("should leave stale pull requests open when no token is available", func(t *testing.T) {
		// given
		ctx := newFallbackContext(t, gitHubRemoteURL, newDecliningFallback())

		// when
		provider, _ := commands.ResolveCleanupProvider(ctx, gitforgeEntities.GITHUB, "main")

		// then
		assert.Nil(t, provider)
	})

	//nolint:paralleltest // test mutates global state or uses t.Setenv
	t.Run("should ask the fallback once across checking for and opening a pull request", func(t *testing.T) {
		// given -- the fallback finds nothing, so neither call reaches the forge
		fallback := newDecliningFallback()
		ctx := newFallbackContext(t, gitHubRemoteURL, fallback)
		ctx.ProjectConfig.NewVersion = "1.0.0"

		// when
		exists, checkErr := commands.CheckPullRequestExists(ctx, "chore/bump-1.0.0")
		createErr := commands.CreatePullRequest(ctx, ctx.Repo, "chore/bump-1.0.0", gitforgeEntities.GITHUB)

		// then -- both only warn, as they did before when no token was configured
		require.NoError(t, checkErr)
		require.NoError(t, createErr)
		assert.False(t, exists)
		assert.Equal(t,
			[]credentialEntities.CredentialRequest{{ServiceType: gitforgeEntities.GITHUB, Organization: ""}},
			fallback.Requests(),
		)
	})

	//nolint:paralleltest // test mutates global state or uses t.Setenv
	t.Run("should never offer the fallback's token to a push", func(t *testing.T) {
		// given a remote whose path contains github.com: gitforge takes it for GitHub by that
		// substring, exactly as it would take a lookalike host for GitHub
		remote := newGitHubLookalikeRemote(t)
		repo := newRepoWithOrigin(t, remote)
		head, err := repo.Head()
		require.NoError(t, err)
		const branchName = "chore/bump-1.0.0"
		require.NoError(t, repo.Storer.SetReference(
			plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), head.Hash()),
		))

		serviceType, err := gitInfra.NewGitOperations(registry).GetRemoteServiceType(repo)
		require.NoError(t, err)
		require.Equal(t, gitforgeEntities.GITHUB, serviceType, "the remote must pass for GitHub")

		fallback := newSupplyingFallback()
		ctx := &commands.RepoContext{
			Repo:               repo,
			GlobalGitConfig:    newTestGlobalGitConfig(),
			GlobalConfig:       entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig:      entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
			CredentialFallback: fallback,
		}

		// when
		err = commands.PushChanges(ctx, branchName)

		// then
		require.NoError(t, err)
		assert.Empty(t, fallback.Requests(), "git operations must never use the fallback's token")
	})
}

// TestProcessRepoCredentialFallback is deliberately not parallel: it calls t.Setenv, and it
// mutates package-level globals that other tests read.
func TestProcessRepoCredentialFallback(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("SSH_AUTH_SOCK", "")
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
		0o600,
	))

	registry := newForgeProviderRegistry(t)
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should ask the fallback once when cleanup and the pull request both need a token", func(t *testing.T) {
		// given a GitHub repository holding a stale bump branch, whose configuration holds no
		// token, and a fallback that finds none either, so no call reaches the forge
		remote := newGitHubLookalikeRemote(t)
		repoPath, repo := createTestRepo(t)
		commitChangelog(t, repo, repoPath,
			"# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n"+
				"## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n")
		_, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remote}})
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)
		require.NoError(t, repo.Push(&git.PushOptions{
			RemoteName: "origin",
			RefSpecs: []config.RefSpec{
				config.RefSpec(head.Name().String() + ":" + head.Name().String()),
				config.RefSpec(head.Name().String() + ":refs/heads/chore/bump-0.9.0"),
			},
		}))

		fallback := newDecliningFallback()
		commands.SetCredentialFallback(fallback)
		t.Cleanup(func() { commands.SetCredentialFallback(nil) })

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("fixture-repo").
			BuildProjectConfig()

		// when
		err = commands.ProcessRepo(globalConfig, projectConfig)

		// then the release went out, the stale branch is gone, and the fallback was asked once
		require.NoError(t, err)
		bare, err := git.PlainOpen(remote)
		require.NoError(t, err)
		_, err = bare.Reference(plumbing.NewBranchReferenceName("chore/bump-1.1.0"), false)
		require.NoError(t, err, "the release branch should reach the remote")
		_, err = bare.Reference(plumbing.NewBranchReferenceName("chore/bump-0.9.0"), false)
		require.ErrorIs(t, err, plumbing.ErrReferenceNotFound, "the stale branch should be cleaned up")
		assert.Equal(t,
			[]credentialEntities.CredentialRequest{{ServiceType: gitforgeEntities.GITHUB, Organization: ""}},
			fallback.Requests(),
		)
	})
}

// newSupplyingFallback returns a credential fallback that supplies a token from the gh CLI.
func newSupplyingFallback() *doubles.CredentialResolverStub {
	return &doubles.CredentialResolverStub{
		Credential: credentialEntities.Credential{Token: "fixture-fallback-token-placeholder", Source: "gh CLI"},
	}
}

// newDecliningFallback returns a credential fallback that finds no token, for the reasons
// gitforge's chain gives when GITHUB_TOKEN and GH_TOKEN are unset and gh is not installed.
func newDecliningFallback() *doubles.CredentialResolverStub {
	return &doubles.CredentialResolverStub{
		Err: fmt.Errorf(
			"%w: GITHUB_TOKEN and GH_TOKEN are not set; the gh CLI is not installed",
			credentialEntities.ErrNoCredential,
		),
	}
}

// newFallbackContext returns the context of a repository whose origin is remoteURL and whose
// configuration holds no token for any forge, with fallback standing in for gitforge's
// credential chain.
func newFallbackContext(
	t *testing.T,
	remoteURL string,
	fallback credentialEntities.CredentialResolver,
) *commands.RepoContext {
	t.Helper()

	return &commands.RepoContext{
		Repo:               newRepoWithOrigin(t, remoteURL),
		GlobalConfig:       entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
		ProjectConfig:      entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		CredentialFallback: fallback,
	}
}

// newRepoWithOrigin returns a committed repository whose origin is remoteURL.
func newRepoWithOrigin(t *testing.T, remoteURL string) *git.Repository {
	t.Helper()

	_, repo := createTestRepo(t)
	_, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remoteURL}})
	require.NoError(t, err)
	return repo
}

// newGitHubLookalikeRemote creates an empty bare repository whose path contains github.com,
// which gitforge's URL matching takes for GitHub, and returns its path. Being a local path, it
// is pushed to without any network or credential.
func newGitHubLookalikeRemote(t *testing.T) string {
	t.Helper()

	remote := makeDir(t, filepath.Join(t.TempDir(), "github.com", "fixture-owner", "fixture-repo.git"))
	_, err := git.PlainInit(remote, true)
	require.NoError(t, err)
	return remote
}

// commitChangelog writes the changelog at the root of the repository and commits it.
func commitChangelog(t *testing.T, repo *git.Repository, repoPath, changelog string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "CHANGELOG.md"), []byte(changelog), 0o600))
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add("CHANGELOG.md")
	require.NoError(t, err)
	_, err = worktree.Commit("added the changelog", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
}

// newForgeProviderRegistry returns the provider registry the application wires up, whose
// adapters recognise GitHub, GitLab and Azure DevOps remotes.
func newForgeProviderRegistry(t *testing.T) *repositories.ProviderRegistry {
	t.Helper()

	container := dig.New()
	require.NoError(t, repositories.RegisterProviders(container))
	var registry *repositories.ProviderRegistry
	require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
		registry = r
	}))
	return registry
}

// logRecorder is a handwritten logrus hook: it keeps every entry the standard logger emits
// while it is installed, at every level.
type logRecorder struct {
	entries []recordedEntry
}

// recordedEntry is what a logRecorder keeps of one entry. The entry itself cannot be kept:
// logrus reuses it for the next one.
type recordedEntry struct {
	message string
	fields  string
	level   logger.Level
}

// Levels subscribes the recorder to every level.
func (r *logRecorder) Levels() []logger.Level {
	return logger.AllLevels
}

// Fire records one entry.
func (r *logRecorder) Fire(entry *logger.Entry) error {
	r.entries = append(r.entries, recordedEntry{
		message: entry.Message,
		fields:  fmt.Sprint(entry.Data),
		level:   entry.Level,
	})
	return nil
}

// messagesAt returns the message of every recorded entry at level, in order.
func (r *logRecorder) messagesAt(level logger.Level) []string {
	var messages []string
	for _, entry := range r.entries {
		if entry.level == level {
			messages = append(messages, entry.message)
		}
	}
	return messages
}

// mentions reports whether any recorded entry, at any level, carries text in its message or
// in its fields.
func (r *logRecorder) mentions(text string) bool {
	for _, entry := range r.entries {
		if strings.Contains(entry.message, text) || strings.Contains(entry.fields, text) {
			return true
		}
	}
	return false
}

// recordLogs installs a logRecorder on the standard logger, with every level enabled, until
// the test ends. The caller must not be parallel: the standard logger is shared by every test
// in the package, so a parallel test's entries would be recorded too.
func recordLogs(t *testing.T) *logRecorder {
	t.Helper()

	recorder := &logRecorder{entries: nil}
	hooks := make(logger.LevelHooks)
	hooks.Add(recorder)
	standard := logger.StandardLogger()
	originalHooks := standard.ReplaceHooks(hooks)
	originalLevel := standard.GetLevel()
	standard.SetLevel(logger.TraceLevel)
	t.Cleanup(func() {
		standard.SetLevel(originalLevel)
		standard.ReplaceHooks(originalHooks)
	})

	return recorder
}
