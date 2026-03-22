//go:build unit

package commands_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
	langEntities "github.com/rios0rios0/langforge/pkg/domain/entities"
)

// generateTestSSHKey creates a valid Ed25519 SSH private key in OpenSSH format for testing.
func generateTestSSHKey(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pemBlock, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	return pem.EncodeToMemory(pemBlock)
}

func TestDetectProjectLanguage(t *testing.T) {
	t.Parallel()

	t.Run("should detect language by special pattern", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {SpecialPatterns: []string{"go.mod"}, Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		language, err := commands.DetectProjectLanguage(globalConfig, tmpDir)

		// then
		require.NoError(t, err)
		assert.Equal(t, "golang", language)
	})

	t.Run("should detect language by file extension", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("print('hello')"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"python": {Extensions: []string{"py"}},
			}).
			BuildGlobalConfig()

		// when
		language, err := commands.DetectProjectLanguage(globalConfig, tmpDir)

		// then
		require.NoError(t, err)
		assert.Equal(t, "python", language)
	})

	t.Run("should return error when no language is detected", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {SpecialPatterns: []string{"go.mod"}, Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		language, err := commands.DetectProjectLanguage(globalConfig, tmpDir)

		// then
		assert.ErrorIs(t, err, commands.ErrProjectLanguageNotRecognized)
		assert.Empty(t, language)
	})
}

func TestResolveConfigKey(t *testing.T) {
	t.Parallel()

	t.Run("should return direct match when langforge name is a config key", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"python": {Extensions: []string{"py"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.ResolveConfigKey(globalConfig, langEntities.LanguagePython)

		// then
		assert.Equal(t, "python", result)
	})

	t.Run("should return alias when langforge name is not a config key", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.ResolveConfigKey(globalConfig, langEntities.LanguageGo)

		// then
		assert.Equal(t, "golang", result)
	})

	t.Run("should return empty string when no match found", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"python": {Extensions: []string{"py"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.ResolveConfigKey(globalConfig, langEntities.Language("rust"))

		// then
		assert.Empty(t, result)
	})
}

func TestBuildGitforgeRepo(t *testing.T) {
	t.Parallel()

	t.Run("should parse GitHub SSH URL", func(t *testing.T) {
		// given
		url := "git@github.com:myorg/myrepo.git"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "myorg", repo.Organization)
		assert.Equal(t, "myrepo", repo.Name)
		assert.Equal(t, url, repo.RemoteURL)
	})

	t.Run("should parse GitHub HTTPS URL", func(t *testing.T) {
		// given
		url := "https://github.com/myorg/myrepo.git"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "myorg", repo.Organization)
		assert.Equal(t, "myrepo", repo.Name)
	})

	t.Run("should parse GitLab SSH URL", func(t *testing.T) {
		// given
		url := "git@gitlab.com:group/subgroup/project.git"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "group/subgroup", repo.Organization)
		assert.Equal(t, "project", repo.Name)
	})

	t.Run("should parse GitLab HTTPS URL", func(t *testing.T) {
		// given
		url := "https://gitlab.com/group/subgroup/project.git"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "group/subgroup", repo.Organization)
		assert.Equal(t, "project", repo.Name)
	})

	t.Run("should parse Azure DevOps SSH URL", func(t *testing.T) {
		// given
		url := "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "myorg", repo.Organization)
		assert.Equal(t, "myproject", repo.Project)
		assert.Equal(t, "myrepo", repo.Name)
	})

	t.Run("should parse Azure DevOps HTTPS URL", func(t *testing.T) {
		// given
		url := "https://dev.azure.com/myorg/myproject/_git/myrepo"

		// when
		repo := commands.BuildGitforgeRepo(url, "main")

		// then
		assert.Equal(t, "myorg", repo.Organization)
		assert.Equal(t, "myproject", repo.Project)
		assert.Equal(t, "myrepo", repo.Name)
	})
}

