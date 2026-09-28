package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylergannon/gimbal"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
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
		// Temporal's report that the worker confirmed the cancellation.
		return temporal.NewCanceledError()
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

// testEnvironment is an environment whose Docker is a script that logs each
// invocation to dockerLog and exits with dockerExit. Its Postgres is
// unreachable, so removing its bootstrap row fails.
func testEnvironment(t *testing.T, dockerExit int) (env *Environment, dockerLog string) {
	t.Helper()
	dir := t.TempDir()
	dockerLog = filepath.Join(dir, "docker.log")
	script := filepath.Join(dir, "docker")
	body := "#!/bin/sh\necho \"$*\" >> " + strconv.Quote(dockerLog) + "\nexit " + strconv.Itoa(dockerExit) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := pgxpool.New(context.Background(), "postgres://gimbal@127.0.0.1:1/gimbal?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	b := &Backend{cfg: Config{DockerExecutable: script}, db: db, owner: "owner", containers: []string{"worker-c"}, environments: map[string]*Environment{}}
	env = &Environment{backend: b, name: "dev", bootstrap: bootstrap{Name: "scoped-dev", Queue: "queue", Container: "worker-c", Mounts: []string{dir}}}
	b.environments["dev"] = env
	return env, dockerLog
}

func dockerCalls(t *testing.T, dockerLog string) string {
	t.Helper()
	data, err := os.ReadFile(dockerLog)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// assertRemoved checks that env removed its worker container and now refuses
// work with the removal error.
func assertRemoved(t *testing.T, env *Environment, dockerLog string, err error) {
	t.Helper()
	if calls := dockerCalls(t, dockerLog); calls != "rm -f worker-c\n" {
		t.Fatalf("docker calls = %q, want the worker removed once", calls)
	}
	removed := env.removal()
	if removed == nil || !strings.Contains(removed.Error(), `environment "dev" was removed after an unconfirmed cancellation`) {
		t.Fatalf("removal = %v, want the environment removed", removed)
	}
	if !errors.Is(err, removed) {
		t.Fatalf("error = %v, want it to carry the removal", err)
	}
	if _, startErr := env.Start(context.Background(), gimbal.ExecutionCommand{Operation: "later", Workdir: env.bootstrap.Mounts[0], Command: "true"}, nil, nil); !errors.Is(startErr, removed) {
		t.Fatalf("Start after removal = %v, want the removal", startErr)
	}
}

func TestProcessStopReturnsTerminalActivityFailure(t *testing.T) {
	want := errors.New("worker command process group remains after SIGKILL")
	h := &fakeActivityHandle{cancelled: make(chan struct{}), getErr: want}
	env, _ := testEnvironment(t, 0)
	p := &Process{ctx: context.Background(), environment: env, handle: h}
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
	env, _ := testEnvironment(t, 0)
	p := &Process{ctx: context.Background(), environment: env, handle: h}
	if err := p.awaitStarted(context.Background()); err != nil {
		t.Fatalf("awaitStarted()=%v, want completed process result to acknowledge startup", err)
	}
	code, err := p.Wait()
	if code != 7 || err != nil {
		t.Fatalf("Wait()=(%d,%v), want nonzero exit code 7", code, err)
	}
}

type unconfirmedCancelHandle struct{}

func (unconfirmedCancelHandle) Get(ctx context.Context, _ any) error {
	<-ctx.Done()
	return ctx.Err()
}
func (unconfirmedCancelHandle) Describe(context.Context, client.DescribeActivityOptions) (*client.ActivityExecutionDescription, error) {
	return nil, errors.New("unexpected Describe call")
}
func (unconfirmedCancelHandle) Cancel(context.Context, client.CancelActivityOptions) error {
	return nil
}

type channelWriter chan string

func (w channelWriter) Write(p []byte) (int, error) {
	w <- string(append([]byte(nil), p...))
	return len(p), nil
}

func TestUnconfirmedCancellationStopsOutputRelayBeforeWaitReturns(t *testing.T) {
	previousTimeout := activityWaitTimeout
	activityWaitTimeout = 20 * time.Millisecond
	t.Cleanup(func() { activityWaitTimeout = previousTimeout })

	path := filepath.Join(t.TempDir(), "stdout.log")
	if err := os.WriteFile(path, []byte("live output"), 0o600); err != nil {
		t.Fatal(err)
	}
	lateFile, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lateFile.Close() })
	output := make(channelWriter, 10000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env, dockerLog := testEnvironment(t, 0)
	p := &Process{
		ctx: ctx, environment: env, handle: unconfirmedCancelHandle{}, stdoutPath: path,
		relayStop: make(chan struct{}), relayDone: make(chan error, 2), relayEnded: make(chan struct{}),
	}
	go func() { p.relayDone <- relayOutput(path, output, p.relayStop) }()
	go func() { p.relayDone <- nil }()
	producerStop, producerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(producerDone)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-producerStop:
				return
			case <-ticker.C:
				_, _ = lateFile.WriteString("x")
			}
		}
	}()
	_, err = p.Wait()
	if err == nil || !strings.Contains(err.Error(), "cancellation was not confirmed") {
		t.Fatalf("Wait() error=%v, want bounded unconfirmed-cancellation error", err)
	}
	assertRemoved(t, env, dockerLog, err)
	select {
	case got := <-output:
		if got != "live output" {
			t.Fatalf("relayed output=%q, want live output", got)
		}
	case <-time.After(time.Second):
		t.Fatal("output relay did not drain before Wait returned")
	}
	deliveredAtReturn := len(output)
	close(producerStop)
	<-producerDone
	if _, err := lateFile.WriteString("late output"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if len(output) != deliveredAtReturn {
		t.Fatalf("relay wrote after Wait returned: output count %d -> %d", deliveredAtReturn, len(output))
	}
}

