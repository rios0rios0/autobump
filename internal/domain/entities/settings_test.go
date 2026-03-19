//go:build unit

package entities_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestMergeLanguagesConfig(t *testing.T) {
	t.Parallel()

	defaults := map[string]entities.LanguageConfig{
		"typescript": {
			Extensions:      []string{"ts"},
			SpecialPatterns: []string{"package.json", "tsconfig.json", "yarn.lock"},
			VersionFiles: []entities.VersionFile{
				{Path: "package.json", Patterns: []string{`(\s*"version":\s*")\d+\.\d+\.\d+(",)`}},
			},
		},
		"golang": {
			Extensions:      []string{"go"},
			SpecialPatterns: []string{"go.mod"},
		},
		"python": {
			Extensions:      []string{"py"},
			SpecialPatterns: []string{"pyproject.toml", "setup.cfg", "setup.py"},
			VersionFiles: []entities.VersionFile{
				{Path: "{project_name}/__init__.py", Patterns: []string{`(__version__\s*=\s*")\d+\.\d+\.\d+(")`}},
			},
		},
	}

	t.Run("should keep all defaults when user config is empty map", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Equal(t, defaults, result)
	})

	t.Run("should append user version files to default version files", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "opensearch_dashboards.json", Patterns: []string{`(\s*"version":\s*")\d+\.\d+\.\d+(",)`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts"}, ts.Extensions)
		assert.Equal(t, []string{"package.json", "tsconfig.json", "yarn.lock"}, ts.SpecialPatterns)
		assert.Len(t, ts.VersionFiles, 2)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		assert.Equal(t, "opensearch_dashboards.json", ts.VersionFiles[1].Path)
	})

	t.Run("should override default version file when user provides same path", func(t *testing.T) {
		// given
		customPattern := `(\s*"version":\s*")\d+\.\d+\.\d+\.\d+(",)`
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "package.json", Patterns: []string{customPattern}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Len(t, ts.VersionFiles, 1)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		assert.Equal(t, []string{customPattern}, ts.VersionFiles[0].Patterns)
	})

	t.Run("should keep default extensions when user provides only version files", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "manifest.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts"}, ts.Extensions)
		assert.Equal(t, []string{"package.json", "tsconfig.json", "yarn.lock"}, ts.SpecialPatterns)
	})

	t.Run("should add new language not present in defaults", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"ruby": {
				Extensions:      []string{"rb"},
				SpecialPatterns: []string{"Gemfile"},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Contains(t, result, "ruby")
		assert.Equal(t, []string{"rb"}, result["ruby"].Extensions)
		assert.Contains(t, result, "typescript")
		assert.Contains(t, result, "golang")
		assert.Contains(t, result, "python")
	})

	t.Run("should keep default language untouched when not in user config", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "extra.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Equal(t, defaults["golang"], result["golang"])
		assert.Equal(t, defaults["python"], result["python"])
	})

	t.Run("should deduplicate special patterns when user repeats default values", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				SpecialPatterns: []string{"package.json", "webpack.config.js"},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"package.json", "tsconfig.json", "yarn.lock", "webpack.config.js"}, ts.SpecialPatterns)
	})

	t.Run("should deduplicate extensions when user repeats default values", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				Extensions: []string{"ts", "tsx"},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts", "tsx"}, ts.Extensions)
	})
}

func TestFindProjectConfigFile(t *testing.T) {
	t.Parallel()

	t.Run("should find .autobump.yaml in project directory", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, configPath, result)
	})

	t.Run("should find .autobump.yml in project directory", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, configPath, result)
	})

	t.Run("should find autobump.yaml in project directory", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "autobump.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, configPath, result)
	})

	t.Run("should find autobump.yml in project directory", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "autobump.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, configPath, result)
	})

	t.Run("should return empty string when no config file exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Empty(t, result)
	})

	t.Run("should prefer .autobump.yaml over .autobump.yml", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ".autobump.yaml")
		ymlPath := filepath.Join(tmpDir, ".autobump.yml")
		require.NoError(t, os.WriteFile(yamlPath, []byte("languages: {}"), 0o644))
		require.NoError(t, os.WriteFile(ymlPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, yamlPath, result)
	})

	t.Run("should prefer .autobump.yaml over autobump.yaml", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		dotPath := filepath.Join(tmpDir, ".autobump.yaml")
		plainPath := filepath.Join(tmpDir, "autobump.yaml")
		require.NoError(t, os.WriteFile(dotPath, []byte("languages: {}"), 0o644))
		require.NoError(t, os.WriteFile(plainPath, []byte("languages: {}"), 0o644))

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Equal(t, dotPath, result)
	})

	t.Run("should return empty string when directory does not exist", func(t *testing.T) {
		// given
		nonExistentDir := filepath.Join(t.TempDir(), "does-not-exist")

		// when
		result := entities.FindProjectConfigFile(nonExistentDir)

		// then
		assert.Empty(t, result)
	})
}

