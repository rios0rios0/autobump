package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

// effectiveEnv reads env the way exec does: the last value of a duplicated key wins.
func effectiveEnv(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok {
			values[name] = value
		}
	}
	return values
}

// assertEmptyDir fails the test with whatever dir still holds.
func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Empty(t, names, "these were left behind in %s", dir)
}

func TestNewRepositoryWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("should create the workspace in the temporary directory with room for the clone", func(t *testing.T) {
		t.Parallel()

		// given / when
		root, err := commands.NewRepositoryWorkspace()
		t.Cleanup(func() { commands.RemoveWorkspace(root) })

		// then
		require.NoError(t, err)
		//nolint:usetesting // the system temporary directory is what is being tested
		assert.Equal(t, filepath.Clean(os.TempDir()), filepath.Dir(root))
		assert.True(t, commands.IsWorkspaceName(filepath.Base(root)),
			"the stale-workspace sweep only removes a directory it recognises by name, got %q", filepath.Base(root))
		assert.NoDirExists(t, commands.WorkspaceRepoPath(root), "the clone creates its own directory")
		assert.DirExists(t, filepath.Join(commands.WorkspaceToolingPath(root), "tmp"),
			"mktemp fails in a TMPDIR that does not exist")
	})
}

func TestRemoveWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("should remove the clone and the refresh's caches with the workspace", func(t *testing.T) {
		t.Parallel()

		// given
		root, err := commands.NewRepositoryWorkspace()
		require.NoError(t, err)
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(commands.WorkspaceRepoPath(root), ".git"), 0o700))
		npmCache := filepath.Join(commands.WorkspaceToolingPath(root), "npm", "_cacache")
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(npmCache, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(npmCache, "index"), []byte("{}"), 0o600))

		// when
		commands.RemoveWorkspace(root)

		// then
		assert.NoDirExists(t, root)
	})

	t.Run("should do nothing for a local project, which has no workspace", func(t *testing.T) {
		t.Parallel()

		// given / when / then
		assert.NotPanics(t, func() { commands.RemoveWorkspace("") })
	})
}

