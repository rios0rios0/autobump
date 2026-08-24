//go:build !windows

package commands

import (
	"errors"
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the command in its own process group and makes
// cancellation kill that group rather than the single process AutoBump started.
//
// [exec.CommandContext]'s default cancellation signals only the direct child. A refresh
// command is allowed to be `sh -c ...`, so the process that outlives the timeout is
// routinely a descendant the shell forked, which the default would leave running with
// the pipe still open. Signalling the negative pid reaches every process in the group.
func configureProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}

		// The group id equals the leader's pid because Setpgid was requested above.
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			// The group is already gone, which is the outcome being asked for.
			return nil
		}

		return err
	}
}
