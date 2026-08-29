package entities_test

import (
	"os"
	"path/filepath"
	"strconv"
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

	t.Run("should turn a language's refresh on when a later layer sets it", func(t *testing.T) {
		t.Parallel()

		// given
		inherited := map[string]entities.LanguageConfig{
			"typescript": {Extensions: []string{"ts"}},
		}
		enabled := true
		overrides := map[string]entities.LanguageConfig{
			"typescript": {Refresh: &enabled},
		}

		// when
		result := entities.MergeLanguagesConfig(inherited, overrides)

		// then
		ts := result["typescript"]
		require.NotNil(t, ts.Refresh)
		assert.True(t, *ts.Refresh)
		assert.Equal(t, []string{"ts"}, ts.Extensions, "the inherited fields must survive")
	})

	t.Run("should turn a language's refresh off when a later layer clears it", func(t *testing.T) {
		t.Parallel()

		// given
		enabled := true
		disabled := false
		inherited := map[string]entities.LanguageConfig{
			"typescript": {Refresh: &enabled},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {Refresh: &disabled},
		}

		// when
		result := entities.MergeLanguagesConfig(inherited, overrides)

		// then
		ts := result["typescript"]
		require.NotNil(t, ts.Refresh, "an explicit false is a decision, not an omission")
		assert.False(t, *ts.Refresh)
	})

	t.Run("should keep the inherited refresh when a later layer omits it", func(t *testing.T) {
		t.Parallel()

		// given
		enabled := true
		inherited := map[string]entities.LanguageConfig{
			"typescript": {Refresh: &enabled},
		}
		overrides := map[string]entities.LanguageConfig{
			"typescript": {Extensions: []string{"tsx"}},
		}

		// when
		result := entities.MergeLanguagesConfig(inherited, overrides)

		// then
		ts := result["typescript"]
		require.NotNil(t, ts.Refresh)
		assert.True(t, *ts.Refresh)
	})
}

// writeNamedConfig writes a per-project config under name and returns its path.
func writeNamedConfig(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
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

func TestApplyRestrictedLayer(t *testing.T) {
	t.Parallel()

	t.Run("should decode a languages section", func(t *testing.T) {
		t.Parallel()

		// given
		layer := restrictedLayer("languages:\n  python:\n    extensions:\n      - 'py'\n")

		// when
		cfg, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"py"}, cfg.LanguagesConfig["python"].Extensions)
	})

	t.Run("should ignore a credential a repository tried to set", func(t *testing.T) {
		t.Parallel()

		// given
		base := &entities.GlobalConfig{GitHubAccessToken: "operator-token"}
		layer := restrictedLayer("github_access_token: 'repository-token'\n")

		// when
		cfg, err := entities.ApplyLayer(base, layer)

		// then
		require.NoError(t, err)
		assert.Equal(t, "operator-token", cfg.GitHubAccessToken,
			"a repository has no field to put a credential in, so there is no trust check "+
				"to get wrong -- the key simply has nowhere to land")
	})

	t.Run("should ignore every operator-only key", func(t *testing.T) {
		t.Parallel()

		// given -- the keys a repository's own file must never speak for
		base := &entities.GlobalConfig{
			GitLabAccessToken:      "operator-gitlab",
			GitHubAccessToken:      "operator-github",
			AzureDevOpsAccessToken: "operator-azure",
			GpgKeyPath:             "operator-gpg",
			SSHKeyPath:             "operator-ssh",
			BumpBranchPrefix:       "chore/bump-",
			Projects:               []entities.ProjectConfig{{Path: "/operator/repo"}},
		}
		layer := restrictedLayer(`
gitlab_access_token: 'repo-gitlab'
github_access_token: 'repo-github'
azure_devops_access_token: 'repo-azure'
gpg_key_path: 'repo-gpg'
gpg_key_passphrase: 'repo-gpg-pass'
ssh_key_path: 'repo-ssh'
ssh_key_passphrase: 'repo-ssh-pass'
ssh_auth_sock: 'repo-sock'
bump_branch_prefix: 'feat/'
providers:
  - type: 'github'
    token: 'repo-token'
    organizations: ['attacker']
projects:
  - path: '/repo/somewhere-else'
`)

		// when
		cfg, err := entities.ApplyLayer(base, layer)

		// then
		require.NoError(t, err)
		assert.Equal(t, "operator-gitlab", cfg.GitLabAccessToken)
		assert.Equal(t, "operator-github", cfg.GitHubAccessToken)
		assert.Equal(t, "operator-azure", cfg.AzureDevOpsAccessToken)
		assert.Equal(t, "operator-gpg", cfg.GpgKeyPath)
		assert.Empty(t, cfg.GpgKeyPassphrase)
		assert.Equal(t, "operator-ssh", cfg.SSHKeyPath)
		assert.Empty(t, cfg.SSHKeyPassphrase)
		assert.Empty(t, cfg.SSHAuthSock)
		assert.Equal(t, "chore/bump-", cfg.BumpBranchPrefix,
			"the prefix decides what stale-branch cleanup deletes")
		assert.Empty(t, cfg.Providers)
		assert.Equal(t, []entities.ProjectConfig{{Path: "/operator/repo"}}, cfg.Projects)
	})

	t.Run("should accept the settings a repository may speak for", func(t *testing.T) {
		t.Parallel()

		// given
		layer := restrictedLayer(`
changelog_path: 'CHANGELOG_PROPRIETARY.md'
versioning: 'fork-dot'
detect_chlog: false
cleanup_stale_branches: false
refresh: true
`)

		// when
		cfg, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.NoError(t, err)
		assert.Equal(t, "CHANGELOG_PROPRIETARY.md", cfg.ChangelogPath)
		assert.Equal(t, entities.VersioningForkDot, cfg.Versioning)
		require.NotNil(t, cfg.DetectChlog)
		assert.False(t, *cfg.DetectChlog)
		require.NotNil(t, cfg.CleanupStaleBranches)
		assert.False(t, *cfg.CleanupStaleBranches)
		require.NotNil(t, cfg.Refresh)
		assert.True(t, *cfg.Refresh)
	})

	t.Run("should return an error when the document is not valid YAML", func(t *testing.T) {
		t.Parallel()

		// given
		layer := restrictedLayer("invalid: [yaml: {broken")

		// when
		cfg, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should ignore an unknown key rather than refusing to run", func(t *testing.T) {
		t.Parallel()

		// given
		layer := restrictedLayer(
			"custom_unknown_field: 'value'\nlanguages:\n  go:\n    extensions:\n      - 'go'\n",
		)

		// when
		cfg, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.NoError(t, err)
		assert.Contains(t, cfg.LanguagesConfig, "go")
	})
}

