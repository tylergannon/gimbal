//go:build !windows

package codex

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryStartupLock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errStartupLockBusy
	}
	return err
}
func unlockStartup(f *os.File) { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }
