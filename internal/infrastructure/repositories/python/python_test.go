//go:build unit

package python_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/python"
)

func TestGetProjectName(t *testing.T) {
	t.Parallel()

	t.Run("should return project name when pyproject.toml exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		content := `[project]
name = "my-python-project"
version = "1.0.0"
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(content), 0o644))
		p := python.Python{ProjectConfig: entities.ProjectConfig{Path: tmpDir}}

		// when
		name, err := p.GetProjectName()

		// then
		require.NoError(t, err)
		assert.Equal(t, "my-python-project", name)
	})

	t.Run("should return error when pyproject.toml does not exist", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		p := python.Python{ProjectConfig: entities.ProjectConfig{Path: tmpDir}}

		// when
		name, err := p.GetProjectName()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, python.ErrPyprojectNotFound)
		assert.Empty(t, name)
	})

	t.Run("should return empty name when pyproject.toml has no project name", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		content := `[tool.pytest]
minversion = "6.0"
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(content), 0o644))
		p := python.Python{ProjectConfig: entities.ProjectConfig{Path: tmpDir}}

		// when
		name, err := p.GetProjectName()

		// then
		require.NoError(t, err)
		assert.Empty(t, name)
	})

	t.Run("should return error when pyproject.toml is malformed", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte("invalid toml [[["), 0o644))
		p := python.Python{ProjectConfig: entities.ProjectConfig{Path: tmpDir}}

		// when
		name, err := p.GetProjectName()

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error decoding pyproject.toml")
		assert.Empty(t, name)
	})
}
