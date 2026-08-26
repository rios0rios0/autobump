package commands

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// chlog (https://github.com/luizjhonata/chlog) is a fragment-based changelog tool:
// instead of editing a shared CHANGELOG.md, every change is written to its own YAML
// file under .changes/unreleased/, which removes changelog merge conflicts entirely.
// The tool later compiles those fragments into ordinary Keep a Changelog sections.
//
// That means a chlog repository has a permanently empty [Unreleased] section, which
// AutoBump would otherwise read as "nothing to release". The helpers below detect the
// layout and turn the pending fragments back into [Unreleased] lines so the rest of the
// bumper -- emptiness check, SemVer calculation, fork mode, rewriting -- keeps working
// on plain Keep a Changelog content and needs no knowledge of chlog.
//
// chlog is a command-only Go module (everything lives under internal/), so its format is
// re-implemented here rather than imported.

// chlog configuration file names, in the order chlog itself probes them.
const (
	chlogConfigFileName    = ".chlog.yaml"
	chlogAltConfigFileName = ".chlog.yml"
)

// Defaults mirroring chlog's DefaultConfig(). They apply whenever a repository uses
// chlog without committing a configuration file, which chlog fully supports.
const (
	defaultChlogChangesDir    = ".changes"
	defaultChlogUnreleasedDir = "unreleased"
	defaultChlogChangelogPath = "CHANGELOG.md"
)

// Bump levels a kind can infer, named as chlog names them. There is deliberately no
// `autoMajor`: chlog has no such constant either, because under SemVer a major bump is a
// property of the change rather than of its category, signalled per fragment by
// `chlog new --breaking`.
const (
	autoMinor = "minor"
	autoPatch = "patch"
)

// chlogFallbackSection receives fragments whose kind is not one of the six Keep a
// Changelog sections. chlog lets a repository define arbitrary kind labels, but
// gitforge only ever emits the six known sections, so an unmapped kind would be
// dropped silently. "Changed" is the neutral bucket that keeps the entry.
const chlogFallbackSection = entities.SectionChanged

// chlogContinuationIndent prefixes every line of a multi-line fragment body after the
// first, so a wrapped sentence stays one changelog entry and a nested list stays nested.
const chlogContinuationIndent = "  "

// chlogLinesPerFragment is a capacity hint: a heading plus a bullet per fragment.
const chlogLinesPerFragment = 2

// ErrChlogPendingVersionFiles is returned when chlog has already batched fragments into
// .changes/v<version>.md but nobody has merged them into the changelog yet. Those files
// carry a version chlog has decided; releasing a different one would rewrite history.
var (
	ErrChlogPendingVersionFiles = errors.New("chlog has batched but unmerged version files")

	// ErrChlogPathEscapesProject is returned when .chlog.yaml configures a path that would
	// take the bumper outside the repository it was pointed at.
	ErrChlogPathEscapesProject = errors.New("chlog path escapes the project root")

	// ErrChlogNotRegularFile is returned when a path that must be a plain file is a symlink,
	// a directory, or a device.
	ErrChlogNotRegularFile = errors.New("not a regular file")
)

// keepAChangelogSections maps a lower-cased kind label to its canonical Keep a Changelog
// section. chlog's six default kinds are exactly these sections, which is what lets the
// fragments be spliced straight into [Unreleased].
//
//nolint:gochecknoglobals // read-only lookup table
var keepAChangelogSections = map[string]string{
	"added":      entities.SectionAdded,
	"changed":    entities.SectionChanged,
	"deprecated": entities.SectionDeprecated,
	"removed":    entities.SectionRemoved,
	"fixed":      entities.SectionFixed,
	"security":   entities.SectionSecurity,
}

// ChlogKind is a single entry of the "kinds" list in .chlog.yaml. Only Label is used
// here: AutoBump derives the bump from the resulting Keep a Changelog sections, so
// chlog's own Auto mapping is deliberately not applied.
type ChlogKind struct {
	Label string `yaml:"label"`
	Auto  string `yaml:"auto,omitempty"`
}

// ChlogConfig is the subset of .chlog.yaml that matters to AutoBump. The rendering
// templates (versionFormat, kindFormat, changeFormat) are intentionally ignored, since
// AutoBump renders the release section itself.
type ChlogConfig struct {
	ChangesDir    string      `yaml:"changesDir"`
	UnreleasedDir string      `yaml:"unreleasedDir"`
	ChangelogPath string      `yaml:"changelogPath"`
	Kinds         []ChlogKind `yaml:"kinds"`
}

