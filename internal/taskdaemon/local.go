package taskdaemon

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/tylergannon/gimbal/backend"
)

// Local connects the same native owner directly, without a per-task process or HTTP listener.
type Local struct{ *Daemon }

func (d *Daemon) Connection() backend.Connection {
	l := Local{d}
	return backend.Connection{Execution: l, Events: l}
}
func (l Local) Session(c context.Context, v backend.Session) (backend.Reply, error) {
	id, e := l.create(c, v)
	return backend.Reply{ID: id}, e
}
func (l Local) Turn(c context.Context, v backend.Turn) error       { return l.turn(v) }
func (l Local) Process(c context.Context, v backend.Process) error { return l.process(v) }
func (l Local) Result(c context.Context, v backend.Target) (backend.Result, error) {
	return l.result(v), nil
}
func (l Local) Interrupt(c context.Context, v backend.Target) error     { return l.interrupt(v) }
func (l Local) Steer(c context.Context, v backend.Target) (bool, error) { return l.steer(c, v) }
func (l Local) Close(c context.Context, v backend.Target) error         { return l.closeSession(c, v) }
func (l Local) Cleanup(c context.Context, v backend.Scope) error        { return l.cleanup(c, v) }
func (l Local) Pending(c context.Context, v backend.Scope) (backend.State, error) {
	return l.state(v), nil
}
func (l Local) Acknowledge(c context.Context, v backend.Target) error { return l.acknowledge(v) }
func (l Local) ReadEvents(c context.Context, v backend.Poll) (backend.Events, error) {
	p := l.poll(v)
	return backend.Events{Events: p.Events, Cursor: v.After + len(p.Events), Gap: p.Error != "", Error: p.Error}, nil
}
func (l Local) ReadBytes(c context.Context, v backend.Artifact) (io.ReadCloser, error) {
	l.mu.Lock()
	t := l.tasks[key(v.Run, v.Task)]
	l.mu.Unlock()
	if t == nil {
		return nil, fmt.Errorf("task not found")
	}
	t.mu.Lock()
	path := t.captures[v.Name]
	t.mu.Unlock()
	return os.Open(path)
}
func (l Local) WriteBytes(c context.Context, v backend.Artifact, r io.Reader) error {
	return l.writeBytes(v, r)
}
func (l Local) Service(c context.Context, v backend.Service) (backend.ServiceAddress, error) {
	return l.service(v)
}
func (l Local) StartWorkflow(c context.Context, v backend.Workflow) (backend.WorkflowStatus, error) {
	return backend.WorkflowStatus{}, fmt.Errorf("workflow admission is provided by the host")
}
func (l Local) WorkflowStatus(c context.Context, v backend.Target) (backend.WorkflowStatus, error) {
	return backend.WorkflowStatus{}, fmt.Errorf("workflow admission is provided by the host")
}
func (l Local) ControlWorkflow(c context.Context, v backend.Control) (bool, error) {
	return false, fmt.Errorf("workflow controls are provided by the host")
}
