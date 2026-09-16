package gimble

import (
	"context"
	"encoding/json"
)

// HarnessAdapter is the agent-specific code for one coding-agent harness
// (for example, Codex or Claude Code). It is untyped: a raw JSON Schema goes
// in, raw JSON comes out; Generate validates and decodes. Whatever an
// adapter allocates for a session (a process, a subscription, a map entry)
// is released by Close, which the runtime calls for real when the scope
// that owns the session ends.
type HarnessAdapter interface {
	// CreateSession reserves one adapter session and returns its id. A harness
	// may start its native process or conversation lazily in RunTurn. effort
	// is the reasoning effort the role was bound to, empty when the run left
	// it to the harness's own default.
	CreateSession(ctx context.Context, model, effort, workdir string) (string, error)

	// RunTurn runs one turn and blocks until it ends. It passes every event
	// to onEvent as it arrives and returns the turn's output and the
	// harness's own report of what the turn spent. Cancelling ctx interrupts
	// the native turn and returns ctx.Err().
	RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(AgentEvent) error) (TurnResult, error)

	// Steer sends a message into the session's running turn and reports
	// whether it landed there. landed is false, with a nil error, when no
	// turn was running to receive it, including when the turn ended while
	// the steer was on its way. The error is for a harness that could not
	// be reached.
	Steer(ctx context.Context, sessionID, message string) (landed bool, err error)

	// Fork returns a new native session with the conversation so far.
	Fork(ctx context.Context, sessionID string) (string, error)

	// Close releases whatever the adapter holds for the session. Idempotent.
	// Called by the runtime when the owning scope ends.
	Close(ctx context.Context, sessionID string) error
}

// ModelBinding is what one role runs on: the harness that serves it, the
// model, and the reasoning effort. A run binds every role its workflow
// names; the workflow itself names only the role.
type ModelBinding struct {
	Adapter HarnessAdapter
	Model   string
	// Effort is the reasoning effort, empty to leave it to the harness.
	Effort string
}

// TurnResult is what one turn produced.
type TurnResult struct {
	// Output is the structured result when a schema was sent, and the final
	// message encoded as a JSON string when none was. Generate validates and
	// decodes it.
	Output json.RawMessage

	// Usage is the harness's own report for the turn, keyed by model name.
	// It is nil when the harness states no turn report; the session then
	// accounts for the turn from its step events alone.
	Usage map[string]Usage
}
