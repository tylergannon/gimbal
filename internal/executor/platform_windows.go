package executor

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
}
func lockBusy(err error) bool { return errors.Is(err, windows.ERROR_LOCK_VIOLATION) }
func unlockFile(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}

// Windows cannot FlushFileBuffers on a directory opened by os.Open. Write syncs
// the file before replacement; do not turn a successful replacement into an
// admission failure by attempting a Unix directory fsync.
func SyncDir(string) error { return nil }

const startupHandleEnv = "GIMBAL_EXECUTOR_STARTUP_HANDLE"

func prepareOwner(cmd *exec.Cmd, pipe *os.File) (func(), error) {
	var handle windows.Handle
	process := windows.CurrentProcess()
	if err := windows.DuplicateHandle(process, windows.Handle(pipe.Fd()), process, &handle, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	cmd.Env = append(os.Environ(), startupHandleEnv+"="+strconv.FormatUint(uint64(handle), 10))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags:              windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(handle)},
	}
	return func() { _ = windows.CloseHandle(handle) }, nil
}
func startupFile() *os.File {
	value := os.Getenv(startupHandleEnv)
	_ = os.Unsetenv(startupHandleEnv)
	handle, err := strconv.ParseUint(value, 10, 64)
	if err != nil || handle == 0 {
		return nil
	}
	// The startup pipe must not leak into the native command or harness.
	_ = windows.SetHandleInformation(windows.Handle(handle), windows.HANDLE_FLAG_INHERIT, 0)
	return os.NewFile(uintptr(handle), "startup")
}

func openLockFile(path string, create bool) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	disposition := uint32(windows.OPEN_EXISTING)
	if create {
		disposition = windows.OPEN_ALWAYS
	}
	// Vacuum removes terminal state while holding its lease, as on Unix.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, disposition, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
