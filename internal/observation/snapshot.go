package observation

import (
	"encoding/json"

	"github.com/tylergannon/gimble/internal/sessionstate"
)

// Run statuses. Only a lifecycle record sets one: a native part, step or
// execution event never decides that a workflow finished.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Scope statuses. An ended scope with no error is "ended", never
// "succeeded": a range-body error in a PromiseLoop leaves the task and loop scopes
// ended with an empty error while the run's own error is set.
const (
	StatusEnded = "ended"
)

// Interview statuses describe whether its question is still waiting for a
// person or has accepted its one answer.
const (
	InterviewStatusPending  = "pending"
	InterviewStatusAnswered = "answered"
)

// Placement is where a native event happened in the workflow. It lives
// outside the native envelope, so native IDs stay unchanged inside it and
// two concurrent turns never share a projection.
type Placement struct {
	Scope   string `json:"scope"`
	Session string `json:"session"`
	Turn    string `json:"turn"`
}

// Transcript is one turn's complete session projection snapshot and the
// current native provenance of each of its messages. It is not a table: the
// message text it projects stays in the session log.
type Transcript struct {
	Snapshot   sessionstate.Snapshot      `json:"snapshot"`
	Provenance map[string]json.RawMessage `json:"provenance"`
}

// RunSnapshot is the complete public observation of one run: the tables, the
// roll-ups computed from them, and one transcript per turn. It is the body of
// GET /api/runs/:runID, the first SSE frame, and the SSR load's `snapshot`
// property.
type RunSnapshot struct {
	Stream      string                      `json:"stream"`
	Position    uint64                      `json:"position"`
	Run         RunRow                      `json:"run"`
	Scopes      map[string]ScopeRow         `json:"scopes"`
	Sessions    map[string]SessionRow       `json:"sessions"`
	Interviews  map[string]InterviewRow     `json:"interviews"`
	Turns       map[string]TurnRow          `json:"turns"`
	TurnUsage   map[string]map[string]Usage `json:"turn_usage"`
	ModelCalls  map[string][]ModelCallRow   `json:"model_calls"`
	Commands    map[string]CommandRow       `json:"commands"`
	Totals      Totals                      `json:"totals"`
	Transcripts map[string]Transcript       `json:"transcripts"`
}
