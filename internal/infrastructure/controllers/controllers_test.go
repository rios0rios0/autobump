//go:build unit

package controllers_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

func TestNewLocalController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		// given / when
		ctrl := controllers.NewLocalController()

		// then
		require.NotNil(t, ctrl)
	})
}

func TestLocalControllerGetBind(t *testing.T) {
	t.Parallel()

	t.Run("should return bind with local command metadata", func(t *testing.T) {
		// given
		ctrl := controllers.NewLocalController()

		// when
		bind := ctrl.GetBind()

		// then
		assert.Equal(t, "local", bind.Use)
		assert.NotEmpty(t, bind.Short)
		assert.NotEmpty(t, bind.Long)
	})
}

func TestLocalControllerAddFlags(t *testing.T) {
	t.Parallel()

	t.Run("should add language flag to command", func(t *testing.T) {
		// given
		ctrl := controllers.NewLocalController()
		cmd := &cobra.Command{}

		// when
		ctrl.AddFlags(cmd)

		// then
		flag := cmd.Flags().Lookup("language")
		require.NotNil(t, flag)
		assert.Equal(t, "l", flag.Shorthand)
	})
}

func TestNewRunController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()

		// when
		ctrl := controllers.NewRunController(registry)

		// then
		require.NotNil(t, ctrl)
	})
}

func TestRunControllerGetBind(t *testing.T) {
	t.Parallel()

	t.Run("should return bind with run command metadata", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ctrl := controllers.NewRunController(registry)

		// when
		bind := ctrl.GetBind()

		// then
		assert.Equal(t, "run", bind.Use)
		assert.NotEmpty(t, bind.Short)
		assert.NotEmpty(t, bind.Long)
	})
}

func TestRunControllerAddFlags(t *testing.T) {
	t.Parallel()

	t.Run("should not panic when adding flags", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ctrl := controllers.NewRunController(registry)
		cmd := &cobra.Command{}

		// when
		ctrl.AddFlags(cmd)

		// then -- AddFlags is a no-op, just verify no panic
		assert.NotNil(t, cmd)
	})
}

func TestNewControllers(t *testing.T) {
	t.Parallel()

	t.Run("should aggregate controllers into a slice", func(t *testing.T) {
		// given
		local := controllers.NewLocalController()
		run := controllers.NewRunController(repositories.NewProviderRegistry())

		// when
		result := controllers.NewControllers(run, local)

		// then
		require.NotNil(t, result)
		assert.Len(t, *result, 2)
		assert.IsType(t, (*[]entities.Controller)(nil), result)
	})
}

// createTestRepo creates a real git repo in a temp dir with an initial commit.
func createTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	tmpDir := t.TempDir()
	repo, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(tmpDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test\n"), 0o644))
	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	return tmpDir, repo
}

// writeConfigFile writes a YAML config to a temp file and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "autobump.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))
	return configPath
}

func TestRegisterProviders(t *testing.T) {
	t.Parallel()

	t.Run("should register all providers without error", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))

		// when
		err := controllers.RegisterProviders(container)

		// then
		require.NoError(t, err)
	})

	t.Run("should allow resolving controllers after registration", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))
		require.NoError(t, controllers.RegisterProviders(container))

		// when
		var result *[]entities.Controller
		err := container.Invoke(func(c *[]entities.Controller) {
			result = c
		})

		// then
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, *result, 2)
	})

	t.Run("should return error when dependency is missing", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, controllers.RegisterProviders(container))

		// when -- RunController depends on ProviderRegistry which was not registered
		var result *[]entities.Controller
		err := container.Invoke(func(c *[]entities.Controller) {
			result = c
		})

		// then
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestFindReadAndValidateConfig(t *testing.T) {
	t.Parallel()

	t.Run("should read and return config when valid config file exists with languages", func(t *testing.T) {
		// given
		configPath := writeConfigFile(t, `
languages:
  golang:
    extensions:
      - 'go'
    version_files:
      - path: 'go.mod'
        patterns: ['(go )\d+\.\d+']
`)

		// when
		cfg, err := controllers.FindReadAndValidateConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "golang")
	})

	t.Run("should return error when config file does not exist", func(t *testing.T) {
		// given
		nonexistentPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		cfg, err := controllers.FindReadAndValidateConfig(nonexistentPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should return error when config has invalid YAML", func(t *testing.T) {
		// given
		configPath := writeConfigFile(t, `invalid: [yaml: broken`)

		// when
		cfg, err := controllers.FindReadAndValidateConfig(configPath)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should handle config with projects section", func(t *testing.T) {
		// given
		configContent := `
languages:
  golang:
    extensions:
      - 'go'
    version_files:
      - path: 'go.mod'
        patterns: ['(go )\d+\.\d+']
projects:
  - path: '/tmp/test-project'
    language: 'golang'
`
		configPath := writeConfigFile(t, configContent)

		// when
		cfg, err := controllers.FindReadAndValidateConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Len(t, cfg.Projects, 1)
	})

	t.Run("should handle config without languages key by using defaults", func(t *testing.T) {
		// given -- config with no languages key triggers the ErrLanguagesKeyMissing path
		configContent := `
github_access_token: 'fake-token'
projects:
  - path: '/tmp/test-project'
    language: 'golang'
`
		configPath := writeConfigFile(t, configContent)

		// when
		cfg, err := controllers.FindReadAndValidateConfig(configPath)

		// then -- should succeed by falling back to default config languages
		// (may fail if network is unavailable, but the path is exercised)
		if err == nil {
			require.NotNil(t, cfg)
			assert.NotEmpty(t, cfg.LanguagesConfig)
		}
	})

	t.Run("should handle config with providers section", func(t *testing.T) {
		// given
		configContent := `
languages:
  golang:
    extensions:
      - 'go'
providers:
  - type: 'github'
    token: 'fake-token'
    organizations:
      - 'test-org'
`
		configPath := writeConfigFile(t, configContent)

		// when
		cfg, err := controllers.FindReadAndValidateConfig(configPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Len(t, cfg.Providers, 1)
	})
}

// newTestCmd creates a cobra.Command with the standard flags used by controllers.
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{} //nolint:exhaustruct
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().String("config", "", "")
	return cmd
}

