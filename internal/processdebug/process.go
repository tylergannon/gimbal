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
	Time                  time.Time `json:"time"`
	Event                 string    `json:"event"`
	Owner                 string    `json:"owner,omitempty"`
	Kind                  string    `json:"kind,omitempty"`
	Run                   string    `json:"run,omitempty"`
	Session               string    `json:"session,omitempty"`
	PID                   int       `json:"pid,omitempty"`
	Process               *process  `json:"process,omitempty"`
	AgeSeconds            int       `json:"owner_end_age_seconds,omitempty"`
	ObservationAgeSeconds int       `json:"observation_end_age_seconds,omitempty"`
	Count                 int       `json:"count,omitempty"`
	RequestedAt           time.Time `json:"requested_at,omitzero"`
	CensusAt              time.Time `json:"census_at,omitzero"`
	DiscoveryDelayMS      *int64    `json:"discovery_delay_ms,omitempty"`
	Detail                string    `json:"detail,omitempty"`
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
		if e.IsDir() || !strings.HasPrefix(e.Name(), "process-") || (!strings.HasSuffix(e.Name(), ".jsonl") && !strings.HasSuffix(e.Name(), ".jsonl.1")) {
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
	end                         chan struct{}
	reap                        chan struct{}
	stop                        chan struct{}
	endOnce, reapOnce, stopOnce sync.Once
	endRequested                atomic.Bool
}

func Start(kind, id string, pid int) *Owner {
	return start(kind, id, pid, false, "", "", "", nil, os.ErrNotExist)
}

// Candidates observes the selected shared Codex daemon without attributing its children to a turn.
func Candidates(sessionID string) *Owner {
	return start("codex_shared_daemon", sessionID, 0, true, "", "", sessionID, nil, os.ErrNotExist)
}

func Service(id, run string, pid int) *Owner {
	return start("service", id, pid, false, "", run, "", nil, os.ErrNotExist)
}
func Executor(id, session, run string, pid int, result string, prior os.FileInfo, priorErr error) *Owner {
	return start("executor", id, pid, false, result, run, session, prior, priorErr)
}
func ExecutorCandidates(id, session, run, result string, prior os.FileInfo, priorErr error) *Owner {
	return start("codex_shared_daemon", id, 0, true, result, run, session, prior, priorErr)
}

func start(kind, id string, pid int, shared bool, result, run, session string, prior os.FileInfo, priorErr error) *Owner {
	if os.Getenv("GIMBAL_DEBUG_PROCESSES") != "1" || (pid <= 0 && !shared) {
		return nil
	}
	select {
	case slots <- struct{}{}:
	default:
		droppedOwners.Add(1)
		return nil
	}
	requestedAt := time.Now()
	var anchor process
	if !shared {
		anchor, _ = initialIdentity(pid)
	}
	o := &Owner{end: make(chan struct{}), reap: make(chan struct{}), stop: make(chan struct{})}
	go func() {
		defer func() { <-slots }()
		observe(o, kind, id, pid, shared, result, run, session, anchor, requestedAt, prior, priorErr)
	}()
	return o
}
func (o *Owner) Reaped() {
	if o != nil {
		o.reapOnce.Do(func() { close(o.reap) })
	}
}
func (o *Owner) StopRequested() {
	if o != nil {
		o.stopOnce.Do(func() { close(o.stop) })
	}
}
func (o *Owner) End() {
	if o != nil {
		o.endOnce.Do(func() { o.endRequested.Store(true); close(o.end) })
	}
}

