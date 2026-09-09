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

func TestUnwrapChangelogEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		lines    []string
		expected []string
	}{
		{
			name: "should join a two-line entry onto one line",
			lines: []string{
				"### Fixed", "",
				"- fixed the retry backoff",
				"  removed the exponential cap while doing so",
			},
			expected: []string{
				"### Fixed", "",
				"- fixed the retry backoff removed the exponential cap while doing so",
			},
		},
		{
			name: "should join every continuation line when an entry spans more than two lines",
			lines: []string{
				"### Added", "",
				"- added detection for projects using chlog,",
				"  which keeps pending changes as one YAML file per change under",
				"  `.changes/unreleased/` instead of in `CHANGELOG.md`",
			},
			expected: []string{
				"### Added", "",
				"- added detection for projects using chlog, which keeps pending changes as " +
					"one YAML file per change under `.changes/unreleased/` instead of in `CHANGELOG.md`",
			},
		},
		{
			name:  "should leave an already single-line entry unchanged",
			lines: []string{"### Added", "", "- added OAuth2 login"},
			expected: []string{
				"### Added", "", "- added OAuth2 login",
			},
		},
		{
			// NormalizeUnreleasedSection only ever touches [Unreleased] and returns released
			// sections verbatim; this is what makes the correction retroactive for a
			// changelog's whole history rather than just its newest entry.
			name: "should join a wrapped entry in an already released section",
			lines: []string{
				"# Changelog", "",
				"## [Unreleased]", "",
				"## [1.2.0] - 2026-01-01", "",
				"### Fixed", "",
				"- fixed the retry backoff",
				"  removed the exponential cap while doing so",
			},
			expected: []string{
				"# Changelog", "",
				"## [Unreleased]", "",
				"## [1.2.0] - 2026-01-01", "",
				"### Fixed", "",
				"- fixed the retry backoff removed the exponential cap while doing so",
			},
		},
		{
			// A wrapped entry immediately followed by the next section must not swallow that
			// section's own heading.
			name: "should stop joining at the next heading",
			lines: []string{
				"### Fixed", "",
				"- fixed the retry backoff",
				"  and its logging",
				"### Added", "",
				"- added OAuth2 login",
			},
			expected: []string{
				"### Fixed", "",
				"- fixed the retry backoff and its logging",
				"### Added", "",
				"- added OAuth2 login",
			},
		},
		{
			// A blank line closes the entry rather than being folded into it: a
			// reference-style link block at the end of the file is blank-separated from the
			// last release section, and reading it as one more continuation line would
			// splice it into the entry above and break the link definition.
			name: "should not join a comparison link separated from the last entry by a blank line",
			lines: []string{
				"### Added", "",
				"- added zulu",
				"",
				"[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD",
			},
			expected: []string{
				"### Added", "",
				"- added zulu",
				"",
				"[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD",
			},
		},
		{
			// A nested list is structure the writer put there, not a wrapped sentence, and
			// this runs over released history where flattening it would be permanent.
			name: "should keep a nested list nested instead of folding it into its parent",
			lines: []string{
				"## [1.4.0] - 2026-02-01", "",
				"### Added", "",
				"- added multi-provider support:",
				"  - GitHub",
				"  - GitLab",
				"  - Azure DevOps",
			},
			expected: []string{
				"## [1.4.0] - 2026-02-01", "",
				"### Added", "",
				"- added multi-provider support:",
				"  - GitHub",
				"  - GitLab",
				"  - Azure DevOps",
			},
		},
		{
			// A nested item opens an entry of its own, so its wrap joins onto itself, and
			// the entry that follows the sub-list is not folded into the parent either.
			name: "should join a wrapped nested item onto itself rather than onto its parent",
			lines: []string{
				"### Added", "",
				"- added multi-provider support:",
				"  - GitHub, which needs a token",
				"    carrying the repo scope",
				"- added a second entry",
			},
			expected: []string{
				"### Added", "",
				"- added multi-provider support:",
				"  - GitHub, which needs a token carrying the repo scope",
				"- added a second entry",
			},
		},
		{
			name: "should keep an ordered nested list nested",
			lines: []string{
				"### Added", "",
				"- added a migration guide:",
				"  1. stop the service",
				"  2. run the migration",
			},
			expected: []string{
				"### Added", "",
				"- added a migration guide:",
				"  1. stop the service",
				"  2. run the migration",
			},
		},
		{
			// A version opening a wrapped line is not an ordered list item: the marker only
			// matches when whitespace follows it, and "1.26" has none after its first dot.
			name: "should join a wrapped line that opens with a version",
			lines: []string{
				"### Changed", "",
				"- changed the toolchain to Go",
				"  1.26 for the new vet checks",
			},
			expected: []string{
				"### Changed", "",
				"- changed the toolchain to Go 1.26 for the new vet checks",
			},
		},
		{
			name: "should keep a fenced code block under an entry verbatim",
			lines: []string{
				"### Added", "",
				"- added the `refresh` key:",
				"  ```yaml",
				"  # enable it per project",
				"  refresh: true",
				"  ```",
				"- added a second entry",
			},
			expected: []string{
				"### Added", "",
				"- added the `refresh` key:",
				"  ```yaml",
				"  # enable it per project",
				"  refresh: true",
				"  ```",
				"- added a second entry",
			},
		},
		{
			// Only a "#" at column 0 is a heading. An indented one is an issue reference
			// inside the entry, and closing the entry on it left the wrap this removes.
			name: "should treat an indented issue reference as a continuation",
			lines: []string{
				"### Fixed", "",
				"- fixed the retry backoff",
				"  #123 tracked the exponential cap",
			},
			expected: []string{
				"### Fixed", "",
				"- fixed the retry backoff #123 tracked the exponential cap",
			},
		},
		{
			name:     "should leave a document with no entries unchanged",
			lines:    []string{"# Changelog", "", "## [Unreleased]", ""},
			expected: []string{"# Changelog", "", "## [Unreleased]", ""},
		},
		{
			name:     "should return an empty slice for empty input",
			lines:    []string{},
			expected: []string{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			unwrapped := entities.UnwrapChangelogEntries(testCase.lines)

			// then
			assert.Equal(t, testCase.expected, unwrapped)
		})
	}

	t.Run("should change nothing when the document is unwrapped twice", func(t *testing.T) {
		t.Parallel()

		// given the changelog is read several times per run
		lines := []string{
			"### Fixed", "",
			"- fixed the retry backoff",
			"  removed the exponential cap while doing so",
			"- fixed the provider list:",
			"  - GitHub",
			"  ```yaml",
			"  refresh: true",
			"  ```",
		}

		// when
		once := entities.UnwrapChangelogEntries(lines)
		twice := entities.UnwrapChangelogEntries(once)

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
