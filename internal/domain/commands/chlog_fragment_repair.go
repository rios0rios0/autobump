package commands

import (
	"regexp"
	"strings"
)

// chlogTopLevelKeyRegex matches the line that opens a top-level mapping entry of a fragment
// ("body: ..."), capturing the key and everything after the separator.
var chlogTopLevelKeyRegex = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):[ \t]+(.*)$`)

// repairChlogFragmentQuoting re-escapes the apostrophes inside the single-quoted top-level
// values of a fragment, answering the repaired document and whether anything changed.
//
// A single-quoted YAML scalar escapes an apostrophe by doubling it, and a hand-written or
// script-written fragment often forgets to: `body: 'the sweep's notices'` ends the string at
// "sweep" and fails the whole parse with "mapping values are not allowed" -- which stopped
// every release of a repository over one entry. The value of each such entry runs from its
// opening quote to the last quote before the next top-level key, so its text is recoverable:
// the outer quotes are stripped, the apostrophes that were already doubled are folded back,
// and every apostrophe is doubled again. A value that was escaped correctly comes out
// byte-for-byte the same.
//
// It is only ever applied to a document that already failed to parse, and its answer is
// parsed again by the caller, so a repair that does not produce valid YAML changes nothing.
func repairChlogFragmentQuoting(data string) (string, bool) {
	lines := strings.Split(data, "\n")
	repaired := make([]string, 0, len(lines))
	changed := false

	for index := 0; index < len(lines); {
		match := chlogTopLevelKeyRegex.FindStringSubmatch(lines[index])
		if match == nil || !strings.HasPrefix(match[2], "'") {
			repaired = append(repaired, lines[index])
			index++
			continue
		}

		end := index + 1
		for end < len(lines) && !chlogTopLevelKeyRegex.MatchString(lines[end]) {
			end++
		}
		entry := append([]string{match[2]}, lines[index+1:end]...)
		value := strings.TrimRight(strings.Join(entry, "\n"), " \t\n")

		if len(value) < len("''") || !strings.HasSuffix(value, "'") {
			repaired = append(repaired, lines[index:end]...)
			index = end
			continue
		}

		inner := value[1 : len(value)-1]
		escaped := strings.ReplaceAll(strings.ReplaceAll(inner, "''", "'"), "'", "''")
		if escaped != inner {
			changed = true
		}
		rebuilt := strings.Split(match[1]+": '"+escaped+"'", "\n")
		repaired = append(repaired, rebuilt...)
		// Keep the blank lines that trailed the entry, so the document keeps its shape.
		for trailing := end - 1; trailing > index && strings.TrimSpace(lines[trailing]) == ""; trailing-- {
			repaired = append(repaired, "")
		}
		index = end
	}

	return strings.Join(repaired, "\n"), changed
}
