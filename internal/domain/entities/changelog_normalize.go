package entities

import (
	"regexp"
	"slices"
	"strings"

	changelogEntities "github.com/rios0rios0/gitforge/pkg/changelog/domain/entities"
)

// The six Keep a Changelog sections. They are the only headings the release pipeline
// renders, so an entry that ends up anywhere else never reaches a release.
const (
	SectionAdded      = "Added"
	SectionChanged    = "Changed"
	SectionDeprecated = "Deprecated"
	SectionFixed      = "Fixed"
	SectionRemoved    = "Removed"
	SectionSecurity   = "Security"
)

// UnreleasedHeaderName is the name carried by the "## [Unreleased]" heading, as captured
// by a Keep a Changelog version header.
const UnreleasedHeaderName = "Unreleased"

// changelogFoldSeparator joins an entry's continuation lines onto its bullet so the whole
// entry travels as a single line. It is a NUL surrounded by spaces: NUL does not occur in
// changelog prose, and the spaces leave it a one-character word, which the similarity
// comparison discards along with every other one-character token -- so folding an entry
// does not change how it compares against the others. Folding is undone before anything is
// written, so the separator never reaches disk.
const changelogFoldSeparator = " \x00 "

// changelogSectionOrder is the order sections are rendered in. It mirrors the order the
// release renderer uses, so a normalised [Unreleased] section and the release section
// built from it come out in the same order and normalising twice changes nothing.
//
//nolint:gochecknoglobals // constant-like ordered list
var changelogSectionOrder = []string{
	SectionAdded,
	SectionChanged,
	SectionDeprecated,
	SectionFixed,
	SectionRemoved,
	SectionSecurity,
}

// canonicalChangelogSections maps a lower-cased heading word to its canonical section.
//
//nolint:gochecknoglobals // read-only lookup table
var canonicalChangelogSections = map[string]string{
	"added":      SectionAdded,
	"changed":    SectionChanged,
	"deprecated": SectionDeprecated,
	"fixed":      SectionFixed,
	"removed":    SectionRemoved,
	"security":   SectionSecurity,
}

// changelogSectionHeadingRegex matches a section heading whatever depth and casing it was
// written with. Repairing those is not cosmetic: the release renderer buckets entries by
// the exact string "### Added", so "#### added" silently drops every entry underneath it.
var changelogSectionHeadingRegex = regexp.MustCompile(
	`(?i)^\s*#{1,6}\s*(added|changed|deprecated|fixed|removed|security)\b`,
)

// changelogVersionHeaderRegex matches a Keep a Changelog version header line.
var changelogVersionHeaderRegex = regexp.MustCompile(`^\s*##\s*\[([^\]]+)\]`)

// changelogEntry is one logical entry: the bullet that opens it plus the continuation
// lines that belong to it.
//
// Grouping them is what makes the rules correct. Every rule below moves an entry --
// deduplication drops it, verb reclassification files it elsewhere, ordering re-places it
// -- and a continuation line judged on its own gets torn away from the bullet it explains.
// A fragment body wrapped over two lines, whose second line happens to start with
// "removed", would otherwise be published as an orphaned line under "### Removed".
type changelogEntry struct {
	bullet       string
	continuation []string
}

// changelogSection is one "### <Section>" block: any prose a writer put between the
// heading and the first bullet, then the entries themselves.
type changelogSection struct {
	prose   []string
	entries []changelogEntry
}

// MatchChangelogVersionHeader returns the name a Keep a Changelog "## [x]" header carries
// and whether the line is such a header.
func MatchChangelogVersionHeader(line string) (string, bool) {
	match := changelogVersionHeaderRegex.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}

	return match[1], true
}

// NormalizeUnreleasedSection applies the changelog business rules to the [Unreleased]
// section and returns the document with that section rewritten. Released sections are
// history and are returned verbatim.
//
// The rules are the ones a release has always applied to a hand-written changelog:
// headings are repaired, breaking-change markers are written in the one spelling the bump
// counter understands, identical and near-identical entries collapse, an entry filed under
// the wrong section is moved to the one its verb names, and entries are ordered. Running
// them here rather than only inside the SemVer pipeline is what extends them to every
// release path -- fork versioning, which rewrites the section without consulting that
// pipeline at all, and chlog fragments, which arrive as many independent files that no
// human ever saw side by side.
//
// It is idempotent: normalising an already normalised section changes nothing, which
// matters because the changelog is read several times per run.
func NormalizeUnreleasedSection(lines []string) []string {
	headerIdx, nextIdx := unreleasedBounds(lines)
	if headerIdx == -1 {
		return lines
	}

	body := normalizeUnreleasedBody(lines[headerIdx+1 : nextIdx])
	if body == nil {
		return lines
	}

	normalized := make([]string, 0, len(lines)+len(body))
	normalized = append(normalized, lines[:headerIdx+1]...)
	normalized = append(normalized, "")
	normalized = append(normalized, body...)
	if nextIdx < len(lines) {
		normalized = append(normalized, "")
		normalized = append(normalized, lines[nextIdx:]...)
	}

	return normalized
}