// restrictedLayer builds the layer a repository's own configuration is applied as.
func restrictedLayer(content string) entities.ConfigLayer {
	//nolint:exhaustruct // Strict is false for a restricted layer by construction
	return entities.ConfigLayer{
		Name:     entities.LayerProjectConfig,
		Origin:   ".autobump.yaml",
		Data:     []byte(content),
		Scope:    entities.ScopeRestricted,
		Optional: true,
	}
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
}

func TestRefreshEnabled(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	t.Run("should be off when nothing sets it", func(t *testing.T) {
		t.Parallel()

		// given -- the refresh starts a package manager, so it is opt-in rather than opt-out
		global := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"typescript": {}},
		}

		// when
		result := entities.RefreshEnabled(global, &entities.ProjectConfig{}, "typescript")

		// then
		assert.False(t, result)
	})

	t.Run("should be on when the top level sets it", func(t *testing.T) {
		t.Parallel()

		// given
		global := &entities.GlobalConfig{Refresh: &enabled}

		// when
		result := entities.RefreshEnabled(global, &entities.ProjectConfig{}, "typescript")

		// then
		assert.True(t, result)
	})

	t.Run("should let the language override the top level", func(t *testing.T) {
		t.Parallel()

		// given
		global := &entities.GlobalConfig{
			Refresh: &disabled,
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {Refresh: &enabled},
			},
		}

		// when
		result := entities.RefreshEnabled(global, &entities.ProjectConfig{}, "typescript")

		// then
		assert.True(t, result)
	})

	t.Run("should let the project override the language", func(t *testing.T) {
		t.Parallel()

		// given
		global := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {Refresh: &enabled},
			},
		}

		// when
		result := entities.RefreshEnabled(global, &entities.ProjectConfig{Refresh: &disabled}, "typescript")

		// then
		assert.False(t, result, "an explicit false must beat a true below it")
	})

	t.Run("should be off for a language that does not set it", func(t *testing.T) {
		t.Parallel()

		// given
		global := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"typescript": {Refresh: &enabled},
			},
		}

		// when
		result := entities.RefreshEnabled(global, &entities.ProjectConfig{}, "golang")

		// then
		assert.False(t, result)
	})

	t.Run("should tolerate a nil configuration", func(t *testing.T) {
		t.Parallel()

		// when
		result := entities.RefreshEnabled(nil, nil, "typescript")

		// then
		assert.False(t, result)
	})
}

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

	t.Run("should return nil when the configuration is usable", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects: []entities.ProjectConfig{
				{Path: "/some/path"},
			},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg)

		// then
		assert.NoError(t, err)
	})

	t.Run("should accept a configuration with no languages of its own", func(t *testing.T) {
		t.Parallel()

		// given -- the built-in defaults are the base of every run, so an empty languages
		// map here means the layers were folded without them, not that the operator forgot
		cfg := &entities.GlobalConfig{LanguagesConfig: nil}

		// when
		err := entities.ValidateGlobalConfig(cfg)

		// then
		assert.NoError(t, err)
	})

	t.Run("should return an error when a project path is empty", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {}},
			Projects:        []entities.ProjectConfig{{Path: ""}},
		}

		// when
		err := entities.ValidateGlobalConfig(cfg)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "projects[0].path")
	})

	t.Run("should return an error when the bump branch prefix is unusable", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{BumpBranchPrefix: "chore/"}

		// when
		err := entities.ValidateGlobalConfig(cfg)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, entities.ErrBumpBranchPrefixInvalid)
	})
}

