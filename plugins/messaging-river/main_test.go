package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func TestNamespaceValidationIsPureAndStrict(t *testing.T) {
	var registry gimbal.Registry
	if err := Register(&registry); err != nil {
		t.Fatal(err)
	}
	d, _ := registry.Messaging("river")
	raw := json.RawMessage(`{"DSNFile":"/does/not/exist","Environments":{"build":"worker-a"}}`)
	if err := d.ValidateConfiguration(raw); err != nil {
		t.Fatal(err)
	}
	if err := d.ValidateEnvironment(raw, "build"); err != nil {
		t.Fatal(err)
	}
	if err := d.ValidateEnvironment(raw, "missing"); err == nil {
		t.Fatal("unknown environment accepted")
	}
	for _, raw := range []string{`{"DSNFile":"x","extra":1}`, `{"DSNFile":"x"} {}`, `{"DSNFile":"x","Environments":{"x":" "}}`, `{"DSNFile":"x","ReconnectSeconds":-1}`} {
		if err := d.ValidateConfiguration(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	queue := fmt.Sprintf("gimbal_test_%d", time.Now().UnixNano())
	c := configuration{DSNFile: path, Queue: queue, ControlQueue: queue + "_control", Workers: 1}
	tr, err := open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.initialize(ctx); err != nil {
		tr.pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM river_job WHERE queue=ANY($1)", []string{c.Queue, c.ControlQueue})
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM gimbal_message_events WHERE queue=$1", c.Queue)
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM gimbal_message_acceptances WHERE namespace=$1", c.Queue)
		_, _ = tr.pool.Exec(cleanupCtx, "DELETE FROM gimbal_message_owners WHERE namespace=$1", c.Queue)
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
		workerDone <- serve(workerCtx, raw, backend.Dependencies{}, func(_ context.Context, r backend.MessageRequest) error {
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
		case err := <-workerDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("worker shutdown: %v", err)
			}
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
	case err := <-slowDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("listener shutdown: %v", err)
		}
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
		done <- serve(workerCtx, raw, backend.Dependencies{}, func(ctx context.Context, _ backend.MessageRequest) error {
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

func TestConnectBudgetDoesNotCloseConnection(t *testing.T) {
	tr, ctx := integration(t)
	var registry gimbal.Registry
	if err := Register(&registry); err != nil {
		t.Fatal(err)
	}
	definition, _ := registry.Messaging("river")
	raw, _ := json.Marshal(tr.config)
	connectCtx, cancel := context.WithCancel(ctx)
	bus, err := definition.Connect(connectCtx, raw, backend.Dependencies{})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	connection := bus.(*transport)
	defer func() { _ = connection.Close() }()
	event := backend.MessageEvent{ID: "after-connect", Created: time.Now(), Kind: "completed", Result: &backend.MessageResult{}}
	if err := bus.PublishEvent(ctx, event); err != nil {
		t.Fatalf("Connect budget ended, operation must still work: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bus.PublishEvent(ctx, event); err == nil {
		t.Fatal("explicit close must release the pool")
	}
}

func TestRepeatedConnectDoesNotTakeWorkerInitializationLock(t *testing.T) {
	tr, ctx := integration(t)
	lock, err := pgx.ConnectConfig(ctx, tr.pool.Config().ConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close(context.Background()) }()
	if _, err = lock.Exec(ctx, "SELECT pg_advisory_lock(745289004)"); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		connectCtx, cancel := context.WithTimeout(ctx, time.Second)
		caller, err := open(connectCtx, tr.config)
		cancel()
		if err != nil {
			t.Fatalf("caller %d must not wait for worker initialization lock: %v", i, err)
		}
		err = caller.PublishEvent(ctx, backend.MessageEvent{ID: fmt.Sprintf("cheap-%d", i), Created: time.Now()})
		_ = caller.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestConnectDoesNotInitializeMissingSchema(t *testing.T) {
	tr, ctx := integration(t)
	schema := fmt.Sprintf("river_caller_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := tr.pool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = tr.pool.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE") }()
	raw, err := os.ReadFile(tr.config.DSNFile)
	if err != nil {
		t.Fatal(err)
	}
	// Both PostgreSQL URI and keyword DSNs support search_path. Keep the new
	// private file in the test temp directory; never log connection strings.
	dsn := strings.TrimSpace(string(raw))
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal("invalid integration DSN")
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}
	c := tr.config
	c.DSNFile = filepath.Join(t.TempDir(), "dsn")
	if err := os.WriteFile(c.DSNFile, []byte(dsn), 0600); err != nil {
		t.Fatal(err)
	}
	caller, err := open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = caller.Close() }()
	var count int
	if err := tr.pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema=$1", schema).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("caller initialized %d tables", count)
	}
	err = caller.PublishRequest(ctx, backend.MessageRequest{ID: "before-worker", Kind: "turn", Created: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "start the configured message-worker") {
		t.Fatalf("missing actionable worker initialization error: %v", err)
	}
	// The same initialization performed by serve must make the store usable.
	if err := caller.initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err := caller.PublishRequest(ctx, backend.MessageRequest{ID: "after-worker-init", Kind: "turn", Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresNamedWorkersReceiveOnlyAddressedRequests(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"east": "worker-a", "west": "worker-b"}
	tr.config.ReconnectSeconds = 3
	queues := []string{workerQueue(tr.config.Queue, "worker-a", false), workerQueue(tr.config.Queue, "worker-a", true), workerQueue(tr.config.Queue, "worker-b", false), workerQueue(tr.config.Queue, "worker-b", true)}
	t.Cleanup(func() {
		_, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues)
		_, _ = tr.pool.Exec(context.Background(), "DELETE FROM gimbal_message_owners WHERE namespace=$1", tr.config.Queue)
	})
	if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: "not-admitted", Environment: "east", Kind: "identified-process"}); err == nil {
		t.Fatal("unregistered owner accepted")
	}
	type arrival struct{ worker, id, environment, assigned string }
	arrivals := make(chan arrival, 4)
	start := func(id string) (context.CancelFunc, <-chan error) {
		t.Helper()
		cfg := tr.config
		cfg.WorkerID = id
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		workerCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		ready := make(chan struct{})
		go func() {
			done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, func(_ context.Context, r backend.MessageRequest) error {
				arrivals <- arrival{id, r.ID, r.Environment, r.WorkerID}
				return nil
			})
		}()
		select {
		case <-ready:
		case err := <-done:
			cancel()
			t.Fatalf("worker %s: %v", id, err)
		case <-ctx.Done():
			cancel()
			t.Fatal(ctx.Err())
		}
		return cancel, done
	}
	stopA, doneA := start("worker-a")
	stopB, doneB := start("worker-b")
	defer func() {
		stopA()
		stopB()
		for _, done := range []<-chan error{doneA, doneB} {
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, context.Canceled) {
					t.Errorf("worker exit: %v", err)
				}
			case <-time.After(12 * time.Second):
				t.Error("worker did not stop")
			}
		}
	}()
	duplicate := tr.config
	duplicate.WorkerID = "worker-a"
	raw, _ := json.Marshal(duplicate)
	if err := serve(ctx, raw, backend.Dependencies{}, func(context.Context, backend.MessageRequest) error { t.Error("duplicate admitted"); return nil }); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate registration: %v", err)
	}
	for _, r := range []backend.MessageRequest{{ID: "one", Environment: "east", Kind: "identified-process"}, {ID: "two", Environment: "west", Kind: "identified-process"}} {
		if err := tr.PublishRequest(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]arrival{}
	for len(seen) < 2 {
		select {
		case a := <-arrivals:
			seen[a.id] = a
		case <-ctx.Done():
			t.Fatalf("missing addressed deliveries: %v", seen)
		}
	}
	if seen["one"] != (arrival{"worker-a", "one", "east", "worker-a"}) || seen["two"] != (arrival{"worker-b", "two", "west", "worker-b"}) {
		t.Fatalf("incorrect delivery: %+v", seen)
	}
	if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: "session", Environment: "east", Kind: "identified-session", Owner: backend.Owner{Run: "run", Task: "agent"}, SessionID: "agent"}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-arrivals:
		if got.worker != "worker-a" {
			t.Fatalf("session: %+v", got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	tr.config.Environments = map[string]string{"east": "worker-b", "west": "worker-b"}
	if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: "turn", Environment: "east", Kind: "identified-turn", Owner: backend.Owner{Run: "run", Task: "turn"}, SessionID: "agent"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.PublishRequest(ctx, backend.MessageRequest{ID: "new", Environment: "east", Kind: "identified-process", Owner: backend.Owner{Run: "run", Task: "new"}}); err != nil {
		t.Fatal(err)
	}
	seen = map[string]arrival{}
	for len(seen) < 2 {
		select {
		case a := <-arrivals:
			seen[a.id] = a
		case <-ctx.Done():
			t.Fatalf("missing rebind deliveries: %v", seen)
		}
	}
	if seen["turn"].worker != "worker-a" || seen["new"].worker != "worker-b" {
		t.Fatalf("rebind moved retained owner: %+v", seen)
	}
}

func TestPostgresNamedNativeCommand(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"selected": "native-worker"}
	tr.config.WorkerID = "native-worker"
	raw, err := json.Marshal(tr.config)
	if err != nil {
		t.Fatal(err)
	}
	queues := []string{workerQueue(tr.config.Queue, "native-worker", false), workerQueue(tr.config.Queue, "native-worker", true)}
	t.Cleanup(func() {
		_, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues)
		_, _ = tr.pool.Exec(context.Background(), "DELETE FROM gimbal_message_owners WHERE namespace=$1", tr.config.Queue)
	})
	daemon := taskdaemon.New(t.TempDir())
	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	ready := make(chan struct{})
	go func() {
		done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, daemon.Messages(tr))
	}()
	defer func() {
		stop()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("worker: %v", err)
			}
		case <-time.After(12 * time.Second):
			t.Error("worker did not stop")
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	owner := backend.Owner{Run: "run", Task: "command"}
	identity := backend.NewExecutionIdentity("process")
	req := effects.Request(identity.RequestID, "identified-process", owner)
	req.ExecutionIdentity = identity
	req.Process = &backend.Process{Owner: owner, ExecutionIdentity: identity, Command: "sh", Args: []string{"-c", "printf named-native"}, Workdir: t.TempDir()}
	callCtx := effects.WithEnvironment(effects.WithConnection(ctx, backend.Connection{Messages: tr}), "selected")
	event, err := effects.MessageCall(callCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if event.Environment != "selected" || event.WorkerID != "native-worker" || event.Result == nil || event.Result.Process == nil || event.Result.Process.ExitCode != 0 || string(event.Result.Stdout) != "named-native" {
		t.Fatalf("native result: %+v", event)
	}
	if err := effects.Cleanup(callCtx, "", owner.Run, "", "cleanup-native"); err != nil {
		t.Fatalf("named native cleanup: %v", err)
	}
}