func TestServiceTypeToProviderName(t *testing.T) {
	t.Parallel()

	t.Run("should return github for GITHUB type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.GITHUB

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "github", name)
	})

	t.Run("should return gitlab for GITLAB type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.GITLAB

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "gitlab", name)
	})

	t.Run("should return azuredevops for AZUREDEVOPS type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.AZUREDEVOPS

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "azuredevops", name)
	})

	t.Run("should return empty string for unknown type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.UNKNOWN

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Empty(t, name)
	})
}

func TestResolveToken(t *testing.T) {
	t.Parallel()

	t.Run("should return project access token when set", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("global-token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithProjectAccessToken("project-token").
			BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "project-token", token)
	})

	t.Run("should return GitHub global token when project token is empty", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("github-global").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "github-global", token)
	})

	t.Run("should return GitLab CI job token as fallback", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitLabCIJobToken("ci-job-token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "ci-job-token", token)
	})

	t.Run("should return empty string for unknown service type", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		token := commands.ResolveToken(gitforgeEntities.UNKNOWN, globalConfig, projectConfig)

		// then
		assert.Empty(t, token)
	})
}

func TestCollectTokens(t *testing.T) {
	t.Parallel()

	t.Run("should return project access token first when set", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("global-github").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithProjectAccessToken("project-token").
			BuildProjectConfig()

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 2)
		assert.Equal(t, "project-token", tokens[0])
		assert.Equal(t, "global-github", tokens[1])
	})

	t.Run("should return GitLab access token and CI job token", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitLabAccessToken("gitlab-pat").
			WithGitLabCIJobToken("ci-job-token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 2)
		assert.Equal(t, "gitlab-pat", tokens[0])
		assert.Equal(t, "ci-job-token", tokens[1])
	})

	t.Run("should return Azure DevOps token", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithAzureDevOpsAccessToken("ado-token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		tokens := commands.CollectTokens(gitforgeEntities.AZUREDEVOPS, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 1)
		assert.Equal(t, "ado-token", tokens[0])
	})

	t.Run("should return empty slice when no tokens configured", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Empty(t, tokens)
	})

	t.Run("should return empty slice for unknown service type", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("github-token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		tokens := commands.CollectTokens(gitforgeEntities.UNKNOWN, globalConfig, projectConfig)

		// then
		assert.Empty(t, tokens)
	})
}

func TestCollectSSHAuthMethods(t *testing.T) { //nolint:paralleltest // t.Setenv is incompatible with t.Parallel
	t.Run("should return empty slice when no SSH config and no agent", func(t *testing.T) {
		// given
		t.Setenv("SSH_AUTH_SOCK", "")
		t.Setenv("HOME", t.TempDir())
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		assert.Empty(t, methods)
	})

	t.Run("should return SSH key auth when ssh_key_path is configured", func(t *testing.T) {
		// given
		t.Setenv("SSH_AUTH_SOCK", "")
		keyDir := t.TempDir()
		keyPath := filepath.Join(keyDir, "test_key")
		keyContent := generateTestSSHKey(t)
		require.NoError(t, os.WriteFile(keyPath, keyContent, 0o600))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithSSHKeyPath(keyPath).
			BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		require.Len(t, methods, 1)
		assert.Equal(t, "ssh-public-keys", methods[0].Name())
	})

	t.Run("should return empty slice when ssh_key_path points to nonexistent file", func(t *testing.T) {
		// given
		t.Setenv("SSH_AUTH_SOCK", "")
		t.Setenv("HOME", t.TempDir())
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithSSHKeyPath("/nonexistent/path/key").
			BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		assert.Empty(t, methods)
	})
}

