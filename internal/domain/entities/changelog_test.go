//go:build unit

package entities_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestSortChangelogEntries(t *testing.T) {
	t.Parallel()

	t.Run("should sort entries alphabetically within a single section", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- added feature Z",
			"- added feature A",
			"- added feature M",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added feature A",
			"- added feature M",
			"- added feature Z",
			"",
		}, result)
	})

	t.Run("should return entries unchanged when already sorted", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- added alpha",
			"- added beta",
			"- added gamma",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, lines, result)
	})

	t.Run("should return a single entry unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Fixed",
			"",
			"- fixed a bug",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, lines, result)
	})

	t.Run("should handle sections with no entries", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"### Changed",
			"",
			"- changed something",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, lines, result)
	})

	t.Run("should sort entries independently within each section", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- added zebra feature",
			"- added alpha feature",
			"",
			"### Changed",
			"",
			"- changed zulu behavior",
			"- changed bravo behavior",
			"",
			"### Fixed",
			"",
			"- fixed yankee bug",
			"- fixed charlie bug",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added alpha feature",
			"- added zebra feature",
			"",
			"### Changed",
			"",
			"- changed bravo behavior",
			"- changed zulu behavior",
			"",
			"### Fixed",
			"",
			"- fixed charlie bug",
			"- fixed yankee bug",
			"",
		}, result)
	})

	t.Run("should sort entries with backticks and special characters correctly", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- added `foo()` method",
			"- added `bar()` method",
			"- added `baz()` method",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added `bar()` method",
			"- added `baz()` method",
			"- added `foo()` method",
			"",
		}, result)
	})

	t.Run("should sort BREAKING CHANGE entries among other entries", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Changed",
			"",
			"- changed zulu config format",
			"- **BREAKING CHANGE:** removed legacy API endpoints",
			"- **BREAKING CHANGE:** altered authentication flow",
			"- changed alpha behavior",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Changed",
			"",
			"- **BREAKING CHANGE:** altered authentication flow",
			"- **BREAKING CHANGE:** removed legacy API endpoints",
			"- changed alpha behavior",
			"- changed zulu config format",
			"",
		}, result)
	})

	t.Run("should sort entries case-insensitively", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- Added Zulu feature",
			"- added alpha feature",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added alpha feature",
			"- Added Zulu feature",
			"",
		}, result)
	})

	t.Run("should sort entries in all version sections", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"## [2.0.0] - 2026-03-25",
			"",
			"### Added",
			"",
			"- added zulu feature",
			"- added alpha feature",
			"",
			"## [1.0.0] - 2026-01-01",
			"",
			"### Fixed",
			"",
			"- fixed zulu bug",
			"- fixed alpha bug",
			"",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"## [2.0.0] - 2026-03-25",
			"",
			"### Added",
			"",
			"- added alpha feature",
			"- added zulu feature",
			"",
			"## [1.0.0] - 2026-01-01",
			"",
			"### Fixed",
			"",
			"- fixed alpha bug",
			"- fixed zulu bug",
			"",
		}, result)
	})

	t.Run("should preserve comparison links at end of file", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"### Added",
			"",
			"- added zulu",
			"- added alpha",
			"",
			"[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD",
			"[1.0.0]: https://github.com/user/repo/releases/tag/v1.0.0",
		}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Equal(t, []string{
			"### Added",
			"",
			"- added alpha",
			"- added zulu",
			"",
			"[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD",
			"[1.0.0]: https://github.com/user/repo/releases/tag/v1.0.0",
		}, result)
	})

	t.Run("should return empty slice for empty input", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{}

		// when
		result := entities.SortChangelogEntries(lines)

		// then
		assert.Empty(t, result)
	})
}

func TestProcessChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should preserve all older versions when a changelog entry mentions [Unreleased]", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new dashboard widget",
			"",
			"## [3.2.0] - 2026-03-14",
			"",
			"### Added",
			"",
			"- added NVD database caching",
			"",
			"## [3.1.0] - 2026-03-12",
			"",
			"### Added",
			"",
			"- added changelog validation verifying entries are under the `[Unreleased]` section",
			"- added new container to support Golang version `1.26.0`",
			"",
			"### Changed",
			"",
			"- changed the delivery script for Go projects",
			"",
			"### Fixed",
			"",
			"- fixed changelog validation crashing when the changelog has no versioned sections (only `[Unreleased]`)",
			"",
			"## [3.0.0] - 2026-02-10",
			"",
			"### Changed",
			"",
			"- **BREAKING CHANGE:** removed legacy API endpoints",
			"",
			"## [2.0.0] - 2024-08-07",
			"",
			"### Added",
			"",
			"- added initial release",
		}

		// when
		version, content, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0", version.String())
		joined := strings.Join(content, "\n")
		assert.Contains(t, joined, "## [3.2.0]", "version 3.2.0 should be preserved")
		assert.Contains(t, joined, "## [3.1.0]", "version 3.1.0 should be preserved")
		assert.Contains(t, joined, "## [3.0.0]", "version 3.0.0 should be preserved")
		assert.Contains(t, joined, "## [2.0.0]", "version 2.0.0 should be preserved")
		assert.Contains(t, joined, "verifying entries are under the `[Unreleased]` section",
			"entry mentioning [Unreleased] should be preserved verbatim")
		assert.Contains(t, joined, "only `[Unreleased]`",
			"entry mentioning [Unreleased] in parentheses should be preserved")
	})

	t.Run("should preserve comparison links containing [Unreleased] at end of file", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Fixed",
			"",
			"- fixed a typo in the README",
			"",
			"## [1.2.0] - 2025-01-15",
			"",
			"### Added",
			"",
			"- added export functionality",
			"",
			"## [1.1.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- added import functionality",
			"",
			"[Unreleased]: https://github.com/user/repo/compare/v1.2.0...HEAD",
			"[1.2.0]: https://github.com/user/repo/compare/v1.1.0...v1.2.0",
			"[1.1.0]: https://github.com/user/repo/releases/tag/v1.1.0",
		}

		// when
		version, content, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.2.1", version.String())
		joined := strings.Join(content, "\n")
		assert.Contains(t, joined, "## [1.2.0]", "version 1.2.0 should be preserved")
		assert.Contains(t, joined, "## [1.1.0]", "version 1.1.0 should be preserved")
		assert.Contains(t, joined,
			"[Unreleased]: https://github.com/user/repo/compare/v1.2.0...HEAD",
			"comparison link with [Unreleased] should be preserved")
		assert.Contains(t, joined,
			"[1.1.0]: https://github.com/user/repo/releases/tag/v1.1.0",
			"comparison link for oldest version should be preserved")
	})

	t.Run("should return sorted entries after processing new changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added zulu feature",
			"- added alpha feature",
			"- added mike feature",
			"",
		}

		// when
		version, content, err := entities.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "0.1.0", version.String())

		joined := strings.Join(content, "\n")
		addedAlphaIdx := strings.Index(joined, "- added alpha feature")
		addedMikeIdx := strings.Index(joined, "- added mike feature")
		addedZuluIdx := strings.Index(joined, "- added zulu feature")
		require.NotEqual(t, -1, addedAlphaIdx, "alpha entry should exist")
		require.NotEqual(t, -1, addedMikeIdx, "mike entry should exist")
		require.NotEqual(t, -1, addedZuluIdx, "zulu entry should exist")
		assert.Less(t, addedAlphaIdx, addedMikeIdx,
			"alpha should appear before mike (sorted)")
		assert.Less(t, addedMikeIdx, addedZuluIdx,
			"mike should appear before zulu (sorted)")
	})

	t.Run("should return sorted entries after processing changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added zulu feature",
			"- added alpha feature",
			"- added mike feature",
			"",
			"### Fixed",
			"",
			"- fixed zulu bug",
			"- fixed alpha bug",
			"",
			"## [1.0.0] - 2026-01-01",
			"",
			"### Added",
			"",
			"- added initial release",
			"",
		}

		// when
		version, content, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", version.String())

		joined := strings.Join(content, "\n")
		addedAlphaIdx := strings.Index(joined, "- added alpha feature")
		addedMikeIdx := strings.Index(joined, "- added mike feature")
		addedZuluIdx := strings.Index(joined, "- added zulu feature")
		require.NotEqual(t, -1, addedAlphaIdx, "alpha entry should exist")
		require.NotEqual(t, -1, addedMikeIdx, "mike entry should exist")
		require.NotEqual(t, -1, addedZuluIdx, "zulu entry should exist")
		assert.Less(t, addedAlphaIdx, addedMikeIdx,
			"alpha should appear before mike (sorted)")
		assert.Less(t, addedMikeIdx, addedZuluIdx,
			"mike should appear before zulu (sorted)")

		fixedAlphaIdx := strings.Index(joined, "- fixed alpha bug")
		fixedZuluIdx := strings.Index(joined, "- fixed zulu bug")
		require.NotEqual(t, -1, fixedAlphaIdx, "fixed alpha entry should exist")
		require.NotEqual(t, -1, fixedZuluIdx, "fixed zulu entry should exist")
		assert.Less(t, fixedAlphaIdx, fixedZuluIdx,
			"fixed alpha should appear before fixed zulu (sorted)")
	})
}
