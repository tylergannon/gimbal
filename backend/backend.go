// Package backend defines the concrete execution boundary.
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type Owner struct{ Run, Scope, Task string }
type Process struct {
	ExecutionIdentity ExecutionIdentity
	Deadline          time.Time `json:",omitzero"`
	Owner
	Workdir, Command string
	Args             []string
	Service          bool
	Requires         Requirements      `json:",omitempty"`
	Produces         bool              `json:",omitempty"`
	Yields           []string          `json:",omitempty"`
	Store            string            `json:",omitempty"`
	Environment      map[string]string `json:"-"`
}
type ProcessResult struct {
	ExitCode    int
	Interrupted bool
	Error       string
}
type Scope struct{ Run, Scope string }
type Session struct {
	ExecutionIdentity ExecutionIdentity
	Owner
	Harness, Model, Effort, Workdir, Parent string
}
type TaskFile struct {
	Name      string
	Reference string
	Bytes     []byte
}
type Turn struct {
	ExecutionIdentity ExecutionIdentity
	Deadline          time.Time `json:",omitzero"`
	Files             []TaskFile
	Requires          Requirements `json:",omitempty"`
	Owner
	Session, Prompt string
	Schema          json.RawMessage `json:",omitempty"`
}
type Target struct {
	Run, ID, Message  string
	ExecutionIdentity ExecutionIdentity
}
type Reply struct {
	ID    string
	Value bool
	Error string
}
type Poll struct {
	Run, ID string
	After   int
}
type Progress struct {
	Events []json.RawMessage
	Done   bool
	Result json.RawMessage
	Error  string
}
type State struct {
	Pending int
	Error   string
}

// Selection chooses one registered implementation and its validated configuration.
type Selection struct {
	Name          string
	Configuration json.RawMessage
}

// BackendSelection selects orchestration and secrets independently of messaging.
type BackendSelection struct {
	Orchestrator Selection
	Secrets      Selection
}

// Route is retained independently of an execution environment's filesystem.
type Route struct {
	Name          string
	Configuration json.RawMessage
	// Environment names an orchestrated task placement; it is never a display label.
	Environment string
	// Label describes a direct execution route in the console without selecting placement.
	Label    string            `json:",omitempty"`
	Backends *BackendSelection `json:",omitempty"`
}
type Events struct {
	Events           []json.RawMessage
	Cursor           int
	Gap, Unavailable bool
	Error            string
}
type Result struct {
	Interrupted   bool
	Done          bool
	Result        json.RawMessage
	Error         string
	InvalidOutput bool
	Expired       bool
}
type Artifact struct {
	Owner
	Name string
}
type Workflow struct {
	ID, Name, Workdir, Project string
	Roles                      map[string]string
	Input                      json.RawMessage
	TaskRoute                  Route
}
type Control struct{ Run, Action, Target, Message, By string }
type WorkflowStatus struct {
	ID, State, Error string
	CleanupPending   bool
}
type Observation struct {
	Snapshot         json.RawMessage
	Deltas           []json.RawMessage
	Cursor           string
	Gap, Unavailable bool
}

// Execution owns mutations and authoritative completion; event availability does not affect it.
type Execution interface {
	Session(context.Context, Session) (Reply, error)
	Turn(context.Context, Turn) error
	Process(context.Context, Process) error
	Result(context.Context, Target) (Result, error)
	Interrupt(context.Context, Target) error
	Steer(context.Context, Target) (bool, error)
	Close(context.Context, Target) error
	Cleanup(context.Context, Scope) error
	Pending(context.Context, Scope) (State, error)
	ReadBytes(context.Context, Artifact) (io.ReadCloser, error)
	WriteBytes(context.Context, Artifact, io.Reader) error
	StartWorkflow(context.Context, Workflow) (WorkflowStatus, error)
	WorkflowStatus(context.Context, Target) (WorkflowStatus, error)
	ControlWorkflow(context.Context, Control) (bool, error)
}
type TaskEvents interface {
	ReadEvents(context.Context, Poll) (Events, error)
}
type RunObservations interface {
	ReadObservation(context.Context, Target, string) (Observation, error)
}
type Connection struct {
	Messages     Messaging
	Orchestrator OrchestratorBackend
	Secrets      SecretsBackend
	Execution    Execution
	Events       TaskEvents
	Observations RunObservations
}

// TurnResult transports native output as opaque bytes, preserving whitespace,
// invalid JSON and UTF-8 exactly. Usage is the native accounting envelope.
type TurnResult struct {
	Output []byte
	Usage  json.RawMessage
}

// Close releases connection-owned messaging and orchestration handles after
// cleanup, never native work or scope service use.
func (c Connection) Close() error {
	var errs []error
	if closer, ok := c.Messages.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := c.Orchestrator.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := c.Secrets.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}