func TestReadProjectConfig(t *testing.T) {
	t.Parallel()

	t.Run("should decode valid YAML with languages section", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		content := "languages:\n  python:\n    extensions:\n      - 'py'\n"
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "python")
		assert.Equal(t, []string{"py"}, cfg.LanguagesConfig["python"].Extensions)
	})

	t.Run("should decode valid YAML without languages section", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		content := "github_access_token: 'some-token'\n"
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Nil(t, cfg.LanguagesConfig)
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		// given
		configPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should return error when file contains invalid YAML", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("invalid: [yaml: {broken"), 0o644))

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should ignore unknown fields in non-strict mode", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		content := "custom_unknown_field: 'value'\nlanguages:\n  go:\n    extensions:\n      - 'go'\n"
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "go")
	})

	t.Run("should correctly parse version files with regex patterns", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".autobump.yaml")
		content := `languages:
  typescript:
    version_files:
      - path: 'package.json'
        patterns:
          - '(\s*"version":\s*")\d+\.\d+\.\d+(",)'
`
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		ts := cfg.LanguagesConfig["typescript"]
		require.Len(t, ts.VersionFiles, 1)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		require.Len(t, ts.VersionFiles[0].Patterns, 1)
	})
}

func TestCopyGlobalConfigWithLanguageOverrides(t *testing.T) {
	t.Parallel()

	t.Run("should create a copy with merged languages without mutating original", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			GitHubAccessToken: "my-token",
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"python": {Extensions: []string{"py"}},
		}

		// when
		result := entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		assert.Contains(t, result.LanguagesConfig, "golang")
		assert.Contains(t, result.LanguagesConfig, "python")
		assert.Equal(t, "my-token", result.GitHubAccessToken)
		assert.NotContains(t, original.LanguagesConfig, "python")
		assert.Len(t, original.LanguagesConfig, 1)
	})

	t.Run("should preserve all non-language fields from original", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			GitHubAccessToken:      "gh-token",
			GitLabAccessToken:      "gl-token",
			AzureDevOpsAccessToken: "ado-token",
			GpgKeyPath:             "/path/to/key",
			GpgKeyPassphrase:       "passphrase",
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		overrides := map[string]entities.LanguageConfig{}

		// when
		result := entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		assert.Equal(t, "gh-token", result.GitHubAccessToken)
		assert.Equal(t, "gl-token", result.GitLabAccessToken)
		assert.Equal(t, "ado-token", result.AzureDevOpsAccessToken)
		assert.Equal(t, "/path/to/key", result.GpgKeyPath)
		assert.Equal(t, "passphrase", result.GpgKeyPassphrase)
	})

	t.Run("should handle empty overrides returning equivalent languages", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		overrides := map[string]entities.LanguageConfig{}

		// when
		result := entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		assert.Equal(t, original.LanguagesConfig, result.LanguagesConfig)
	})

	t.Run("should add new language not present in original", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"ruby": {Extensions: []string{"rb"}, SpecialPatterns: []string{"Gemfile"}},
		}

		// when
		result := entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		assert.Contains(t, result.LanguagesConfig, "golang")
		assert.Contains(t, result.LanguagesConfig, "ruby")
		assert.Equal(t, []string{"rb"}, result.LanguagesConfig["ruby"].Extensions)
	})

	t.Run("should merge version files for existing language", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {
					Extensions: []string{"ts"},
					VersionFiles: []entities.VersionFile{
						{Path: "package.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
					},
				},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "manifest.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
				},
			},
		}

		// when
		result := entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		ts := result.LanguagesConfig["typescript"]
		assert.Len(t, ts.VersionFiles, 2)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		assert.Equal(t, "manifest.json", ts.VersionFiles[1].Path)
		assert.Equal(t, []string{"ts"}, ts.Extensions)
	})

	t.Run("should not mutate the original LanguagesConfig map", func(t *testing.T) {
		// given
		original := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"golang": {Extensions: []string{"go"}},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"golang": {SpecialPatterns: []string{"go.sum"}},
			"python": {Extensions: []string{"py"}},
		}
		originalLangsCount := len(original.LanguagesConfig)

		// when
		_ = entities.CopyGlobalConfigWithLanguageOverrides(original, overrides)

		// then
		assert.Len(t, original.LanguagesConfig, originalLangsCount)
		assert.NotContains(t, original.LanguagesConfig, "python")
		assert.Empty(t, original.LanguagesConfig["golang"].SpecialPatterns)
	})
}
