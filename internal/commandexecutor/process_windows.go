package commandexecutor

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const forcedStopError = "Windows command terminated forcibly; graceful interrupt is unavailable"

func runCommandProcess(cmd *exec.Cmd, _ time.Duration) (bool, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return false, err
	}
	// Suspend before user code runs, so descendants inherit the job from their
	// first instruction. The independent command owner itself is outside it.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP}
	var mu sync.Mutex
	var forced atomic.Bool
	cmd.Cancel = func() error {
		mu.Lock()
		defer mu.Unlock()
		err := windows.TerminateJobObject(job, 1)
		if err == nil {
			forced.Store(true)
		}
		return err
	}
	mu.Lock()
	err = cmd.Start()
	if err == nil {
		var process windows.Handle
		process, err = windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err == nil {
			err = windows.AssignProcessToJobObject(job, process)
			_ = windows.CloseHandle(process)
		}
		if err == nil {
			err = resumeCommand(uint32(cmd.Process.Pid))
		}
	}
	mu.Unlock()
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return forced.Load(), err
	}
	err = cmd.Wait()
	// Closing the job kills remaining ordinary descendants before result.json.
	return forced.Load(), err
}

func resumeCommand(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		return err
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("suspended command %d has no primary thread", pid)
	}
	return err
}
