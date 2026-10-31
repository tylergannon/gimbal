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
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/secretcallback"
)

type task struct {
	process      *effects.Process
	envPath      string
	input        json.RawMessage
	turn         *effects.Turn
	command      *executor.Descriptor
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
	err     error
	harness string
	request effects.Session
	owner   effects.Owner
	adapter gimbal.HarnessAdapter
	native  string
	ready   chan struct{}
	cancel  context.CancelFunc
	closeMu sync.Mutex
}
type Daemon struct {
	executorRoot   string
	stateRoot      string
	restoreErr     error
	messageMu      sync.Mutex
	controlMu      sync.Mutex
	recoverTasks   []func()
	recoveryOnce   sync.Once
	messages       map[string]*disposition
	activeMessages map[string]bool
	Secrets        backend.SecretsBackend
	files          map[backend.Artifact]string
	Workdir        string
	mu             sync.Mutex
	closed         map[effects.Scope]time.Time
	sessions       map[string]*session
	tasks          map[string]*task
	// admitting owns service preflight until native admission finishes. Cleanup
	// cancels and joins it using the same task boundary, without holding mu in
	// an externally supplied dependency. It stores no infrastructure leases.
	admitting map[string]*task
	adapter   func(string) (gimbal.HarnessAdapter, error)
}

func New(executorRoot string) *Daemon {
	d := &Daemon{messages: map[string]*disposition{}, activeMessages: map[string]bool{}, executorRoot: executorRoot, files: map[backend.Artifact]string{}, closed: map[effects.Scope]time.Time{}, sessions: map[string]*session{}, tasks: map[string]*task{}}
	d.adapter = func(harness string) (gimbal.HarnessAdapter, error) {
		adapter, err := binding.Adapter(harness)
		if independent, ok := adapter.(*agentexecutor.Adapter); ok {
			independent.Root = executorRoot
		}
		return adapter, err
	}
	d.restoreErr = d.restore()

	return d
}

// Ready restores task owners and reports any state-loading failure before a
// native host advertises itself as ready to accept tasks.
func (d *Daemon) Ready() error {
	d.ensureRecovery()
	return d.restoreErr
}