func TestRefreshEnv(t *testing.T) {
	t.Parallel()

	t.Run("should return the process environment unchanged for a local project", func(t *testing.T) {
		t.Parallel()

		// given / when
		env := commands.RefreshEnv("")

		// then
		assert.Equal(t, os.Environ(), env)
	})

	t.Run("should move the package managers' caches and temporary files into the workspace", func(t *testing.T) {
		t.Parallel()

		// given
		toolingDir := t.TempDir()

		// when
		env := effectiveEnv(commands.RefreshEnv(toolingDir))

		// then
		for _, variable := range []string{
			"TMPDIR", "XDG_CACHE_HOME", "npm_config_cache", "PNPM_HOME", "YARN_GLOBAL_FOLDER", "COREPACK_HOME",
		} {
			assert.True(t, strings.HasPrefix(env[variable], toolingDir+string(os.PathSeparator)),
				"%s should point into the workspace, got %q", variable, env[variable])
		}
	})

	t.Run("should leave the operator's configuration and a project's committed cache alone", func(t *testing.T) {
		t.Parallel()

		// given
		toolingDir := t.TempDir()

		// when
		env := effectiveEnv(commands.RefreshEnv(toolingDir))

		// then
		for _, variable := range []string{"HOME", "XDG_CONFIG_HOME"} {
			assert.Equal(t, os.Getenv(variable), env[variable], "%s must not be redirected", variable)
		}
		assert.NotContains(t, env, "YARN_CACHE_FOLDER",
			"it would move a zero-install project's committed cache out of the repository")
		assert.NotContains(t, env, "npm_config_store_dir", "npm warns about it on every command")
	})

	t.Run("should reach the package manager the recipe runs", func(t *testing.T) {
		t.Parallel()

		// given -- a recipe standing in for npm, reporting where it was told to cache
		projectDir := t.TempDir()
		toolingDir := t.TempDir()
		run := []string{"sh", "-c", `printf '%s\n%s\n' "$npm_config_cache" "$TMPDIR" > where.txt`}

		// when
		err := commands.RunRefreshRecipe(
			projectDir, toolingDir, run, []string{"where.txt"}, nil, time.Minute, time.Second,
		)

		// then
		require.NoError(t, err)
		content, readErr := os.ReadFile(filepath.Join(projectDir, "where.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, filepath.Join(toolingDir, "npm")+"\n"+filepath.Join(toolingDir, "tmp")+"\n", string(content))
	})
}

// TestCleanupStaleWorkspaces is not parallel: it points TMPDIR at a directory of its own
// with t.Setenv, because the sweep reads the system temporary directory.
func TestCleanupStaleWorkspaces(t *testing.T) {
	t.Run("should remove only the workspaces a killed run abandoned", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		t.Setenv("TMPDIR", tmpDir)
		abandoned := filepath.Join(tmpDir, "autobump-1234567")
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(abandoned, "repo", ".git"), 0o700))
		running := filepath.Join(tmpDir, "autobump-7654321")
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(running, 0o700))
		// An operator's own directories: only a digit run and nothing else is a name
		// MkdirTemp produces, so a version, a suffix or a word after the prefix is not.
		notOurs := []string{
			filepath.Join(tmpDir, "autobump-notes"),
			filepath.Join(tmpDir, "autobump-3.3.0"),
			filepath.Join(tmpDir, "autobump-1-wip"),
		}
		for _, dir := range notOurs {
			// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
			require.NoError(t, os.MkdirAll(dir, 0o700))
		}
		notADirectory := filepath.Join(tmpDir, "autobump-2024.log")
		require.NoError(t, os.WriteFile(notADirectory, []byte("log\n"), 0o600))
		past := time.Now().Add(-time.Hour)
		for _, path := range append([]string{abandoned, notADirectory}, notOurs...) {
			require.NoError(t, os.Chtimes(path, past, past))
		}

		// when
		commands.CleanupStaleWorkspaces()

		// then
		assert.NoDirExists(t, abandoned)
		assert.DirExists(t, running, "a run still in progress keeps its workspace")
		for _, dir := range notOurs {
			assert.DirExists(t, dir, "only names MkdirTemp produces are swept")
		}
		assert.FileExists(t, notADirectory, "only directories are swept")
	})

	t.Run("should leave a link named like a workspace, and what it points to, alone", func(t *testing.T) {
		// given -- an old directory elsewhere, linked into the temporary directory under
		// a name the sweep would otherwise take
		tmpDir := t.TempDir()
		t.Setenv("TMPDIR", tmpDir)
		target := t.TempDir()
		kept := filepath.Join(target, "kept.txt")
		require.NoError(t, os.WriteFile(kept, []byte("kept\n"), 0o600))
		past := time.Now().Add(-time.Hour)
		require.NoError(t, os.Chtimes(target, past, past))
		link := filepath.Join(tmpDir, "autobump-1234567")
		require.NoError(t, os.Symlink(target, link))

		// when
		commands.CleanupStaleWorkspaces()

		// then
		_, err := os.Lstat(link)
		require.NoError(t, err, "the link itself is not a workspace")
		assert.FileExists(t, kept)
	})
}

// TestProcessRepoLeavesNothingBehind is not parallel: it points TMPDIR and HOME at
// directories of its own with t.Setenv, and replaces the package-level git operations.
func TestProcessRepoLeavesNothingBehind(t *testing.T) {
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should remove the workspace when the clone fails", func(t *testing.T) {
		// given -- a host no provider recognises, so the clone fails at once without
		// touching the network, after the workspace was already created for it
		tmpDir := t.TempDir()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"),
			[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"), 0o600))
		t.Setenv("TMPDIR", tmpDir)
		t.Setenv("HOME", home)
		projectConfig := &entities.ProjectConfig{
			Path: "https://example.invalid/org/repo.git",
			Name: "repo",
		}

		// when
		err := commands.ProcessRepo(&entities.GlobalConfig{}, projectConfig)

		// then
		require.Error(t, err)
		assertEmptyDir(t, tmpDir)
	})
}
