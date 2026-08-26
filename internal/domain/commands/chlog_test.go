package commands_test

import (
	"fmt"
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

	t.Run("should read the breaking flag when a fragment declares one", func(t *testing.T) {
		t.Parallel()

		// given the shape "chlog new --kind Changed --breaking --body ..." writes
		tmpDir := t.TempDir()
		writeFragment(t, tmpDir, "100-a1b2.yaml",
			"kind: 'Changed'\nbody: 'dropped the v1 endpoint'\nbreaking: true\ntime: '2026-07-20T10:00:00Z'\n")
		writeFragment(t, tmpDir, "200-c3d4.yaml", "kind: 'Added'\nbody: 'added OAuth2 login'\n")

		// when
		fragments, err := readFragments(t, tmpDir)

		// then
		require.NoError(t, err)
		require.Len(t, fragments, 2)
		assert.False(t, fragments[0].Breaking, "the Added fragment declares nothing")
		assert.True(t, fragments[1].Breaking)
	})

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

	// The marker rules differ only in the fragment written and the entry that has to come
	// out, so they are a table rather than half a dozen copies of the same body. Several
	// spellings have to render the same entry, so they share a case.
	markerCases := []struct {
		name     string
		kind     string
		bodies   []string
		breaking bool
		expected []string
	}{
		{
			name: "should announce a breaking change once however the body spells it",
			kind: "Changed",
			bodies: []string{
				"BREAKING CHANGE: dropped the v1 endpoint",
				"**BREAKING CHANGE:** dropped the v1 endpoint",
				"**BREAKING CHANGE**: dropped the v1 endpoint",
				"BREAKING CHANGE: BREAKING CHANGE: dropped the v1 endpoint",
			},
			breaking: true,
			expected: []string{"### Changed", "", "- **BREAKING CHANGE:** dropped the v1 endpoint"},
		},
		{
			// A fragment written by hand, or by a writer who forgot the flag.
			name:     "should mark the entry when only the body says it is breaking",
			kind:     "Changed",
			bodies:   []string{"BREAKING CHANGE: dropped the v1 endpoint"},
			expected: []string{"### Changed", "", "- **BREAKING CHANGE:** dropped the v1 endpoint"},
		},
		{
			// The flag is the only place chlog records it -- the tool never renders a marker,
			// and the bump is calculated from the rendered lines.
			name:     "should mark the entry when only the fragment flag says it is breaking",
			kind:     "Changed",
			bodies:   []string{"changed the configuration format"},
			breaking: true,
			expected: []string{"### Changed", "", "- **BREAKING CHANGE:** changed the configuration format"},
		},
		{
			name:     "should mark only the opening line when a breaking body spans several lines",
			kind:     "Changed",
			bodies:   []string{"dropped the v1 endpoint\nthe replacement is /v2/tokens"},
			breaking: true,
			expected: []string{
				"### Changed",
				"",
				"- **BREAKING CHANGE:** dropped the v1 endpoint",
				"  the replacement is /v2/tokens",
			},
		},
		{
			name:     "should leave an ordinary fragment untouched when nothing marks it",
			kind:     "Added",
			bodies:   []string{"added OAuth2 login"},
			expected: []string{"### Added", "", "- added OAuth2 login"},
		},
	}

	for _, testCase := range markerCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			for _, body := range testCase.bodies {
				// given
				fragment := commands.ChlogFragment{
					Kind: testCase.kind, Body: body, Breaking: testCase.breaking,
				}

				// when
				rendered := commands.RenderChlogFragments([]commands.ChlogFragment{fragment}, &config)

				// then
				assert.Equal(t, testCase.expected, rendered, "body %q", body)
			}
		})
	}
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

