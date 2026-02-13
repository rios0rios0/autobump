package domain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/domain"
)

func TestIsChangelogUnreleasedEmpty(t *testing.T) {
	t.Parallel()

	t.Run("should return false when unreleased section has changes", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed something important",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		empty, err := domain.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.False(t, empty)
	})

	t.Run("should return true when unreleased section has no changes", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		empty, err := domain.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.True(t, empty)
	})

	t.Run("should return false when only unreleased section exists with content", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added a new feature",
		}

		// when
		empty, err := domain.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.False(t, empty)
	})

	t.Run("should return true when only unreleased section exists without content", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
		}

		// when
		empty, err := domain.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.True(t, empty)
	})

	t.Run("should return false when multiple versions exist and unreleased has content", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"- added feature X",
			"",
			"## [2.0.0] - 2025-06-01",
			"",
			"### Added",
			"",
			"- release 2.0",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		empty, err := domain.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.False(t, empty)
	})
}

func TestFindLatestVersion(t *testing.T) {
	t.Parallel()

	t.Run("should return the version when a single version exists", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"## [Unreleased]",
			"",
			"## [1.0.0] - 2025-01-01",
		}

		// when
		ver, err := domain.FindLatestVersion(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", ver.String())
	})

	t.Run("should return the highest version when multiple versions exist", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"## [Unreleased]",
			"## [2.1.0] - 2025-06-01",
			"## [1.5.0] - 2025-03-01",
			"## [1.0.0] - 2025-01-01",
		}

		// when
		ver, err := domain.FindLatestVersion(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.1.0", ver.String())
	})

	t.Run("should return the highest version when versions are not in order", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"## [Unreleased]",
			"## [1.0.0] - 2025-01-01",
			"## [3.0.0] - 2025-09-01",
			"## [2.0.0] - 2025-06-01",
		}

		// when
		ver, err := domain.FindLatestVersion(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.0.0", ver.String())
	})

	t.Run("should return error when no version is found", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- some new feature",
		}

		// when
		ver, err := domain.FindLatestVersion(lines)

		// then
		assert.Nil(t, ver)
		assert.ErrorIs(t, err, domain.ErrNoVersionFoundInChangelog)
	})

	t.Run("should return error when lines are empty", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{}

		// when
		ver, err := domain.FindLatestVersion(lines)

		// then
		assert.Nil(t, ver)
		assert.ErrorIs(t, err, domain.ErrNoVersionFoundInChangelog)
	})
}

func TestFixSectionHeadings(t *testing.T) {
	t.Parallel()

	t.Run("should keep correct headings unchanged when already at level 3", func(t *testing.T) {
		t.Parallel()

		// given
		section := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- something",
		}

		// when
		domain.FixSectionHeadings(section)

		// then
		assert.Equal(t, "### Added", section[2])
	})

	t.Run("should fix headings when they have wrong levels", func(t *testing.T) {
		t.Parallel()

		// given
		section := []string{
			"## [Unreleased]",
			"",
			"#### Added",
			"",
			"- something",
			"",
			"## Changed",
			"",
			"- another thing",
		}

		// when
		domain.FixSectionHeadings(section)

		// then
		assert.Equal(t, "### Added", section[2])
		assert.Equal(t, "### Changed", section[6])
	})

	t.Run("should preserve case when headings are lowercase", func(t *testing.T) {
		t.Parallel()

		// given
		section := []string{
			"## [Unreleased]",
			"",
			"### added",
			"",
			"- something",
		}

		// when
		domain.FixSectionHeadings(section)

		// then
		assert.Equal(t, "### added", section[2])
	})

	t.Run("should fix all section types when all have wrong levels", func(t *testing.T) {
		t.Parallel()

		// given
		section := []string{
			"## [Unreleased]",
			"#### Added",
			"#### Changed",
			"#### Deprecated",
			"#### Removed",
			"#### Fixed",
			"#### Security",
		}

		// when
		domain.FixSectionHeadings(section)

		// then
		assert.Equal(t, "### Added", section[1])
		assert.Equal(t, "### Changed", section[2])
		assert.Equal(t, "### Deprecated", section[3])
		assert.Equal(t, "### Removed", section[4])
		assert.Equal(t, "### Fixed", section[5])
		assert.Equal(t, "### Security", section[6])
	})
}

