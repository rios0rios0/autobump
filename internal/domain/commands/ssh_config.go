package commands

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/kevinburke/ssh_config"
	logger "github.com/sirupsen/logrus"
)

const (
	// systemSSHConfigPath is the machine-wide ssh_config OpenSSH reads after the user's own.
	systemSSHConfigPath = "/etc/ssh/ssh_config"

	// defaultSSHPort is the port an SSH remote is dialed on when neither its URL nor
	// ssh_config names one. go-git parses an scp-like remote (git@host:path), which cannot
	// carry a port, to this port, so a remote on it is treated as naming none.
	defaultSSHPort = 22
)

// The discovered ssh_config files are decoded once and reused: the SSH transport resolves
// the remote's host on every connection a release opens, and neither the discovery nor its
// warnings are worth repeating per connection within one CLI run.
var (
	sshConfigOnce   sync.Once       //nolint:gochecknoglobals // memoization guard for cachedSSHConfig
	cachedSSHConfig *sshConfigFiles //nolint:gochecknoglobals // memoized per process by sshConfigOnce
)

// sshConfigSource names one ssh_config file and the home directory its paths are relative
// to. Windows marks a Windows profile's file, whose paths ssh.exe resolves on the Windows
// side: `~` there is the profile, and a drive letter is a drive WSL mounts under the
// automount root. System marks the machine-wide file, which a distribution ships and the
// operator seldom edits, so a failure to read it is reported only at debug level.
type sshConfigSource struct {
	Path    string
	Home    string
	Windows bool
	System  bool
}

// sshConfigFile is a decoded ssh_config file and the source it was read from.
type sshConfigFile struct {
	source sshConfigSource
	config *ssh_config.Config
}

// sshConfigFiles is the ordered list of ssh_config files AutoBump reads. They are consulted
// as one configuration, the way OpenSSH consults its user and system files: for each
// keyword the first file that sets it wins, and IdentityFile accumulates across them.
type sshConfigFiles struct {
	files         []sshConfigFile
	automountRoot string
}

// sshHostSettings is what the ssh_config files say about one host, limited to the keywords
// AutoBump applies. The zero value means the files say nothing: the host is dialed as the
// remote URL names it, and the agent's keys are offered as the agent lists them.
type sshHostSettings struct {
	// HostName is the host to dial instead of the alias, or empty when none is set.
	HostName string

	// Port is the port to dial, or 0 when none is set.
	Port int

	// IdentityFiles are the IdentityFile paths, expanded, in the order they were found.
	IdentityFiles []string

	// IdentitiesOnly restricts the keys offered to the ones IdentityFiles name.
	IdentitiesOnly bool
}

// sshTokens are the values the %-tokens of an ssh_config value expand to.
type sshTokens struct {
	home       string
	host       string
	remoteUser string
}

// sshConfig returns the ssh_config files this host offers, decoded once per process.
func sshConfig() *sshConfigFiles {
	sshConfigOnce.Do(func() {
		automountRoot, sources := discoverSSHConfigSources()
		cachedSSHConfig = loadSSHConfigFiles(sources, automountRoot)
	})
	return cachedSSHConfig
}

// discoverSSHConfigSources returns the ssh_config files AutoBump reads, in precedence
// order, together with the WSL automount root their Windows paths are mapped into -- an
// empty root when no Windows profile's file is among them:
//
//   - the user's ~/.ssh/config;
//   - the machine-wide /etc/ssh/ssh_config;
//   - under WSL, when git runs Windows' ssh.exe (gitRunsWindowsSSH), the .ssh/config of the
//     Windows profile PATH reaches into, which is the file ssh.exe reads and where such a
//     setup defines its host aliases. When PATH reaches into no profile, every profile's.
//
// A WSL machine whose git runs the Linux ssh reads no Windows file, so AutoBump resolves a
// host there exactly as git does. Files that do not exist are kept here and skipped when the
// files are loaded.
func discoverSSHConfigSources() (string, []sshConfigSource) {
	var sources []sshConfigSource
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warnf("Cannot determine the home directory while looking for ssh_config: %v", err)
	} else {
		sources = append(sources, sshConfigSource{Path: filepath.Join(home, ".ssh", "config"), Home: home})
	}
	sources = append(sources, sshConfigSource{Path: systemSSHConfigPath, Home: home, System: true})

	if !runningUnderWSL() || !gitRunsWindowsSSH() {
		return "", sources
	}
	automountRoot, automountEnabled := wslAutomountRoot()
	if !automountEnabled {
		logger.Debug("WSL automount is disabled in /etc/wsl.conf, skipping Windows ssh_config discovery")
		return "", sources
	}

	windowsSources := currentWindowsProfileSources(
		windowsSSHConfigSourcesUnder(automountRoot), windowsProfilesOnPath(automountRoot, os.Getenv("PATH")),
	)
	return automountRoot, append(sources, windowsSources...)
}

