package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// ErrNoSSHCredential reports that an SSH push was attempted without any credential
// AutoBump is able to use.
var ErrNoSSHCredential = errors.New("no usable SSH credential for pushing")

const (
	// wslKernelMarker is the substring Microsoft's WSL kernels carry in their release string.
	wslKernelMarker = "microsoft"

	// procVersionPath names the kernel version file consulted to recognise WSL.
	procVersionPath = "/proc/version"
)

// sshEnvironment captures the host facts that decide whether AutoBump can authenticate an
// SSH push. It is plain data so the diagnosis can be rendered — and tested — without
// touching the host it describes.
type sshEnvironment struct {
	// UnusableAgentSockets lists agent sockets that exist on disk but yielded no credential.
	//
	// The list is not "sockets that might work": this is built only after
	// collectSSHAuthMethods returned nothing, and its auto-detect loop has by then dialed
	// every path detectSSHAgentSockets reports. So a non-empty list means a dead agent — a
	// relay that exited leaving its socket behind, or an ssh-agent that is gone — which is
	// the opposite of what the existence of the file suggests.
	UnusableAgentSockets []string

	// IsWSL reports whether AutoBump is running under the Windows Subsystem for Linux.
	IsWSL bool

	// SSHCommand holds git's core.sshCommand. AutoBump never executes it; it is read only
	// to recognise the configuration that makes its absence surprising.
	SSHCommand string
}

// explainSSHPushFailure wraps a failed push with the reason AutoBump had no SSH credential
// to offer. A push over a non-SSH remote, and one that failed with a credential in hand, are
// returned unchanged: the explanation is only ever added to the failure it actually explains.
func explainSSHPushFailure(ctx *RepoContext, pushErr error) error {
	if !isSSHRemote(originRemoteURL(ctx.Repo)) {
		return pushErr
	}

	// Recomputed rather than threaded through the push: this runs only on the failure path,
	// and the probe behind it stats and dials a local socket without touching the network.
	if len(collectSSHAuthMethods(ctx.GlobalConfig)) > 0 {
		return pushErr
	}

	env := detectSSHEnvironment(gitSSHCommand(ctx.GlobalGitConfig))

	return fmt.Errorf(
		"%w: %w\n\n%s",
		ErrNoSSHCredential, pushErr, describeMissingSSHCredential(ctx.GlobalConfig, env),
	)
}

// isSSHRemote reports whether a remote URL is pushed over SSH, matching the transport
// detection gitforge applies in PushWithTransportDetection.
func isSSHRemote(remoteURL string) bool {
	return strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://")
}

// originRemoteURL returns the first URL configured for origin, or an empty string when the
// remote is missing or carries none. An empty string is not an SSH remote, which leaves the
// original push error untouched — the diagnosis never displaces a more specific failure.
func originRemoteURL(repo *git.Repository) string {
	if repo == nil {
		return ""
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return ""
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return ""
	}

	return urls[0]
}

// gitSSHCommand returns git's core.sshCommand. AutoBump never runs it; it is read so the
// diagnosis can name the setting the operator expected to be in force.
func gitSSHCommand(config *gitconfig.Config) string {
	if config == nil {
		return ""
	}

	return config.Raw.Section("core").Option("sshCommand")
}

// detectSSHEnvironment reads the host facts behind an SSH credential failure. sshCommand is
// git's core.sshCommand, which the caller already holds.
func detectSSHEnvironment(sshCommand string) sshEnvironment {
	return sshEnvironment{
		UnusableAgentSockets: detectSSHAgentSockets(),
		IsWSL:                runningUnderWSL(),
		SSHCommand:           sshCommand,
	}
}

// runningUnderWSL reports whether the current kernel is a WSL one. The environment
// variables WSL sets are checked first because they cost nothing; the kernel release is
// the fallback for a shell that did not inherit them.
func runningUnderWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}

	version, err := os.ReadFile(procVersionPath)
	if err != nil {
		return false
	}

	return kernelIsWSL(string(version))
}

