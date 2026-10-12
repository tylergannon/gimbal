package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

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
