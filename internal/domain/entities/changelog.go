package entities

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

		// Parse into sections, deduplicate, and rebuild
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
			unreleasedSection, sections, currentSection,
			&majorChanges, &minorChanges, &patchChanges,
		)

		// Deduplicate entries in each section
		for _, section := range sections {
			*section = DeduplicateEntries(*section)
		}

		// Check if any sections have content after dedup
		hasContent := false
		for _, section := range sections {
			if len(*section) > 0 {
				hasContent = true
				break
			}
		}

		if hasContent {
			newSection := MakeNewSections(sections, *initialVersion)
			newContent = append(newContent, newSection...)
		} else {
			// Fallback: use original unreleased content
			newSection := MakeNewSectionsFromUnreleased(unreleasedSection, *initialVersion)
			newContent = append(newContent, newSection...)
		}
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

// deduplicationOverlapThreshold is the minimum overlap ratio to consider two entries as duplicates.
const deduplicationOverlapThreshold = 0.6

// stopWords are common words stripped during tokenization for similarity comparison.
//
//nolint:gochecknoglobals // constant-like lookup table
var stopWords = map[string]bool{
	"the": true, "to": true, "and": true, "all": true, "their": true,
	"its": true, "a": true, "an": true, "of": true, "in": true,
	"for": true, "with": true, "from": true, "by": true, "on": true,
	"is": true, "was": true, "are": true, "were": true, "be": true,
	"been": true, "being": true, "has": true, "have": true, "had": true,
	"that": true, "this": true, "it": true, "as": true,
}

// backtickPattern matches backtick-wrapped content.
var backtickPattern = regexp.MustCompile("`[^`]*`")

// versionPattern matches semver-like version numbers (e.g., 1.26.0, v2.3.1).
var versionPattern = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)?`)

// normalizeEntry strips a changelog entry down to its semantic core for comparison.
// It removes the leading "- ", backtick-wrapped content, version numbers, and lowercases.
func normalizeEntry(entry string) string {
	s := strings.TrimSpace(entry)
	s = strings.TrimPrefix(s, "- ")
	s = backtickPattern.ReplaceAllString(s, "")
	s = versionPattern.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	// collapse whitespace
	return strings.Join(strings.Fields(s), " ")
}

// tokenize splits a normalized entry into significant words, removing stop words.
func tokenize(normalized string) []string {
	words := strings.Fields(normalized)
	var tokens []string
	for _, w := range words {
		if !stopWords[w] && len(w) > 1 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// extractMaxVersion finds the highest semver version mentioned in an entry's raw text.
// Returns nil if no version is found.
func extractMaxVersion(entry string) *semver.Version {
	matches := versionPattern.FindAllString(entry, -1)
	var maxVer *semver.Version
	for _, m := range matches {
		v, err := semver.NewVersion(m)
		if err != nil {
			continue
		}
		if maxVer == nil || v.GreaterThan(maxVer) {
			maxVer = v
		}
	}
	return maxVer
}

// overlapRatio computes the token overlap ratio between two token slices.
// It returns len(intersection) / min(len(a), len(b)).
// Returns 0 if either slice is empty.
func overlapRatio(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}

	intersection := 0
	for _, t := range b {
		if set[t] {
			intersection++
		}
	}

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	return float64(intersection) / float64(minLen)
}

// DeduplicateEntries removes duplicate and semantically overlapping changelog entries.
// Phase 1 removes exact duplicates (keeping first occurrence).
// Phase 2 detects entries about the same topic using token overlap and keeps
// the most specific one (highest version mentioned, or longest).
func DeduplicateEntries(entries []string) []string {
	if len(entries) <= 1 {
		return entries
	}

	// Phase 1: exact dedup (preserve order, keep first)
	seen := make(map[string]bool, len(entries))
	var unique []string
	for _, e := range entries {
		normalized := strings.TrimSpace(e)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		unique = append(unique, e)
	}

	if len(unique) <= 1 {
		return unique
	}

	// Phase 2: semantic overlap dedup
	// Pre-compute tokens for each entry
	type entryInfo struct {
		raw    string
		tokens []string
		ver    *semver.Version
	}

	infos := make([]entryInfo, len(unique))
	for i, e := range unique {
		infos[i] = entryInfo{
			raw:    e,
			tokens: tokenize(normalizeEntry(e)),
			ver:    extractMaxVersion(e),
		}
	}

	// Mark entries to remove (index -> true)
	removed := make(map[int]bool)

	for i := range infos {
		if removed[i] {
			continue
		}
		for j := i + 1; j < len(infos); j++ {
			if removed[j] {
				continue
			}

			ratio := overlapRatio(infos[i].tokens, infos[j].tokens)
			if ratio < deduplicationOverlapThreshold {
				continue
			}

			// Entries overlap -- determine which to keep
			loser := pickLoser(infos[i], infos[j], i, j)
			removed[loser] = true
		}
	}

	var result []string
	for i, info := range infos {
		if !removed[i] {
			result = append(result, info.raw)
		}
	}
	return result
}

// pickLoser decides which of two overlapping entries to remove.
// Returns the index of the entry that should be discarded.
func pickLoser(a, b struct {
	raw    string
	tokens []string
	ver    *semver.Version
}, idxA, idxB int,
) int {
	// Prefer entry with higher version
	switch {
	case a.ver != nil && b.ver != nil:
		if a.ver.GreaterThan(b.ver) {
			return idxB
		}
		if b.ver.GreaterThan(a.ver) {
			return idxA
		}
	case a.ver != nil:
		return idxB // a has a version, b doesn't
	case b.ver != nil:
		return idxA // b has a version, a doesn't
	}

	// Prefer longer (more specific) entry
	if len(a.raw) != len(b.raw) {
		if len(a.raw) > len(b.raw) {
			return idxB
		}
		return idxA
	}

	// Same length, keep first encountered
	return idxB
}

// recountChanges re-counts major/minor/patch changes from deduplicated sections.
func recountChanges(sections map[string]*[]string) (int, int, int) {
	major, minor, patch := 0, 0, 0
	for key, section := range sections {
		for _, line := range *section {
			switch {
			case strings.HasPrefix(line, "- **BREAKING CHANGE:**"):
				major++
			case key == "Added":
				minor++
			default:
				patch++
			}
		}
	}
	return major, minor, patch
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

	// Deduplicate entries in each section and recount changes
	for _, section := range sections {
		*section = DeduplicateEntries(*section)
	}
	majorChanges, minorChanges, patchChanges = recountChanges(sections)

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
