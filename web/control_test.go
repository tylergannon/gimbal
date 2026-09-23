package web

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/live"
	"github.com/tylergannon/gimble/internal/observation"
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

func TestCancelRunPersistsACancelledRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runtime.Run(ctx, "cancelled-run", nil, func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return context.Cause(ctx)
		})
	}()
	<-entered
	id := startedRunID(t, project)

	result, err := routes.Skgo_cancelRun(runtime.ctx, routes.CancelRun{Run: id})
	if err != nil || !result.Accepted {
		t.Fatalf("cancel run = %+v, %v; want accepted", result, err)
	}
	var killed gimble.Killed
	if err := <-done; !errors.As(err, &killed) || killed.By != "person" || killed.Target != "" {
		t.Fatalf("run error = %#v; want person kill of root scope", err)
	}

	snapshot, err := observation.NewRegistry(filepath.Join(project, ".gimble")).Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != observation.StatusCancelled {
		t.Fatalf("reloaded run status = %q, want %q", snapshot.Run.Status, observation.StatusCancelled)
	}
}
