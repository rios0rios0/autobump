package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	logger "github.com/sirupsen/logrus"
)

// The package managers AutoBump has a resolution-only recipe for, and the subcommand they
// share. Named because the recipe, the detector table and the manager lookup all repeat them.
const (
	managerYarn = "yarn"
	managerNpm  = "npm"
	managerPnpm = "pnpm"

	subcommandInstall = "install"
)

// refreshRecipe is a resolution-only command AutoBump owns.
//
// Nothing in it comes from configuration. A configuration file says *whether* to refresh;
// what runs is a compile-time constant of this program. That is the whole reason the
// project layer can override everything else freely: there is no argv for a repository to
// supply, so there is no execution to sanitize and no trust boundary to get wrong.
type refreshRecipe struct {
	// Manager is the package manager's name, for logs and errors.
	Manager string

	// Run is the argv, executed directly rather than through a shell.
	Run []string

	// Files are glob patterns, relative to the project root, naming what the command
	// regenerates. Only these are staged, so a refresh cannot sweep an operator's
	// unrelated worktree changes into the release commit.
	Files []string

	// Env is appended to the process environment for this command.
	Env []string
}

// refreshDetector picks the recipe a project uses, or reports that none applies.
type refreshDetector func(projectPath string) (refreshRecipe, bool)

// refreshRecipes maps a configured language key to the detector for its ecosystem.
//
// A language absent from this table has no derived file that AutoBump's rewrite can
// invalidate, so `refresh: true` there is a warning rather than an error. That is most of
// them: a lockfile only goes stale when the rewrite changes a string the lockfile keys on,
// and go.mod/go.sum never record the module's own version, Python's version file is
// `{project_name}/__init__.py` while poetry and pdm exclude the root project, and
// pom.xml/build.gradle/Chart.yaml/*.csproj have no lockfile that carries it at all. A
// JavaScript workspace is the case that does: moving the range one package declares on its
// sibling changes a resolution descriptor.
//
//nolint:gochecknoglobals // read-only lookup table
var refreshRecipes = map[string]refreshDetector{
	"typescript": detectNodeRecipe,
	"javascript": detectNodeRecipe,
	"node":       detectNodeRecipe,
}

//nolint:gochecknoglobals // read-only recipe definitions
var (
	// yarnBerryRecipe uses the mode Renovate and Dependabot use. `update-lockfile` skips
	// the link step altogether, so no build or lifecycle script runs.
	//
	// It carries no --no-immutable: from Yarn 3.2 (yarnpkg/berry#3933) this mode disables
	// immutable installs by itself, and passing --immutable alongside it is an error. The
	// environment variable is the escape hatch for 2.x-3.1, where it does not.
	yarnBerryRecipe = refreshRecipe{
		Manager: managerYarn,
		Run:     []string{managerYarn, subcommandInstall, "--mode=update-lockfile"},
		Files:   []string{"yarn.lock"},
		Env:     []string{"YARN_ENABLE_IMMUTABLE_INSTALLS=false"},
	}

	// npmRecipe passes --ignore-scripts because npm has run the root package's `prepare`
	// and `postinstall` under --package-lock-only since npm 7 (npm/cli#2787). It cannot
	// change the resulting lockfile, so refusing the scripts costs nothing.
	npmRecipe = refreshRecipe{
		Manager: managerNpm,
		Run:     []string{managerNpm, subcommandInstall, "--package-lock-only", "--ignore-scripts"},
		Files:   []string{"package-lock.json"},
	}

	// pnpmRecipe passes --no-frozen-lockfile because pnpm turns frozen-lockfile on by
	// default when it detects CI -- which is exactly where `autobump run` lives, and a
	// frozen install aborts on the out-of-date lockfile this command exists to repair.
	pnpmRecipe = refreshRecipe{
		Manager: managerPnpm,
		Run: []string{
			managerPnpm, subcommandInstall, "--lockfile-only", "--no-frozen-lockfile", "--ignore-scripts",
		},
		Files: []string{"pnpm-lock.yaml"},
	}
)

// nodeMarker is one way of telling which package manager a repository uses.
type nodeMarker struct {
	Name   string
	Detect refreshDetector
}

// nodeMarkers is ordered, and the order is the point. A repository migrating between
// package managers carries two lockfiles, and `packageManager` is the one signal that says
// which of them is current rather than which of them is left over.
//
//nolint:gochecknoglobals // read-only lookup table
var nodeMarkers = []nodeMarker{
	{Name: "the packageManager field", Detect: detectByPackageManagerField},
	{Name: ".yarnrc.yml", Detect: detectByYarnrc},
	{Name: "pnpm-lock.yaml", Detect: markerRecipe("pnpm-lock.yaml", pnpmRecipe)},
	{Name: "yarn.lock", Detect: detectByYarnLock},
	{Name: "package-lock.json", Detect: markerRecipe("package-lock.json", npmRecipe)},
}

