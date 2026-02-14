package entities

import (
	"github.com/Masterminds/semver/v3"

	forgeEntities "github.com/rios0rios0/gitforge/domain/entities"
)

// --- Type aliases for shared types from gitforge ---

type ServiceType = forgeEntities.ServiceType
type BranchStatus = forgeEntities.BranchStatus
type LatestTag = forgeEntities.LatestTag
type Repository = forgeEntities.Repository
type ProviderConfig = forgeEntities.ProviderConfig
type ControllerBind = forgeEntities.ControllerBind
type RepositoryDiscoverer = forgeEntities.RepositoryDiscoverer

// --- ServiceType constants ---

const (
	UNKNOWN     = forgeEntities.UNKNOWN
	GITHUB      = forgeEntities.GITHUB
	GITLAB      = forgeEntities.GITLAB
	AZUREDEVOPS = forgeEntities.AZUREDEVOPS
	BITBUCKET   = forgeEntities.BITBUCKET
	CODECOMMIT  = forgeEntities.CODECOMMIT
)

// --- BranchStatus constants ---

const (
	BranchCreated      = forgeEntities.BranchCreated
	BranchExistsWithPR = forgeEntities.BranchExistsWithPR
	BranchExistsNoPR   = forgeEntities.BranchExistsNoPR
)

// --- Changelog constants ---

const InitialReleaseVersion = forgeEntities.InitialReleaseVersion

// --- Changelog errors ---

var (
	ErrNoVersionFoundInChangelog  = forgeEntities.ErrNoVersionFoundInChangelog
	ErrNoChangesFoundInUnreleased = forgeEntities.ErrNoChangesFoundInUnreleased
)

// --- Config errors ---

var (
	ErrConfigFileNotFound = forgeEntities.ErrConfigFileNotFound
	ErrConfigKeyMissing   = forgeEntities.ErrConfigKeyMissing
)

// --- Changelog function wrappers ---

// IsChangelogUnreleasedEmpty checks whether the unreleased section is empty.
func IsChangelogUnreleasedEmpty(lines []string) (bool, error) {
	return forgeEntities.IsChangelogUnreleasedEmpty(lines)
}

// FindLatestVersion finds the latest version in changelog lines.
func FindLatestVersion(lines []string) (*semver.Version, error) {
	return forgeEntities.FindLatestVersion(lines)
}

// ProcessChangelog processes changelog lines and returns the next version and new content.
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	return forgeEntities.ProcessChangelog(lines)
}

// ProcessNewChangelog handles changelogs with only [Unreleased] section.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	return forgeEntities.ProcessNewChangelog(lines)
}

// MakeNewSectionsFromUnreleased creates new section contents for initial release.
func MakeNewSectionsFromUnreleased(unreleasedSection []string, version semver.Version) []string {
	return forgeEntities.MakeNewSectionsFromUnreleased(unreleasedSection, version)
}

// FixSectionHeadings fixes the section headings in the unreleased section.
func FixSectionHeadings(unreleasedSection []string) {
	forgeEntities.FixSectionHeadings(unreleasedSection)
}

// DeduplicateEntries removes duplicate and semantically overlapping entries.
func DeduplicateEntries(entries []string) []string {
	return forgeEntities.DeduplicateEntries(entries)
}

// MakeNewSections creates new section contents for the CHANGELOG file.
func MakeNewSections(sections map[string]*[]string, nextVersion semver.Version) []string {
	return forgeEntities.MakeNewSections(sections, nextVersion)
}

// ParseUnreleasedIntoSections parses the unreleased section into change type sections.
func ParseUnreleasedIntoSections(
	unreleasedSection []string,
	sections map[string]*[]string,
	currentSection *[]string,
	majorChanges, minorChanges, patchChanges *int,
) {
	forgeEntities.ParseUnreleasedIntoSections(
		unreleasedSection, sections, currentSection,
		majorChanges, minorChanges, patchChanges,
	)
}

// UpdateSection updates the unreleased section and calculates the next version.
func UpdateSection(
	unreleasedSection []string,
	nextVersion semver.Version,
) ([]string, *semver.Version, error) {
	return forgeEntities.UpdateSection(unreleasedSection, nextVersion)
}

// InsertChangelogEntry inserts entries into the changelog under Unreleased/Changed.
func InsertChangelogEntry(content string, entries []string) string {
	return forgeEntities.InsertChangelogEntry(content, entries)
}

// --- Config function wrappers ---

// ResolveToken expands ${ENV_VAR} references and reads from file if path exists.
func ResolveToken(raw string) string {
	return forgeEntities.ResolveToken(raw)
}

// ValidateProviders validates provider configuration entries.
func ValidateProviders(providers []ProviderConfig) error {
	return forgeEntities.ValidateProviders(providers)
}
