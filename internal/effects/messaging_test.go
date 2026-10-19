package effects

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

type silentMessages struct{}

func (silentMessages) PublishRequest(context.Context, backend.MessageRequest) error { return nil }
func (silentMessages) PublishEvent(context.Context, backend.MessageEvent) error     { return nil }
func (silentMessages) ListenEvents(ctx context.Context, _ backend.MessageSelection, _ time.Time, _ func(backend.MessageEvent) error) error {
	<-ctx.Done()
	return ctx.Err()
}
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