// ChlogFragment is one pending change, as stored in .changes/unreleased/<name>.yaml.
//
// Breaking is what "chlog new --breaking" writes. chlog uses it only to force a major bump
// when it picks a version itself and never renders it, but AutoBump picks the version from
// the rendered Keep a Changelog lines -- so the flag has to become the marker the SemVer
// calculation counts, or a breaking fragment would silently release as a minor.
type ChlogFragment struct {
	Kind     string    `yaml:"kind"`
	Body     string    `yaml:"body"`
	Breaking bool      `yaml:"breaking,omitempty"`
	Time     time.Time `yaml:"time"`
	Path     string    `yaml:"-"`
}

// DefaultChlogConfig returns chlog's own defaults, mirroring DefaultConfig() in
// chlog's internal/config.go.
//
// `Auto` is carried for fidelity and is deliberately unread: AutoBump derives the bump
// from the rendered Keep a Changelog sections, not from the fragment's kind. Only `Label`
// is used here -- to map a kind to its section and to order the sections.
//
// Changed and Removed map to `minor`, not `major`. chlog moved them off `major` (there is
// no `autoMajor` constant left in the tool at all) because under SemVer a major bump means
// a backward-incompatible change, which is a property of the change and not of its
// category -- it is signalled per fragment by `chlog new --breaking`. Mirroring a value
// the tool no longer has would make this function's own doc comment false.
func DefaultChlogConfig() ChlogConfig {
	return ChlogConfig{
		ChangesDir:    defaultChlogChangesDir,
		UnreleasedDir: defaultChlogUnreleasedDir,
		ChangelogPath: defaultChlogChangelogPath,
		Kinds: []ChlogKind{
			{Label: "Added", Auto: autoMinor},
			{Label: "Changed", Auto: autoMinor},
			{Label: "Deprecated", Auto: autoMinor},
			{Label: "Removed", Auto: autoMinor},
			{Label: "Fixed", Auto: autoPatch},
			{Label: "Security", Auto: autoPatch},
		},
	}
}

// UnreleasedPath returns the directory holding the pending fragments.
func (c *ChlogConfig) UnreleasedPath(projectPath string) string {
	return filepath.Join(projectPath, c.ChangesDir, c.UnreleasedDir)
}

