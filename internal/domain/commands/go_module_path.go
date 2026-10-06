package commands

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/storer"
	logger "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	modsemver "golang.org/x/mod/semver"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

const (
	goModFileName = "go.mod"
	goSumFileName = "go.sum"

	// firstSuffixedMajor is the first major version whose module path must end in "/vN".
	firstSuffixedMajor = 2

	// maxReferenceFileSize bounds the files scanned for references to the module path.
	// Documentation, build scripts and pipeline definitions are far smaller; anything this
	// large is generated or binary, and reading it would only slow the release down.
	maxReferenceFileSize = 4 << 20

	// binarySniffLength is how much of a file is inspected for a NUL byte before it is
	// treated as text, the heuristic git itself uses.
	binarySniffLength = 8000

	majorDecimalBase = 10
	majorBitSize     = 64

	// cleanIndexStage is the stage of an index entry without a merge conflict. go-git's
	// index.Merged is 1, the first of the conflict stages, so it cannot say this.
	cleanIndexStage index.Stage = 0
)

// ErrGoModulePathNotFound reports a go.mod whose module directive could not be located.
var ErrGoModulePathNotFound = errors.New("could not locate the module path in go.mod")

// skippedModuleTrees names the directories whose files the rewrite never touches. None of
// them is the module's own source: vendored dependencies, and fixtures, which Go never
// compiles and which may quote the old path on purpose.
//
//nolint:gochecknoglobals // read-only lookup table
var skippedModuleTrees = map[string]bool{
	"node_modules": true,
	"testdata":     true,
	"vendor":       true,
}

// moduleReferenceContexts are what has to come right before the module path, on the same
// line, for a file other than Go source to be naming the module itself. A bare match is
// not enough: the path is also a repository URL, and every badge and link to
// https://github.com/<owner>/<repo>/... would be rewritten into a page that does not exist.
//
//nolint:gochecknoglobals // read-only lookup table
var moduleReferenceContexts = []*regexp.Regexp{
	regexp.MustCompile("[\"`]$"),                                          // a quoted import path or inline code
	regexp.MustCompile(`\bpkg\.go\.dev/$`),                                // a documentation link
	regexp.MustCompile(`\bgo\s+(?:get|install|run|doc|list)\s(?:.*\s)?$`), // a go command
	regexp.MustCompile(`-X[\s=]+['"]?$`),                                  // a linker flag setting a variable
	regexp.MustCompile(`(?i)\bmodule:?\s+$`),                              // the module directive, quoted
}

// majorSuffixElement matches a major version path element at the start of what follows a
// module path. Whether it really is one depends on what comes after it, which is why
// followedByMajorSuffix checks the next byte as well.
var majorSuffixElement = regexp.MustCompile(`^/v[1-9][0-9]*`)

// GoModulePathChange is the rewrite a release makes so that a Go module's path names the
// major version the release publishes.
//
// Go requires a module released at v2 or later to say so in its path: a library tagged
// v4.2.0 whose go.mod still declares the unsuffixed path is invisible at v4 -- `go get`
// rejects every v2+ tag and settles for the newest v0/v1 release or a pseudo-version.
type GoModulePathChange struct {
	// From is the module path go.mod declared before the release.
	From string
	// To is the module path the released major version requires.
	To string
	// Major is the major version the release publishes.
	Major uint64
	// NewMajor reports that the release starts a major version, which is a breaking
	// change. When false the path had fallen behind a major version released earlier,
	// and the release repairs it.
	NewMajor bool
	// Files lists every file the rewrite changes, as paths under the project root.
	Files []string

	contents map[string][]byte
}

// goModulePathTarget is the outcome of deciding whether a release needs a module path
// rewrite, before any file is read for it.
type goModulePathTarget struct {
	from     string
	to       string
	major    uint64
	newMajor bool
}

// prepareGoModulePathChange decides whether this release has to rewrite the module path
// and, when it does, prepares every edit in memory without writing any of them.
//
// It returns nil when there is nothing to do: no go.mod, a fork versioning mode, a path
// that already names the released major, or a module whose versions Go never resolves.
// Preparing before anything is written is what lets a rewrite that cannot be made stop
// the release while the working tree is still exactly as it was found.
func prepareGoModulePathChange(ctx *RepoContext, changelogPath string) (*GoModulePathChange, error) {
	target, needed, err := resolveGoModulePathTarget(ctx, changelogPath)
	if err != nil || !needed {
		return nil, err
	}

	change := &GoModulePathChange{
		From:     target.from,
		To:       target.to,
		Major:    target.major,
		NewMajor: target.newMajor,
		contents: map[string][]byte{},
	}

	excluded, err := moduleRewriteExclusions(ctx, changelogPath)
	if err != nil {
		return nil, err
	}
	if err = change.prepare(ctx.ProjectConfig.Path, ctx.Repo, excluded); err != nil {
		return nil, err
	}

	logger.Infof("Changing the Go module path from %s to %s for the v%d release (%d file(s))",
		change.From, change.To, change.Major, len(change.Files))
	return change, nil
}