func TestDetectSSHAgentSockets(t *testing.T) { //nolint:paralleltest // t.Setenv is incompatible with t.Parallel
	t.Run("should return SSH_AUTH_SOCK from environment when set to a valid socket", func(t *testing.T) {
		// given
		sockDir, err := os.MkdirTemp("", "s-*")
		require.NoError(t, err)
		defer os.RemoveAll(sockDir) //nolint:errcheck // test cleanup

		sockPath := filepath.Join(sockDir, "a.sock")
		listener, err := net.Listen("unix", sockPath)
		require.NoError(t, err)
		defer listener.Close() //nolint:errcheck // test cleanup

		t.Setenv("SSH_AUTH_SOCK", sockPath)
		t.Setenv("HOME", t.TempDir())

		// when
		sockets := commands.DetectSSHAgentSockets()

		// then
		require.NotEmpty(t, sockets)
		assert.Equal(t, sockPath, sockets[0])
	})
}

func TestGetForgeProvider(t *testing.T) {
	t.Parallel()

	t.Run("should return error for unsupported service type", func(t *testing.T) {
		// given / when
		provider, err := commands.GetForgeProvider(gitforgeEntities.UNKNOWN, "some-token")

		// then
		require.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "unsupported service type")
	})

	t.Run("should return error when provider registry is nil", func(t *testing.T) {
		// given
		commands.SetProviderRegistry(nil)

		// when
		provider, err := commands.GetForgeProvider(gitforgeEntities.GITHUB, "some-token")

		// then
		require.Error(t, err)
		assert.Nil(t, provider)
	})
}

func TestRepoToProjectConfig(t *testing.T) {
	t.Parallel()

	t.Run("should convert repository and provider config to project config", func(t *testing.T) {
		// given
		repo := entitybuilders.NewRepositoryBuilder().
			WithName("my-repo").
			WithOrganization("my-org").
			WithRemoteURL("https://github.com/my-org/my-repo.git").
			BuildRepository()
		provCfg := entitybuilders.NewProviderConfigBuilder().
			WithType("github").
			WithToken("test-token").
			BuildProviderConfig()

		// when
		result := commands.RepoToProjectConfig(repo, provCfg)

		// then
		assert.Equal(t, "https://github.com/my-org/my-repo.git", result.Path)
		assert.Equal(t, "my-repo", result.Name)
		assert.Equal(t, "test-token", result.ProjectAccessToken)
	})
}

func TestLoadProjectConfigOverrides(t *testing.T) {
	t.Parallel()

	t.Run("should return original config when no per-project config exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Equal(t, globalConfig, result)
	})

	t.Run("should merge per-project language overrides into global config", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  python:\n    extensions:\n      - 'py'\n    version_files:\n      - path: 'custom_version.py'\n        patterns:\n          - '(__version__\\s*=\\s*\")\\d+\\.\\d+\\.\\d+(\")'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Contains(t, result.LanguagesConfig, "golang")
		assert.Contains(t, result.LanguagesConfig, "python")
		assert.NotContains(t, globalConfig.LanguagesConfig, "python")
	})

	t.Run("should return original config when per-project config has no languages key", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "github_access_token: 'ignored-token'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("original-token").
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Equal(t, globalConfig, result)
		assert.Equal(t, "original-token", result.GitHubAccessToken)
	})

	t.Run("should not mutate global config when per-project config is invalid YAML", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte("invalid: [yaml: {broken"),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Equal(t, globalConfig, result)
	})

	t.Run("should not mutate global config when merging overrides", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  ruby:\n    extensions:\n      - 'rb'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()
		originalLangsCount := len(globalConfig.LanguagesConfig)

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Len(t, globalConfig.LanguagesConfig, originalLangsCount)
		assert.NotContains(t, globalConfig.LanguagesConfig, "ruby")
		assert.Contains(t, result.LanguagesConfig, "ruby")
	})

	t.Run("should override version file patterns for existing language", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  typescript:\n    version_files:\n      - path: 'opensearch_dashboards.json'\n        patterns:\n          - '(\"version\":\\s*\")\\d+\\.\\d+\\.\\d+(\")'  \n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"typescript": {
					Extensions: []string{"ts"},
					VersionFiles: []entities.VersionFile{
						{Path: "package.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
					},
				},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		ts := result.LanguagesConfig["typescript"]
		assert.Len(t, ts.VersionFiles, 2)
		assert.Equal(t, []string{"ts"}, ts.Extensions)
	})

	t.Run("should add new language from per-project config while keeping global languages", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  java:\n    extensions:\n      - 'java'\n    special_patterns:\n      - 'pom.xml'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
				"python": {Extensions: []string{"py"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Contains(t, result.LanguagesConfig, "golang")
		assert.Contains(t, result.LanguagesConfig, "python")
		assert.Contains(t, result.LanguagesConfig, "java")
	})

	t.Run("should handle .autobump.yml variant", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  ruby:\n    extensions:\n      - 'rb'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Contains(t, result.LanguagesConfig, "ruby")
	})

	t.Run("should handle autobump.yaml variant without dot prefix", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "languages:\n  ruby:\n    extensions:\n      - 'rb'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Contains(t, result.LanguagesConfig, "ruby")
	})

	t.Run("should preserve global tokens even if per-project config contains tokens", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configContent := "github_access_token: 'project-token'\nlanguages:\n  ruby:\n    extensions:\n      - 'rb'\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".autobump.yaml"),
			[]byte(configContent),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("global-token").
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, tmpDir)

		// then
		assert.Equal(t, "global-token", result.GitHubAccessToken)
		assert.Contains(t, result.LanguagesConfig, "ruby")
	})
}

