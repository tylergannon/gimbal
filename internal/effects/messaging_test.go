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
