//go:build !windows

package directory

import (
	"errors"
	"os"

	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/unix"
)

func lockWriter(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return runstore.ErrBusy
	}
	return err
}
