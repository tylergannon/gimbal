package processdebug

import (
	"bufio"
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
	t.Setenv("GIMBAL_DEBUG_PROCESSES", "0")
	if got := Start("test", "test", os.Getpid()); got != nil {
		t.Fatal("disabled observer started")
	}
}

func TestSurvivingOwnedChild(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("ps enumeration unsupported")
	}
	t.Setenv("GIMBAL_DEBUG_PROCESSES", "1")
	dir := t.TempDir()
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
	data, err := os.ReadFile(output.path)
	if err != nil {
		t.Fatal(err)
	}
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
	observer := Executor("live-worker", "session_abc", worker.Process.Pid, result)
	defer observer.End()
	waitLog(t, `"event":"owned","owner":"live-worker"`)
	waitLog(t, `"session":"session_abc"`)
	if err := os.WriteFile(result, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	waitLog(t, `"event":"suspect_survivor","owner":"live-worker"`)
	if strings.Contains(string(data), fmt.Sprintf(`"pid":%d`, unrelated.Process.Pid)) {
		t.Fatal("unrelated service was attributed to owner")
	}
	if !strings.Contains(string(data), fmt.Sprintf(`"pid":%d`, childPID)) {
		t.Fatal("missing child")
	}
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
