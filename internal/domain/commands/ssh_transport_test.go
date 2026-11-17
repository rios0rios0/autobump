package commands_test

import (
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

// errSessionRecorded ends a session the recordingTransport only records.
var errSessionRecorded = errors.New("session recorded")

// recordingTransport is a go-git transport that records the endpoint and auth method a
// session was asked for instead of opening one.
type recordingTransport struct {
	endpoint *transport.Endpoint
	auth     transport.AuthMethod
}

func (r *recordingTransport) NewUploadPackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.UploadPackSession, error) {
	r.endpoint, r.auth = endpoint, auth
	return nil, errSessionRecorded
}

func (r *recordingTransport) NewReceivePackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.ReceivePackSession, error) {
	r.endpoint, r.auth = endpoint, auth
	return nil, errSessionRecorded
}

func TestSSHTransport(t *testing.T) {
	t.Parallel()

	t.Run("should push to the HostName and Port the remote's alias names", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n")
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(
			next,
			[]commands.SSHConfigSource{{Path: config, Home: home}},
			"",
			nil,
		)
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)

		// when
		_, err = sshTransport.NewReceivePackSession(endpoint, nil)

		// then
		require.ErrorIs(t, err, errSessionRecorded)
		assert.Equal(t, "ssh.github.com", next.endpoint.Host)
		assert.Equal(t, 443, next.endpoint.Port)
		assert.Equal(t, endpoint.Path, next.endpoint.Path)
		assert.Nil(t, next.auth, "a missing auth method is left to go-git's defaults")
	})

	t.Run("should ask for the host key types known_hosts records for the resolved host", func(t *testing.T) {
		t.Parallel()

		// given -- known_hosts names the HostName, as OpenSSH records it, not the alias
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n")
		hostKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		knownHosts := writeSSHFixture(t, filepath.Join(home, ".ssh", "known_hosts"),
			xknownhosts.Line([]string{"ssh.github.com:443"}, hostKey)+"\n")
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(
			next, []commands.SSHConfigSource{{Path: config, Home: home}}, "", []string{knownHosts},
		)
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)
		auth := &gitssh.PublicKeysCallback{
			User:     "git",
			Callback: func() ([]ssh.Signer, error) { return nil, nil },
		}
		auth.HostKeyCallback = ssh.FixedHostKey(hostKey)

		// when
		_, err = sshTransport.NewUploadPackSession(endpoint, auth)

		// then
		require.ErrorIs(t, err, errSessionRecorded)
		pinned, isAgentAuth := next.auth.(*gitssh.PublicKeysCallback)
		require.True(t, isAgentAuth)
		assert.Equal(t, []string{ssh.KeyAlgoED25519}, pinned.HostKeyAlgorithms)
		assert.Empty(t, auth.HostKeyAlgorithms, "the caller's auth method is left alone")
	})

	t.Run("should leave the host key types to go-git when the method brings no host key callback", func(t *testing.T) {
		t.Parallel()

		// given -- go-git then reads its own known_hosts, which AutoBump's types would not match
		home := t.TempDir()
		hostKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		knownHosts := writeSSHFixture(t, filepath.Join(home, "known_hosts"),
			xknownhosts.Line([]string{"github.com:22"}, hostKey)+"\n")
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(next, nil, "", []string{knownHosts})
		endpoint, err := transport.NewEndpoint("git@github.com:owner/repo.git")
		require.NoError(t, err)
		auth := &gitssh.PublicKeysCallback{User: "git", Callback: func() ([]ssh.Signer, error) { return nil, nil }}

		// when
		_, err = sshTransport.NewUploadPackSession(endpoint, auth)

		// then
		require.ErrorIs(t, err, errSessionRecorded)
		assert.Same(t, auth, next.auth)
		assert.Empty(t, auth.HostKeyAlgorithms)
	})

	t.Run(
		"should offer an agent only the keys the alias's IdentityFile names when IdentitiesOnly is set",
		func(t *testing.T) {
			t.Parallel()

			// given -- the agent lists the alias's key last, behind keys of other accounts
			home := t.TempDir()
			agentKeys := newAgentSigners(t, 3)
			writePublicKeyFixture(t, filepath.Join(home, ".ssh", "mine.pub"), agentKeys[2].PublicKey())
			config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
				"Host github.com-mine\n"+
				"  HostName ssh.github.com\n"+
				"  IdentityFile ~/.ssh/mine.pub\n"+
				"  IdentitiesOnly yes\n")
			next := &recordingTransport{}
			sshTransport := commands.NewSSHTransportWith(
				next,
				[]commands.SSHConfigSource{{Path: config, Home: home}},
				"",
				nil,
			)
			endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
			require.NoError(t, err)
			auth := &gitssh.PublicKeysCallback{
				User:     "git",
				Callback: func() ([]ssh.Signer, error) { return agentKeys, nil },
			}

			// when
			_, err = sshTransport.NewReceivePackSession(endpoint, auth)

			// then
			require.ErrorIs(t, err, errSessionRecorded)
			narrowed, isAgentAuth := next.auth.(*gitssh.PublicKeysCallback)
			require.True(t, isAgentAuth)
			offered, err := narrowed.Callback()
			require.NoError(t, err)
			assert.Equal(t, []ssh.Signer{agentKeys[2]}, offered)
		},
	)

	t.Run("should leave a key file configured as ssh_key_path as it is", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		writePublicKeyFixture(
			t,
			filepath.Join(home, ".ssh", "other.pub"),
			newTestSigner(t, newEd25519Key(t)).PublicKey(),
		)
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  IdentityFile ~/.ssh/other.pub\n"+
			"  IdentitiesOnly yes\n")
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(
			next,
			[]commands.SSHConfigSource{{Path: config, Home: home}},
			"",
			nil,
		)
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)
		auth := &gitssh.PublicKeys{User: "git", Signer: newTestSigner(t, newEd25519Key(t))}

		// when
		_, err = sshTransport.NewReceivePackSession(endpoint, auth)

		// then
		require.ErrorIs(t, err, errSessionRecorded)
		assert.Same(t, auth, next.auth)
	})
}

func TestSSHTransportEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should offer no key when IdentitiesOnly names keys the agent does not hold", func(t *testing.T) {
		t.Parallel()

		// given -- OpenSSH offers nothing in this case either; the transport warns once why
		home := t.TempDir()
		stranger := newTestSigner(t, newEd25519Key(t)).PublicKey()
		writePublicKeyFixture(t, filepath.Join(home, ".ssh", "stranger.pub"), stranger)
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  IdentityFile ~/.ssh/stranger.pub\n"+
			"  IdentitiesOnly yes\n")
		agentKeys := newAgentSigners(t, 2)
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(
			next,
			[]commands.SSHConfigSource{{Path: config, Home: home}},
			"",
			nil,
		)
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)
		auth := &gitssh.PublicKeysCallback{
			User:     "git",
			Callback: func() ([]ssh.Signer, error) { return agentKeys, nil },
		}
		_, err = sshTransport.NewReceivePackSession(endpoint, auth)
		require.ErrorIs(t, err, errSessionRecorded)
		narrowed, isAgentAuth := next.auth.(*gitssh.PublicKeysCallback)
		require.True(t, isAgentAuth)

		// when -- a release connects more than once
		first, firstErr := narrowed.Callback()
		second, secondErr := narrowed.Callback()

		// then
		require.NoError(t, firstErr)
		require.NoError(t, secondErr)
		assert.Empty(t, first)
		assert.Empty(t, second)
	})

	t.Run("should ask for the recorded host key types for a key file configured as ssh_key_path", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		hostKey := newTestSigner(t, newEd25519Key(t)).PublicKey()
		knownHosts := writeSSHFixture(t, filepath.Join(home, "known_hosts"),
			xknownhosts.Line([]string{"github.com:22"}, hostKey)+"\n")
		next := &recordingTransport{}
		sshTransport := commands.NewSSHTransportWith(next, nil, "", []string{knownHosts})
		endpoint, err := transport.NewEndpoint("git@github.com:owner/repo.git")
		require.NoError(t, err)
		auth := &gitssh.PublicKeys{User: "git", Signer: newTestSigner(t, newEd25519Key(t))}
		auth.HostKeyCallback = ssh.FixedHostKey(hostKey)

		// when
		_, err = sshTransport.NewReceivePackSession(endpoint, auth)

		// then -- the key itself is offered as configured
		require.ErrorIs(t, err, errSessionRecorded)
		pinned, isKeyAuth := next.auth.(*gitssh.PublicKeys)
		require.True(t, isKeyAuth)
		assert.Same(t, auth.Signer, pinned.Signer)
		assert.Equal(t, []string{ssh.KeyAlgoED25519}, pinned.HostKeyAlgorithms)
	})
}

// TestDiscoverSSHConfigSources is deliberately not parallel: it points HOME somewhere else
// with t.Setenv, which the runtime forbids in a parallel test.
func TestDiscoverSSHConfigSources(t *testing.T) {
	t.Run("should read the user's file before the machine-wide one", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)

		// when
		_, sources := commands.DiscoverSSHConfigSources()

		// then -- under WSL the Windows profiles' files follow these two
		require.GreaterOrEqual(t, len(sources), 2)
		assert.Equal(t, commands.SSHConfigSource{Path: filepath.Join(home, ".ssh", "config"), Home: home}, sources[0])
		assert.Equal(t, commands.SSHConfigSource{Path: "/etc/ssh/ssh_config", Home: home}, sources[1])
		for _, source := range sources[2:] {
			assert.True(t, source.Windows, "only Windows profiles' files come after the Linux ones")
		}
	})
}

