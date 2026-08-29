package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// refreshCommandTimeout bounds a single refresh command. A lockfile resolution that
// hangs on an unreachable registry would otherwise stall the whole run, which in
// batch and discover modes means every repository queued behind it.
const refreshCommandTimeout = 10 * time.Minute

// refreshCommandWaitDelay bounds how long Wait keeps reading output after the command
// itself has exited.
//
// The timeout above only governs the process AutoBump starts. A command is free to
// leave a descendant running — `sh -c` makes that a one-liner — and that descendant
// inherits the write end of the output pipe, so Wait would keep blocking on a process
// no cancellation reaches. WaitDelay is the standard-library answer: once the direct
// child is gone, Wait gives the pipes this long and then abandons them, which is what
// makes the advertised bound real rather than advisory.
const refreshCommandWaitDelay = 10 * time.Second

// ErrRefreshManagerMissing is returned when the package manager a project's refresh
// needs is not on PATH.
//
// It aborts that repository's release rather than skipping the refresh. Skipping opens
// exactly the pull request the feature exists to prevent -- green locally, rejected by the
// first CI job that installs dependencies -- and the operator would have no way to tell
// that from a release that simply had nothing to refresh. In `run` mode the blast radius
// is one repository: DiscoverAndProcess logs the failure and moves on.
var ErrRefreshManagerMissing = errors.New("refresh package manager not found")

// runRefreshCommands regenerates the files that derive from the version files AutoBump has
// just rewritten, and returns the ones it should stage alongside them.
//
// A failure aborts the release. The refresh exists to keep a derived file in step with the
// version files, so continuing past a failure would open exactly the pull request it is
// meant to prevent.
func runRefreshCommands(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) ([]string, error) {
	if projectConfig.Language == "" {
		return nil, nil
	}

	if !entities.RefreshEnabled(globalConfig, projectConfig, projectConfig.Language) {
		return nil, nil
	}

	detect, known := refreshRecipes[projectConfig.Language]
	if !known {
		logger.Warnf(
			"`refresh` is on for language %q, but AutoBump has no refresh recipe for it: "+
				"nothing it rewrites there invalidates a derived file",
			projectConfig.Language,
		)
		return nil, nil
	}

	recipe, found := detect(projectConfig.Path)
	if !found {
		logger.Warnf(
			"Could not identify a package manager in %s; skipping the refresh",
			projectConfig.Path,
		)
		return nil, nil
	}

	if _, err := exec.LookPath(recipe.Run[0]); err != nil {
		return nil, fmt.Errorf(
			"%w: %q is not on PATH, and %s needs it to refresh %s",
			ErrRefreshManagerMissing, recipe.Run[0], recipe.Manager, projectConfig.Path,
		)
	}

	err := runRefreshRecipe(
		projectConfig.Path, recipe, refreshCommandTimeout, refreshCommandWaitDelay,
	)
	if err != nil {
		return nil, err
	}

	matches, err := resolveRefreshedFiles(projectConfig.Path, recipe.Files)
	if err != nil {
		return nil, err
	}

	return dedupPaths(matches), nil
}

// runRefreshRecipe executes one recipe with the project root as its working directory.
// Output is captured rather than streamed so that a failure can report what the command
// actually said -- a package manager's diagnosis of an unresolvable range is the only
// useful thing in the error.
//
// timeout and waitDelay are parameters rather than reads of the two constants so the tests
// can exercise both bounds without waiting minutes for them.
func runRefreshRecipe(
	projectPath string,
	recipe refreshRecipe,
	timeout time.Duration,
	waitDelay time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	printable := strings.Join(recipe.Run, " ")
	logger.Infof("Refreshing with: %s", printable)

	//nolint:gosec // G204: the argv is a compile-time constant of this program. Recipes
	// live in refresh_recipes.go and no configuration layer can contribute to one -- the
	// config only says whether to refresh.
	command := exec.CommandContext(ctx, recipe.Run[0], recipe.Run[1:]...)
	command.Dir = projectPath
	command.Env = append(os.Environ(), recipe.Env...)
	command.WaitDelay = waitDelay
	configureProcessGroup(command)

	output, err := command.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))

	// A command that exited cleanly but left something behind holding the pipe reports
	// ErrWaitDelay. The release is not in question — only the tail of the output is — so
	// it is worth saying out loud and worth not failing over.
	if errors.Is(err, exec.ErrWaitDelay) {
		logger.Warnf(
			"Refresh command %q exited but left a process holding its output open; "+
				"stopped reading after %s", printable, waitDelay,
		)
		return nil
	}

	if err != nil {
		return fmt.Errorf("refresh command %q failed: %w\n%s", printable, err, trimmed)
	}

	logger.Debugf("Refresh command %q output:\n%s", printable, trimmed)
	return nil
}

// resolveRefreshedFiles expands the declared globs against the project root and keeps
// the regular files that exist. A pattern that matches nothing is not an error: the
// same configuration is reused across every project of a language, and a repository
// that carries no lockfile still has to release.
func resolveRefreshedFiles(projectPath string, patterns []string) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectPath, pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid refresh file pattern %q: %w", pattern, err)
		}

		for _, match := range matches {
			info, statErr := os.Stat(match)
			if statErr != nil || !info.Mode().IsRegular() {
				continue
			}
			files = append(files, match)
		}
	}

	return files, nil
}

// dedupPaths removes repeated paths while preserving order, so a file named by two
// commands is staged once.
func dedupPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
