package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/effects"
)

type testHarness struct {
	closes      atomic.Int32
	schemaBytes atomic.Int32
	fail        atomic.Bool
	started     chan struct{}
	steered     chan string
}

func (h *testHarness) CreateSession(context.Context, string, string, string) (string, error) {
	return "native", nil
}
func (h *testHarness) RunTurn(ctx context.Context, _, prompt string, schema json.RawMessage, event func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	h.schemaBytes.Store(int32(len(schema)))
	_ = event(gimbal.AgentEvent{Type: "native", Data: json.RawMessage(`{}`), NativeRef: json.RawMessage(`{"native":"preserved"}`)})
	if h.started != nil {
		close(h.started)
		<-ctx.Done()
		return gimbal.TurnResult{}, ctx.Err()
	}
	return gimbal.TurnResult{Output: json.RawMessage(`"done"`)}, nil
}
func (h *testHarness) Steer(_ context.Context, _, message string) (bool, error) {
	h.steered <- message
	return true, nil
}
func (h *testHarness) Fork(context.Context, string) (string, error) { return "fork", nil }
func (h *testHarness) Close(context.Context, string) error {
	h.closes.Add(1)
	if h.fail.Load() {
		return errors.New("remote cleanup failed")
	}
	return nil
}

func TestHarnessResultsNativeEventsAndRetryableScopeCleanup(t *testing.T) {
	d := New()
	native := &testHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	h := &Harness{Endpoint: server.URL, Name: "fake"}
	ctx := effects.WithOwner(context.Background(), effects.Owner{Run: "run", Scope: "child.1", Task: "child.1/agent.1"})
	id, err := h.CreateSession(ctx, "model", "", "/")
	if err != nil {
		t.Fatal(err)
	}
	forkCtx := effects.WithOwner(ctx, effects.Owner{Run: "run", Scope: "child.1", Task: "child.1/fork.1"})
	if _, err := h.Fork(forkCtx, id); err != nil {
		t.Fatal(err)
	}
	turnCtx := effects.WithOwner(ctx, effects.Owner{Run: "run", Scope: "child.1", Task: "child.1/agent.1/turn.1"})
	var ref json.RawMessage
	result, err := h.RunTurn(turnCtx, id, "go", nil, func(e gimbal.AgentEvent) error { ref = e.NativeRef; return errors.New("observation unavailable") })
	if err != nil || string(result.Output) != `"done"` || len(ref) == 0 || native.schemaBytes.Load() != 0 {
		t.Fatalf("result=%s ref=%s err=%v", result.Output, ref, err)
	}
	native.fail.Store(true)
	if err := effects.Cleanup(ctx, server.URL, "run", "child.1"); err == nil {
		t.Fatal("false cleanup success")
	}
	if d.state(effects.Scope{Run: "run"}).Pending != 2 {
		t.Fatal("lost failed sessions")
	}
	if _, err := h.CreateSession(ctx, "model", "", "/"); err == nil {
		t.Fatal("admitted work after cleanup")
	}
	native.fail.Store(false)
	if err := effects.Cleanup(ctx, server.URL, "run", "child.1"); err != nil {
		t.Fatal(err)
	}
	if err := effects.Cleanup(ctx, server.URL, "run", "child.1"); err != nil {
		t.Fatal(err)
	}
	if d.state(effects.Scope{Run: "run"}).Pending != 0 {
		t.Fatal("cleanup remains pending")
	}
}

