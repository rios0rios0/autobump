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
)

// makeDir creates a directory tree under a t.TempDir() root. Directories need their
// execute bit to be traversable at all, so 0700 is the tightest mode that works; the
// tree lives inside t.TempDir(), which is already 0700 and removed when the test ends.
func makeDir(t *testing.T, path string) string {
	t.Helper()
	// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
	require.NoError(t, os.MkdirAll(path, 0o700))
	return path
}

// writeFragment drops a chlog fragment into <dir>/.changes/unreleased/<name>.
func writeFragment(t *testing.T, dir, name, content string) string {
	t.Helper()
	unreleasedDir := makeDir(t, filepath.Join(dir, ".changes", "unreleased"))
	path := filepath.Join(unreleasedDir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func writeChlogConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chlog.yaml"), []byte(content), 0o600))
}

// readFragments detects the project's chlog configuration and reads its pending fragments,
// which is the pairing every fragment-reading test needs.
func readFragments(t *testing.T, dir string) ([]commands.ChlogFragment, error) {
	t.Helper()
	config, _, err := commands.DetectChlog(dir)
	require.NoError(t, err)
	return commands.ReadChlogFragments(dir, config)
}

// chlogBaseChangelog is a released changelog with an empty unreleased section — the state
// chlog leaves behind, since its pending changes live in the fragments instead.
func chlogBaseChangelog() []string {
	return []string{
		"# Changelog",
		"",
		"## [Unreleased]",
		"",
		"## [1.2.0] - 2026-01-01",
		"",
		"### Added",
		"",
		"- added the first release",
	}
}

func TestDetectChlog(t *testing.T) {
	t.Parallel()

	t.Run("should report chlog when only the fragment directory exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		makeDir(t, filepath.Join(tmpDir, ".changes", "unreleased"))

		// when
		config, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.True(t, usesChlog)
		assert.Equal(t, ".changes", config.ChangesDir)
		assert.Equal(t, "unreleased", config.UnreleasedDir)
		assert.Equal(t, "CHANGELOG.md", config.ChangelogPath)
	})

	t.Run("should report chlog when only the configuration file exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changelogPath: docs/CHANGELOG.md\n")

		// when
		config, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.True(t, usesChlog)
		assert.Equal(t, "docs/CHANGELOG.md", config.ChangelogPath)
	})

	t.Run("should honour custom directories when the configuration overrides them", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changesDir: .notes\nunreleasedDir: pending\n")
		makeDir(t, filepath.Join(tmpDir, ".notes", "pending"))

		// when
		config, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.True(t, usesChlog)
		assert.Equal(t, filepath.Join(tmpDir, ".notes", "pending"), config.UnreleasedPath(tmpDir))
	})

	t.Run("should fall back to the defaults when the configuration omits keys", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "kinds:\n  - label: Added\n")

		// when
		config, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.True(t, usesChlog)
		assert.Equal(t, ".changes", config.ChangesDir)
		assert.Equal(t, "CHANGELOG.md", config.ChangelogPath)
	})

	t.Run("should not report chlog when the project uses neither marker", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()

		// when
		_, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.False(t, usesChlog)
	})

	t.Run("should return an error when the configuration is malformed", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changesDir: [this is not a string\n")

		// when
		_, _, err := commands.DetectChlog(tmpDir)

		// then
		require.Error(t, err)
	})

	t.Run("should reject a configured path that escapes the project root", func(t *testing.T) {
		t.Parallel()

		// given — .chlog.yaml is committed by the repository being released, and these
		// values drive globbing, reading and deletion, so they are untrusted input
		escaping := map[string]string{
			"absolute changesDir":     "changesDir: /etc\n",
			"parent changesDir":       "changesDir: ../../elsewhere\n",
			"parent unreleasedDir":    "unreleasedDir: ../../../etc\n",
			"absolute changelogPath":  "changelogPath: /etc/passwd\n",
			"escaping directory pair": "changesDir: .changes\nunreleasedDir: ../../..\n",
		}

		for name, config := range escaping {
			tmpDir := t.TempDir()
			writeChlogConfig(t, tmpDir, config)

			// when
			_, _, err := commands.DetectChlog(tmpDir)

			// then
			require.ErrorIsf(t, err, commands.ErrChlogPathEscapesProject, "case %q", name)
		}
	})

	t.Run("should not report chlog when the fragment path is a file rather than a directory", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		makeDir(t, filepath.Join(tmpDir, ".changes"))
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".changes", "unreleased"), []byte("not a directory\n"), 0o600))

		// when
		_, usesChlog, err := commands.DetectChlog(tmpDir)

		// then
		require.NoError(t, err)
		assert.False(t, usesChlog)
	})
}

