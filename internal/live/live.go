// Package live is the seam between gimble.Run and the web runtime. A run
// hands its Controller to whoever put a hook in the ctx, under the run's
// id, for as long as the run is in progress; the runtime keeps those in a
// table so an operator with only ids can steer a session or kill a scope
// or a turn.
package live

import "context"

// Controller is the face of one run in progress. The session, scope, and
// turn ids are the ones the run log carries: lap.3/coder.1 for a session,
// lap.3 for a scope, lap.3/coder.1/turn.2 for a turn. An unknown or already
// ended id is an error.
type Controller interface {
	// Steer sends message into the session's running turn as the person
	// watching the run, and reports whether it landed there.
	Steer(ctx context.Context, sessionID, message string) (landed bool, err error)
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
