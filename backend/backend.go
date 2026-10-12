// Package backend defines the concrete, standard-library-only execution boundary.
package backend

import (
	"context"
	"encoding/json"
	"io"
)

type Owner struct{ Run, Scope, Task string }
type Process struct {
	Owner
	Workdir, Command string
	Args             []string
	Service          bool
}
type ProcessResult struct {
	ExitCode    int
	Interrupted bool
	Error       string
}
type Scope struct{ Run, Scope string }
type Session struct {
	Owner
	Harness, Model, Effort, Workdir, Parent string
}
type TaskFile struct {
	Name      string
	Reference string
	Bytes     []byte
}
type Turn struct {
	Files []TaskFile
	Owner
	Session, Prompt string
	Schema          json.RawMessage `json:",omitempty"`
}
type Target struct{ Run, ID, Message string }
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

// Route is retained independently of an execution environment's filesystem.
type Route struct {
	Name          string
	Configuration json.RawMessage
	Environment   string
}
type Events struct {
	Events           []json.RawMessage
	Cursor           int
	Gap, Unavailable bool
	Error            string
}
type Result struct {
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
type Service struct {
	Process
	Port     int
	Consumer string
}
type ServiceAddress struct{ ID, URL string }

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
	Acknowledge(context.Context, Target) error
	ReadBytes(context.Context, Artifact) (io.ReadCloser, error)
	WriteBytes(context.Context, Artifact, io.Reader) error
	Service(context.Context, Service) (ServiceAddress, error)
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
