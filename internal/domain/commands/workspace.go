package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"
)

// workspacePrefix names the directory a remote repository is cloned into, as an
// [os.MkdirTemp] pattern. MkdirTemp appends a run of digits to it and nothing else,
// which is what isWorkspaceName relies on.
const workspacePrefix = "autobump-"

// staleWorkspaceAge is how old a workspace must be before the sweep treats it as
// abandoned. A repository takes seconds to release, so anything this old belongs to
// a run that was killed, not to one still running beside this one.
const staleWorkspaceAge = 30 * time.Minute

const (
	workspaceRepoDir    = "repo"
	workspaceToolingDir = "tooling"
	toolingTempDir      = "tmp"
	workspaceDirMode    = 0o700
)

// refreshCacheVariables point the package managers a refresh recipe runs at the
// repository's workspace, so what they download and every temporary file they write
// are removed with the clone instead of collecting in the operator's home across every
// repository a run reaches.
//
// Only caches move. HOME and XDG_CONFIG_HOME stay where they are, so `.npmrc`,
// `.yarnrc.yml` and the registry credentials in them are read as before. Two variables
// are deliberately absent: YARN_CACHE_FOLDER would pull a zero-install project's
// committed `.yarn/cache` out of the repository, and npm_config_store_dir -- pnpm's older
// spelling -- makes npm warn about an unknown setting on every command, while PNPM_HOME
// moves the same store on every pnpm version. A recipe for another ecosystem adds its
// own cache variables here.
//
//nolint:gochecknoglobals // read-only lookup table
var refreshCacheVariables = []struct {
	variable string
	dir      string
}{
	{variable: "TMPDIR", dir: toolingTempDir},
	{variable: "XDG_CACHE_HOME", dir: "cache"},
	{variable: "npm_config_cache", dir: "npm"},
	{variable: "PNPM_HOME", dir: "pnpm"},
	{variable: "YARN_GLOBAL_FOLDER", dir: "yarn"},
	{variable: "COREPACK_HOME", dir: "corepack"},
}

// newRepositoryWorkspace creates the directory a remote repository costs this run: the
// clone goes in `repo/`, and `tooling/` takes what the lockfile refresh writes outside
// it. It is returned even when only partly prepared, so the caller removes it either way.
func newRepositoryWorkspace() (string, error) {
	root, err := os.MkdirTemp("", workspacePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// `mktemp` fails in a TMPDIR that does not exist, so it is the one tooling
	// directory created up front; every cache is created by its own tool on first use.
	tempDir := filepath.Join(workspaceToolingPath(root), toolingTempDir)
	// A directory needs the owner search bit, so 0o700 is the least-privilege mode one
	// can be created with; the rule compares it against 0600 regardless.
	// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
	if err = os.MkdirAll(tempDir, workspaceDirMode); err != nil {
		return root, fmt.Errorf("failed to prepare the workspace: %w", err)
	}
	return root, nil
}

// workspaceRepoPath is where a workspace holds its clone.
func workspaceRepoPath(root string) string {
	return filepath.Join(root, workspaceRepoDir)
}

// workspaceToolingPath is where a workspace holds the refresh's caches and temporary files.
func workspaceToolingPath(root string) string {
	return filepath.Join(root, workspaceToolingDir)
}

// removeWorkspace deletes a repository's workspace. An empty root is a local project,
// which has none. A failure is reported rather than returned: it happens on the way out
// of ProcessRepo, which has already decided its outcome, and a workspace left behind is
// exactly the growth the workspace exists to prevent, so it has to be visible.
func removeWorkspace(root string) {
	if root == "" {
		return
	}
	if err := os.RemoveAll(root); err != nil {
		logger.Warnf("Could not remove the workspace %s: %v", root, err)
		return
	}
	logger.Debugf("Removed the workspace %s", root)
}

// refreshEnv returns the environment a refresh recipe runs with: the process
// environment, with the package managers' caches and temporary files moved into
// toolingDir. An empty toolingDir -- a local project, refreshed in the operator's own
// checkout -- returns the process environment unchanged.
func refreshEnv(toolingDir string) []string {
	env := os.Environ()
	if toolingDir == "" {
		return env
	}
	for _, redirect := range refreshCacheVariables {
		env = append(env, redirect.variable+"="+filepath.Join(toolingDir, redirect.dir))
	}
	return env
}

// isWorkspaceName reports whether name is one [os.MkdirTemp] could have given a
// workspace: the prefix followed by digits and nothing else. The sweep deletes whatever
// this accepts, so it is exact -- an operator's own `autobump-notes`, `autobump-3.3.0` or
// `autobump-1-wip` in the temporary directory is not a workspace.
func isWorkspaceName(name string) bool {
	digits, found := strings.CutPrefix(name, workspacePrefix)
	return found && digits != "" && strings.Trim(digits, "0123456789") == ""
}

// CleanupStaleWorkspaces removes the workspaces a run left behind because it was killed
// (SIGKILL, out of memory) before ProcessRepo could remove them. Each one holds a whole
// clone, so on a machine that runs AutoBump on a schedule they would otherwise pile up
// run after run. Only workspaces older than staleWorkspaceAge are touched, so a run
// still in progress beside this one keeps its own.
//
// It lists the temporary directory instead of globbing it, because a glob cannot say
// "digits and nothing else" and the sweep deletes whatever it selects.
func CleanupStaleWorkspaces() {
	tempDir := os.TempDir()
	// A listing cut short by an error still returns the entries it read, and those are
	// judged like any other.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		logger.Debugf("Could not list the temporary directory %s: %v", tempDir, err)
	}
	cutoff := time.Now().Add(-staleWorkspaceAge)
	for _, entry := range entries {
		// An entry describes a symbolic link as a link, never as the directory it points
		// to, so a link named like a workspace is skipped rather than followed.
		if !entry.IsDir() || !isWorkspaceName(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.ModTime().After(cutoff) {
			continue
		}
		workspace := filepath.Join(tempDir, entry.Name())
		logger.Debugf("Removing the stale workspace %s", workspace)
		removeWorkspace(workspace)
	}
}
