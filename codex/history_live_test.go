package codex

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestNativeTurnHistoryReconciliationLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1; uses gpt-5.6-luna")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	a := New().(*adapter)
	id, err := a.CreateSession(ctx, "gpt-5.6-luna", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := a.Close(ctx, id); err != nil {
			t.Error(err)
		}
	}()
	result, err := a.RunTurn(ctx, id, "Reply exactly HISTORY_OK", nil, func(gimbal.AgentEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Output) != `"HISTORY_OK"` {
		t.Fatalf("result=%s", result.Output)
	}
	snapshot, err := readThread(ctx, a.current(), id, "full")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || snapshot.Turns[0].Status != "completed" || turnAnswer(snapshot.Turns[0]) != "HISTORY_OK" {
		t.Fatalf("native history=%+v", snapshot)
	}
}
