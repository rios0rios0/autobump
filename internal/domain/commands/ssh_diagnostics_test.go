package commands_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
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

		// given -- originPushURL returns an empty string when it cannot resolve a remote
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
		env := commands.SSHEnvironment{}

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

	t.Run("should keep the WSL hint when a detected agent socket answers nothing", func(t *testing.T) {
		t.Parallel()

		// given -- a dead relay leaves its socket file behind. Nothing reaches this renderer
		// until every detected socket has failed to dial, so an existing socket here means a
		// bridge that died, which is exactly when the operator needs the explanation
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{
			IsWSL:                true,
			UnusableAgentSockets: []string{"/home/u/.ssh/agent.sock"},
		}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "named pipe")
	})

	t.Run("should name a detected socket that answers nothing when ssh_auth_sock is unset", func(t *testing.T) {
		t.Parallel()

		// given -- SSH_AUTH_SOCK is exported and points at a dead socket; telling the operator
		// to export it is the least useful thing the message could say
		config := &entities.GlobalConfig{}
		env := commands.SSHEnvironment{UnusableAgentSockets: []string{"/home/u/.ssh/agent.sock"}}

		// when
		message := commands.DescribeMissingSSHCredential(config, env)

		// then
		assert.Contains(t, message, "/home/u/.ssh/agent.sock")
		assert.Contains(t, message, "nothing is listening on it")
		assert.NotContains(t, message, "export SSH_AUTH_SOCK")
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

func TestOriginPushURL(t *testing.T) {
	t.Parallel()

	t.Run("should return the only URL when origin has one", func(t *testing.T) {
		t.Parallel()

		// given
		repo := newRepoWithOriginPushURL(t, "git@github.com:rios0rios0/autobump.git", "")

		// when
		result := commands.OriginPushURL(repo)

		// then
		assert.Equal(t, "git@github.com:rios0rios0/autobump.git", result)
	})

	t.Run("should return the pushurl when origin fetches from another URL", func(t *testing.T) {
		t.Parallel()

		// given -- what `git remote set-url --push` leaves: an HTTPS fetch URL, an SSH push URL
		repo := newRepoWithOriginPushURL(t,
			"https://github.com/rios0rios0/autobump.git", "git@github.com-mine:rios0rios0/autobump.git",
		)

		// when
		result := commands.OriginPushURL(repo)

		// then
		assert.Equal(t, "git@github.com-mine:rios0rios0/autobump.git", result)
	})

	t.Run("should return the last URL when origin has two url entries", func(t *testing.T) {
		t.Parallel()

		// given -- go-git pushes to the last of origin's URLs, whether a pushurl or a url set it
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"git@github.com:rios0rios0/autobump.git", "git@gitlab.com:rios0rios0/autobump.git"},
		})
		require.NoError(t, err)

		// when
		result := commands.OriginPushURL(repo)

		// then
		assert.Equal(t, "git@gitlab.com:rios0rios0/autobump.git", result)
	})

	t.Run("should return empty when the repository has no origin", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)

		// when
		result := commands.OriginPushURL(repo)

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty when there is no repository", func(t *testing.T) {
		t.Parallel()

		// given
		var repo *git.Repository

		// when
		result := commands.OriginPushURL(repo)

		// then
		assert.Empty(t, result)
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

	t.Run("should name the host and where aliases are read when an SSH host does not resolve", func(t *testing.T) {
		t.Parallel()

		// given
		repo, err := git.PlainInit(filepath.Join(t.TempDir(), "repo"), false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"git@github.com-mine:rios0rios0/autobump.git"},
		})
		require.NoError(t, err)

		ctx := &commands.RepoContext{Repo: repo, GlobalConfig: &entities.GlobalConfig{}}
		pushErr := newUnresolvedHostPushError("github.com-mine")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		require.ErrorIs(t, result, commands.ErrSSHHostUnresolved)
		require.ErrorIs(t, result, pushErr, "the original failure stays in the chain")
		require.NotErrorIs(t, result, commands.ErrNoSSHCredential)
		assert.Contains(t, result.Error(), `"github.com-mine"`)
		assert.Contains(t, result.Error(), "HostName")
	})

	t.Run("should explain an unresolved SSH push URL when origin fetches over HTTPS", func(t *testing.T) {
		t.Parallel()

		// given -- the push dialed the SSH pushurl; the HTTPS url beside it is only fetched from
		repo := newRepoWithOriginPushURL(t,
			"https://github.com/rios0rios0/autobump.git", "git@github.com-mine:rios0rios0/autobump.git",
		)
		ctx := &commands.RepoContext{
			Repo:         repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
		}
		pushErr := newUnresolvedHostPushError("github.com-mine")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		require.ErrorIs(t, result, commands.ErrSSHHostUnresolved)
		require.ErrorIs(t, result, pushErr, "the original failure stays in the chain")
		assert.Contains(t, result.Error(), `"github.com-mine"`)
	})

	t.Run("should return the push error unchanged when origin pushes over HTTPS beside an SSH URL", func(t *testing.T) {
		t.Parallel()

		// given -- an HTTPS push whose host does not resolve: ssh_config has nothing to do with it,
		// whatever origin fetches from
		repo := newRepoWithOriginPushURL(t,
			"git@github.com-mine:rios0rios0/autobump.git", "https://git.example.invalid/rios0rios0/autobump.git",
		)
		ctx := &commands.RepoContext{
			Repo:         repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
		}
		pushErr := newUnresolvedHostPushError("git.example.invalid")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		assert.Equal(t, pushErr, result)
		assert.NotErrorIs(t, result, commands.ErrSSHHostUnresolved)
	})
}

