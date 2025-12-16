package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	openpgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestGpgKey generates a new GPG key entity for testing purposes.
func generateTestGpgKey() (*openpgp.Entity, error) {
	config := &packet.Config{RSABits: 2048}
	entity, err := openpgp.NewEntity(faker.Name(), faker.Sentence(), faker.Email(), config)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

// serializeGpgKeyToReader serializes the given GPG key entity into an io.Reader.
func serializeGpgKeyToReader(entity *openpgp.Entity) (io.Reader, error) {
	var buf bytes.Buffer
	armorWriter, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return nil, err
	}
	defer armorWriter.Close()

	if err = entity.SerializePrivate(armorWriter, nil); err != nil {
		return nil, err
	}

	return &buf, nil
}

func TestGetGpgKey(t *testing.T) {
	t.Run("should return GPG key entity for valid key data", func(t *testing.T) {
		// given
		entity, err := generateTestGpgKey()
		require.NoError(t, err)

		gpgKeyReader, err := serializeGpgKeyToReader(entity)
		require.NoError(t, err)

		// when
		key, err := getGpgKey(gpgKeyReader)

		// then
		require.NoError(t, err, "should not return an error")
		assert.NotNil(t, key, "key should not be nil")
	})

	t.Run("should return error for invalid key data", func(t *testing.T) {
		// given
		gpgKeyReader := bytes.NewReader([]byte("invalid key data"))

		// when
		_, err := getGpgKey(gpgKeyReader)

		// then
		require.Error(t, err, "should return an error")

		var invalidArgumentError openpgpErrors.InvalidArgumentError
		require.ErrorAs(t, err, &invalidArgumentError, "should be InvalidArgumentError")

		expectedErrMsg := "failed to read private key file: openpgp: invalid argument: no armored data found"
		assert.Equal(t, expectedErrMsg, err.Error(), "error message should match")
	})
}

func TestReadLines(t *testing.T) {
	t.Run("should read lines from a valid file", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		content := "line1\nline2\nline3"
		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)

		// when
		lines, err := readLines(filePath)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Len(t, lines, 3, "should have 3 lines")
		assert.Equal(t, "line1", lines[0], "first line should match")
		assert.Equal(t, "line2", lines[1], "second line should match")
		assert.Equal(t, "line3", lines[2], "third line should match")
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		// given
		filePath := "/non/existent/file.txt"

		// when
		lines, err := readLines(filePath)

		// then
		require.Error(t, err, "should return an error")
		assert.Nil(t, lines, "lines should be nil")
		assert.Contains(t, err.Error(), "failed to open file", "error should mention file opening failure")
	})
}

func TestWriteLines(t *testing.T) {
	t.Run("should write lines to a file", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output.txt")
		lines := []string{"line1", "line2", "line3"}

		// when
		err := writeLines(filePath, lines)

		// then
		require.NoError(t, err, "should not return an error")

		// Verify the file content
		content, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr)
		assert.Contains(t, string(content), "line1", "should contain line1")
		assert.Contains(t, string(content), "line2", "should contain line2")
		assert.Contains(t, string(content), "line3", "should contain line3")
	})

	t.Run("should return error when writing to invalid path", func(t *testing.T) {
		// given
		filePath := "/non/existent/directory/output.txt"
		lines := []string{"line1"}

		// when
		err := writeLines(filePath, lines)

		// then
		require.Error(t, err, "should return an error")
		assert.Contains(t, err.Error(), "failed to create file", "error should mention file creation failure")
	})
}
