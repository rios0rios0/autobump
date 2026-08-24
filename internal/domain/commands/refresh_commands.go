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
		err := runRefreshCommand(
			projectConfig.Path, refreshCommand, refreshCommandTimeout, refreshCommandWaitDelay,
		)
		if err != nil {
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
//
// timeout and waitDelay are parameters rather than reads of the two constants so the
// tests can exercise both bounds without waiting minutes for them.
func runRefreshCommand(
	projectPath string,
	refreshCommand entities.RefreshCommand,
	timeout time.Duration,
	waitDelay time.Duration,
) error {
	if len(refreshCommand.Run) == 0 {
		return ErrRefreshCommandEmpty
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	printable := strings.Join(refreshCommand.Run, " ")
	logger.Infof("Running refresh command: %s", printable)

	//nolint:gosec // G204: refresh commands are only read from the operator's own global
	// configuration. entities.SanitizeUntrustedLanguages drops any that a released
	// repository declares in its own .autobump.yaml, so a discovered repository cannot
	// reach this call.
	command := exec.CommandContext(ctx, refreshCommand.Run[0], refreshCommand.Run[1:]...)
	command.Dir = projectPath
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
