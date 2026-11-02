//go:build linux

package processdebug

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func snapshotSystem() (map[int]process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := map[int]process{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		line := string(b)
		left, right := strings.IndexByte(line, '('), strings.LastIndexByte(line, ')')
		if left < 0 || right <= left {
			continue
		}
		f := strings.Fields(line[right+1:])
		if len(f) < 20 || f[0] == "Z" {
			continue
		}
		ppid, e1 := strconv.Atoi(f[1])
		group, e2 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil {
			continue
		}
		out[pid] = process{PID: pid, Parent: ppid, Group: group, Birth: fmt.Sprintf("ticks:%s", f[19]), Executable: line[left+1 : right]}
	}
	return out, nil
}

// Read only the child’s kernel stat while it is still unreaped.
func initialIdentity(pid int) (process, error) {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return process{}, err
	}
	line := string(b)
	right := strings.LastIndexByte(line, ')')
	if right < 0 {
		return process{}, fmt.Errorf("invalid proc stat")
	}
	f := strings.Fields(line[right+1:])
	if len(f) < 20 || f[19] == "0" {
		return process{}, fmt.Errorf("invalid proc stat")
	}
	return process{PID: pid, Birth: "ticks:" + f[19]}, nil
}
