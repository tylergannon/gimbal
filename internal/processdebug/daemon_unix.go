//go:build darwin || linux

package processdebug

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// selectedDaemon verifies an exact socket match for a single same-user codex
// process. This does not establish ownership of any daemon descendant.
func selectedDaemon() (process, error) {
	before, _, err := snapshotFresh()
	if err != nil {
		return process{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "codex", "app-server", "daemon", "version").Output()
	if err != nil {
		return process{}, err
	}
	var v struct {
		Status     string `json:"status"`
		SocketPath string `json:"socketPath"`
	}
	if json.Unmarshal(b, &v) != nil || v.Status != "running" || v.SocketPath == "" {
		return process{}, errors.New("daemon unavailable")
	}
	path, err := filepath.EvalSymlinks(v.SocketPath)
	if err != nil {
		return process{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return process{}, errors.New("socket unavailable")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	b, err = exec.CommandContext(ctx2, "lsof", "-n", "-a", "-U", "-u", strconv.Itoa(os.Getuid()), "-Fpcn", "--", path).Output()
	if err != nil {
		return process{}, err
	}
	pid := 0
	command := ""
	matches := map[int]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
			command = ""
		case 'c':
			command = line[1:]
		case 'n':
			if line[1:] == path && command == "codex" && pid > 0 {
				matches[pid] = true
			}
		}
	}
	if len(matches) != 1 {
		return process{}, errors.New("socket owner ambiguous")
	}
	for pid = range matches {
		old, ok := before[pid]
		if !ok {
			return process{}, errors.New("daemon identity not observed before socket lookup")
		}
		after, _, err := snapshotFresh()
		if err != nil {
			return process{}, err
		}
		now, ok := after[pid]
		if !ok || !sameProcess(old, now) {
			return process{}, errors.New("daemon identity changed during socket lookup")
		}
		return now, nil
	}
	return process{}, errors.New("socket owner unavailable")
}
