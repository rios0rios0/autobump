package domain

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

// DefaultChangelogURL is the URL of the default CHANGELOG template.
const DefaultChangelogURL = "https://raw.githubusercontent.com/rios0rios0/" +
	"autobump/main/configs/CHANGELOG.template.md"

// InitialReleaseVersion is the version used when no version is found in the changelog.
// When a changelog only has [Unreleased] section, we bump directly to 1.0.0.
const InitialReleaseVersion = "1.0.0"

var (
	ErrNoVersionFoundInChangelog  = errors.New("no version found in the changelog")
	ErrNoChangesFoundInUnreleased = errors.New("no changes found in the unreleased section")
)

// IsChangelogUnreleasedEmpty checks whether the unreleased section of the changelog is empty.
func IsChangelogUnreleasedEmpty(lines []string) (bool, error) {
	latestVersion, err := FindLatestVersion(lines)
	// If no version found, check if unreleased section has content
	noVersionFound := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !noVersionFound {
		return true, err
	}

	unreleased := false
	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if !noVersionFound &&
			strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			// Only stop at the version section if we found a version
			unreleased = false
		}

		if unreleased {
			re := regexp.MustCompile(`^\s*-\s*[^ ]+`)
			if match := re.MatchString(line); match {
				return false, nil
			}
		}
	}

	return true, nil
}

// FindLatestVersion finds the latest version in the changelog lines.
func FindLatestVersion(lines []string) (*semver.Version, error) {
	// Regular expression to match version lines
	versionRegex := regexp.MustCompile(`^\s*##\s*\[([^\]]+)\]`)

	var latestVersion *semver.Version
	for _, line := range lines {
		if versionMatch := versionRegex.FindStringSubmatch(line); versionMatch != nil {
			// Skip the "Unreleased" version
			if versionMatch[1] == "Unreleased" {
				continue
			}

			version, err := semver.NewVersion(versionMatch[1])
			if err != nil {
				log.Errorf("Error parsing version '%s': %v", versionMatch[1], err)
				return nil, fmt.Errorf("error parsing version '%s': %w", versionMatch[1], err)
			}

			if latestVersion == nil || version.GreaterThan(latestVersion) {
				latestVersion = version
			}
		}
	}

	if latestVersion == nil {
		return nil, ErrNoVersionFoundInChangelog
	}

	return latestVersion, nil
}

// ProcessChangelog processes the changelog lines and returns the next version and the new content.
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	// Variables to hold the new content
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	// Find the latest version in the changelog
	latestVersion, err := FindLatestVersion(lines)
	isNewChangelog := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !isNewChangelog {
		log.Errorf("Error finding latest version: %v", err)
		return nil, nil, err
	}

	// For new changelogs (only [Unreleased] section), bump directly to 1.0.0
	if isNewChangelog {
		log.Infof("No previous version found, will release as %s", InitialReleaseVersion)
		return ProcessNewChangelog(lines)
	}

	log.Infof("Previous version: %s", latestVersion)
	nextVersion := *latestVersion

	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			unreleased = false
			if len(unreleasedSection) > 0 {
				// Process the unreleased section
				var updatedSection []string
				var updatedVersion *semver.Version
				updatedSection, updatedVersion, err = UpdateSection(unreleasedSection, nextVersion)
				if err != nil {
					log.Errorf("Error updating section: %v", err)
					return nil, nil, err
				}
				// Add the updated section to the new content
				newContent = append(newContent, updatedSection...)
				unreleasedSection = nil
				nextVersion = *updatedVersion
			}
		}

		if unreleased {
			unreleasedSection = append(unreleasedSection, line)
		} else {
			newContent = append(newContent, line)
		}
	}

	log.Infof("Next calculated version: %s", nextVersion)
	return &nextVersion, newContent, nil
}

// ProcessNewChangelog handles changelogs that only have [Unreleased] section.
// It bumps directly to 1.0.0 without calculating based on changes.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		}

		if unreleased {
			unreleasedSection = append(unreleasedSection, line)
		} else {
			newContent = append(newContent, line)
		}
	}

	// Create the initial release version
	initialVersion, _ := semver.NewVersion(InitialReleaseVersion)

	if len(unreleasedSection) > 0 {
		// Fix section headings
		FixSectionHeadings(unreleasedSection)

		// Create new section for 1.0.0 release
		newSection := MakeNewSectionsFromUnreleased(unreleasedSection, *initialVersion)
		newContent = append(newContent, newSection...)
	}

	log.Infof("Next calculated version: %s", InitialReleaseVersion)
	return initialVersion, newContent, nil
}

