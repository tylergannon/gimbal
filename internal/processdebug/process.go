// Package processdebug provides temporary, best-effort process observation.
package processdebug

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxOwners = 128
const maxProcesses = 1024
const maxLog = 1 << 20

type process struct {
	PID        int    `json:"pid"`
	Parent     int    `json:"parent"`
	Group      int    `json:"group"`
	Birth      string `json:"birth"`
	Executable string `json:"executable"`
}
type event struct {
	Time       time.Time `json:"time"`
	Event      string    `json:"event"`
	Owner      string    `json:"owner,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	PID        int       `json:"pid,omitempty"`
	Process    *process  `json:"process,omitempty"`
	AgeSeconds int       `json:"owner_end_age_seconds,omitempty"`
	Count      int       `json:"count,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}
type logger struct {
	mu      sync.Mutex
	path    string
	size    int64
	dropped atomic.Uint64
}

var once sync.Once
var output *logger
var slots = make(chan struct{}, maxOwners)
var droppedOwners atomic.Uint64

func getLogger() *logger {
	once.Do(func() {
		dir, err := os.UserCacheDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: cache directory unavailable")
			return
		}
		dir = filepath.Join(dir, "gimbal", "process-debug")
		if err = os.MkdirAll(dir, 0700); err != nil {
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: cannot create private directory")
			return
		}
		if err = os.Chmod(dir, 0700); err != nil {
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: cannot secure directory")
			return
		}
		// Bound old runs as well as each run’s rotating files. Active older logs
		// may be unlinked under extreme concurrency; observation is best effort.
		pruneLogs(dir)
		f, err := os.CreateTemp(dir, "process-*.jsonl")
		if err != nil {
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: cannot create log")
			return
		}
		name := f.Name()
		_ = f.Close()
		output = &logger{path: name}
		fmt.Fprintf(os.Stderr, "gimbal process diagnostics: %s\n", f.Name())
	})
	return output
}

func pruneLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		name     string
		modified time.Time
	}
	var files []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "process-") || !(strings.HasSuffix(e.Name(), ".jsonl") || strings.HasSuffix(e.Name(), ".jsonl.1")) {
			continue
		}
		info, err := e.Info()
		if err == nil && info.Mode().IsRegular() {
			files = append(files, item{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.After(files[j].modified) })
	for _, f := range files[min(30, len(files)):] {
		if os.Remove(filepath.Join(dir, f.name)) == nil {
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: old log removed to bound retention")
		}
	}
}

func (l *logger) write(e event) {
	e.Time = time.Now().UTC()
	b, err := json.Marshal(e)
	if err != nil {
		l.dropped.Add(1)
		return
	}
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(b)) > maxLog {
		_ = os.Remove(l.path + ".1")
		if os.Rename(l.path, l.path+".1") != nil {
			l.dropped.Add(1)
			fmt.Fprintln(os.Stderr, "gimbal process diagnostics: log rotation failed")
			return
		}
		l.size = 0
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		l.dropped.Add(1)
		fmt.Fprintln(os.Stderr, "gimbal process diagnostics: log unavailable")
		return
	}
	n, writeErr := f.Write(b)
	closeErr := f.Close()
	l.size += int64(n)
	if writeErr != nil || closeErr != nil {
		l.dropped.Add(1)
		fmt.Fprintln(os.Stderr, "gimbal process diagnostics: log write failed")
	}

}

// Owner observes a directly launched process and the descendants seen while it
// runs. Its calls never wait for enumeration or disk I/O.
type Owner struct {
	end               chan struct{}
	reap              chan struct{}
	endOnce, reapOnce sync.Once
}

func Start(kind, id string, pid int) *Owner {
	return start(kind, id, pid, false, "")
}

// Candidates observes the selected shared Codex daemon without attributing its children to a turn.
func Candidates(sessionID string) *Owner { return start("codex_shared_daemon", sessionID, 0, true, "") }

func Executor(id string, pid int, result string) *Owner {
	return start("executor", id, pid, false, result)
}
func ExecutorCandidates(id, result string) *Owner {
	return start("codex_shared_daemon", id, 0, true, result)
}

func start(kind, id string, pid int, shared bool, result string) *Owner {
	if os.Getenv("GIMBAL_DEBUG_PROCESSES") != "1" || (pid <= 0 && !shared) {
		return nil
	}
	select {
	case slots <- struct{}{}:
	default:
		droppedOwners.Add(1)
		return nil
	}
	o := &Owner{end: make(chan struct{}), reap: make(chan struct{})}
	go func() { defer func() { <-slots }(); observe(o, kind, id, pid, shared, result) }()
	return o
}
func (o *Owner) Reaped() {
	if o != nil {
		o.reapOnce.Do(func() { close(o.reap) })
	}
}
func (o *Owner) End() {
	if o != nil {
		o.endOnce.Do(func() { close(o.end) })
	}
}

