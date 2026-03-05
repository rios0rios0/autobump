package entities

import (
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

// ProcessChangelog delegates to gitforge's Changelog.Process.
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	return changelogEntities.NewChangelog(lines).Process()
}

// ProcessNewChangelog delegates to gitforge's Changelog.ProcessNew.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	return changelogEntities.NewChangelog(lines).ProcessNew()
}
