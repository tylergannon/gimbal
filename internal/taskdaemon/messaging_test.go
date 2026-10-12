package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/internal/observation"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"github.com/tylergannon/gimbal/internal/effects"
)

type directMessages struct {
	*messagefiles.Store
	handle backend.MessageHandler
	ctx    context.Context
}

func (b *directMessages) PublishRequest(_ context.Context, r backend.MessageRequest) error {
	return b.handle(b.ctx, r)
}
func messagingOwner(t *testing.T) (*Daemon, *directMessages, context.Context) {
	t.Helper()
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	d := New()
	bus := &directMessages{Store: store, ctx: ctx}
	bus.handle = d.Messages(bus)
	return d, bus, effects.WithConnection(ctx, backend.Connection{Messages: bus})
}
func TestMessagingSessionSurvivesTwoCompletedTurns(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &testHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	h := &Harness{Name: "fake", Connection: backend.Connection{Messages: bus}}
	owner := effects.Owner{Run: "run", Scope: "scope", Task: "agent"}
	ctx = effects.WithOwner(ctx, owner)
	id, err := h.CreateSession(ctx, "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		owner.Task = fmt.Sprintf("turn.%d", i)
		turnCtx := effects.WithOwner(ctx, owner)
		out, err := h.RunTurn(turnCtx, id, "go", nil, nil)
		if err != nil || string(out.Output) != `"done"` {
			t.Fatalf("turn %d: %s %v", i, out.Output, err)
		}
	}
	if native.closes.Load() != 0 {
		t.Fatal("completed turn closed conversation")
	}
	if err = h.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	if native.closes.Load() != 1 {
		t.Fatal("explicit close absent")
	}
}
func TestMessagingInterruptStopsInflightTurn(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &testHarness{started: make(chan struct{})}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	h := &Harness{Name: "fake", Connection: backend.Connection{Messages: bus}}
	ctx = effects.WithOwner(ctx, effects.Owner{Run: "run", Task: "agent"})
	id, err := h.CreateSession(ctx, "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	turnCtx, cancel := context.WithCancel(effects.WithOwner(ctx, effects.Owner{Run: "run", Task: "turn"}))
	done := make(chan error, 1)
	go func() { _, err := h.RunTurn(turnCtx, id, "wait", nil, nil); done <- err }()
	<-native.started
	cancel()
	select {
	case err = <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("turn cancellation starved")
	}
	if err = effects.Cleanup(ctx, "", "run", ""); err != nil {
		t.Fatal(err)
	}
}
func TestMessagingServiceEstablishesThenScopeCleanupCompletes(t *testing.T) {
	_, _, ctx := messagingOwner(t)
	r := effects.Request("process", effects.Owner{Run: "run", Scope: "scope", Task: "service"})
	r.Process = &backend.Process{Owner: r.Owner, Command: "sh", Args: []string{"-c", "printf ready; sleep 30"}, Service: true}
	if err := effects.Submit(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err := effects.Await(ctx, r, "established", nil); err != nil {
		t.Fatal(err)
	}
	if err := effects.Cleanup(ctx, "", "run", "scope"); err != nil {
		t.Fatal(err)
	}
	event, err := effects.Await(ctx, r, "completed", nil)
	if err != nil {
		t.Fatal(err)
	}
	if event.Result == nil || event.Result.Process == nil || !event.Result.Process.Interrupted {
		t.Fatalf("result %#v", event.Result)
	}
}

func TestMessagingPreservesInvalidOutputAndRawBytes(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &invalidNativeHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	h := &Harness{Name: "fake", Connection: backend.Connection{Messages: bus}}
	ctx = effects.WithOwner(ctx, effects.Owner{Run: "run", Task: "agent"})
	id, err := h.CreateSession(ctx, "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	schema := json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}`)
	for i := 1; i <= 3; i++ {
		turn := effects.WithOwner(ctx, effects.Owner{Run: "run", Task: fmt.Sprintf("turn.%d", i)})
		result, err := h.RunTurn(turn, id, "answer", schema, nil)
		if i < 3 && (!errors.Is(err, gimbal.ErrInvalidOutput) || string(result.Output) != `{"n":"wrong"}`) {
			t.Fatalf("invalid answer %q: %v", result.Output, err)
		}
		if i == 3 && (err != nil || string(result.Output) != " \n {\"n\":2} \t") {
			t.Fatalf("raw answer changed %q: %v", result.Output, err)
		}
	}
}

func TestMessagingRejectsContradictionsBeforeEffects(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &testHarness{}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) {
		t.Error("invalid request opened a native session")
		return native, nil
	}
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	owner := backend.Owner{Run: "run", Scope: "scope", Task: "command"}
	valid := effects.Request("process", owner)
	valid.Process = &backend.Process{Owner: owner, Command: "sh", Args: []string{"-c", "touch \"" + marker + "\""}}
	tests := []backend.MessageRequest{}
	wrongOwner := valid
	payload := *valid.Process
	payload.Run = "other"
	wrongOwner.Process = &payload
	tests = append(tests, wrongOwner)
	wrongSession := valid
	wrongSession.SessionID = "unexpected"
	tests = append(tests, wrongSession)
	wrongKind := valid
	wrongKind.Kind = "turn"
	tests = append(tests, wrongKind)
	absent := valid
	absent.Process = nil
	tests = append(tests, absent)
	multiple := valid
	multiple.Session = &backend.Session{Owner: owner}
	tests = append(tests, multiple)
	session := effects.Request("session", owner)
	session.Session = &backend.Session{Run: "other", Task: owner.Task}
	session.SessionID = owner.Task
	tests = append(tests, session)
	turn := effects.Request("turn", owner)
	turn.SessionID = "one"
	turn.Turn = &backend.Turn{Owner: owner, Session: "two"}
	tests = append(tests, turn)
	for _, request := range tests {
		if err := bus.PublishRequest(ctx, request); err == nil {
			t.Errorf("admitted %+v", request)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command side effect exists: %v", err)
	}
	if len(d.tasks) != 0 || len(d.sessions) != 0 {
		t.Fatal("invalid request admitted work")
	}
	observe, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	_ = bus.ListenEvents(observe, backend.MessageSelection{Run: owner.Run}, valid.Created, func(e backend.MessageEvent) error { t.Errorf("invalid admission published %+v", e); return nil })
	// Correcting the same unadmitted request identity remains possible.
	if err := bus.PublishRequest(ctx, valid); err != nil {
		t.Fatal(err)
	}
	if _, err := effects.Await(ctx, valid, "completed", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("valid command did not execute", err)
	}
}

func TestMessagingExplicitBytesReachNativeExecutor(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &filesHarness{read: make(chan string, 1)}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	h := &Harness{Name: "test", Connection: backend.Connection{Messages: bus}}
	ctx = effects.WithOwner(ctx, backend.Owner{Run: "run", Task: "agent"})
	id, err := h.CreateSession(ctx, "model", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bytes := []byte("only-in-memory-" + ulid.Make().String())
	ctx = effects.WithOwner(effects.WithTaskFiles(ctx, []backend.TaskFile{{Name: "unique", Bytes: bytes}}), backend.Owner{Run: "run", Task: "turn"})
	out, err := h.RunTurn(ctx, id, "read supplied file", nil, nil)
	if err != nil || string(out.Output) != `"ok"` {
		t.Fatalf("%s %v", out.Output, err)
	}
	if got := <-native.read; got != string(bytes) {
		t.Fatalf("native read %q", got)
	}
}

type lostCompletion struct {
	*directMessages
	turnID string
}

func (b *lostCompletion) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	if r.Kind == "turn" {
		b.turnID = r.ID
	}
	return b.directMessages.PublishRequest(ctx, r)
}
func (b *lostCompletion) ListenEvents(ctx context.Context, s backend.MessageSelection, since time.Time, fn func(backend.MessageEvent) error) error {
	return b.Store.ListenEvents(ctx, s, since, func(e backend.MessageEvent) error {
		if e.RequestID == b.turnID && e.Kind == "completed" {
			return errors.New("injected listener loss after retained native completion")
		}
		return fn(e)
	})
}
func TestRetainedNativeSuccessSurvivesNormalWorkflowDeliveryLoss(t *testing.T) {
	d, direct, ctx := messagingOwner(t)
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return &testHarness{}, nil }
	bus := &lostCompletion{directMessages: direct}
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	ctx = observation.WithRegistry(effects.WithRunID(gimbal.Project(ctx, project), "lost-success"), registry)
	ctx = effects.WithConnection(ctx, backend.Connection{Messages: bus})
	err := gimbal.Run(ctx, "lost-success", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Name: "test", Connection: backend.Connection{Messages: bus}}, Model: "test"}}, func(ctx context.Context) error {
		_, err := gimbal.NewSession(ctx, "agent", t.TempDir()).Generate[gimbal.Text](ctx, "work")
		return err
	})
	if _, ok := errors.AsType[*backend.DeliveryUncertain](err); !ok {
		t.Fatalf("not unresolved: %v", err)
	}
	snapshot, err := registry.Snapshot("lost-success")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 {
		t.Fatalf("turns %d", len(snapshot.Turns))
	}
	for _, turn := range snapshot.Turns {
		if !turn.Unresolved || turn.Interrupted || !turn.EventsUnavailable {
			t.Fatalf("wrong turn outcome %+v", turn)
		}
		native := d.result(backend.Target{Run: "lost-success", ID: turn.ID})
		var result backend.TurnResult
		if !native.Done || native.Error != "" || json.Unmarshal(native.Result, &result) != nil || string(result.Output) != `"done"` {
			t.Fatalf("native success lost: %+v", native)
		}
	}
}