func observe(o *Owner, kind, id string, pid int, shared bool, result, run, session string, anchor process, requestedAt time.Time, prior os.FileInfo, priorErr error) {
	l := getLogger()
	if l == nil {
		return
	}
	// IDs are existing opaque operation identifiers; do not emit paths or inputs.
	id = safeID(id)
	kind = safeID(kind)
	run = safeID(run)
	session = safeID(session)
	l.write(event{Event: "start", Owner: id, Kind: kind, Run: run, Session: session, PID: pid, RequestedAt: requestedAt, Detail: "missing run or session correlation is unavailable; sampled ancestry may be incomplete"})
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		l.write(event{Event: "unavailable", Owner: id, Detail: "process enumeration unsupported on this platform"})
		return
	}
	if result != "" && priorErr == nil && prior != nil {
		l.write(event{Event: "result_baseline", Owner: id, Detail: "existing result ignored until changed; launch ownership may be a replay"})
	} else if result != "" && priorErr != nil && !os.IsNotExist(priorErr) {
		l.write(event{Event: "unavailable", Owner: id, Detail: "result state before launch unavailable; completion cannot be established"})
	}
	var daemon process
	if shared {
		var err error
		daemon, err = resolveDaemon()
		if err != nil {
			l.write(event{Event: "unavailable", Owner: id, Detail: "shared daemon identity unavailable"})
			return
		}
		pid = daemon.PID
	}
	known := map[int]process{}
	baseline := map[int]process{}
	firstSample := true
	stopping := false
	neutralStopSample := false
	var lastDrops uint64
	processCapacityReported := false
	rootAttempted := false
	ended := time.Time{}
	reap := o.reap
	end := o.end
	stop := o.stop
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	sample := func() int {
		count := 0
		var snapshot map[int]process
		var censusAt time.Time
		var err error
		if !rootAttempted {
			snapshot, censusAt, err = snapshotFresh()
		} else {
			snapshot, censusAt, err = cachedSnapshot()
		}
		if err != nil {
			l.write(event{Event: "unavailable", Owner: id, Detail: "process enumeration failed"})
			return -1
		}
		if !rootAttempted {
			p, ok := snapshot[pid]
			if shared && ok && !sameProcess(p, daemon) {
				ok = false
			}
			if !shared {
				if ok && !sameProcess(p, anchor) {
					ok = false
				}
				select {
				case <-o.reap:
					ok = false
				default:
				}
			}
			rootAttempted = true
			if shared {
				delay := censusAt.Sub(requestedAt).Milliseconds()
				l.write(event{Event: "first_census", Owner: id, RequestedAt: requestedAt, CensusAt: censusAt, DiscoveryDelayMS: &delay, Detail: "first census follows asynchronous daemon discovery and may include work started before or during discovery"})
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
		// Shared candidates are a sampled cohort observed before the end signal
		// or result publication. Never add daemon activity during the grace period.
		canDiscover := !shared || (ended.IsZero() && !o.endRequested.Load() && !resultPublished(result, prior, priorErr))
		if shared && firstSample && !canDiscover {
			l.write(event{Event: "unavailable", Owner: id, Detail: "observation ended before its first census; baseline and pre-end cohort unavailable"})
		}
		if canDiscover {
			added, dropped := discover(known, snapshot)
			if dropped > 0 && !processCapacityReported {
				processCapacityReported = true
				l.dropped.Add(uint64(dropped))
				l.write(event{Event: "dropped_process", Owner: id, Count: dropped, Detail: "process attribution capacity reached"})
			}
			for _, item := range classify(added, shared, firstSample, baseline) {
				p := item.process
				l.write(event{Event: item.label, Owner: id, Process: &p})
			}
		}
		if firstSample && shared {
			detail := "first census may include work begun before or during discovery; missed ancestry remains unknown"
			if !canDiscover {
				detail = "baseline unavailable because observation ended before its first census"
			}
			l.write(event{Event: "baseline_count", Owner: id, Count: len(baseline), Detail: detail})
		}
		firstSample = false

		if !ended.IsZero() {
			baselineCount := 0
			for n, p := range known {
				if cur, ok := snapshot[n]; ok && sameProcess(cur, p) && (!shared || n != pid) {
					if shared {
						if base, ok := baseline[n]; ok && sameProcess(base, p) {
							baselineCount++
							continue
						}
					}
					count++
					q := cur
					label := "suspect_survivor"
					if stopping && neutralStopSample {
						label = "stop_observation"
					}
					if shared {
						label = "candidate_survivor"
					}
					e := event{Event: label, Owner: id, Process: &q}
					if shared {
						e.ObservationAgeSeconds = int(time.Since(ended).Seconds())
					} else {
						e.AgeSeconds = int(time.Since(ended).Seconds())
					}
					l.write(e)
				}
			}
			label := "survivor_count"
			if stopping && neutralStopSample {
				label = "stop_observation_count"
			}
			detail := "missed ancestry unknown"
			if shared {
				label = "candidate_count"
				detail = "cohort first seen after baseline and before observation end; sampling may miss ancestry and other clients remain possible"
				l.write(event{Event: "baseline_alive_count", Owner: id, Count: baselineCount, Detail: "baseline processes are not candidates"})
			}
			e := event{Event: label, Owner: id, Count: count, Detail: fmt.Sprintf("dropped=%d; %s", l.dropped.Load()+droppedOwners.Load(), detail)}
			if shared {
				e.ObservationAgeSeconds = int(time.Since(ended).Seconds())
			} else {
				e.AgeSeconds = int(time.Since(ended).Seconds())
			}
			l.write(e)
		}
		return count
	}
	sample()
	var grace <-chan time.Time
	var timer *time.Timer
	stage := 0
	var reapedAt time.Time
	finish := func(label string) {
		ended = time.Now()
		stopping = label == "stop_requested"
		e := event{Event: label, Owner: id, PID: pid}
		if shared {
			e.Event = "observation_end"
			e.PID = 0
			e.Detail = "shared daemon observation ended; daemon lifetime is independent"
		}
		l.write(e)
		_, _, _ = snapshotFresh()
		neutralStopSample = stopping
		sample()
		neutralStopSample = false
		timer = time.NewTimer(10 * time.Second)
		grace = timer.C
	}
	for {
		select {
		case <-reap:
			if !shared {
				l.write(event{Event: "reaped", Owner: id, PID: pid})
			}
			reap = nil
			reapedAt = time.Now()
		case <-stop:
			stop = nil
			if ended.IsZero() {
				finish("stop_requested")
			}
		case <-end:
			end = nil
			if ended.IsZero() {
				finish("owner_end")
			} else {
				if shared {
					l.write(event{Event: "observation_end", Owner: id, Detail: "shared daemon observation ended; daemon lifetime is independent"})
				} else {
					l.write(event{Event: "owner_end", Owner: id, PID: pid})
				}
			}
		case <-ticker.C:
			if n := droppedOwners.Load(); n > lastDrops {
				l.write(event{Event: "dropped_owner", Owner: id, Count: int(n - lastDrops), Detail: "owner capacity reached"})
				lastDrops = n
			}
			if ended.IsZero() {
				if result != "" && resultPublished(result, prior, priorErr) {
					finish("owner_end")
				} else {
					sample()
					if result != "" && !reapedAt.IsZero() && time.Since(reapedAt) > 2*time.Second {
						l.write(event{Event: "unavailable", Owner: id, Detail: "worker reaped without observed result; logical owner end unknown"})
						return
					}
				}
			}
		case <-grace:
			_, _, _ = snapshotFresh()
			count := sample()
			if count > 0 {
				fmt.Fprintln(os.Stderr, survivorWarning(id, count, int(time.Since(ended).Seconds()), shared))
			}
			if stage == 0 {
				stage++
				timer.Reset(50 * time.Second)
			} else {
				return
			}
		}
	}
}

func resultPublished(path string, prior os.FileInfo, priorErr error) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && resultChanged(prior, priorErr, info)
}