func TestReadChlogFragments(t *testing.T) {
	t.Parallel()

	// The three ordering rules differ only in the fragments on disk and the order they
	// have to come back in, so they are a table rather than three copies of the same body.
	type writtenFragment struct{ name, content string }
	type expectedFragment struct{ kind, body string }

	orderingCases := []struct {
		name     string
		written  []writtenFragment
		expected []expectedFragment
	}{
		{
			name: "should order fragments by configured kind when several are pending",
			written: []writtenFragment{
				{"300-c3d4.yaml", "kind: Fixed\nbody: fixed the retry backoff\n"},
				{"100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n"},
			},
			expected: []expectedFragment{
				{"Added", "added OAuth2 login"},
				{"Fixed", "fixed the retry backoff"},
			},
		},
		{
			name: "should order fragments by timestamp when they share a kind",
			written: []writtenFragment{
				{"100-a1b2.yaml", "kind: Added\nbody: added SSO support\ntime: 2026-07-21T10:00:00Z\n"},
				{"200-c3d4.yaml", "kind: Added\nbody: added OAuth2 login\ntime: 2026-07-20T10:00:00Z\n"},
			},
			expected: []expectedFragment{
				{"Added", "added OAuth2 login"},
				{"Added", "added SSO support"},
			},
		},
		{
			name: "should order an unconfigured kind last when kinds are mixed",
			written: []writtenFragment{
				{"100-a1b2.yaml", "kind: Performance\nbody: sped up the parser\n"},
				{"200-c3d4.yaml", "kind: Added\nbody: added OAuth2 login\n"},
			},
			expected: []expectedFragment{
				{"Added", "added OAuth2 login"},
				{"Performance", "sped up the parser"},
			},
		},
	}

	for _, testCase := range orderingCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given
			tmpDir := t.TempDir()
			for _, written := range testCase.written {
				writeFragment(t, tmpDir, written.name, written.content)
			}

			// when
			fragments, err := readFragments(t, tmpDir)

			// then
			require.NoError(t, err)
			require.Len(t, fragments, len(testCase.expected))
			for i, expected := range testCase.expected {
				assert.Equal(t, expected.kind, fragments[i].Kind)
				assert.Equal(t, expected.body, fragments[i].Body)
			}
		})
	}

	t.Run("should read .yml fragments when a fragment uses the short extension", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeFragment(t, tmpDir, "100-a1b2.yml", "kind: Added\nbody: added OAuth2 login\n")

		// when
		fragments, err := readFragments(t, tmpDir)

		// then
		require.NoError(t, err)
		require.Len(t, fragments, 1)
		assert.Equal(t, "added OAuth2 login", fragments[0].Body)
	})

	t.Run("should skip a fragment when it carries no body", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: \"\"\n")

		// when
		fragments, err := readFragments(t, tmpDir)

		// then
		require.NoError(t, err)
		assert.Empty(t, fragments)
	})

	t.Run("should skip a fragment when it is a symlink rather than a regular file", func(t *testing.T) {
		t.Parallel()

		// given — a repository can commit a symlink pointing anywhere on the host, and
		// fragment bodies are published verbatim, so following one would leak host files
		tmpDir := t.TempDir()
		secret := filepath.Join(tmpDir, "secret.txt")
		require.NoError(t, os.WriteFile(secret, []byte("kind: Added\nbody: leaked host content\n"), 0o600))
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		require.NoError(t, os.Symlink(
			secret, filepath.Join(tmpDir, ".changes", "unreleased", "200-evil.yaml")))

		// when
		fragments, err := readFragments(t, tmpDir)

		// then
		require.NoError(t, err)
		require.Len(t, fragments, 1)
		assert.Equal(t, "added OAuth2 login", fragments[0].Body)
	})

	t.Run("should return an error when a fragment is malformed", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: [Added\n")

		// when
		_, err := readFragments(t, tmpDir)

		// then
		require.Error(t, err)
	})

	t.Run("should return no fragments when the unreleased directory is empty", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		makeDir(t, filepath.Join(tmpDir, ".changes", "unreleased"))

		// when
		fragments, err := readFragments(t, tmpDir)

		// then
		require.NoError(t, err)
		assert.Empty(t, fragments)
	})

	t.Run("should refuse to run when chlog has batched but unmerged version files", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".changes", "v1.2.0.md"), []byte("## [1.2.0]\n"), 0o600))

		// when
		_, err := readFragments(t, tmpDir)

		// then
		require.ErrorIs(t, err, commands.ErrChlogPendingVersionFiles)
		assert.Contains(t, err.Error(), "v1.2.0.md")
		assert.Contains(t, err.Error(), "chlog merge")
	})
}