func TestIndependentMessagingInterruptPreservesNativeCancellation(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	native := &testHarness{started: make(chan struct{})}
	d.adapter = func(string) (gimbal.HarnessAdapter, error) { return native, nil }
	project := t.TempDir()
	registry := observation.NewRegistry(project)
	ctx = observation.WithRegistry(effects.WithRunID(gimbal.Project(ctx, project), "independent-interrupt"), registry)
	ctx = effects.WithConnection(ctx, backend.Connection{Messages: bus})
	done := make(chan error, 1)
	go func() {
		done <- gimbal.Run(ctx, "interrupt", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: &Harness{Name: "test", Connection: backend.Connection{Messages: bus}}, Model: "test"}}, func(ctx context.Context) error {
			_, err := gimbal.NewSession(ctx, "agent", project).Generate[gimbal.Text](ctx, "wait for independent interrupt")
			return err
		})
	}()
	<-native.started
	snapshot, err := registry.Snapshot("independent-interrupt")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 {
		t.Fatalf("turns %d", len(snapshot.Turns))
	}
	for _, turn := range snapshot.Turns {
		control := effects.Request("interrupt", backend.Owner{Run: turn.Run, Scope: turn.Scope, Task: "independent-control"})
		control.Target = &backend.Target{Run: turn.Run, ID: turn.ID}
		result, err := effects.MessageCall(ctx, control)
		if err != nil || result.Result == nil || result.Result.Unresolved {
			t.Fatalf("interrupt not confirmed: %+v %v", result, err)
		}
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if ctx.Err() != nil {
		t.Fatal("independent control cancelled workflow caller context")
	}
	snapshot, err = registry.Snapshot("independent-interrupt")
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range snapshot.Turns {
		if !turn.Interrupted || turn.Unresolved {
			t.Fatalf("wrong native outcome: %+v", turn)
		}
	}
}