// kernelIsWSL reports whether a kernel version string names a WSL kernel. Split out from the
// file read so the recognition itself is testable on a host that is not WSL.
func kernelIsWSL(version string) bool {
	return strings.Contains(strings.ToLower(version), wslKernelMarker)
}

// describeMissingSSHCredential renders the operator-facing explanation for an SSH push that
// found no credential. It states what AutoBump actually looks for, because the usual cause
// is a working SSH setup that AutoBump structurally cannot reach rather than a missing key.
func describeMissingSSHCredential(config *entities.GlobalConfig, env sshEnvironment) string {
	var message strings.Builder

	message.WriteString(
		"AutoBump pushes with go-git, a pure-Go SSH client, and never executes the \"ssh\" binary. " +
			"Settings that redirect that binary — git's core.sshCommand, or an \"ssh\" wrapper earlier " +
			"on PATH — therefore do not apply to the push, even though commit signing does honour " +
			"gpg.ssh.program.\n\n",
	)

	message.WriteString("AutoBump accepts either of the following, and found neither:\n")
	message.WriteString(describeKeyPathState(config))
	message.WriteString(describeAgentSocketState(config, env))

	if hint := describeWSLHint(env); hint != "" {
		message.WriteString("\n" + hint)
	}

	return message.String()
}

// describeKeyPathState reports what became of the configured ssh_key_path. A path that is
// set but unusable is a different problem from one that was never set, and saying which
// avoids sending the operator to re-check a setting that is already correct.
func describeKeyPathState(config *entities.GlobalConfig) string {
	if config.SSHKeyPath == "" {
		return "  - a private key file: set \"ssh_key_path\" (for example ~/.ssh/id_ed25519)\n"
	}

	return fmt.Sprintf(
		"  - a private key file: \"ssh_key_path\" is set to %q but no key could be loaded from it "+
			"(see the warning logged above)\n",
		config.SSHKeyPath,
	)
}

// describeAgentSocketState reports what became of the agent socket. A socket that exists but
// answers nothing is called out separately from one that was never configured, because the
// fix is to restart the agent rather than to configure anything — and telling someone to
// export a variable they have already exported is the least useful thing this can say.
func describeAgentSocketState(config *entities.GlobalConfig, env sshEnvironment) string {
	if config.SSHAuthSock != "" {
		return fmt.Sprintf(
			"  - a Unix-domain SSH agent socket: \"ssh_auth_sock\" is set to %q but nothing is "+
				"listening there\n",
			config.SSHAuthSock,
		)
	}

	if len(env.UnusableAgentSockets) > 0 {
		return fmt.Sprintf(
			"  - a Unix-domain SSH agent socket: %s exists but nothing is listening on it\n",
			strings.Join(env.UnusableAgentSockets, ", "),
		)
	}

	return "  - a Unix-domain SSH agent socket: export SSH_AUTH_SOCK, or set \"ssh_auth_sock\"\n"
}

// describeWSLHint returns the WSL-specific cause when the host shows it, and an empty string
// otherwise. Interop makes every other git command work, so without this the failure reads
// as AutoBump losing a key that demonstrably exists.
//
// Being on WSL is the whole condition. An earlier version also withheld the hint when an
// agent socket had been detected, on the reasoning that the pipe was then already bridged —
// but nothing reaches this function until every detected socket has failed to dial, so that
// guard only ever fired for a bridge that had died, which is exactly when the explanation is
// most needed. describeAgentSocketState names the dead socket; this supplies the why.
func describeWSLHint(env sshEnvironment) string {
	if !env.IsWSL {
		return ""
	}

	hint := "This machine is WSL"
	if env.SSHCommand != "" {
		hint += fmt.Sprintf(", and git delegates SSH to %q", env.SSHCommand)
	}

	return hint + ", so the keys are held by a Windows agent behind a named pipe. Go cannot " +
		"dial a Windows named pipe, so the push cannot reach them. Bridge the pipe to a " +
		"Unix-domain socket and point SSH_AUTH_SOCK at it — see \"SSH authentication on WSL\" " +
		"in the AutoBump README.\n"
}
