package controllers_test

import (
	"errors"
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

func TestNewRootController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		t.Parallel()

		// given / when
		ctrl := controllers.NewRootController()

		// then
		require.NotNil(t, ctrl)
	})
}

func TestRootControllerAddFlags(t *testing.T) {
	t.Parallel()

	t.Run("should add the language flag and bound the positional path", func(t *testing.T) {
		t.Parallel()

		// given -- these belong to the root command now, which is why AddFlags also sets
		// Args: one definition rather than the two the `local` subcommand used to need
		ctrl := controllers.NewRootController()
		cmd := &cobra.Command{}

		// when
		ctrl.AddFlags(cmd)

		// then
		flag := cmd.Flags().Lookup("language")
		require.NotNil(t, flag)
		assert.Equal(t, "l", flag.Shorthand)
		require.NotNil(t, cmd.Args)
		require.NoError(t, cmd.Args(cmd, []string{"."}))
		assert.Error(t, cmd.Args(cmd, []string{"one", "two"}))
	})
}

func TestNewRunController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

		// given
		run := controllers.NewRunController(repositories.NewProviderRegistry())
		selfUpdate := controllers.NewSelfUpdateController(commands.NewSelfUpdateCommand(
			func(_, _ bool) error { return nil },
		))
		version := controllers.NewVersionController(commands.NewVersionCommand())

		// when
		result := controllers.NewControllers(run, selfUpdate, version)

		// then -- RootController is deliberately absent: this slice is what becomes
		// subcommands, and there is no `local` subcommand any more
		require.NotNil(t, result)
		assert.Len(t, *result, 3)
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
		t.Parallel()

		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))

		// when
		err := controllers.RegisterProviders(container)

		// then
		require.NoError(t, err)
	})

	t.Run("should allow resolving controllers after registration", func(t *testing.T) {
		t.Parallel()

		// given
		container := dig.New()
		require.NoError(t, container.Provide(func() commands.SelfUpdateRunnerFunc {
			return func(_, _ bool) error { return nil }
		}))
		require.NoError(t, commands.RegisterProviders(container))
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
		assert.Len(t, *result, 3, "run, self-update and version -- RootController is not a subcommand")
	})

	t.Run("should return error when dependency is missing", func(t *testing.T) {
		t.Parallel()

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

// errOffline stands in for an unreachable GitHub.
var errOffline = errors.New("offline")

// offlineFetch is the published-defaults fetch for a machine with no network. Every test
// below uses it: the layering takes bytes, so none of it needs a network, and the old
// shape reached raw.githubusercontent.com on every single sub-test.
func offlineFetch(string) ([]byte, error) { return nil, errOffline }

func TestResolveConfigLayers(t *testing.T) {
	t.Parallel()

	t.Run("should fold the operator's file onto the built-in defaults", func(t *testing.T) {
		t.Parallel()

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
		cfg, err := controllers.ResolveWithFetch(configPath, offlineFetch)

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.LanguagesConfig, "golang")
		assert.Contains(t, cfg.LanguagesConfig, "typescript",
			"the built-in defaults are the base, so the languages the operator did not "+
				"mention must still be there")
	})

	t.Run("should return an error when the named config does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		nonexistentPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

		// when
		cfg, err := controllers.ResolveWithFetch(nonexistentPath, offlineFetch)

		// then
		require.Error(t, err, "a file the operator named by hand must be readable")
		assert.Nil(t, cfg)
	})

	t.Run("should return an error when the named config is invalid YAML", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := writeConfigFile(t, `invalid: [yaml: broken`)

		// when
		cfg, err := controllers.ResolveWithFetch(configPath, offlineFetch)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should return an error when the bump branch prefix is unusable", func(t *testing.T) {
		t.Parallel()

		// given -- caught at startup, before a single branch has been listed, let alone
		// deleted
		configPath := writeConfigFile(t, "bump_branch_prefix: 'chore/'\n")

		// when
		cfg, err := controllers.ResolveWithFetch(configPath, offlineFetch)

		// then
		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}

// These cannot join TestResolveConfigLayers above: they resolve the operator layer from
// $HOME, so they need t.Setenv to stop reading the developer's own configuration -- and
// t.Setenv is refused anywhere under a parallel ancestor.
func TestResolveConfigLayersFromHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	t.Run("should run on the built-in defaults alone", func(t *testing.T) {
		// given -- no -c, nothing in $HOME, and the published defaults unreachable
		// when
		cfg, err := controllers.ResolveWithFetch("", offlineFetch)

		// then
		require.NoError(t, err, "no configuration of one's own is not an error")
		require.NotNil(t, cfg)
		assert.NotEmpty(t, cfg.LanguagesConfig)
	})

	t.Run("should fold the published defaults when they can be fetched", func(t *testing.T) {
		// given
		published := func(string) ([]byte, error) {
			return []byte("languages:\n  brandnew:\n    extensions: ['bn']\n"), nil
		}

		// when
		cfg, err := controllers.ResolveWithFetch("", published)

		// then
		require.NoError(t, err)
		assert.Contains(t, cfg.LanguagesConfig, "brandnew",
			"a language added on main must reach an installed binary without a release")
		assert.Contains(t, cfg.LanguagesConfig, "typescript")
	})

	t.Run("should ignore a credential the published defaults try to set", func(t *testing.T) {
		// given -- bytes fetched over the network are not the operator speaking
		published := func(string) ([]byte, error) {
			return []byte("github_access_token: 'ghp_from_the_internet'\n"), nil
		}

		// when
		cfg, err := controllers.ResolveWithFetch("", published)

		// then
		require.NoError(t, err)
		assert.Empty(t, cfg.GitHubAccessToken)
	})
}

