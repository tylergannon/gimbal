//go:build darwin

package processdebug

import (
	"fmt"
	"golang.org/x/sys/unix"
	"strings"
)

func snapshotSystem() (map[int]process, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	out := make(map[int]process, len(processes))
	for _, p := range processes {
		if p.Proc.P_stat == 5 {
			continue
		}
		pid := int(p.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		out[pid] = process{PID: pid, Parent: int(p.Eproc.Ppid), Group: int(p.Eproc.Pgid), Birth: fmt.Sprintf("%d.%06d", p.Proc.P_starttime.Sec, p.Proc.P_starttime.Usec), Executable: strings.TrimRight(string(p.Proc.P_comm[:]), "\x00")}
	}
	return out, nil
}

// This query runs at the launch boundary before the caller starts Wait.
func initialIdentity(pid int) (process, error) {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return process{}, err
	}
	if p.Proc.P_starttime.Sec == 0 && p.Proc.P_starttime.Usec == 0 {
		return process{}, fmt.Errorf("birth identity unavailable")
	}
	return process{PID: int(p.Proc.P_pid), Birth: fmt.Sprintf("%d.%06d", p.Proc.P_starttime.Sec, p.Proc.P_starttime.Usec)}, nil
}