func TestProcessWaitPreservesNonzeroExitAsCode(t *testing.T) {
	h := &fakeActivityHandle{code: 9, cancelled: make(chan struct{})}
	env, _ := testEnvironment(t, 0)
	p := &Process{ctx: context.Background(), environment: env, handle: h, workdir: "/tmp/project"}
	code, err := p.Wait()
	if code != 9 || err != nil {
		t.Fatalf("Wait()=(%d,%v), want (9,nil)", code, err)
	}
}

func TestProcessStopCancelsActivityAndWaitsForIt(t *testing.T) {
	h := &fakeActivityHandle{cancelled: make(chan struct{})}
	env, dockerLog := testEnvironment(t, 0)
	p := &Process{ctx: context.Background(), environment: env, handle: h}
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
	if calls := dockerCalls(t, dockerLog); calls != "" || env.removal() != nil {
		t.Fatalf("a confirmed stop removed the environment: docker %q, removal %v", calls, env.removal())
	}
}

// A command activity that ends without the worker confirming it (a timeout
// or a lost transport) removes the environment, whether Wait or Stop saw it.
func TestUnconfirmedCommandCompletionRemovesEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		stop bool
	}{
		{"wait/heartbeat timeout", temporal.NewTimeoutError(enums.TIMEOUT_TYPE_HEARTBEAT, nil), false},
		{"wait/transport", errors.New("rpc error: connection reset"), false},
		{"stop/heartbeat timeout", temporal.NewTimeoutError(enums.TIMEOUT_TYPE_HEARTBEAT, nil), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, dockerLog := testEnvironment(t, 0)
			p := &Process{ctx: context.Background(), environment: env, handle: &fakeActivityHandle{cancelled: make(chan struct{}), getErr: tc.err}}
			var err error
			if tc.stop {
				err = p.Stop()
			} else {
				_, err = p.Wait()
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("error = %v, want the completion %v preserved", err, tc.err)
			}
			assertRemoved(t, env, dockerLog, err)
			if second := p.Stop(); !errors.Is(second, env.removal()) {
				t.Fatalf("Stop after removal = %v, want the removal", second)
			}
			if calls := dockerCalls(t, dockerLog); strings.Count(calls, "rm -f") != 1 {
				t.Fatalf("docker calls = %q, want one removal", calls)
			}
		})
	}
}

