package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/tylergannon/gimbal/backend"
)

func TestAdmissionDoesNotReplayOrPropagateShutdownRetry(t *testing.T) {
	var calls atomic.Int32
	w := &actionWorker{ctx: context.Background(), seen: map[string]bool{}, handler: func(context.Context, backend.MessageRequest) error { calls.Add(1); return context.Canceled }}
	job := &river.Job[actionArgs]{Args: actionArgs{Request: backend.MessageRequest{ID: "one"}}}
	err := w.Work(context.Background(), job)
	var cancelled *rivertype.JobCancelError
	if !errors.As(err, &cancelled) || errors.Is(err, context.Canceled) {
		t.Fatalf("error must cancel River job without triggering shutdown retry: %v", err)
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate admitted")
	}
}

func integration(t *testing.T) (*transport, context.Context) {
	t.Helper()
	path := os.Getenv("GIMBAL_RIVER_TEST_DSN_FILE")
	if path == "" {
		t.Skip("set GIMBAL_RIVER_TEST_DSN_FILE to a private PostgreSQL DSN file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	queue := fmt.Sprintf("gimbal_test_%d", time.Now().UnixNano())
	c := configuration{DSNFile: path, Queue: queue, ControlQueue: queue + "_control", Workers: 1}
	tr, err := open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM river_job WHERE queue=ANY($1)", []string{c.Queue, c.ControlQueue})
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM gimbal_message_events WHERE queue=$1", c.Queue)
		tr.pool.Close()
	})
	return tr, ctx
}

func TestPostgresRetainedEventsIndependentAndLateOlder(t *testing.T) {
	tr, ctx := integration(t)
	since := time.Now().UTC().Truncate(time.Millisecond)
	newer := backend.MessageEvent{ID: "02", RequestID: "request", Created: since, Run: "run", SessionID: "session", Kind: "completed", Result: &backend.MessageResult{Output: []byte{0xff, '{'}, InvalidOutput: true, Usage: json.RawMessage(`{"tokens":3}`)}}
	if err := tr.PublishEvent(ctx, newer); err != nil {
		t.Fatal(err)
	}
	// A duplicate publication cannot replace the retained authoritative result.
	conflicting := newer
	conflicting.Result = &backend.MessageResult{Output: []byte("changed")}
	if err := tr.PublishEvent(ctx, conflicting); err != nil {
		t.Fatal(err)
	}
	older := newer
	older.ID = "01"
	older.Kind = "activity"
	older.Result = nil
	finished := errors.New("listener finished")
	var seen []string
	err := tr.ListenEvents(ctx, backend.MessageSelection{RequestID: "request", Run: "run", SessionID: "session"}, since, func(e backend.MessageEvent) error {
		seen = append(seen, e.ID)
		if e.ID == "02" {
			if !reflect.DeepEqual(e.Result, newer.Result) {
				t.Fatalf("result lost: %#v", e.Result)
			}
			// Appended after the later ID was already delivered, at exactly the same
			// millisecond. A moving timestamp/ULID cursor would lose this event.
			return tr.PublishEvent(ctx, older)
		}
		return finished
	})
	if !errors.Is(err, finished) || !reflect.DeepEqual(seen, []string{"02", "01"}) {
		t.Fatalf("seen %v, err %v", seen, err)
	}
	// A separate run observer can reconnect and obtain both retained records.
	seen = nil
	err = tr.ListenEvents(ctx, backend.MessageSelection{Run: "run"}, since, func(e backend.MessageEvent) error {
		seen = append(seen, e.ID)
		if len(seen) == 2 {
			return finished
		}
		return nil
	})
	if !errors.Is(err, finished) || !reflect.DeepEqual(seen, []string{"01", "02"}) {
		t.Fatalf("independent observer: %v %v", seen, err)
	}
}

