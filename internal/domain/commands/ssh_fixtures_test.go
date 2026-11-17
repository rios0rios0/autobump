package commands_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
)

// newEd25519Key returns a fresh Ed25519 private key, a shape the agent keyring, the test
// server's host keys and ssh.NewSignerFromKey all accept.
func newEd25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return key
}

// newECDSAKey returns a fresh ECDSA P-256 private key: the host key type x/crypto asks a
// server for ahead of Ed25519.
func newECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

// newTestSigner wraps a private key as an ssh.Signer.
func newTestSigner(t *testing.T, key any) ssh.Signer {
	t.Helper()

	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	return signer
}

// writeSSHFixture writes content to path, creating its directory with makeDir.
func writeSSHFixture(t *testing.T, path, content string) string {
	t.Helper()

	makeDir(t, filepath.Dir(path))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// writePublicKeyFixture writes publicKey to path in the authorized_keys form ssh-keygen and
// 1Password write a .pub file in.
func writePublicKeyFixture(t *testing.T, path string, publicKey ssh.PublicKey) string {
	t.Helper()

	return writeSSHFixture(t, path, string(ssh.MarshalAuthorizedKey(publicKey)))
}

// serveSSHAgent serves an in-memory SSH agent holding keys, in the order given, on a Unix
// socket for the lifetime of the test, and returns the socket's path. Every connection is
// served on its own, the way a real agent -- or the npiperelay bridge in front of one --
// serves them.
func serveSSHAgent(t *testing.T, keys ...any) string {
	t.Helper()

	keyring := agent.NewKeyring()
	for _, key := range keys {
		require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: key}))
	}

	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()

	return socketPath
}

// scriptedSSHAgent is an SSH agent socket whose every connection is handled by a script, so
// a test can drop chosen connections the way a relay does, or have the agent refuse.
type scriptedSSHAgent struct {
	socketPath string

	mu          sync.Mutex
	connections int
}

// serveScriptedSSHAgent serves handle, called with each connection's 1-based number, on a
// Unix socket for the lifetime of the test.
func serveScriptedSSHAgent(t *testing.T, handle func(number int, conn net.Conn)) *scriptedSSHAgent {
	t.Helper()

	scripted := &scriptedSSHAgent{socketPath: filepath.Join(t.TempDir(), "agent.sock")}
	listener, err := net.Listen("unix", scripted.socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	// Connections are numbered as they are accepted, in the order they were made, not when
	// their handler gets to run: the handlers race each other.
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			number := scripted.accept()
			go func() {
				defer conn.Close()
				handle(number, conn)
			}()
		}
	}()

	return scripted
}

// connectionCount returns how many connections the agent accepted.
func (s *scriptedSSHAgent) connectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.connections
}

// accept counts an accepted connection and returns its number.
func (s *scriptedSSHAgent) accept() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections++
	return s.connections
}

// refusingSSHAgent is an agent that answers every request to list its keys with a refusal.
type refusingSSHAgent struct {
	agent.Agent
}

// List refuses to list the agent's keys.
func (refusingSSHAgent) List() ([]*agent.Key, error) {
	return nil, errors.New("listing refused")
}

// gitSSHServer is a loopback SSH server speaking just enough of git's protocol for a
// session to start: it accepts one authorized key, answers every exec request with an
// empty reference advertisement, and records each public key a client offers.
type gitSSHServer struct {
	addr *net.TCPAddr

	mu      sync.Mutex
	offered []string
}

// startGitSSHServer starts a gitSSHServer presenting hostKeys for the lifetime of the test.
func startGitSSHServer(t *testing.T, authorized ssh.PublicKey, hostKeys ...ssh.Signer) *gitSSHServer {
	t.Helper()

	server := &gitSSHServer{}
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			server.recordOffered(key)
			if bytes.Equal(key.Marshal(), authorized.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("key not authorized")
		},
	}
	for _, hostKey := range hostKeys {
		config.AddHostKey(hostKey)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	addr, isTCP := listener.Addr().(*net.TCPAddr)
	require.True(t, isTCP)
	server.addr = addr

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveGitSSHConnection(conn, config)
		}
	}()

	return server
}

// knownHostsLine returns the known_hosts line recording hostKey for the server, under the
// [host]:port name OpenSSH gives a host on a port other than 22.
func (s *gitSSHServer) knownHostsLine(hostKey ssh.PublicKey) string {
	return xknownhosts.Line([]string{s.addr.String()}, hostKey) + "\n"
}

// port returns the port the server listens on.
func (s *gitSSHServer) port() string {
	return strconv.Itoa(s.addr.Port)
}

// offeredKeys returns the SHA256 fingerprints of the keys clients offered, in the order
// they were first offered.
func (s *gitSSHServer) offeredKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.offered)
}

// recordOffered records a key a client offered, once.
func (s *gitSSHServer) recordOffered(key ssh.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fingerprint := ssh.FingerprintSHA256(key)
	if !slices.Contains(s.offered, fingerprint) {
		s.offered = append(s.offered, fingerprint)
	}
}

// serveGitSSHConnection runs the SSH handshake on conn and serves its session channels.
func serveGitSSHConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	_, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(requests)

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only sessions are served")
			continue
		}
		channel, channelRequests, acceptErr := newChannel.Accept()
		if acceptErr != nil {
			continue
		}
		go serveGitSession(channel, channelRequests)
	}
}

// serveGitSession accepts the exec request git-upload-pack and git-receive-pack arrive as,
// refusing every other request, until the client closes the session.
func serveGitSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for request := range requests {
		if request.Type != "exec" {
			if request.WantReply {
				_ = request.Reply(false, nil)
			}
			continue
		}

		_ = request.Reply(true, nil)
		go answerWithEmptyAdvertisement(channel)
	}
}

// answerWithEmptyAdvertisement sends the reference advertisement of an empty repository --
// a lone flush packet -- and exits 0 once the client is done. go-git answers an empty
// advertisement with a flush packet of its own, so waiting for the client to close first
// keeps the server from closing the channel under that write.
func answerWithEmptyAdvertisement(channel ssh.Channel) {
	_, _ = channel.Write([]byte("0000"))
	_, _ = io.Copy(io.Discard, channel)
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	_ = channel.Close()
}