// resolveGoModulePathTarget works out the path the release requires, and whether it
// differs from the one go.mod declares.
func resolveGoModulePathTarget(ctx *RepoContext, changelogPath string) (goModulePathTarget, bool, error) {
	if IsForkVersioning(entities.ResolveVersioning(ctx.GlobalConfig, ctx.ProjectConfig)) {
		return goModulePathTarget{}, false, nil
	}

	from, err := readGoModulePath(ctx.ProjectConfig.Path)
	if err != nil || from == "" {
		return goModulePathTarget{}, false, err
	}

	previous, next, err := releaseVersions(ctx.GlobalConfig, ctx.ProjectConfig, changelogPath)
	if err != nil {
		return goModulePathTarget{}, false, err
	}

	to, needed := goModulePathForMajor(from, next.Major())
	if !needed {
		return goModulePathTarget{}, false, nil
	}

	published, err := publishesGoModuleVersions(ctx.Repo)
	if err != nil {
		return goModulePathTarget{}, false, err
	}
	if !published {
		logger.Infof(
			"Leaving the Go module path %s as it is: no vX.Y.Z tag exists, so Go never resolves this module's versions",
			from,
		)
		return goModulePathTarget{}, false, nil
	}

	return goModulePathTarget{
		from:     from,
		to:       to,
		major:    next.Major(),
		newMajor: previous == nil || previous.Major() < next.Major(),
	}, true, nil
}

// readGoModulePath returns the module path the project's go.mod declares, or "" when the
// project has none. A go.mod that is not a regular file is left alone: the rewrite would
// otherwise write through a symlink that may point outside the repository.
func readGoModulePath(projectPath string) (string, error) {
	goModPath := filepath.Join(projectPath, goModFileName)

	info, err := os.Lstat(goModPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to inspect %s: %w", goModPath, err)
	}
	if !info.Mode().IsRegular() {
		logger.Warnf("Leaving the Go module path as it is: %s is not a regular file", goModPath)
		return "", nil
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", goModPath, err)
	}

	return modfile.ModulePath(data), nil
}

// releaseVersions returns the version the changelog last released, nil when it released
// none, and the version this release publishes.
func releaseVersions(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
	changelogPath string,
) (*semver.Version, *semver.Version, error) {
	lines, err := readChangelogLines(globalConfig, projectConfig, changelogPath)
	if err != nil {
		return nil, nil, err
	}

	previous, err := entities.FindLatestVersion(lines)
	if errors.Is(err, entities.ErrNoVersionFoundInChangelog) {
		initial, parseErr := semver.NewVersion(entities.InitialReleaseVersion)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("failed to parse the initial release version: %w", parseErr)
		}
		return nil, initial, nil
	}
	if err != nil {
		return nil, nil, err
	}

	next, _, err := entities.ProcessChangelog(lines)
	if err != nil {
		return nil, nil, err
	}

	return previous, next, nil
}

// goModulePathForMajor returns the module path a release at major version `major`
// requires, and whether it differs from `from`.
//
// The path only ever moves forwards: a path already naming a higher major than the
// release is a state no rewrite can repair, so it is left for a human. A gopkg.in path
// carries its major as ".vN" and is resolved by the gopkg.in redirector, so it is left
// alone too.
func goModulePathForMajor(from string, major uint64) (string, bool) {
	if major < firstSuffixedMajor {
		return "", false
	}

	prefix, pathMajor, valid := module.SplitPathVersion(from)
	if !valid || strings.HasPrefix(pathMajor, ".") {
		return "", false
	}

	current := uint64(1)
	if pathMajor != "" {
		parsed, err := strconv.ParseUint(strings.TrimPrefix(pathMajor, "/v"), majorDecimalBase, majorBitSize)
		if err != nil {
			return "", false
		}
		current = parsed
	}
	if current >= major {
		return "", false
	}

	return prefix + "/v" + strconv.FormatUint(major, majorDecimalBase), true
}

