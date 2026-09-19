package web

import (
	"context"
	"errors"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/live"
	routes "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
)

type controlledRun struct {
	turn  string
	scope string
	cause error
}

func (*controlledRun) AnswerInterview(string, string) error { return nil }
func (*controlledRun) Steer(context.Context, string, string) (bool, error) {
	return false, nil
}
func (*controlledRun) SteerLoop(string, string) error { return nil }
func (r *controlledRun) CancelTurn(turn string, cause error) error {
	r.turn, r.cause = turn, cause
	return nil
}
func (r *controlledRun) CancelScope(scope string, cause error) error {
	r.scope, r.cause = scope, cause
	return nil
}

func TestRunPageControlsReachTheLiveRun(t *testing.T) {
	runs := live.NewRuns()
	controlled := &controlledRun{}
	release := runs.Hook("run.1", controlled)
	defer release()
	ctx := live.WithRuns(t.Context(), runs)

	stopped, err := routes.Skgo_stopTurn(ctx, routes.StopTurn{Run: "run.1", Turn: "coder.1/turn.2"})
	if err != nil || !stopped.Accepted || controlled.turn != "coder.1/turn.2" {
		t.Fatalf("stop turn = %+v, %v; target = %q", stopped, err, controlled.turn)
	}
	var killed gimble.Killed
	if !errors.As(controlled.cause, &killed) || killed.Target != controlled.turn || killed.By != "person" {
		t.Fatalf("stop cause = %#v, want person kill of %q", controlled.cause, controlled.turn)
	}

	cancelled, err := routes.Skgo_cancelRun(ctx, routes.CancelRun{Run: "run.1"})
	if err != nil || !cancelled.Accepted || controlled.scope != "" {
		t.Fatalf("cancel run = %+v, %v; scope = %q", cancelled, err, controlled.scope)
	}
	if !errors.As(controlled.cause, &killed) || killed.Target != "" || killed.By != "person" {
		t.Fatalf("cancel cause = %#v, want person kill of root scope", controlled.cause)
	}
}
