//go:build unit

package support_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/support"
)

func TestReadLines(t *testing.T) {
	t.Parallel()

	t.Run("should read all lines from a file when file exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3"), 0o644))

		// when
		lines, err := support.ReadLines(filePath)

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		// given
		filePath := "/nonexistent/path/file.txt"

		// when
		lines, err := support.ReadLines(filePath)

		// then
		require.Error(t, err)
		assert.Nil(t, lines)
		assert.Contains(t, err.Error(), "failed to open file")
	})

	t.Run("should return empty slice when file is empty", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "empty.txt")
		require.NoError(t, os.WriteFile(filePath, []byte(""), 0o644))

		// when
		lines, err := support.ReadLines(filePath)

		// then
		require.NoError(t, err)
		assert.Empty(t, lines)
	})
}

func TestWriteLines(t *testing.T) {
	t.Parallel()

	t.Run("should write lines to a file when path is valid", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output.txt")
		lines := []string{"line1", "line2", "line3"}

		// when
		err := support.WriteLines(filePath, lines)

		// then
		require.NoError(t, err)
		content, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr)
		assert.Equal(t, "line1\nline2\nline3\n", string(content))
	})

	t.Run("should return error when path is not writable", func(t *testing.T) {
		// given
		filePath := "/nonexistent/dir/output.txt"
		lines := []string{"line1"}

		// when
		err := support.WriteLines(filePath, lines)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create file")
	})

	t.Run("should write empty file when lines is empty", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "empty.txt")

		// when
		err := support.WriteLines(filePath, []string{})

		// then
		require.NoError(t, err)
		content, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr)
		assert.Equal(t, "", string(content))
	})
}
