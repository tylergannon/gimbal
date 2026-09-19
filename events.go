package gimble

import (
	"encoding/json"
	"time"

	"github.com/tylergannon/polytype"
)

// JSONText is JSON source text carried opaquely by an event. Complete events
// contain one encoded value; delta events may contain only the next fragment.
// Text preserves provider-specific payloads while the event remains a closed
// Polytype shape.
type JSONText string

// AgentEvent is one OpenCode session event. Adapters supply Type, Data,
// Metadata, and NativeRef. Gimble assigns event identity and
// timestamps before it records or observes the event.
type AgentEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Created   int64           `json:"created,omitempty"`
	Data      json.RawMessage `json:"data"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	NativeRef json.RawMessage `json:"-"`
}

// LifecycleEvent is one typed change to a run's lifecycle. The interface is
// sealed: Gimble produces the concrete variants defined here.
type LifecycleEvent interface{ lifecycleEvent() }

// RunStarted records the beginning of a workflow run.
type RunStarted struct {
	Name string `json:"name"`
}

func (RunStarted) lifecycleEvent() {}

// RunEnded records the result of a workflow run.
type RunEnded struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

func (RunEnded) lifecycleEvent() {}

// RunCancelled records a workflow run stopped by context cancellation.
type RunCancelled struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Error  string `json:"error"`
}

func (RunCancelled) lifecycleEvent() {}

// ScopeBegan records entry into one scope instance. Loop marks a PromiseLoop's
// own scope, the one whose planner an operator can send a message to.
type ScopeBegan struct {
	Name string                  `json:"name"`
	Loop bool                    `json:"loop"`
	Task polytype.Optional[Task] `json:"task,omitzero"`
}

func (ScopeBegan) lifecycleEvent() {}

// ScopeEnded records exit from one scope instance.
type ScopeEnded struct {
	Error string `json:"error"`
}

func (ScopeEnded) lifecycleEvent() {}

// PlannerDecision records the task selected for the next dispatch. An absent
// Task records that the planner ended dispatch.
type PlannerDecision struct {
	Task polytype.Optional[Task] `json:"task,omitzero"`
}

func (PlannerDecision) lifecycleEvent() {}

// ValueArtifact describes a complete scope value stored under the run. File is
// relative to the run directory. Format is "text" for a Set string and "json"
// for every other value; Preview is bounded prompt text, not a second complete
// value.
type ValueArtifact struct {
	File    string `json:"file"`
	Size    int64  `json:"size"`
	Format  string `json:"format"`
	Preview string `json:"preview"`
}

// ValueSet records one value written into a scope. A small value is carried in
// Value. A file-backed value carries Artifact instead; the two are mutually
// exclusive. A later record for the same key may replace the inline recording
// with an artifact when aggregate prompt budgeting first abbreviates it.
type ValueSet struct {
	Key      string                           `json:"key"`
	Value    polytype.Optional[JSONText]      `json:"value,omitzero"`
	Artifact polytype.Optional[ValueArtifact] `json:"artifact,omitzero"`
}

func (ValueSet) lifecycleEvent() {}

// SessionCreated records a new agent session or fork.
type SessionCreated struct {
	Name    string `json:"name"`
	Adapter string `json:"adapter"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Workdir string `json:"workdir"`
	Parent  string `json:"parent"`
}

func (SessionCreated) lifecycleEvent() {}

// SessionClosed records that a scope closed one of its sessions. Error is
// set when the session had a native id and the adapter's Close failed;
// a session that never allocated a native id is always closed cleanly.
type SessionClosed struct {
	Error string `json:"error"`
}

func (SessionClosed) lifecycleEvent() {}

// TurnStarted records the beginning of one agent turn.
type TurnStarted struct {
	Prompt     string `json:"prompt"`
	OutputType string `json:"output_type"`
}

func (TurnStarted) lifecycleEvent() {}

// TurnEnded records the result and accounting for one agent turn. Usage is
// what the turn spent, per model: the harness's own turn report when it
// stated one, and otherwise the turn's step events summed under the
// session's model.
type TurnEnded struct {
	Result      JSONText      `json:"result"`
	Error       string        `json:"error"`
	Usage       []ModelUsage  `json:"usage"`
	Duration    time.Duration `json:"duration"`
	Interrupted bool          `json:"interrupted"`
}

func (TurnEnded) lifecycleEvent() {}

