package directory

import (
	"errors"
	"os"

	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/windows"
)

func lockWriter(file *os.File) error {
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return runstore.ErrBusy
	}
	return err
}
