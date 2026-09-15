//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package agy

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalProcess(process *os.Process, signal os.Signal) error {
	native, ok := signal.(syscall.Signal)
	if !ok {
		return process.Signal(signal)
	}
	return syscall.Kill(-process.Pid, native)
}

func killProcess(process *os.Process) {
	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
}