func TestValidateBumpBranchPrefix(t *testing.T) {
	t.Parallel()

	// The prefix is not only what new branches are named after -- it is the argument to a
	// destructive operation. cleanupStaleBumpBranches deletes every remote branch starting
	// with it and closes the pull request attached to each, so a prefix wider than the
	// operator meant does not produce a confusing branch name, it deletes other people's
	// work. An operator's typo is as capable of that as a hostile repository would be.
	accepted := []string{
		"",                  // unset means the default, which is valid by construction
		"chore/bump-",       // AutoBump's own
		"chore/autoupdate-", // AutoUpdate's, in the same namespace
		"release/autobump-",
		"a/b",
	}
	for _, prefix := range accepted {
		t.Run("should accept "+strconv.Quote(prefix), func(t *testing.T) {
			t.Parallel()

			// when
			err := entities.ValidateBumpBranchPrefix(prefix)

			// then
			assert.NoError(t, err)
		})
	}

	rejected := map[string]string{
		"an empty prefix matches every branch":          "   ",
		"a bare name can escape the namespace":          "bump-",
		"a protected branch name":                       "main",
		"another protected branch name":                 "MASTER",
		"a bare namespace sweeps every tool's branches": "chore/",
		"a refs/ prefix silently matches nothing":       "refs/heads/bump-",
		"a name git will not accept":                    "chore/bump ",
		"a double slash":                                "chore//bump-",
		"a parent traversal":                            "chore/../bump-",
		"a leading dash":                                "-chore/bump-",
		"a leading slash":                               "/chore/bump-",
		"a .lock suffix":                                "chore/bump.lock",
		"a glob character":                              "chore/bump-*",
		"a control character":                           "chore/bump-\x01",
	}
	for reason, prefix := range rejected {
		t.Run("should reject "+reason, func(t *testing.T) {
			t.Parallel()

			// when
			err := entities.ValidateBumpBranchPrefix(prefix)

			// then
			require.Error(t, err, "prefix %q must be rejected", prefix)
			assert.ErrorIs(t, err, entities.ErrBumpBranchPrefixInvalid)
		})
	}

	t.Run("should accept the default prefix", func(t *testing.T) {
		t.Parallel()

		// when
		err := entities.ValidateBumpBranchPrefix(entities.DefaultBumpBranchPrefix)

		// then
		assert.NoError(t, err, "the default must satisfy the rules it is offered as the fix for")
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

func TestFinalizeGlobalConfig(t *testing.T) {
	t.Parallel()

	t.Run("should derive a project name from its path", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{
			Projects: []entities.ProjectConfig{
				{Path: "https://gitlab.com/group/repo1.git"},
				{Path: "/home/user/repo2"},
				{Path: "/home/user/repo3", Name: "already-named"},
			},
		}

		// when
		entities.FinalizeGlobalConfig(cfg)

		// then
		assert.Equal(t, "repo1", cfg.Projects[0].Name)
		assert.Equal(t, "repo2", cfg.Projects[1].Name)
		assert.Equal(t, "already-named", cfg.Projects[2].Name)
	})

	t.Run("should read a token out of the file it names", func(t *testing.T) {
		t.Parallel()

		// given
		tokenPath := filepath.Join(t.TempDir(), "token.key")
		require.NoError(t, os.WriteFile(tokenPath, []byte("ghp_from_file\n"), 0o600))
		cfg := &entities.GlobalConfig{GitHubAccessToken: tokenPath}

		// when
		entities.FinalizeGlobalConfig(cfg)

		// then
		assert.Equal(t, "ghp_from_file", cfg.GitHubAccessToken)
	})

	t.Run("should leave an inline token alone", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := &entities.GlobalConfig{GitHubAccessToken: "ghp_inline"}

		// when
		entities.FinalizeGlobalConfig(cfg)

		// then
		assert.Equal(t, "ghp_inline", cfg.GitHubAccessToken)
	})
}

func TestReadLayerData(t *testing.T) {
	t.Parallel()

	t.Run("should read a layer from a file", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := filepath.Join(t.TempDir(), "autobump.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("versioning: 'fork-dot'\n"), 0o600))

		// when
		data, err := entities.ReadLayerData(configPath)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(data), "fork-dot")
	})

	t.Run("should return an error when the file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		data, err := entities.ReadLayerData(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestFindOperatorConfig(t *testing.T) {
	t.Parallel()

	t.Run("should return the path it was given", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := "/some/explicit/path.yaml"

		// when
		result := entities.FindOperatorConfig(configPath)

		// then
		assert.Equal(t, configPath, result)
	})
}

