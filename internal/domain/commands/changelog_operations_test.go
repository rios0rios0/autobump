package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

// writeChangelog writes lines as a CHANGELOG.md in dir, one per line and with a trailing
// newline, and returns its path. An empty slice writes an empty file, which is what the
// cases covering a missing or unreadable changelog rely on.
func writeChangelog(t *testing.T, dir string, lines []string) string {
	t.Helper()

	var content strings.Builder
	for _, line := range lines {
		content.WriteString(line)
		content.WriteString("\n")
	}

	path := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(path, []byte(content.String()), 0o644))

	return path
}

func TestShouldBumpProject(t *testing.T) {
	t.Parallel()

	// The two cases differ only in whether [Unreleased] has entries under it, which is
	// the entire question ShouldBumpProject answers.
	testCases := []struct {
		name       string
		unreleased []string
		expected   bool
	}{
		{
			name:       "should return true when unreleased section has entries",
			unreleased: []string{"### Added", "", "- added new feature", ""},
			expected:   true,
		},
		{
			name:       "should return false when unreleased section is empty",
			unreleased: nil,
			expected:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given
			lines := append([]string{"# Changelog", "", "## [Unreleased]", ""}, testCase.unreleased...)
			lines = append(lines, "## [1.0.0] - 2026-01-01", "", "### Added", "", "- added initial release")
			changelogPath := writeChangelog(t, t.TempDir(), lines)
			ctx := &commands.RepoContext{
				ProjectConfig: entitybuilders.NewProjectConfigBuilder().
					WithName("test-project").
					BuildProjectConfig(),
			}

			// when
			result, err := commands.ShouldBumpProject(ctx, changelogPath)

			// then
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, result)
		})
	}

	t.Run("should return error when changelog file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := &commands.RepoContext{
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when
		result, err := commands.ShouldBumpProject(ctx, "/nonexistent/CHANGELOG.md")

		// then
		require.Error(t, err)
		assert.False(t, result)
	})
}

func TestUpdateChangelogFile(t *testing.T) {
	t.Parallel()

	t.Run("should extract version and update changelog when unreleased entries exist", func(t *testing.T) {
		t.Parallel()

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
			"",
			"### Added",
			"",
			"- added initial release",
		})

		// when
		version, err := commands.UpdateChangelogFile(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, version)
		assert.Equal(t, "1.1.0", version.String())
	})

	t.Run("should return error when changelog has no unreleased entries", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"## [1.0.0] - 2026-01-01",
			"",
			"### Added",
			"",
			"- added initial release",
		})

		// when
		version, err := commands.UpdateChangelogFile(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.Error(t, err)
		assert.Nil(t, version)
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given / when
		version, err := commands.UpdateChangelogFile(nil, nil, "/nonexistent/CHANGELOG.md")

		// then
		require.Error(t, err)
		assert.Nil(t, version)
	})
}

func TestGetNextVersion(t *testing.T) {
	t.Parallel()

	t.Run("should return next minor version when changelog has existing versions", func(t *testing.T) {
		t.Parallel()

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

		// when
		version, err := commands.GetNextVersion(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.4.0", version.String())
	})

	t.Run("should return initial version when changelog has no versions", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added initial feature",
		})

		// when
		version, err := commands.GetNextVersion(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, entities.InitialReleaseVersion, version.String())
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given / when
		version, err := commands.GetNextVersion(nil, nil, "/nonexistent/CHANGELOG.md")

		// then
		require.Error(t, err)
		assert.Nil(t, version)
	})
}

func TestGeneratePRDescription(t *testing.T) {
	t.Parallel()

	t.Run("should generate description when version files are configured", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "pom.xml"),
			[]byte("<project><version>1.1.0</version></project>"),
			0o644,
		))
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{
					"java": {
						VersionFiles: []entities.VersionFile{
							{Path: "pom.xml", Patterns: []string{`(<version>)[^<]+(</version>)`}},
						},
					},
				}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(tmpDir).
				WithName("my-project").
				WithLanguage("java").
				WithNewVersion("1.2.0").
				BuildProjectConfig(),
		}

		// when
		result := commands.GeneratePRDescription(ctx)

		// then
		assert.Contains(t, result, "1.2.0")
		assert.Contains(t, result, "my-project")
		assert.Contains(t, result, "pom.xml")
		assert.Contains(t, result, "CHANGELOG.md")
		assert.Contains(t, result, "Verify version file updates")
	})

	t.Run("should generate description without version file checklist when no version files", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(t.TempDir()).
				WithName("go-project").
				WithLanguage("").
				WithNewVersion("2.0.0").
				BuildProjectConfig(),
		}

		// when
		result := commands.GeneratePRDescription(ctx)

		// then
		assert.Contains(t, result, "2.0.0")
		assert.Contains(t, result, "go-project")
		assert.NotContains(t, result, "Verify version file updates")
	})

	t.Run("should use custom changelog path when configured", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(t.TempDir()).
				WithName("project").
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}
		ctx.ProjectConfig.ChangelogPath = "docs/CHANGES.md"

		// when
		result := commands.GeneratePRDescription(ctx)

		// then
		assert.Contains(t, result, "docs/CHANGES.md")
		assert.NotContains(t, result, "CHANGELOG.md")
	})
}

func TestEnsureProjectLanguage(t *testing.T) {
	t.Parallel()

	t.Run("should detect language when not already set", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{
					"go": {SpecialPatterns: []string{"go.mod"}, Extensions: []string{"go"}},
				}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(tmpDir).
				WithLanguage("").
				BuildProjectConfig(),
		}

		// when
		commands.EnsureProjectLanguage(ctx)

		// then
		assert.NotEmpty(t, ctx.ProjectConfig.Language)
	})

	t.Run("should keep existing language when already set", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithLanguage("java").
				BuildProjectConfig(),
		}

		// when
		commands.EnsureProjectLanguage(ctx)

		// then
		assert.Equal(t, "java", ctx.ProjectConfig.Language)
	})

	t.Run("should set language to empty when detection fails", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(tmpDir).
				WithLanguage("").
				BuildProjectConfig(),
		}

		// when
		commands.EnsureProjectLanguage(ctx)

		// then
		assert.Empty(t, ctx.ProjectConfig.Language)
	})
}