func TestKeepChlogUnreleasedDirectory(t *testing.T) {
	t.Parallel()

	config := commands.DefaultChlogConfig()

	t.Run("should create the placeholder when the directory has been emptied", func(t *testing.T) {
		t.Parallel()

		// given -- Git tracks files, not directories, so an emptied fragment directory
		// would vanish from the commit and take the layout chlog is detected by with it
		tmpDir := t.TempDir()
		makeDir(t, filepath.Join(tmpDir, ".changes", "unreleased"))

		// when
		kept, err := commands.KeepChlogUnreleasedDirectory(tmpDir, &config)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, ".changes", "unreleased", ".gitkeep"), kept)
		assert.FileExists(t, kept)
	})

	t.Run("should create the directory when the project has none yet", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()

		// when
		kept, err := commands.KeepChlogUnreleasedDirectory(tmpDir, &config)

		// then
		require.NoError(t, err)
		assert.FileExists(t, kept)
	})

	t.Run("should return the existing placeholder when one is already there", func(t *testing.T) {
		t.Parallel()

		// given -- a placeholder left by an aborted run is still untracked, so the path is
		// returned for staging even though nothing is written
		tmpDir := t.TempDir()
		unreleasedDir := makeDir(t, filepath.Join(tmpDir, ".changes", "unreleased"))
		keepPath := filepath.Join(unreleasedDir, ".gitkeep")
		require.NoError(t, os.WriteFile(keepPath, []byte("kept by hand\n"), 0o600))

		// when
		kept, err := commands.KeepChlogUnreleasedDirectory(tmpDir, &config)

		// then
		require.NoError(t, err)
		assert.Equal(t, keepPath, kept)
		content, readErr := os.ReadFile(keepPath)
		require.NoError(t, readErr)
		assert.Equal(t, "kept by hand\n", string(content))
	})

	t.Run("should leave the placeholder alone when it is not a regular file", func(t *testing.T) {
		t.Parallel()

		// given -- writing through a symlink the repository committed would touch a file
		// outside it, and the link itself already holds the directory open
		tmpDir := t.TempDir()
		outside := filepath.Join(tmpDir, "outside.txt")
		require.NoError(t, os.WriteFile(outside, []byte("host content\n"), 0o600))
		unreleasedDir := makeDir(t, filepath.Join(tmpDir, ".changes", "unreleased"))
		require.NoError(t, os.Symlink(outside, filepath.Join(unreleasedDir, ".gitkeep")))

		// when
		kept, err := commands.KeepChlogUnreleasedDirectory(tmpDir, &config)

		// then
		require.NoError(t, err)
		assert.Empty(t, kept)
		content, readErr := os.ReadFile(outside)
		require.NoError(t, readErr)
		assert.Equal(t, "host content\n", string(content))
	})

	t.Run("should honour the directories configured in .chlog.yaml", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		custom := commands.ChlogConfig{ChangesDir: "docs/changes", UnreleasedDir: "pending"}

		// when
		kept, err := commands.KeepChlogUnreleasedDirectory(tmpDir, &custom)

		// then
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, "docs", "changes", "pending", ".gitkeep"), kept)
		assert.FileExists(t, kept)
	})
}

