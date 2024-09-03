package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	openpgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestGpgKey generates a new GPG key entity for testing purposes
func generateTestGpgKey() (*openpgp.Entity, error) {
	config := &packet.Config{RSABits: 2048}
	entity, err := openpgp.NewEntity(faker.Name(), faker.Sentence(), faker.Email(), config)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

// serializeGpgKeyToReader serializes the given GPG key entity into an io.Reader
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

// Test function for a valid GPG key
func TestGetGpgKey_ValidKey(t *testing.T) {
	t.Parallel()

	// Arrange
	entity, err := generateTestGpgKey()
	require.NoError(t, err)

	gpgKeyReader, err := serializeGpgKeyToReader(entity)
	require.NoError(t, err)

	// Act
	key, err := getGpgKey(gpgKeyReader)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, key)
}

// Test function for an invalid reader
func TestGetGpgKey_InvalidReader(t *testing.T) {
	t.Parallel()

	// Arrange
	gpgKeyReader := bytes.NewReader([]byte("invalid key data"))

	// Act
	_, err := getGpgKey(gpgKeyReader)

	// Assert
	require.Error(t, err)

	var invalidArgumentError openpgpErrors.InvalidArgumentError
	require.ErrorAs(t, err, &invalidArgumentError)

	expectedErrMsg := "failed to read private key file: openpgp: invalid argument: no armored data found"
	assert.Equal(t, expectedErrMsg, err.Error())
}
