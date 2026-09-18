package commands_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestIsSSHRemote(t *testing.T) {
	t.Parallel()

	t.Run("should report SSH when the remote uses scp-like syntax", func(t *testing.T) {
		t.Parallel()

		// given
		remote := "git@github.com:rios0rios0/autobump.git"

		// when
		result := commands.IsSSHRemote(remote)

		// then
		assert.True(t, result)
	})

	t.Run("should report SSH when the remote uses the ssh scheme", func(t *testing.T) {
		t.Parallel()

		// given
		remote := "ssh://git@ssh.dev.azure.com/v3/org/project/repo"

		// when
		result := commands.IsSSHRemote(remote)

		// then
		assert.True(t, result)
	})

	t.Run("should not report SSH when the remote is HTTPS", func(t *testing.T) {
		t.Parallel()

		// given -- an HTTPS push authenticates with a token, so the SSH diagnosis must not fire
		remote := "https://github.com/rios0rios0/autobump.git"

		// when
		result := commands.IsSSHRemote(remote)

		// then
		assert.False(t, result)
	})

	t.Run("should not report SSH when the remote is unknown", func(t *testing.T) {
		t.Parallel()

		// given -- originRemoteURL returns an empty string when it cannot resolve a remote
		remote := ""

		// when
		result := commands.IsSSHRemote(remote)

		// then
		assert.False(t, result)
	})
}

func TestDescribeMissingSSHCredential(t *testing.T) {
	t.Parallel()

	t.Run("should state that the push never runs the ssh binary when nothing is configured", func(t *testing.T) {
		t.Parallel()

		// given -- the fact that resolves the confusion: interop replaces a binary AutoBump
		// never executes, so a working `git push` says nothing about AutoBump's own push
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "never executes the \"ssh\" binary")
		assert.Contains(t, message, "core.sshCommand")
		assert.Contains(t, message, "gpg.ssh.program")
	})

	t.Run("should name both accepted credential shapes when neither is configured", func(t *testing.T) {
		t.Parallel()

		// given
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "ssh_key_path")
		assert.Contains(t, message, "SSH_AUTH_SOCK")
		assert.Contains(t, message, "ssh_auth_sock")
	})

	t.Run("should report the configured key path when no key could be loaded from it", func(t *testing.T) {
		t.Parallel()

		// given -- a set-but-unusable path is a different problem from an unset one
		config := &entities.GlobalConfig{SSHKeyPath: "/nonexistent/id_ed25519"}
		env := commands.SSHEnvironment{}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "/nonexistent/id_ed25519")
		assert.Contains(t, message, "no key could be loaded")
	})

	t.Run("should report the configured socket when nothing is listening there", func(t *testing.T) {
		t.Parallel()

		// given
		config := &entities.GlobalConfig{SSHAuthSock: "/nonexistent/agent.sock"}
		env := commands.SSHEnvironment{AuthSockUsable: false}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "/nonexistent/agent.sock")
		assert.Contains(t, message, "nothing is")
		assert.Contains(t, message, "listening there")
	})

	t.Run("should explain the Windows named pipe when running on WSL", func(t *testing.T) {
		t.Parallel()

		// given
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{IsWSL: true}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "WSL")
		assert.Contains(t, message, "named pipe")
		assert.Contains(t, message, "SSH authentication on WSL")
	})

	t.Run("should name core.sshCommand in the hint when git delegates SSH on WSL", func(t *testing.T) {
		t.Parallel()

		// given
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{IsWSL: true, SSHCommand: "ssh.exe"}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "\"ssh.exe\"")
	})

	t.Run("should omit the WSL hint when an agent socket is usable", func(t *testing.T) {
		t.Parallel()

		// given -- a reachable agent means the pipe is already bridged, so the push failed
		// for some other reason and blaming WSL would send the operator down a dead end
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{IsWSL: true, AuthSockUsable: true}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.NotContains(t, message, "named pipe")
	})

	t.Run("should omit the WSL hint when not running on WSL", func(t *testing.T) {
		t.Parallel()

		// given
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{IsWSL: false}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.NotContains(t, message, "named pipe")
		assert.NotContains(t, message, "WSL")
	})
}