func TestSSHTransportAgainstAServer(t *testing.T) {
	t.Parallel()

	t.Run("should reach the git protocol through the alias, offering only the alias's key", func(t *testing.T) {
		t.Parallel()

		// given -- the agent lists the alias's key last, behind two keys the forge rejects
		forge := newForgeFixture(t)
		home := t.TempDir()
		writePublicKeyFixture(t, filepath.Join(home, ".ssh", "chosen.pub"), forge.authorizedKey)
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host forge-alias\n"+
			"  HostName 127.0.0.1\n"+
			"  Port "+forge.server.port()+"\n"+
			"  IdentityFile ~/.ssh/chosen.pub\n"+
			"  IdentitiesOnly yes\n")
		socket := serveSSHAgent(t, newEd25519Key(t), newEd25519Key(t), forge.authorizedPrivateKey)
		auth := agentAuthVerifyingAgainst(t, socket, forge.knownHosts)
		sshTransport := commands.NewSSHTransportWith(
			gitssh.DefaultClient,
			[]commands.SSHConfigSource{{Path: config, Home: home}},
			"",
			[]string{forge.knownHosts},
		)
		endpoint, err := transport.NewEndpoint("git@forge-alias:owner/repo.git")
		require.NoError(t, err)

		// when -- the session dials, verifies the host key and authenticates before it returns
		session, err := sshTransport.NewUploadPackSession(endpoint, auth)
		require.NoError(t, err)
		t.Cleanup(func() { _ = session.Close() })
		_, err = session.AdvertisedReferences()

		// then -- the server spoke git's protocol, so the handshake and the signature passed
		require.ErrorIs(t, err, transport.ErrEmptyRemoteRepository)
		assert.Equal(t, []string{ssh.FingerprintSHA256(forge.authorizedKey)}, forge.server.offeredKeys())
	})

	t.Run("should fail host key verification without the transport", func(t *testing.T) {
		t.Parallel()

		// given -- the same connection through go-git's bare transport
		forge := newForgeFixture(t)
		auth := agentAuthVerifyingAgainst(t, serveSSHAgent(t, forge.authorizedPrivateKey), forge.knownHosts)
		endpoint, err := transport.NewEndpoint("ssh://git@127.0.0.1:" + forge.server.port() + "/owner/repo.git")
		require.NoError(t, err)

		// when
		_, err = gitssh.DefaultClient.NewUploadPackSession(endpoint, auth)

		// then -- x/crypto asked for ECDSA first, and known_hosts records only Ed25519
		require.ErrorContains(t, err, "knownhosts: key mismatch")
	})
}

// forgeFixture is a loopback forge that presents an ECDSA and an Ed25519 host key, as
// GitHub does, with a known_hosts file recording only the Ed25519 one, as OpenSSH writes
// it, and that accepts one user key.
type forgeFixture struct {
	server               *gitSSHServer
	knownHosts           string
	authorizedKey        ssh.PublicKey
	authorizedPrivateKey ed25519.PrivateKey
}

// newForgeFixture starts a forgeFixture for the lifetime of the test.
func newForgeFixture(t *testing.T) *forgeFixture {
	t.Helper()

	ed25519HostKey := newTestSigner(t, newEd25519Key(t))
	authorizedPrivateKey := newEd25519Key(t)
	authorizedKey := newTestSigner(t, authorizedPrivateKey).PublicKey()
	server := startGitSSHServer(t, authorizedKey, newTestSigner(t, newECDSAKey(t)), ed25519HostKey)
	knownHosts := writeSSHFixture(
		t, filepath.Join(t.TempDir(), "known_hosts"), server.knownHostsLine(ed25519HostKey.PublicKey()),
	)

	return &forgeFixture{
		server:               server,
		knownHosts:           knownHosts,
		authorizedKey:        authorizedKey,
		authorizedPrivateKey: authorizedPrivateKey,
	}
}

// agentAuthVerifyingAgainst returns the agent auth method AutoBump builds for socketPath,
// verifying host keys against knownHosts instead of the host's own discovered files.
func agentAuthVerifyingAgainst(t *testing.T, socketPath, knownHosts string) *gitssh.PublicKeysCallback {
	t.Helper()

	agentAuth, isAgentAuth := commands.SSHAgentAuthFromSocket(socketPath).(*gitssh.PublicKeysCallback)
	require.True(t, isAgentAuth)
	hostKeys := commands.BuildKnownHostsDB([]string{knownHosts})
	require.NotNil(t, hostKeys)
	agentAuth.HostKeyCallback = hostKeys.HostKeyCallback()
	return agentAuth
}