// A removal whose docker rm fails says the worker may still be running and
// leaves the container for Backend.Close to try again.
func TestFailedRemovalKeepsContainerForClose(t *testing.T) {
	env, _ := testEnvironment(t, 1)
	err := env.remove()
	if err == nil || !strings.Contains(err.Error(), "its worker may still be running") {
		t.Fatalf("remove = %v, want the worker reported possibly running", err)
	}
	if !slices.Equal(env.backend.containers, []string{"worker-c"}) {
		t.Fatalf("containers = %v, want worker-c kept for Close", env.backend.containers)
	}
	if again := env.remove(); again != err {
		t.Fatalf("second remove = %v, want the first removal %v", again, err)
	}
}

func TestActivityUnconfirmed(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"result", nil, false},
		{"cancelled", temporal.NewCanceledError(), false},
		{"application failure", temporal.NewApplicationError("codex: provider failed", "wrapError"), false},
		{"unconfirmed turn", temporal.NewApplicationError("codex: turn t1 was not confirmed stopped", codexTurnUnconfirmed), true},
		{"wrapped unconfirmed turn", fmt.Errorf("execution: harness turn: %w", temporal.NewApplicationError("x", codexTurnUnconfirmed)), true},
		{"heartbeat timeout", temporal.NewTimeoutError(enums.TIMEOUT_TYPE_HEARTBEAT, nil), true},
		{"transport", errors.New("rpc error: unavailable"), true},
		{"get deadline", context.DeadlineExceeded, true},
	} {
		if got := activityUnconfirmed(tc.err); got != tc.want {
			t.Errorf("%s: activityUnconfirmed(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestCheckWorkerBinary(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "gimbal-worker")
	plain := filepath.Join(dir, "plain")
	for path, mode := range map[string]os.FileMode{executable: 0o755, plain: 0o644} {
		if err := os.WriteFile(path, nil, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkWorkerBinary(executable); err != nil {
		t.Fatalf("checkWorkerBinary(executable) = %v", err)
	}
	for _, path := range []string{"", "bin/gimbal-worker", filepath.Join(dir, "missing"), dir, plain} {
		if err := checkWorkerBinary(path); err == nil || !strings.Contains(err.Error(), "worker_binary") {
			t.Errorf("checkWorkerBinary(%q) = %v, want an error naming worker_binary", path, err)
		}
	}
}

// The worker runs the configured binary, mounted read-only, as the
// container's entrypoint; a failed start names that binary.
func TestStartWorkerMountsConfiguredBinaryAsEntrypoint(t *testing.T) {
	env, dockerLog := testEnvironment(t, 1)
	b := env.backend
	b.cfg.WorkerBinary, b.cfg.DockerImage = "/opt/build/gimbal-worker", "consumer:local"
	err := b.startWorker(context.Background(), env.bootstrap)
	if err == nil || !strings.Contains(err.Error(), `worker_binary "/opt/build/gimbal-worker" in image "consumer:local"`) {
		t.Fatalf("startWorker = %v, want a failure naming the worker binary and image", err)
	}
	run, _, _ := strings.Cut(dockerCalls(t, dockerLog), "\n")
	if !strings.Contains(run, " -v /opt/build/gimbal-worker:/opt/gimbal/gimbal-worker:ro ") ||
		!strings.Contains(run, " --entrypoint /opt/gimbal/gimbal-worker ") || !strings.HasSuffix(run, " consumer:local worker") {
		t.Fatalf("docker run = %q, want the binary mounted read-only as the entrypoint", run)
	}
}

func TestProcessWaitCancellationReachesActivity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := &fakeActivityHandle{cancelled: make(chan struct{})}
	env, _ := testEnvironment(t, 0)
	p := &Process{ctx: ctx, environment: env, handle: h}
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