// InterviewQuestionAsked records a question waiting for a person's answer.
// LifecycleRecord supplies its scope and session placement.
type InterviewQuestionAsked struct {
	Name       string `json:"name"`
	QuestionID string `json:"question_id"`
	Question   string `json:"question"`
}

func (InterviewQuestionAsked) lifecycleEvent() {}

// InterviewQuestionAnswered records the answer accepted for one interview
// question. An empty answer records that the person ended the interview.
type InterviewQuestionAnswered struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

func (InterviewQuestionAnswered) lifecycleEvent() {}

// CommandStarted records a command RunCommand or Check is starting in the
// record's scope. ID is the scope's key and Name with an ordinal, as in
// lap.3/check.2; Workdir is absolute.
type CommandStarted struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Workdir string   `json:"workdir"`
}

func (CommandStarted) lifecycleEvent() {}

// CommandEnded records how the command ID ended. A command that ran has its
// exit code and no error. One that could not start has exit code -1 and the
// error, and one its ctx cancelled has both and Interrupted as well. Stdout
// and Stderr are exact for a small stream. Past the size a return keeps they
// are bounded head/tail excerpts with an omission marker and an absolute path.
// StdoutFile and StderrFile are run-relative paths to the complete streams;
// every command that starts has both files while it runs.
type CommandEnded struct {
	ID          string        `json:"id"`
	ExitCode    int           `json:"exit_code"`
	Stdout      string        `json:"stdout"`
	Stderr      string        `json:"stderr"`
	StdoutFile  string        `json:"stdout_file"`
	StderrFile  string        `json:"stderr_file"`
	Error       string        `json:"error"`
	Interrupted bool          `json:"interrupted"`
	Duration    time.Duration `json:"duration"`
}

func (CommandEnded) lifecycleEvent() {}

// SuperviseAttached records a reviewer attached to a worker turn.
type SuperviseAttached struct {
	Reviewer    string        `json:"reviewer"`
	Worker      string        `json:"worker"`
	Instruction string        `json:"instruction"`
	Interval    time.Duration `json:"interval"`
}

func (SuperviseAttached) lifecycleEvent() {}

// Steer records a message sent to a running session or dropped after it ended.
type Steer struct {
	Target  string `json:"target"`
	Source  string `json:"source"`
	Message string `json:"message"`
	Landed  bool   `json:"landed"`
}

func (Steer) lifecycleEvent() {}

// Killed is the cause of a ctx that an operator cancelled on purpose, and
// the lifecycle record of that kill. Target is the scope key or turn id
// that was killed; By is who did it, "" when unknown. context.Cause(ctx)
// returns it in every scope and turn under the target, so workflow code
// that cares checks errors.As(context.Cause(ctx), &killed); code that does
// not care sees ctx.Err() as before.
type Killed struct {
	Target string `json:"target"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}

func (Killed) lifecycleEvent() {}

func (k Killed) Error() string {
	s := "gimble: " + k.Target + " was killed"
	if k.By != "" {
		s += " by " + k.By
	}
	if k.Reason != "" {
		s += ": " + k.Reason
	}
	return s
}

// Complete marks the durable end of a run log. RecordingError reports an
// earlier failure in another log owned by the run; an absent Complete means
// the run log itself did not finish durably.
type Complete struct {
	RecordingError string `json:"recording_error"`
}

func (Complete) lifecycleEvent() {}

// LifecycleRecord places one lifecycle event in a run or project log. Seq is
// a gap-free ordinal in run.jsonl, which has exactly one writer for the
// run's lifetime. project.jsonl is written by every concurrently active
// run's own writer, so Seq is always 0 there; order those records by Time
// instead.
type LifecycleRecord struct {
	Seq     uint64                    `json:"seq"`
	Time    time.Time                 `json:"time"`
	Scope   string                    `json:"scope"`
	Session polytype.Optional[string] `json:"session,omitzero"`
	Turn    polytype.Optional[string] `json:"turn,omitzero"`
	Event   LifecycleEvent            `json:"event"`
}

// AgentRecord places one harness event in a session transcript.
type AgentRecord struct {
	Seq       uint64          `json:"seq"`
	Time      time.Time       `json:"time"`
	Scope     string          `json:"scope"`
	Session   string          `json:"session"`
	Turn      string          `json:"turn"`
	Event     AgentEvent      `json:"event"`
	NativeRef json.RawMessage `json:"native_ref,omitempty"`
}
