package commands

import (
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	logger "github.com/sirupsen/logrus"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// sshTransport is go-git's SSH transport with the operator's SSH client configuration
// applied to every connection. AutoBump never runs the ssh binary, so nothing else applies
// it; and because the transport sits under every SSH connection go-git opens, it covers the
// push, the clone and the stale-branch listing AutoBump asks for as much as the SSH agent
// fallback gitforge opens on its own after AutoBump's methods fail. For each connection it:
//
//   - dials the HostName and Port the remote's host alias names in ssh_config, read under
//     WSL from the Windows profiles too, where a setup that delegates SSH to ssh.exe defines
//     its aliases;
//   - offers an SSH agent's keys the way OpenSSH would for that alias: the keys its
//     IdentityFile entries name first, and with IdentitiesOnly nothing else;
//   - negotiates only the host key types known_hosts records for the host, so the key the
//     server presents is one the host key callback can verify.
type sshTransport struct {
	next     transport.Transport
	config   func() *sshConfigFiles
	hostKeys func() *knownhosts.HostKeyDB

	// warned holds the aliases a connection has already warned about offering no key, so
	// a release that connects several times says so once.
	warned sync.Map
}

// NewSSHTransport wraps go-git's SSH transport (gitssh.DefaultClient) so every connection
// it opens honours the operator's ssh_config and known_hosts; main.go installs the result
// for the "ssh" scheme. The files are discovered on the first connection, not here, so a
// run that never touches SSH neither pays for the discovery nor sees its warnings.
func NewSSHTransport(next transport.Transport) transport.Transport {
	return newSSHTransport(next, sshConfig, knownHostsDB)
}

// newSSHTransport builds the transport from where it reads ssh_config and known_hosts,
// which is what a test replaces.
func newSSHTransport(
	next transport.Transport,
	config func() *sshConfigFiles,
	hostKeys func() *knownhosts.HostKeyDB,
) *sshTransport {
	return &sshTransport{next: next, config: config, hostKeys: hostKeys}
}

// NewUploadPackSession opens a fetch, clone or remote listing session to the endpoint, with
// the operator's SSH client configuration applied.
func (t *sshTransport) NewUploadPackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.UploadPackSession, error) {
	target, prepared := t.prepare(endpoint, auth)
	return t.next.NewUploadPackSession(target, prepared)
}

// NewReceivePackSession opens a push session to the endpoint, with the operator's SSH
// client configuration applied.
func (t *sshTransport) NewReceivePackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.ReceivePackSession, error) {
	target, prepared := t.prepare(endpoint, auth)
	return t.next.NewReceivePackSession(target, prepared)
}

// prepare resolves the endpoint's host alias and fits auth to the host it resolves to.
func (t *sshTransport) prepare(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (*transport.Endpoint, transport.AuthMethod) {
	settings := t.config().resolve(endpoint.Host, endpoint.User)
	target := settings.target(endpoint)
	if target != endpoint {
		logger.Debugf("Resolved SSH host %q to %s through ssh_config", endpoint.Host, sshHostWithPort(target))
	}

	auth = t.withIdentities(auth, endpoint.Host, settings)
	return target, withHostKeyAlgorithms(auth, t.hostKeys(), sshHostWithPort(target))
}

// withIdentities makes an SSH agent auth method offer the agent's keys the way OpenSSH
// would for the alias (see selectAgentSigners). Only an agent's keys are chosen among: a key
// file configured as ssh_key_path is offered as configured, and an alias with no
// IdentityFile leaves the agent's keys in the agent's order.
func (t *sshTransport) withIdentities(
	auth transport.AuthMethod,
	alias string,
	settings sshHostSettings,
) transport.AuthMethod {
	agentAuth, isAgentAuth := auth.(*gitssh.PublicKeysCallback)
	if !isAgentAuth || agentAuth == nil || agentAuth.Callback == nil || len(settings.IdentityFiles) == 0 {
		return auth
	}

	identities := loadSSHIdentities(settings.IdentityFiles)
	listSigners := agentAuth.Callback
	narrowed := *agentAuth
	narrowed.Callback = func() ([]ssh.Signer, error) {
		signers, err := listSigners()
		if err != nil {
			return nil, err
		}

		selected := selectAgentSigners(signers, identities, settings.IdentitiesOnly)
		if len(selected) == 0 && len(signers) > 0 {
			t.warnNoKeyOffered(alias, settings)
		}
		return selected, nil
	}
	return &narrowed
}

// warnNoKeyOffered explains, once per alias, a connection that offers no key at all:
// IdentitiesOnly narrowed the agent's keys to the ones IdentityFile names, and the agent
// holds none of them. The server's refusal that follows says nothing about why.
func (t *sshTransport) warnNoKeyOffered(alias string, settings sshHostSettings) {
	if _, alreadyWarned := t.warned.LoadOrStore(alias, struct{}{}); alreadyWarned {
		return
	}
	logger.Warnf(
		"The ssh_config entry for %q sets IdentitiesOnly, and the SSH agent holds none of the keys "+
			"its IdentityFile names (%s), so no key is offered: add that key to the agent, or point "+
			"IdentityFile at the public key of one it holds",
		alias, strings.Join(settings.IdentityFiles, ", "),
	)
}

// withHostKeyAlgorithms returns a copy of an SSH auth method that negotiates only the host
// key types known_hosts records for hostWithPort, when the method verifies host keys with a
// callback it was given.
//
// A server presents one host key, of the type the handshake settles on, and x/crypto
// settles on the first type in its own preference order that the server offers -- ECDSA
// ahead of Ed25519. known_hosts commonly records only the type OpenSSH settled on, which is
// Ed25519 for GitHub, so the key presented is not the one recorded and the callback fails
// with "knownhosts: key mismatch" against a host it actually trusts. go-git asks for the
// recorded types only when it reads known_hosts itself; every callback AutoBump gives an
// auth method is built from the same knownHostsDB these types are read from.
func withHostKeyAlgorithms(
	auth transport.AuthMethod,
	hostKeys *knownhosts.HostKeyDB,
	hostWithPort string,
) transport.AuthMethod {
	if hostKeys == nil {
		return auth
	}

	switch method := auth.(type) {
	case *gitssh.PublicKeysCallback:
		if method != nil && needsHostKeyAlgorithms(method.HostKeyCallbackHelper) {
			pinned := *method
			pinned.HostKeyAlgorithms = hostKeys.HostKeyAlgorithms(hostWithPort)
			return &pinned
		}
	case *gitssh.PublicKeys:
		if method != nil && needsHostKeyAlgorithms(method.HostKeyCallbackHelper) {
			pinned := *method
			pinned.HostKeyAlgorithms = hostKeys.HostKeyAlgorithms(hostWithPort)
			return &pinned
		}
	}
	return auth
}

// needsHostKeyAlgorithms reports whether an auth method verifies host keys with a callback
// it was given and has chosen no host key types. A method without a callback is left alone:
// go-git gives it a default callback reading its own known_hosts, which the types read from
// AutoBump's discovery would not match.
func needsHostKeyAlgorithms(helper gitssh.HostKeyCallbackHelper) bool {
	return helper.HostKeyCallback != nil && len(helper.HostKeyAlgorithms) == 0
}
