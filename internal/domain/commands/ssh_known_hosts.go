package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	logger "github.com/sirupsen/logrus"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

const (
	// sshKnownHostsEnv names the environment variable OpenSSH and go-git honour as an
	// explicit known_hosts location. When it is set it wins over every other source,
	// matching go-git's own default so the two never disagree about what the operator
	// asked for.
	sshKnownHostsEnv = "SSH_KNOWN_HOSTS"

	// systemKnownHostsPath is the machine-wide known_hosts OpenSSH consults beside the
	// user's own file.
	systemKnownHostsPath = "/etc/ssh/ssh_known_hosts"

	// wslConfPath is the WSL configuration file whose [automount] section can move or
	// disable the mount point Windows drives appear under.
	wslConfPath = "/etc/wsl.conf"

	// defaultAutomountRoot is where WSL mounts the Windows drives when /etc/wsl.conf
	// does not say otherwise, which puts the Windows profiles' known_hosts under
	// /mnt/c/Users/<name>/.ssh/.
	defaultAutomountRoot = "/mnt/"
)

// The discovered callback is built once and reused: hostKeyCallback runs for every SSH
// auth method a release collects, and neither the discovery nor its warnings are worth
// repeating per method within one CLI run.
var (
	hostKeyCallbackOnce sync.Once           //nolint:gochecknoglobals // memoization guard for cachedHostKeyCb
	cachedHostKeyCb     ssh.HostKeyCallback //nolint:gochecknoglobals // memoized per process by hostKeyCallbackOnce
)

// hostKeyCallback returns an ssh.HostKeyCallback that validates against every known_hosts
// file this host offers, or nil when there is none, letting go-git fall back to its own
// default host key verification.
//
// The result is memoized for the lifetime of the process: discovery stats a handful of
// paths and, under WSL, globs the Windows profiles, none of which changes mid-run.
func hostKeyCallback() ssh.HostKeyCallback {
	hostKeyCallbackOnce.Do(func() {
		cachedHostKeyCb = buildHostKeyCallback(discoverKnownHostsFiles())
	})
	return cachedHostKeyCb
}

// SSHHostKeyCallback returns a host key callback for gitforge's default SSH agent
// fallback — the push and remote listing gitforge performs with no explicit auth method.
// Without a callback that fallback verifies hosts through go-git's own lookup, which
// knows nothing about the Windows-side files discovered here and fails when
// ~/.ssh/known_hosts is absent. Handing gitforge this callback keeps every SSH connection
// AutoBump makes verifying against the same known_hosts.
//
// The returned callback resolves discovery lazily, on the first host key check, so a run
// that never touches SSH (autobump version, self-update) neither pays for the discovery
// nor emits its warnings. hostKeyCallback memoizes, so repeated checks are cheap. When no
// known_hosts file exists at all it fails the check with a clear message rather than
// silently accepting any host key.
func SSHHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		cb := hostKeyCallback()
		if cb == nil {
			return fmt.Errorf(
				"no known_hosts file found to verify host %q: set %s or create ~/.ssh/known_hosts",
				hostname, sshKnownHostsEnv,
			)
		}
		return cb(hostname, remote, key)
	}
}

// buildHostKeyCallback turns discovered known_hosts files into a verification callback.
// An empty list, or files skeema/knownhosts cannot parse, yield nil so go-git produces its
// own (actionable) error rather than a connection AutoBump silently never verified.
func buildHostKeyCallback(files []string) ssh.HostKeyCallback {
	if len(files) == 0 {
		warnNoKnownHosts()
		return nil
	}

	cb, err := knownhosts.New(files...)
	if err != nil {
		logger.Warnf(
			"Cannot parse known_hosts from %s: %v, deferring to go-git default host key verification",
			strings.Join(files, ", "), err,
		)
		return nil
	}

	logger.Debugf("Loaded SSH host keys from %s", strings.Join(files, ", "))
	return ssh.HostKeyCallback(cb)
}

// warnNoKnownHosts tells the operator the search came up empty, naming the WSL-specific
// places that were included when this host is WSL: the file they expect AutoBump to read
// lives on the Windows side, and the message has to say that side was searched too before
// it can send them anywhere useful.
func warnNoKnownHosts() {
	places := "$" + sshKnownHostsEnv + ", ~/.ssh/known_hosts and " + systemKnownHostsPath
	if runningUnderWSL() {
		places += " and the Windows profiles under the WSL automount root"
	}
	logger.Warnf(
		"Cannot find any known_hosts file (searched %s), deferring to go-git default host key verification",
		places,
	)
}

