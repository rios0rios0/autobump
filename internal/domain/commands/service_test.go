//go:build unit

package commands_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
	langEntities "github.com/rios0rios0/langforge/pkg/domain/entities"
)

// mockFileAccessProvider is a minimal FileAccessProvider for unit tests.
type mockFileAccessProvider struct {
	files map[string]string // path -> content; missing key means "not found"
}

func (m *mockFileAccessProvider) Name() string        { return "mock" }
func (m *mockFileAccessProvider) MatchesURL(_ string) bool { return false }
func (m *mockFileAccessProvider) AuthToken() string   { return "" }
func (m *mockFileAccessProvider) CloneURL(_ gitforgeEntities.Repository) string { return "" }
func (m *mockFileAccessProvider) DiscoverRepositories(
	_ context.Context, _ string,
) ([]gitforgeEntities.Repository, error) {
	return nil, nil
}
func (m *mockFileAccessProvider) CreatePullRequest(
	_ context.Context, _ gitforgeEntities.Repository, _ gitforgeEntities.PullRequestInput,
) (*gitforgeEntities.PullRequest, error) {
	return nil, nil
}
func (m *mockFileAccessProvider) PullRequestExists(
	_ context.Context, _ gitforgeEntities.Repository, _ string,
) (bool, error) {
	return false, nil
}
func (m *mockFileAccessProvider) GetFileContent(
	_ context.Context, _ gitforgeEntities.Repository, path string,
) (string, error) {
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return "", errors.New("file not found")
}
func (m *mockFileAccessProvider) ListFiles(
	_ context.Context, _ gitforgeEntities.Repository, _ string,
) ([]gitforgeEntities.File, error) {
	return nil, nil
}
func (m *mockFileAccessProvider) GetTags(
	_ context.Context, _ gitforgeEntities.Repository,
) ([]string, error) {
	return nil, nil
}
func (m *mockFileAccessProvider) HasFile(
	_ context.Context, _ gitforgeEntities.Repository, path string,
) bool {
	_, ok := m.files[path]
	return ok
}
func (m *mockFileAccessProvider) CreateBranchWithChanges(
	_ context.Context, _ gitforgeEntities.Repository, _ gitforgeEntities.BranchInput,
) error {
	return nil
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

func TestFetchPRTemplate(t *testing.T) {
	t.Parallel()

	repo := gitforgeEntities.Repository{Name: "test-repo", Organization: "test-org"}

	t.Run("should return bump-specific template when it exists", func(t *testing.T) {
		t.Parallel()

		// given
		provider := &mockFileAccessProvider{
			files: map[string]string{
				".github/pull_request_template/bump.md": "bump template content",
				".github/pull_request_template.md":      "default template content",
			},
		}

		// when
		content, found := commands.FetchPRTemplate(context.Background(), provider, repo)

		// then
		assert.True(t, found)
		assert.Equal(t, "bump template content", content)
	})

	t.Run("should fall back to default template when bump-specific template is missing", func(t *testing.T) {
		t.Parallel()

		// given
		provider := &mockFileAccessProvider{
			files: map[string]string{
				".github/pull_request_template.md": "default template content",
			},
		}

		// when
		content, found := commands.FetchPRTemplate(context.Background(), provider, repo)

		// then
		assert.True(t, found)
		assert.Equal(t, "default template content", content)
	})

	t.Run("should fall back to root-level template when .github templates are missing", func(t *testing.T) {
		t.Parallel()

		// given
		provider := &mockFileAccessProvider{
			files: map[string]string{
				"PULL_REQUEST_TEMPLATE.md": "root template content",
			},
		}

		// when
		content, found := commands.FetchPRTemplate(context.Background(), provider, repo)

		// then
		assert.True(t, found)
		assert.Equal(t, "root template content", content)
	})

	t.Run("should return not found when no template file exists", func(t *testing.T) {
		t.Parallel()

		// given
		provider := &mockFileAccessProvider{files: map[string]string{}}

		// when
		content, found := commands.FetchPRTemplate(context.Background(), provider, repo)

		// then
		assert.False(t, found)
		assert.Empty(t, content)
	})
}

func TestApplyTemplateVars(t *testing.T) {
	t.Parallel()

	t.Run("should replace version placeholder", func(t *testing.T) {
		t.Parallel()

		// given
		template := "Version: {{version}}"

		// when
		result := commands.ApplyTemplateVars(template, "1.2.3", "my-project")

		// then
		assert.Equal(t, "Version: 1.2.3", result)
	})

	t.Run("should replace project placeholder", func(t *testing.T) {
		t.Parallel()

		// given
		template := "Project: {{project}}"

		// when
		result := commands.ApplyTemplateVars(template, "1.2.3", "my-project")

		// then
		assert.Equal(t, "Project: my-project", result)
	})

	t.Run("should replace both version and project placeholders", func(t *testing.T) {
		t.Parallel()

		// given
		template := "Bumping {{project}} to {{version}}"

		// when
		result := commands.ApplyTemplateVars(template, "2.0.0", "autobump")

		// then
		assert.Equal(t, "Bumping autobump to 2.0.0", result)
	})

	t.Run("should leave template unchanged when no placeholders are present", func(t *testing.T) {
		t.Parallel()

		// given
		template := "No placeholders here"

		// when
		result := commands.ApplyTemplateVars(template, "1.0.0", "proj")

		// then
		assert.Equal(t, "No placeholders here", result)
	})
}