func TestParseUnreleasedIntoSections(t *testing.T) {
	t.Parallel()

	t.Run("should parse entries into correct section when a single section exists", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added feature A",
			"- added feature B",
		}
		sections := makeEmptySections()
		var currentSection *[]string
		major, minor, patch := 0, 0, 0

		// when
		domain.ParseUnreleasedIntoSections(unreleased, sections, currentSection, &major, &minor, &patch)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 2, minor)
		assert.Equal(t, 0, patch)
		assert.Len(t, *sections["Added"], 2)
	})

	t.Run("should distribute entries across sections when multiple sections exist", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added feature A",
			"",
			"### Changed",
			"",
			"- changed behavior B",
			"",
			"### Fixed",
			"",
			"- fixed bug C",
		}
		sections := makeEmptySections()
		var currentSection *[]string
		major, minor, patch := 0, 0, 0

		// when
		domain.ParseUnreleasedIntoSections(unreleased, sections, currentSection, &major, &minor, &patch)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 1, minor)
		assert.Equal(t, 2, patch) // Changed + Fixed = 2 patch
		assert.Len(t, *sections["Added"], 1)
		assert.Len(t, *sections["Changed"], 1)
		assert.Len(t, *sections["Fixed"], 1)
	})

	t.Run("should increment major counter when breaking change is present", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- **BREAKING CHANGE:** removed backward compatibility",
			"- changed minor thing",
		}
		sections := makeEmptySections()
		var currentSection *[]string
		major, minor, patch := 0, 0, 0

		// when
		domain.ParseUnreleasedIntoSections(unreleased, sections, currentSection, &major, &minor, &patch)

		// then
		assert.Equal(t, 1, major)
		assert.Equal(t, 0, minor)
		assert.Equal(t, 1, patch)
	})

	t.Run("should keep all counters at zero when unreleased section is empty", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
		}
		sections := makeEmptySections()
		var currentSection *[]string
		major, minor, patch := 0, 0, 0

		// when
		domain.ParseUnreleasedIntoSections(unreleased, sections, currentSection, &major, &minor, &patch)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 0, minor)
		assert.Equal(t, 0, patch)
	})

	t.Run("should skip blank lines when counting entries", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"",
			"- added feature A",
			"",
			"",
		}
		sections := makeEmptySections()
		var currentSection *[]string
		major, minor, patch := 0, 0, 0

		// when
		domain.ParseUnreleasedIntoSections(unreleased, sections, currentSection, &major, &minor, &patch)

		// then
		assert.Equal(t, 1, minor)
		assert.Len(t, *sections["Added"], 1)
	})
}

func TestRecountChanges(t *testing.T) {
	t.Parallel()

	t.Run("should return all zeros when sections are empty", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()

		// when
		major, minor, patch := domain.RecountChangesForTest(sections)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 0, minor)
		assert.Equal(t, 0, patch)
	})

	t.Run("should count minor changes when only Added section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Added"] = []string{"- added feature A", "- added feature B"}

		// when
		major, minor, patch := domain.RecountChangesForTest(sections)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 2, minor)
		assert.Equal(t, 0, patch)
	})

	t.Run("should count patch changes when only Changed section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Changed"] = []string{"- changed behavior"}

		// when
		major, minor, patch := domain.RecountChangesForTest(sections)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 0, minor)
		assert.Equal(t, 1, patch)
	})

	t.Run("should count major changes when breaking change is present", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Changed"] = []string{
			"- **BREAKING CHANGE:** removed API",
			"- changed minor thing",
		}

		// when
		major, minor, patch := domain.RecountChangesForTest(sections)

		// then
		assert.Equal(t, 1, major)
		assert.Equal(t, 0, minor)
		assert.Equal(t, 1, patch)
	})

	t.Run("should count all change types when all sections have entries", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Added"] = []string{"- added A"}
		*sections["Changed"] = []string{"- changed B"}
		*sections["Fixed"] = []string{"- fixed C"}
		*sections["Removed"] = []string{"- removed D"}
		*sections["Deprecated"] = []string{"- deprecated E"}
		*sections["Security"] = []string{"- security F"}

		// when
		major, minor, patch := domain.RecountChangesForTest(sections)

		// then
		assert.Equal(t, 0, major)
		assert.Equal(t, 1, minor)
		assert.Equal(t, 5, patch)
	})
}

