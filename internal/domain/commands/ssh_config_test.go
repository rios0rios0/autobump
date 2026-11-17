package commands_test

import (
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestResolveSSHHost(t *testing.T) {
	t.Parallel()

	t.Run("should resolve an alias from a Windows profile's file the way ssh.exe reads it", func(t *testing.T) {
		t.Parallel()

		// given -- the alias layout that keeps one key per account for the same forge
		root := t.TempDir()
		profile := filepath.Join(root, "c", "Users", "Someone")
		config := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n"+
			"  IdentityFile ~/.ssh/laptop@mine.ssh.pub\n"+
			"  IdentitiesOnly yes\n")
		sources := []commands.SSHConfigSource{{Path: config, Home: profile, Windows: true}}

		// when
		settings := commands.ResolveSSHHost(sources, root, "github.com-mine", "git")

		// then -- ~ is the Windows profile, not the WSL home
		assert.Equal(t, commands.SSHHostSettings{
			HostName:       "ssh.github.com",
			Port:           443,
			IdentityFiles:  []string{filepath.Join(profile, ".ssh", "laptop@mine.ssh.pub")},
			IdentitiesOnly: true,
		}, settings)
	})

	t.Run("should take each keyword from the first file that sets it and accumulate IdentityFile", func(t *testing.T) {
		t.Parallel()

		// given -- the WSL home's file is read before the Windows profile's
		root := t.TempDir()
		linuxHome := filepath.Join(root, "home")
		profile := filepath.Join(root, "c", "Users", "Someone")
		linuxConfig := writeSSHFixture(t, filepath.Join(linuxHome, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  Port 2222\n"+
			"  IdentityFile ~/.ssh/first.pub\n")
		windowsConfig := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n"+
			"  IdentityFile ~/.ssh/second.pub\n")
		sources := []commands.SSHConfigSource{
			{Path: linuxConfig, Home: linuxHome},
			{Path: windowsConfig, Home: profile, Windows: true},
		}

		// when
		settings := commands.ResolveSSHHost(sources, root, "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName, "only the Windows file sets HostName")
		assert.Equal(t, 2222, settings.Port, "the first file that sets Port wins")
		assert.Equal(t, []string{
			filepath.Join(linuxHome, ".ssh", "first.pub"),
			filepath.Join(profile, ".ssh", "second.pub"),
		}, settings.IdentityFiles, "IdentityFile accumulates, each path against its own file's home")
		assert.False(t, settings.IdentitiesOnly)
	})

	t.Run("should map a Windows drive path into the WSL automount root", func(t *testing.T) {
		t.Parallel()

		// given
		root := t.TempDir()
		profile := filepath.Join(root, "c", "Users", "Someone")
		config := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host dev.azure.com-work\n"+
			"  IdentityFile C:\\Users\\Someone\\.ssh\\work.pub\n"+
			"  IdentityFile ~\\.ssh\\backup.pub\n")
		sources := []commands.SSHConfigSource{{Path: config, Home: profile, Windows: true}}

		// when
		settings := commands.ResolveSSHHost(sources, root, "dev.azure.com-work", "git")

		// then
		assert.Equal(t, []string{
			filepath.Join(root, "c", "Users", "Someone", ".ssh", "work.pub"),
			filepath.Join(profile, ".ssh", "backup.pub"),
		}, settings.IdentityFiles)
	})

	t.Run("should expand the tokens OpenSSH documents for HostName and IdentityFile", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host *.internal\n"+
			"  HostName %h.example.com\n"+
			"  IdentityFile %d/.ssh/%h-%r-100%%.pub\n"+
			"  IdentityFile ~/.ssh/%k.pub\n")
		sources := []commands.SSHConfigSource{{Path: config, Home: home}}

		// when
		settings := commands.ResolveSSHHost(sources, "", "git.internal", "git")

		// then -- %h in IdentityFile is the HostName, and an unknown token is left as written
		assert.Equal(t, "git.internal.example.com", settings.HostName)
		assert.Equal(t, []string{
			filepath.Join(home, ".ssh", "git.internal.example.com-git-100%.pub"),
			filepath.Join(home, ".ssh", "%k.pub"),
		}, settings.IdentityFiles)
	})

	t.Run("should say nothing about a host no file names", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n")
		sources := []commands.SSHConfigSource{{Path: config, Home: home}}

		// when
		settings := commands.ResolveSSHHost(sources, "", "gitlab.com", "git")

		// then
		assert.Equal(t, commands.SSHHostSettings{}, settings)
	})

	t.Run("should skip a file it cannot parse and keep the others", func(t *testing.T) {
		t.Parallel()

		// given -- kevinburke/ssh_config refuses Match exec, so the whole first file is lost
		home := t.TempDir()
		unparseable := writeSSHFixture(t, filepath.Join(home, "unparseable"), ""+
			"Match exec \"true\"\n"+
			"  HostName wrong.example.com\n")
		valid := writeSSHFixture(t, filepath.Join(home, "valid"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n")
		sources := []commands.SSHConfigSource{{Path: unparseable, Home: home}, {Path: valid, Home: home}}

		// when
		settings := commands.ResolveSSHHost(sources, "", "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName)
	})

	t.Run("should read a CRLF file and ignore a file that does not exist", func(t *testing.T) {
		t.Parallel()

		// given -- a Windows editor writes CRLF line endings
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"),
			"Host github.com-mine\r\n  HostName ssh.github.com\r\n  Port 443\r\n")
		sources := []commands.SSHConfigSource{
			{Path: filepath.Join(home, "absent"), Home: home},
			{Path: config, Home: home},
		}

		// when
		settings := commands.ResolveSSHHost(sources, "", "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName)
		assert.Equal(t, 443, settings.Port)
	})
}

func TestSSHHostTarget(t *testing.T) {
	t.Parallel()

	t.Run("should dial the HostName and Port the alias names", func(t *testing.T) {
		t.Parallel()

		// given -- go-git parses an scp-like remote to port 22, which the URL cannot carry
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)
		settings := commands.SSHHostSettings{HostName: "ssh.github.com", Port: 443}

		// when
		target := commands.SSHHostTarget(settings, endpoint)

		// then
		assert.Equal(t, "ssh.github.com", target.Host)
		assert.Equal(t, 443, target.Port)
		assert.Equal(t, endpoint.Path, target.Path)
		assert.Equal(t, "github.com-mine", endpoint.Host, "the remote's own endpoint is left alone")
	})

	t.Run("should keep a port the URL names over the configured Port", func(t *testing.T) {
		t.Parallel()

		// given -- as `ssh -p` wins over the file
		endpoint, err := transport.NewEndpoint("ssh://git@forge-alias:2222/owner/repo.git")
		require.NoError(t, err)
		settings := commands.SSHHostSettings{HostName: "forge.example.com", Port: 443}

		// when
		target := commands.SSHHostTarget(settings, endpoint)

		// then
		assert.Equal(t, "forge.example.com", target.Host)
		assert.Equal(t, 2222, target.Port)
	})

	t.Run("should return the endpoint itself when the alias names neither", func(t *testing.T) {
		t.Parallel()

		// given
		endpoint, err := transport.NewEndpoint("git@github.com:owner/repo.git")
		require.NoError(t, err)

		// when
		target := commands.SSHHostTarget(commands.SSHHostSettings{}, endpoint)

		// then
		assert.Same(t, endpoint, target)
	})
}

func TestWindowsSSHConfigSourcesUnder(t *testing.T) {
	t.Parallel()

	t.Run("should find every mounted profile's ssh_config with the profile as its home", func(t *testing.T) {
		t.Parallel()

		// given
		root := t.TempDir()
		first := writeSSHFixture(t, filepath.Join(root, "c", "Users", "First", ".ssh", "config"), "")
		second := writeSSHFixture(t, filepath.Join(root, "d", "Users", "Second", ".ssh", "config"), "")
		writeSSHFixture(t, filepath.Join(root, "wsl", "Users", "NotADrive", ".ssh", "config"), "")

		// when
		sources := commands.WindowsSSHConfigSourcesUnder(root)

		// then
		assert.ElementsMatch(t, []commands.SSHConfigSource{
			{Path: first, Home: filepath.Join(root, "c", "Users", "First"), Windows: true},
			{Path: second, Home: filepath.Join(root, "d", "Users", "Second"), Windows: true},
		}, sources)
	})
}
