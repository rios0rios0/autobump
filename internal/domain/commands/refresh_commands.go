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

// ErrRefreshCommandEmpty is returned when a configured refresh command has no
// executable. An empty `run` is a typo in the config rather than a repository state
// worth tolerating, so it is reported instead of skipped.
var ErrRefreshCommandEmpty = errors.New("refresh command has no executable")

// runRefreshCommands executes the language's refresh commands in the project
// directory and returns the existing files they were declared to regenerate, so the
// caller can stage them alongside the version files.
//
// A failure aborts the release. The commands exist to keep a derived file in step
// with the version files, so continuing past one would open exactly the pull request
// the feature is meant to prevent — green-looking locally, rejected by the first
// pipeline job that installs dependencies.
func runRefreshCommands(
	globalConfig *entities.GlobalConfig,
	projectConfig *entities.ProjectConfig,
) ([]string, error) {
	if projectConfig.Language == "" {
		return nil, nil
	}

	languageConfig, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists || len(languageConfig.RefreshCommands) == 0 {
		return nil, nil
	}

	var refreshed []string
	for _, refreshCommand := range languageConfig.RefreshCommands {
		if err := runRefreshCommand(projectConfig.Path, refreshCommand); err != nil {
			return nil, err
		}

		matches, err := resolveRefreshedFiles(projectConfig.Path, refreshCommand.Files)
		if err != nil {
			return nil, err
		}
		refreshed = append(refreshed, matches...)
	}

	return dedupPaths(refreshed), nil
}

// runRefreshCommand executes one command with the project root as its working
// directory. Output is captured rather than streamed so that a failure can report
// what the command actually said — a package manager's diagnosis of an unresolvable
// range is the only useful thing in the error.
func runRefreshCommand(projectPath string, refreshCommand entities.RefreshCommand) error {
	if len(refreshCommand.Run) == 0 {
		return ErrRefreshCommandEmpty
	}

	ctx, cancel := context.WithTimeout(context.Background(), refreshCommandTimeout)
	defer cancel()

	printable := strings.Join(refreshCommand.Run, " ")
	logger.Infof("Running refresh command: %s", printable)

	//nolint:gosec // G204: the command comes from the operator's own configuration file,
	// which already names the repositories AutoBump clones, commits to and pushes.
	command := exec.CommandContext(ctx, refreshCommand.Run[0], refreshCommand.Run[1:]...)
	command.Dir = projectPath

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"refresh command %q failed: %w\n%s", printable, err, strings.TrimSpace(string(output)),
		)
	}

	logger.Debugf("Refresh command %q output:\n%s", printable, strings.TrimSpace(string(output)))
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