func TestChlogSectionForKind(t *testing.T) {
	t.Parallel()

	config := commands.DefaultChlogConfig()

	t.Run("should map every default kind to its Keep a Changelog section", func(t *testing.T) {
		t.Parallel()

		// given
		kinds := map[string]string{
			"Added": "Added", "Changed": "Changed", "Deprecated": "Deprecated",
			"Removed": "Removed", "Fixed": "Fixed", "Security": "Security",
		}

		for kind, expected := range kinds {
			// when
			section := commands.ChlogSectionForKind(kind, &config)

			// then
			assert.Equal(t, expected, section)
		}
	})

	t.Run("should normalise casing when a fragment kind is lower-cased", func(t *testing.T) {
		t.Parallel()

		// given / when
		section := commands.ChlogSectionForKind("  security  ", &config)

		// then
		assert.Equal(t, "Security", section)
	})

	t.Run("should fall back to Changed when the kind has no matching section", func(t *testing.T) {
		t.Parallel()

		// given
		custom := commands.ChlogConfig{Kinds: []commands.ChlogKind{{Label: "Performance"}}}

		// when
		section := commands.ChlogSectionForKind("Performance", &custom)

		// then
		assert.Equal(t, "Changed", section)
	})

	t.Run("should fall back to Changed when the fragment has no kind", func(t *testing.T) {
		t.Parallel()

		// given / when
		section := commands.ChlogSectionForKind("", &config)

		// then
		assert.Equal(t, "Changed", section)
	})
}

func TestRenderChlogFragments(t *testing.T) {
	t.Parallel()

	config := commands.DefaultChlogConfig()

	t.Run("should group fragments under one heading when they share a kind", func(t *testing.T) {
		t.Parallel()

		// given
		fragments := []commands.ChlogFragment{
			{Kind: "Added", Body: "added OAuth2 login"},
			{Kind: "Added", Body: "added SSO support"},
			{Kind: "Fixed", Body: "fixed the retry backoff"},
		}

		// when
		rendered := commands.RenderChlogFragments(fragments, &config)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added OAuth2 login",
			"- added SSO support",
			"",
			"### Fixed",
			"",
			"- fixed the retry backoff",
		}, rendered)
	})

	t.Run("should indent continuation lines when the body spans several lines", func(t *testing.T) {
		t.Parallel()

		// given
		fragments := []commands.ChlogFragment{
			{Kind: "Added", Body: "added OAuth2 login\nso that operators stop sharing passwords"},
		}

		// when
		rendered := commands.RenderChlogFragments(fragments, &config)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added OAuth2 login",
			"  so that operators stop sharing passwords",
		}, rendered)
	})

	t.Run("should not double the bullet when the body already carries one", func(t *testing.T) {
		t.Parallel()

		// given
		fragments := []commands.ChlogFragment{{Kind: "Fixed", Body: "- fixed the retry backoff"}}

		// when
		rendered := commands.RenderChlogFragments(fragments, &config)

		// then
		assert.Equal(t, "- fixed the retry backoff", rendered[len(rendered)-1])
	})

	t.Run("should render nothing when there are no fragments", func(t *testing.T) {
		t.Parallel()

		// given / when
		rendered := commands.RenderChlogFragments(nil, &config)

		// then
		assert.Empty(t, rendered)
	})
}