// MakeNewSectionsFromUnreleased creates new section contents for initial release.
func MakeNewSectionsFromUnreleased(unreleasedSection []string, version semver.Version) []string {
	var newSection []string

	// Create a new unreleased section
	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")

	// Create the new section with the version and the current date
	newSection = append(
		newSection,
		fmt.Sprintf("## [%s] - %s", version.String(), time.Now().Format("2006-01-02")),
	)
	newSection = append(newSection, "")

	// Copy content from unreleased section (skip the [Unreleased] header)
	for _, line := range unreleasedSection {
		if !strings.Contains(line, "[Unreleased]") {
			newSection = append(newSection, line)
		}
	}

	return newSection
}

// FixSectionHeadings fixes the section headings in the unreleased section.
func FixSectionHeadings(unreleasedSection []string) {
	re := regexp.MustCompile(`(?i)^\s*#+\s*(Added|Changed|Deprecated|Removed|Fixed|Security)`)
	for i, line := range unreleasedSection {
		if re.MatchString(line) {
			correctedLine := "### " + strings.TrimSpace(strings.ReplaceAll(line, "#", ""))
			unreleasedSection[i] = correctedLine
		}
	}
}

// MakeNewSections creates new section contents for the beginning of the CHANGELOG file.
func MakeNewSections(
	sections map[string]*[]string,
	nextVersion semver.Version,
) []string {
	var newSection []string
	// Create a new unreleased section
	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")

	// Create the new section with the next version and the current date
	newSection = append(
		newSection,
		fmt.Sprintf("## [%s] - %s", nextVersion.String(), time.Now().Format("2006-01-02")),
	)
	// add a blank line between sections
	newSection = append(newSection, "")

	// Add the sections to the newly created release section
	keys := []string{"Added", "Changed", "Deprecated", "Fixed", "Removed", "Security"}
	for _, key := range keys {
		section := sections[key]

		// Append sections only if they have content
		if len(*section) > 0 {
			newSection = append(newSection, "### "+key)
			newSection = append(newSection, "")
			newSection = append(newSection, *section...)
			newSection = append(newSection, "")
		}
	}
	return newSection
}

// ParseUnreleasedIntoSections parses the unreleased section into change type sections.
func ParseUnreleasedIntoSections(
	unreleasedSection []string,
	sections map[string]*[]string,
	currentSection *[]string,
	majorChanges, minorChanges, patchChanges *int,
) {
	for _, line := range unreleasedSection {
		trimmedLine := strings.TrimSpace(line)

		// Check if the line is a section header
		for header := range sections {
			if strings.HasPrefix(trimmedLine, "### "+header) {
				currentSection = sections[header]
			}
		}

		// If the line is not empty, and not a section header, add it to the current section
		if currentSection != nil && trimmedLine != "" && trimmedLine != "-" &&
			!strings.HasPrefix(trimmedLine, "##") {
			*currentSection = append(*currentSection, line)

			// Increment the change counters based on the line content
			switch {
			case strings.HasPrefix(line, "- **BREAKING CHANGE:**"):
				*majorChanges++
			case currentSection == sections["Added"]:
				*minorChanges++
			default:
				*patchChanges++
			}
		}
	}
}

// UpdateSection updates the unreleased section and calculates the next version.
func UpdateSection(
	unreleasedSection []string,
	nextVersion semver.Version,
) ([]string, *semver.Version, error) {
	// Fix the section headings
	FixSectionHeadings(unreleasedSection)

	sections := map[string]*[]string{
		"Added":      {},
		"Changed":    {},
		"Deprecated": {},
		"Removed":    {},
		"Fixed":      {},
		"Security":   {},
	}

	var currentSection *[]string
	majorChanges, minorChanges, patchChanges := 0, 0, 0

	ParseUnreleasedIntoSections(
		unreleasedSection,
		sections,
		currentSection,
		&majorChanges,
		&minorChanges,
		&patchChanges,
	)

	// If no changes were found, return an error
	if majorChanges == 0 && minorChanges == 0 && patchChanges == 0 {
		return nil, nil, ErrNoChangesFoundInUnreleased
	}

	switch {
	case majorChanges > 0:
		nextVersion = nextVersion.IncMajor()
	case minorChanges > 0:
		nextVersion = nextVersion.IncMinor()
	case patchChanges > 0:
		nextVersion = nextVersion.IncPatch()
	}

	// Sort the items inside the sections alphabetically
	for _, section := range sections {
		sort.Strings(*section)
	}

	newSection := MakeNewSections(sections, nextVersion)
	return newSection, &nextVersion, nil
}
