package web

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	routes "github.com/tylergannon/gimbal/internal/skgo/links/onzggl3sn52xizlt"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"
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
func (*controlledRun) CleanupPending() bool           { return false }
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

	stopped, err := routes.Skgo_stopTurn(ctx, app.RequestEvent[params.Params]{}, routes.StopTurn{Run: "run.1", Turn: "coder.1/turn.2"})
	if err != nil || !stopped.Accepted || controlled.turn != "coder.1/turn.2" {
		t.Fatalf("stop turn = %+v, %v; target = %q", stopped, err, controlled.turn)
	}
	var killed gimbal.Killed
	if !errors.As(controlled.cause, &killed) || killed.Target != controlled.turn || killed.By != "person" {
		t.Fatalf("stop cause = %#v, want person kill of %q", controlled.cause, controlled.turn)
	}

	cancelled, err := routes.Skgo_cancelRun(ctx, app.RequestEvent[params.Params]{}, routes.CancelRun{Run: "run.1"})
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
	_, runtime, err := newProject(ctx, project, WithNoWeb())
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

	result, err := routes.Skgo_cancelRun(runtime.Context(), app.RequestEvent[params.Params]{}, routes.CancelRun{Run: id})
	if err != nil || !result.Accepted {
		t.Fatalf("cancel run = %+v, %v; want accepted", result, err)
	}
	var killed gimbal.Killed
	if err := <-done; !errors.As(err, &killed) || killed.By != "person" || killed.Target != "" {
		t.Fatalf("run error = %#v; want person kill of root scope", err)
	}

	snapshot, err := observation.NewRegistry(filepath.Join(project, ".gimbal")).Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Run.Status != observation.StatusCancelled {
		t.Fatalf("reloaded run status = %q, want %q", snapshot.Run.Status, observation.StatusCancelled)
	}
}

// End the real local run after the route has found its controller but before
// CancelScope reaches it, making the normal end-versus-click race deterministic.
type endingLocalRun struct {
	live.Controller
	end  chan<- struct{}
	done <-chan struct{}
}

func (r *endingLocalRun) CancelScope(scope string, cause error) error {
	close(r.end)
	<-r.done
	return r.Controller.CancelScope(scope, cause)
}

func TestCancelRunThatEndsDuringControlReturnsLocalNotFound(t *testing.T) {
	runs := live.NewRuns()
	started := make(chan string, 1)
	entered := make(chan struct{})
	end := make(chan struct{})
	done := make(chan struct{})
	result := make(chan error, 1)
	ctx := live.WithHook(gimbal.Project(t.Context(), t.TempDir()), func(id string, controller live.Controller) func() {
		release := runs.Hook(id, &endingLocalRun{Controller: controller, end: end, done: done})
		started <- id
		return release
	})
	go func() {
		result <- gimbal.Run(ctx, "local-end-race", nil, func(context.Context) error {
			close(entered)
			<-end
			return nil
		})
		close(done)
	}()
	id := <-started
	<-entered
	accepted, err := routes.Skgo_cancelRun(live.WithRuns(t.Context(), runs), app.RequestEvent[params.Params]{}, routes.CancelRun{Run: id})
	var status *skgo.HTTPError
	if accepted.Accepted || !errors.As(err, &status) || status.Status != http.StatusNotFound {
		t.Fatalf("local end race: accepted=%+v error=%v; want 404", accepted, err)
	}
	if want := "Run " + id + " is no longer running."; status.Message != want {
		t.Fatalf("local end race message = %q; want %q", status.Message, want)
	}
	if err := <-result; err != nil {
		t.Fatalf("local run: %v", err)
	}
}