func TestMergeChlogIntoUnreleased(t *testing.T) {
	t.Parallel()

	t.Run("should fill the unreleased section when it is empty", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{"# Changelog", "", "## [Unreleased]", "", "## [1.0.0] - 2026-01-01"}
		fragmentLines := []string{"### Added", "", "- added OAuth2 login"}

		// when
		merged := commands.MergeChlogIntoUnreleased(lines, fragmentLines)

		// then
		joined := strings.Join(merged, "\n")
		assert.Contains(t, joined, "## [Unreleased]\n\n### Added\n\n- added OAuth2 login")
		assert.Contains(t, joined, "## [1.0.0] - 2026-01-01")
	})

	t.Run("should keep hand-written entries when both sources hold work", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog", "",
			"## [Unreleased]", "",
			"### Fixed", "",
			"- fixed the retry backoff", "",
			"## [1.0.0] - 2026-01-01",
		}
		fragmentLines := []string{"### Added", "", "- added OAuth2 login"}

		// when
		merged := commands.MergeChlogIntoUnreleased(lines, fragmentLines)

		// then
		joined := strings.Join(merged, "\n")
		assert.Contains(t, joined, "- fixed the retry backoff")
		assert.Contains(t, joined, "- added OAuth2 login")
	})

	t.Run("should create the unreleased section when the changelog has none", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{"# Changelog", "", "## [1.0.0] - 2026-01-01", "", "- added initial release"}
		fragmentLines := []string{"### Added", "", "- added OAuth2 login"}

		// when
		merged := commands.MergeChlogIntoUnreleased(lines, fragmentLines)

		// then
		joined := strings.Join(merged, "\n")
		assert.Contains(t, joined, "## [Unreleased]\n\n### Added\n\n- added OAuth2 login")
		assert.Less(t, strings.Index(joined, "## [Unreleased]"), strings.Index(joined, "## [1.0.0]"))
	})

	t.Run("should leave the changelog untouched when there are no fragments", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{"# Changelog", "", "## [Unreleased]"}

		// when
		merged := commands.MergeChlogIntoUnreleased(lines, nil)

		// then
		assert.Equal(t, lines, merged)
	})
}

func TestDeleteChlogFragments(t *testing.T) {
	t.Parallel()

	t.Run("should remove the files when fragments are consumed", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		first := writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		second := writeFragment(t, tmpDir, "200-c3d4.yaml", "kind: Fixed\nbody: fixed the backoff\n")

		// when
		deleted, err := commands.DeleteChlogFragments([]commands.ChlogFragment{
			{Path: first}, {Path: second},
		})

		// then
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{first, second}, deleted)
		assert.NoFileExists(t, first)
		assert.NoFileExists(t, second)
	})

	t.Run("should return an error when a fragment file is already gone", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		missing := filepath.Join(tmpDir, ".changes", "unreleased", "404-dead.yaml")

		// when
		_, err := commands.DeleteChlogFragments([]commands.ChlogFragment{{Path: missing}})

		// then
		require.Error(t, err)
	})
}

func TestReadChangelogLinesWithChlog(t *testing.T) {
	t.Parallel()

	baseChangelog := chlogBaseChangelog()

	t.Run("should splice fragments into the unreleased section when the project uses chlog", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, baseChangelog)
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")

		// when
		lines, err := commands.ReadChangelogLines(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.NoError(t, err)
		assert.Contains(t, strings.Join(lines, "\n"), "- added OAuth2 login")
	})

	t.Run("should return the file verbatim when the project does not use chlog", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, baseChangelog)

		// when
		lines, err := commands.ReadChangelogLines(nil, &entities.ProjectConfig{Path: tmpDir}, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, baseChangelog, lines)
	})

	t.Run("should ignore the fragments when detection is disabled", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, baseChangelog)
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		disabled := false

		// when
		lines, err := commands.ReadChangelogLines(
			&entities.GlobalConfig{DetectChlog: &disabled},
			&entities.ProjectConfig{Path: tmpDir},
			changelogPath,
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, baseChangelog, lines)
	})
}

