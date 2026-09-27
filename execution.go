package gimbal

import (
	"context"
	"io"
)

// ExecutionBackend supplies named execution environments for one Run. Run owns
// the backend and closes it after its sessions and services have closed.
// Provider configuration belongs to the implementation's constructor.
type ExecutionBackend interface {
	io.Closer
	Resolve(context.Context, string) (ExecutionEnvironment, error)
}

// ExecutionEnvironment is one live place shared by all actions using its name.
// Harness adapters run against this environment's filesystem and processes.
// Resolving the same name must not silently replace a lost environment.
type ExecutionEnvironment interface {
	Harness(WorkflowRole) HarnessAdapter
	Start(context.Context, ExecutionCommand, io.Writer, io.Writer) (ExecutionProcess, error)
}

// ExecutionCommand names a process in an execution environment. Workdir must
// be an absolute path available in that environment.
type ExecutionCommand struct {
	Operation string
	Session   string
	Role      string
	Workdir   string
	Command   string
	Args      []string
}

// ExecutionProcess is a started command or resident service. Output is written
// to Start's writers until Wait returns. Stop is idempotent and waits for owned
// processes to terminate, or returns a cleanup error. Wait preserves a nonzero
// process exit as an exit code; transport/start/cancellation failures are errors.
type ExecutionProcess interface {
	Workdir() string
	Wait() (int, error)
	Stop() error
}

type environmentKey struct{}

// InEnvironment selects the named environment for commands, services, and
// sessions created with the returned context. A session keeps that binding
// through subsequent turns and forks. Without an execution backend, naming an
// environment is an error rather than permission to run on the caller's host.
func InEnvironment(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, environmentKey{}, name)
}

// RunOption configures execution of a run.
type RunOption func(*run)

// WithExecution gives Run ownership of backend. The workflow must select an
// environment with InEnvironment before creating remote sessions or processes.
func WithExecution(backend ExecutionBackend) RunOption {
	return func(r *run) { r.backend = backend }
}
