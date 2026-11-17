package commands_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net"
	"sync/atomic"
	"testing"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestSSHAgentSigners(t *testing.T) {
	t.Parallel()

	t.Run("should list the agent's keys in the agent's order", func(t *testing.T) {
		t.Parallel()

		// given
		first, second := newEd25519Key(t), newEd25519Key(t)
		listSigners := agentSignerCallback(t, serveSSHAgent(t, first, second))

		// when
		signers, err := listSigners()

		// then
		require.NoError(t, err)
		require.Len(t, signers, 2)
		assert.Equal(t, newTestSigner(t, first).PublicKey().Marshal(), signers[0].PublicKey().Marshal())
		assert.Equal(t, newTestSigner(t, second).PublicKey().Marshal(), signers[1].PublicKey().Marshal())
	})

	t.Run("should still sign after the callback that listed the keys returned", func(t *testing.T) {
		t.Parallel()

		// given -- x/crypto signs only after the callback returns; a signer bound to the
		// connection that listed the keys used to fail here with "use of closed network
		// connection", which failed every agent-based push AutoBump attempted
		listSigners := agentSignerCallback(t, serveSSHAgent(t, newEd25519Key(t)))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		data := []byte("session identifier the server asks to be signed")

		// when
		signature, err := signers[0].Sign(rand.Reader, data)

		// then
		require.NoError(t, err)
		require.NoError(t, signers[0].PublicKey().Verify(data, signature))
	})

	t.Run("should sign with rsa-sha2-256 when the handshake asks for it", func(t *testing.T) {
		t.Parallel()

		// given -- most servers now refuse the SHA-1 ssh-rsa signature, so an RSA key has to
		// be able to sign with a SHA-2 algorithm through the agent
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		listSigners := agentSignerCallback(t, serveSSHAgent(t, rsaKey))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		algorithmSigner, isAlgorithmSigner := signers[0].(ssh.AlgorithmSigner)
		require.True(t, isAlgorithmSigner, "x/crypto negotiates SHA-2 only for an ssh.AlgorithmSigner")
		data := []byte("session identifier the server asks to be signed")

		// when
		signature, err := algorithmSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA256)

		// then
		require.NoError(t, err)
		assert.Equal(t, ssh.KeyAlgoRSASHA256, signature.Format)
		require.NoError(t, signers[0].PublicKey().Verify(data, signature))
	})

	t.Run("should refuse an algorithm the agent protocol cannot ask for", func(t *testing.T) {
		t.Parallel()

		// given -- DSA has no agent signature flag, and is not the Ed25519 key's own algorithm
		const unsupportedAlgorithm = "ssh-dss"
		listSigners := agentSignerCallback(t, serveSSHAgent(t, newEd25519Key(t)))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		algorithmSigner, isAlgorithmSigner := signers[0].(ssh.AlgorithmSigner)
		require.True(t, isAlgorithmSigner)

		// when
		_, err = algorithmSigner.SignWithAlgorithm(rand.Reader, []byte("data"), unsupportedAlgorithm)

		// then
		require.ErrorContains(t, err, unsupportedAlgorithm)
	})
}

func TestSSHAgentListingRetries(t *testing.T) {
	t.Parallel()

	t.Run("should list the keys again when the relay dropped the connection", func(t *testing.T) {
		t.Parallel()

		// given -- connection 1 is the liveness probe; the relay then drops connection 2
		keyring := agent.NewKeyring()
		require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: newEd25519Key(t)}))
		scripted := serveScriptedSSHAgent(t, func(number int, conn net.Conn) {
			if number == 2 {
				return
			}
			_ = agent.ServeAgent(keyring, conn)
		})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		signers, err := listSigners()

		// then
		require.NoError(t, err)
		assert.Len(t, signers, 1)
		assert.Equal(t, 3, scripted.connectionCount(), "the probe, the dropped listing and the retry")
	})

	t.Run("should not list again when the agent refused", func(t *testing.T) {
		t.Parallel()

		// given -- a refusal is the agent's answer, and asking again would ask the operator again
		scripted := serveScriptedSSHAgent(t, func(_ int, conn net.Conn) {
			_ = agent.ServeAgent(refusingSSHAgent{Agent: agent.NewKeyring()}, conn)
		})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		_, err := listSigners()

		// then
		require.Error(t, err)
		assert.Equal(t, 2, scripted.connectionCount(), "the probe and one listing")
	})

	t.Run("should give up when the relay keeps dropping the connection", func(t *testing.T) {
		t.Parallel()

		// given
		scripted := serveScriptedSSHAgent(t, func(int, net.Conn) {})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		_, err := listSigners()

		// then
		require.ErrorContains(t, err, "cannot list the SSH agent's keys")
		assert.Equal(t, 4, scripted.connectionCount(), "the probe and three listings")
	})

	t.Run("should list again when the relay dropped the connection before the request was written", func(t *testing.T) {
		t.Parallel()

		// given -- every connection is closed on the agent's side before the client writes
		var dials atomic.Int32
		dial := func() (net.Conn, error) {
			dials.Add(1)
			client, agentSide := net.Pipe()
			_ = agentSide.Close()
			return client, nil
		}

		// when
		_, err := commands.ListSSHAgentSigners(dial)

		// then -- a request that never reached the agent cannot have asked anyone anything
		require.Error(t, err)
		assert.Equal(t, int32(3), dials.Load(), "three listings")
	})

	t.Run("should list again when the relay dropped the connection before the answer", func(t *testing.T) {
		t.Parallel()

		// given -- the agent's side reads the request and closes without answering
		var dials atomic.Int32
		dial := func() (net.Conn, error) {
			dials.Add(1)
			client, agentSide := net.Pipe()
			go func() {
				_, _ = agentSide.Read(make([]byte, 64))
				_ = agentSide.Close()
			}()
			return client, nil
		}

		// when
		_, err := commands.ListSSHAgentSigners(dial)

		// then
		require.Error(t, err)
		assert.Equal(t, int32(3), dials.Load(), "three listings")
	})
}

// agentSignerCallback returns the signer callback of the agent auth method AutoBump builds
// for socketPath.
func agentSignerCallback(t *testing.T, socketPath string) func() ([]ssh.Signer, error) {
	t.Helper()

	method := commands.SSHAgentAuthFromSocket(socketPath)
	agentAuth, isAgentAuth := method.(*gitssh.PublicKeysCallback)
	require.True(t, isAgentAuth, "the agent auth method must be a PublicKeysCallback")
	return agentAuth.Callback
}
