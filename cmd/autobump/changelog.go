package main

import (
	"bufio"
	"fmt"
	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

func UpdateChangelogFile(filePath string) (*semver.Version, error) {
	log.Info("Reading lines from the changelog file")
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	log.Info("Processing the changelog content")
	version, newContent, err := processChangelog(lines)
	if err != nil {
		return nil, err
	}

	log.Info("Writing the new content to the changelog file")
	err = writeLines(filePath, newContent)
	if err != nil {
		return nil, err
	}

	log.Info("Changelog file updated successfully")
	return version, nil
}

func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}

func findLatestVersion(lines []string) (*semver.Version, error) {
	log.Info("Starting findLatestVersion")

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

	log.Infof("Latest version found: %s", latestVersion)
	log.Info("Finished findLatestVersion")
	return latestVersion, nil
}

func processChangelog(lines []string) (*semver.Version, []string, error) {
	log.Info("Starting processChangelog")

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
	log.Infof("Latest version found: %s", latestVersion)

	nextVersion := latestVersion.IncPatch()
	log.Infof("Next version: %s", nextVersion)

	for _, line := range lines {

		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
			log.Info("Unreleased section found")
		} else if strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			unreleased = false
			log.Info("End of unreleased section")
			if len(unreleasedSection) > 0 {
				// Process the unreleased section
				log.Info("Processing unreleased section")
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

	log.Info("Finished processChangelog")
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

	for title, section := range sections {
		// Append sections only if they have content
		if len(*section) > 0 {
			newSection = append(newSection, fmt.Sprintf("### %s", title))
			newSection = append(newSection, "") // add a blank line between sections
			newSection = append(newSection, *section...)
		}
	}

	newSection = append(newSection, "")
	return newSection, &nextVersion, nil
}
