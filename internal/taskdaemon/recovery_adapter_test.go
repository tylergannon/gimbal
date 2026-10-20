package taskdaemon

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
)

type countingHarness struct {
	testHarness
	turns atomic.Int32
}

func (h *countingHarness) RunTurn(ctx context.Context, native, prompt string, schema json.RawMessage, event func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	h.turns.Add(1)
	return h.testHarness.RunTurn(ctx, native, prompt, schema, event)
}

// A recovered accepted turn is settled from executor evidence, not by asking the
// session's adapter. Seam H audit: recovery is provider-specific and does not
// cross HarnessAdapter.RunTurn, so substituting the adapter cannot change it.
func TestRecoveredTurnDoesNotCrossSessionAdapter(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	owner := effects.Owner{Run: "run", Task: "agent"}
	session := effects.Session{ExecutionIdentity: backend.NewExecutionIdentity("session"), Owner: owner, Harness: "codex", Model: "luna", Workdir: root}
	if err := d.writeState("sessions", key(owner.Run, owner.Task), savedSession{Harness: "codex", Request: session, Native: string(executor.NewSessionID())}); err != nil {
		t.Fatal(err)
	}
	turn := effects.Turn{ExecutionIdentity: backend.NewExecutionIdentity("turn"), Owner: effects.Owner{Run: "run", Task: "turn"}, Session: "agent", Prompt: "must not dispatch"}
	input, _ := json.Marshal(turn)
	if err := d.writeState("tasks", key("run", "turn"), savedTask{Owner: turn.Owner, Turn: &turn, Input: input}); err != nil {
		t.Fatal(err)
	}
	fake := &countingHarness{}
	// New restores sessions before any adapter hook can be installed, so a
	// restored session always holds binding.Adapter's choice.
	restored := New(root)
	s := restored.sessions[key(owner.Run, owner.Task)]
	if _, ok := s.adapter.(*agentexecutor.Adapter); !ok {
		t.Fatalf("restored session adapter %T", s.adapter)
	}
	s.adapter = fake
	restored.ensureRecovery()
	if restored.restoreErr != nil {
		t.Fatal(restored.restoreErr)
	}
	select {
	case <-restored.tasks[key("run", "turn")].done:
	case <-time.After(time.Second):
		t.Fatal("recovered turn did not settle")
	}
	if got := fake.turns.Load(); got != 0 {
		t.Fatalf("recovery called the selected adapter's RunTurn %d times", got)
	}
	if result := restored.result(backend.Target{Run: "run", ID: "turn"}); !result.Done || !result.Unresolved {
		t.Fatalf("recovered turn outcome %+v", result)
	}
}
