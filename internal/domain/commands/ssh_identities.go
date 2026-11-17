package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	logger "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// errNoKeyInIdentityFile reports an IdentityFile that holds no key AutoBump can read.
var errNoKeyInIdentityFile = errors.New("no key can be read from it")

// sshIdentity is a key an alias's IdentityFile names. Its public half picks the matching
// key out of the SSH agent. Signer is set when the file is a private key AutoBump can use
// without a passphrase, which is offered directly when the agent does not hold the key, as
// OpenSSH does; an encrypted key has to be in the agent, or configured as ssh_key_path.
type sshIdentity struct {
	path      string
	publicKey ssh.PublicKey
	signer    ssh.Signer
}

// loadSSHIdentities reads the key behind each IdentityFile path, in order, skipping the
// paths that name no key AutoBump can read.
func loadSSHIdentities(paths []string) []sshIdentity {
	identities := make([]sshIdentity, 0, len(paths))
	for _, path := range paths {
		identity, err := loadSSHIdentity(path)
		if err != nil {
			logger.Debugf("Skipping IdentityFile %s: %v", path, err)
			continue
		}
		identities = append(identities, identity)
	}
	return identities
}

// loadSSHIdentity reads the key an IdentityFile names. The file may be a public key itself
// -- the form 1Password documents for telling ssh which agent key a host uses -- or a
// private key: an unencrypted one yields the key outright, and an encrypted OpenSSH key
// still carries its public half in the clear. Failing both, the .pub ssh-keygen writes
// beside a private key gives the public half, which is all there is on disk when the key
// lives only in an agent.
func loadSSHIdentity(path string) (sshIdentity, error) {
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		if identity, found := identityFromKeyFile(path, data); found {
			return identity, nil
		}
	}

	if pubData, err := os.ReadFile(path + ".pub"); err == nil {
		if publicKey, _, _, _, parseErr := ssh.ParseAuthorizedKey(pubData); parseErr == nil {
			return sshIdentity{path: path, publicKey: publicKey}, nil
		}
	}

	if readErr != nil {
		return sshIdentity{}, readErr
	}
	return sshIdentity{}, fmt.Errorf("%s: %w", path, errNoKeyInIdentityFile)
}

// identityFromKeyFile reads the key a file holds: a public key, an unencrypted private key,
// or the public half an encrypted OpenSSH private key carries.
func identityFromKeyFile(path string, data []byte) (sshIdentity, bool) {
	if publicKey, _, _, _, err := ssh.ParseAuthorizedKey(data); err == nil {
		return sshIdentity{path: path, publicKey: publicKey}, true
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return sshIdentity{path: path, publicKey: signer.PublicKey(), signer: signer}, true
	}

	var passphraseErr *ssh.PassphraseMissingError
	if errors.As(err, &passphraseErr) && passphraseErr.PublicKey != nil {
		return sshIdentity{path: path, publicKey: passphraseErr.PublicKey}, true
	}
	return sshIdentity{}, false
}

// selectSigners orders and narrows the keys offered for one alias the way OpenSSH's
// pubkey_prepare does: first the agent keys the alias's IdentityFile entries name, in
// IdentityFile order, then -- unless IdentitiesOnly is set -- the agent's other keys, in
// the agent's order, and last the unencrypted IdentityFile keys the agent does not hold.
// Offering the named key first is what lets a host alias choose the account on a forge
// that signs in whoever owns the first key it recognises.
func selectSigners(agentSigners []ssh.Signer, identities []sshIdentity, identitiesOnly bool) []ssh.Signer {
	selected := make([]ssh.Signer, 0, len(agentSigners)+len(identities))
	taken := make([]bool, len(agentSigners))
	var fromFiles []ssh.Signer
	for _, identity := range identities {
		if index := agentSignerIndex(agentSigners, taken, identity.publicKey); index >= 0 {
			logger.Debugf("Offering the SSH agent key IdentityFile %s names", identity.path)
			selected = append(selected, agentSigners[index])
			taken[index] = true
			continue
		}
		if identity.signer != nil && !signersInclude(fromFiles, identity.publicKey) {
			logger.Debugf(
				"Offering the unencrypted key IdentityFile %s holds, which the SSH agent does not",
				identity.path,
			)
			fromFiles = append(fromFiles, identity.signer)
			continue
		}
		logger.Debugf("IdentityFile %s names a key the SSH agent does not hold", identity.path)
	}

	if !identitiesOnly {
		for index, signer := range agentSigners {
			if !taken[index] {
				selected = append(selected, signer)
			}
		}
	}
	return append(selected, fromFiles...)
}

// agentSignerIndex returns the index of the agent key whose public half is publicKey, among
// the keys not taken yet, or -1 when the agent does not hold it.
func agentSignerIndex(agentSigners []ssh.Signer, taken []bool, publicKey ssh.PublicKey) int {
	wanted := publicKey.Marshal()
	for index, signer := range agentSigners {
		if !taken[index] && bytes.Equal(signer.PublicKey().Marshal(), wanted) {
			return index
		}
	}
	return -1
}

// signersInclude reports whether one of signers has publicKey as its public half.
func signersInclude(signers []ssh.Signer, publicKey ssh.PublicKey) bool {
	wanted := publicKey.Marshal()
	for _, signer := range signers {
		if bytes.Equal(signer.PublicKey().Marshal(), wanted) {
			return true
		}
	}
	return false
}
