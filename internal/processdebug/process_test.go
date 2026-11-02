package processdebug

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDisabledDoesNotStartObserver(t *testing.T) {
	for _, flag := range []string{"", "0", "true"} {
		t.Setenv("GIMBAL_DEBUG_PROCESSES", flag)
		if Start("test", "test", os.Getpid()) != nil || Candidates("session") != nil || Executor("id", "", "", os.Getpid(), "", nil, os.ErrNotExist) != nil || ExecutorCandidates("id", "", "", "", nil, os.ErrNotExist) != nil {
			t.Fatalf("observer started for %q", flag)
		}
	}
}

func TestSurvivingOwnedChild(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("ps enumeration unsupported")
	}
	if os.Getenv("GIMBAL_PROCESSDEBUG_TEST_CHILD") != "1" {
		dir := t.TempDir()
		cmd := exec.Command(os.Args[0], "-test.run=^TestSurvivingOwnedChild$")
		cmd.Env = append(os.Environ(), "GIMBAL_PROCESSDEBUG_TEST_CHILD=1", "GIMBAL_PROCESSDEBUG_TEST_DIR="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v\n%s", err, out)
		}
		return
	}
	t.Setenv("GIMBAL_DEBUG_PROCESSES", "1")
	dir := os.Getenv("GIMBAL_PROCESSDEBUG_TEST_DIR")
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	unrelated := exec.Command("sleep", "30")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() }()
	cmd := exec.Command("sh", "-c", "sleep 30 & echo $!; wait")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("no child pid")
	}
	childPID, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	child, err := os.FindProcess(childPID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Kill() }()
	o := Start("test", "survivor-test", cmd.Process.Pid)
	if o == nil {
		t.Fatal("observer unavailable")
	}
	defer o.End()
	// Wait until sampling has established the child before reparenting it.
	waitLog(t, fmt.Sprintf(`"pid":%d`, childPID))
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	o.Reaped()
	o.End()
	waitLog(t, `"event":"suspect_survivor","owner":"survivor-test"`)
	// An exited and reaped worker is not reported as a live survivor.
	exited := exec.Command("sleep", "30")
	if err := exited.Start(); err != nil {
		t.Fatal(err)
	}
	exitedObserver := Start("test", "exited-worker", exited.Process.Pid)
	waitLog(t, `"event":"owned","owner":"exited-worker"`)
	_ = exited.Process.Kill()
	_ = exited.Wait()
	exitedObserver.Reaped()
	exitedObserver.End()
	waitLog(t, `"event":"survivor_count","owner":"exited-worker"`)
	check, _ := os.ReadFile(output.path)
	if strings.Contains(string(check), `"event":"suspect_survivor","owner":"exited-worker"`) {
		t.Fatal("reaped worker reported alive")
	}
	// A live worker is still observed when its result marks logical completion.
	worker := exec.Command("sleep", "30")
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = worker.Process.Kill(); _ = worker.Wait() }()
	result := dir + "/result.json"
	observer := Executor("live-worker", "session_abc", "run_abc", worker.Process.Pid, result, nil, os.ErrNotExist)
	defer observer.End()
	waitLog(t, `"event":"owned","owner":"live-worker"`)
	waitLog(t, `"session":"session_abc"`)
	if err := os.WriteFile(result, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	waitLog(t, `"event":"suspect_survivor","owner":"live-worker"`)
	data, err := os.ReadFile(output.path)
	if err != nil {
		t.Fatal(err)
	}
	// An unchanged result from an earlier launch must not finish this owner.
	stale := dir + "/stale.json"
	if err := os.WriteFile(stale, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	prior, err := os.Stat(stale)
	if err != nil {
		t.Fatal(err)
	}
	staleWorker := exec.Command("sleep", "30")
	if err := staleWorker.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = staleWorker.Process.Kill(); _ = staleWorker.Wait() }()
	staleObserver := Executor("stale-worker", "", "run_abc", staleWorker.Process.Pid, stale, prior, nil)
	defer staleObserver.End()
	waitLog(t, `"event":"result_baseline","owner":"stale-worker"`)
	time.Sleep(700 * time.Millisecond)
	current, _ := os.ReadFile(output.path)
	if strings.Contains(string(current), `"event":"owner_end","owner":"stale-worker"`) {
		t.Fatal("unchanged stale result ended owner")
	}
	if err := os.WriteFile(stale, []byte("new result publication"), 0600); err != nil {
		t.Fatal(err)
	}
	waitLog(t, `"event":"owner_end","owner":"stale-worker"`)
	if strings.Contains(string(data), fmt.Sprintf(`"pid":%d`, unrelated.Process.Pid)) {
		t.Fatal("unrelated service was attributed to owner")
	}
	if !strings.Contains(string(data), fmt.Sprintf(`"pid":%d`, childPID)) {
		t.Fatal("missing child")
	}
	// A stop request observes a live service without calling it a suspect.
	svc := Service("scope.1/server.1", "run_1", unrelated.Process.Pid)
	if svc == nil {
		t.Fatal("missing service observer")
	}
	defer svc.End()
	waitLog(t, `"event":"owned","owner":"scope.1/server.1"`)
	svc.StopRequested()
	waitLog(t, `"event":"stop_observation","owner":"scope.1/server.1"`)
	b, _ := os.ReadFile(output.path)
	if strings.Contains(string(b), `"event":"suspect_survivor","owner":"scope.1/server.1"`) {
		t.Fatal("normal stop marked suspect immediately")
	}
	// Existing observers report dropped capacity even while none has ended.
	held := 0
	for {
		select {
		case slots <- struct{}{}:
			held++
		default:
			goto full
		}
	}
full:
	defer func() {
		for i := 0; i < held; i++ {
			<-slots
		}
	}()
	before := droppedOwners.Load()
	if Start("test", "overflow", os.Getpid()) != nil || droppedOwners.Load() != before+1 {
		t.Fatal("owner capacity not enforced")
	}
	waitLog(t, `"event":"dropped_owner"`)
	missing := Start("test", "missing-root", 2000000000)
	if missing == nil { // release one slot for the missing-root gap fixture
		<-slots
		held--
		missing = Start("test", "missing-root", 2000000000)
	}
	if missing == nil {
		t.Fatal("could not start missing-root fixture")
	}
	defer missing.End()
	waitLog(t, `"event":"unavailable","owner":"missing-root"`)

}
func waitLog(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if logger := getLogger(); logger != nil {
			b, _ := os.ReadFile(logger.path)
			if strings.Contains(string(b), want) {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("diagnostic did not contain %s", want)
}

func TestIdentityRequiresBirthAndPID(t *testing.T) {
	a := process{PID: 10, Birth: "one"}
	if !sameProcess(a, a) {
		t.Fatal("same process differs")
	}
	if sameProcess(a, process{PID: 10, Birth: "two"}) || sameProcess(a, process{PID: 11, Birth: "one"}) || sameProcess(process{PID: 10}, process{PID: 10}) {
		t.Fatal("accepted reused or unknown identity")
	}
}

func TestLogRotationAndRetention(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/process-test.jsonl"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	l := &logger{path: path, size: maxLog}
	l.write(event{Event: "rotation"})
	if _, err = os.Stat(path + ".1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		if err := os.WriteFile(fmt.Sprintf("%s/process-%02d.jsonl", dir, i), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	pruneLogs(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 30 {
		t.Fatalf("retained %d files", len(entries))
	}
}

func TestCorrelationAndWarnings(t *testing.T) {
	for _, id := range []string{"server.1", "lap.3/bakeoff.1/server.1", "session_abc"} {
		if safeID(id) != id {
			t.Errorf("lost identity %q", id)
		}
	}
	for _, id := range []string{"/private/path", "../path", "lap//server", "prompt with spaces"} {
		if safeID(id) != "unavailable" {
			t.Errorf("accepted %q", id)
		}
	}
	owned := survivorWarning("server.1", 2, 10, false)
	if !strings.Contains(owned, "known-owned") || !strings.Contains(owned, "suspects, not confirmed leaks") {
		t.Fatal(owned)
	}
	shared := survivorWarning("turn_1", 1, 10, true)
	if !strings.Contains(shared, "candidates") || !strings.Contains(shared, "ownership is unknown") {
		t.Fatal(shared)
	}
}

func TestAttributionAcrossCensusAndPIDReuse(t *testing.T) {
	root := process{PID: 10, Birth: "root"}
	old := process{PID: 11, Parent: 10, Birth: "old"}
	known := map[int]process{10: root}
	baseline := map[int]process{}
	first := map[int]process{10: root, 11: old}
	added, dropped := discover(known, first)
	labels := classify(added, true, true, baseline)
	if dropped != 0 || len(labels) != 1 || labels[0].label != "baseline" {
		t.Fatalf("baseline: %+v, %d", labels, dropped)
	}
	fresh := process{PID: 12, Parent: 10, Birth: "new"}
	second := map[int]process{10: root, 11: old, 12: fresh}
	added, _ = discover(known, second)
	labels = classify(added, true, false, baseline)
	if len(labels) != 1 || labels[0].label != "candidate" || labels[0].process.PID != 12 {
		t.Fatalf("candidate: %+v", labels)
	}
	// A reused parent's PID cannot confer ownership on its new child.
	reused := process{PID: 11, Parent: 10, Birth: "replacement"}
	orphan := process{PID: 13, Parent: 11, Birth: "orphan"}
	added, _ = discover(known, map[int]process{10: root, 11: reused, 13: orphan})
	if len(added) != 0 {
		t.Fatalf("attached through stale parent: %+v", added)
	}
	if sameProcess(known[11], reused) {
		t.Fatal("reused PID remained owned")
	}
	// A child whose parent was missed cannot be attributed.
	missing := process{PID: 14, Parent: 99, Birth: "missing"}
	added, _ = discover(known, map[int]process{10: root, 14: missing})
	if len(added) != 0 {
		t.Fatalf("invented missing ancestry: %+v", added)
	}
}

func TestAttributionCapacityAndWriteFailure(t *testing.T) {
	known := map[int]process{}
	for i := 1; i <= maxProcesses; i++ {
		known[i] = process{PID: i, Birth: "b"}
	}
	root := known[1]
	child := process{PID: maxProcesses + 1, Parent: 1, Birth: "child"}
	added, dropped := discover(known, map[int]process{1: root, child.PID: child})
	if len(added) != 0 || dropped == 0 || len(known) != maxProcesses {
		t.Fatal("process bound not enforced")
	}
	l := &logger{path: t.TempDir() + "/missing/log"}
	l.write(event{Event: "test"})
	if l.dropped.Load() != 1 {
		t.Fatal("write failure was not exposed")
	}
}

// The resolver seam selects only a disposable stand-in. The production
// observer, process census, ancestry traversal and baseline logic are real.
func TestSharedDaemonStandin(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unsupported")
	}
	if os.Getenv("GIMBAL_PROCESSDEBUG_SHARED_CHILD") != "1" {
		dir := t.TempDir()
		cmd := exec.Command(os.Args[0], "-test.run=^TestSharedDaemonStandin$")
		cmd.Env = append(os.Environ(), "GIMBAL_PROCESSDEBUG_SHARED_CHILD=1", "GIMBAL_PROCESSDEBUG_TEST_DIR="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("shared fixture: %v\n%s", err, out)
		}
		return
	}
	dir := os.Getenv("GIMBAL_PROCESSDEBUG_TEST_DIR")
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("GIMBAL_DEBUG_PROCESSES", "1")
	parent := exec.Command("sh", "-c", "sleep 30 & echo $!; read ignored; sleep 30 & echo $!; wait")
	input, err := parent.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = parent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Process.Kill(); _ = parent.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("missing baseline child")
	}
	baselinePID, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	baselineChild, _ := os.FindProcess(baselinePID)
	defer func() { _ = baselineChild.Kill() }()
	anchor, err := initialIdentity(parent.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	resolveDaemon = func() (process, error) { time.Sleep(200 * time.Millisecond); return anchor, nil }
	owner := Candidates("shared-session")
	if owner == nil {
		t.Fatal("observer unavailable")
	}
	defer owner.End()
	waitLog(t, `"event":"baseline_count","owner":"shared-session"`)
	if _, err = input.Write([]byte("continue\n")); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatal("missing later child")
	}
	laterPID, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	laterChild, _ := os.FindProcess(laterPID)
	defer func() { _ = laterChild.Kill() }()
	waitEvent(t, "shared-session", "candidate", laterPID, 0, 5*time.Second)
	owner.Reaped() // Simulates the independent launching worker being reaped.
	owner.End()
	waitEvent(t, "shared-session", "observation_end", 0, 0, 5*time.Second)
	waitEvent(t, "shared-session", "candidate_survivor", laterPID, 0, 5*time.Second)
	waitEvent(t, "shared-session", "candidate_count", 0, 0, 5*time.Second)
	events := readEvents(t)
	var baselineSeen, baselineSurvivor, timing, rootSeen, baselineCount, candidateCount, owned, daemonReaped, correctEnd bool
	for _, e := range events {
		if e.Owner != "shared-session" {
			continue
		}
		if e.Event == "shared_daemon" && e.Process != nil && e.Process.PID == parent.Process.Pid {
			rootSeen = true
		}
		if e.Event == "reaped" || e.Event == "owner_end" {
			daemonReaped = true
		}
		if e.Event == "observation_end" && e.PID == 0 && strings.Contains(e.Detail, "daemon lifetime is independent") {
			correctEnd = true
		}
		if e.Event == "owned" {
			owned = true
		}
		if e.Event == "baseline_alive_count" && e.Count >= 1 {
			baselineCount = true
		}
		if e.Event == "candidate_count" && e.Count == 1 {
			candidateCount = true
		}
		if e.Event == "baseline" && e.Process != nil && e.Process.PID == baselinePID {
			baselineSeen = true
		}
		if e.Event == "candidate_survivor" && e.Process != nil && e.Process.PID == baselinePID {
			baselineSurvivor = true
		}
		if e.Event == "first_census" && !e.RequestedAt.IsZero() && !e.CensusAt.Before(e.RequestedAt) && e.DiscoveryDelayMS != nil && *e.DiscoveryDelayMS >= 150 && e.Detail != "" {
			timing = true
		}
	}
	if !baselineSeen || baselineSurvivor || !timing || !rootSeen || !baselineCount || !candidateCount || owned || daemonReaped || !correctEnd {
		t.Fatalf("baseline=%v reported_candidate=%v timing=%v root=%v baseline_count=%v candidate_count=%v owned=%v reaped=%v end=%v", baselineSeen, baselineSurvivor, timing, rootSeen, baselineCount, candidateCount, owned, daemonReaped, correctEnd)
	}
	// The reap signal still terminates a result-missing worker observation.
	missing := ExecutorCandidates("missing-result", "session_1", "", dir+"/never-published", nil, os.ErrNotExist)
	waitEvent(t, "missing-result", "shared_daemon", parent.Process.Pid, 0, 5*time.Second)
	missing.Reaped()
	waitEvent(t, "missing-result", "unavailable", 0, 0, 5*time.Second)
	for _, e := range readEvents(t) {
		if e.Owner == "missing-result" && e.Event == "reaped" {
			t.Fatal("shared daemon incorrectly reported reaped")
		}
	}
	current, err := initialIdentity(parent.Process.Pid)
	if err != nil || !sameProcess(anchor, current) {
		t.Fatalf("shared stand-in did not remain alive: %v", err)
	}

}