func TestConfigLayerAssembly(t *testing.T) {
	t.Parallel()

	t.Run("should assemble the three operator-facing layers in order", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := writeConfigFile(t, "versioning: 'semver'\n")
		published := func(string) ([]byte, error) { return []byte("# empty\n"), nil }

		// when
		names, err := controllers.LayerNamesWithFetch(configPath, published)

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{
			entities.LayerBuiltInDefaults,
			entities.LayerPublishedDefaults,
			entities.LayerOperatorConfig,
		}, names)
	})

	t.Run("should omit the published defaults when they cannot be fetched", func(t *testing.T) {
		t.Parallel()

		// given
		configPath := writeConfigFile(t, "versioning: 'semver'\n")

		// when
		names, err := controllers.LayerNamesWithFetch(configPath, offlineFetch)

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{
			entities.LayerBuiltInDefaults,
			entities.LayerOperatorConfig,
		}, names)
	})
}

// newTestCmd builds the bare command the controllers read their flags from.
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{} //nolint:exhaustruct // a bare command is the point: the flags below are what the controllers read
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().String("config", "", "")
	return cmd
}

func TestRootControllerExecute(t *testing.T) {
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
		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

		ctrl := controllers.NewRootController()
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

// TestRunControllerExecute is deliberately not parallel: it mutates package-level globals that other tests read.
func TestRunControllerExecute(t *testing.T) {
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

	const languages = "languages:\n  golang:\n    extensions:\n      - 'go'\n"
	const githubProvider = "providers:\n  - type: 'github'\n    token: 'fake-token'\n" +
		"    organizations:\n      - 'nonexistent-org'\n"

	// Every case asserts the same thing, because it is the only thing Execute exposes:
	// it logs and returns rather than reporting failure to its caller, so "did not
	// panic" is the whole observable contract. What varies is the config it is handed.
	testCases := []struct {
		name string
		// config receives the path of a repository with a releasable changelog, for the
		// cases that need one to appear under `projects:`.
		config  func(repoPath string) string
		verbose bool
	}{
		{
			name:   "should not panic when config has no providers and no projects",
			config: func(string) string { return languages },
		},
		{
			name: "should iterate projects when projects are configured",
			config: func(repoPath string) string {
				return languages + "projects:\n  - path: '" + repoPath + "'\n    language: 'golang'\n"
			},
		},
		{
			name: "should log error on invalid provider validation",
			config: func(string) string {
				return languages + "providers:\n  - type: ''\n    token: ''\n    organizations: []\n"
			},
		},
		{
			name: "should run both when both providers and projects exist",
			config: func(repoPath string) string {
				return languages + githubProvider +
					"projects:\n  - path: '" + repoPath + "'\n    language: 'golang'\n"
			},
		},
		{
			name:    "should set verbose log level when verbose flag is set",
			config:  func(string) string { return languages },
			verbose: true,
		},
		{
			name:   "should attempt discovery when providers are configured with valid type",
			config: func(string) string { return languages + githubProvider },
		},
	}

	t.Run("should not panic when config file is invalid", func(t *testing.T) {
		// given
		cmd := newRunControllerCmd(t, "/nonexistent/config.yaml", false)

		// when / then
		assertRunControllerSurvives(t, registry, cmd)
	})

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// given
			repoPath, _ := createTestRepo(t)
			changelog := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n" +
				"### Added\n\n- added initial release\n"
			require.NoError(
				t,
				os.WriteFile(filepath.Join(repoPath, "CHANGELOG.md"), []byte(changelog), 0o644),
			)

			cfgPath := writeConfigFile(t, testCase.config(repoPath))
			cmd := newRunControllerCmd(t, cfgPath, testCase.verbose)

			// when / then
			assertRunControllerSurvives(t, registry, cmd)
		})
	}
}

// newRunControllerCmd builds the cobra command the RunController reads its flags from.
func newRunControllerCmd(t *testing.T, cfgPath string, verbose bool) *cobra.Command {
	t.Helper()

	cmd := newTestCmd()
	if verbose {
		require.NoError(t, cmd.Flags().Set("verbose", "true"))
	}
	require.NoError(t, cmd.Flags().Set("config", cfgPath))

	return cmd
}

// assertRunControllerSurvives runs the controller and asserts it did not panic, which is
// the only outcome Execute makes visible.
func assertRunControllerSurvives(
	t *testing.T, registry *repositories.ProviderRegistry, cmd *cobra.Command,
) {
	t.Helper()

	ctrl := controllers.NewRunController(registry)
	assert.NotPanics(t, func() {
		ctrl.Execute(cmd, []string{})
	})
}