func TestMakeNewSections(t *testing.T) {
	t.Parallel()

	t.Run("should include section header and entry when a single section has content", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Added"] = []string{"- added feature A"}
		version := mustParseVersion(t, "1.1.0")

		// when
		result := domain.MakeNewSections(sections, *version)

		// then
		assert.Contains(t, result[0], "[Unreleased]")
		assert.Contains(t, result[2], "[1.1.0]")
		assert.Contains(t, result[2], time.Now().Format("2006-01-02"))
		found := false
		for _, line := range result {
			if line == "### Added" {
				found = true
			}
		}
		assert.True(t, found, "should contain ### Added section")
	})

	t.Run("should include only populated sections when multiple sections have content", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Added"] = []string{"- added A"}
		*sections["Fixed"] = []string{"- fixed B"}
		version := mustParseVersion(t, "2.0.0")

		// when
		result := domain.MakeNewSections(sections, *version)

		// then
		resultStr := strings.Join(result, "\n")
		assert.Contains(t, resultStr, "### Added")
		assert.Contains(t, resultStr, "### Fixed")
		assert.NotContains(t, resultStr, "### Changed")
		assert.NotContains(t, resultStr, "### Removed")
	})

	t.Run("should omit all section headers when all sections are empty", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		version := mustParseVersion(t, "1.0.0")

		// when
		result := domain.MakeNewSections(sections, *version)

		// then
		assert.Contains(t, result[0], "[Unreleased]")
		assert.Contains(t, result[2], "[1.0.0]")
		for _, line := range result {
			assert.NotContains(t, line, "### ")
		}
	})

	t.Run("should preserve correct section order when sections are populated out of order", func(t *testing.T) {
		t.Parallel()

		// given
		sections := makeEmptySections()
		*sections["Security"] = []string{"- security A"}
		*sections["Added"] = []string{"- added B"}
		*sections["Fixed"] = []string{"- fixed C"}
		version := mustParseVersion(t, "1.0.0")

		// when
		result := domain.MakeNewSections(sections, *version)

		// then
		addedIdx, fixedIdx, securityIdx := -1, -1, -1
		for i, line := range result {
			switch line {
			case "### Added":
				addedIdx = i
			case "### Fixed":
				fixedIdx = i
			case "### Security":
				securityIdx = i
			}
		}
		assert.Greater(t, fixedIdx, addedIdx, "Fixed should come after Added")
		assert.Greater(t, securityIdx, fixedIdx, "Security should come after Fixed")
	})
}

func TestMakeNewSectionsFromUnreleased(t *testing.T) {
	t.Parallel()

	t.Run("should produce a single unreleased header when creating initial release", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- initial feature",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		result := domain.MakeNewSectionsFromUnreleased(unreleased, *version)

		// then
		assert.Contains(t, result[0], "[Unreleased]")
		assert.Contains(t, result[2], "[1.0.0]")
		unreleasedCount := 0
		for _, line := range result {
			if strings.Contains(line, "[Unreleased]") {
				unreleasedCount++
			}
		}
		assert.Equal(t, 1, unreleasedCount, "should only have one Unreleased header")
	})
}

