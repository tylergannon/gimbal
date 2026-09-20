//go:build !unix

package opencode

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
)

func acquireFileLock(context.Context, string) (io.Closer, error) {
	return nil, errors.New("shared OpenCode lifecycle is not supported on this operating system")
}

func prepareServerProcess(*exec.Cmd) {}

func processIdentity(int) (string, bool, error) {
	return "", false, errors.New("shared OpenCode lifecycle is not supported on this operating system")
}

func signalProcessGroup(int, syscall.Signal) error {
	return errors.New("shared OpenCode lifecycle is not supported on this operating system")
}

func waitForProcessExit(context.Context, int, string, time.Duration) bool { return false }