// discoverKnownHostsFiles returns the known_hosts files this host offers, in precedence
// order and without the ones that do not exist:
//
//   - SSH_KNOWN_HOSTS, when the operator set it (a [filepath.SplitList] of paths),
//     alone — an explicit location replaces discovery, exactly as it does in go-git;
//   - the user's ~/.ssh/known_hosts;
//   - the machine-wide /etc/ssh/ssh_known_hosts;
//   - under WSL, every Windows profile's known_hosts reachable through the automount
//     root, because a WSL setup that delegates SSH to Windows (ssh.exe, Git for Windows)
//     keeps the host keys there and has no reason to duplicate them into the WSL home.
//
// The Windows files are merged with the WSL ones rather than replacing them, which is
// what OpenSSH does across its own system and user files; a host trusted on either side
// verifies. Their CRLF line endings are handled by the parser, which trims every line.
func discoverKnownHostsFiles() []string {
	if env := os.Getenv(sshKnownHostsEnv); env != "" {
		return existingFiles(filepath.SplitList(env))
	}

	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".ssh", "known_hosts"))
	} else {
		logger.Warnf("Cannot determine the home directory while looking for known_hosts: %v", err)
	}
	candidates = append(candidates, systemKnownHostsPath)

	if runningUnderWSL() {
		candidates = append(candidates, windowsKnownHostsFiles()...)
	}

	return existingFiles(candidates)
}

// existingFiles keeps the paths that exist and are regular files, in order. The filter
// is what lets discovery hand candidates to knownhosts.New, which fails on any file it
// cannot open rather than skipping it.
func existingFiles(paths []string) []string {
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		//nolint:gosec // paths come from SSH_KNOWN_HOSTS, the home directory or a fixed glob, all trusted
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	return files
}

// windowsKnownHostsFiles returns the known_hosts of every Windows user profile mounted
// under the WSL automount root. All profiles are merged rather than guessing which one is
// "the" user's: resolving that would mean spawning a Windows process (cmd.exe, wslvar)
// for a path the glob already found, and host keys trusted by another local profile are
// not a meaningful threat on a single-user machine.
func windowsKnownHostsFiles() []string {
	root, automountEnabled := wslAutomountRoot()
	if !automountEnabled {
		logger.Debug("WSL automount is disabled in /etc/wsl.conf, skipping Windows known_hosts discovery")
		return nil
	}
	return windowsKnownHostsFilesUnder(root)
}

// windowsKnownHostsFilesUnder globs the Windows profile layout under an automount root:
// <root><drive letter>/Users/*/.ssh/known_hosts. Reserved entries (Public, Default, …)
// need no exclusion — they never contain the file.
func windowsKnownHostsFilesUnder(root string) []string {
	drives, err := os.ReadDir(root)
	if err != nil {
		logger.Debugf("Cannot read the WSL automount root %s: %v", root, err)
		return nil
	}

	var files []string
	for _, drive := range drives {
		if !drive.IsDir() || !isDriveLetterName(drive.Name()) {
			continue
		}
		matches, globErr := filepath.Glob(filepath.Join(root, drive.Name(), "Users", "*", ".ssh", "known_hosts"))
		if globErr != nil {
			logger.Debugf("Cannot glob the Windows known_hosts under %s: %v", drive.Name(), globErr)
			continue
		}
		files = append(files, matches...)
	}
	return files
}

// isDriveLetterName reports whether a directory name under the automount root is a
// Windows drive letter. WSL mounts each drive as a single-letter directory; anything
// else there (/mnt/wsl, /mnt/wslg, stray user directories) is not a drive.
func isDriveLetterName(name string) bool {
	if len(name) != 1 {
		return false
	}
	c := name[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// wslAutomountRoot reads where WSL mounts the Windows drives. A missing or unreadable
// /etc/wsl.conf means the defaults are in force: automount enabled at /mnt/.
func wslAutomountRoot() (string, bool) {
	data, err := os.ReadFile(wslConfPath)
	if err != nil {
		return defaultAutomountRoot, true
	}
	return parseWSLAutomountRoot(data)
}

// parseWSLAutomountRoot extracts the [automount] root and enabled settings from
// wsl.conf content. It is an INI subset on purpose — wsl.conf has no nesting, no
// continuation and no escaping worth a dependency: a section header switches scope,
// a comment (# or ;) is skipped, and inside [automount] the root and enabled keys are
// read, values optionally quoted the way the WSL documentation shows them.
func parseWSLAutomountRoot(data []byte) (string, bool) {
	root, enabled := defaultAutomountRoot, true
	inAutomount := false

	for rawLine := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inAutomount = strings.EqualFold(strings.TrimSuffix(line, "]"), "[automount")
			continue
		}
		if !inAutomount {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		switch strings.ToLower(strings.TrimSpace(key)) {
		case "enabled":
			enabled = !strings.EqualFold(value, "false")
		case "root":
			if value != "" {
				root = value
			}
		}
	}

	return root, enabled
}
