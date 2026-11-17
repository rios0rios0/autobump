package commands

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	logger "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	// sshAgentListAttempts bounds how many times listing the agent's keys is tried when the
	// connection is dropped before the agent answers.
	sshAgentListAttempts = 3

	// sshAgentListRetryDelay is the pause before listing again. A relay that dropped the
	// connection has settled by then: on a WSL npiperelay bridge a listing right after a
	// connection that closed without a request failed most of the time, and one 200ms later
	// never did.
	sshAgentListRetryDelay = 250 * time.Millisecond
)

// errUnsupportedAgentAlgorithm reports a signature algorithm the SSH agent protocol has no
// flag to ask for.
var errUnsupportedAgentAlgorithm = errors.New("the SSH agent protocol cannot request this signature algorithm")

// sshAgentDialer opens a new connection to an SSH agent.
type sshAgentDialer func() (net.Conn, error)

// sshAgentConn is a connection to an SSH agent that remembers whether writing to it or
// reading from it failed. x/crypto reports a dropped connection and a request the agent
// refused alike, as a plain error, so this is what tells a listing worth retrying -- the
// request never reached the agent, or its answer never came back -- from one the agent
// itself turned down. A relay that drops the connection may do so before the request is
// written, failing the write with a broken pipe, or after, failing the read with EOF. The
// flag is atomic because x/crypto reads a closable connection from a goroutine of its own.
type sshAgentConn struct {
	net.Conn

	failed atomic.Bool
}

// sshAgentSigner is a key the SSH agent holds, signing over a connection of its own.
//
// x/crypto signs with the signers a PublicKeysCallback returns only after the callback has
// returned, so a signer cannot be bound to the connection that listed the keys if that
// connection is closed when the callback returns: every signature then fails with "use of
// closed network connection". Dialing once per signature keeps each connection inside a
// single call, and nothing stays open between them.
type sshAgentSigner struct {
	dial      sshAgentDialer
	publicKey ssh.PublicKey
}

// listSSHAgentSigners lists the agent's keys, in the agent's order, over a connection it
// closes before returning. Each key comes back as an sshAgentSigner, which dials the agent
// again when it is asked to sign.
//
// A listing whose connection is dropped before the agent answers is tried again after a
// pause. The liveness probe in sshAgentAuthFromSocket closes its connection without a
// request, and a relay that starts a process per connection -- npiperelay, bridging a
// Windows agent into WSL -- can drop the next connection while that one is still winding
// down. Signing is not retried: a dropped signature request may already have asked the
// operator to approve it.
func listSSHAgentSigners(dial sshAgentDialer) ([]ssh.Signer, error) {
	var keys []*agent.Key
	for attempt := 1; ; attempt++ {
		listed, dropped, err := listSSHAgentKeys(dial)
		if err == nil {
			keys = listed
			break
		}
		if !dropped || attempt == sshAgentListAttempts {
			return nil, fmt.Errorf("cannot list the SSH agent's keys: %w", err)
		}
		logger.Debugf("The SSH agent dropped the connection before listing its keys, retrying: %v", err)
		time.Sleep(sshAgentListRetryDelay)
	}

	signers := make([]ssh.Signer, 0, len(keys))
	for _, key := range keys {
		publicKey, parseErr := ssh.ParsePublicKey(key.Blob)
		if parseErr != nil {
			logger.Debugf("Skipping SSH agent key %q: %v", key.Comment, parseErr)
			continue
		}
		signers = append(signers, &sshAgentSigner{dial: dial, publicKey: publicKey})
	}

	return signers, nil
}

// listSSHAgentKeys lists the agent's keys over one connection, reporting whether a failure
// came from the connection being dropped rather than from the agent's answer.
func listSSHAgentKeys(dial sshAgentDialer) ([]*agent.Key, bool, error) {
	raw, err := dial()
	if err != nil {
		return nil, false, err
	}
	conn := &sshAgentConn{Conn: raw}
	defer closeSSHAgentConnection(conn)

	keys, err := agent.NewClient(conn).List()
	if err != nil {
		return nil, conn.failed.Load(), err
	}
	return keys, false, nil
}

// Read reads from the agent, remembering whether a read failed.
func (c *sshAgentConn) Read(buffer []byte) (int, error) {
	read, err := c.Conn.Read(buffer)
	if err != nil {
		c.failed.Store(true)
	}
	return read, err
}

// Write writes to the agent, remembering whether a write failed.
func (c *sshAgentConn) Write(buffer []byte) (int, error) {
	written, err := c.Conn.Write(buffer)
	if err != nil {
		c.failed.Store(true)
	}
	return written, err
}

// PublicKey returns the public half of the agent key.
func (s *sshAgentSigner) PublicKey() ssh.PublicKey {
	return s.publicKey
}

// Sign asks the agent for a signature made with the key's own algorithm.
func (s *sshAgentSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return s.SignWithAlgorithm(rand, data, "")
}

// SignWithAlgorithm asks the agent for a signature made with algorithm, which for an RSA key
// may be rsa-sha2-256 or rsa-sha2-512 rather than the SHA-1 ssh-rsa most servers now refuse.
// Implementing ssh.AlgorithmSigner is what lets x/crypto negotiate those algorithms.
func (s *sshAgentSigner) SignWithAlgorithm(_ io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	flags, err := s.signatureFlags(algorithm)
	if err != nil {
		return nil, err
	}

	conn, err := s.dial()
	if err != nil {
		return nil, fmt.Errorf("cannot reach the SSH agent to sign: %w", err)
	}
	defer closeSSHAgentConnection(conn)

	signature, err := agent.NewClient(conn).SignWithFlags(s.publicKey, data, flags)
	if err != nil {
		return nil, fmt.Errorf("the SSH agent did not sign: %w", err)
	}

	return signature, nil
}

// signatureFlags returns the agent protocol flags that request algorithm. The key's own
// algorithm needs none -- for a certificate, the algorithm of the key it certifies -- and
// the protocol can ask for nothing but the two SHA-2 variants of RSA besides.
func (s *sshAgentSigner) signatureFlags(algorithm string) (agent.SignatureFlags, error) {
	keyType := s.publicKey.Type()
	if certificate, isCertificate := s.publicKey.(*ssh.Certificate); isCertificate {
		keyType = certificate.Key.Type()
	}

	switch algorithm {
	case "", keyType:
		return 0, nil
	case ssh.KeyAlgoRSASHA256:
		return agent.SignatureFlagRsaSha256, nil
	case ssh.KeyAlgoRSASHA512:
		return agent.SignatureFlagRsaSha512, nil
	default:
		return 0, fmt.Errorf("%w: %q", errUnsupportedAgentAlgorithm, algorithm)
	}
}

// closeSSHAgentConnection closes a connection to the agent, logging a failure rather than
// returning it: the call that opened the connection has already got what it needed.
func closeSSHAgentConnection(conn net.Conn) {
	if err := conn.Close(); err != nil {
		logger.Debugf("Failed to close SSH agent connection: %v", err)
	}
}
