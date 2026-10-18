// Package taskdaemon bridges task requests to independent native executors. HTTP is only one caller
// of these handlers; provisioning and workflow observation remain elsewhere.
package taskdaemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/secretcallback"
)

type task struct {
	owner        effects.Owner
	service      bool
	started      time.Time
	admissionCtx context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	mu           sync.Mutex
	events       []json.RawMessage
	result       json.RawMessage
	err          error
	stop         func(context.Context) error
	released     bool
	captures     map[string]string
	completed    time.Time
	expired      bool
}
type session struct {
	owner   effects.Owner
	adapter gimbal.HarnessAdapter
	native  string
	ready   chan struct{}
	cancel  context.CancelFunc
	closeMu sync.Mutex
}
type Daemon struct {
	Secrets  backend.SecretsBackend
	files    map[backend.Artifact]string
	Workdir  string
	mu       sync.Mutex
	closed   map[effects.Scope]time.Time
	sessions map[string]*session
	tasks    map[string]*task
	// admitting owns service preflight until native admission finishes. Cleanup
	// cancels and joins it using the same task boundary, without holding mu in
	// an externally supplied dependency. It stores no infrastructure leases.
	admitting map[string]*task
	adapter   func(string) (gimbal.HarnessAdapter, error)
}

func New() *Daemon {
	return &Daemon{files: map[backend.Artifact]string{}, closed: map[effects.Scope]time.Time{}, sessions: map[string]*session{}, tasks: map[string]*task{}, adapter: binding.Adapter}
}
func key(run, id string) string { return run + "\x00" + id }
func within(child, parent string) bool {
	return parent == "" || child == parent || strings.HasPrefix(child, parent+"/")
}
func (d *Daemon) admit(o effects.Owner) error {
	d.expireLocked(time.Now())
	if o.Run == "" || o.Task == "" {
		return errors.New("run and task identities are required")
	}
	for s := range d.closed {
		if s.Run == o.Run && within(o.Scope, s.Scope) {
			return fmt.Errorf("scope %q is closed", o.Scope)
		}
	}
	return nil
}
func (d *Daemon) create(_ context.Context, req effects.Session) (string, error) {
	d.mu.Lock()
	if err := d.admit(req.Owner); err != nil {
		d.mu.Unlock()
		return "", err
	}
	id := key(req.Run, req.Task)
	if d.sessions[id] != nil {
		d.mu.Unlock()
		return "", errors.New("session already exists")
	}
	var parent *session
	if req.Parent != "" {
		parent = d.sessions[key(req.Run, req.Parent)]
		if parent == nil {
			d.mu.Unlock()
			return "", errors.New("parent session not found")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	s := &session{owner: req.Owner, ready: make(chan struct{}), cancel: cancel}
	d.sessions[id] = s
	d.mu.Unlock()
	defer cancel()
	var a gimbal.HarnessAdapter
	var native string
	var err error
	if parent != nil {
		select {
		case <-parent.ready:
			a = parent.adapter
			if a == nil {
				err = errors.New("parent creation failed")
			} else {
				native, err = a.Fork(ctx, parent.native)
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
	} else {
		a, err = d.adapter(req.Harness)
		if err == nil {
			native, err = a.CreateSession(ctx, req.Model, req.Effort, d.workdir(req.Workdir))
		}
	}
	d.mu.Lock()
	s.adapter, s.native = a, native
	if err != nil && native == "" {
		delete(d.sessions, id)
	}
	admissionErr := d.admit(req.Owner)
	close(s.ready)
	d.mu.Unlock()
	if err != nil {
		return "", err
	}
	if admissionErr != nil {
		return "", admissionErr
	}
	return req.Task, nil
}
func (d *Daemon) turn(req effects.Turn) error { return d.turnEvents(req, nil) }
func (d *Daemon) turnEvents(req effects.Turn, emit func(json.RawMessage)) error {
	_, finish, err := d.acquire(context.Background(), req.Owner, req.Requires)
	defer finish()
	if err != nil {
		return err
	}
	for _, file := range req.Files {
		if err := d.writeBytes(backend.Artifact{Owner: req.Owner, Name: file.Name}, bytes.NewReader(file.Bytes)); err != nil {
			return err
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.admit(req.Owner); err != nil {
		return err
	}
	s := d.sessions[key(req.Run, req.Session)]
	if s == nil {
		return errors.New("session not found")
	}
	select {
	case <-s.ready:
		if s.native == "" {
			return errors.New("session creation failed")
		}
	default:
		return errors.New("session is being created")
	}
	for _, file := range req.Files {
		path := d.files[backend.Artifact{Owner: req.Owner, Name: file.Name}]
		raw := append([]byte(nil), file.Bytes...)
		for _, reference := range req.Files {
			if reference.Reference != "" {
				native := d.files[backend.Artifact{Owner: req.Owner, Name: reference.Name}]
				raw = bytes.ReplaceAll(raw, []byte("Complete value: "+reference.Reference), []byte("Complete value: "+native))
			}
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			return err
		}
	}
	for _, file := range req.Files {
		path := d.files[backend.Artifact{Owner: req.Owner, Name: file.Name}]
		if file.Reference != "" {
			req.Prompt = strings.ReplaceAll(req.Prompt, "Complete value: "+file.Reference, "Complete value: "+path)
		} else {
			req.Prompt += "\nRead task file " + file.Name + " at " + path + "."
		}
	}
	id := key(req.Run, req.Task)
	if pending := d.admitting[id]; pending != nil && pending.admissionCtx.Err() != nil {
		return pending.admissionCtx.Err()
	}
	if d.tasks[id] != nil {
		return errors.New("turn already started; read its result")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &task{owner: req.Owner, cancel: cancel, done: make(chan struct{})}
	d.tasks[id] = t
	closeCallback := func() {}
	if len(req.Requires) > 0 {
		address, close, err := secretcallback.Open(ctx, d.Secrets, req.Owner, req.Requires)
		if err != nil {
			delete(d.tasks, id)
			cancel()
			return err
		}
		closeCallback = close
		req.Prompt += "\nRequired stores are available for this operation as JSON via GET " + address + ". Call this endpoint when you need the values."
	}
	go func() {
		defer closeCallback()
		result, err := s.adapter.RunTurn(ctx, s.native, req.Prompt, req.Schema, func(event gimbal.AgentEvent) error {
			raw, e := json.Marshal(struct {
				gimbal.AgentEvent
				Native json.RawMessage
			}{event, event.NativeRef})
			if e == nil {
				t.mu.Lock()
				t.events = append(t.events, raw)
				t.mu.Unlock()
				if emit != nil {
					emit(raw)
				}
			}
			return nil
		})
		t.mu.Lock()
		usage, _ := json.Marshal(result.Usage)
		t.result, _ = json.Marshal(backend.TurnResult{Output: result.Output, Usage: usage})
		t.err = err
		t.mu.Unlock()
		t.mu.Lock()
		t.completed = time.Now()
		t.mu.Unlock()
		close(t.done)
	}()
	return nil
}
func (d *Daemon) poll(req effects.Poll) effects.Progress {
	d.mu.Lock()
	d.expireLocked(time.Now())
	t := d.tasks[key(req.Run, req.ID)]
	d.mu.Unlock()
	if t == nil {
		return effects.Progress{Done: true, Error: "task not found"}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.expired {
		return effects.Progress{Done: true, Error: "task payload expired"}
	}
	if req.After < 0 || req.After > len(t.events) {
		return effects.Progress{Done: true, Error: "invalid event cursor"}
	}
	p := effects.Progress{Events: append([]json.RawMessage(nil), t.events[req.After:]...)}
	select {
	case <-t.done:
		p.Done = true
		p.Result = t.result
		if t.err != nil {
			p.Error = t.err.Error()
		}
	default:
	}
	return p
}
func (d *Daemon) interrupt(req effects.Target) error {
	d.mu.Lock()
	t := d.tasks[key(req.Run, req.ID)]
	d.mu.Unlock()
	if t == nil {
		return errors.New("task not found")
	}
	t.cancel()
	return nil
}
func (d *Daemon) steer(ctx context.Context, req effects.Target) (bool, error) {
	d.mu.Lock()
	s := d.sessions[key(req.Run, req.ID)]
	d.mu.Unlock()
	if s == nil {
		return false, errors.New("session not found")
	}
	select {
	case <-s.ready:
		if s.native == "" {
			return false, errors.New("session creation failed")
		}
		return s.adapter.Steer(ctx, s.native, req.Message)
	case <-ctx.Done():
		return false, ctx.Err()
	}
}
func (d *Daemon) closeSession(ctx context.Context, req effects.Target) error {
	d.mu.Lock()
	s := d.sessions[key(req.Run, req.ID)]
	d.mu.Unlock()
	if s == nil {
		return nil
	}
	s.cancel()
	select {
	case <-s.ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.native == "" {
		return nil
	}
	if err := s.adapter.Close(ctx, s.native); err != nil {
		return err
	}
	d.mu.Lock()
	delete(d.sessions, key(req.Run, req.ID))
	d.mu.Unlock()
	return nil
}
func (d *Daemon) cleanup(ctx context.Context, req effects.Scope) error {
	if err := d.cleanupNative(ctx, req); err != nil {
		return err
	}
	if d.Secrets != nil {
		return d.Secrets.Release(ctx, req)
	}
	return nil
}

// Messaging callers release the shared store after all assigned daemons reply.
func (d *Daemon) cleanupNative(ctx context.Context, req effects.Scope) error {
	d.mu.Lock()
	d.closed[req] = time.Now()
	var tasks []*task
	for _, pending := range d.admitting {
		if pending.owner.Run == req.Run && within(pending.owner.Scope, req.Scope) {
			tasks = append(tasks, pending)
		}
	}
	var sessions []effects.Target
	for _, t := range d.tasks {
		if t.owner.Run == req.Run && within(t.owner.Scope, req.Scope) {
			tasks = append(tasks, t)
		}
	}
	for _, s := range d.sessions {
		if s.owner.Run == req.Run && within(s.owner.Scope, req.Scope) {
			sessions = append(sessions, effects.Target{Run: req.Run, ID: s.owner.Task})
		}
	}
	for artifact, path := range d.files {
		if artifact.Run == req.Run && within(artifact.Scope, req.Scope) {
			_ = os.Remove(path)
			delete(d.files, artifact)
		}
	}
	d.mu.Unlock()
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].service != tasks[j].service {
			return !tasks[i].service
		}
		return tasks[i].started.After(tasks[j].started)
	})
	for _, t := range tasks {
		if !t.service {
			t.cancel()
		}
	}
	var errs []error
	for _, s := range sessions {
		if err := d.closeSession(ctx, s); err != nil {
			errs = append(errs, err)
		}
	}
	for _, t := range tasks {
		if t.service && len(errs) > 0 {
			break
		}
		t.cancel()
		if t.stop != nil {
			if err := t.stop(ctx); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		select {
		case <-t.done:
			d.mu.Lock()
			t.released = true
			d.retireLocked(t)
			d.mu.Unlock()
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		}
	}
	return errors.Join(errs...)
}
func (d *Daemon) state(req effects.Scope) effects.State {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, s := range d.sessions {
		if s.owner.Run == req.Run && within(s.owner.Scope, req.Scope) {
			n++
		}
	}
	for _, t := range d.tasks {
		if !t.released && t.owner.Run == req.Run && within(t.owner.Scope, req.Scope) {
			n++
		}
	}
	return effects.State{Pending: n}
}
