package entities

import (
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	changelogEntities "github.com/rios0rios0/gitforge/pkg/changelog/domain/entities"
)

// DefaultChangelogURL is the URL of the default CHANGELOG template.
const DefaultChangelogURL = "https://raw.githubusercontent.com/rios0rios0/" +
	"autobump/main/configs/CHANGELOG.template.md"

// Re-export gitforge errors and constants for backward compatibility.
var (
	ErrNoVersionFoundInChangelog  = changelogEntities.ErrNoVersionFoundInChangelog
	ErrNoChangesFoundInUnreleased = changelogEntities.ErrNoChangesFoundInUnreleased
)

// InitialReleaseVersion is re-exported from gitforge.
const InitialReleaseVersion = changelogEntities.InitialReleaseVersion

// IsChangelogUnreleasedEmpty delegates to gitforge's Changelog.IsUnreleasedEmpty.
func IsChangelogUnreleasedEmpty(lines []string) (bool, error) {
	return changelogEntities.NewChangelog(lines).IsUnreleasedEmpty()
}

// FindLatestVersion delegates to gitforge's Changelog.FindLatestVersion.
func FindLatestVersion(lines []string) (*semver.Version, error) {
	return changelogEntities.NewChangelog(lines).FindLatestVersion()
}

// ProcessChangelog delegates to gitforge's Changelog.Process and sorts entries alphabetically.
//
// The whole document is unwrapped and the [Unreleased] section is normalised first.
// Unwrapping repairs what both the SemVer pipeline and the sort below cannot see past -- a
// multi-line entry read as more than one line -- and normalising repairs a mis-cased heading
// whose entries it would drop, or a breaking-change marker spelled in a way it does not
// count. See UnwrapChangelogEntries and NormalizeUnreleasedSection. Unwrapping runs again
// immediately before the sort so the invariant holds locally at the one call that needs it,
// regardless of what the pipeline in between did to the content.
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	prepared := NormalizeUnreleasedSection(UnwrapChangelogEntries(lines))

	version, content, err := changelogEntities.NewChangelog(prepared).Process()
	if err != nil {
		return nil, nil, err
	}
	return version, SortChangelogEntries(UnwrapChangelogEntries(content)), nil
}

// ProcessNewChangelog delegates to gitforge's Changelog.ProcessNew and sorts entries alphabetically.
//
// The whole document is unwrapped and the [Unreleased] section is normalised first, for the
// reasons given on ProcessChangelog.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	prepared := NormalizeUnreleasedSection(UnwrapChangelogEntries(lines))

	version, content, err := changelogEntities.NewChangelog(prepared).ProcessNew()
	if err != nil {
		return nil, nil, err
	}
	return version, SortChangelogEntries(UnwrapChangelogEntries(content)), nil
}

// SortChangelogEntries sorts bullet entries (lines starting with "- ")
// alphabetically (case-insensitive) within each contiguous run.
func SortChangelogEntries(lines []string) []string {
	result := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "- ") {
			result = append(result, lines[i])
			i++
			continue
		}

		var bullets []string
		for i < len(lines) && strings.HasPrefix(lines[i], "- ") {
			bullets = append(bullets, lines[i])
			i++
		}

		slices.SortStableFunc(bullets, func(a, b string) int {
			return strings.Compare(strings.ToLower(a), strings.ToLower(b))
		})

		result = append(result, bullets...)
	}
	return result
}