func TestRiverQueueDeliveryControlsAndAdmissionCompletion(t *testing.T) {
	tr, ctx := integration(t)
	raw, _ := json.Marshal(tr.config)
	workerCtx, stop := context.WithCancel(ctx)
	workerDone := make(chan error, 1)
	blocked := make(chan struct{})
	release := make(chan struct{})
	control := make(chan struct{})
	var executions atomic.Int32
	go func() {
		workerDone <- serve(workerCtx, raw, func(_ context.Context, r backend.MessageRequest) error {
			switch r.ID {
			case "blocked":
				close(blocked)
				<-release
			case "control":
				close(control)
			case "native":
				executions.Add(1) // Ownership admitted; native work can remain active.
			case "error":
				return context.Canceled
			}
			return nil
		})
	}()
	t.Cleanup(func() {
		stop()
		select {
		case <-workerDone:
		case <-time.After(12 * time.Second):
			t.Error("worker did not stop")
		}
	})
	send := func(id, kind string) {
		t.Helper()
		if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: id, Kind: kind, Created: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	send("blocked", "session")
	select {
	case <-blocked:
	case <-ctx.Done():
		t.Fatal("action not delivered")
	}
	send("control", "cleanup")
	select {
	case <-control:
	case <-ctx.Done():
		t.Fatal("control starved by action admission")
	}
	close(release)
	send("native", "turn")
	send("native", "turn")
	send("error", "turn")
	deadline := time.NewTicker(20 * time.Millisecond)
	defer deadline.Stop()
	for {
		var completed, cancelled int
		err := tr.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE state='completed'),count(*) FILTER (WHERE state='cancelled')
   FROM river_job WHERE queue=ANY($1)`, []string{tr.config.Queue, tr.config.ControlQueue}).Scan(&completed, &cancelled)
		if err != nil {
			t.Fatal(err)
		}
		if completed == 4 && cancelled == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("jobs did not finalize: %d completed, %d cancelled", completed, cancelled)
		case <-deadline.C:
		}
	}
	if executions.Load() != 1 {
		t.Fatalf("duplicate native execution: %d", executions.Load())
	}
	var invalid int
	if err := tr.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE queue=ANY($1) AND (max_attempts<>1 OR attempt<>1)`, []string{tr.config.Queue, tr.config.ControlQueue}).Scan(&invalid); err != nil || invalid != 0 {
		t.Fatalf("unsafe retries: %d %v", invalid, err)
	}
	var actualQueue string
	if err := tr.pool.QueryRow(ctx, `SELECT queue FROM river_job WHERE args->'request'->>'ID'='control' AND queue=$1`, tr.config.ControlQueue).Scan(&actualQueue); err != nil {
		t.Fatalf("control queue selection: %v", err)
	}
	t.Log("real River: action queue blocked, independent control queue delivered cleanup; duplicate request admitted once; jobs completed on admission; error cancelled; every job attempt=1/max_attempts=1")
}

func TestPostgresSlowListenerDoesNotBlockPublisherOrAnotherListener(t *testing.T) {
	tr, ctx := integration(t)
	event := backend.MessageEvent{ID: "independent", Created: time.Now(), Kind: "completed", Result: &backend.MessageResult{}}
	if err := tr.PublishEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	slowCtx, stop := context.WithCancel(ctx)
	defer stop()
	entered := make(chan struct{})
	slowDone := make(chan error, 1)
	go func() {
		slowDone <- tr.ListenEvents(slowCtx, backend.MessageSelection{}, event.Created, func(backend.MessageEvent) error {
			close(entered)
			<-slowCtx.Done()
			return slowCtx.Err()
		})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("slow listener missing")
	}
	finished := errors.New("received")
	if err := tr.ListenEvents(ctx, backend.MessageSelection{}, event.Created, func(backend.MessageEvent) error { return finished }); !errors.Is(err, finished) {
		t.Fatal(err)
	}
	event.ID = "while-slow"
	if err := tr.PublishEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	stop()
	select {
	case <-slowDone:
	case <-ctx.Done():
		t.Fatal("listener did not release")
	}
}

func TestRiverShutdownDoesNotRequeueAdmission(t *testing.T) {
	tr, ctx := integration(t)
	raw, _ := json.Marshal(tr.config)
	workerCtx, stop := context.WithCancel(ctx)
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serve(workerCtx, raw, func(ctx context.Context, _ backend.MessageRequest) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	t.Cleanup(stop)
	if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: "interrupted", Kind: "session", Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("admission not started")
	}
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("worker did not stop")
	}
	var state string
	var attempt int
	if err := tr.pool.QueryRow(ctx, `SELECT state,attempt FROM river_job WHERE queue=$1`, tr.config.Queue).Scan(&state, &attempt); err != nil {
		t.Fatal(err)
	}
	if state != "cancelled" || attempt != 1 {
		t.Fatalf("shutdown replay risk: state=%s attempt=%d", state, attempt)
	}
	t.Log("real River shutdown left the interrupted admission cancelled at attempt 1, not available or retryable")
}
