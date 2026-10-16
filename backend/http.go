package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
)

// HTTP connects to a daemon without coupling execution to observation delivery.
type HTTP struct {
	Endpoint string
	Client   *http.Client
}

func (h HTTP) call(ctx context.Context, path string, in, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", h.Endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c := h.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("backend %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
func (h HTTP) reply(ctx context.Context, op string, in any) (Reply, error) {
	var r Reply
	err := h.call(ctx, "/tasks/"+op, in, &r)
	if err == nil && r.Error != "" {
		err = fmt.Errorf("%s", r.Error)
	}
	return r, err
}
func (h HTTP) Session(c context.Context, v Session) (Reply, error) { return h.reply(c, "session", v) }
func (h HTTP) Turn(c context.Context, v Turn) error                { _, e := h.reply(c, "turn", v); return e }
func (h HTTP) Process(c context.Context, v Process) error          { _, e := h.reply(c, "process", v); return e }
func (h HTTP) Result(c context.Context, v Target) (Result, error) {
	var r Result
	e := h.call(c, "/tasks/result", v, &r)
	return r, e
}
func (h HTTP) Interrupt(c context.Context, v Target) error {
	_, e := h.reply(c, "interrupt", v)
	return e
}
func (h HTTP) Steer(c context.Context, v Target) (bool, error) {
	r, e := h.reply(c, "steer", v)
	return r.Value, e
}
func (h HTTP) Close(c context.Context, v Target) error  { _, e := h.reply(c, "close", v); return e }
func (h HTTP) Cleanup(c context.Context, v Scope) error { _, e := h.reply(c, "cleanup", v); return e }
func (h HTTP) Pending(c context.Context, v Scope) (State, error) {
	var r State
	e := h.call(c, "/tasks/state", v, &r)
	return r, e
}
func (h HTTP) Acknowledge(c context.Context, v Target) error { _, e := h.reply(c, "ack", v); return e }
func (h HTTP) ReadEvents(c context.Context, v Poll) (Events, error) {
	var r Events
	e := h.call(c, "/tasks/events", v, &r)
	return r, e
}
func (h HTTP) StartWorkflow(c context.Context, v Workflow) (WorkflowStatus, error) {
	var r WorkflowStatus
	e := h.call(c, "/workflows/start", v, &r)
	return r, e
}
func (h HTTP) WorkflowStatus(c context.Context, v Target) (WorkflowStatus, error) {
	var r WorkflowStatus
	e := h.call(c, "/workflows/status", v, &r)
	return r, e
}
func (h HTTP) ControlWorkflow(c context.Context, v Control) (bool, error) {
	var r Reply
	e := h.call(c, "/workflows/control", v, &r)
	if e == nil && r.Error != "" {
		e = fmt.Errorf("%s", r.Error)
	}
	return r.Value, e
}
func (h HTTP) ReadObservation(c context.Context, v Target, cursor string) (Observation, error) {
	var r Observation
	e := h.call(c, "/workflows/observation", struct {
		Target
		Cursor string
	}{v, cursor}, &r)
	return r, e
}

func (h HTTP) ReadBytes(c context.Context, v Artifact) (io.ReadCloser, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	path := "/tasks/bytes"
	if v.Task == "" {
		path = "/workflows/bytes"
	}
	req, e := http.NewRequestWithContext(c, "POST", h.Endpoint+path, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	r, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	if r.StatusCode == http.StatusNotFound {
		_ = r.Body.Close()
		return nil, fmt.Errorf("artifact: HTTP %d: %w", r.StatusCode, fs.ErrNotExist)
	}
	if r.StatusCode != 200 {
		_ = r.Body.Close()
		return nil, fmt.Errorf("artifact: HTTP %d", r.StatusCode)
	}
	return r.Body, nil
}
func (h HTTP) WriteBytes(c context.Context, v Artifact, body io.Reader) error {
	raw, e := io.ReadAll(body)
	if e != nil {
		return e
	}
	_, e = h.reply(c, "write-bytes", struct {
		Artifact
		Bytes []byte
	}{v, raw})
	return e
}
