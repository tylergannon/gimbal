//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package agy

import (
	"os"
	"os/exec"
)

func configureProcess(*exec.Cmd) {}

func signalProcess(process *os.Process, signal os.Signal) error {
	return process.Signal(signal)
}

func killProcess(process *os.Process) {
	_ = process.Kill()
}
