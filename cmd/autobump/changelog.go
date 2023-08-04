package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

func updateChangelogFile(changelogPath string) (*semver.Version, error) {
	lines, err := readLines(changelogPath)
	if err != nil {
		return nil, err
	}

	version, newContent, err := processChangelog(lines)
	if err != nil {
		return nil, err
	}

	err = writeLines(changelogPath, newContent)
	if err != nil {
		return nil, err
	}

	return version, nil
}

func findLatestVersion(lines []string) (*semver.Version, error) {
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
				return nil, err
			}

			if latestVersion == nil || version.GreaterThan(latestVersion) {
				latestVersion = version
			}
		}
	}

	if latestVersion == nil {
		err := fmt.Errorf("no version found in changelog")
		log.Errorf("Error: %v", err)
		return nil, err
	}

	return latestVersion, nil
}

func processChangelog(lines []string) (*semver.Version, []string, error) {
	// Variables to hold the new content
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	// Find the latest version in the changelog
	latestVersion, err := findLatestVersion(lines)
	if err != nil {
		log.Errorf("Error finding latest version: %v", err)
		return nil, nil, err
	}
	log.Infof("Previous version: %s", latestVersion)

	nextVersion := latestVersion.IncPatch()
	for _, line := range lines {

		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			unreleased = false
			if len(unreleasedSection) > 0 {
				// Process the unreleased section
				updatedSection, updatedVersion, err := updateSection(unreleasedSection, nextVersion)
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

func updateSection(unreleasedSection []string, nextVersion semver.Version) ([]string, *semver.Version, error) {
	var newSection []string
	var currentSection *[]string
	sections := map[string]*[]string{
		"Added":   {},
		"Changed": {},
		"Removed": {},
	}

	majorChanges := 0
	minorChanges := 0
	patchChanges := 0

	for _, line := range unreleasedSection {
		trimmedLine := strings.TrimSpace(line)
		switch {
		case strings.Contains(line, "### Added"):
			currentSection = sections["Added"]
		case strings.Contains(line, "### Changed"):
			currentSection = sections["Changed"]
		case strings.Contains(line, "### Removed"):
			currentSection = sections["Removed"]
		default:
			if currentSection != nil && trimmedLine != "" && trimmedLine != "-" && !strings.HasPrefix(trimmedLine, "##") {
				*currentSection = append(*currentSection, line)
				if strings.HasPrefix(line, "**BREAKING CHANGE: **") {
					majorChanges++
				} else if currentSection == sections["Added"] {
					minorChanges++
				} else {
					patchChanges++
				}
			}
		}
	}

	switch {
	case majorChanges > 0:
		for i := 1; i < majorChanges; i++ {
			nextVersion = nextVersion.IncMajor()
		}
		break
	case minorChanges > 0:
		for i := 1; i < minorChanges; i++ {
			nextVersion = nextVersion.IncMinor()
		}
		break
	case patchChanges > 0:
		for i := 1; i < patchChanges; i++ {
			nextVersion = nextVersion.IncPatch()
		}
		break
	}

	// Sort the items inside the sections alphabetically
	for _, section := range sections {
		sort.Strings(*section)
	}

	// Create the new section with the next version and the current date
	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")
	newSection = append(newSection, "### Added")
	newSection = append(newSection, "")
	newSection = append(newSection, "-")
	newSection = append(newSection, "")
	newSection = append(newSection, "### Changed")
	newSection = append(newSection, "")
	newSection = append(newSection, "-")
	newSection = append(newSection, "")
	newSection = append(newSection, "### Removed")
	newSection = append(newSection, "")
	newSection = append(newSection, "-")
	newSection = append(newSection, "")

	newSection = append(newSection, fmt.Sprintf("## [%s] - %s", nextVersion.String(), time.Now().Format("2006-01-02")))
	newSection = append(newSection, "") // add a blank line between sections

	keys := []string{"Added", "Changed", "Removed"}
	for _, key := range keys {
		section := sections[key]

		// Append sections only if they have content
		if len(*section) > 0 {
			newSection = append(newSection, fmt.Sprintf("### %s", key))
			newSection = append(newSection, "")
			newSection = append(newSection, *section...)
			newSection = append(newSection, "")
		}
	}

	return newSection, &nextVersion, nil
}
