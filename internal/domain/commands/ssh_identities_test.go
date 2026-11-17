package commands_test

import (
	"encoding/pem"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestIdentityPublicKey(t *testing.T) {
	t.Parallel()

	t.Run("should read the public key an IdentityFile ending in .pub names", func(t *testing.T) {
		t.Parallel()

		// given -- the form 1Password documents for choosing an agent key per host
		publicKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		path := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "work.pub"), publicKey)

		// when
		result, err := commands.IdentityPublicKey(path)

		// then
		require.NoError(t, err)
		assert.Equal(t, publicKey.Marshal(), result.Marshal())
	})

	t.Run("should read the .pub beside a private key path even when the private key is absent", func(t *testing.T) {
		t.Parallel()

		// given -- the key lives in the agent, and only its public half is on disk
		publicKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		dir := t.TempDir()
		writePublicKeyFixture(t, filepath.Join(dir, "id_work.pub"), publicKey)

		// when
		result, err := commands.IdentityPublicKey(filepath.Join(dir, "id_work"))

		// then
		require.NoError(t, err)
		assert.Equal(t, publicKey.Marshal(), result.Marshal())
	})

	t.Run("should read the public half of an unencrypted private key with no .pub", func(t *testing.T) {
		t.Parallel()

		// given
		key := newEd25519Key(t)
		block, err := ssh.MarshalPrivateKey(key, "")
		require.NoError(t, err)
		path := writeSSHFixture(t, filepath.Join(t.TempDir(), "id_work"), string(pem.EncodeToMemory(block)))

		// when
		result, err := commands.IdentityPublicKey(path)

		// then
		require.NoError(t, err)
		assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), result.Marshal())
	})

	t.Run("should read the public half of an encrypted OpenSSH key without its passphrase", func(t *testing.T) {
		t.Parallel()

		// given -- an OpenSSH key carries its public half in the clear
		key := newEd25519Key(t)
		block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte("fixture-passphrase-placeholder"))
		require.NoError(t, err)
		path := writeSSHFixture(t, filepath.Join(t.TempDir(), "id_work"), string(pem.EncodeToMemory(block)))

		// when
		result, err := commands.IdentityPublicKey(path)

		// then
		require.NoError(t, err)
		assert.Equal(t, newTestSigner(t, key).PublicKey().Marshal(), result.Marshal())
	})

	t.Run("should fail when neither the file nor its .pub exists", func(t *testing.T) {
		t.Parallel()

		// given
		path := filepath.Join(t.TempDir(), "absent")

		// when
		_, err := commands.IdentityPublicKey(path)

		// then
		require.Error(t, err)
	})
}

func TestSelectAgentSigners(t *testing.T) {
	t.Parallel()

	t.Run("should offer only the named keys, in IdentityFile order, when IdentitiesOnly is set", func(t *testing.T) {
		t.Parallel()

		// given
		agentKeys := newAgentSigners(t, 3)
		dir := t.TempDir()
		third := writePublicKeyFixture(t, filepath.Join(dir, "third.pub"), agentKeys[2].PublicKey())
		first := writePublicKeyFixture(t, filepath.Join(dir, "first.pub"), agentKeys[0].PublicKey())

		// when
		selected := commands.SelectAgentSigners(agentKeys, []string{third, first}, true)

		// then
		assert.Equal(t, []ssh.Signer{agentKeys[2], agentKeys[0]}, selected)
	})

	t.Run("should offer the named keys first and the agent's others after them", func(t *testing.T) {
		t.Parallel()

		// given -- without IdentitiesOnly, OpenSSH falls back to the rest of the agent
		agentKeys := newAgentSigners(t, 3)
		named := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "named.pub"), agentKeys[2].PublicKey())

		// when
		selected := commands.SelectAgentSigners(agentKeys, []string{named}, false)

		// then
		assert.Equal(t, []ssh.Signer{agentKeys[2], agentKeys[0], agentKeys[1]}, selected)
	})

	t.Run("should offer nothing when IdentitiesOnly is set and the agent holds no named key", func(t *testing.T) {
		t.Parallel()

		// given
		agentKeys := newAgentSigners(t, 2)
		stranger := newTestSigner(t, newEd25519Key(t)).PublicKey()
		named := writePublicKeyFixture(t, filepath.Join(t.TempDir(), "stranger.pub"), stranger)

		// when
		selected := commands.SelectAgentSigners(agentKeys, []string{named}, true)

		// then
		assert.Empty(t, selected)
	})

	t.Run("should skip an IdentityFile that names no readable key", func(t *testing.T) {
		t.Parallel()

		// given
		agentKeys := newAgentSigners(t, 2)
		absent := filepath.Join(t.TempDir(), "absent.pub")

		// when
		selected := commands.SelectAgentSigners(agentKeys, []string{absent}, false)

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
