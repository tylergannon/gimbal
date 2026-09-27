package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type fakeActivityHandle struct {
	code        int
	getErr      error
	cancelled   chan struct{}
	cancelCalls atomic.Int32
}

func (h *fakeActivityHandle) Get(ctx context.Context, result any) error {
	select {
	case <-h.cancelled:
		if h.getErr != nil {
			return h.getErr
		}
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	if h.getErr != nil {
		return h.getErr
	}
	result.(*commandResult).ExitCode = h.code
	return nil
}

func TestProcessStopReturnsTerminalActivityFailure(t *testing.T) {
	want := errors.New("worker command process group remains after SIGKILL")
	h := &fakeActivityHandle{cancelled: make(chan struct{}), getErr: want}
	p := &Process{ctx: context.Background(), handle: h}
	if err := p.Stop(); !errors.Is(err, want) {
		t.Fatalf("Stop() error=%v, want terminal activity failure %v", err, want)
	}
}
func (h *fakeActivityHandle) Cancel(context.Context, client.CancelActivityOptions) error {
	h.cancelCalls.Add(1)
	select {
	case <-h.cancelled:
	default:
		close(h.cancelled)
	}
	return nil
}
func (h *fakeActivityHandle) Describe(context.Context, client.DescribeActivityOptions) (*client.ActivityExecutionDescription, error) {
	return nil, errors.New("unexpected Describe call")
}

type completedActivityHandle struct{ *fakeActivityHandle }

func (h *completedActivityHandle) Describe(context.Context, client.DescribeActivityOptions) (*client.ActivityExecutionDescription, error) {
	return &client.ActivityExecutionDescription{Status: enums.ACTIVITY_EXECUTION_STATUS_COMPLETED}, nil
}

func TestProcessStartupAcceptsFastCompletedCommandResult(t *testing.T) {
	h := &completedActivityHandle{fakeActivityHandle: &fakeActivityHandle{code: 7, cancelled: make(chan struct{})}}
	p := &Process{ctx: context.Background(), handle: h}
	if err := p.awaitStarted(context.Background()); err != nil {
		t.Fatalf("awaitStarted()=%v, want completed process result to acknowledge startup", err)
	}
	code, err := p.Wait()
	if code != 7 || err != nil {
		t.Fatalf("Wait()=(%d,%v), want nonzero exit code 7", code, err)
	}
}

func TestProcessWaitPreservesNonzeroExitAsCode(t *testing.T) {
	h := &fakeActivityHandle{code: 9, cancelled: make(chan struct{})}
	p := &Process{ctx: context.Background(), handle: h, workdir: "/tmp/project"}
	code, err := p.Wait()
	if code != 9 || err != nil {
		t.Fatalf("Wait()=(%d,%v), want (9,nil)", code, err)
	}
}

func TestProcessStopCancelsActivityAndWaitsForIt(t *testing.T) {
	h := &fakeActivityHandle{cancelled: make(chan struct{})}
	p := &Process{ctx: context.Background(), handle: h}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	if h.cancelCalls.Load() != 1 {
		t.Fatalf("cancel calls=%d, want 1", h.cancelCalls.Load())
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if h.cancelCalls.Load() != 1 {
		t.Fatalf("second Stop repeated cancellation: %d", h.cancelCalls.Load())
	}
}

func TestProcessWaitCancellationReachesActivity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := &fakeActivityHandle{cancelled: make(chan struct{})}
	p := &Process{ctx: ctx, handle: h}
	cancel()
	_, err := p.Wait()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error=%v, want context.Canceled", err)
	}
	if h.cancelCalls.Load() != 1 {
		t.Fatalf("cancel calls=%d, want explicit activity cancellation", h.cancelCalls.Load())
	}
}

func TestCommandExitSurvivesPostgresEventFailure(t *testing.T) {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, "postgres://gimbal:gimbal@127.0.0.1:1/gimbal?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stdout, stderr := filepath.Join(t.TempDir(), "stdout.log"), filepath.Join(t.TempDir(), "stderr.log")
	result, activityErr := runCommandActivity(ctx, db, commandInput{Environment: "test", Operation: "exit-9", Workdir: t.TempDir(), Command: "sh", Args: []string{"-c", "exit 9"}, StdoutPath: stdout, StderrPath: stderr})
	if activityErr != nil || result.ExitCode != 9 || result.RecordingError == "" {
		t.Fatalf("activity result=(%+v,%v), want exit 9, surfaced recording failure, and nil execution error", result, activityErr)
	}
}