func TestPostgresWorkerAuthorityReacquiresAfterConnectionLoss(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.ReconnectSeconds = 3
	owner, err := registerWorker(ctx, tr, "reconnect")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	pid := owner.conn.PgConn().PID()
	var killed bool
	if err = tr.pool.QueryRow(ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&killed); err != nil || !killed {
		t.Fatalf("terminate own authority connection: %v", err)
	}
	if err = owner.confirm(ctx); err == nil {
		t.Fatal("lost authority was accepted")
	}
	if err = owner.reacquire(ctx, tr, "reconnect"); err != nil {
		t.Fatal(err)
	}
	if err = owner.confirm(ctx); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := registerWorker(ctx, tr, "reconnect"); err == nil {
		duplicate.close()
		t.Fatal("duplicate obtained reacquired lock")
	}
}

func TestPostgresListenerReconnectsAndReplays(t *testing.T) {
	tr, ctx := integration(t)
	config := tr.pool.Config().Copy()
	var blocked atomic.Bool
	var failed atomic.Int32
	config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		if blocked.Load() {
			failed.Add(1)
			return nil, errors.New("injected connection loss")
		}
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, address)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	listener := &transport{config: tr.config, pool: pool}
	listener.config.ReconnectSeconds = 3
	finished := errors.New("received completion")
	result := make(chan error, 1)
	seen := make(chan backend.MessageEvent, 2)
	go func() {
		result <- listener.ListenEvents(ctx, backend.MessageSelection{RequestID: "reconnect"}, time.Time{}, func(e backend.MessageEvent) error {
			seen <- e
			if e.Kind == "completed" {
				return finished
			}
			return nil
		})
	}()
	event := backend.MessageEvent{ID: "reconnect-accepted", RequestID: "reconnect", Created: time.Now(), Kind: "accepted"}
	if err := tr.PublishEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	select {
	case <-seen:
	case <-ctx.Done():
		t.Fatal("initial event missing")
	}
	blocked.Store(true)
	pool.Reset()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for failed.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("listener did not observe disconnect")
		case <-time.After(10 * time.Millisecond):
		}
	}
	event.ID = "reconnect-completed"
	event.Kind = "completed"
	event.Result = &backend.MessageResult{Output: []byte("replayed")}
	if err := tr.PublishEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	blocked.Store(false)
	select {
	case got := <-seen:
		if got.ID != event.ID || string(got.Result.Output) != "replayed" {
			t.Fatalf("wrong replay: %+v", got)
		}
	case <-ctx.Done():
		t.Fatal("completion was not replayed")
	}
	if err := <-result; !errors.Is(err, finished) {
		t.Fatalf("listener: %v", err)
	}
}