// windowsSSHConfigSourcesUnder returns the ssh_config of every Windows profile mounted under
// an automount root, each with its profile directory -- <root><drive>/Users/<profile> -- as
// the home its paths are relative to.
func windowsSSHConfigSourcesUnder(automountRoot string) []sshConfigSource {
	paths := windowsProfileSSHFilesUnder(automountRoot, "config")
	sources := make([]sshConfigSource, 0, len(paths))
	for _, path := range paths {
		profile := filepath.Dir(filepath.Dir(path))
		sources = append(sources, sshConfigSource{Path: path, Home: profile, Windows: true})
	}
	return sources
}

// loadSSHConfigFiles decodes the sources that exist, in order. A file that cannot be read
// or parsed -- kevinburke/ssh_config accepts no `Match` criterion but `host` and `all`, so
// `Match exec` or `Match final` fail it -- is skipped rather than failing the run: its
// aliases are lost, the other files' are not. The operator's own file is reported with a
// warning; the system file, which a distribution ships, only at debug level, since its
// failure costs nothing the operator configured and recurs on every run.
//
// Each file is decoded the way OpenSSH reads a user's file, so a relative Include in it is
// looked up under ~/.ssh. The system file's own Include is absolute on the distributions
// that ship one, and a Windows profile's file rarely has any.
func loadSSHConfigFiles(sources []sshConfigSource, automountRoot string) *sshConfigFiles {
	files := make([]sshConfigFile, 0, len(sources))
	for _, source := range sources {
		report := logger.Warnf
		if source.System {
			report = logger.Debugf
		}

		data, err := os.ReadFile(source.Path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				report("Cannot read ssh_config %s: %v, ignoring it", source.Path, err)
			}
			continue
		}

		config, err := ssh_config.DecodeBytes(data)
		if err != nil {
			report("Cannot parse ssh_config %s: %v, ignoring the host aliases it defines", source.Path, err)
			continue
		}

		logger.Debugf("Loaded SSH client configuration from %s", source.Path)
		files = append(files, sshConfigFile{source: source, config: config})
	}

	return &sshConfigFiles{files: files, automountRoot: automountRoot}
}

// resolve returns what the files say about the host alias a remote URL names; remoteUser
// is the user the URL names, which %r stands for. HostName, Port and IdentitiesOnly come
// from the first file that sets them, and IdentityFile accumulates across the files, each
// path expanded against the home of the file that names it.
func (c *sshConfigFiles) resolve(alias, remoteUser string) sshHostSettings {
	var settings sshHostSettings
	if c == nil || alias == "" {
		return settings
	}

	if hostName := c.first(alias, "HostName"); hostName != "" {
		settings.HostName = expandSSHTokens(hostName, sshTokens{host: alias, remoteUser: remoteUser})
	}
	if port, err := strconv.Atoi(c.first(alias, "Port")); err == nil && port > 0 {
		settings.Port = port
	}
	settings.IdentitiesOnly = strings.EqualFold(c.first(alias, "IdentitiesOnly"), "yes")

	host := alias
	if settings.HostName != "" {
		host = settings.HostName
	}
	for _, file := range c.files {
		values, err := file.config.GetAll(alias, "IdentityFile")
		if err != nil {
			logger.Debugf("Cannot read IdentityFile for %q from %s: %v", alias, file.source.Path, err)
			continue
		}
		for _, value := range values {
			settings.IdentityFiles = append(
				settings.IdentityFiles,
				c.identityPath(
					file.source,
					value,
					sshTokens{home: file.source.Home, host: host, remoteUser: remoteUser},
				),
			)
		}
	}

	return settings
}

// first returns the value the first file that sets keyword for alias gives it, or an
// empty string when none does.
func (c *sshConfigFiles) first(alias, keyword string) string {
	for _, file := range c.files {
		value, err := file.config.Get(alias, keyword)
		if err != nil {
			logger.Debugf("Cannot read %s for %q from %s: %v", keyword, alias, file.source.Path, err)
			continue
		}
		if value != "" {
			return value
		}
	}
	return ""
}

