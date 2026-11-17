package commands_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestGitSSHCommandInEffect(t *testing.T) {
	t.Parallel()

	environment := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}

	t.Run("should take GIT_SSH_COMMAND over core.sshCommand and GIT_SSH", func(t *testing.T) {
		t.Parallel()

		// given
		getenv := environment(map[string]string{"GIT_SSH_COMMAND": "ssh -i key", "GIT_SSH": "plink"})

		// when
		command := commands.GitSSHCommandInEffect(getenv, "ssh.exe")

		// then
		assert.Equal(t, "ssh -i key", command)
	})

	t.Run("should take core.sshCommand over GIT_SSH", func(t *testing.T) {
		t.Parallel()

		// given
		getenv := environment(map[string]string{"GIT_SSH": "plink"})

		// when
		command := commands.GitSSHCommandInEffect(getenv, "ssh.exe")

		// then
		assert.Equal(t, "ssh.exe", command)
	})

	t.Run("should take GIT_SSH when nothing else is set", func(t *testing.T) {
		t.Parallel()

		// given
		getenv := environment(map[string]string{"GIT_SSH": "/usr/bin/ssh"})

		// when
		command := commands.GitSSHCommandInEffect(getenv, "")

		// then
		assert.Equal(t, "/usr/bin/ssh", command)
	})

	t.Run("should fall back to plain ssh from PATH", func(t *testing.T) {
		t.Parallel()

		// given / when
		command := commands.GitSSHCommandInEffect(environment(nil), "")

		// then
		assert.Equal(t, "ssh", command)
	})
}

func TestRunsWindowsSSH(t *testing.T) {
	t.Parallel()

	onPath := func(program string) func(string) (string, error) {
		return func(string) (string, error) { return program, nil }
	}
	notOnPath := func(string) (string, error) { return "", errors.New("not found") }

	t.Run("should recognise a command that names ssh.exe", func(t *testing.T) {
		t.Parallel()

		// given / when / then -- the way core.sshCommand is usually set under WSL
		assert.True(t, commands.RunsWindowsSSH("ssh.exe", notOnPath))
		assert.True(t, commands.RunsWindowsSSH(`"/mnt/c/Windows/System32/OpenSSH/SSH.EXE" -o BatchMode=yes`, notOnPath))
	})

	t.Run("should recognise an ssh wrapper script on PATH that calls ssh.exe", func(t *testing.T) {
		t.Parallel()

		// given
		wrapper := writeSSHFixture(t, filepath.Join(t.TempDir(), "ssh"), "#!/bin/bash\nssh.exe \"$@\"\n")

		// when / then
		assert.True(t, commands.RunsWindowsSSH("ssh", onPath(wrapper)))
	})

	t.Run("should recognise an ssh on PATH that links to ssh.exe", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		target := writeSSHFixture(t, filepath.Join(dir, "ssh.exe"), "MZ")
		link := filepath.Join(dir, "ssh")
		require.NoError(t, os.Symlink(target, link))

		// when / then
		assert.True(t, commands.RunsWindowsSSH("ssh", onPath(link)))
	})

	t.Run("should not count the Linux ssh", func(t *testing.T) {
		t.Parallel()

		// given -- a binary, and a wrapper that does not reach Windows
		dir := t.TempDir()
		binary := writeSSHFixture(t, filepath.Join(dir, "bin", "ssh"), "\x7fELF"+strings.Repeat("\x00", 64))
		wrapper := writeSSHFixture(t, filepath.Join(dir, "wrap", "ssh"), "#!/bin/sh\nexec /usr/bin/ssh \"$@\"\n")

		// when / then
		assert.False(t, commands.RunsWindowsSSH("ssh", onPath(binary)))
		assert.False(t, commands.RunsWindowsSSH("ssh -o BatchMode=yes", onPath(wrapper)))
	})

	t.Run("should not count a program that is not on PATH", func(t *testing.T) {
		t.Parallel()

		// given / when / then
		assert.False(t, commands.RunsWindowsSSH("ssh", notOnPath))
	})
}

func TestWindowsProfilesOnPath(t *testing.T) {
	t.Parallel()

	t.Run("should find the profile the Windows PATH entries sit under", func(t *testing.T) {
		t.Parallel()

		// given -- the per-user entries WSL appends from the Windows PATH
		pathList := strings.Join([]string{
			"/usr/local/bin",
			"/mnt/c/WINDOWS/system32",
			"/mnt/c/Users/Someone/AppData/Local/Microsoft/WindowsApps",
			"/mnt/c/Users/Someone/.dotnet/tools",
		}, ":")

		// when
		profiles := commands.WindowsProfilesOnPath("/mnt/", pathList)

		// then
		assert.Equal(t, []string{"/mnt/c/Users/Someone"}, profiles)
	})

	t.Run("should honour an automount root written without a trailing slash", func(t *testing.T) {
		t.Parallel()

		// given / when
		profiles := commands.WindowsProfilesOnPath("/windir", "/windir/d/Users/Other/bin:/windir/wsl/Users/x")

		// then
		assert.Equal(t, []string{"/windir/d/Users/Other"}, profiles)
	})

	t.Run("should find nothing when PATH holds no Windows entry", func(t *testing.T) {
		t.Parallel()

		// given / when -- appendWindowsPath turned off in /etc/wsl.conf
		profiles := commands.WindowsProfilesOnPath("/mnt/", "/usr/local/bin:/usr/bin:/bin")

		// then
		assert.Empty(t, profiles)
	})
}

func TestCurrentWindowsProfileSources(t *testing.T) {
	t.Parallel()

	sources := []commands.SSHConfigSource{
		{Path: "/mnt/c/Users/Other/.ssh/config", Home: "/mnt/c/Users/Other", Windows: true},
		{Path: "/mnt/c/Users/Someone/.ssh/config", Home: "/mnt/c/Users/Someone", Windows: true},
	}

	t.Run("should keep only the profile PATH reaches into", func(t *testing.T) {
		t.Parallel()

		// given / when -- PATH may spell the profile in another case than the directory
		kept := commands.CurrentWindowsProfileSources(sources, []string{"/mnt/c/users/someone"})

		// then
		assert.Equal(t, sources[1:], kept)
	})

	t.Run("should keep every profile when PATH reaches into none", func(t *testing.T) {
		t.Parallel()

		// given / when
		kept := commands.CurrentWindowsProfileSources(sources, nil)

		// then
		assert.Equal(t, sources, kept)
	})
}
