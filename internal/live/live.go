// Package live is the seam between gimbal.Run and the web runtime. A run
// hands its Controller to whoever put a hook in the ctx, under the run's
// id, for as long as the run is in progress; the runtime keeps those in a
// table so an operator with only ids can steer a session or a loop's
// planner, or kill a scope or a turn.
package live

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// CancellationDeliveryError reports an actual backend delivery error or timeout.
// Lifecycle rejection before delivery is not a delivery failure.
type CancellationDeliveryError struct{ Err error }

func (e *CancellationDeliveryError) Error() string { return e.Err.Error() }
func (e *CancellationDeliveryError) Unwrap() error { return e.Err }

// ErrCancellationInProgress means the hosted cancellation/finish lock is held.
var ErrCancellationInProgress = errors.New("gimbal: run is cancelling or finishing")

// Controller is the face of one run in progress. The session, scope, and
// turn ids are the ones the run log carries: lap.3/coder.1 for a session,
// lap.3 for a scope, lap.3/coder.1/turn.2 for a turn. An unknown or already
// ended id is an error, except whole-run cancellation can retry retained
// remote cleanup.
type Controller interface {
	// AnswerInterview delivers the first answer to the pending interview
	// question id. An empty answer ends the interview normally.
	AnswerInterview(questionID, answer string) error
	// Steer sends message into the session's running turn as the person
	// watching the run, and reports whether it landed there.
	Steer(ctx context.Context, sessionID, message string) (landed bool, err error)
	// SteerLoop holds message for the planner of the loop scope key. It
	// reaches the planner at its next planning decision, whether or not a
	// turn is running when it is sent.
	SteerLoop(key, message string) error
	// CancelScope ends the scope's ctx with cause. For a terminal run with
	// pending cleanup, key "" retries cessation of its owned sessions.
	CancelScope(key string, cause error) error
	// CancelTurn ends only that turn's ctx with cause.
	CancelTurn(id string, cause error) error
	// CleanupPending retains stop control until owned remote sessions cease.
	CleanupPending() bool
}

// Hook is called when a run starts, with its id and Controller. The func it
// returns is called when the run's body has returned. CleanupPending keeps
// the controller reachable if remote cleanup remains unresolved.
type Hook func(id string, run Controller) (release func())

type hookKey struct{}

// WithHook puts hook in the ctx for every run started under it.
func WithHook(ctx context.Context, hook Hook) context.Context {
	return context.WithValue(ctx, hookKey{}, hook)
}

// FromContext returns the ctx's Hook, or nil when no runtime is watching.
func FromContext(ctx context.Context) Hook {
	hook, _ := ctx.Value(hookKey{}).(Hook)
	return hook
}

// Runs is the table of the runs in progress, by id. It rides in the runtime's
// context the way the observation registry does, so that the web runtime's
// own methods and the page's remote functions reach one table rather than
// each holding a view of the runs.
type Runs struct {
	mu       sync.Mutex
	runs     map[string]Controller
	retained map[string]bool
}

// NewRuns returns an empty table.
func NewRuns() *Runs {
	return &Runs{runs: map[string]Controller{}, retained: map[string]bool{}}
}

// Hook is the table's Hook: it holds run under id until the run's body has
// returned. Pass it to WithHook.
func (t *Runs) Hook(id string, run Controller) (release func()) {
	t.mu.Lock()
	t.runs[id] = run
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		if run.CleanupPending() {
			t.retained[id] = true
		} else {
			delete(t.runs, id)
		}
		t.mu.Unlock()
	}
}

// InProgress returns the controller of an active run or a terminal run with
// pending remote cleanup. Fully settled runs are gone from the table.
func (t *Runs) InProgress(id string) (Controller, error) {
	t.mu.Lock()
	run := t.runs[id]
	if t.retained[id] {
		if !run.CleanupPending() {
			delete(t.runs, id)
			delete(t.retained, id)
			run = nil
		}
	}
	t.mu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("gimbal: no run %q in progress", id)
	}
	return run, nil
}

type runsKey struct{}

// WithRuns puts the table in the ctx for anything serving from it to find.
func WithRuns(ctx context.Context, runs *Runs) context.Context {
	return context.WithValue(ctx, runsKey{}, runs)
}

// RunsFrom returns the ctx's table, or nil when no runtime put one there.
func RunsFrom(ctx context.Context) *Runs {
	runs, _ := ctx.Value(runsKey{}).(*Runs)
	return runs
}