// identityPath turns an IdentityFile value into a path AutoBump can open. A leading ~ and
// the tokens are expanded against the home of the file that names the path, and a Windows
// profile's file is read the way ssh.exe reads it: backslashes separate directories, and a
// drive letter names a drive WSL mounts under the automount root.
func (c *sshConfigFiles) identityPath(source sshConfigSource, value string, tokens sshTokens) string {
	path := value
	if source.Windows {
		path = strings.ReplaceAll(path, `\`, "/")
	}

	if path == "~" {
		path = source.Home
	} else if rest, found := strings.CutPrefix(path, "~/"); found {
		path = filepath.Join(source.Home, rest)
	}
	path = expandSSHTokens(path, tokens)

	if source.Windows {
		path = windowsDrivePathUnder(path, c.automountRoot)
	}
	return path
}

// target returns the endpoint to dial for ep: the HostName and Port the alias names, when
// it names them, and ep itself otherwise. A port written in the URL wins over Port, as
// `ssh -p` wins over the file; an scp-like URL cannot carry one, and go-git parses it to
// 22, so a remote on 22 counts as naming none.
func (s sshHostSettings) target(ep *transport.Endpoint) *transport.Endpoint {
	if s.HostName == "" && s.Port == 0 {
		return ep
	}

	resolved := *ep
	if s.HostName != "" {
		resolved.Host = s.HostName
	}
	if s.Port != 0 && (ep.Port <= 0 || ep.Port == defaultSSHPort) {
		resolved.Port = s.Port
	}
	return &resolved
}

// sshHostWithPort is the address go-git dials for ep -- 22 when the endpoint has no port --
// and the name known_hosts records the host under.
func sshHostWithPort(ep *transport.Endpoint) string {
	port := ep.Port
	if port <= 0 {
		port = defaultSSHPort
	}
	return net.JoinHostPort(ep.Host, strconv.Itoa(port))
}

// expandSSHTokens expands the %-tokens of an ssh_config value that AutoBump can know: %d
// (the home directory), %h (the host), %r (the remote user) and %% (a literal %). Any other
// token is kept as written, which leaves a path naming a file that does not exist rather
// than one AutoBump guessed.
func expandSSHTokens(value string, tokens sshTokens) string {
	if !strings.Contains(value, "%") {
		return value
	}

	replacements := map[byte]string{
		'%': "%",
		'd': tokens.home,
		'h': tokens.host,
		'r': tokens.remoteUser,
	}

	var expanded strings.Builder
	rest := value
	for {
		before, after, found := strings.Cut(rest, "%")
		expanded.WriteString(before)
		if !found {
			return expanded.String()
		}
		if after == "" {
			expanded.WriteString("%")
			return expanded.String()
		}

		if replacement, known := replacements[after[0]]; known {
			expanded.WriteString(replacement)
		} else {
			expanded.WriteByte('%')
			expanded.WriteByte(after[0])
		}
		rest = after[1:]
	}
}

// windowsDrivePathUnder maps a Windows drive path (C:/Users/...) to where WSL mounts that
// drive under automountRoot (/mnt/c/Users/...). Any other path is returned unchanged.
func windowsDrivePathUnder(path, automountRoot string) string {
	if automountRoot == "" || len(path) < len("C:/") || path[1] != ':' || path[2] != '/' ||
		!isDriveLetterName(path[:1]) {
		return path
	}
	return filepath.Join(automountRoot, strings.ToLower(path[:1]), path[2:])
}

// readsWindowsProfiles reports whether a Windows profile's ssh_config is among the places
// searched, which is so exactly when the files were loaded with an automount root.
func (c *sshConfigFiles) readsWindowsProfiles() bool {
	return c != nil && c.automountRoot != ""
}

// describeSSHConfigSearch names the ssh_config files AutoBump reads, for a message that has
// to send the operator to one of them. Under WSL it says whether the Windows profile's file
// is among them, and when it is not, what would make it so: an alias defined only there is
// the usual reason a host does not resolve.
func describeSSHConfigSearch(isWSL, windowsProfiles bool) string {
	switch {
	case windowsProfiles:
		return "~/.ssh/config, " + systemSSHConfigPath + " and the Windows profile's .ssh/config"
	case isWSL:
		return "~/.ssh/config and " + systemSSHConfigPath + " (the Windows profile's .ssh/config is read only " +
			"when git runs Windows' ssh.exe, through GIT_SSH_COMMAND, core.sshCommand, GIT_SSH or an ssh on PATH)"
	default:
		return "~/.ssh/config and " + systemSSHConfigPath
	}
}
