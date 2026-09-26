//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup puts the command in its own process group so the whole tree
// (including backgrounded grandchildren) can be signalled together. pi sets
// `detached: process.platform !== "win32"` on the spawn, which is the same
// group-per-command arrangement.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// KillProcessTree kills a pid and every process in its group, pi's Unix
// killProcessTree: SIGKILL the negative pid, falling back to the pid alone.
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// killProcessTree kills the command's process group, the form os/exec's Cancel
// hook needs. The child is the group leader, so signalling -pid reaches every
// descendant, not just the direct child.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// exitSignalNumber reports the signal that terminated the process, standing in
// for the signal name Node passes alongside a null exit code.
func exitSignalNumber(state *os.ProcessState) (int, bool) {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return 0, false
	}
	return int(status.Signal()), true
}