// TestExplainSSHPushFailureWithoutSSHCredential is deliberately not parallel: it empties
// SSH_AUTH_SOCK and points HOME at an empty directory with t.Setenv, which the runtime forbids
// in a parallel test, so that collectSSHAuthMethods finds no SSH credential on any host.
func TestExplainSSHPushFailureWithoutSSHCredential(t *testing.T) {
	tests := []struct {
		name     string
		fetchURL string
		// pushURL is origin's pushurl, or empty when origin has only fetchURL.
		pushURL string
		// wantHost is the host the diagnosis names, or empty when it must name none.
		wantHost string
		// fetchHost is the host of fetchURL when the push went elsewhere, which the diagnosis
		// must not name.
		fetchHost string
	}{
		{
			name:     "should name the host of origin's only URL when it is SSH",
			fetchURL: "git@github.com:rios0rios0/autobump.git",
			wantHost: "github.com",
		},
		{
			name:      "should explain the SSH push URL when origin fetches over HTTPS",
			fetchURL:  "https://github.com/rios0rios0/autobump.git",
			pushURL:   "git@github.com-mine:rios0rios0/autobump.git",
			wantHost:  "github.com-mine",
			fetchHost: "github.com",
		},
		{
			name:      "should name the push URL's host when origin fetches from another SSH host",
			fetchURL:  "git@gitlab.com:rios0rios0/autobump.git",
			pushURL:   "ssh://git@ssh.dev.azure.com/v3/rios0rios0/autobump/autobump",
			wantHost:  "ssh.dev.azure.com",
			fetchHost: "gitlab.com",
		},
		{
			name:     "should name no host when origin's SSH URL names none",
			fetchURL: "ssh:///rios0rios0/autobump.git",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// given
			t.Setenv("SSH_AUTH_SOCK", "")
			t.Setenv("HOME", t.TempDir())
			ctx := &commands.RepoContext{
				Repo:         newRepoWithOriginPushURL(t, test.fetchURL, test.pushURL),
				GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			}
			pushErr := errors.New("could not push changes to remote repository")

			// when
			result := commands.ExplainSSHPushFailure(ctx, pushErr)

			// then
			require.ErrorIs(t, result, commands.ErrNoSSHCredential)
			require.ErrorIs(t, result, pushErr, "the original failure stays in the chain")
			if test.wantHost == "" {
				assert.NotContains(t, result.Error(), "the host of origin's push URL")
				return
			}
			assert.Contains(t, result.Error(),
				fmt.Sprintf("pushing to %q, the host of origin's push URL", test.wantHost))
			if test.fetchHost != "" {
				assert.NotContains(t, result.Error(), fmt.Sprintf("%q", test.fetchHost))
			}
		})
	}

	t.Run("should return the push error unchanged when origin pushes over HTTPS beside an SSH URL", func(t *testing.T) {
		// given -- an HTTPS push authenticates with a token, so a missing SSH credential is not
		// why it failed, even though origin fetches over SSH
		t.Setenv("SSH_AUTH_SOCK", "")
		t.Setenv("HOME", t.TempDir())
		ctx := &commands.RepoContext{
			Repo: newRepoWithOriginPushURL(t,
				"git@github.com-mine:rios0rios0/autobump.git", "https://github.com/rios0rios0/autobump.git",
			),
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
		}
		pushErr := errors.New("authentication required")

		// when
		result := commands.ExplainSSHPushFailure(ctx, pushErr)

		// then
		assert.Equal(t, pushErr, result)
		assert.NotErrorIs(t, result, commands.ErrNoSSHCredential)
	})
}

