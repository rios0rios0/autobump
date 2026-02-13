package domain_test

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/domain"
)

func TestNormalizeEntry(t *testing.T) {
	t.Parallel()

	t.Run("should strip leading dash when entry starts with dash", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- changed something important"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Equal(t, "changed something important", result)
	})

	t.Run("should strip backtick content when entry contains backticks", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- changed the Go version to `1.26.0` and updated all module dependencies"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.NotContains(t, result, "1.26.0")
		assert.NotContains(t, result, "`")
	})

	t.Run("should strip version numbers when entry contains semver versions", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- upgraded from 1.25.0 to 1.26.0"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.NotContains(t, result, "1.25.0")
		assert.NotContains(t, result, "1.26.0")
	})

	t.Run("should lowercase all characters when entry has mixed case", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- Changed the Go Module Dependencies"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Equal(t, "changed the go module dependencies", result)
	})

	t.Run("should collapse whitespace when entry has multiple spaces", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- changed   the   Go   version  to  `1.26.0`  and  updated"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.NotContains(t, result, "  ")
	})

	t.Run("should return empty string when entry is empty", func(t *testing.T) {
		t.Parallel()

		// given
		entry := ""

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return dash when entry is only a dash with space", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- "

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Equal(t, "-", result)
	})

	t.Run("should return dash when entry is padded dash with spaces", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "  - "

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Equal(t, "-", result)
	})

	t.Run("should return unchanged content when entry has no leading dash", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "changed something without dash"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.Equal(t, "changed something without dash", result)
	})

	t.Run("should strip all backtick-wrapped content when entry has multiple backtick pairs", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- upgraded `package-a` and `package-b` to latest versions"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.NotContains(t, result, "package-a")
		assert.NotContains(t, result, "package-b")
		assert.Contains(t, result, "upgraded")
		assert.Contains(t, result, "latest versions")
	})

	t.Run("should strip v-prefixed version when entry contains v-prefix version", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- bumped to v2.3.1"

		// when
		result := domain.NormalizeEntryForTest(entry)

		// then
		assert.NotContains(t, result, "v2.3.1")
		assert.NotContains(t, result, "2.3.1")
	})
}

func TestTokenize(t *testing.T) {
	t.Parallel()

	t.Run("should remove stop words when input contains common stop words", func(t *testing.T) {
		t.Parallel()

		// given
		input := "changed the go module dependencies to their latest versions"

		// when
		tokens := domain.TokenizeForTest(input)

		// then
		assert.NotContains(t, tokens, "the")
		assert.NotContains(t, tokens, "to")
		assert.NotContains(t, tokens, "their")
		assert.Contains(t, tokens, "changed")
		assert.Contains(t, tokens, "go")
		assert.Contains(t, tokens, "module")
	})

	t.Run("should remove single-character words when input has short words", func(t *testing.T) {
		t.Parallel()

		// given
		input := "a b c changed go"

		// when
		tokens := domain.TokenizeForTest(input)

		// then
		assert.NotContains(t, tokens, "b")
		assert.NotContains(t, tokens, "c")
		assert.Contains(t, tokens, "changed")
		assert.Contains(t, tokens, "go")
	})

	t.Run("should return empty slice when input is empty", func(t *testing.T) {
		t.Parallel()

		// given
		input := ""

		// when
		tokens := domain.TokenizeForTest(input)

		// then
		assert.Empty(t, tokens)
	})

	t.Run("should return empty slice when input contains only stop words", func(t *testing.T) {
		t.Parallel()

		// given
		input := "the to and all their"

		// when
		tokens := domain.TokenizeForTest(input)

		// then
		assert.Empty(t, tokens)
	})
}