func TestUpdateSection(t *testing.T) {
	t.Parallel()

	t.Run("should bump patch version when only Changed section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed behavior of the parser",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.1", nextVer.String())
	})

	t.Run("should bump minor version when only Added section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new endpoint",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", nextVer.String())
	})

	t.Run("should bump major version when breaking change is present", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- **BREAKING CHANGE:** removed backward compatibility",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", nextVer.String())
	})

	t.Run("should return error when no changes are found", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		_, _, err := domain.UpdateSection(unreleased, *version)

		// then
		assert.ErrorIs(t, err, domain.ErrNoChangesFoundInUnreleased)
	})

	t.Run("should sort entries alphabetically when multiple entries exist", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added websocket support for real-time notifications",
			"- added authentication middleware for protected routes",
			"- added pagination component for dashboard tables",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		result, _, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		var entries []string
		for _, line := range result {
			if strings.HasPrefix(line, "- added") {
				entries = append(entries, line)
			}
		}
		require.Len(t, entries, 3)
		assert.Equal(t, "- added authentication middleware for protected routes", entries[0])
		assert.Equal(t, "- added pagination component for dashboard tables", entries[1])
		assert.Equal(t, "- added websocket support for real-time notifications", entries[2])
	})

	t.Run("should bump major version when breaking change coexists with other changes", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"### Changed",
			"",
			"- **BREAKING CHANGE:** removed old API",
			"- changed minor thing",
		}
		version := mustParseVersion(t, "1.5.3")

		// when
		_, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", nextVer.String())
	})

	t.Run("should generate correct format with unreleased and version headers", func(t *testing.T) {
		t.Parallel()

		// given
		unreleased := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added feature A",
			"",
			"### Fixed",
			"",
			"- fixed bug B",
		}
		version := mustParseVersion(t, "1.0.0")

		// when
		result, nextVer, err := domain.UpdateSection(unreleased, *version)

		// then
		require.NoError(t, err)
		resultStr := strings.Join(result, "\n")
		assert.Contains(t, resultStr, "## [Unreleased]")
		expectedVersionLine := fmt.Sprintf("## [%s] - %s", nextVer.String(), time.Now().Format("2006-01-02"))
		assert.Contains(t, resultStr, expectedVersionLine)
	})
}

func TestProcessChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should bump patch version when only Fixed section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Fixed",
			"",
			"- fixed a critical bug",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		nextVer, _, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.1", nextVer.String())
	})

	t.Run("should bump minor version when only Added section has entries", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		nextVer, _, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", nextVer.String())
	})

	t.Run("should bump major version when breaking change is present", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- **BREAKING CHANGE:** removed old API",
			"",
			"## [2.5.3] - 2025-01-01",
			"",
			"### Added",
			"",
			"- something",
		}

		// when
		nextVer, _, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "3.0.0", nextVer.String())
	})

	t.Run("should start at 1.0.0 when no previous version exists", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added initial features",
		}

		// when
		nextVer, _, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", nextVer.String())
	})

	t.Run("should preserve existing content when processing changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added new feature",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		_, content, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		resultStr := strings.Join(content, "\n")
		assert.Contains(t, resultStr, "[1.0.0]")
		assert.Contains(t, resultStr, "initial release")
	})

	t.Run("should deduplicate entries when processing changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Changed",
			"",
			"- changed the Go version to `1.26.0` and updated all module dependencies",
			"- changed the Go module dependencies to their latest versions",
			"- changed the Go module dependencies to their latest versions",
			"",
			"## [1.0.0] - 2025-01-01",
			"",
			"### Added",
			"",
			"- initial release",
		}

		// when
		_, content, err := domain.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		changedCount := 0
		for _, line := range content {
			if strings.HasPrefix(line, "- changed") {
				changedCount++
			}
		}
		assert.Equal(t, 1, changedCount, "should have exactly one Changed entry after dedup")
	})
}

func TestProcessNewChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should return version 1.0.0 when processing a new changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added initial features",
		}

		// when
		ver, content, err := domain.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", ver.String())
		resultStr := strings.Join(content, "\n")
		assert.Contains(t, resultStr, "[1.0.0]")
		assert.Contains(t, resultStr, "[Unreleased]")
	})

	t.Run("should deduplicate entries when processing a new changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- added feature A",
			"- added feature A",
		}

		// when
		_, content, err := domain.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		featureCount := 0
		for _, line := range content {
			if strings.Contains(line, "added feature A") {
				featureCount++
			}
		}
		assert.Equal(t, 1, featureCount, "should deduplicate entries")
	})

	t.Run("should preserve header content when processing a new changelog", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"",
			"All notable changes to this project will be documented in this file.",
			"",
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- initial feature",
		}

		// when
		_, content, err := domain.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		resultStr := strings.Join(content, "\n")
		assert.Contains(t, resultStr, "# Changelog")
		assert.Contains(t, resultStr, "All notable changes")
	})
}

// makeEmptySections creates an empty sections map for testing.
func makeEmptySections() map[string]*[]string {
	return map[string]*[]string{
		"Added":      {},
		"Changed":    {},
		"Deprecated": {},
		"Removed":    {},
		"Fixed":      {},
		"Security":   {},
	}
}

// mustParseVersion is a test helper that parses a semver version or fails the test.
