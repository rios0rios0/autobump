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

	// The rules differ only in the [Unreleased] body they are given and the one they have to
	// produce, so they are a table rather than a dozen copies of the same body.
	rewriteCases := []struct {
		name     string
		body     []string
		expected []string
	}{
		{
			// The release renderer buckets by the exact string "### Added", so a heading
			// written at the wrong depth or casing takes every entry under it out of the
			// release.
			name:     "should repair the heading when it is written at the wrong depth or casing",
			body:     []string{"#### added", "", "- added OAuth2 login"},
			expected: []string{"### Added", "", "- added OAuth2 login"},
		},
		{
			// What a changelog looks like once fragments are spliced next to hand-written
			// entries that use the same headings.
			name: "should merge the headings when the same section is opened twice",
			body: []string{
				"### Added", "", "- added OAuth2 login", "",
				"### Added", "", "- added SSO support",
			},
			expected: []string{"### Added", "", "- added OAuth2 login", "- added SSO support"},
		},
		{
			name:     "should drop the repeat when two entries are identical",
			body:     []string{"### Added", "", "- added OAuth2 login", "- added OAuth2 login"},
			expected: []string{"### Added", "", "- added OAuth2 login"},
		},
		{
			name: "should keep the fuller entry when two entries nearly overlap",
			body: []string{
				"### Added", "",
				"- added support for the new provider",
				"- added support for the new provider adapter",
			},
			expected: []string{"### Added", "", "- added support for the new provider adapter"},
		},
		{
			name: "should file the entry under the section its verb names",
			body: []string{
				"### Changed", "", "- removed the deprecated helper", "- changed the retry backoff",
			},
			expected: []string{
				"### Changed", "", "- changed the retry backoff",
				"",
				"### Removed", "", "- removed the deprecated helper",
			},
		},
		{
			name: "should order the sections and the entries inside them",
			body: []string{
				"### Security", "", "- rotated the signing key", "",
				"### Added", "", "- added SSO support", "- added OAuth2 login",
			},
			expected: []string{
				"### Added", "", "- added OAuth2 login", "- added SSO support",
				"",
				"### Security", "", "- rotated the signing key",
			},
		},
		{
			// A continuation line that opens with a verb; judged on its own it would be
			// filed under "### Removed", orphaned from the bullet it explains.
			name: "should move an entry as a whole when it spans several lines",
			body: []string{
				"### Fixed", "",
				"- fixed the retry backoff",
				"  removed the exponential cap while doing so",
				"- added a nothing entry that sorts first",
			},
			expected: []string{
				"### Added", "", "- added a nothing entry that sorts first",
				"",
				"### Fixed", "",
				"- fixed the retry backoff",
				"  removed the exponential cap while doing so",
			},
		},
		{
			// Both spell the same change, so canonicalising them collapses the pair.
			name: "should write one canonical marker when entries spell it differently",
			body: []string{
				"### Changed", "",
				"- BREAKING CHANGE: dropped the v1 endpoint",
				"- **BREAKING CHANGE:** BREAKING CHANGE: dropped the v1 endpoint",
			},
			expected: []string{"### Changed", "", "- **BREAKING CHANGE:** dropped the v1 endpoint"},
		},
		{
			// Rewriting a section must not silently drop what is not a bullet.
			name: "should keep the prose a writer put under a heading",
			body: []string{
				"### Added", "",
				"Everything below needs the new runtime.", "",
				"- added OAuth2 login",
			},
			expected: []string{
				"### Added", "",
				"Everything below needs the new runtime.", "",
				"- added OAuth2 login",
			},
		},
		{
			name: "should keep the lines a writer put above the first heading",
			body: []string{
				"This release needs a manual migration step.", "",
				"### Added", "", "- added OAuth2 login",
			},
			expected: []string{
				"This release needs a manual migration step.",
				"",
				"### Added", "", "- added OAuth2 login",
			},
		},
	}

	for _, testCase := range rewriteCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given
			lines := changelogWith(testCase.body...)

			// when
			normalized := entities.NormalizeUnreleasedSection(lines)

			// then the body is bracketed by the blank lines that separate it from the headers
			expected := append([]string{""}, testCase.expected...)
			assert.Equal(t, append(expected, ""), unreleasedOf(t, normalized))
		})
	}

	// A section the rules find nothing to act on is preserved rather than rewritten into
	// something else, so these cases assert on the whole document instead.
	untouchedCases := []struct {
		name  string
		lines []string
	}{
		{
			name:  "should leave the document alone when it has no unreleased section",
			lines: []string{"# Changelog", "", "## [1.2.0] - 2026-01-01", "", "- added the first release"},
		},
		{
			name:  "should leave the section alone when it holds nothing the rules recognise",
			lines: changelogWith("see the release notes instead"),
		},
		{
			// The state a chlog repository is permanently in.
			name: "should leave the section alone when it is empty",
			lines: []string{
				"# Changelog", "", "## [Unreleased]", "", "## [1.2.0] - 2026-01-01", "",
				"### Added", "", "- added the first release",
			},
		},
	}

	for _, testCase := range untouchedCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			normalized := entities.NormalizeUnreleasedSection(testCase.lines)

			// then
			assert.Equal(t, testCase.lines, normalized)
		})
	}

	t.Run("should leave the released sections untouched", func(t *testing.T) {
		t.Parallel()

		// given
		lines := changelogWith("#### added", "", "- added OAuth2 login")

		// when
		normalized := entities.NormalizeUnreleasedSection(lines)

		// then
		assert.Equal(t, []string{"# Changelog", "", "## [Unreleased]"}, normalized[:3])
		assert.Equal(t, []string{
			"## [1.2.0] - 2026-01-01", "", "### Added", "", "- added the first release",
		}, normalized[len(normalized)-5:])
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