func TestExtractMaxVersion(t *testing.T) {
	t.Parallel()

	t.Run("should return the version when entry contains a single version", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- changed the Go version to `1.26.0`"

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		require.NotNil(t, ver)
		assert.Equal(t, "1.26.0", ver.String())
	})

	t.Run("should return the highest version when entry contains multiple versions", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- upgraded from 1.25.0 to 1.26.0"

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		require.NotNil(t, ver)
		assert.Equal(t, "1.26.0", ver.String())
	})

	t.Run("should return nil when entry contains no version", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- changed the Go module dependencies to their latest versions"

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		assert.Nil(t, ver)
	})

	t.Run("should return version without prefix when entry contains v-prefixed version", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- bumped to v2.3.1"

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		require.NotNil(t, ver)
		assert.Equal(t, "2.3.1", ver.String())
	})

	t.Run("should return version with patch zero when entry contains two-digit version", func(t *testing.T) {
		t.Parallel()

		// given
		entry := "- updated dependency to 3.5"

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		require.NotNil(t, ver)
		assert.Equal(t, "3.5.0", ver.String())
	})

	t.Run("should return nil when entry is empty", func(t *testing.T) {
		t.Parallel()

		// given
		entry := ""

		// when
		ver := domain.ExtractMaxVersionForTest(entry)

		// then
		assert.Nil(t, ver)
	})
}

func TestOverlapRatio(t *testing.T) {
	t.Parallel()

	t.Run("should return 1.0 when both sets are identical", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{"changed", "go", "module"}
		setB := []string{"changed", "go", "module"}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		assert.InDelta(t, 1.0, ratio, 0.001)
	})

	t.Run("should return 0.0 when sets have no overlap", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{"added", "new", "feature"}
		setB := []string{"fixed", "bug", "crash"}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		assert.InDelta(t, 0.0, ratio, 0.001)
	})

	t.Run("should return partial ratio when sets partially overlap", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{"changed", "go", "version", "updated", "module", "dependencies"}
		setB := []string{"changed", "go", "module", "dependencies", "latest", "versions"}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		// intersection: {changed, go, module, dependencies} = 4, min(6, 6) = 6
		assert.InDelta(t, 4.0/6.0, ratio, 0.001)
	})

	t.Run("should return 0.0 when first set is empty", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{}
		setB := []string{"a", "b"}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		assert.InDelta(t, 0.0, ratio, 0.001)
	})

	t.Run("should return 0.0 when second set is empty", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{"a", "b"}
		setB := []string{}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		assert.InDelta(t, 0.0, ratio, 0.001)
	})

	t.Run("should return 0.0 when both sets are empty", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{}
		setB := []string{}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		assert.InDelta(t, 0.0, ratio, 0.001)
	})

	t.Run("should return 1.0 when first set is a subset of second", func(t *testing.T) {
		t.Parallel()

		// given
		setA := []string{"go", "module"}
		setB := []string{"go", "module", "dependencies", "updated"}

		// when
		ratio := domain.OverlapRatioForTest(setA, setB)

		// then
		// intersection: {go, module} = 2, min(2, 4) = 2
		assert.InDelta(t, 1.0, ratio, 0.001)
	})
}

func TestDeduplicateEntries(t *testing.T) {
	t.Parallel()

	t.Run("should keep versioned entry when it subsumes a generic one", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, entries[0], result[0])
	})

	t.Run("should keep highest version entry when multiple versioned entries exist", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go version to `1.25.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, entries[0], result[0])
	})

	t.Run("should keep versioned entry when generic entries are repeated", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
			"- changed the Go module dependencies to their latest versions",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, entries[0], result[0])
	})

	t.Run("should keep single entry when exact duplicates exist", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go module dependencies to their latest versions",
			"- changed the Go module dependencies to their latest versions",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, entries[0], result[0])
	})

	t.Run("should return empty slice when input is empty", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty slice when input is nil", func(t *testing.T) {
		t.Parallel()

		// given
		var entries []string

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return unchanged entry when only one entry exists", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{"- added new feature"}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Equal(t, entries, result)
	})

	t.Run("should keep all entries when they are completely different", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- added new authentication feature",
			"- fixed crash on startup",
			"- removed deprecated API endpoint",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Equal(t, entries, result)
	})

	t.Run("should preserve order when all entries are unique", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- added new authentication module with JWT support",
			"- fixed crash on startup when database is unavailable",
			"- removed deprecated XML configuration parser",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Equal(t, entries, result)
	})

	t.Run("should keep single entry when three exact duplicates exist", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- fixed the login bug",
			"- fixed the login bug",
			"- fixed the login bug",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, "- fixed the login bug", result[0])
	})

	t.Run("should deduplicate entries when they differ only by whitespace", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- fixed the login bug",
			"  - fixed the login bug  ",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
	})

	t.Run("should prefer longer entry when shorter is a subset of longer", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- updated dependencies",
			"- updated dependencies across all modules to fix security vulnerabilities",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Equal(t, entries[1], result[0])
	})

	t.Run("should keep higher version when lower and higher versioned entries exist", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- upgraded library to `2.0.0`",
			"- upgraded library to `3.0.0`",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Contains(t, result[0], "3.0.0")
	})

	t.Run("should keep unique entries alongside deduplicated ones when mixed", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- added new CLI flag for verbose output",
			"- changed the Go module dependencies to their latest versions",
			"- fixed null pointer in config parser",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 3)
		assert.Equal(t, entries[0], result[0])
		assert.Equal(t, entries[1], result[1])
		assert.Equal(t, entries[3], result[2])
	})

	t.Run("should keep at least one entry when breaking change coexists with regular change", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- **BREAKING CHANGE:** changed the API response format",
			"- changed the API response headers",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("should merge entries when backtick-wrapped names are stripped during normalization", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- upgraded `react` from `17.0.0` to `18.0.0`",
			"- upgraded `react-dom` from `17.0.0` to `18.0.0`",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
	})

	t.Run("should keep both entries when topics are genuinely different", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- upgraded the authentication library to support OAuth2 flows",
			"- upgraded the database driver to handle connection pooling",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 2)
	})

	t.Run("should keep higher version entry when same topic has different phrasing", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- upgraded Go version to `1.26.0` and updated module dependencies",
			"- changed Go version to `1.25.0` and updated module dependencies",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Contains(t, result[0], "1.26.0")
	})

	t.Run("should keep unique entries alongside four identical duplicates", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- updated all dependencies",
			"- added README section about deployment",
			"- updated all dependencies",
			"- updated all dependencies",
			"- fixed typo in error message",
			"- updated all dependencies",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 3)
		assert.Equal(t, "- updated all dependencies", result[0])
		assert.Equal(t, "- added README section about deployment", result[1])
		assert.Equal(t, "- fixed typo in error message", result[2])
	})

	t.Run("should keep higher version when lower version is listed first", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- changed the Go version to `1.25.0` and updated all module dependencies",
			"- changed the Go version to `1.26.0` and updated all module dependencies",
		}

		// when
		result := domain.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
		assert.Contains(t, result[0], "1.26.0")
	})
}

func TestUpdateSectionWithDeduplication(t *testing.T) {
	t.Parallel()

	t.Run("should deduplicate before calculating version bump", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
			"- changed the Go module dependencies to their latest versions",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		result, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		require.NotNil(t, nextVer)
		assert.Equal(t, "1.0.1", nextVer.String())
		changedCount := 0
		for _, line := range result {
			if len(line) > 0 && line[0] == '-' {
				changedCount++
			}
		}
		assert.Equal(t, 1, changedCount, "should have exactly one entry after dedup")
	})

	t.Run("should calculate correct version after deduplication across sections", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new CLI command for discovery",
			"- added new CLI command for discovery",
			"",
			"### Changed",
			"",
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
		}
		version := mustParseVersion(t, "2.0.0")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		require.NotNil(t, nextVer)
		assert.Equal(t, "2.1.0", nextVer.String())
	})

	t.Run("should keep single entry when all entries are duplicates of each other", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed something",
			"- changed something",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.1", nextVer.String())
	})
}

// mustParseVersion is a test helper that parses a semver version or fails the test.
func mustParseVersion(t *testing.T, raw string) *semver.Version {
	t.Helper()

	v, err := semver.NewVersion(raw)
	require.NoError(t, err)

	return v
}