// DetectChlog reports whether a project uses chlog and returns the effective
// configuration. A repository counts as a chlog user when it commits a .chlog.yaml or
// when the unreleased fragment directory exists -- the configuration file is optional.
//
// Unlike chlog, the search never walks above projectPath: AutoBump is strictly
// per-repository, and a .chlog.yaml in a parent directory says nothing about this repo.
func DetectChlog(projectPath string) (*ChlogConfig, bool, error) {
	config, hasConfigFile, err := loadChlogConfig(projectPath)
	if err != nil {
		return nil, false, err
	}

	// A stat failure other than "absent" -- a permission error, a broken mount -- must not
	// be read as "this project does not use chlog": that would silently skip its fragments.
	unreleasedPath := config.UnreleasedPath(projectPath)
	info, err := os.Stat(unreleasedPath)
	switch {
	case err == nil:
		if info.IsDir() {
			return config, true, nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, false, fmt.Errorf(
			"failed to inspect the chlog fragment directory %s: %w", unreleasedPath, err)
	}

	return config, hasConfigFile, nil
}

// loadChlogConfig reads .chlog.yaml (or .chlog.yml) from the project root, filling any
// unset field from chlog's defaults. The boolean reports whether a file was found.
func loadChlogConfig(projectPath string) (*ChlogConfig, bool, error) {
	config := DefaultChlogConfig()

	for _, name := range []string{chlogConfigFileName, chlogAltConfigFileName} {
		configPath := filepath.Join(projectPath, name)

		data, err := readRegularFile(configPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			// An existing but unreadable config is an error, not an absent one: falling
			// back to the defaults would quietly look in the wrong place for fragments.
			return nil, false, fmt.Errorf("failed to read chlog config %s: %w", configPath, err)
		}

		// Unknown keys are tolerated: AutoBump only reads a subset of .chlog.yaml.
		if err = yaml.Unmarshal(data, &config); err != nil {
			return nil, false, fmt.Errorf("failed to parse chlog config %s: %w", configPath, err)
		}

		applyChlogConfigDefaults(&config)
		if err = validateChlogConfig(&config, configPath); err != nil {
			return nil, false, err
		}
		return &config, true, nil
	}

	return &config, false, nil
}

// applyChlogConfigDefaults restores chlog's defaults for keys the file left out.
func applyChlogConfigDefaults(config *ChlogConfig) {
	if strings.TrimSpace(config.ChangesDir) == "" {
		config.ChangesDir = defaultChlogChangesDir
	}
	if strings.TrimSpace(config.UnreleasedDir) == "" {
		config.UnreleasedDir = defaultChlogUnreleasedDir
	}
	if strings.TrimSpace(config.ChangelogPath) == "" {
		config.ChangelogPath = defaultChlogChangelogPath
	}
	if len(config.Kinds) == 0 {
		config.Kinds = DefaultChlogConfig().Kinds
	}
}

// validateChlogConfig rejects configured paths that leave the project root.
//
// .chlog.yaml is committed by the repository being released, and in discovery mode
// AutoBump clones repositories it does not own, so these values are untrusted input. They
// drive globbing, reading, and -- for the consumed fragments -- deletion, so an absolute
// or parent-escaping value would let a hostile configuration reach the host filesystem.
func validateChlogConfig(config *ChlogConfig, configPath string) error {
	fields := []struct{ name, value string }{
		{"changesDir", config.ChangesDir},
		{"unreleasedDir", config.UnreleasedDir},
		{"changelogPath", config.ChangelogPath},
		// The directories are joined, so a pair that is individually harmless but escapes
		// once combined has to be rejected too.
		{"changesDir/unreleasedDir", filepath.Join(config.ChangesDir, config.UnreleasedDir)},
	}

	for _, field := range fields {
		if !isPathInsideProject(field.value) {
			return fmt.Errorf("%w: %s %q in %s must be relative to the project root",
				ErrChlogPathEscapesProject, field.name, field.value, configPath)
		}
	}

	return nil
}

// isPathInsideProject reports whether a configured path stays within the project root.
// An absolute path, "..", or anything starting with "../" would address a file the bumper
// was never pointed at.
func isPathInsideProject(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

// readRegularFile reads a file only when the path itself is a regular file.
//
// A repository can commit a symlink under .changes/unreleased pointing anywhere on the
// host, and AutoBump publishes fragment bodies verbatim into the changelog, so following
// one would leak host files into a release. [os.Lstat] inspects the link rather than its
// target, which is what makes the refusal possible.
func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrChlogNotRegularFile, path)
	}

	return os.ReadFile(path)
}

// ReadChlogFragments loads every pending fragment, sorted so the rendered output is
// deterministic: by the kind order declared in the configuration, then by timestamp,
// then by file path.
//
// It refuses to run when chlog has batched fragments into .changes/v<version>.md
// without merging them, because those files already carry a decided version.
func ReadChlogFragments(projectPath string, config *ChlogConfig) ([]ChlogFragment, error) {
	if err := ensureNoPendingChlogVersionFiles(projectPath, config); err != nil {
		return nil, err
	}

	paths, err := chlogFragmentPaths(projectPath, config)
	if err != nil {
		return nil, err
	}

	fragments := make([]ChlogFragment, 0, len(paths))
	for _, path := range paths {
		var fragment *ChlogFragment
		fragment, err = readChlogFragment(path)
		if err != nil {
			return nil, err
		}
		if fragment == nil {
			continue
		}
		fragments = append(fragments, *fragment)
	}

	sortChlogFragments(fragments, config)
	return fragments, nil
}

// chlogFragmentPaths lists the fragment files in the unreleased directory. chlog writes
// ".yaml", but ".yml" is accepted so a hand-written fragment is not ignored.
func chlogFragmentPaths(projectPath string, config *ChlogConfig) ([]string, error) {
	unreleasedPath := config.UnreleasedPath(projectPath)

	var paths []string
	for _, pattern := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(unreleasedPath, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to list chlog fragments in %s: %w", unreleasedPath, err)
		}
		paths = append(paths, matches...)
	}

	slices.Sort(paths)
	return paths, nil
}