func TestProcessRepo(t *testing.T) {
	t.Parallel()

	t.Run("should return error when git repo cannot be opened", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath("/nonexistent/path/that/does/not/exist").
			BuildProjectConfig()

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		assert.Error(t, err)
	})
}

func TestShouldBumpProject(t *testing.T) {
	t.Parallel()

	emptyChangelog := `# Changelog

## [Unreleased]

### Added

### Changed

### Removed
`
	populatedChangelog := `# Changelog

## [Unreleased]

### Added

- added new feature

### Changed

### Removed
`

	t.Run("should return false when NewChangelogCreated is true and unreleased is empty", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte(emptyChangelog), 0o644))

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			BuildProjectConfig()
		ctx := &commands.RepoContext{
			ProjectConfig:       projectConfig,
			NewChangelogCreated: true,
		}

		// when
		result, err := commands.ShouldBumpProject(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("should return false when NewChangelogCreated is false and unreleased is empty", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte(emptyChangelog), 0o644))

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			BuildProjectConfig()
		ctx := &commands.RepoContext{
			ProjectConfig:       projectConfig,
			NewChangelogCreated: false,
		}

		// when
		result, err := commands.ShouldBumpProject(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("should return true when unreleased section has real entries", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte(populatedChangelog), 0o644))

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			BuildProjectConfig()
		ctx := &commands.RepoContext{
			ProjectConfig:       projectConfig,
			NewChangelogCreated: false,
		}

		// when
		result, err := commands.ShouldBumpProject(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.True(t, result)
	})
}