// detectNodeRecipe walks the markers in order and returns the first recipe one identifies.
func detectNodeRecipe(projectPath string) (refreshRecipe, bool) {
	for _, marker := range nodeMarkers {
		if recipe, found := marker.Detect(projectPath); found {
			logger.Debugf("Refreshing with %s, identified by %s", recipe.Manager, marker.Name)
			return recipe, true
		}
	}

	return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
}

// managerRecipes maps a package manager's name to its recipe, for the markers that name
// the manager directly rather than implying it.
//
//nolint:gochecknoglobals // read-only lookup table
var managerRecipes = map[string]refreshRecipe{
	managerYarn: yarnBerryRecipe,
	managerNpm:  npmRecipe,
	managerPnpm: pnpmRecipe,
}

// packageJSON is the sliver of package.json that says which package manager is in use.
type packageJSON struct {
	PackageManager string `json:"packageManager"`
}

// detectByPackageManagerField reads the `packageManager` field Corepack standardised.
// It is the most reliable marker because it states an intent rather than leaving one to be
// inferred from which files happen to be on disk.
func detectByPackageManagerField(projectPath string) (refreshRecipe, bool) {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	var manifest packageJSON
	if unmarshalErr := json.Unmarshal(data, &manifest); unmarshalErr != nil {
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	name, version, _ := strings.Cut(manifest.PackageManager, "@")
	recipe, known := managerRecipes[strings.TrimSpace(name)]
	if !known {
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	if recipe.Manager == managerYarn && isYarnClassic(version) {
		warnYarnClassic(projectPath)
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	return recipe, true
}

// detectByYarnrc treats a .yarnrc.yml as Yarn Berry. Classic reads .yarnrc, without the
// extension and without the YAML, so the two never collide.
func detectByYarnrc(projectPath string) (refreshRecipe, bool) {
	if _, err := os.Stat(filepath.Join(projectPath, ".yarnrc.yml")); err != nil {
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	return yarnBerryRecipe, true
}

// detectByYarnLock distinguishes a Berry lockfile from a Classic one by the header each
// writes. The file name is identical, and the difference matters: Classic has no
// resolution-only install, so there is no safe recipe for it at all.
func detectByYarnLock(projectPath string) (refreshRecipe, bool) {
	data, err := os.ReadFile(filepath.Join(projectPath, "yarn.lock"))
	if err != nil {
		return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
	}

	head := string(data)
	if len(head) > yarnLockHeaderBytes {
		head = head[:yarnLockHeaderBytes]
	}

	if strings.Contains(head, "__metadata:") {
		return yarnBerryRecipe, true
	}

	if strings.Contains(head, "yarn lockfile v1") {
		warnYarnClassic(projectPath)
	}

	return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
}

// yarnLockHeaderBytes is how much of a lockfile is read to identify it. Both markers live
// in the preamble, and a lockfile can be tens of megabytes.
const yarnLockHeaderBytes = 512

// yarnClassicMajor is the Yarn major version that has no resolution-only install mode.
const yarnClassicMajor = "1"

// isYarnClassic reports whether a `packageManager` version names Yarn 1.x.
func isYarnClassic(version string) bool {
	major, _, _ := strings.Cut(strings.TrimSpace(version), ".")
	if major == "" {
		return false
	}
	if _, err := strconv.Atoi(major); err != nil {
		return false
	}

	return major == yarnClassicMajor
}

// warnYarnClassic says why a Yarn Classic project is left alone. Silence would look like
// the refresh having run, which is the failure mode the whole feature exists to prevent.
func warnYarnClassic(projectPath string) {
	logger.Warnf(
		"Skipping the refresh for %s: it uses Yarn Classic, which has no install mode that "+
			"resolves the lockfile without also linking and running install scripts. "+
			"Migrate to Yarn 2+ or turn `refresh` off for this project",
		projectPath,
	)
}

// markerRecipe builds a detector that answers with one recipe when a marker file exists.
func markerRecipe(marker string, recipe refreshRecipe) refreshDetector {
	return func(projectPath string) (refreshRecipe, bool) {
		if _, err := os.Stat(filepath.Join(projectPath, marker)); err != nil {
			return refreshRecipe{}, false //nolint:exhaustruct // the zero value is the "none" case
		}
		return recipe, true
	}
}