// A pre-existing result must change before it can mark this launch complete.
func resultChanged(before os.FileInfo, beforeErr error, after os.FileInfo) bool {
	if beforeErr != nil {
		return os.IsNotExist(beforeErr)
	}
	if before == nil || after == nil {
		return false
	}
	return !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime())
}

var resolveDaemon = selectedDaemon

type attributed struct {
	process process
	label   string
}

func classify(added []process, shared, first bool, baseline map[int]process) []attributed {
	out := make([]attributed, 0, len(added))
	for _, p := range added {
		label := "owned"
		if shared {
			label = "candidate"
			if first {
				label = "baseline"
				baseline[p.PID] = p
			}
		}
		out = append(out, attributed{p, label})
	}
	return out
}

// discover only follows a parent whose PID and birth still match the census.
func discover(known, census map[int]process) ([]process, int) {
	var added []process
	dropped := 0
	for changed := true; changed; {
		changed = false
		for n, p := range census {
			if _, ok := known[n]; ok {
				continue
			}
			parent, ok := known[p.Parent]
			if !ok {
				continue
			}
			current, ok := census[p.Parent]
			if !ok || !sameProcess(current, parent) {
				continue
			}
			if len(known) >= maxProcesses {
				dropped++
				continue
			}
			known[n] = p
			added = append(added, p)
			changed = true
		}
	}
	return added, dropped
}

// All observers in this process share a recent immutable census. The mutex
// serializes enumeration only inside diagnostics, never in workflow code.
var census struct {
	sync.Mutex
	at        time.Time
	processes map[int]process
	err       error
}

func cachedSnapshot() (map[int]process, time.Time, error) {
	census.Lock()
	defer census.Unlock()
	if time.Since(census.at) < 200*time.Millisecond {
		return census.processes, census.at, census.err
	}
	census.processes, census.err = snapshotSystem()
	census.at = time.Now()
	return census.processes, census.at, census.err
}

func snapshotFresh() (map[int]process, time.Time, error) {
	census.Lock()
	defer census.Unlock()
	census.processes, census.err = snapshotSystem()
	census.at = time.Now()
	return census.processes, census.at, census.err
}

func sameProcess(a, b process) bool { return a.PID == b.PID && a.Birth != "" && a.Birth == b.Birth }

func survivorWarning(owner string, count, age int, shared bool) string {
	if shared {
		return fmt.Sprintf("gimbal process diagnostics: %d shared-daemon candidates remain %ds after observation %s ended; task ownership is unknown", count, age, owner)
	}
	return fmt.Sprintf("gimbal process diagnostics: %d known-owned processes remain %ds after owner %s ended; these are suspects, not confirmed leaks", count, age, owner)
}
func safeID(s string) string {
	if len(s) > 128 || s == "" {
		return "unavailable"
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return "unavailable"
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
				return "unavailable"
			}
		}
	}
	return s
}
