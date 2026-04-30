//go:build unit

package commands_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestParseForkVersion(t *testing.T) {
	t.Parallel()

	t.Run("should parse 4-segment fork version when mode is fork-dot", func(t *testing.T) {
		// given
		input := "3.3.0.16"

		// when
		parsed, err := commands.ParseForkVersion(input, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		require.NotNil(t, parsed)
		assert.Equal(t, "3.3.0", parsed.Upstream)
		assert.Equal(t, ".", parsed.Separator)
		assert.Equal(t, 16, parsed.Fork)
	})

	t.Run("should parse dash-separated fork version when mode is fork-dash", func(t *testing.T) {
		// given
		input := "1.21.0-9"

		// when
		parsed, err := commands.ParseForkVersion(input, entities.VersioningForkDash)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0", parsed.Upstream)
		assert.Equal(t, "-", parsed.Separator)
		assert.Equal(t, 9, parsed.Fork)
	})

	t.Run("should accept either separator when mode is empty", func(t *testing.T) {
		// given
		dotInput := "v2.5.1.42"
		dashInput := "v2.5.1-42"

		// when
		dotParsed, dotErr := commands.ParseForkVersion(dotInput, "")
		dashParsed, dashErr := commands.ParseForkVersion(dashInput, "")

		// then
		require.NoError(t, dotErr)
		require.NoError(t, dashErr)
		assert.Equal(t, ".", dotParsed.Separator)
		assert.Equal(t, "-", dashParsed.Separator)
		assert.Equal(t, 42, dotParsed.Fork)
		assert.Equal(t, 42, dashParsed.Fork)
	})

	t.Run("should reject 4-segment version when mode is fork-dash", func(t *testing.T) {
		// given
		input := "3.3.0.16"

		// when
		_, err := commands.ParseForkVersion(input, entities.VersioningForkDash)

		// then
		require.Error(t, err)
		assert.True(t, errors.Is(err, commands.ErrInvalidForkVersion))
	})

	t.Run("should reject dash-separated version when mode is fork-dot", func(t *testing.T) {
		// given
		input := "1.21.0-9"

		// when
		_, err := commands.ParseForkVersion(input, entities.VersioningForkDot)

		// then
		require.Error(t, err)
		assert.True(t, errors.Is(err, commands.ErrInvalidForkVersion))
	})

	t.Run("should reject malformed version strings", func(t *testing.T) {
		// given
		bogusInputs := []string{
			"",
			"3.3.0",
			"3.3.0.",
			"3.3.0.x",
			"foo.bar.baz.1",
			"3.3.0.16-rc1",
		}

		for _, input := range bogusInputs {
			// when
			_, err := commands.ParseForkVersion(input, entities.VersioningForkDot)

			// then
			require.Errorf(t, err, "input %q should fail to parse", input)
		}
	})
}

func TestNextForkVersion(t *testing.T) {
	t.Parallel()

	t.Run("should bump only the trailing digit when mode is fork-dot", func(t *testing.T) {
		// given
		current := "3.3.0.16"

		// when
		next, err := commands.NextForkVersion(current, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", next)
	})

	t.Run("should bump only the trailing digit when mode is fork-dash", func(t *testing.T) {
		// given
		current := "1.21.0-9"

		// when
		next, err := commands.NextForkVersion(current, entities.VersioningForkDash)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0-10", next)
	})

	t.Run("should seed initial version when current is empty for fork-dot", func(t *testing.T) {
		// given / when
		next, err := commands.NextForkVersion("", entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, entities.InitialReleaseVersion+".1", next)
	})

	t.Run("should seed initial version when current is empty for fork-dash", func(t *testing.T) {
		// given / when
		next, err := commands.NextForkVersion("", entities.VersioningForkDash)

		// then
		require.NoError(t, err)
		assert.Equal(t, entities.InitialReleaseVersion+"-1", next)
	})

	t.Run("should fail when mode is not a fork mode", func(t *testing.T) {
		// given / when
		_, err := commands.NextForkVersion("1.0.0", entities.VersioningSemver)

		// then
		require.Error(t, err)
		assert.True(t, errors.Is(err, commands.ErrInvalidForkVersion))
	})
}

func TestIsForkVersioning(t *testing.T) {
	t.Parallel()

	t.Run("should return true for fork-dot and fork-dash", func(t *testing.T) {
		// given / when / then
		assert.True(t, commands.IsForkVersioning(entities.VersioningForkDot))
		assert.True(t, commands.IsForkVersioning(entities.VersioningForkDash))
	})

	t.Run("should return false for semver and unknown modes", func(t *testing.T) {
		// given / when / then
		assert.False(t, commands.IsForkVersioning(entities.VersioningSemver))
		assert.False(t, commands.IsForkVersioning(""))
		assert.False(t, commands.IsForkVersioning("calver"))
	})
}

func TestFindLatestForkVersion(t *testing.T) {
	t.Parallel()

	t.Run("should find latest fork-dot version while skipping unreleased", func(t *testing.T) {
		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"## [3.3.0.16] - 2026-04-20",
			"",
			"## [3.3.0.15] - 2026-03-25",
		}

		// when
		parsed, err := commands.FindLatestForkVersion(lines, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.16", parsed.String())
	})

	t.Run("should find latest fork-dash version while skipping unreleased", func(t *testing.T) {
		// given
		lines := []string{
			"## [Unreleased]",
			"",
			"## [1.21.0-9] - 2026-01-12",
			"",
			"## [1.21.0-8] - 2025-12-29",
		}

		// when
		parsed, err := commands.FindLatestForkVersion(lines, entities.VersioningForkDash)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0-9", parsed.String())
	})

	t.Run("should ignore headers that do not match the requested mode", func(t *testing.T) {
		// given
		lines := []string{
			"## [Unreleased]",
			"",
			"## [1.0.0] - 2026-01-01",
		}

		// when
		_, err := commands.FindLatestForkVersion(lines, entities.VersioningForkDot)

		// then
		require.Error(t, err)
		assert.True(t, errors.Is(err, commands.ErrNoForkVersionFound))
	})
}

func TestProcessForkChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should rewrite changelog with next fork-dot version", func(t *testing.T) {
		// given
		lines := []string{
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
		}

		// when
		nextVersion, newContent, err := commands.ProcessForkChangelog(lines, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", nextVersion)
		joined := strings.Join(newContent, "\n")
		assert.Contains(t, joined, "## [Unreleased]")
		assert.Contains(t, joined, "## [3.3.0.17] - ")
		assert.Contains(t, joined, "- fixed sidebar selected item background")
		assert.Contains(t, joined, "## [3.3.0.16] - 2026-04-20")
		// The previous version must still appear after the new release header.
		newReleaseIdx := strings.Index(joined, "## [3.3.0.17]")
		previousIdx := strings.Index(joined, "## [3.3.0.16]")
		assert.Less(t, newReleaseIdx, previousIdx, "new release header must appear above previous release")
	})

	t.Run("should rewrite changelog with next fork-dash version", func(t *testing.T) {
		// given
		lines := []string{
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
		}

		// when
		nextVersion, newContent, err := commands.ProcessForkChangelog(lines, entities.VersioningForkDash)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.21.0-10", nextVersion)
		joined := strings.Join(newContent, "\n")
		assert.Contains(t, joined, "## [1.21.0-10] - ")
		assert.Contains(t, joined, "- changed loading spinner color")
	})

	t.Run("should seed initial version when no prior fork version exists", func(t *testing.T) {
		// given
		lines := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added first feature",
		}

		// when
		nextVersion, newContent, err := commands.ProcessForkChangelog(lines, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, entities.InitialReleaseVersion+".1", nextVersion)
		joined := strings.Join(newContent, "\n")
		assert.Contains(t, joined, "## ["+nextVersion+"] - ")
	})

	t.Run("should keep changelog unchanged when unreleased section is empty", func(t *testing.T) {
		// given
		lines := []string{
			"## [Unreleased]",
			"",
			"## [3.3.0.16] - 2026-04-20",
			"",
			"### Fixed",
			"",
			"- raised job timeout",
		}

		// when
		nextVersion, newContent, err := commands.ProcessForkChangelog(lines, entities.VersioningForkDot)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.3.0.17", nextVersion)
		// Body of unreleased was empty so the file content should not gain a new release header.
		joined := strings.Join(newContent, "\n")
		assert.NotContains(t, joined, "## [3.3.0.17]")
	})

	t.Run("should fail when mode is not a fork mode", func(t *testing.T) {
		// given / when
		_, _, err := commands.ProcessForkChangelog([]string{"## [Unreleased]"}, entities.VersioningSemver)

		// then
		require.Error(t, err)
	})
}
