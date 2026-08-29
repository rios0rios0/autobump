package commands_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// refreshConfig builds the smallest configuration that reaches the runner: one language,
// a project pointing at dir, and the refresh either on or off.
//
// The entities are constructed directly rather than through the builders in
// test/domain/entitybuilders, because those carry a build tag this file deliberately
// does not.
func refreshConfig(dir, language string, refresh bool) (*entities.GlobalConfig, *entities.ProjectConfig) {
	globalConfig := &entities.GlobalConfig{
		Refresh:         &refresh,
		LanguagesConfig: map[string]entities.LanguageConfig{language: {}},
	}
	projectConfig := &entities.ProjectConfig{Path: dir, Language: language}

	return globalConfig, projectConfig
}

func TestRunRefreshCommands(t *testing.T) {
	t.Parallel()

	t.Run("should do nothing when the refresh is off", func(t *testing.T) {
		t.Parallel()

		// given -- the refresh is opt-in, so a project with a lockfile and no `refresh`
		// must still be left alone
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0o600))
		globalConfig, projectConfig := refreshConfig(dir, "typescript", false)

		// when
		files, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("should do nothing when the language is unknown", func(t *testing.T) {
		t.Parallel()

		// given -- nothing AutoBump rewrites in a Go project invalidates a derived file,
		// so `refresh: true` there is a warning rather than a failure
		globalConfig, projectConfig := refreshConfig(t.TempDir(), "golang", true)

		// when
		files, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("should do nothing when no package manager can be identified", func(t *testing.T) {
		t.Parallel()

		// given -- a TypeScript project with no lockfile and no packageManager field
		globalConfig, projectConfig := refreshConfig(t.TempDir(), "typescript", true)

		// when
		files, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("should do nothing when the project has no language", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig, projectConfig := refreshConfig(t.TempDir(), "typescript", true)
		projectConfig.Language = ""

		// when
		files, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Empty(t, files)
	})
}

// This cannot join TestRunRefreshCommands above: it calls t.Parallel(), and t.Setenv is
// refused anywhere under a parallel ancestor.
func TestRunRefreshCommandsWithoutPackageManager(t *testing.T) {
	// given -- skipping a refresh whose package manager is missing would open exactly the
	// pull request the refresh exists to prevent, and would look identical to a release
	// that simply had nothing to refresh
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"packageManager":"pnpm@9.0.0"}`), 0o600))
	globalConfig, projectConfig := refreshConfig(dir, "typescript", true)
	t.Setenv("PATH", t.TempDir())

	// when
	files, err := commands.RunRefreshCommands(globalConfig, projectConfig)

	// then
	require.ErrorIs(t, err, commands.ErrRefreshManagerMissing)
	assert.Empty(t, files)
}

func TestRunRefreshCommandBounds(t *testing.T) {
	t.Parallel()

	t.Run("should stop waiting when the command leaves a process holding its output", func(t *testing.T) {
		t.Parallel()

		// given
		// The shell exits at once and leaves `sleep` holding the write end of the output
		// pipe. Without a wait delay the call blocks for the sleep's full duration, which
		// the command's own timeout never reaches because the process it names is gone.
		run := []string{"sh", "-c", "sleep 30 &"}

		// when
		started := time.Now()
		err := commands.RunRefreshRecipe(
			t.TempDir(), run, []string{"yarn.lock"}, nil, time.Minute, 200*time.Millisecond,
		)
		elapsed := time.Since(started)

		// then
		require.NoError(t, err)
		assert.Less(t, elapsed, 10*time.Second, "the call should have abandoned the pipe, not waited for the sleep")
	})

	t.Run("should kill the whole process group when the command outruns its timeout", func(t *testing.T) {
		t.Parallel()

		// given
		// The shell waits, so the process that has to die is the grandchild. Killing only
		// the process AutoBump started would leave it running and let it create the file.
		dir := t.TempDir()
		run := []string{"sh", "-c", "(sleep 2; touch survivor.txt) & wait"}

		// when
		started := time.Now()
		err := commands.RunRefreshRecipe(
			dir, run, []string{"survivor.txt"}, nil, 200*time.Millisecond, time.Second,
		)
		elapsed := time.Since(started)

		// then
		require.Error(t, err)
		assert.Less(t, elapsed, 10*time.Second)

		// The grandchild would have created the file by now had it survived the kill.
		time.Sleep(3 * time.Second)
		assert.NoFileExists(t, filepath.Join(dir, "survivor.txt"))
	})
}

func TestResolveRefreshedFiles(t *testing.T) {
	t.Parallel()

	t.Run("should skip a directory when a pattern matches one", func(t *testing.T) {
		t.Parallel()

		// given
		// The directory is created 0600 rather than the usual 0750 because anything
		// above 0600 is a semgrep finding, and this test needs no more: Glob reads the
		// parent and Stat needs no traversal into the directory itself. Reaching for
		// os.MkdirTemp to dodge the literal instead trades the finding for a
		// `usetesting` one, and t.TempDir cannot place the directory inside dir.
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "build"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "build.lock"), []byte("x"), 0o600))

		// when
		files, err := commands.ResolveRefreshedFiles(dir, []string{"build*"})

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{filepath.Join(dir, "build.lock")}, files)
	})

	t.Run("should fail when the pattern is malformed", func(t *testing.T) {
		t.Parallel()

		// when
		_, err := commands.ResolveRefreshedFiles(t.TempDir(), []string{"["})

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid refresh file pattern")
	})
}