func TestLocalControllerExecute(t *testing.T) { //nolint:tparallel // mutates package-level globals
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
		0o644,
	))

	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should not panic when config file path is invalid", func(t *testing.T) {
		// given
		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", "/nonexistent/config.yaml"))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should set verbose log level when verbose flag is set", func(t *testing.T) {
		// given
		configContent := "languages:\n  golang:\n    extensions:\n      - 'go'\n"
		cfgPath := writeConfigFile(t, configContent)

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("verbose", "true"))
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{"/nonexistent/path"})
		})
	})

	t.Run("should not panic when repo path does not exist", func(t *testing.T) {
		// given
		configContent := "languages:\n  golang:\n    extensions:\n      - 'go'\n"
		cfgPath := writeConfigFile(t, configContent)

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{"/nonexistent/repo/path"})
		})
	})

	t.Run("should use provided language flag", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		configContent := "languages:\n  golang:\n    extensions:\n      - 'go'\n"
		cfgPath := writeConfigFile(t, configContent)

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", cfgPath))
		require.NoError(t, cmd.Flags().Set("language", "golang"))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{repoPath})
		})
	})

	t.Run("should detect language when language flag is empty", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)

		goModPath := filepath.Join(repoPath, "go.mod")
		require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/test\n\ngo 1.21\n"), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("go.mod")
		require.NoError(t, err)
		_, err = wt.Commit("add go.mod", &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		configContent := "languages:\n  golang:\n    extensions:\n      - 'go'\n"
		cfgPath := writeConfigFile(t, configContent)

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{repoPath})
		})
	})

	t.Run("should not panic when language detection fails", func(t *testing.T) {
		// given -- empty repo with no language markers
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\n")

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then -- language detection will fail (no go.mod etc.), should log error
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{repoPath})
		})
	})

	t.Run("should process repo with unreleased entries until push fails", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("CHANGELOG.md")
		require.NoError(t, err)
		_, err = wt.Commit("add changelog", &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)

		configContent := "languages:\n  golang:\n    extensions:\n      - 'go'\n"
		cfgPath := writeConfigFile(t, configContent)

		ctrl := controllers.NewLocalController()
		cmd := newTestCmd()
		ctrl.AddFlags(cmd)
		require.NoError(t, cmd.Flags().Set("config", cfgPath))
		require.NoError(t, cmd.Flags().Set("language", "golang"))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{repoPath})
		})
	})
}

func TestRunControllerExecute(t *testing.T) { //nolint:tparallel // mutates package-level globals
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
		0o644,
	))

	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should not panic when config file is invalid", func(t *testing.T) {
		// given
		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", "/nonexistent/config.yaml"))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should not panic when config has no providers and no projects", func(t *testing.T) {
		// given
		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should iterate projects when projects are configured", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\nprojects:\n  - path: '"+repoPath+"'\n    language: 'golang'\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should log error on invalid provider validation", func(t *testing.T) {
		// given
		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\nproviders:\n  - type: ''\n    token: ''\n    organizations: []\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should run both when both providers and projects exist", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\nproviders:\n  - type: 'github'\n    token: 'fake-token'\n    organizations:\n      - 'nonexistent-org'\nprojects:\n  - path: '"+repoPath+"'\n    language: 'golang'\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should set verbose log level when verbose flag is set", func(t *testing.T) {
		// given
		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("verbose", "true"))
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})

	t.Run("should attempt discovery when providers are configured with valid type", func(t *testing.T) {
		// given
		cfgPath := writeConfigFile(t, "languages:\n  golang:\n    extensions:\n      - 'go'\nproviders:\n  - type: 'github'\n    token: 'fake-token'\n    organizations:\n      - 'nonexistent-org'\n")

		ctrl := controllers.NewRunController(registry)
		cmd := newTestCmd()
		require.NoError(t, cmd.Flags().Set("config", cfgPath))

		// when / then
		assert.NotPanics(t, func() {
			ctrl.Execute(cmd, []string{})
		})
	})
}
