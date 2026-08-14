package commands

import (
	"fmt"
	"regexp"
	"strings"

	langDart "github.com/rios0rios0/langforge/pkg/infrastructure/languages/dart"
)

// mavenParentPlaceholder stands in for the <parent> block while the project
// version is substituted.
const mavenParentPlaceholder = "<!--AUTOBUMP_PARENT_PLACEHOLDER-->"

var mavenParentRe = regexp.MustCompile(`(?s)<parent>.*?</parent>`)

// versionFileHook customises how one particular version file is rewritten.
//
// The substitution engine is otherwise language-agnostic: it writes the version
// calculated from the changelog into whatever the configured regex captured.
// Two files need more than that, so the exceptions live in a keyed table rather
// than as branches inside the rewrite — a third one is a map entry, not an edit
// to the engine.
type versionFileHook struct {
	// mask hides a region of the file from substitution, returning the masked
	// content and the region it removed. Nil when the file needs no masking.
	mask func(content string) (masked string, hidden string)

	// unmask restores what mask removed. Nil when mask is nil.
	unmask func(content string, hidden string) string

	// version derives the value to write from the value currently in the file
	// and the version calculated from the changelog. Nil means "write the
	// calculated version verbatim", which is what every other file wants.
	version func(current string, newVersion string) string
}

// versionFileHooks maps a version file's base name to its rewrite exceptions.
//
//nolint:gochecknoglobals // read-only dispatch table keyed by version file name
var versionFileHooks = map[string]versionFileHook{
	"pom.xml":      {mask: maskMavenParent, unmask: unmaskMavenParent},
	"pubspec.yaml": {version: langDart.BumpBuildNumber},
}

// applyVersionPatterns writes newVersion into the first match of every
// configured pattern, applying the file's hook around and inside the rewrite.
func applyVersionPatterns(
	content string,
	patterns []string,
	newVersion string,
	hook versionFileHook,
) (string, error) {
	var hidden string
	if hook.mask != nil {
		content, hidden = hook.mask(content)
	}

	for _, pattern := range patterns {
		re, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			return "", wrapInvalidVersionPattern(pattern, compileErr)
		}
		content = replaceFirstVersion(content, re, newVersion, hook.version)
	}

	if hook.unmask != nil && hidden != "" {
		content = hook.unmask(content, hidden)
	}

	return content, nil
}

// replaceFirstVersion rewrites only the first match of re, leaving any later
// occurrence alone — that is what keeps a plugin version in build.gradle and
// appVersion in Chart.yaml from being bumped along with the project's own.
func replaceFirstVersion(
	content string,
	re *regexp.Regexp,
	newVersion string,
	resolve func(current string, newVersion string) string,
) string {
	replaced := false
	return re.ReplaceAllStringFunc(content, func(match string) string {
		if replaced {
			return match
		}
		replaced = true

		value := newVersion
		if resolve != nil {
			value = resolve(currentVersionValue(re, match), newVersion)
		}

		return re.ReplaceAllString(match, "${1}"+value+"${2}")
	})
}

// currentVersionValue returns the slice of match that the substitution is about
// to overwrite: everything between the group 1 prefix and the group 2 suffix
// that the template preserves. That is the version the file carries today.
func currentVersionValue(re *regexp.Regexp, match string) string {
	const (
		prefixEndIndex   = 3 // end offset of capture group 1
		suffixStartIndex = 4 // start offset of capture group 2
	)

	loc := re.FindStringSubmatchIndex(match)
	if loc == nil {
		return match
	}

	start, end := 0, len(match)
	if len(loc) > prefixEndIndex && loc[prefixEndIndex] >= 0 {
		start = loc[prefixEndIndex]
	}
	if len(loc) > suffixStartIndex+1 && loc[suffixStartIndex] >= 0 {
		end = loc[suffixStartIndex]
	}
	if start > end {
		return match
	}

	return match[start:end]
}

// wrapInvalidVersionPattern reports a version_files pattern that does not compile.
// The pattern comes from configuration, so the message names it.
func wrapInvalidVersionPattern(pattern string, err error) error {
	return fmt.Errorf("invalid regex pattern %q in version file config: %w", pattern, err)
}

// maskMavenParent hides the <parent> block so that the first <version> match is
// the project's own, which appears after </parent>, and not the parent's.
func maskMavenParent(content string) (string, string) {
	loc := mavenParentRe.FindStringIndex(content)
	if loc == nil {
		return content, ""
	}
	return content[:loc[0]] + mavenParentPlaceholder + content[loc[1]:], content[loc[0]:loc[1]]
}

// unmaskMavenParent restores the block maskMavenParent removed.
func unmaskMavenParent(content string, hidden string) string {
	return strings.Replace(content, mavenParentPlaceholder, hidden, 1)
}
