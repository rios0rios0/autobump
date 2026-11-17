package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	logger "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// sshIdentity is a key an alias's IdentityFile names, reduced to its public half: that is
// all it takes to pick the matching key out of the SSH agent, and AutoBump offers only
// agent keys here -- a private key on disk is used through ssh_key_path instead.
type sshIdentity struct {
	path      string
	publicKey ssh.PublicKey
}

// loadSSHIdentities reads the public key behind each IdentityFile path, in order, skipping
// the paths that name no key AutoBump can read.
func loadSSHIdentities(paths []string) []sshIdentity {
	identities := make([]sshIdentity, 0, len(paths))
	for _, path := range paths {
		publicKey, err := identityPublicKey(path)
		if err != nil {
			logger.Debugf("Skipping IdentityFile %s: %v", path, err)
			continue
		}
		identities = append(identities, sshIdentity{path: path, publicKey: publicKey})
	}
	return identities
}

// identityPublicKey returns the public key an IdentityFile names. The file may be a public
// key itself -- the form 1Password documents for telling ssh which agent key a host uses --
// or a private key, whose public half is read from the .pub file ssh-keygen writes beside
// it, or failing that from the private key file, which an OpenSSH key carries in the clear
// even when the key itself is encrypted.
func identityPublicKey(path string) (ssh.PublicKey, error) {
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		if publicKey, _, _, _, err := ssh.ParseAuthorizedKey(data); err == nil {
			return publicKey, nil
		}
	}

	if pubData, err := os.ReadFile(path + ".pub"); err == nil {
		if publicKey, _, _, _, parseErr := ssh.ParseAuthorizedKey(pubData); parseErr == nil {
			return publicKey, nil
		}
	}

	if readErr != nil {
		return nil, readErr
	}
	return privateKeyPublicHalf(data)
}

// privateKeyPublicHalf returns the public half of a private key file without needing its
// passphrase: an unencrypted key yields it directly, and an encrypted OpenSSH key reports
// it in the error that asks for the passphrase.
func privateKeyPublicHalf(data []byte) (ssh.PublicKey, error) {
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return signer.PublicKey(), nil
	}

	var passphraseErr *ssh.PassphraseMissingError
	if errors.As(err, &passphraseErr) && passphraseErr.PublicKey != nil {
		return passphraseErr.PublicKey, nil
	}
	return nil, fmt.Errorf("no public key can be read from it: %w", err)
}

// selectAgentSigners orders and narrows the agent's keys for one alias the way OpenSSH's
// pubkey_prepare does: first the agent keys the alias's IdentityFile entries name, in
// IdentityFile order, then -- unless IdentitiesOnly is set -- the agent's other keys, in
// the agent's order. Offering the named key first is what lets a host alias choose the
// account on a forge that signs in whoever owns the first key it recognises.
func selectAgentSigners(agentSigners []ssh.Signer, identities []sshIdentity, identitiesOnly bool) []ssh.Signer {
	selected := make([]ssh.Signer, 0, len(agentSigners))
	taken := make([]bool, len(agentSigners))
	for _, identity := range identities {
		index := agentSignerIndex(agentSigners, taken, identity.publicKey)
		if index < 0 {
			logger.Debugf("IdentityFile %s names a key the SSH agent does not hold", identity.path)
			continue
		}
		logger.Debugf("Offering the SSH agent key IdentityFile %s names", identity.path)
		selected = append(selected, agentSigners[index])
		taken[index] = true
	}

	if identitiesOnly {
		return selected
	}
	for index, signer := range agentSigners {
		if !taken[index] {
			selected = append(selected, signer)
		}
	}
	return selected
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