func key(run, id string) string { return run + "\x00" + id }
func within(child, parent string) bool {
	return parent == "" || child == parent || strings.HasPrefix(child, parent+"/")
}
func (d *Daemon) admit(o effects.Owner) error {
	if d.restoreErr != nil {
		return d.restoreErr
	}
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
	d.ensureRecovery()
	if err := req.ExecutionIdentity.Validate("session"); err != nil {
		return "", err
	}
	d.mu.Lock()
	if err := d.admit(req.Owner); err != nil {
		d.mu.Unlock()
		return "", err
	}
	id := key(req.Run, req.Task)
	if existing := d.sessions[id]; existing != nil {
		d.mu.Unlock()
		if !sameJSON(existing.request, req) {
			return "", errors.New("session identity conflicts with accepted input")
		}
		<-existing.ready
		if existing.err != nil {
			return "", existing.err
		}
		if existing.native == "" {
			return "", errors.New("session creation failed")
		}
		return req.Task, nil
	}
	var parent *session
	if req.Parent != "" {
		parent = d.sessions[key(req.Run, req.Parent)]
		if parent == nil {
			d.mu.Unlock()
			return "", errors.New("parent session not found")
		}
	}
	ctx, cancel := context.WithTimeout(effects.WithExecutionIdentity(effects.WithOwner(context.Background(), req.Owner), req.ExecutionIdentity), 30*time.Second)
	harness := req.Harness
	if parent != nil {
		harness = parent.harness
	}
	s := &session{harness: harness, request: req, owner: req.Owner, ready: make(chan struct{}), cancel: cancel}
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
	if err == nil {
		err = d.writeState("sessions", id, savedSession{Request: req, Native: native, Harness: harness})
	}
	if err != nil && native == "" {
		delete(d.sessions, id)
	}
	s.err = err
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
	if found, err := d.existingTask(req.Owner, req); found || err != nil {
		return err
	}
	input, _ := json.Marshal(req)
	if err := req.ExecutionIdentity.Validate("turn"); err != nil {
		return err
	}
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
	if existing := d.tasks[id]; existing != nil {
		if string(existing.input) != string(input) {
			return errors.New("task identity conflicts with accepted inputs")
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	if !req.Deadline.IsZero() {
		cancel()
		ctx, cancel = context.WithDeadline(context.Background(), req.Deadline)
	}
	ctx = effects.WithExecutionIdentity(effects.WithOwner(ctx, req.Owner), req.ExecutionIdentity)
	t := &task{input: input, turn: &req, owner: req.Owner, cancel: cancel, done: make(chan struct{})}
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
	if err := d.saveTaskLocked(t); err != nil {
		delete(d.tasks, id)
		cancel()
		closeCallback()
		return err
	}
	go d.runTurn(ctx, s, req, t, emit, closeCallback)
	return nil
}
func (d *Daemon) poll(req effects.Poll) effects.Progress {
	d.ensureRecovery()
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
func (d *Daemon) steerNative(ctx context.Context, req effects.Target) (bool, error) {
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
	if err := req.ExecutionIdentity.Validate("close"); err != nil {
		return err
	}
	ctx = effects.WithExecutionIdentity(ctx, req.ExecutionIdentity)
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
	_ = os.Remove(d.statePath("sessions", key(req.Run, req.ID)))
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
	if err := d.writeState("closed", key(req.Run, req.Scope), struct {
		Scope  effects.Scope
		Closed time.Time
	}{req, d.closed[req]}); err != nil {
		d.mu.Unlock()
		return err
	}
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
			sessions = append(sessions, effects.Target{ExecutionIdentity: backend.NewExecutionIdentity("close"), Run: req.Run, ID: s.owner.Task})
		}
	}
	for artifact, path := range d.files {
		if artifact.Run == req.Run && within(artifact.Scope, req.Scope) {
			_ = os.Remove(path)
			delete(d.files, artifact)
			_ = os.Remove(d.statePath("files", key(artifact.Run, artifact.Task)+"\x00"+artifact.Name))
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
			t.mu.Lock()
			saveErr := d.saveTaskLocked(t)
			t.mu.Unlock()
			if saveErr != nil {
				errs = append(errs, saveErr)
			}
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

func (d *Daemon) runTurn(ctx context.Context, s *session, req effects.Turn, t *task, emit func(json.RawMessage), closeCallback func()) {
	defer closeCallback()
	replayed := 0
	retained := 0
	if effects.Reconstructing(ctx) {
		retained = len(t.events)
	}
	emitEvent := func(event gimbal.AgentEvent) error {
		replayed++
		if replayed <= retained {
			return nil
		}
		raw, e := json.Marshal(struct {
			gimbal.AgentEvent
			Native json.RawMessage
		}{event, event.NativeRef})
		if e == nil {
			t.mu.Lock()
			t.events = append(t.events, raw)
			if err := d.saveTaskLocked(t); err != nil {
				t.mu.Unlock()
				return err
			}
			t.mu.Unlock()
			if emit != nil {
				emit(raw)
			}
		}
		return nil
	}
	var result gimbal.TurnResult
	var err error
	if effects.Reconstructing(ctx) {
		root, e := executor.Root(d.executorRoot)
		dir := filepath.Join(root, req.ExecutionIdentity.ExecutorOperation)
		if e != nil {
			err = e
		} else if !executor.Exists(filepath.Join(dir, "request.json")) {
			err = &backend.DeliveryUncertain{RequestID: req.ExecutionIdentity.RequestID, Err: errors.New("accepted native turn evidence unavailable")}
		} else {
			descriptor := executor.Describe(dir, true)
			stopDone := make(chan struct{})
			controlDone := make(chan error, 1)
			go func() {
				select {
				case <-ctx.Done():
					stop, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_, e := executor.Call(stop, dir, executor.Control{Op: "cancel"})
					controlDone <- e
				case <-stopDone:
					controlDone <- nil
				}
			}()
			result, err = agentexecutor.Wait(context.Background(), descriptor, emitEvent)
			close(stopDone)
			if controlErr := <-controlDone; controlErr != nil {
				err = &backend.DeliveryUncertain{RequestID: req.ExecutionIdentity.RequestID, Err: errors.Join(err, controlErr)}
			}
		}
	} else {
		result, err = s.adapter.RunTurn(ctx, s.native, req.Prompt, req.Schema, emitEvent)
	}
	usage, _ := json.Marshal(result.Usage)
	raw, _ := json.Marshal(backend.TurnResult{Output: result.Output, Usage: usage})
	d.completeTask(t, raw, err)
}