// unreleasedBounds locates the [Unreleased] header and the version header that follows it.
// The first index is -1 when the document has no [Unreleased] section.
func unreleasedBounds(lines []string) (int, int) {
	headerIdx := -1

	for i, line := range lines {
		name, isHeader := MatchChangelogVersionHeader(line)
		if !isHeader {
			continue
		}
		if headerIdx == -1 {
			if name == UnreleasedHeaderName {
				headerIdx = i
			}
			continue
		}

		return headerIdx, i
	}

	return headerIdx, len(lines)
}

// normalizeUnreleasedBody applies the rules to the body of the [Unreleased] section and
// renders it back. It returns nil when the body holds nothing the rules recognise, which
// leaves an unusual section untouched rather than rewriting it into something else.
func normalizeUnreleasedBody(body []string) []string {
	preamble, sections := parseUnreleasedBody(body)
	if len(sections) == 0 {
		return nil
	}

	applyChangelogRules(sections)

	return renderUnreleasedBody(preamble, sections)
}

// parseUnreleasedBody splits the body into the lines that precede any section heading and
// the sections themselves. Repeated headings for the same section are merged, which is what
// a changelog looks like once chlog fragments have been spliced next to hand-written
// entries that use the same headings.
func parseUnreleasedBody(body []string) ([]string, map[string]*changelogSection) {
	var preamble []string
	sections := make(map[string]*changelogSection, len(changelogSectionOrder))
	var current *changelogSection

	for _, line := range body {
		if name, isHeading := matchChangelogSectionHeading(line); isHeading {
			if sections[name] == nil {
				sections[name] = &changelogSection{}
			}
			current = sections[name]
			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		switch {
		case current == nil:
			preamble = append(preamble, line)
		case isChangelogBullet(line):
			current.entries = append(current.entries, changelogEntry{bullet: line})
		case len(current.entries) == 0:
			current.prose = append(current.prose, line)
		default:
			last := &current.entries[len(current.entries)-1]
			last.continuation = append(last.continuation, line)
		}
	}

	return preamble, sections
}

// applyChangelogRules runs the rules over the parsed sections, in the order that lets each
// one see the repairs made by the previous: markers are canonicalised first so two entries
// that differ only in how they spell "breaking change" collapse and so reclassification
// recognises a breaking entry and leaves it alone; deduplication runs again after
// reclassification because moving entries can bring duplicates together for the first time.
func applyChangelogRules(sections map[string]*changelogSection) {
	for _, section := range sections {
		normalizeSectionMarkers(section)
		section.entries = deduplicateChangelogEntries(section.entries)
	}

	reclassifyChangelogEntries(sections)

	for _, section := range sections {
		section.entries = deduplicateChangelogEntries(section.entries)
		slices.SortStableFunc(section.entries, func(a, b changelogEntry) int {
			return strings.Compare(strings.ToLower(a.bullet), strings.ToLower(b.bullet))
		})
	}
}

// normalizeSectionMarkers rewrites every breaking-change marker in a section into the one
// spelling the bump counter recognises.
func normalizeSectionMarkers(section *changelogSection) {
	for i := range section.entries {
		text := strings.TrimPrefix(strings.TrimSpace(section.entries[i].bullet), "-")
		normalized := NormalizeBreakingChangeMarker(strings.TrimSpace(text), false)
		section.entries[i].bullet = "- " + normalized
	}
}

// deduplicateChangelogEntries removes entries the deduplication rules judge redundant --
// exact repeats and entries whose significant words overlap almost entirely.
//
// The decision is delegated by comparing the bullets alone: the rule is shared with the
// hand-written path and must not fork. Survivors come back in their original order, so
// walking both lists together restores each surviving bullet's continuation lines. Bullets
// are unique within a section once the exact repeats are gone, so that pairing is
// unambiguous.
func deduplicateChangelogEntries(entries []changelogEntry) []changelogEntry {
	if len(entries) <= 1 {
		return entries
	}

	bullets := make([]string, len(entries))
	for i, entry := range entries {
		bullets[i] = entry.bullet
	}

	kept := changelogEntities.DeduplicateEntries(bullets)

	result := make([]changelogEntry, 0, len(kept))
	next := 0
	for i, entry := range entries {
		if next < len(kept) && bullets[i] == kept[next] {
			result = append(result, entry)
			next++
		}
	}

	return result
}

// reclassifyChangelogEntries moves each entry to the section its leading verb names, so an
// entry opening "removed ..." filed under "### Changed" is published under "### Removed".
//
// The rule itself is delegated, one entry at a time: asking where a single entry belongs
// makes the answer unambiguous, where handing over a whole section would return a list of
// strings that could no longer be paired back with their continuation lines.
func reclassifyChangelogEntries(sections map[string]*changelogSection) {
	for _, name := range changelogSectionOrder {
		section := sections[name]
		if section == nil {
			continue
		}

		kept := make([]changelogEntry, 0, len(section.entries))
		for _, entry := range section.entries {
			target := sectionForEntry(entry.bullet, name)
			if target == name {
				kept = append(kept, entry)
				continue
			}
			if sections[target] == nil {
				sections[target] = &changelogSection{}
			}
			sections[target].entries = append(sections[target].entries, entry)
		}
		section.entries = kept
	}
}

// sectionForEntry reports which section an entry's leading verb calls for, given where it
// is filed today.
func sectionForEntry(bullet, current string) string {
	probe := make(map[string]*[]string, len(changelogSectionOrder))
	for _, name := range changelogSectionOrder {
		probe[name] = &[]string{}
	}
	*probe[current] = []string{bullet}

	changelogEntities.ReclassifyEntriesByVerb(probe)

	for _, name := range changelogSectionOrder {
		if len(*probe[name]) > 0 {
			return name
		}
	}

	return current
}

// renderUnreleasedBody writes the sections back as Keep a Changelog lines. It returns nil
// when no section holds anything, so a body of empty headings is left as it was found.
func renderUnreleasedBody(preamble []string, sections map[string]*changelogSection) []string {
	rendered := make([]string, 0, len(preamble))
	rendered = append(rendered, preamble...)

	written := false
	for _, name := range changelogSectionOrder {
		section := sections[name]
		if section == nil || (len(section.entries) == 0 && len(section.prose) == 0) {
			continue
		}

		if len(rendered) > 0 {
			rendered = append(rendered, "")
		}
		rendered = append(rendered, "### "+name, "")
		if len(section.prose) > 0 {
			rendered = append(rendered, section.prose...)
			if len(section.entries) > 0 {
				rendered = append(rendered, "")
			}
		}
		for _, entry := range section.entries {
			rendered = append(rendered, entry.bullet)
			rendered = append(rendered, entry.continuation...)
		}
		written = true
	}

	if !written {
		return nil
	}

	return rendered
}

// matchChangelogSectionHeading returns the canonical section a heading line names.
func matchChangelogSectionHeading(line string) (string, bool) {
	match := changelogSectionHeadingRegex.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}

	return canonicalChangelogSections[strings.ToLower(match[1])], true
}

