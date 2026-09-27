package execution

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
)

type fakeActivityHandle struct {
	code        int
	cancelled   chan struct{}
	cancelCalls atomic.Int32
}

func (h *fakeActivityHandle) Get(ctx context.Context, result any) error {
	select {
	case <-h.cancelled:
		return errors.New("activity canceled")
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	result.(*commandResult).ExitCode = h.code
	return nil
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
