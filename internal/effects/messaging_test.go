package effects

import (
	"context"
	"strings"
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
func TestNoExecutorIsBoundedAcceptanceUncertainty(t *testing.T) {
	ctx := WithConnection(context.Background(), backend.Connection{Messages: silentMessages{}})
	r := Request("turn", Owner{Run: "run", Task: "turn"})
	if err := Submit(ctx, r); err != nil {
		t.Fatal(err)
	}
	_, err := await(ctx, r, "completed", nil, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), r.ID) || !strings.Contains(err.Error(), "acceptance unconfirmed") || strings.Contains(err.Error(), "execution failed") {
		t.Fatalf("wrong uncertainty: %v", err)
	}
}
