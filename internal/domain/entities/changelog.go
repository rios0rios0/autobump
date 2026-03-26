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
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	version, content, err := changelogEntities.NewChangelog(lines).Process()
	if err != nil {
		return nil, nil, err
	}
	return version, SortChangelogEntries(content), nil
}

// ProcessNewChangelog delegates to gitforge's Changelog.ProcessNew and sorts entries alphabetically.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	version, content, err := changelogEntities.NewChangelog(lines).ProcessNew()
	if err != nil {
		return nil, nil, err
	}
	return version, SortChangelogEntries(content), nil
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