func TestGitSSHCommand(t *testing.T) {
	t.Parallel()

	t.Run("should return the configured core.sshCommand", func(t *testing.T) {
		t.Parallel()

		// given
		config := gitconfig.NewConfig()
		config.Raw.Section("core").SetOption("sshCommand", "ssh.exe")

		// when
		result := commands.GitSSHCommand(config)

		// then
		assert.Equal(t, "ssh.exe", result)
	})

	t.Run("should find the option when git wrote the key lowercased", func(t *testing.T) {
		t.Parallel()

		// given -- `git config core.sshCommand` stores the key as `sshcommand` on disk
		config := gitconfig.NewConfig()
		config.Raw.Section("core").SetOption("sshcommand", "ssh.exe")

		// when
		result := commands.GitSSHCommand(config)

		// then
		assert.Equal(t, "ssh.exe", result)
	})

	t.Run("should return empty when the option is unset", func(t *testing.T) {
		t.Parallel()

		// given
		config := gitconfig.NewConfig()

		// when
		result := commands.GitSSHCommand(config)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty when there is no git config", func(t *testing.T) {
		t.Parallel()

		// given
		var config *gitconfig.Config

		// when
		result := commands.GitSSHCommand(config)

		// then
		assert.Empty(t, result)
	})
}

func TestOriginRemoteURL(t *testing.T) {
	t.Parallel()

	t.Run("should return the origin URL when the remote is configured", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"git@github.com:rios0rios0/autobump.git"},
		})
		require.NoError(t, err)

		// when
		result := commands.OriginRemoteURL(repo)

		// then
		assert.Equal(t, "git@github.com:rios0rios0/autobump.git", result)
	})

	t.Run("should return empty when the repository has no origin", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)

		// when
		result := commands.OriginRemoteURL(repo)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty when there is no repository", func(t *testing.T) {
		t.Parallel()

		// given
		var repo *git.Repository

		// when
		result := commands.OriginRemoteURL(repo)

		// then
		assert.Empty(t, result)
	})
}

func TestKernelIsWSL(t *testing.T) {
	t.Parallel()

	t.Run("should recognise a WSL2 kernel when the release names microsoft", func(t *testing.T) {
		t.Parallel()

		// given
		version := "Linux version 6.18.33.2-microsoft-standard-WSL2 (root@f1bbfb02316b)"

		// when
		result := commands.KernelIsWSL(version)

		// then
		assert.True(t, result)
	})

	t.Run("should recognise the vendor string regardless of case", func(t *testing.T) {
		t.Parallel()

		// given
		version := "Linux version 5.15.0-Microsoft-standard"

		// when
		result := commands.KernelIsWSL(version)

		// then
		assert.True(t, result)
	})

	t.Run("should not recognise a bare-metal kernel", func(t *testing.T) {
		t.Parallel()

		// given
		version := "Linux version 6.11.0-generic (buildd@lcy02)"

		// when
		result := commands.KernelIsWSL(version)

		// then
		assert.False(t, result)
	})
}

func TestExplainSSHPushFailure(t *testing.T) {
	t.Parallel()

	t.Run("should return the push error unchanged when the remote is HTTPS", func(t *testing.T) {
		t.Parallel()

		// given -- an HTTPS push authenticates with a token, so a missing SSH credential is
		// not the reason it failed and the diagnosis must not displace the real one
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/rios0rios0/autobump.git"},
		})
		require.NoError(t, err)

		ctx := &commands.RepoContext{Repo: repo, GlobalConfig: &entities.GlobalConfig{}}
		pushErr := errors.New("authentication required")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		assert.Equal(t, pushErr, result)
		assert.NotErrorIs(t, result, commands.ErrNoSSHCredential)
	})

	t.Run("should return the push error unchanged when there is no origin remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)

		ctx := &commands.RepoContext{Repo: repo, GlobalConfig: &entities.GlobalConfig{}}
		pushErr := errors.New("some other failure")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		assert.Equal(t, pushErr, result)
	})
}
