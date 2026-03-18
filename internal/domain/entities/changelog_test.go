//go:build unit

package entities_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

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
}