// newRepoWithOriginPushURL returns a repository whose origin fetches from fetchURL and, when
// pushURL is not empty, pushes to pushURL. The pushurl is written to .git/config the way
// `git remote set-url --push` writes it: go-git's RemoteConfig has no field for one, and
// reading it from the file is what appends it to RemoteConfig.URLs.
func newRepoWithOriginPushURL(t *testing.T, fetchURL, pushURL string) *git.Repository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "repo")
	_, err := git.PlainInit(path, false)
	require.NoError(t, err)

	origin := "[remote \"origin\"]\n\turl = " + fetchURL + "\n"
	wantURLs := []string{fetchURL}
	if pushURL != "" {
		origin += "\tpushurl = " + pushURL + "\n"
		wantURLs = append(wantURLs, pushURL)
	}
	configPath := filepath.Join(path, ".git", "config")
	existing, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, append(existing, origin...), 0o600))

	repo, err := git.PlainOpen(path)
	require.NoError(t, err)
	remote, err := repo.Remote("origin")
	require.NoError(t, err)
	require.Equal(t, wantURLs, remote.Config().URLs, "go-git should read the pushurl after the url")
	return repo
}

// newUnresolvedHostPushError returns the error a push fails with when go-git's dial finds no
// address for host, which is what it returns for a host alias DNS has never heard of.
func newUnresolvedHostPushError(host string) error {
	dnsErr := &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	return fmt.Errorf("could not push changes to remote repository: %w",
		&net.OpError{Op: "dial", Net: "tcp", Err: dnsErr})
}

func TestDescribeUnresolvedSSHHost(t *testing.T) {
	t.Parallel()

	t.Run("should name the host and the ssh_config files gitforge reads", func(t *testing.T) {
		t.Parallel()

		// given -- the description gitforge's SSHConfigSearchDescription gives
		searched := "~/.ssh/config, /etc/ssh/ssh_config and the Windows profile's .ssh/config"

		// when
		message := commands.DescribeUnresolvedSSHHost("github.com-mine", searched)

		// then
		assert.Contains(t, message, `"github.com-mine"`)
		assert.Contains(t, message, searched)
		assert.Contains(t, message, "HostName")
	})
}

// TestDetectSSHEnvironment is deliberately not parallel: it mutates SSH_AUTH_SOCK with
// t.Setenv, which the runtime forbids in a parallel test.
//
// It covers the seam an earlier revision got wrong. gitforge's DetectSSHAgentSockets only stats
// for ModeSocket, so a socket file that answers nothing still appears here -- and because this
// runs only after collectSSHAuthMethods failed to dial every one of those paths, that is the
// only thing it can mean. Asserting it against a real socket is what keeps the field's name
// honest; the renderer tests build the struct by hand and would not notice it drifting.
func TestDetectSSHEnvironment(t *testing.T) {
	t.Run("should report a socket that exists as unusable", func(t *testing.T) {
		// given -- a real listener, closed before the probe, so the file outlives the agent
		// exactly as it does when a relay dies
		socketPath := filepath.Join(t.TempDir(), "agent.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		// Go unlinks a Unix socket on Close; a relay that dies does not, and the leftover
		// file is the whole point of the case, so the cleanup is turned off here.
		unixListener, ok := listener.(*net.UnixListener)
		require.True(t, ok)
		unixListener.SetUnlinkOnClose(false)
		require.NoError(t, listener.Close())
		t.Setenv("SSH_AUTH_SOCK", socketPath)

		// when
		env := commands.DetectSSHEnvironment("ssh.exe")

		// then
		assert.Contains(t, env.UnusableAgentSockets, socketPath)
		assert.Equal(t, "ssh.exe", env.SSHCommand)
	})

	t.Run("should report no sockets when SSH_AUTH_SOCK names nothing", func(t *testing.T) {
		// given
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "absent.sock"))

		// when
		env := commands.DetectSSHEnvironment("")

		// then
		assert.NotContains(t, env.UnusableAgentSockets, "absent.sock")
	})
}