// isChangelogBullet reports whether a line opens a new entry. Only an unindented bullet
// does: an indented one is a nested list item belonging to the entry above it.
func isChangelogBullet(line string) bool {
	return strings.HasPrefix(line, "- ")
}

// foldUnreleasedEntries collapses every entry in the [Unreleased] section onto a single
// line so the SemVer pipeline, which reads one entry per line, sees whole entries.
//
// Without this the pipeline treats a continuation line as an entry of its own: it counts
// as a change, it is compared for duplication against real entries, and -- if it happens to
// start with "removed", "added", "fixed" or "deprecated" -- it is moved to another section,
// leaving the bullet it explained behind. Folding is undone on the way out.
func foldUnreleasedEntries(lines []string) []string {
	headerIdx, nextIdx := unreleasedBounds(lines)
	if headerIdx == -1 {
		return lines
	}

	folded := make([]string, 0, len(lines))
	folded = append(folded, lines[:headerIdx+1]...)
	folded = append(folded, foldEntries(lines[headerIdx+1:nextIdx])...)
	folded = append(folded, lines[nextIdx:]...)

	return folded
}

// foldEntries joins each bullet with the continuation lines that follow it. Blank lines are
// dropped: the pipeline ignores them and re-emits its own.
func foldEntries(body []string) []string {
	folded := make([]string, 0, len(body))
	open := -1

	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case isChangelogBullet(line):
			folded = append(folded, line)
			open = len(folded) - 1
		case strings.HasPrefix(trimmed, "#"):
			folded = append(folded, line)
			open = -1
		case open >= 0:
			folded[open] += changelogFoldSeparator + line
		default:
			folded = append(folded, line)
		}
	}

	return folded
}

// unfoldChangelogEntries splits folded entries back into their original lines. It runs over
// the whole document because the pipeline moves the folded entries into the release section
// it writes.
func unfoldChangelogEntries(lines []string) []string {
	unfolded := make([]string, 0, len(lines))

	for _, line := range lines {
		if !strings.Contains(line, changelogFoldSeparator) {
			unfolded = append(unfolded, line)
			continue
		}
		unfolded = append(unfolded, strings.Split(line, changelogFoldSeparator)...)
	}

	return unfolded
}
