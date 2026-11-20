package commands

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

var (
	// ErrNoSSHCredential reports that an SSH push was attempted without any credential
	// AutoBump is able to use.
	ErrNoSSHCredential = errors.New("no usable SSH credential for pushing")

	// ErrSSHHostUnresolved reports an SSH push whose host has no address -- typically a host
	// alias defined only in a file AutoBump does not read.
	ErrSSHHostUnresolved = errors.New("cannot resolve the SSH host")
)

// sshEnvironment captures the host facts that decide whether AutoBump can authenticate an
// SSH push. It is plain data so the diagnosis can be rendered — and tested — without
// touching the host it describes.
type sshEnvironment struct {
	// UnusableAgentSockets lists agent sockets that exist on disk but yielded no credential.
	//
	// The list is not "sockets that might work": this is built only after
	// collectSSHAuthMethods returned nothing, and its auto-detect loop has by then dialed
	// every path gitforge's DetectSSHAgentSockets reports. So a non-empty list means a dead agent — a
	// relay that exited leaving its socket behind, or an ssh-agent that is gone — which is
	// the opposite of what the existence of the file suggests.
	UnusableAgentSockets []string

	// IsWSL reports whether AutoBump is running under the Windows Subsystem for Linux.
	IsWSL bool

	// SSHCommand holds git's core.sshCommand. AutoBump never executes it; it is read only
	// to recognise the configuration that makes its absence surprising.
	SSHCommand string
}

// explainSSHPushFailure wraps a failed SSH push with what AutoBump knows about its cause: a
// host that has no address, or no SSH credential to offer. A push over a non-SSH remote, and
// one that failed for any other reason, are returned unchanged: the explanation is only ever
// added to the failure it actually explains.
//
// The remote is origin's push URL (originPushURL), the one the push dialed, and never its
// first URL, the one fetches use: with a pushurl beside it the two can differ in transport and
// host, and reading the first one left an SSH push beside an HTTPS fetch URL unexplained and
// explained an HTTPS push beside an SSH fetch URL as an SSH one.
func explainSSHPushFailure(ctx *RepoContext, pushErr error) error {
	pushURL := originPushURL(ctx.Repo)
	if !isSSHRemote(pushURL) {
		return pushErr
	}

	if host, unresolved := unresolvedSSHHost(pushErr); unresolved {
		return fmt.Errorf(
			"%w: %w\n\n%s",
			ErrSSHHostUnresolved, pushErr,
			describeUnresolvedSSHHost(remoteHost(pushURL), host, gitInfra.SSHConfigSearchDescription()),
		)
	}

	// Recomputed rather than threaded through the push: this runs only on the failure path,
	// and the probe behind it stats and dials a local socket without touching the network.
	if len(collectSSHAuthMethods(ctx.GlobalConfig)) > 0 {
		return pushErr
	}

	env := detectSSHEnvironment(gitSSHCommand(ctx.GlobalGitConfig))

	return fmt.Errorf(
		"%w%s: %w\n\n%s",
		ErrNoSSHCredential, describePushHost(pushURL), pushErr,
		describeMissingSSHCredential(ctx.GlobalConfig, env),
	)
}

// describePushHost names the host of origin's push URL for the diagnosis, which tells the
// operator where the push went when origin fetches from another URL, or returns an empty
// string when the URL names no host. Only the host is named: a URL can carry a credential in
// its userinfo.
func describePushHost(pushURL string) string {
	host := remoteHost(pushURL)
	if host == "" {
		return ""
	}

	return fmt.Sprintf(" to %q, the host of origin's push URL", host)
}

// remoteHost returns the host name a remote URL names, in any form go-git reads (scp-like,
// ssh://, http(s)://), or an empty string for a URL that names none. It reads the host as
// gitforge's push does when it compares hosts (remoteHost, in pkg/git/infrastructure).
func remoteHost(remoteURL string) string {
	endpoint, err := transport.NewEndpoint(remoteURL)
	if err != nil {
		return ""
	}

	return endpoint.Host
}

// unresolvedSSHHost returns the host name a failed push found no address for, when that is
// why it failed.
func unresolvedSSHHost(pushErr error) (string, bool) {
	var dnsErr *net.DNSError
	if errors.As(pushErr, &dnsErr) && dnsErr.IsNotFound {
		return dnsErr.Name, true
	}
	return "", false
}

// describeUnresolvedSSHHost renders the operator-facing explanation for an SSH host that has
// no address: pushHost is the host origin's push URL names, dialedHost the name DNS found no
// address for, and searched names the ssh_config files read on this host, as gitforge's
// SSHConfigSearchDescription gives them.
//
// gitforge's SSH transport resolves host aliases itself, from those files, and dials the host
// as the URL writes it unless the alias has a HostName, which replaces it; Go reports the name
// exactly as it was dialed. So a dialedHost that is the URL's own host is an alias defined
// somewhere else, or a name that does not exist at all, and without saying so an alias that
// works for every git command reads as a network failure. Any other dialedHost is the HostName
// of an alias those files do define (describeUnresolvedSSHHostName).
func describeUnresolvedSSHHost(pushHost, dialedHost, searched string) string {
	if pushHost != "" && dialedHost != pushHost {
		return describeUnresolvedSSHHostName(pushHost, dialedHost, searched)
	}

	return fmt.Sprintf(
		"AutoBump could not resolve the SSH host %q. AutoBump never executes the \"ssh\" binary, so a "+
			"host alias applies to the push only when it is defined in an ssh_config file AutoBump reads: "+
			"%s. Define the alias there with at least a HostName; AutoBump applies its HostName, Port, "+
			"IdentityFile and IdentitiesOnly.\n",
		dialedHost, searched,
	)
}

// describeUnresolvedSSHHostName renders the explanation for a host alias whose HostName has no
// address: alias is the host origin's push URL names, and hostName the HostName an ssh_config
// file gives it, the name the push dialed in its place. The alias is defined and was applied, so
// asking the operator to define it would send them to a line that is already right.
func describeUnresolvedSSHHostName(alias, hostName, searched string) string {
	return fmt.Sprintf(
		"AutoBump could not resolve the SSH host %q. It is the HostName of %q, the host alias origin's "+
			"push URL names: the alias is defined in an ssh_config file AutoBump reads (%s), and AutoBump "+
			"applied it, but its HostName does not resolve. Correct that HostName, or make sure the name "+
			"resolves from this machine.\n",
		hostName, alias, searched,
	)
}

// isSSHRemote reports whether a remote URL is pushed over SSH, matching the transport
// detection gitforge applies in PushWithTransportDetection.
func isSSHRemote(remoteURL string) bool {
	return strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://")
}

// originPushURL returns the URL a push to origin dials, as gitforge's push picks it
// (gitInfra.OriginPushURL): the last of origin's URLs, which is its pushurl when one is
// set. It returns an empty string when the remote is missing or carries no URL. An empty
// string is not an SSH remote, which leaves the original push error untouched — the
// diagnosis never displaces a more specific failure.
func originPushURL(repo *git.Repository) string {
	if repo == nil {
		return ""
	}
	pushURL, err := gitInfra.OriginPushURL(repo)
	if err != nil {
		return ""
	}

	return pushURL
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
		UnusableAgentSockets: gitInfra.DetectSSHAgentSockets(),
		IsWSL:                gitInfra.RunningUnderWSL(),
		SSHCommand:           sshCommand,
	}
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