func TestPostgresListenerReconnectBudgetExhaustion(t *testing.T) {
	tr, ctx := integration(t)
	config := tr.pool.Config().Copy()
	config.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("injected outage") }
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	listener := &transport{config: tr.config, pool: pool}
	listener.config.ReconnectSeconds = 1
	started := time.Now()
	err = listener.ListenEvents(ctx, backend.MessageSelection{RequestID: "unavailable"}, time.Time{}, func(backend.MessageEvent) error { t.Error("unexpected event"); return nil })
	if err == nil || !strings.Contains(err.Error(), "reconnect budget exhausted") || time.Since(started) > 3*time.Second {
		t.Fatalf("reconnect not bounded: %v after %v", err, time.Since(started))
	}
}

type droppedCompletion struct {
	backend.Messaging
	dropped chan struct{}
}

func (d *droppedCompletion) PublishEvent(ctx context.Context, event backend.MessageEvent) error {
	if event.Kind == "completed" {
		select {
		case d.dropped <- struct{}{}:
		default:
		}
		return errors.New("injected completion delivery loss")
	}
	return d.Messaging.PublishEvent(ctx, event)
}

func TestPostgresNamedNativeCompletionRecoversAfterRestart(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"selected": "restart-worker"}
	tr.config.WorkerID = "restart-worker"
	raw, err := json.Marshal(tr.config)
	if err != nil {
		t.Fatal(err)
	}
	queues := []string{workerQueue(tr.config.Queue, "restart-worker", false), workerQueue(tr.config.Queue, "restart-worker", true)}
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues) })
	root := t.TempDir()
	marker := filepath.Join(root, "count")
	dropped := &droppedCompletion{Messaging: tr, dropped: make(chan struct{}, 1)}
	start := func(daemon *taskdaemon.Daemon, bus backend.Messaging, recover bool) (context.CancelFunc, <-chan error) {
		t.Helper()
		workerCtx, cancel := context.WithCancel(ctx)
		ready := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error {
				if recover {
					if err := daemon.RecoverMessages(workerCtx, bus); err != nil {
						return err
					}
				}
				close(ready)
				return nil
			}}, daemon.Messages(bus))
		}()
		select {
		case <-ready:
		case err := <-done:
			cancel()
			t.Fatalf("start: %v", err)
		case <-ctx.Done():
			cancel()
			t.Fatal(ctx.Err())
		}
		return cancel, done
	}
	stop, done := start(taskdaemon.New(root), dropped, false)
	identity := backend.NewExecutionIdentity("process")
	owner := backend.Owner{Run: "run", Task: "once"}
	req := effects.Request(identity.RequestID, "identified-process", owner)
	req.ExecutionIdentity = identity
	req.Process = &backend.Process{Owner: owner, ExecutionIdentity: identity, Command: "sh", Args: []string{"-c", "printf x >> \"$1\"; printf recovered", "sh", marker}, Workdir: root}
	callCtx := effects.WithEnvironment(effects.WithConnection(ctx, backend.Connection{Messages: tr}), "selected")
	if err := effects.Submit(callCtx, req); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dropped.dropped:
	case <-ctx.Done():
		t.Fatal("completion was not reached")
	}
	stop()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	stop, done = start(taskdaemon.New(root), tr, true)
	defer func() { stop(); <-done }()
	event, err := effects.Await(callCtx, req, "completed", nil)
	if err != nil {
		t.Fatal(err)
	}
	if event.WorkerID != "restart-worker" || event.Result == nil || string(event.Result.Stdout) != "recovered" {
		t.Fatalf("recovered event: %+v", event)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "x" {
		t.Fatalf("native execution repeated: %q %v", data, err)
	}
}

