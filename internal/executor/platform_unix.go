//go:build !windows

package executor

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func lockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func lockBusy(err error) bool   { return errors.Is(err, unix.EWOULDBLOCK) }
func unlockFile(f *os.File)     { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }

// SyncDir makes a completed file replacement durable on Unix.
func SyncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
func prepareOwner(cmd *exec.Cmd, pipe *os.File) (func(), error) {
	cmd.ExtraFiles = []*os.File{pipe}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return func() {}, nil
}
func startupFile() *os.File { return os.NewFile(3, "startup") }

func openLockFile(path string, create bool) (*os.File, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	return os.OpenFile(path, flags, 0600)
}
