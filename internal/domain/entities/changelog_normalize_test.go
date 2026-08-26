package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// unreleasedOf returns the [Unreleased] body of a normalised changelog, which is what every
// rule below is asserted on. The released sections are asserted separately, once.
func unreleasedOf(t *testing.T, lines []string) []string {
	t.Helper()

	body := make([]string, 0, len(lines))
	inside := false
	for _, line := range lines {
		name, isHeader := entities.MatchChangelogVersionHeader(line)
		if isHeader {
			inside = name == entities.UnreleasedHeaderName
			continue
		}
		if inside {
			body = append(body, line)
		}
	}

	return body
}

// changelogWith wraps an [Unreleased] body in a released changelog.
func changelogWith(body ...string) []string {
	lines := []string{"# Changelog", "", "## [Unreleased]", ""}
	lines = append(lines, body...)
	lines = append(lines,
		"", "## [1.2.0] - 2026-01-01", "", "### Added", "", "- added the first release")

	return lines
}

func TestNormalizeUnreleasedSection(t *testing.T) {
	t.Parallel()

	t.Run("should repair the heading when it is written at the wrong depth or casing", func(t *testing.T) {
		t.Parallel()

		// given the release renderer buckets by the exact string "### Added", so a
		// mis-written heading takes every entry underneath it out of the release
		lines := changelogWith("#### added", "", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t,
			[]string{"", "### Added", "", "- added OAuth2 login", ""},
			unreleasedOf(t, normalized))
	})

	t.Run("should merge the headings when the same section is opened twice", func(t *testing.T) {
		t.Parallel()

		// given this is what a changelog looks like once fragments are spliced next to
		// hand-written entries that use the same headings
		lines := changelogWith(
			"### Added", "", "- added OAuth2 login", "",
			"### Added", "", "- added SSO support")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t,
			[]string{"", "### Added", "", "- added OAuth2 login", "- added SSO support", ""},
			unreleasedOf(t, normalized))
	})

	t.Run("should drop the repeat when two entries are identical", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"### Added", "", "- added OAuth2 login", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t,
			[]string{"", "### Added", "", "- added OAuth2 login", ""},
			unreleasedOf(t, normalized))
	})

	t.Run("should keep the fuller entry when two entries nearly overlap", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"### Added", "",
			"- added support for the new provider",
			"- added support for the new provider adapter")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t,
			[]string{"", "### Added", "", "- added support for the new provider adapter", ""},
			unreleasedOf(t, normalized))
	})

	t.Run("should file the entry under the section its verb names", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"### Changed", "", "- removed the deprecated helper", "- changed the retry backoff")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"",
			"### Changed", "", "- changed the retry backoff",
			"",
			"### Removed", "", "- removed the deprecated helper",
			"",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should order the sections and the entries inside them", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"### Security", "", "- rotated the signing key", "",
			"### Added", "", "- added SSO support", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"",
			"### Added", "", "- added OAuth2 login", "- added SSO support",
			"",
			"### Security", "", "- rotated the signing key",
			"",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should move an entry as a whole when it spans several lines", func(t *testing.T) {
		t.Parallel()

		// given a continuation line that opens with a verb; judged on its own it would be
		// filed under "### Removed", orphaned from the bullet it explains
		lines := changelogWith(
			"### Fixed", "",
			"- fixed the retry backoff",
			"  removed the exponential cap while doing so",
			"- added a nothing entry that sorts first")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"",
			"### Added", "", "- added a nothing entry that sorts first",
			"",
			"### Fixed", "",
			"- fixed the retry backoff",
			"  removed the exponential cap while doing so",
			"",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should write one canonical marker when entries spell it differently", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"### Changed", "",
			"- BREAKING CHANGE: dropped the v1 endpoint",
			"- **BREAKING CHANGE:** BREAKING CHANGE: dropped the v1 endpoint")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then both spell the same change, so canonicalising them collapses the pair
		assert.Equal(t, []string{
			"", "### Changed", "", "- **BREAKING CHANGE:** dropped the v1 endpoint", "",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should leave the released sections untouched", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith("#### added", "", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"## [1.2.0] - 2026-01-01", "", "### Added", "", "- added the first release",
		}, normalized[len(normalized)-5:])
		assert.Equal(t, []string{"# Changelog", "", "## [Unreleased]"}, normalized[:3])
	})

	t.Run("should change nothing when the section is normalised twice", func(t *testing.T) {
		t.Parallel()

		// given the changelog is read several times per run
		lines := changelogWith(
			"#### added", "", "- added OAuth2 login", "",
			"### Changed", "", "- removed the deprecated helper")

		// when
		once := entities.NormalizeUnreleasedSection(lines)
		twice := entities.NormalizeUnreleasedSection(once)

		// then
		assert.Equal(t, once, twice)
	})

	t.Run("should keep the prose a writer put under a heading", func(t *testing.T) {
		t.Parallel()

		// given rewriting a section must not silently drop what is not a bullet
		lines := changelogWith(
			"### Added", "",
			"Everything below needs the new runtime.", "",
			"- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"", "### Added", "",
			"Everything below needs the new runtime.", "",
			"- added OAuth2 login", "",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should keep the lines a writer put above the first heading", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith(
			"This release needs a manual migration step.", "",
			"### Added", "", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{
			"", "This release needs a manual migration step.", "",
			"### Added", "", "- added OAuth2 login", "",
		}, unreleasedOf(t, normalized))
	})

	t.Run("should leave the document alone when it has no unreleased section", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{"# Changelog", "", "## [1.2.0] - 2026-01-01", "", "- added the first release"}

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, lines, normalized)
	})

	t.Run("should leave the section alone when it holds nothing the rules recognise", func(t *testing.T) {
		t.Parallel()

		// given an unusual section is preserved rather than rewritten into something else
		lines := changelogWith("see the release notes instead")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, lines, normalized)
	})

	t.Run("should leave the section alone when it is empty", func(t *testing.T) {
		t.Parallel()

		// given the state a chlog repository is permanently in
		lines := []string{
			"# Changelog", "", "## [Unreleased]", "", "## [1.2.0] - 2026-01-01", "",
			"### Added", "", "- added the first release",
		}

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, lines, normalized)
	})
}

func TestMatchChangelogVersionHeader(t *testing.T) {
	t.Parallel()

	t.Run("should return the name when the line is a version header", func(t *testing.T) {
		t.Parallel()

		// given / when
		name, isHeader := entities.MatchChangelogVersionHeader("## [1.2.0] - 2026-01-01")

		// then
		assert.True(t, isHeader)
		assert.Equal(t, "1.2.0", name)
	})

	t.Run("should return the unreleased name when the line opens the pending section", func(t *testing.T) {
		t.Parallel()

		// given / when
		name, isHeader := entities.MatchChangelogVersionHeader("## [Unreleased]")

		// then
		assert.True(t, isHeader)
		assert.Equal(t, entities.UnreleasedHeaderName, name)
	})

	t.Run("should report no match when the line is a section heading", func(t *testing.T) {
		t.Parallel()

		// given / when
		name, isHeader := entities.MatchChangelogVersionHeader("### Added")

		// then
		assert.False(t, isHeader)
		assert.Empty(t, name)
	})
}