func TestPostgresNamedCleanupReachesEveryRetainedOwner(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"east": "cleanup-a", "west": "cleanup-b"}
	queues := []string{}
	stops := []context.CancelFunc{}
	dones := []<-chan error{}
	for _, worker := range []string{"cleanup-a", "cleanup-b"} {
		cfg := tr.config
		cfg.WorkerID = worker
		raw, _ := json.Marshal(cfg)
		daemon := taskdaemon.New(t.TempDir())
		workerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		ready := make(chan struct{})
		go func() {
			done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, daemon.Messages(tr))
		}()
		select {
		case <-ready:
		case err := <-done:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		stops = append(stops, stop)
		dones = append(dones, done)
		queues = append(queues, workerQueue(tr.config.Queue, worker, false), workerQueue(tr.config.Queue, worker, true))
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
		for _, done := range dones {
			<-done
		}
	}()
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues) })
	call := func(environment, scope, task string) error {
		identity := backend.NewExecutionIdentity("process")
		owner := backend.Owner{Run: "cleanup-run", Scope: scope, Task: task}
		req := effects.Request(identity.RequestID, "identified-process", owner)
		req.ExecutionIdentity = identity
		req.Process = &backend.Process{Owner: owner, ExecutionIdentity: identity, Command: "sh", Args: []string{"-c", "printf ok"}, Workdir: t.TempDir()}
		selected := effects.WithEnvironment(effects.WithConnection(ctx, backend.Connection{Messages: tr}), environment)
		_, err := effects.MessageCall(selected, req)
		return err
	}
	if err := call("east", "parent/a", "one"); err != nil {
		t.Fatal(err)
	}
	if err := call("west", "parent/b", "two"); err != nil {
		t.Fatal(err)
	}
	cleanupCtx := effects.WithConnection(ctx, backend.Connection{Messages: tr})
	if err := effects.Cleanup(cleanupCtx, "", "cleanup-run", "parent", "scope-cleanup"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ env, scope string }{{"east", "parent/a"}, {"west", "parent/b"}} {
		if err := call(v.env, v.scope, "after-"+v.env); err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("owner %s did not close scope: %v", v.env, err)
		}
	}
}

