package entities

import (
	"regexp"
	"strings"
)

// BreakingChangePrefix is the single spelling of the breaking-change marker that the
// SemVer calculation recognises. The bump counter matches "- **BREAKING CHANGE:**" on the
// entry, and the verb-based reclassification leaves an entry where its author filed it
// only when it carries exactly that prefix.
//
// Every other spelling a writer reaches for -- "BREAKING CHANGE:", "**BREAKING CHANGE**:",
// "BREAKING-CHANGE:", the forms Conventional Commits teaches -- reads as breaking to a
// human and as an ordinary patch entry to the bumper, so entries are rewritten to this one.
const BreakingChangePrefix = "**BREAKING CHANGE:** "

// breakingChangeMarkerRegex matches one leading breaking-change marker, in any spelling
// that occurs in practice. There are two alternatives because the colon is what separates
// a marker from an ordinary sentence: an emphasised marker may carry the colon inside the
// emphasis ("**BREAKING CHANGE:**"), outside it ("**BREAKING CHANGE**:"), or -- since the
// emphasis already announces it -- not at all, while an unemphasised marker must have it.
// Without that rule an entry opening "breaking changes are documented below" would be
// mistaken for a marker and lose its first three words.
var breakingChangeMarkerRegex = regexp.MustCompile(
	`(?i)^[ \t]*(?:` +
		`(?:\*\*|__)[ \t]*breaking[ \t_-]?changes?[ \t]*:?[ \t]*(?:\*\*|__)[ \t]*:?` +
		`|breaking[ \t_-]?changes?[ \t]*:` +
		`)[ \t]*`,
)

// NormalizeBreakingChangeMarker rewrites an entry so that a breaking change is announced
// exactly once, in the canonical spelling.
//
// `breaking` is the flag the writer set on the change itself -- `chlog new --breaking`.
// Setting it and *also* opening the body with "BREAKING CHANGE:" is the natural thing to
// do, since that is how the same fact is written in a commit footer, and it is why the
// marker has to be stripped before the canonical one is put back: announcing the flag
// blindly would publish "**BREAKING CHANGE:** BREAKING CHANGE: ...". Markers are stripped
// repeatedly so a body that already carries a doubled one is repaired too.
//
// An entry with no marker and no flag is returned untouched, so this is safe to run over
// an entire changelog.
func NormalizeBreakingChangeMarker(text string, breaking bool) string {
	stripped, found := stripBreakingChangeMarkers(text)
	if !found && !breaking {
		return text
	}

	// A body that is nothing but the marker would otherwise render as a bare "- ".
	if stripped == "" {
		return strings.TrimSpace(BreakingChangePrefix)
	}

	return BreakingChangePrefix + stripped
}

// stripBreakingChangeMarkers removes every leading breaking-change marker and reports
// whether it removed any. The original text is returned unchanged when there was none, so
// a caller can tell "no marker" apart from "a marker and nothing else".
func stripBreakingChangeMarkers(text string) (string, bool) {
	stripped := text
	found := false

	for {
		match := breakingChangeMarkerRegex.FindStringIndex(stripped)
		if match == nil {
			break
		}
		stripped = stripped[match[1]:]
		found = true
	}

	if !found {
		return text, false
	}

	return strings.TrimSpace(stripped), true
}