func observe(o *Owner, kind, id string, pid int, shared bool, result string) {
	l := getLogger()
	if l == nil {
		return
	}
	// IDs are existing opaque operation identifiers; do not emit paths or inputs.
	id = safeID(id)
	kind = safeID(kind)
	l.write(event{Event: "start", Owner: id, Kind: kind, PID: pid, Detail: "session and turn correlation unavailable unless encoded in owner; sampled ancestry may be incomplete"})
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		l.write(event{Event: "unavailable", Owner: id, Detail: "process enumeration unsupported on this platform"})
		return
	}
	if shared {
		var err error
		pid, err = selectedDaemon()
		if err != nil {
			l.write(event{Event: "unavailable", Owner: id, Detail: "shared daemon identity unavailable"})
			return
		}
	}
	known := map[int]process{}
	rootAttempted := false
	ended := time.Time{}
	reap := o.reap
	end := o.end
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	sample := func() {
		snapshot, err := snapshot()
		if err != nil {
			l.write(event{Event: "unavailable", Owner: id, Detail: "process enumeration failed"})
			return
		}
		if !rootAttempted {
			rootAttempted = true
			p, ok := snapshot[pid]
			if !shared || !ok {
				snapshot, err = snapshotFresh()
				if err != nil {
					l.write(event{Event: "unavailable", Owner: id, Detail: "process enumeration failed"})
					return
				}
				p, ok = snapshot[pid]
			}
			if !shared {
				select {
				case <-o.reap:
					ok = false
				default:
				}
			}
			if ok {
				known[pid] = p
				label := "owned"
				if shared {
					label = "shared_daemon"
				}
				l.write(event{Event: label, Owner: id, Process: &p})
			} else {
				l.write(event{Event: "unavailable", Owner: id, Detail: "launch identity not observed; descendants cannot be attributed"})
			}
		}
		// Add only children whose currently observed parent still has the same birth.
		for changed := true; changed; {
			changed = false
			for n, p := range snapshot {
				if _, ok := known[n]; ok {
					continue
				}
				parent, ok := known[p.Parent]
				if !ok {
					continue
				}
				current, ok := snapshot[p.Parent]
				if !ok || !sameProcess(current, parent) {
					continue
				}
				if len(known) >= maxProcesses {
					l.dropped.Add(1)
					break
				}
				known[n] = p
				changed = true
				label := "owned"
				if shared {
					label = "candidate"
				}
				l.write(event{Event: label, Owner: id, Process: &p})
			}
		}
		if !ended.IsZero() {
			count := 0
			for n, p := range known {
				if cur, ok := snapshot[n]; ok && sameProcess(cur, p) && (!shared || n != pid) {
					count++
					q := cur
					label := "suspect_survivor"
					if shared {
						label = "candidate_survivor"
					}
					l.write(event{Event: label, Owner: id, Process: &q, AgeSeconds: int(time.Since(ended).Seconds())})
				}
			}
			label := "survivor_count"
			detail := "missed ancestry unknown"
			if shared {
				label = "candidate_count"
				detail = "shared daemon children may belong to other clients; task ownership unknown; missed ancestry unknown"
			}
			l.write(event{Event: label, Owner: id, Count: count, AgeSeconds: int(time.Since(ended).Seconds()), Detail: fmt.Sprintf("dropped=%d; %s", l.dropped.Load()+droppedOwners.Load(), detail)})
		}
	}
	sample()
	var grace <-chan time.Time
	var timer *time.Timer
	stage := 0
	var reapedAt time.Time
	finish := func() {
		ended = time.Now()
		l.write(event{Event: "owner_end", Owner: id, PID: pid})
		_, _ = snapshotFresh()
		sample()
		timer = time.NewTimer(10 * time.Second)
		grace = timer.C
		end = nil
	}
	for {
		select {
		case <-reap:
			l.write(event{Event: "reaped", Owner: id, PID: pid})
			reap = nil
			reapedAt = time.Now()
		case <-end:
			finish()
		case <-ticker.C:
			if ended.IsZero() {
				sample()
				if result != "" {
					if _, err := os.Stat(result); err == nil {
						finish()
					} else if !reapedAt.IsZero() && time.Since(reapedAt) > 2*time.Second {
						l.write(event{Event: "unavailable", Owner: id, Detail: "worker reaped without observed result; logical owner end unknown"})
						return
					}
				}
			}
		case <-grace:
			sample()
			if stage == 0 {
				stage++
				timer.Reset(50 * time.Second)
			} else {
				return
			}
		}
	}
}

// All observers in this process share a recent immutable census. The mutex
// serializes enumeration only inside diagnostics, never in workflow code.
var census struct {
	sync.Mutex
	at        time.Time
	processes map[int]process
	err       error
}

func snapshot() (map[int]process, error) {
	census.Lock()
	defer census.Unlock()
	if time.Since(census.at) < 200*time.Millisecond {
		return census.processes, census.err
	}
	census.processes, census.err = snapshotSystem()
	census.at = time.Now()
	return census.processes, census.err
}

func snapshotFresh() (map[int]process, error) {
	census.Lock()
	defer census.Unlock()
	census.processes, census.err = snapshotSystem()
	census.at = time.Now()
	return census.processes, census.err
}

func sameProcess(a, b process) bool { return a.PID == b.PID && a.Birth != "" && a.Birth == b.Birth }

func safeID(s string) string {
	if len(s) > 128 {
		s = s[:128]
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "unavailable"
		}
	}
	if s == "" {
		return "unavailable"
	}
	return s
}
