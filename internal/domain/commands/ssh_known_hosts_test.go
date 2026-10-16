package commands_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestParseWSLAutomountRoot(t *testing.T) {
	t.Parallel()

	t.Run("should return the WSL defaults for empty content", func(t *testing.T) {
		t.Parallel()

		// given / when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(""))

		// then
		assert.Equal(t, "/mnt/", root)
		assert.True(t, enabled)
	})

	t.Run("should read a custom root from the automount section", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "[automount]\nroot = /windows/\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/windows/", root)
		assert.True(t, enabled)
	})

	t.Run("should honour enabled = false", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "[automount]\nenabled = false\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.False(t, enabled)
		assert.Equal(t, "/mnt/", root)
	})

	t.Run("should ignore keys outside the automount section", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "[boot]\nsystemd = true\nroot = /elsewhere/\n[automount]\nroot = /windows/\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/windows/", root)
		assert.True(t, enabled)
	})

	t.Run("should skip comments and blank lines", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "# a comment\n; another one\n\n[automount]\n# root = /nope/\nroot=/yes/\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/yes/", root)
		assert.True(t, enabled)
	})

	t.Run("should strip quotes around values", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "[automount]\nroot = \"/wsl/mnt/\"\n"

		// when
		root, _ := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/wsl/mnt/", root)
	})

	t.Run("should match the section name case-insensitively", func(t *testing.T) {
		t.Parallel()

		// given
		conf := "[Automount]\nROOT = /upper/\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/upper/", root)
		assert.True(t, enabled)
	})

	t.Run("should keep a root value carrying Windows line endings", func(t *testing.T) {
		t.Parallel()

		// given -- wsl.conf edited from the Windows side
		conf := "[automount]\r\nroot = /windows/\r\n"

		// when
		root, enabled := commands.ParseWSLAutomountRoot([]byte(conf))

		// then
		assert.Equal(t, "/windows/", root)
		assert.True(t, enabled)
	})
}

func TestWindowsKnownHostsFilesUnder(t *testing.T) {
	t.Parallel()

	t.Run("should find the known_hosts of every mounted profile", func(t *testing.T) {
		t.Parallel()

		// given
		root := t.TempDir()
		alice := filepath.Join(root, "c", "Users", "alice", ".ssh", "known_hosts")
		carol := filepath.Join(root, "d", "Users", "carol", ".ssh", "known_hosts")
		// Fixture directories need the owner search bit to be enterable, so 0o700 is
		// the least-privilege mode a directory can take; the rule compares against
		// 0600 regardless, as does the one on the 0o755 profile directory below.
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Dir(alice), 0o700))
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Dir(carol), 0o700))
		require.NoError(t, os.WriteFile(alice, []byte("github.com ssh-ed25519 AAAA\n"), 0o600))
		require.NoError(t, os.WriteFile(carol, []byte("github.com ssh-ed25519 AAAA\n"), 0o600))
		// A profile without .ssh, a non-drive directory, and a stray file at the root
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(root, "c", "Users", "bob"), 0o755))
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(root, "wsl", "Users", "dave", ".ssh"), 0o700))
		require.NoError(
			t,
			os.WriteFile(filepath.Join(root, "wsl", "Users", "dave", ".ssh", "known_hosts"), []byte("x\n"), 0o600),
		)

		// when
		files := commands.WindowsKnownHostsFilesUnder(root)

		// then
		assert.ElementsMatch(t, []string{alice, carol}, files)
	})

	t.Run("should return nothing when the root does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		root := filepath.Join(t.TempDir(), "missing")

		// when / then
		assert.Empty(t, commands.WindowsKnownHostsFilesUnder(root))
	})
}

// TestDiscoverKnownHostsFiles is deliberately not parallel: it mutates the process
// environment (SSH_KNOWN_HOSTS, HOME).
func TestDiscoverKnownHostsFiles(t *testing.T) {
	t.Run("should let SSH_KNOWN_HOSTS replace discovery when it names existing files", func(t *testing.T) {
		// given
		home := t.TempDir()
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte("a\n"), 0o600))
		explicit := filepath.Join(t.TempDir(), "explicit_known_hosts")
		require.NoError(t, os.WriteFile(explicit, []byte("b\n"), 0o600))
		t.Setenv("HOME", home)
		t.Setenv("SSH_KNOWN_HOSTS", explicit)

		// when
		files := commands.DiscoverKnownHostsFiles()

		// then -- only the explicit file: SSH_KNOWN_HOSTS wins outright, as in go-git
		assert.Equal(t, []string{explicit}, files)
	})

	t.Run("should discover nothing when SSH_KNOWN_HOSTS only names missing files", func(t *testing.T) {
		// given
		t.Setenv("SSH_KNOWN_HOSTS", filepath.Join(t.TempDir(), "absent"))

		// when / then
		assert.Empty(t, commands.DiscoverKnownHostsFiles())
	})

	t.Run("should include the home known_hosts when the env variable is unset", func(t *testing.T) {
		// given
		home := t.TempDir()
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
		homeFile := filepath.Join(home, ".ssh", "known_hosts")
		require.NoError(t, os.WriteFile(homeFile, []byte("a\n"), 0o600))
		t.Setenv("HOME", home)
		t.Setenv("SSH_KNOWN_HOSTS", "")

		// when
		files := commands.DiscoverKnownHostsFiles()

		// then -- the home file is in the result; on a WSL host the Windows profiles
		// and the system file may join it, which is exactly what the merge promises
		assert.Contains(t, files, homeFile)
	})
}

func TestBuildHostKeyCallback(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when no file was discovered", func(t *testing.T) {
		t.Parallel()

		// given / when / then
		assert.Nil(t, commands.BuildHostKeyCallback(nil))
	})

	t.Run("should verify a host key listed in a CRLF file", func(t *testing.T) {
		t.Parallel()

		// given -- a Windows-side known_hosts: CRLF endings, written by ssh.exe
		trusted, wrongKey := ed25519GenerateSSHKey(t), ed25519GenerateSSHKey(t)
		line := xknownhosts.Line([]string{"github.com:22"}, trusted)
		path := filepath.Join(t.TempDir(), "known_hosts")
		require.NoError(t, os.WriteFile(path, []byte(line+"\r\n"), 0o600))

		// when
		cb := commands.BuildHostKeyCallback([]string{path})

		// then -- go-git always hands the callback a host:port address (getHostWithPort)
		require.NotNil(t, cb)
		addr := &net.TCPAddr{IP: net.ParseIP("140.82.112.3"), Port: 22}
		require.NoError(t, cb("github.com:22", addr, trusted), "the trusted key must verify")
		require.Error(t, cb("github.com:22", addr, wrongKey), "an untrusted key must be rejected")
	})

	t.Run("should return nil when the file cannot be parsed", func(t *testing.T) {
		t.Parallel()

		// given -- a directory cannot be parsed as known_hosts
		path := t.TempDir()

		// when / then
		assert.Nil(t, commands.BuildHostKeyCallback([]string{path}))
	})
}

func ed25519GenerateSSHKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return sshPub
}
