package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGlobalConfig(t *testing.T) {
	t.Run("should validate successfully when all required fields are present", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: "/home/user/test", ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
			GpgKeyPath:        "/home/user/.gnupg/autobump.asc",
			GitLabAccessToken: faker.Password(),
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.NoError(t, err, "should not return an error for valid config")
	})

	t.Run("should return error when projects are missing in batch mode", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			LanguagesConfig: map[string]LanguageConfig{"Go": {}},
		}

		// when
		err := validateGlobalConfig(&globalConfig, true)

		// then
		require.ErrorIs(t, err, ErrConfigKeyMissingError, "should return ErrConfigKeyMissingError")
	})

	t.Run("should return error when project path is missing", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: "", ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig: map[string]LanguageConfig{"Go": {}},
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.ErrorIs(t, err, ErrConfigKeyMissingError, "should return ErrConfigKeyMissingError for missing path")
	})

	t.Run(
		"should return error when project access token is missing in batch mode without global token",
		func(t *testing.T) {
			// given
			globalConfig := GlobalConfig{
				Projects: []ProjectConfig{
					{Path: faker.Word(), ProjectAccessToken: ""},
				},
				LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
				GitLabAccessToken: "",
			}

			// when
			err := validateGlobalConfig(&globalConfig, true)

			// then
			require.ErrorIs(
				t,
				err,
				ErrConfigKeyMissingError,
				"should return ErrConfigKeyMissingError for missing access token",
			)
		},
	)

	t.Run("should return error when languages config is missing", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: faker.Word(), ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig: nil,
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.ErrorIs(t, err, ErrLanguagesKeyMissingError, "should return ErrLanguagesKeyMissingError")
	})
}

// createMinimalConfig is a helper function to create a minimal test config file.
func createMinimalConfig(t *testing.T, additionalConfig string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".autobump.yaml")
	configContent := additionalConfig + `
projects:
  - path: /test/project
languages:
  Go:
    extensions: [".go"]
    special_patterns: ["go.mod"]
    version_files:
      - path: "go.mod"
        patterns: ["^module "]
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)
	return configPath
}

func TestReadConfig_GitHubTokenFromEnvironment(t *testing.T) {
	t.Run("should read GitHub token from GITHUB_TOKEN environment variable", func(t *testing.T) {
		// given
		expectedToken := faker.Password()
		t.Setenv("GITHUB_TOKEN", expectedToken)
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedToken, config.GitHubAccessToken, "should read token from GITHUB_TOKEN")
	})

	t.Run("should read GitHub token from GH_TOKEN environment variable", func(t *testing.T) {
		// given
		expectedToken := faker.Password()
		t.Setenv("GH_TOKEN", expectedToken)
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedToken, config.GitHubAccessToken, "should read token from GH_TOKEN")
	})

	t.Run("should prefer config file token over environment variable", func(t *testing.T) {
		// given
		configToken := faker.Password()
		envToken := faker.Password()
		t.Setenv("GITHUB_TOKEN", envToken)
		configPath := createMinimalConfig(t, "github_access_token: "+configToken+"\n")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, configToken, config.GitHubAccessToken, "should prefer config file token")
		assert.NotEqual(t, envToken, config.GitHubAccessToken, "should not use env token when config has one")
	})

	t.Run("should prefer GITHUB_TOKEN over GH_TOKEN", func(t *testing.T) {
		// given
		githubToken := faker.Password()
		ghToken := faker.Password()
		t.Setenv("GITHUB_TOKEN", githubToken)
		t.Setenv("GH_TOKEN", ghToken)
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, githubToken, config.GitHubAccessToken, "should prefer GITHUB_TOKEN over GH_TOKEN")
	})

	t.Run("should handle empty environment variables gracefully", func(t *testing.T) {
		// given
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Empty(t, config.GitHubAccessToken, "should have empty token when env vars are empty")
	})
}

func TestReadConfig_AzureDevOpsTokenFromEnvironment(t *testing.T) {
	t.Run("should read Azure DevOps token from SYSTEM_ACCESSTOKEN environment variable", func(t *testing.T) {
		// given
		expectedToken := faker.Password()
		t.Setenv("SYSTEM_ACCESSTOKEN", expectedToken)
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedToken, config.AzureDevOpsAccessToken, "should read token from SYSTEM_ACCESSTOKEN")
	})

	t.Run("should prefer config file token over environment variable", func(t *testing.T) {
		// given
		configToken := faker.Password()
		envToken := faker.Password()
		t.Setenv("SYSTEM_ACCESSTOKEN", envToken)
		configPath := createMinimalConfig(t, "azure_devops_access_token: "+configToken+"\n")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, configToken, config.AzureDevOpsAccessToken, "should prefer config file token")
		assert.NotEqual(t, envToken, config.AzureDevOpsAccessToken, "should not use env token when config has one")
	})

	t.Run("should handle empty environment variables gracefully", func(t *testing.T) {
		// given
		t.Setenv("SYSTEM_ACCESSTOKEN", "")
		configPath := createMinimalConfig(t, "")

		// when
		config, err := readConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Empty(t, config.AzureDevOpsAccessToken, "should have empty token when env var is empty")
	})
}
