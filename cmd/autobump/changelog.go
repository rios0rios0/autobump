package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

const defaultChangelogUrl = "https://raw.githubusercontent.com/rios0rios0/" +
	"autobump/main/configs/CHANGELOG.template.md"

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

func getNextVersion(changelogPath string) (*semver.Version, error) {
	lines, err := readLines(changelogPath)
	if err != nil {
		return nil, err
	}

	version, _, err := processChangelog(lines)
	if err != nil {
		return nil, err
	}

	return version, nil
}

// createChangelogIfNotExists create an empty CHANGELOG file if it doesn't exist
func createChangelogIfNotExists(changelogPath string) (bool, error) {
	if _, err := os.Stat(changelogPath); os.IsNotExist(err) {
		log.Warnf("Creating empty CHANGELOG file at '%s'.", changelogPath)
		fileContent, err := downloadFile(defaultChangelogUrl)
		if err != nil {
			log.Errorf("It wasn't possible to download the CHANGELOG model file: %v", err)
		}

		err = os.WriteFile(changelogPath, fileContent, 0o644)
		if err != nil {
			log.Errorf("Error creating CHANGELOG file: %v", err)
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func isChangelogUnreleasedEmpty(changelogPath string) (bool, error) {
	lines, err := readLines(changelogPath)
	if err != nil {
		return false, err
	}

	latestVersion, err := findLatestVersion(lines)
	if err != nil {
		return false, err
	}

	unreleased := false
	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
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

	nextVersion := *latestVersion
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

func updateSection(
	unreleasedSection []string,
	nextVersion semver.Version,
) ([]string, *semver.Version, error) {
	var newSection []string
	var currentSection *[]string
	sections := map[string]*[]string{
		"Added":      {},
		"Changed":    {},
		"Deprecated": {},
		"Removed":    {},
		"Fixed":      {},
		"Security":   {},
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
		case strings.Contains(line, "### Deprecated"):
			currentSection = sections["Deprecated"]
		case strings.Contains(line, "### Removed"):
			currentSection = sections["Removed"]
		case strings.Contains(line, "### Fixed"):
			currentSection = sections["Fixed"]
		case strings.Contains(line, "### Security"):
			currentSection = sections["Security"]
		default:
			if currentSection != nil && trimmedLine != "" && trimmedLine != "-" &&
				!strings.HasPrefix(trimmedLine, "##") {
				*currentSection = append(*currentSection, line)
				if strings.HasPrefix(line, "- **BREAKING CHANGE:**") {
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
		for i := 0; i < majorChanges; i++ {
			nextVersion = nextVersion.IncMajor()
		}
	case minorChanges > 0:
		for i := 0; i < minorChanges; i++ {
			nextVersion = nextVersion.IncMinor()
		}
	case patchChanges > 0:
		for i := 0; i < patchChanges; i++ {
			nextVersion = nextVersion.IncPatch()
		}
	}

	// Sort the items inside the sections alphabetically
	for _, section := range sections {
		sort.Strings(*section)
	}

	// Create the new section with the next version and the current date
	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")

	newSection = append(
		newSection,
		fmt.Sprintf("## [%s] - %s", nextVersion.String(), time.Now().Format("2006-01-02")),
	)
	// add a blank line between sections
	newSection = append(newSection, "")

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
