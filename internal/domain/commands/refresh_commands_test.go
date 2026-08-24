package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// configWithRefreshCommands builds the smallest configuration that reaches the runner:
// one language carrying the commands under test, and a project pointing at dir.
//
// The entities are constructed directly rather than through the builders in
// test/domain/entitybuilders, because those carry a build tag this file deliberately
// does not.
func configWithRefreshCommands(
	dir string,
	refreshCommands []entities.RefreshCommand,
) (*entities.GlobalConfig, *entities.ProjectConfig) {
	globalConfig := &entities.GlobalConfig{
		LanguagesConfig: map[string]entities.LanguageConfig{
			"typescript": {RefreshCommands: refreshCommands},
		},
	}
	projectConfig := &entities.ProjectConfig{Path: dir, Language: "typescript"}

	return globalConfig, projectConfig
}

func TestRunRefreshCommands(t *testing.T) {
	t.Parallel()

	t.Run("should report the declared file when the command regenerates it", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "echo refreshed > yarn.lock"}, Files: []string{"yarn.lock"}},
		})

		// when
		refreshed, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		require.Len(t, refreshed, 1)
		assert.Equal(t, filepath.Join(dir, "yarn.lock"), refreshed[0])

		content, readErr := os.ReadFile(filepath.Join(dir, "yarn.lock"))
		require.NoError(t, readErr)
		assert.Equal(t, "refreshed\n", string(content))
	})

	t.Run("should run the command from the project root when a relative path is written", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "pwd > cwd.txt"}, Files: []string{"cwd.txt"}},
		})

		// when
		_, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		content, readErr := os.ReadFile(filepath.Join(dir, "cwd.txt"))
		require.NoError(t, readErr)

		resolvedDir, evalErr := filepath.EvalSymlinks(dir)
		require.NoError(t, evalErr)
		assert.Equal(t, resolvedDir+"\n", string(content))
	})

	t.Run("should expand a glob when the declared file is a pattern", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "touch a.lock b.lock"}, Files: []string{"*.lock"}},
		})

		// when
		refreshed, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Equal(
			t,
			[]string{filepath.Join(dir, "a.lock"), filepath.Join(dir, "b.lock")},
			refreshed,
		)
	})

	t.Run("should report a file once when two commands declare it", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "echo one > shared.lock"}, Files: []string{"shared.lock"}},
			{Run: []string{"sh", "-c", "echo two >> shared.lock"}, Files: []string{"shared.lock"}},
		})

		// when
		refreshed, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{filepath.Join(dir, "shared.lock")}, refreshed)
	})

	// The four no-op paths differ only in what makes them a no-op, so they are a table
	// rather than four near-identical bodies.
	noopCases := []struct {
		reason   string
		commands []entities.RefreshCommand
		language string
	}{
		{
			reason:   "the declared file was not produced",
			commands: []entities.RefreshCommand{{Run: []string{"sh", "-c", "true"}, Files: []string{"yarn.lock"}}},
			language: "typescript",
		},
		{
			reason:   "the language configures no refresh commands",
			commands: nil,
			language: "typescript",
		},
		{
			reason: "the project language is unknown",
			commands: []entities.RefreshCommand{
				{Run: []string{"sh", "-c", "touch yarn.lock"}, Files: []string{"yarn.lock"}},
			},
			language: "",
		},
		{
			reason: "the language is absent from the config",
			commands: []entities.RefreshCommand{
				{Run: []string{"sh", "-c", "touch yarn.lock"}, Files: []string{"yarn.lock"}},
			},
			language: "elixir",
		},
	}

	for _, noopCase := range noopCases {
		t.Run("should report nothing when "+noopCase.reason, func(t *testing.T) {
			t.Parallel()

			// given
			dir := t.TempDir()
			globalConfig, projectConfig := configWithRefreshCommands(dir, noopCase.commands)
			projectConfig.Language = noopCase.language

			// when
			refreshed, err := commands.RunRefreshCommands(globalConfig, projectConfig)

			// then
			require.NoError(t, err)
			assert.Empty(t, refreshed)
			if noopCase.language != "typescript" {
				assert.NoFileExists(t, filepath.Join(dir, "yarn.lock"))
			}
		})
	}

	t.Run("should fail with the command output when the command exits non-zero", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "echo 'lockfile is out of date' >&2; exit 3"}, Files: []string{"yarn.lock"}},
		})

		// when
		refreshed, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "lockfile is out of date")
		assert.Nil(t, refreshed)
	})

	t.Run("should not run a later command when an earlier one fails", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		globalConfig, projectConfig := configWithRefreshCommands(dir, []entities.RefreshCommand{
			{Run: []string{"sh", "-c", "exit 1"}, Files: []string{"first.lock"}},
			{Run: []string{"sh", "-c", "touch second.lock"}, Files: []string{"second.lock"}},
		})

		// when
		_, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(dir, "second.lock"))
	})

	t.Run("should fail when the refresh command names no executable", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig, projectConfig := configWithRefreshCommands(t.TempDir(), []entities.RefreshCommand{
			{Run: nil, Files: []string{"yarn.lock"}},
		})

		// when
		_, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.ErrorIs(t, err, commands.ErrRefreshCommandEmpty)
	})

	t.Run("should fail when the executable does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig, projectConfig := configWithRefreshCommands(t.TempDir(), []entities.RefreshCommand{
			{Run: []string{"autobump-no-such-package-manager"}, Files: []string{"yarn.lock"}},
		})

		// when
		_, err := commands.RunRefreshCommands(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "autobump-no-such-package-manager")
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