// publishesGoModuleVersions reports whether the repository carries a tag Go reads as a
// module version: "v" followed by a full semantic version. Go resolves nothing else, so a
// project tagged "1.2.0" -- an application released as binaries -- is never fetched by
// version, and renaming its module would rewrite every import for no reader at all.
func publishesGoModuleVersions(repo *git.Repository) (bool, error) {
	if repo == nil {
		return false, nil
	}

	tags, err := repo.Tags()
	if err != nil {
		return false, fmt.Errorf("failed to list the repository tags: %w", err)
	}
	defer tags.Close()

	found := false
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if modsemver.IsValid(name) && modsemver.Canonical(name) == name {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to read the repository tags: %w", err)
	}

	return found, nil
}

// moduleRewriteExclusions returns the repository-relative paths the rewrite must not
// touch: the changelog, and the chlog directory. Both are history, and history keeps the
// path it had. A path ending in "/" excludes everything under it.
func moduleRewriteExclusions(ctx *RepoContext, changelogPath string) ([]string, error) {
	changelogName, err := filepath.Rel(ctx.ProjectConfig.Path, changelogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to locate the changelog in the project: %w", err)
	}
	excluded := []string{filepath.ToSlash(changelogName)}

	if !entities.ChlogEnabled(ctx.GlobalConfig, ctx.ProjectConfig) {
		return excluded, nil
	}

	config, usesChlog, err := DetectChlog(ctx.ProjectConfig.Path)
	if err != nil {
		return nil, err
	}
	if usesChlog {
		excluded = append(excluded, filepath.ToSlash(filepath.Clean(config.ChangesDir))+"/")
	}

	return excluded, nil
}

// ReleaseNote returns the Keep a Changelog lines announcing the change, ready to be
// merged into the [Unreleased] section the release is built from. A new major version is
// filed as the breaking change it is; a repaired path is filed under Fixed, which can
// never raise the version the rest of the section calls for.
func (c *GoModulePathChange) ReleaseNote() []string {
	if c == nil {
		return nil
	}

	imports := fmt.Sprintf("import its packages from `%s/...` instead of `%s/...`", c.To, c.From)
	if c.NewMajor {
		return []string{
			"### " + entities.SectionChanged,
			"",
			fmt.Sprintf("- %schanged the Go module path to `%s`, which Go requires of a v%d release: %s",
				entities.BreakingChangePrefix, c.To, c.Major, imports),
		}
	}

	return []string{
		"### " + entities.SectionFixed,
		"",
		fmt.Sprintf(
			"- fixed the Go module path to `%s`, which Go requires of a v%d release and without which "+
				"`go get` could not resolve it: %s",
			c.To, c.Major, imports,
		),
	}
}

// prepare reads every file the rewrite may touch and records the ones it changes.
//
// The candidates come from the index rather than from walking the directory: only what
// the repository tracks belongs to the module, and anything else on disk -- an ignored
// build output, a scratch file in a local checkout -- would otherwise be rewritten and
// then staged into the release commit.
func (c *GoModulePathChange) prepare(projectPath string, repo *git.Repository, excluded []string) error {
	goModPath := filepath.Join(projectPath, goModFileName)

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", goModPath, err)
	}
	rewritten, err := rewriteModuleDirective(data, c.To)
	if err != nil {
		return err
	}
	c.record(goModPath, data, rewritten)

	files, nestedModules, err := trackedModuleFiles(repo)
	if err != nil {
		return err
	}
	c.warnAboutNestedModules(projectPath, nestedModules)

	for _, name := range files {
		if name == goModFileName || path.Base(name) == goSumFileName || isExcludedName(name, excluded) {
			continue
		}
		if err = c.visitFile(filepath.Join(projectPath, filepath.FromSlash(name))); err != nil {
			return err
		}
	}

	return nil
}

// trackedModuleFiles returns the regular files the index tracks for the root module, as
// slash-separated repository-relative paths, and the directories of the nested modules it
// holds.
//
// A directory with its own go.mod is a different module, versioned by its own tags, so
// its files keep their imports. Symlinks and submodules are left out: a symlink committed
// to the repository can point anywhere on the host, and writing the rewrite through it
// would escape the repository.
func trackedModuleFiles(repo *git.Repository) ([]string, []string, error) {
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read the index: %w", err)
	}

	var nestedModules []string
	for _, entry := range idx.Entries {
		if dir, found := strings.CutSuffix(entry.Name, "/"+goModFileName); found {
			nestedModules = append(nestedModules, dir+"/")
		}
	}

	var files []string
	for _, entry := range idx.Entries {
		if entry.Stage != cleanIndexStage || (entry.Mode != filemode.Regular && entry.Mode != filemode.Executable) {
			continue
		}
		if inSkippedModuleTree(entry.Name) || isExcludedName(entry.Name, nestedModules) {
			continue
		}
		files = append(files, entry.Name)
	}

	return files, nestedModules, nil
}

