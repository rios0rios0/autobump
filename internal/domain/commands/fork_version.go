package commands

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// ErrInvalidForkVersion is returned when a version string does not match the
// expected fork pattern (X.Y.Z.N or X.Y.Z-N) for the requested mode.
var ErrInvalidForkVersion = errors.New("invalid fork version")

// forkRewriteOverhead is the maximum number of extra lines a fork-mode rewrite
// may inject into the changelog: an [Unreleased] header, a blank line, the
// new dated header, and, when another section follows, a trailing blank line.
const forkRewriteOverhead = 4

// forkVersionRegex matches the fork version forms supported by the bumper.
// Group 1: upstream X.Y.Z. Group 2: separator (. or -). Group 3: fork digit N.
//

var forkVersionRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)([.\-])(\d+)$`)

// changelogVersionHeaderRegex matches a Keep-a-Changelog version header line.
//

var changelogVersionHeaderRegex = regexp.MustCompile(`^\s*##\s*\[([^\]]+)\]`)

// ForkVersion is the parsed representation of a fork version string.
type ForkVersion struct {
	Upstream  string
	Separator string
	Fork      int
}

// String renders the fork version back to its canonical form.
func (v ForkVersion) String() string {
	return fmt.Sprintf("%s%s%d", v.Upstream, v.Separator, v.Fork)
}

// ParseForkVersion parses a fork version string into its components.
// A leading "v" prefix (e.g. v1.0.0.3) is tolerated but stripped before
// parsing. The mode constrains which separator is accepted; pass an empty
// mode to accept either.
func ParseForkVersion(s, mode string) (*ForkVersion, error) {
	trimmed := strings.TrimSpace(s)
	trimmed = strings.TrimPrefix(trimmed, "v")

	matches := forkVersionRegex.FindStringSubmatch(trimmed)
	if matches == nil {
		return nil, fmt.Errorf("%w: %q does not match X.Y.Z.N or X.Y.Z-N", ErrInvalidForkVersion, s)
	}

	separator := matches[2]
	if !separatorMatchesMode(separator, mode) {
		return nil, fmt.Errorf(
			"%w: separator %q does not match versioning mode %q",
			ErrInvalidForkVersion, separator, mode,
		)
	}

	forkDigit, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("%w: fork digit %q is not an integer", ErrInvalidForkVersion, matches[3])
	}

	return &ForkVersion{
		Upstream:  matches[1],
		Separator: separator,
		Fork:      forkDigit,
	}, nil
}

// NextForkVersion returns the next fork version string for the given mode.
// The upstream segment is preserved and only the trailing fork digit is
// incremented. If the input is empty the function returns the initial fork
// version "0.1.0<sep>1" so that brand-new fork changelogs have a starting
// point.
func NextForkVersion(currentVersion, mode string) (string, error) {
	separator, err := separatorForMode(mode)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(currentVersion) == "" {
		return fmt.Sprintf("%s%s1", entities.InitialReleaseVersion, separator), nil
	}

	parsed, err := ParseForkVersion(currentVersion, mode)
	if err != nil {
		return "", err
	}

	parsed.Fork++
	return parsed.String(), nil
}

// IsForkVersioning reports whether the given mode is one of the fork modes.
func IsForkVersioning(mode string) bool {
	return mode == entities.VersioningForkDot || mode == entities.VersioningForkDash
}

// FindLatestForkVersion scans the changelog lines for the most recent fork
// version header (skipping the [Unreleased] section) and returns its parsed
// form. Returns ErrNoForkVersionFound when no compatible header is present.
func FindLatestForkVersion(lines []string, mode string) (*ForkVersion, error) {
	for _, line := range lines {
		match := changelogVersionHeaderRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		header := match[1]
		if header == "Unreleased" {
			continue
		}
		parsed, err := ParseForkVersion(header, mode)
		if err != nil {
			continue
		}
		return parsed, nil
	}
	return nil, ErrNoForkVersionFound
}

// ErrNoForkVersionFound is returned when a fork-mode changelog has no usable
// previous version header.
var ErrNoForkVersionFound = errors.New("no fork version found in the changelog")

// processForkChangelog rewrites a fork-versioned changelog by moving the
// content of the [Unreleased] section into a new dated header that uses the
// next fork version. The function does NOT bump major/minor/patch based on
// Added/Changed/Fixed -- forks bump only the trailing fork digit.
//
// Returns the new version string and the rewritten file content.
func processForkChangelog(lines []string, mode string) (string, []string, error) {
	if !IsForkVersioning(mode) {
		return "", nil, fmt.Errorf("%w: %q is not a fork mode", ErrInvalidForkVersion, mode)
	}

	current, err := FindLatestForkVersion(lines, mode)
	currentString := ""
	switch {
	case err == nil:
		currentString = current.String()
	case errors.Is(err, ErrNoForkVersionFound):
		// No previous fork version; NextForkVersion will seed an initial value.
	default:
		return "", nil, err
	}

	nextVersion, err := NextForkVersion(currentString, mode)
	if err != nil {
		return "", nil, err
	}

	newContent := rewriteUnreleasedAsForkRelease(lines, nextVersion)
	return nextVersion, newContent, nil
}

// rewriteUnreleasedAsForkRelease moves the body of the [Unreleased] section
// under a freshly minted "## [<version>] - <date>" header and reinstates an
// empty [Unreleased] section above it. Lines outside the unreleased section
// are preserved verbatim.
func rewriteUnreleasedAsForkRelease(lines []string, nextVersion string) []string {
	unreleasedHeaderIdx := -1
	nextSectionIdx := len(lines)

	for i, line := range lines {
		match := changelogVersionHeaderRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if match[1] == "Unreleased" {
			unreleasedHeaderIdx = i
			continue
		}
		if unreleasedHeaderIdx != -1 {
			nextSectionIdx = i
			break
		}
	}

	if unreleasedHeaderIdx == -1 {
		return lines
	}

	body := trimBlankEdges(lines[unreleasedHeaderIdx+1 : nextSectionIdx])
	if len(body) == 0 {
		return lines
	}

	releasedHeader := fmt.Sprintf("## [%s] - %s", nextVersion, time.Now().Format("2006-01-02"))

	rebuilt := make([]string, 0, len(lines)+forkRewriteOverhead)
	rebuilt = append(rebuilt, lines[:unreleasedHeaderIdx]...)
	rebuilt = append(rebuilt, "## [Unreleased]")
	rebuilt = append(rebuilt, "")
	rebuilt = append(rebuilt, releasedHeader)
	rebuilt = append(rebuilt, "")
	rebuilt = append(rebuilt, body...)
	if nextSectionIdx < len(lines) {
		rebuilt = append(rebuilt, "")
		rebuilt = append(rebuilt, lines[nextSectionIdx:]...)
	}
	return rebuilt
}

// trimBlankEdges drops leading/trailing empty lines from a slice without
// mutating the original.
func trimBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start == 0 && end == len(lines) {
		return lines
	}
	return lines[start:end]
}

func separatorForMode(mode string) (string, error) {
	switch mode {
	case entities.VersioningForkDot:
		return ".", nil
	case entities.VersioningForkDash:
		return "-", nil
	default:
		return "", fmt.Errorf("%w: %q is not a fork mode", ErrInvalidForkVersion, mode)
	}
}

func separatorMatchesMode(separator, mode string) bool {
	switch mode {
	case entities.VersioningForkDot:
		return separator == "."
	case entities.VersioningForkDash:
		return separator == "-"
	case "":
		return separator == "." || separator == "-"
	default:
		return false
	}
}