func TestPostgresWorkerStopsAdmissionWhenAuthorityRecoveryExhausts(t *testing.T) {
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"selected": "blocked-worker"}
	tr.config.WorkerID = "blocked-worker"
	tr.config.ReconnectSeconds = 1
	raw, _ := json.Marshal(tr.config)
	queue := workerQueue(tr.config.Queue, "blocked-worker", false)
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=$1", queue) })
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	ready := make(chan struct{})
	done := make(chan error, 1)
	var admitted atomic.Int32
	go func() {
		done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, func(context.Context, backend.MessageRequest) error { admitted.Add(1); return nil })
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	key := authorityKey(tr.config.Queue, "blocked-worker")
	var pid int32
	err := tr.pool.QueryRow(ctx, `SELECT pid FROM pg_locks WHERE locktype='advisory' AND granted AND objsubid=1
 AND classid::bigint=((hashtextextended($1,0) >> 32) & 4294967295)
 AND objid::bigint=(hashtextextended($1,0) & 4294967295)`, key).Scan(&pid)
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := pgx.ConnectConfig(ctx, tr.pool.Config().ConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close(context.Background())
	acquired := make(chan error, 1)
	go func() {
		_, err := blocker.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", key)
		acquired <- err
	}()
	var waiting bool
	for !waiting {
		err = tr.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, blocker.PgConn().PID()).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if !waiting {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	var terminated bool
	if err := tr.pool.QueryRow(ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&terminated); err != nil || !terminated {
		t.Fatalf("terminate own authority: %v", err)
	}
	if err := <-acquired; err != nil {
		t.Fatal(err)
	}
	request := backend.MessageRequest{ID: "queued-during-loss", Environment: "selected", WorkerID: "blocked-worker", Kind: "identified-process"}
	if _, err := tr.client.Insert(ctx, actionArgs{Request: request}, &river.InsertOpts{Queue: queue, MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "could not reacquire registration") {
			t.Fatalf("worker exit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("worker did not stop after loss")
	}
	if admitted.Load() != 0 {
		t.Fatalf("admitted %d requests after authority loss", admitted.Load())
	}
	var state string
	if err := tr.pool.QueryRow(ctx, "SELECT state::text FROM river_job WHERE queue=$1", queue).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state == "cancelled" || state == "discarded" || state == "completed" {
		t.Fatalf("queued request spent under lost authority: %s", state)
	}
}