// warnAboutNestedModules says out loud which nested modules still name the old path. They
// now require a module path that no longer exists here, and only a human can decide how
// each of them should move.
func (c *GoModulePathChange) warnAboutNestedModules(projectPath string, nestedModules []string) {
	for _, dir := range nestedModules {
		nestedGoMod := filepath.Join(projectPath, filepath.FromSlash(dir), goModFileName)
		data, err := os.ReadFile(nestedGoMod)
		if err != nil || !bytes.Contains(data, []byte(c.From)) {
			continue
		}
		logger.Warnf("The nested module in %s refers to %s, which is now %s: update it separately", dir, c.From, c.To)
	}
}

// visitFile records the rewrite of one file: the imports of a Go source file, or the
// references to the module in any other text file. A file missing from the working tree,
// or replaced there by something that is not a regular file, is left alone.
func (c *GoModulePathChange) visitFile(filePath string) error {
	info, err := os.Lstat(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect %s: %w", filePath, err)
	}
	isGo := filepath.Ext(filePath) == ".go"
	if !info.Mode().IsRegular() || (!isGo && info.Size() > maxReferenceFileSize) {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	if !isGo {
		if !bytes.Contains(data[:min(len(data), binarySniffLength)], []byte{0}) {
			c.record(filePath, data, rewriteModuleReferences(data, c.From, c.To))
		}
		return nil
	}

	rewritten, err := rewriteGoImports(data, c.From, c.To)
	if err != nil {
		// A file whose imports do not parse does not compile either -- a template, most
		// likely -- so leaving it as it is cannot break a build that worked.
		logger.Warnf("Leaving %s as it is: %v", filePath, err)
		return nil
	}
	c.record(filePath, data, rewritten)

	return nil
}

// record keeps a file's rewritten content when the rewrite changed it.
func (c *GoModulePathChange) record(path string, before, after []byte) {
	if bytes.Equal(before, after) {
		return
	}
	c.contents[path] = after
	c.Files = append(c.Files, path)
}

// apply writes the prepared rewrites and returns the files it changed.
func (c *GoModulePathChange) apply() ([]string, error) {
	if c == nil {
		return nil, nil
	}

	for _, path := range c.Files {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect %s: %w", path, err)
		}

		if err = os.WriteFile(path, c.contents[path], info.Mode().Perm()); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return slices.Clone(c.Files), nil
}

// inSkippedModuleTree reports whether a repository-relative path lies under one of the
// trees the rewrite never touches.
func inSkippedModuleTree(name string) bool {
	return slices.ContainsFunc(strings.Split(path.Dir(name), "/"), func(element string) bool {
		return skippedModuleTrees[element]
	})
}

// isExcludedName reports whether a repository-relative path is one of the excluded
// names, or lies under an excluded directory (one ending in "/").
func isExcludedName(name string, excluded []string) bool {
	return slices.ContainsFunc(excluded, func(entry string) bool {
		if strings.HasSuffix(entry, "/") {
			return strings.HasPrefix(name, entry)
		}
		return name == entry
	})
}

// rewriteModuleDirective replaces the module path go.mod declares, leaving every other
// byte of the file -- comments, directives, formatting -- exactly as it was.
func rewriteModuleDirective(data []byte, to string) ([]byte, error) {
	file, err := modfile.ParseLax(goModFileName, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", goModFileName, err)
	}
	if file.Module == nil || file.Module.Syntax == nil {
		return nil, ErrGoModulePathNotFound
	}

	start, end, found := moduleTokenSpan(data, file.Module.Syntax)
	if !found {
		return nil, ErrGoModulePathNotFound
	}

	return slices.Concat(data[:start], []byte(modfile.AutoQuote(to)), data[end:]), nil
}

// moduleTokenSpan locates the module path in the source of its directive. The parsed
// token cannot be measured directly: a quoted path is stored unquoted, so its length is
// not the length it has in the file.
func moduleTokenSpan(data []byte, line *modfile.Line) (int, int, bool) {
	start, end := line.Start.Byte, line.End.Byte
	if len(line.Token) == 0 || start < 0 || end > len(data) || start >= end {
		return 0, 0, false
	}

	if quote := data[end-1]; quote == '"' || quote == '`' {
		opening := bytes.LastIndexByte(data[start:end-1], quote)
		if opening < 0 {
			return 0, 0, false
		}
		return start + opening, end, true
	}

	raw := line.Token[len(line.Token)-1]
	if end-len(raw) < start || string(data[end-len(raw):end]) != raw {
		return 0, 0, false
	}
	return end - len(raw), end, true
}

// rewriteGoImports moves every import of the module's own packages to the new path.
//
// Only import paths change, at their exact offsets, so nothing else in the file can move.
// A file that was gofmt-clean is formatted again afterwards: a longer path can shift the
// alignment of a comment trailing an import, and the file must not turn up in the next
// lint run for it.
func rewriteGoImports(src []byte, from, to string) ([]byte, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", src, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the imports: %w", err)
	}

	out := src
	// Back to front, so an edit never moves the offsets of the ones still to make.
	for _, spec := range slices.Backward(file.Imports) {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			continue
		}
		rewritten, matched := rewriteModuleImportPath(importPath, from, to)
		if !matched {
			continue
		}

		start := fileSet.Position(spec.Path.Pos()).Offset
		end := fileSet.Position(spec.Path.End()).Offset
		out = slices.Concat(out[:start], []byte(quoteImportPath(spec.Path.Value, rewritten)), out[end:])
	}

	if bytes.Equal(out, src) {
		return src, nil
	}
	if formatted, formatErr := format.Source(src); formatErr == nil && bytes.Equal(formatted, src) {
		if reformatted, reformatErr := format.Source(out); reformatErr == nil {
			return reformatted, nil
		}
	}

	return out, nil
}

