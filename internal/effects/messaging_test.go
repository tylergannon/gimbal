package effects

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
)

type silentMessages struct{}

func (silentMessages) PublishRequest(context.Context, backend.MessageRequest) error { return nil }
func (silentMessages) PublishEvent(context.Context, backend.MessageEvent) error     { return nil }
func (silentMessages) ListenEvents(ctx context.Context, _ backend.MessageSelection, _ time.Time, _ func(backend.MessageEvent) error) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestAwaitReplaysCompletionFromEarlierWorkerClock(t *testing.T) {
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := Request("request", "identified-turn", Owner{Run: "run", Task: "turn"})
	e := backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: r.Created.Add(-time.Hour), Kind: "completed", Result: &backend.MessageResult{}}
	if err := store.PublishEvent(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	got, err := Await(WithConnection(ctx, backend.Connection{Messages: &replayMessages{Store: store}}), r, "completed", nil)
	if err != nil || got.ID != e.ID {
		t.Fatalf("completion: %+v, %v", got, err)
	}
}

type replayMessages struct{ *messagefiles.Store }

func (*replayMessages) PublishRequest(context.Context, backend.MessageRequest) error { return nil }
func TestCallerBudgetPreservesAcceptanceUncertainty(t *testing.T) {
	budget, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	ctx := WithConnection(budget, backend.Connection{Messages: silentMessages{}})
	r := Request(backend.NewExecutionIdentity("turn").RequestID, "identified-turn", Owner{Run: "run", Task: "turn"})
	_, err := Await(ctx, r, "completed", nil)
	uncertain, _ := errors.AsType[*backend.DeliveryUncertain](err)
	if uncertain == nil || uncertain.RequestID != r.ID || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wrong uncertainty: %v", err)
	}
}

type namedTestMessages struct {
	silentMessages
	got []backend.MessageRequest
	err error
}

func (*namedTestMessages) NamedEnvironments() bool { return true }
func (b *namedTestMessages) PublishRequest(_ context.Context, r backend.MessageRequest) error {
	b.got = append(b.got, r)
	return b.err
}

type testOrchestrator struct{}

func (testOrchestrator) AcquireEnvironment(context.Context, backend.Owner, string) (backend.Environment, error) {
	panic("unexpected capacity")
}
func (testOrchestrator) Release(context.Context, backend.Scope) error { return nil }

func TestNamedSubmitRejectsLegacyRoutingAndPreservesUncertainty(t *testing.T) {
	bus := &namedTestMessages{}
	owner := backend.Owner{Run: "run", Task: "process"}
	id := backend.NewExecutionIdentity("process")
	r := Request(id.RequestID, "identified-process", owner)
	r.ExecutionIdentity = id
	r.Process = &backend.Process{Owner: owner, ExecutionIdentity: id, Command: "true"}
	ctx := WithConnection(t.Context(), backend.Connection{Messages: bus})
	if err := Submit(ctx, r); err == nil || len(bus.got) != 0 {
		t.Fatalf("empty name admitted: %v", err)
	}
	ctx = WithEnvironment(ctx, "selected")
	if err := Submit(WithEndpoint(ctx, "http://legacy"), r); err == nil || len(bus.got) != 0 {
		t.Fatalf("legacy endpoint admitted: %v", err)
	}
	if err := Submit(WithConnection(ctx, backend.Connection{Messages: bus, Orchestrator: testOrchestrator{}}), r); err == nil || len(bus.got) != 0 {
		t.Fatalf("orchestrator admitted: %v", err)
	}
	failure := errors.New("connection lost")
	bus.err = failure
	err := Submit(ctx, r)
	var uncertain *backend.DeliveryUncertain
	if !errors.As(err, &uncertain) || !errors.Is(err, failure) || uncertain.RequestID != r.ID || len(bus.got) != 1 || bus.got[0].Environment != "selected" {
		t.Fatalf("publication result: %v, %+v", err, bus.got)
	}
}
