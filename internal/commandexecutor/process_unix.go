//go:build !windows

package commandexecutor

import (
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"
)

const forcedStopError = "process exceeded SIGINT cleanup grace; external resource cleanup unconfirmed"

func runCommandProcess(cmd *exec.Cmd, grace time.Duration) (bool, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var forced atomic.Bool
	cmd.Cancel = func() error {
		signal := syscall.SIGKILL
		if grace > 0 {
			signal = syscall.SIGINT
		}
		err := syscall.Kill(-cmd.Process.Pid, signal)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		if err != nil || grace <= 0 {
			return err
		}
		grace := time.NewTimer(grace)
		defer grace.Stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			if errors.Is(syscall.Kill(-cmd.Process.Pid, 0), syscall.ESRCH) {
				return nil
			}
			select {
			case <-ticker.C:
			case <-grace.C:
				forced.Store(true)
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
	}

	err := cmd.Run()
	// A command owns its process group. Its ordinary children cannot outlive
	// terminal publication, including when the group leader exits first.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return forced.Load(), err
}
