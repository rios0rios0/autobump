//go:build unit

package commands_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

func TestProcessRepoIntegration(t *testing.T) { //nolint:tparallel // mutates package-level globals

	t.Run("should return error when changelog_path escapes project root", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()
		projectConfig.ChangelogPath = "../../etc/passwd"

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid changelog_path")
	})

	t.Run("should skip when unreleased section is empty", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should create bump branch and update files when unreleased entries exist", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		// Commit the changelog so it survives branch switches
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("CHANGELOG.md")
		require.NoError(t, err)
		_, err = wt.Commit("add changelog", &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)

		registry := repositories.NewProviderRegistry()
		commands.SetProviderRegistry(registry)
		commands.SetGitOperations(gitInfra.NewGitOperations(registry))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()

		// when
		err = commands.ProcessRepo(globalConfig, projectConfig)

		// then — push will fail (no remote), but branch should be created and changelog updated
		// The error is expected because there's no remote to push to
		require.Error(t, err)

		// Verify the changelog was updated with the new version
		updatedChangelog, readErr := os.ReadFile(changelogPath)
		require.NoError(t, readErr)
		assert.Contains(t, string(updatedChangelog), "[1.1.0]")
	})
}

func TestSetGitOperations(t *testing.T) { //nolint:tparallel // mutates package-level globals

	t.Run("should not panic when setting git operations", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ops := gitInfra.NewGitOperations(registry)

		// when / then
		assert.NotPanics(t, func() {
			commands.SetGitOperations(ops)
		})
	})
}

func TestGetLanguageInterface(t *testing.T) {
	t.Parallel()

	t.Run("should return Python language interface when language is python", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "pyproject.toml"),
			[]byte("[project]\nname = \"test-project\"\n"),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"python": {
					Extensions:      []string{"py"},
					SpecialPatterns: []string{"pyproject.toml"},
					VersionFiles: []entities.VersionFile{
						{Path: "{project_name}/__init__.py", Patterns: []string{`(__version__\s*=\s*")\d+\.\d+\.\d+(")`}},
					},
				},
			}).BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("python").
			WithName("test-project").
			BuildProjectConfig()

		// when — getVersionFiles will use the language interface to get the project name
		versionFiles, err := commands.GetVersionFiles(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Len(t, versionFiles, 0) // file doesn't exist but no error
	})
}