// These cases cannot join TestFindOperatorConfig above: it calls t.Parallel(), and t.Setenv
// and t.Chdir panic under a parallel parent.
func TestFindOperatorConfigSearchOrder(t *testing.T) {
	// The bug this ordering exists to prevent: AutoBump runs with the repository it is
	// releasing as the working directory, and that repository legitimately carries its own
	// `.autobump.yaml`. Reading it as the OPERATOR's configuration does not reorder a
	// preference, it substitutes a project's overrides for the operator's -- so the
	// project's settings stop being overrides and replace the layers beneath them.
	t.Run("should prefer the home directory", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		homeConfig := writeNamedConfig(t, home, ".autobump.yaml", "versioning: 'semver'\n")

		repo := t.TempDir()
		writeNamedConfig(t, repo, ".autobump.yaml", "versioning: 'fork-dot'\n")
		t.Chdir(repo)

		// when
		result := entities.FindOperatorConfig("")

		// then
		assert.Equal(t, homeConfig, result)
	})

	t.Run("should not adopt the project's own config as the operator's", func(t *testing.T) {
		// given -- an operator with nothing in $HOME, standing in the repository they are
		// releasing, which carries its own overrides
		t.Setenv("HOME", t.TempDir())

		repo := t.TempDir()
		writeNamedConfig(t, repo, ".autobump.yaml", "versioning: 'fork-dot'\n")
		t.Chdir(repo)

		// when
		result := entities.FindOperatorConfig("")

		// then
		assert.Empty(t, result,
			"a file in the working directory belongs to the project, and a project's config "+
				"is merged on top of the operator's rather than standing in for it")
	})

	// Every name and location the older, wider search also matched. None of them may come
	// back: each one is a way for the repository being released to answer a question that
	// is the operator's.
	workingDirectoryNames := []string{
		"autobump.yaml", ".config/autobump.yaml", "configs/autobump.yaml",
	}
	for _, name := range workingDirectoryNames {
		t.Run("should ignore "+name+" in the working directory", func(t *testing.T) {
			// given
			t.Setenv("HOME", t.TempDir())

			repo := t.TempDir()
			//nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o700))
			require.NoError(t, os.WriteFile(
				filepath.Join(repo, name), []byte("versioning: 'fork-dot'\n"), 0o600,
			))
			t.Chdir(repo)

			// when
			result := entities.FindOperatorConfig("")

			// then
			assert.Empty(t, result)
		})
	}

	t.Run("should report no operator configuration when the home has none", func(t *testing.T) {
		// given
		t.Setenv("HOME", t.TempDir())
		t.Chdir(t.TempDir())

		// when
		result := entities.FindOperatorConfig("")

		// then -- not an error: the built-in defaults are the base of every run
		assert.Empty(t, result)
	})

	t.Run("should return the named path without searching", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeNamedConfig(t, home, ".autobump.yaml", "versioning: 'semver'\n")

		// when
		result := entities.FindOperatorConfig("/explicit/path.yaml")

		// then
		assert.Equal(t, "/explicit/path.yaml", result)
	})
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
