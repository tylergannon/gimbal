package effects

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal/backend"
)

type pendingExecution struct{ backend.Execution }

func (pendingExecution) Pending(context.Context, backend.Scope) (backend.State, error) {
	return backend.State{Pending: 3, Error: "native cleanup remains"}, nil
}

func TestLocalStatePreservesPendingCleanup(t *testing.T) {
	ctx := WithConnection(t.Context(), backend.Connection{Execution: pendingExecution{}})
	var state backend.State
	if err := Call(ctx, "", "state", Scope{Run: "run", Scope: "scope"}, &state); err != nil {
		t.Fatal(err)
	}
	if state.Pending != 3 || state.Error != "native cleanup remains" {
		t.Fatalf("pending cleanup lost: %+v", state)
	}
}