func TestConsumeChlogFragments(t *testing.T) {
	t.Parallel()

	t.Run("should keep the directory detectable when every fragment is consumed", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		fragment := writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		ctx := &commands.RepoContext{ProjectConfig: &entities.ProjectConfig{Path: tmpDir}}

		// when
		consumed, err := commands.ConsumeChlogFragments(ctx)

		// then
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, []string{fragment}, consumed.Removed)
		assert.NoFileExists(t, fragment)
		assert.FileExists(t, consumed.Kept)

		// then -- the next run still recognises the project as a chlog user
		_, usesChlog, detectErr := commands.DetectChlog(tmpDir)
		require.NoError(t, detectErr)
		assert.True(t, usesChlog)
	})

	t.Run("should consume nothing when the project has no fragments", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		ctx := &commands.RepoContext{ProjectConfig: &entities.ProjectConfig{Path: tmpDir}}

		// when
		consumed, err := commands.ConsumeChlogFragments(ctx)

		// then
		require.NoError(t, err)
		assert.Nil(t, consumed)
		assert.NoDirExists(t, filepath.Join(tmpDir, ".changes", "unreleased"))
	})

	t.Run("should consume nothing when chlog detection is disabled", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		fragment := writeFragment(t, tmpDir, "100-a1b2.yaml", "kind: Added\nbody: added OAuth2 login\n")
		disabled := false
		ctx := &commands.RepoContext{
			ProjectConfig: &entities.ProjectConfig{Path: tmpDir, DetectChlog: &disabled},
		}

		// when
		consumed, err := commands.ConsumeChlogFragments(ctx)

		// then
		require.NoError(t, err)
		assert.Nil(t, consumed)
		assert.FileExists(t, fragment)
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

// releasedSectionOf returns the lines of the release section a bump just wrote, which is
// what the rules have to be asserted on: everything above it is the emptied [Unreleased]
// header and everything below it is history.
func releasedSectionOf(t *testing.T, changelogPath, version string) []string {
	t.Helper()

	content, err := os.ReadFile(changelogPath)
	require.NoError(t, err)

	var section []string
	inside := false
	for line := range strings.SplitSeq(string(content), "\n") {
		name, isHeader := entities.MatchChangelogVersionHeader(line)
		if isHeader {
			if inside {
				break
			}
			inside = strings.HasPrefix(name, version)
			continue
		}
		if inside {
			section = append(section, line)
		}
	}

	return section
}

// TestChlogFragmentChangelogRules covers the rules a release applies while compiling the
// pending fragments into CHANGELOG.md. Fragments are the case that needs them most: each is
// written alone, in its own file, so nobody ever sees the pending set side by side and
// nothing stops two branches from describing the same change twice, filing an entry under a
// kind its own verb contradicts, or spelling the breaking-change marker its own way.
//
// The cases differ only in what is on disk and what has to come out, so they are a table
// rather than a copy of the same body each. An empty `changelog` starts from the state chlog
// leaves behind, an empty `versioning` releases under SemVer, and an empty `version` is not
// asserted.
func TestChlogFragmentChangelogRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		changelog  []string
		fragments  []string
		versioning string
		version    string
		released   []string
	}{
		{
			name:      "should announce a breaking change once when the flag and the body both say it",
			fragments: []string{"kind: Changed\nbody: \"BREAKING CHANGE: dropped the v1 endpoint\"\nbreaking: true\n"},
			version:   "2.0.0",
			released: []string{
				"", "### Changed", "", "- **BREAKING CHANGE:** dropped the v1 endpoint", "",
			},
		},
		{
			// The flag is all chlog records -- it never renders a marker of its own.
			name:      "should release a major version when a fragment sets only the breaking flag",
			fragments: []string{"kind: Changed\nbody: changed the configuration format\nbreaking: true\n"},
			version:   "2.0.0",
			released: []string{
				"", "### Changed", "", "- **BREAKING CHANGE:** changed the configuration format", "",
			},
		},
		{
			// Two branches each wrote a fragment for the work they shared.
			name: "should publish one entry when two fragments describe the same change",
			fragments: []string{
				"kind: Added\nbody: added OAuth2 login\n",
				"kind: Added\nbody: added OAuth2 login\n",
			},
			released: []string{"", "### Added", "", "- added OAuth2 login", ""},
		},
		{
			name: "should publish the fuller entry when two fragments nearly overlap",
			fragments: []string{
				"kind: Added\nbody: added support for the new provider\n",
				"kind: Added\nbody: added support for the new provider adapter\n",
			},
			released: []string{"", "### Added", "", "- added support for the new provider adapter", ""},
		},
		{
			// A kind the writer picked that the body itself contradicts.
			name:      "should file the fragment under the section its verb names",
			fragments: []string{"kind: Changed\nbody: removed the deprecated helper\n"},
			released:  []string{"", "### Removed", "", "- removed the deprecated helper", ""},
		},
		{
			name: "should order the sections and the entries when many fragments are pending",
			fragments: []string{
				"kind: Fixed\nbody: fixed the retry backoff\n",
				"kind: Added\nbody: added SSO support\n",
				"kind: Added\nbody: added OAuth2 login\n",
			},
			released: []string{
				"",
				"### Added", "", "- added OAuth2 login", "- added SSO support",
				"",
				"### Fixed", "", "- fixed the retry backoff",
				"",
			},
		},
		{
			// A continuation line judged on its own would be filed under "### Removed",
			// orphaned from the bullet it explains.
			name: "should keep a multi-line fragment whole when a continuation opens with a verb",
			fragments: []string{
				"kind: Fixed\nbody: |\n  fixed the retry backoff\n  removed the exponential cap while doing so\n",
			},
			released: []string{
				"", "### Fixed", "",
				"- fixed the retry backoff",
				"  removed the exponential cap while doing so",
				"",
			},
		},
		{
			// A repository mid-migration to chlog, where both sources hold real work and the
			// hand-written entry repeats what a fragment already says. The mis-written
			// heading is repaired, both sources are kept, and the repeat goes.
			name: "should merge the fragments with the entries already written by hand",
			changelog: []string{
				"# Changelog", "",
				"## [Unreleased]", "",
				"#### added", "",
				"- added OAuth2 login",
				"- added the audit trail", "",
				"## [1.2.0] - 2026-01-01", "",
				"### Added", "",
				"- added the first release",
			},
			fragments: []string{"kind: Added\nbody: added OAuth2 login\n"},
			released: []string{
				"", "### Added", "", "- added OAuth2 login", "- added the audit trail", "",
			},
		},
		{
			// Fork mode rewrites the section without consulting the SemVer pipeline the
			// rules used to live inside.
			name: "should apply the rules when the project uses fork versioning",
			changelog: []string{
				"# Changelog", "",
				"## [Unreleased]", "",
				"#### added", "",
				"- added OAuth2 login", "",
				"## [3.3.0.16] - 2026-01-01", "",
				"### Added", "",
				"- added the first release",
			},
			fragments: []string{
				"kind: Added\nbody: added OAuth2 login\n",
				"kind: Changed\nbody: removed the deprecated helper\n",
			},
			versioning: entities.VersioningForkDot,
			version:    "3.3.0.17",
			released: []string{
				"",
				"### Added", "", "- added OAuth2 login",
				"",
				"### Removed", "", "- removed the deprecated helper",
				"",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given
			tmpDir := t.TempDir()
			baseline := testCase.changelog
			if baseline == nil {
				baseline = chlogBaseChangelog()
			}
			changelogPath := writeChangelog(t, tmpDir, baseline)
			for i, fragment := range testCase.fragments {
				writeFragment(t, tmpDir, fmt.Sprintf("%d00-a1b%d.yaml", i+1, i), fragment)
			}
			projectConfig := &entities.ProjectConfig{Path: tmpDir, Versioning: testCase.versioning}

			// when
			version, err := commands.UpdateChangelogFileString(nil, projectConfig, changelogPath)

			// then
			require.NoError(t, err)
			if testCase.version != "" {
				assert.Equal(t, testCase.version, version)
			}
			assert.Equal(t, testCase.released, releasedSectionOf(t, changelogPath, version))
		})
	}
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
