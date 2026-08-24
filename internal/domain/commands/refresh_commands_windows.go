//go:build windows

package commands

import "os/exec"

// configureProcessGroup is a no-op on Windows, which has no process groups in the POSIX
// sense. Cancellation keeps exec.CommandContext's default of killing the process
// AutoBump started; a surviving descendant is bounded by WaitDelay instead.
func configureProcessGroup(_ *exec.Cmd) {}