// readChlogFragment parses one fragment file. A fragment with an empty body carries no
// changelog content, so it is skipped (nil, nil) rather than emitting an empty bullet.
//
// A fragment that is not a regular file is skipped too, loudly: it is never read, so a
// symlink planted under .changes/unreleased cannot leak its target into the changelog,
// and the surrounding legitimate fragments still get released.
func readChlogFragment(path string) (*ChlogFragment, error) {
	data, err := readRegularFile(path)
	if errors.Is(err, ErrChlogNotRegularFile) {
		logger.Warnf("Skipping chlog fragment %s: it is not a regular file", path)
		return nil, nil //nolint:nilnil // a suspicious fragment is skipped, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read chlog fragment %s: %w", path, err)
	}

	var fragment ChlogFragment
	if err = yaml.Unmarshal(data, &fragment); err != nil {
		return nil, fmt.Errorf("failed to parse chlog fragment %s: %w", path, err)
	}

	if strings.TrimSpace(fragment.Body) == "" {
		logger.Warnf("Skipping chlog fragment %s: it has no body", path)
		return nil, nil //nolint:nilnil // an empty fragment is skipped, not an error
	}

	fragment.Path = path
	return &fragment, nil
}

// ensureNoPendingChlogVersionFiles guards against releasing on top of a half-finished
// "chlog batch" run.
func ensureNoPendingChlogVersionFiles(projectPath string, config *ChlogConfig) error {
	pattern := filepath.Join(projectPath, config.ChangesDir, "v*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list chlog version files in %s: %w", config.ChangesDir, err)
	}
	if len(matches) == 0 {
		return nil
	}

	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, filepath.Base(match))
	}

	return fmt.Errorf(
		"%w: found %s in %s; run \"chlog merge\" (or delete the file) before bumping",
		ErrChlogPendingVersionFiles, strings.Join(names, ", "), config.ChangesDir,
	)
}

// sortChlogFragments orders fragments by configured kind, then timestamp, then path.
// Fragments whose kind is not configured sort last, keeping the output stable even when
// timestamps are absent or identical.
func sortChlogFragments(fragments []ChlogFragment, config *ChlogConfig) {
	order := make(map[string]int, len(config.Kinds))
	for index, kind := range config.Kinds {
		order[strings.ToLower(strings.TrimSpace(kind.Label))] = index
	}

	rank := func(fragment ChlogFragment) int {
		if index, found := order[strings.ToLower(strings.TrimSpace(fragment.Kind))]; found {
			return index
		}
		return len(config.Kinds)
	}

	slices.SortStableFunc(fragments, func(a, b ChlogFragment) int {
		if diff := cmp.Compare(rank(a), rank(b)); diff != 0 {
			return diff
		}
		if diff := a.Time.Compare(b.Time); diff != 0 {
			return diff
		}
		return cmp.Compare(a.Path, b.Path)
	})
}

// RenderChlogFragments turns fragments into Keep a Changelog lines: a "### <Section>"
// heading per section followed by one bullet per fragment. The output is what a human
// would have hand-written in [Unreleased], which is exactly what the rest of the
// pipeline expects.
func RenderChlogFragments(fragments []ChlogFragment, config *ChlogConfig) []string {
	rendered := make([]string, 0, len(fragments)*chlogLinesPerFragment)

	currentSection := ""
	for _, fragment := range fragments {
		section := chlogSectionForKind(fragment.Kind, config)
		if section != currentSection {
			if currentSection != "" {
				rendered = append(rendered, "")
			}
			rendered = append(rendered, "### "+section, "")
			currentSection = section
		}
		rendered = append(rendered, renderChlogBody(fragment)...)
	}

	return rendered
}

// chlogSectionForKind maps a fragment kind onto a Keep a Changelog section.
func chlogSectionForKind(kind string, config *ChlogConfig) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	if section, found := keepAChangelogSections[normalized]; found {
		return section
	}

	// Fall back rather than drop: gitforge only renders the six known sections, so an
	// unmapped kind would vanish from the release notes.
	for _, configured := range config.Kinds {
		if strings.EqualFold(strings.TrimSpace(configured.Label), normalized) {
			logger.Warnf(
				"chlog kind %q has no Keep a Changelog section, filing it under %q",
				kind, chlogFallbackSection,
			)
			return chlogFallbackSection
		}
	}

	logger.Warnf("Unknown chlog kind %q, filing it under %q", kind, chlogFallbackSection)
	return chlogFallbackSection
}

// renderChlogBody renders one fragment as a bullet. Lines after the first are indented so
// a wrapped sentence stays a single entry and a nested list stays nested.
//
// The opening line carries the breaking-change marker when the fragment declares one. It
// is written by NormalizeBreakingChangeMarker rather than prepended, because a writer who
// passes --breaking will often open the body with "BREAKING CHANGE:" as well -- the same
// fact stated the way a commit footer states it -- and prepending would publish the marker
// twice.
func renderChlogBody(fragment ChlogFragment) []string {
	raw := strings.Split(strings.Trim(fragment.Body, "\n"), "\n")
	rendered := make([]string, 0, len(raw))

	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(rendered) == 0 {
			opening := entities.NormalizeBreakingChangeMarker(
				stripBulletPrefix(line), fragment.Breaking)
			rendered = append(rendered, "- "+opening)
			continue
		}
		rendered = append(rendered, chlogContinuationIndent+strings.TrimRight(line, " \t"))
	}

	return rendered
}