// rewriteModuleImportPath returns the import path moved to the new module path, and
// whether it belonged to the module at all. A path that merely shares a prefix with the
// module ("github.com/owner/repo-extra") is another module, and so is one that continues
// with a major version element: "github.com/owner/repo/v5" is a separate module that a
// "github.com/owner/repo" module never contains.
func rewriteModuleImportPath(importPath, from, to string) (string, bool) {
	if importPath == from {
		return to, true
	}

	rest, found := strings.CutPrefix(importPath, from+"/")
	if !found || followedByMajorSuffix([]byte("/"+rest)) {
		return "", false
	}

	return to + "/" + rest, true
}

// quoteImportPath quotes a rewritten import path the way the original was quoted.
func quoteImportPath(original, importPath string) string {
	if strings.HasPrefix(original, "`") {
		return "`" + importPath + "`"
	}
	return strconv.Quote(importPath)
}

// rewriteModuleReferences moves the references to the module in a text file other than
// Go source: an install command, a quoted import path, a documentation link, a linker
// flag. See moduleReferenceContexts for why a bare occurrence of the path is not enough.
func rewriteModuleReferences(data []byte, from, to string) []byte {
	needle := []byte(from)

	var out bytes.Buffer
	copied := 0
	for offset := 0; offset < len(data); {
		index := bytes.Index(data[offset:], needle)
		if index < 0 {
			break
		}
		start := offset + index
		end := start + len(needle)
		offset = end

		if !isModuleReference(data, start, end) {
			continue
		}
		out.Write(data[copied:start])
		out.WriteString(to)
		copied = end
	}

	if copied == 0 {
		return data
	}
	out.Write(data[copied:])
	return out.Bytes()
}

// isModuleReference reports whether the occurrence of the module path at data[start:end]
// names the module: it ends where a path ends, does not continue into another major
// version's path, and follows one of the contexts a module path is referenced in.
func isModuleReference(data []byte, start, end int) bool {
	if end < len(data) && isModulePathByte(data[end]) {
		return false
	}
	if followedByMajorSuffix(data[end:]) {
		return false
	}

	lineStart := bytes.LastIndexByte(data[:start], '\n') + 1
	before := data[lineStart:start]
	for _, context := range moduleReferenceContexts {
		if context.Match(before) {
			return true
		}
	}

	return false
}

// followedByMajorSuffix reports whether rest opens with a major version path element,
// "/v2" and above, ending where a path element ends.
func followedByMajorSuffix(rest []byte) bool {
	element := majorSuffixElement.Find(rest)
	if element == nil || string(element) == "/v1" {
		return false
	}
	return len(element) == len(rest) || !isModulePathByte(rest[len(element)])
}

// isModulePathByte reports whether b can continue a module path element. A "/" cannot:
// it ends one element and starts the next.
func isModulePathByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	default:
		return b == '.' || b == '-' || b == '_' || b == '~'
	}
}
