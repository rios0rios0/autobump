//go:build unit

package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func TestDetectProjectLanguage(t *testing.T) {
	t.Parallel()

	t.Run("should detect language by special pattern", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))

		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {
					SpecialPatterns: []string{"go.mod"},
					Extensions:      []string{"go"},
				},
			},
		}

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

		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"python": {
					Extensions: []string{"py"},
				},
			},
		}

		// when
		language, err := commands.DetectProjectLanguage(globalConfig, tmpDir)

		// then
		require.NoError(t, err)
		assert.Equal(t, "python", language)
	})

	t.Run("should return error when no language is detected", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()

		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {
					SpecialPatterns: []string{"go.mod"},
					Extensions:      []string{"go"},
				},
			},
		}

		// when
		language, err := commands.DetectProjectLanguage(globalConfig, tmpDir)

		// then
		assert.ErrorIs(t, err, commands.ErrProjectLanguageNotRecognized)
		assert.Empty(t, language)
	})
}

func TestHasMatchingExtension(t *testing.T) {
	t.Parallel()

	t.Run("should return true when extension matches", func(t *testing.T) {
		// given
		filename := "main.go"
		extensions := []string{"go", "py"}

		// when
		result := commands.HasMatchingExtension(filename, extensions)

		// then
		assert.True(t, result)
	})

	t.Run("should return false when no extension matches", func(t *testing.T) {
		// given
		filename := "main.rs"
		extensions := []string{"go", "py"}

		// when
		result := commands.HasMatchingExtension(filename, extensions)

		// then
		assert.False(t, result)
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

func TestServiceTypeName(t *testing.T) {
	t.Parallel()

	t.Run("should return github for GITHUB type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.GITHUB

		// when
		name := commands.ServiceTypeName(serviceType)

		// then
		assert.Equal(t, "github", name)
	})

	t.Run("should return gitlab for GITLAB type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.GITLAB

		// when
		name := commands.ServiceTypeName(serviceType)

		// then
		assert.Equal(t, "gitlab", name)
	})

	t.Run("should return azuredevops for AZUREDEVOPS type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.AZUREDEVOPS

		// when
		name := commands.ServiceTypeName(serviceType)

		// then
		assert.Equal(t, "azuredevops", name)
	})

	t.Run("should return empty string for unknown type", func(t *testing.T) {
		// given
		serviceType := gitforgeEntities.UNKNOWN

		// when
		name := commands.ServiceTypeName(serviceType)

		// then
		assert.Empty(t, name)
	})
}

func TestResolveToken(t *testing.T) {
	t.Parallel()

	t.Run("should return project access token when set", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			GitHubAccessToken: "global-token",
		}
		projectConfig := &entities.ProjectConfig{
			ProjectAccessToken: "project-token",
		}

		// when
		token := commands.ResolveToken(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "project-token", token)
	})

	t.Run("should return GitHub global token when project token is empty", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			GitHubAccessToken: "github-global",
		}
		projectConfig := &entities.ProjectConfig{}

		// when
		token := commands.ResolveToken(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "github-global", token)
	})

	t.Run("should return GitLab CI job token as fallback", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			GitLabCIJobToken: "ci-job-token",
		}
		projectConfig := &entities.ProjectConfig{}

		// when
		token := commands.ResolveToken(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		assert.Equal(t, "ci-job-token", token)
	})

	t.Run("should return empty string for unknown service type", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{}
		projectConfig := &entities.ProjectConfig{}

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
		globalConfig := &entities.GlobalConfig{
			GitHubAccessToken: "global-github",
		}
		projectConfig := &entities.ProjectConfig{
			ProjectAccessToken: "project-token",
		}

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 2)
		assert.Equal(t, "project-token", tokens[0])
		assert.Equal(t, "global-github", tokens[1])
	})

	t.Run("should return GitLab access token and CI job token", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			GitLabAccessToken: "gitlab-pat",
			GitLabCIJobToken:  "ci-job-token",
		}
		projectConfig := &entities.ProjectConfig{}

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITLAB, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 2)
		assert.Equal(t, "gitlab-pat", tokens[0])
		assert.Equal(t, "ci-job-token", tokens[1])
	})

	t.Run("should return Azure DevOps token", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			AzureDevOpsAccessToken: "ado-token",
		}
		projectConfig := &entities.ProjectConfig{}

		// when
		tokens := commands.CollectTokens(gitforgeEntities.AZUREDEVOPS, globalConfig, projectConfig)

		// then
		require.Len(t, tokens, 1)
		assert.Equal(t, "ado-token", tokens[0])
	})

	t.Run("should return empty slice when no tokens configured", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{}
		projectConfig := &entities.ProjectConfig{}

		// when
		tokens := commands.CollectTokens(gitforgeEntities.GITHUB, globalConfig, projectConfig)

		// then
		assert.Empty(t, tokens)
	})

	t.Run("should return empty slice for unknown service type", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			GitHubAccessToken: "github-token",
		}
		projectConfig := &entities.ProjectConfig{}

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
		repo := gitforgeEntities.Repository{
			Name:         "my-repo",
			Organization: "my-org",
			RemoteURL:    "https://github.com/my-org/my-repo.git",
		}
		provCfg := entities.ProviderConfig{
			Type:  "github",
			Token: "test-token",
		}

		// when
		result := commands.RepoToProjectConfig(repo, provCfg)

		// then
		assert.Equal(t, "https://github.com/my-org/my-repo.git", result.Path)
		assert.Equal(t, "my-repo", result.Name)
		assert.Equal(t, "test-token", result.ProjectAccessToken)
	})
}

func TestProcessRepo(t *testing.T) {
	t.Parallel()

	t.Run("should return error when git repo cannot be opened", func(t *testing.T) {
		// given
		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		projectConfig := &entities.ProjectConfig{
			Path: "/nonexistent/path/that/does/not/exist",
		}

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		assert.Error(t, err)
	})
}

func TestDetectBySpecialPatterns(t *testing.T) {
	t.Parallel()

	t.Run("should return empty string when no patterns match", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {SpecialPatterns: []string{"go.mod"}},
			},
		}

		// when
		result := commands.DetectBySpecialPatterns(globalConfig, tmpDir)

		// then
		assert.Empty(t, result)
	})

	t.Run("should detect language when pattern matches", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))
		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {SpecialPatterns: []string{"go.mod"}},
			},
		}

		// when
		result := commands.DetectBySpecialPatterns(globalConfig, tmpDir)

		// then
		assert.Equal(t, "golang", result)
	})
}

func TestDetectByExtensions(t *testing.T) {
	t.Parallel()

	t.Run("should return empty string when no extensions match", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# test"), 0o644))
		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}

		// when
		result, err := commands.DetectByExtensions(globalConfig, tmpDir)

		// then
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("should detect language by file extension", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644))
		globalConfig := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}

		// when
		result, err := commands.DetectByExtensions(globalConfig, tmpDir)

		// then
		require.NoError(t, err)
		assert.Equal(t, "golang", result)
	})
}