func TestShouldBumpProjectWithChlog(t *testing.T) {
	t.Parallel()

	t.Run("should compute a minor bump when only chlog fragments hold the changes", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, chlogBaseChangelog())
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		writeFragment(t, tmpDir, "200-c3d4.yaml", "kind: Fixed\nbody: fixed the retry backoff\n")
		projectConfig := &entities.ProjectConfig{Path: tmpDir}

		// when
		version, err := commands.GetNextVersion(nil, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.3.0", version.String())
	})

	t.Run("should compute a major bump when a fragment marks a breaking change", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, chlogBaseChangelog())
		writeFragment(t, tmpDir, "100-a1b2.yaml",
			"kind: Changed\nbody: \"**BREAKING CHANGE:** dropped the v1 endpoint\"\n")
		projectConfig := &entities.ProjectConfig{Path: tmpDir}

		// when
		version, err := commands.GetNextVersion(nil, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", version.String())
	})

	t.Run("should write the fragment entries into the release when the changelog is updated", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		changelogPath := writeChangelog(t, tmpDir, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Fixed",
			"",
			"- fixed the retry backoff",
			"",
			"## [1.2.0] - 2026-01-01",
			"",
			"### Added",
			"",
			"- added the first release",
		})
		writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		projectConfig := &entities.ProjectConfig{Path: tmpDir}

		// when
		version, err := commands.UpdateChangelogFileString(nil, projectConfig, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.3.0", version)

		content, err := os.ReadFile(changelogPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "- added OAuth2 login")
		assert.Contains(t, string(content), "- fixed the retry backoff")
		assert.Contains(t, string(content), "## [1.3.0]")
	})
}

func TestResolveChangelogPathWithChlog(t *testing.T) {
	t.Parallel()

	t.Run("should use the chlog changelog path when the project declares one", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changelogPath: docs/CHANGELOG.md\n")
		ctx := &commands.RepoContext{ProjectConfig: &entities.ProjectConfig{Path: tmpDir}}

		// when
		resolved, err := commands.ResolveChangelogPath(ctx)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, "docs", "CHANGELOG.md"), resolved)
	})

	t.Run("should prefer changelog_path when both it and the chlog path are set", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changelogPath: docs/CHANGELOG.md\n")
		ctx := &commands.RepoContext{
			ProjectConfig: &entities.ProjectConfig{Path: tmpDir, ChangelogPath: "HISTORY.md"},
		}

		// when
		resolved, err := commands.ResolveChangelogPath(ctx)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, "HISTORY.md"), resolved)
	})

	t.Run("should reject a chlog path that escapes the project root", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		writeChlogConfig(t, tmpDir, "changelogPath: ../../etc/passwd\n")
		ctx := &commands.RepoContext{ProjectConfig: &entities.ProjectConfig{Path: tmpDir}}

		// when
		_, err := commands.ResolveChangelogPath(ctx)

		// then
		require.Error(t, err)
	})

	t.Run("should default to CHANGELOG.md when the project does not use chlog", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		ctx := &commands.RepoContext{ProjectConfig: &entities.ProjectConfig{Path: tmpDir}}

		// when
		resolved, err := commands.ResolveChangelogPath(ctx)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, "CHANGELOG.md"), resolved)
	})
}

func TestChlogEnabled(t *testing.T) {
	t.Parallel()

	t.Run("should be enabled when nothing is configured", func(t *testing.T) {
		t.Parallel()

		// given / when / then
		assert.True(t, entities.ChlogEnabled(nil, nil))
	})

	t.Run("should be disabled when the global config turns detection off", func(t *testing.T) {
		t.Parallel()

		// given
		disabled := false

		// when / then
		assert.False(t, entities.ChlogEnabled(&entities.GlobalConfig{DetectChlog: &disabled}, nil))
	})

	t.Run("should let the project override the global setting", func(t *testing.T) {
		t.Parallel()

		// given
		disabled := false
		enabled := true

		// when / then
		assert.True(t, entities.ChlogEnabled(
			&entities.GlobalConfig{DetectChlog: &disabled},
			&entities.ProjectConfig{DetectChlog: &enabled},
		))
	})
}