func readEvents(t *testing.T) []event {
	t.Helper()
	b, err := os.ReadFile(getLogger().path)
	if err != nil {
		t.Fatal(err)
	}
	var out []event
	for _, line := range strings.Split(string(b), "\n") {
		var e event
		if json.Unmarshal([]byte(line), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}
func waitEvent(t *testing.T, owner, kind string, pid, age int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, e := range readEvents(t) {
			if e.Owner == owner && e.Event == kind && (e.AgeSeconds >= age || e.ObservationAgeSeconds >= age) && (pid == 0 || e.Process != nil && e.Process.PID == pid) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("missing event %s for %s at age %d", kind, owner, age)
}

func TestLongServiceGrace(t *testing.T) {
	if os.Getenv("GIMBAL_PROCESSDEBUG_LONG_TEST") != "1" {
		t.Skip("set GIMBAL_PROCESSDEBUG_LONG_TEST=1 for real 10s/60s grace")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unsupported")
	}
	if os.Getenv("GIMBAL_PROCESSDEBUG_LONG_CHILD") != "1" {
		dir := t.TempDir()
		cmd := exec.Command(os.Args[0], "-test.run=^TestLongServiceGrace$", "-test.timeout=90s")
		cmd.Env = append(os.Environ(), "GIMBAL_PROCESSDEBUG_LONG_CHILD=1", "GIMBAL_PROCESSDEBUG_TEST_DIR="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("grace fixture: %v\n%s", err, out)
		}
		return
	}
	dir := os.Getenv("GIMBAL_PROCESSDEBUG_TEST_DIR")
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("GIMBAL_DEBUG_PROCESSES", "1")
	warnings, err := os.Create(dir + "/warnings")
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = warnings
	worker := exec.Command("sleep", "90")
	if err = worker.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = worker.Process.Kill(); _ = worker.Wait(); _ = warnings.Close() }()
	owner := Service("service.1", "run_1", worker.Process.Pid)
	waitEvent(t, "service.1", "owned", worker.Process.Pid, 0, 5*time.Second)
	owner.StopRequested()
	waitEvent(t, "service.1", "stop_observation", worker.Process.Pid, 0, 5*time.Second)
	waitEvent(t, "service.1", "suspect_survivor", worker.Process.Pid, 10, 15*time.Second)
	waitEvent(t, "service.1", "suspect_survivor", worker.Process.Pid, 60, 60*time.Second)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(dir + "/warnings")
		if err == nil && strings.Count(string(b), "suspects, not confirmed leaks") >= 2 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("missing qualified grace warnings")
}
