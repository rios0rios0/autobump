package commands_test

import (
	"crypto/ed25519"
	"encoding/pem"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestLoadSSHIdentity(t *testing.T) {
	t.Parallel()

	t.Run("should read the public key an IdentityFile ending in .pub names", func(t *testing.T) {
		t.Parallel()

		// given -- the form 1Password documents for choosing an agent key per host
		publicKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		path := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "work.pub"), publicKey)

		// when
		result, usable, err := commands.LoadSSHIdentity(path)

		// then -- a public key alone can only pick a key out of the agent
		require.NoError(t, err)
		assert.Equal(t, publicKey.Marshal(), result.Marshal())
		assert.False(t, usable)
	})

	t.Run("should read the .pub beside a private key path even when the private key is absent", func(t *testing.T) {
		t.Parallel()

		// given -- the key lives in the agent, and only its public half is on disk
		publicKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		dir := t.TempDir()
		writePublicKeyFixture(t, filepath.Join(dir, "id_work.pub"), publicKey)

		// when
		result, usable, err := commands.LoadSSHIdentity(filepath.Join(dir, "id_work"))

		// then
		require.NoError(t, err)
		assert.Equal(t, publicKey.Marshal(), result.Marshal())
		assert.False(t, usable)
	})

	t.Run("should load an unencrypted private key as a key usable without the agent", func(t *testing.T) {
		t.Parallel()

		// given -- OpenSSH uses such a key directly, with or without a .pub beside it
		key := newEd25519Key(t)
		dir := t.TempDir()
		path := writePrivateKeyFixture(t, filepath.Join(dir, "id_work"), key)
		writePublicKeyFixture(t, filepath.Join(dir, "id_work.pub"), newTestSigner(t, key).PublicKey())

		// when
		result, usable, err := commands.LoadSSHIdentity(path)

		// then
		require.NoError(t, err)
		assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), result.Marshal())
		assert.True(t, usable)
	})

	t.Run("should read the public half of an encrypted OpenSSH key without its passphrase", func(t *testing.T) {
		t.Parallel()

		// given -- an OpenSSH key carries its public half in the clear
		key := newEd25519Key(t)
		block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte("fixture-passphrase-placeholder"))
		require.NoError(t, err)
		path := writeSSHFixture(t, filepath.Join(t.TempDir(), "id_work"), string(pem.EncodeToMemory(block)))

		// when
		result, usable, err := commands.LoadSSHIdentity(path)

		// then -- without the passphrase it can only pick the key out of the agent
		require.NoError(t, err)
		assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), result.Marshal())
		assert.False(t, usable)
	})

	t.Run("should fail when neither the file nor its .pub exists", func(t *testing.T) {
		t.Parallel()

		// given
		path := filepath.Join(t.TempDir(), "absent")

		// when
		_, _, err := commands.LoadSSHIdentity(path)

		// then
		require.Error(t, err)
	})
}

func TestSelectSigners(t *testing.T) {
	t.Parallel()

	t.Run("should offer only the named keys, in IdentityFile order, when IdentitiesOnly is set", func(t *testing.T) {
		t.Parallel()

		// given
		agentKeys := newAgentSigners(t, 3)
		dir := t.TempDir()
		third := writePublicKeyFixture(t, filepath.Join(dir, "third.pub"), agentKeys[2].PublicKey())
		first := writePublicKeyFixture(t, filepath.Join(dir, "first.pub"), agentKeys[0].PublicKey())

		// when
		selected := commands.SelectSigners(agentKeys, []string{third, first}, true)

		// then
		assert.Equal(t, []ssh.Signer{agentKeys[2], agentKeys[0]}, selected)
	})

	t.Run("should offer the named keys first and the agent's others after them", func(t *testing.T) {
		t.Parallel()

		// given -- without IdentitiesOnly, OpenSSH falls back to the rest of the agent
		agentKeys := newAgentSigners(t, 3)
		named := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "named.pub"), agentKeys[2].PublicKey())

		// when
		selected := commands.SelectSigners(agentKeys, []string{named}, false)

		// then
		assert.Equal(t, []ssh.Signer{agentKeys[2], agentKeys[0], agentKeys[1]}, selected)
	})

	t.Run("should offer nothing when IdentitiesOnly is set and no named key is usable", func(t *testing.T) {
		t.Parallel()

		// given -- only the public half of a key the agent does not hold
		agentKeys := newAgentSigners(t, 2)
		stranger := newTestSigner(t, newEd25519Key(t)).PublicKey()
		named := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "stranger.pub"), stranger)

		// when
		selected := commands.SelectSigners(agentKeys, []string{named}, true)

		// then
		assert.Empty(t, selected)
	})

	t.Run(
		"should offer an unencrypted key file the agent does not hold when IdentitiesOnly is set",
		func(t *testing.T) {
			t.Parallel()

			// given -- the key is on disk, and the agent holds keys of other accounts
			agentKeys := newAgentSigners(t, 2)
			key := newEd25519Key(t)
			path := writePrivateKeyFixture(t, filepath.Join(t.TempDir(), "id_work"), key)

			// when
			selected := commands.SelectSigners(agentKeys, []string{path}, true)

			// then
			require.Len(t, selected, 1)
			assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), selected[0].PublicKey().Marshal())
		},
	)

	t.Run("should offer an unencrypted key file the agent does not hold after the agent's keys", func(t *testing.T) {
		t.Parallel()

		// given -- OpenSSH tries keys only listed in the config file last
		agentKeys := newAgentSigners(t, 2)
		key := newEd25519Key(t)
		path := writePrivateKeyFixture(t, filepath.Join(t.TempDir(), "id_work"), key)

		// when
		selected := commands.SelectSigners(agentKeys, []string{path}, false)

		// then
		require.Len(t, selected, 3)
		assert.Equal(t, []ssh.Signer{agentKeys[0], agentKeys[1]}, selected[:2])
		assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), selected[2].PublicKey().Marshal())
	})

	t.Run("should offer the agent's copy of a key the file holds too", func(t *testing.T) {
		t.Parallel()

		// given -- the agent may hold the key the file names
		key := newEd25519Key(t)
		agentKeys := []ssh.Signer{newTestSigner(t, newEd25519Key(t)), newTestSigner(t, key)}
		path := writePrivateKeyFixture(t, filepath.Join(t.TempDir(), "id_work"), key)

		// when
		selected := commands.SelectSigners(agentKeys, []string{path}, true)

		// then -- once, and the agent's
		assert.Equal(t, []ssh.Signer{agentKeys[1]}, selected)
	})

	t.Run("should skip an IdentityFile that names no readable key", func(t *testing.T) {
		t.Parallel()

		// given
		agentKeys := newAgentSigners(t, 2)
		absent := filepath.Join(t.TempDir(), "absent.pub")

		// when
		selected := commands.SelectSigners(agentKeys, []string{absent}, false)

		// then
		assert.Equal(t, agentKeys, selected)
	})
}

// newAgentSigners returns count signers standing in for the keys an agent lists.
func newAgentSigners(t *testing.T, count int) []ssh.Signer {
	t.Helper()

	signers := make([]ssh.Signer, 0, count)
	for range count {
		signers = append(signers, newTestSigner(t, newEd25519Key(t)))
	}
	return signers
}

// writePrivateKeyFixture writes key to path as an unencrypted OpenSSH private key.
func writePrivateKeyFixture(t *testing.T, path string, key ed25519.PrivateKey) string {
	t.Helper()

	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	return writeSSHFixture(t, path, string(pem.EncodeToMemory(block)))
}
