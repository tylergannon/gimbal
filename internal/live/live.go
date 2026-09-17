// Package live is the seam between gimble.Run and the web runtime. A run
// hands its Controller to whoever put a hook in the ctx, under the run's
// id, for as long as the run is in progress; the runtime keeps those in a
// table so an operator with only ids can steer a session or a loop's
// planner, or kill a scope or a turn.
package live

import (
	"context"
	"fmt"
	"sync"
)

// Controller is the face of one run in progress. The session, scope, and
// turn ids are the ones the run log carries: lap.3/coder.1 for a session,
// lap.3 for a scope, lap.3/coder.1/turn.2 for a turn. An unknown or already
// ended id is an error.
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
	// CancelScope ends the scope's ctx with cause.
	CancelScope(key string, cause error) error
	// CancelTurn ends only that turn's ctx with cause.
	CancelTurn(id string, cause error) error
}

// Hook is called when a run starts, with its id and Controller. The func it
// returns is called when the run's body has returned and the Controller can
// no longer reach anything.
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
	mu   sync.Mutex
	runs map[string]Controller
}

// NewRuns returns an empty table.
func NewRuns() *Runs {
	return &Runs{runs: map[string]Controller{}}
}

// Hook is the table's Hook: it holds run under id until the run's body has
// returned. Pass it to WithHook.
func (t *Runs) Hook(id string, run Controller) (release func()) {
	t.mu.Lock()
	t.runs[id] = run
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		delete(t.runs, id)
		t.mu.Unlock()
	}
}

// InProgress returns the run in progress under id. A run that has finished is
// gone from the table, so it reads the same as one that never existed: an
// error, which is what an operator acting on a stale page is owed.
func (t *Runs) InProgress(id string) (Controller, error) {
	t.mu.Lock()
	run := t.runs[id]
	t.mu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("gimble: no run %q in progress", id)
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
