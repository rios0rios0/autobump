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
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

func TestGetNextVersionString(t *testing.T) {
	t.Parallel()

	t.Run("should compute next semver version when versioning is empty", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"## [2.3.0] - 2026-03-20",
			"",
			"### Added",
			"",
			"- added previous feature",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		next, err := commands.GetNextVersionString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.4.0", next)
	})

	t.Run("should compute next fork-dot version when project sets fork-dot", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Fixed",
			"",
			"- fixed sidebar selected item background",
			"",
			"## [3.3.0.16] - 2026-04-20",
			"",
			"### Fixed",
			"",
			"- raised job timeout",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithVersioning(entities.VersioningForkDot).
			BuildProjectConfig()

		// when
		next, err := commands.GetNextVersionString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", next)
	})

	t.Run("should compute next fork-dash version when global sets fork-dash", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed loading spinner color",
			"",
			"## [1.21.0-9] - 2026-01-12",
			"",
			"### Changed",
			"",
			"- changed link color",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithVersioning(entities.VersioningForkDash).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		next, err := commands.GetNextVersionString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0-10", next)
	})

	t.Run("should let project versioning override global", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"## [3.3.0.16] - 2026-04-20",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithVersioning(entities.VersioningSemver).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithVersioning(entities.VersioningForkDot).
			BuildProjectConfig()

		// when
		next, err := commands.GetNextVersionString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", next)
	})
}

func TestUpdateChangelogFileString(t *testing.T) {
	t.Parallel()

	t.Run("should rewrite the changelog using fork-dot mode", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Fixed",
			"",
			"- fixed sidebar selected item background",
			"",
			"## [3.3.0.16] - 2026-04-20",
			"",
			"### Fixed",
			"",
			"- raised job timeout",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithVersioning(entities.VersioningForkDot).
			BuildProjectConfig()

		// when
		next, err := commands.UpdateChangelogFileString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", next)

		raw, readErr := os.ReadFile(changelogPath)
		require.NoError(t, readErr)
		content := string(raw)
		assert.Contains(t, content, "## [Unreleased]")
		assert.Contains(t, content, "## [3.3.0.17] - ")
		assert.Contains(t, content, "## [3.3.0.16] - 2026-04-20")
		assert.Contains(t, content, "- fixed sidebar selected item background")
	})

	t.Run("should rewrite the changelog using fork-dash mode", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed loading spinner color",
			"",
			"## [1.21.0-9] - 2026-01-12",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithVersioning(entities.VersioningForkDash).
			BuildProjectConfig()

		// when
		next, err := commands.UpdateChangelogFileString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0-10", next)
	})

	t.Run("should fall through to semver when no fork mode is set", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"## [1.0.0] - 2026-01-01",
		})
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		next, err := commands.UpdateChangelogFileString(globalConfig, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", next)
	})
}

func TestLoadProjectConfigOverridesPropagatesVersioning(t *testing.T) {
	t.Parallel()

	t.Run("should set versioning from per-project config when project has none", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		writeProjectConfig(t, tmpDir, "versioning: 'fork-dot'\nchangelog_path: 'CHANGELOG_PROPRIETARY.md'\n")

		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		commands.LoadProjectConfigOverrides(globalConfig, projectConfig, tmpDir)

		// then
		assert.Equal(t, entities.VersioningForkDot, projectConfig.Versioning)
		assert.Equal(t, "CHANGELOG_PROPRIETARY.md", projectConfig.ChangelogPath)
	})

	t.Run("should keep project versioning when project already specifies one", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		writeProjectConfig(t, tmpDir, "versioning: 'fork-dot'\n")

		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithVersioning(entities.VersioningForkDash).
			BuildProjectConfig()

		// when
		commands.LoadProjectConfigOverrides(globalConfig, projectConfig, tmpDir)

		// then
		assert.Equal(t, entities.VersioningForkDash, projectConfig.Versioning)
	})

	t.Run("should leave config unchanged when no .autobump.yaml file exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		result := commands.LoadProjectConfigOverrides(globalConfig, projectConfig, tmpDir)

		// then
		assert.Same(t, globalConfig, result)
		assert.Empty(t, projectConfig.Versioning)
		assert.Empty(t, projectConfig.ChangelogPath)
	})
}

func writeProjectConfig(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, ".autobump.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}