func TestCommandActivityCancellationStopsOSProcess(t *testing.T) {
	db, err := pgxpool.New(context.Background(), "postgres://gimbal:gimbal@127.0.0.1:1/gimbal?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := runCommandActivity(ctx, db, commandInput{Environment: "test", Operation: "cancel", Workdir: dir, Command: "sh", Args: []string{"-c", "sleep 30"}, StdoutPath: filepath.Join(dir, "stdout"), StderrPath: filepath.Join(dir, "stderr")})
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("activity error=%v, want context.Canceled", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("activity did not stop its OS process after cancellation")
	}
}

func TestCommandActivityKillsDescendantAfterDirectProcessExits(t *testing.T) {
	db, err := pgxpool.New(context.Background(), "postgres://gimbal:gimbal@127.0.0.1:1/gimbal?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	childFile := filepath.Join(dir, "child.pid")
	result, err := runCommandActivity(context.Background(), db, commandInput{
		Environment: "test", Operation: "exit-with-child", Workdir: dir, Command: "sh",
		Args:       []string{"-c", "sleep 60 & echo $! > " + childFile + "; exit 0"},
		StdoutPath: filepath.Join(dir, "stdout"), StderrPath: filepath.Join(dir, "stderr"),
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("command activity=(%+v,%v), want a clean direct exit", result, err)
	}
	raw, err := os.ReadFile(childFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background child %d remains after command activity returned: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCommandActivityCleanupFailureSurvivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cleanupErr := errors.New("worker command process group remains after SIGKILL")
	got := commandActivityError(ctx, context.Canceled, cleanupErr)
	if got == nil || !errors.Is(got, cleanupErr) || errors.Is(got, context.Canceled) {
		t.Fatalf("activity error=%v, want cleanup error without cancellation identity", got)
	}
	if got := commandActivityError(ctx, context.Canceled, nil); !errors.Is(got, context.Canceled) {
		t.Fatalf("successful cleanup error=%v, want normal cancellation", got)
	}
}

func TestWaitForWorkerReadyFailsImmediatelyWhenWorkerDies(t *testing.T) {
	started := time.Now()
	probe := &testWorkerReadinessProbe{results: []workerReadinessResult{{err: errors.New("container state is \"exited\"")}}}
	err := waitForWorkerReady(context.Background(), time.Millisecond, probe, bootstrap{})
	if err == nil || !strings.Contains(err.Error(), `container state is "exited"`) {
		t.Fatalf("readiness error=%v, want exited container cause", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("dead worker detection took %s, want immediate failure", elapsed)
	}
}

func TestWaitForWorkerReadyWaitsForPollerThenSucceeds(t *testing.T) {
	probe := &testWorkerReadinessProbe{results: []workerReadinessResult{
		{status: "container is running, waiting for its Temporal activity poller"},
		{status: "container is running, waiting for its Temporal activity poller"},
		{ready: true},
	}}
	err := waitForWorkerReady(context.Background(), time.Millisecond, probe, bootstrap{})
	if err != nil {
		t.Fatalf("readiness check: %v", err)
	}
	if probe.calls != 3 {
		t.Fatalf("readiness checks=%d, want 3", probe.calls)
	}
}

func TestWaitForWorkerReadyHonorsBoundedContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	probe := &testWorkerReadinessProbe{results: []workerReadinessResult{{status: "container is running, waiting for its Temporal activity poller"}}}
	err := waitForWorkerReady(ctx, time.Millisecond, probe, bootstrap{})
	if err == nil || !strings.Contains(err.Error(), "worker readiness timed out") {
		t.Fatalf("readiness error=%v, want bounded timeout", err)
	}
}

type workerReadinessResult struct {
	ready  bool
	status string
	err    error
}

type testWorkerReadinessProbe struct {
	results []workerReadinessResult
	calls   int
}

func (p *testWorkerReadinessProbe) checkWorkerReady(context.Context, bootstrap) (bool, string, error) {
	index := p.calls
	p.calls++
	if index >= len(p.results) {
		index = len(p.results) - 1
	}
	result := p.results[index]
	return result.ready, result.status, result.err
}