func TestGetLatestTagForChangelog(t *testing.T) {
	t.Parallel()

	testAuthor := &object.Signature{Name: "Test", Email: "test@test.com"}

	// initTestRepo creates a git repo with a single commit and returns the repo and commit hash.
	initTestRepo := func(t *testing.T) (*git.Repository, plumbing.Hash) {
		t.Helper()
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)
		wt, err := repo.Worktree()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("hello"), 0o644))
		_, err = wt.Add("README.md")
		require.NoError(t, err)
		hash, err := wt.Commit("initial commit", &git.CommitOptions{Author: testAuthor})
		require.NoError(t, err)
		return repo, hash
	}

	// addCommits appends n dummy commits to the repo and returns the last commit hash.
	addCommits := func(t *testing.T, repo *git.Repository, n int) plumbing.Hash {
		t.Helper()
		wt, err := repo.Worktree()
		require.NoError(t, err)
		tmpDir := wt.Filesystem.Root()
		var lastHash plumbing.Hash
		for i := range n {
			fName := fmt.Sprintf("file-%d.txt", i)
			require.NoError(t, os.WriteFile(filepath.Join(tmpDir, fName), []byte(fName), 0o644))
			_, err = wt.Add(fName)
			require.NoError(t, err)
			lastHash, err = wt.Commit(fmt.Sprintf("commit %d", i), &git.CommitOptions{Author: testAuthor})
			require.NoError(t, err)
		}
		return lastHash
	}

	t.Run("should return nil when repository has no tags", func(t *testing.T) {
		t.Parallel()

		// given
		repo, _ := initTestRepo(t)

		// when
		result, err := commands.GetLatestTagForChangelog(repo)

		// then
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should return tag info when repository has a lightweight tag", func(t *testing.T) {
		t.Parallel()

		// given
		repo, commitHash := initTestRepo(t)
		_, err := repo.CreateTag("v2.0.0", commitHash, nil)
		require.NoError(t, err)

		// when
		result, err := commands.GetLatestTagForChangelog(repo)

		// then
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "2.0.0", result.Tag.String())
	})

	t.Run("should dereference annotated tag via ResolveLatestAnnotatedTag", func(t *testing.T) {
		t.Parallel()

		// given
		repo, commitHash := initTestRepo(t)
		_, err := repo.CreateTag("v1.2.3", commitHash, &git.CreateTagOptions{
			Message: "release v1.2.3",
			Tagger:  testAuthor,
		})
		require.NoError(t, err)

		// when
		result, err := commands.ResolveLatestAnnotatedTag(repo)

		// then
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "1.2.3", result.Tag.String())
	})

	t.Run("should return nil when ResolveLatestAnnotatedTag is called with only lightweight tags", func(t *testing.T) {
		t.Parallel()

		// given
		repo, commitHash := initTestRepo(t)
		_, err := repo.CreateTag("v3.0.0", commitHash, nil)
		require.NoError(t, err)

		// when
		result, err := commands.ResolveLatestAnnotatedTag(repo)

		// then
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should resolve annotated tag through GetLatestTagForChangelog fallback", func(t *testing.T) {
		t.Parallel()

		// given — repo needs 5+ commits so GetLatestTag doesn't return ErrNoTagsFound early
		repo, _ := initTestRepo(t)
		lastHash := addCommits(t, repo, 5)
		_, err := repo.CreateTag("v4.0.0", lastHash, &git.CreateTagOptions{
			Message: "release v4.0.0",
			Tagger:  testAuthor,
		})
		require.NoError(t, err)

		// when — GetLatestTag will fail on annotated tag (CommitObject on tag hash),
		// then fall back to resolveLatestAnnotatedTag
		result, err := commands.GetLatestTagForChangelog(repo)

		// then
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "4.0.0", result.Tag.String())
	})

	t.Run("should return highest annotated tag when repo has mixed lightweight and annotated tags", func(t *testing.T) {
		t.Parallel()

		// given
		repo, commitHash := initTestRepo(t)
		// v3.0.0 as lightweight (highest semver but NOT annotated)
		_, err := repo.CreateTag("v3.0.0", commitHash, nil)
		require.NoError(t, err)
		// v2.0.0 as annotated (lower semver but annotated)
		_, err = repo.CreateTag("v2.0.0", commitHash, &git.CreateTagOptions{
			Message: "release v2.0.0",
			Tagger:  testAuthor,
		})
		require.NoError(t, err)

		// when
		result, err := commands.ResolveLatestAnnotatedTag(repo)

		// then — should return v2.0.0 (the highest annotated tag), not nil
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "2.0.0", result.Tag.String())
	})
}
