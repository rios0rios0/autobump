package commands

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	logger "github.com/sirupsen/logrus"

	gitHelpers "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure/helpers"
)

const (
	// windowsSSHProgram is the name of Windows' OpenSSH client, which reads the ssh_config
	// of the Windows profile it runs as rather than any file inside WSL.
	windowsSSHProgram = "ssh.exe"

	// sshWrapperScanLimit bounds how much of a program on PATH is read to recognise a
	// wrapper script around ssh.exe: such a script is a line or two, and a binary is never
	// read past its first bytes.
	sshWrapperScanLimit = 4096
)

// gitRunsWindowsSSH reports whether git on this machine runs Windows' ssh.exe, which is
// what makes a Windows profile's ssh_config the one its host aliases live in. A WSL machine
// whose git runs the Linux ssh reads ~/.ssh/config instead, and applying a Windows file
// there would change hosts and keys git itself never uses.
func gitRunsWindowsSSH() bool {
	command := gitSSHCommandInEffect(os.Getenv, globalCoreSSHCommand())
	if runsWindowsSSH(command, exec.LookPath) {
		return true
	}
	logger.Debugf("git runs %q rather than Windows' ssh.exe, so no Windows profile's ssh_config is read", command)
	return false
}

// globalCoreSSHCommand returns core.sshCommand from the global git config, read the way
// the rest of AutoBump reads it, or an empty string when it cannot be read.
func globalCoreSSHCommand() string {
	config, err := gitHelpers.GetGlobalGitConfig()
	if err != nil {
		logger.Debugf("Cannot read the global git config for core.sshCommand: %v", err)
		return ""
	}
	return gitSSHCommand(config)
}

// gitSSHCommandInEffect returns the command git runs for SSH, picked the way git picks it:
// GIT_SSH_COMMAND, then core.sshCommand, then GIT_SSH, then plain ssh from PATH.
func gitSSHCommandInEffect(getenv func(string) string, coreSSHCommand string) string {
	for _, command := range []string{getenv("GIT_SSH_COMMAND"), coreSSHCommand, getenv("GIT_SSH")} {
		if strings.TrimSpace(command) != "" {
			return command
		}
	}
	return "ssh"
}

// runsWindowsSSH reports whether an SSH command runs Windows' ssh.exe: it names ssh.exe, or
// its program, found on PATH, is ssh.exe -- through a symlink, too -- or a script that
// calls it, such as an ssh wrapper that runs `ssh.exe "$@"`.
func runsWindowsSSH(command string, lookPath func(string) (string, error)) bool {
	if namesWindowsSSH(command) {
		return true
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	program, err := lookPath(strings.Trim(fields[0], `"'`))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(program); resolveErr == nil {
		program = resolved
	}
	if strings.EqualFold(filepath.Base(program), windowsSSHProgram) {
		return true
	}
	return isWindowsSSHWrapper(program)
}

// isWindowsSSHWrapper reports whether a program is a script that calls ssh.exe.
func isWindowsSSHWrapper(program string) bool {
	file, err := os.Open(program)
	if err != nil {
		return false
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Debugf("Failed to close %s: %v", program, closeErr)
		}
	}()

	head, err := io.ReadAll(io.LimitReader(file, sshWrapperScanLimit))
	if err != nil {
		return false
	}
	return bytes.HasPrefix(head, []byte("#!")) && namesWindowsSSH(string(head))
}

// namesWindowsSSH reports whether text names Windows' ssh.exe.
func namesWindowsSSH(text string) bool {
	return strings.Contains(strings.ToLower(text), windowsSSHProgram)
}

// windowsProfilesOnPath returns the Windows profile directories PATH reaches into. WSL
// appends the Windows PATH by default, and its per-user entries (AppData\Local\Microsoft\
// WindowsApps, among others) sit under the profile of the Windows user running WSL -- the
// user ssh.exe runs as, and so the one profile whose ssh_config it reads.
func windowsProfilesOnPath(automountRoot, pathList string) []string {
	root := strings.TrimSuffix(automountRoot, "/") + "/"
	var profiles []string
	for _, entry := range filepath.SplitList(pathList) {
		rest, found := strings.CutPrefix(entry, root)
		if !found {
			continue
		}
		parts := strings.SplitN(rest, "/", 4) //nolint:mnd // drive, "Users", profile, and the rest
		if len(parts) < 3 || !isDriveLetterName(parts[0]) || !strings.EqualFold(parts[1], "Users") ||
			parts[2] == "" {
			continue
		}
		profile := filepath.Join(root, parts[0], parts[1], parts[2])
		if !slices.ContainsFunc(profiles, func(known string) bool { return strings.EqualFold(known, profile) }) {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

// currentWindowsProfileSources keeps the sources of the profiles PATH reaches into, which is
// the profile ssh.exe reads. When PATH reaches into none -- appendWindowsPath turned off in
// /etc/wsl.conf -- the current user cannot be told apart, and every profile is kept.
func currentWindowsProfileSources(sources []sshConfigSource, profiles []string) []sshConfigSource {
	if len(profiles) == 0 {
		return sources
	}
	return slices.DeleteFunc(slices.Clone(sources), func(source sshConfigSource) bool {
		return !slices.ContainsFunc(
			profiles,
			func(profile string) bool { return strings.EqualFold(profile, source.Home) },
		)
	})
}
