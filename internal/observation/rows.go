package observation

import "encoding/json"

// Tokens is the five counts every harness reports, flat. A count the
// provider did not report is 0: zero is a number, and nothing here says
// whether it was measured or absent.
type Tokens struct {
	Input      float64 `json:"input"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Output     float64 `json:"output"`
	Reasoning  float64 `json:"reasoning"`
}

// Usage is the tokens and the cost the harness stated, 0 when it stated none.
type Usage struct {
	Tokens
	StatedCost float64 `json:"stated_cost"`
}

// add sums one usage into another, field by field.
func (u Usage) add(other Usage) Usage {
	u.Input += other.Input
	u.CacheRead += other.CacheRead
	u.CacheWrite += other.CacheWrite
	u.Output += other.Output
	u.Reasoning += other.Reasoning
	u.StatedCost += other.StatedCost
	return u
}

// RunRow is the run itself. Times are Unix ms, as every time in a row is.
type RunRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Error   string `json:"error"`
	Started int64  `json:"started"`
	Ended   int64  `json:"ended"`
}

// Decision is one planner decision, kept as the record's sequence and the
// event object itself: what the planner chose is the log's own words.
type Decision struct {
	Seq  int64           `json:"seq"`
	Body json.RawMessage `json:"body"`
}

// ScopeRow is one scope instance. Key is the slash path, so the parent is
// the path above it and is not repeated. The root scope's key is "".
type ScopeRow struct {
	Run       string                     `json:"run"`
	Key       string                     `json:"key"`
	Name      string                     `json:"name"`
	Status    string                     `json:"status"`
	Error     string                     `json:"error"`
	Task      json.RawMessage            `json:"task,omitempty"`
	Began     int64                      `json:"began"`
	Ended     int64                      `json:"ended"`
	Values    map[string]json.RawMessage `json:"values"`
	Decisions []Decision                 `json:"decisions"`
}

// SessionRow is one agent conversation. Scope is where the session was
// created, which is not where its turns necessarily ran.
type SessionRow struct {
	Run     string `json:"run"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Adapter string `json:"adapter"`
	Model   string `json:"model"`
	Scope   string `json:"scope"`
	Parent  string `json:"parent"`
	Created int64  `json:"created"`
}

// TurnRow is one agent turn. Scope is where the turn ran, which is what a
// turn's tokens are charged to. Result is the recorded JSON text.
type TurnRow struct {
	Run         string `json:"run"`
	ID          string `json:"id"`
	Session     string `json:"session"`
	Scope       string `json:"scope"`
	Prompt      string `json:"prompt"`
	OutputType  string `json:"output_type"`
	Result      string `json:"result"`
	Error       string `json:"error"`
	Interrupted bool   `json:"interrupted"`
	Started     int64  `json:"started"`
	Ended       int64  `json:"ended"`
	Duration    int64  `json:"duration"`
}

// TurnUsageRow is what one model spent in one turn. It is the only table
// any roll-up reads.
type TurnUsageRow struct {
	Run   string `json:"run"`
	Turn  string `json:"turn"`
	Model string `json:"model"`
	Usage
}

// ModelCallRow is one step that reached a model: the drill below a turn.
// It is a fact and is never summed; Message is the normalized assistant
// message id the step named.
type ModelCallRow struct {
	Run     string `json:"run"`
	Turn    string `json:"turn"`
	Message string `json:"message"`
	Model   string `json:"model"`
	Tokens
	Started int64 `json:"started"`
	Ended   int64 `json:"ended"`
}