// stripBulletPrefix removes a leading bullet marker. chlog stores bodies without one,
// but a hand-written fragment may include it and would otherwise render as "- - text".
func stripBulletPrefix(line string) string {
	trimmed := strings.TrimSpace(line)
	if after, found := strings.CutPrefix(trimmed, "-"); found {
		return strings.TrimSpace(after)
	}
	return trimmed
}

// MergeChlogIntoUnreleased splices rendered fragment lines into the [Unreleased] section
// of a changelog, keeping any entries already written there by hand. Merging rather than
// replacing matters during a migration to chlog, when both sources can hold real work.
//
// Duplicate "### Added"-style headings may result when both sources use the same
// section. That is harmless: the caller normalises the section afterwards, which merges
// repeated headings into one and removes any overlapping bullets.
func MergeChlogIntoUnreleased(lines, fragmentLines []string) []string {
	if len(fragmentLines) == 0 {
		return lines
	}

	unreleasedHeaderIdx, nextSectionIdx := findUnreleasedBounds(lines)
	if unreleasedHeaderIdx == -1 {
		return insertUnreleasedSection(lines, fragmentLines, nextSectionIdx)
	}

	body := trimBlankEdges(lines[unreleasedHeaderIdx+1 : nextSectionIdx])

	merged := make([]string, 0, len(lines)+len(fragmentLines)+chlogLinesPerFragment)
	merged = append(merged, lines[:unreleasedHeaderIdx+1]...)
	merged = append(merged, "")
	if len(body) > 0 {
		merged = append(merged, body...)
		merged = append(merged, "")
	}
	merged = append(merged, fragmentLines...)
	if nextSectionIdx < len(lines) {
		merged = append(merged, "")
		merged = append(merged, lines[nextSectionIdx:]...)
	}

	return merged
}

// findUnreleasedBounds locates the [Unreleased] header and the header that follows it.
// When there is no [Unreleased] header, the first index is -1 and the second points at
// the first version header (or the end of the file), which is where one should go.
func findUnreleasedBounds(lines []string) (int, int) {
	unreleasedHeaderIdx := -1
	firstVersionIdx := len(lines)

	for i, line := range lines {
		header, isHeader := entities.MatchChangelogVersionHeader(line)
		if !isHeader {
			continue
		}
		if header == entities.UnreleasedHeaderName {
			unreleasedHeaderIdx = i
			continue
		}
		if firstVersionIdx == len(lines) {
			firstVersionIdx = i
		}
		if unreleasedHeaderIdx != -1 {
			return unreleasedHeaderIdx, i
		}
	}

	return unreleasedHeaderIdx, firstVersionIdx
}

// insertUnreleasedSection builds an [Unreleased] section for a changelog that has none,
// placing it above the most recent release.
func insertUnreleasedSection(lines, fragmentLines []string, insertIdx int) []string {
	rebuilt := make([]string, 0, len(lines)+len(fragmentLines)+chlogLinesPerFragment)
	rebuilt = append(rebuilt, lines[:insertIdx]...)
	rebuilt = append(rebuilt, "## [Unreleased]", "")
	rebuilt = append(rebuilt, fragmentLines...)
	if insertIdx < len(lines) {
		rebuilt = append(rebuilt, "")
		rebuilt = append(rebuilt, lines[insertIdx:]...)
	}
	return rebuilt
}

// DeleteChlogFragments removes consumed fragment files and returns the paths deleted.
// Deleting is what makes the release final: leaving the fragments in place would ship
// the same entries again on the next run. It mirrors what "chlog batch" does.
func DeleteChlogFragments(fragments []ChlogFragment) ([]string, error) {
	deleted := make([]string, 0, len(fragments))

	for _, fragment := range fragments {
		if fragment.Path == "" {
			continue
		}
		if err := os.Remove(fragment.Path); err != nil {
			return deleted, fmt.Errorf("failed to remove chlog fragment %s: %w", fragment.Path, err)
		}
		deleted = append(deleted, fragment.Path)
	}

	return deleted, nil
}
