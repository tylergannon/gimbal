//go:build darwin || linux

package gimbal

import (
	"errors"
	"os/exec"
	"syscall"
)

func prepareServiceProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func terminateServiceGroup(pid int) error {
	return serviceGroupSignal(pid, syscall.SIGTERM)
}

func killServiceGroup(pid int) error {
	return serviceGroupSignal(pid, syscall.SIGKILL)
}

func serviceGroupSignal(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func serviceGroupAlive(pid int) (bool, error) {
	err := syscall.Kill(-pid, 0)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return true, err
	}
}