func TestTurnSurvivesReaderDisconnectAndCanBeSteeredAndStopped(t *testing.T) {
	d := New()
	native := &testHarness{started: make(chan struct{}), steered: make(chan string, 1)}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	o := effects.Owner{Run: "run", Scope: "", Task: "agent.1"}
	if _, err := d.create(context.Background(), effects.Session{Owner: o}); err != nil {
		t.Fatal(err)
	}
	o.Task = "agent.1/turn.1"
	if err := d.turn(effects.Turn{Owner: o, Session: "agent.1"}); err != nil {
		t.Fatal(err)
	}
	<-native.started
	// No reader needs to remain connected to keep execution alive.
	if p := d.poll(effects.Poll{Run: "run", ID: o.Task}); p.Done {
		t.Fatal("turn ended without cancellation")
	}
	if landed, err := d.steer(context.Background(), effects.Target{Run: "run", ID: "agent.1", Message: "continue"}); err != nil || !landed {
		t.Fatal(landed, err)
	}
	if <-native.steered != "continue" {
		t.Fatal("steer lost")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.cleanup(ctx, effects.Scope{Run: "run"}); err != nil {
		t.Fatal(err)
	}
}

func processRequest(t *testing.T, run, scope, id, script string, service bool) effects.Process {
	t.Helper()
	dir := t.TempDir()
	return effects.Process{Run: run, Scope: scope, Task: id, Command: "sh", Args: []string{"-c", script}, Workdir: dir, Service: service}
}
func TestProcessMeaningAndScopedAntiClaim(t *testing.T) {
	d := New()
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := processRequest(t, "run", "child.1", "child.1/check.1", "printf out; printf err >&2; exit 7", false)
	if err := d.process(command); err != nil {
		t.Fatal(err)
	}
	raw, err := effects.Wait(ctx, server.URL, effects.Target{Run: "run", ID: command.Task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result effects.ProcessResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	outFile, _ := (backend.HTTP{Endpoint: server.URL}).ReadBytes(ctx, backend.Artifact{Owner: command.Owner, Name: "stdout"})
	out, _ := io.ReadAll(outFile)
	_ = outFile.Close()
	errFile, _ := (backend.HTTP{Endpoint: server.URL}).ReadBytes(ctx, backend.Artifact{Owner: command.Owner, Name: "stderr"})
	stderr, _ := io.ReadAll(errFile)
	_ = errFile.Close()
	if result.ExitCode != 7 || result.Error != "" || string(out) != "out" || string(stderr) != "err" {
		t.Fatal(result, string(out), string(stderr))
	}
	missing := processRequest(t, "run", "child.1", "missing", "", false)
	missing.Command = "/no/such/program"
	if err := d.process(missing); err == nil {
		t.Fatal("startup failure was accepted")
	}
	for _, req := range []effects.Process{
		processRequest(t, "run", "child.1", "child.1/service.1", "exec sleep 60", true),
		processRequest(t, "run", "", "parent.1", "exec sleep 60", true),
		processRequest(t, "other", "child.1", "child.1/service.1", "exec sleep 60", true),
	} {
		if err := d.process(req); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.cleanup(ctx, effects.Scope{Run: "run", Scope: "child.1"}); err != nil {
		t.Fatal(err)
	}
	if d.state(effects.Scope{Run: "run"}).Pending != 1 || d.state(effects.Scope{Run: "other"}).Pending != 1 {
		t.Fatal("cleanup crossed ownership boundary")
	}
	if err := d.process(processRequest(t, "run", "child.1/grandchild.1", "late", "true", false)); err == nil {
		t.Fatal("late descendant admitted")
	}
	for _, run := range []string{"run", "other"} {
		if err := d.cleanup(ctx, effects.Scope{Run: run}); err != nil {
			t.Fatal(err)
		}
	}
}

type blockedCreate struct {
	testHarness
	entered chan struct{}
}

func (h *blockedCreate) CreateSession(ctx context.Context, _, _, _ string) (string, error) {
	close(h.entered)
	<-ctx.Done()
	return "", ctx.Err()
}
func TestCreatingSessionDoesNotBlockOtherRunAndCleanupCancelsIt(t *testing.T) {
	d := New()
	h := &blockedCreate{entered: make(chan struct{})}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return h, nil }
	done := make(chan error, 1)
	go func() {
		_, err := d.create(context.Background(), effects.Session{Run: "slow", Task: "agent.1"})
		done <- err
	}()
	<-h.entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.cleanup(ctx, effects.Scope{Run: "other"}); err != nil {
		t.Fatal(err)
	}
	if err := d.cleanup(ctx, effects.Scope{Run: "slow"}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if d.state(effects.Scope{Run: "slow"}).Pending != 0 {
		t.Fatal("creation retained after cancellation")
	}
}

func TestLostStartReplyReconcilesExistingTurnWithoutReplay(t *testing.T) {
	d := New()
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return &testHarness{}, nil }
	native := d.Handler()
	var starts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/turn" {
			starts.Add(1)
			var req effects.Turn
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				return
			}
			if err := d.turn(req); err != nil {
				t.Error(err)
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		native.ServeHTTP(w, r)
	}))
	defer server.Close()
	h := &Harness{Endpoint: server.URL}
	ctx := effects.WithOwner(context.Background(), effects.Owner{Run: "run", Task: "agent.1"})
	id, err := h.CreateSession(ctx, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx = effects.WithOwner(ctx, effects.Owner{Run: "run", Task: "agent.1/turn.1"})
	result, err := h.RunTurn(ctx, id, "go", nil, func(gimbal.AgentEvent) error { return nil })
	if err != nil || string(result.Output) != `"done"` || starts.Load() != 1 {
		t.Fatal(result, err, starts.Load())
	}
}
