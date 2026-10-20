package taskdaemon

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/commandexecutor"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
)

type savedSession struct {
	Harness string
	Request effects.Session
	Native  string
}
type savedTask struct {
	Input                            json.RawMessage
	Released                         bool
	Owner                            effects.Owner
	Turn                             *effects.Turn
	Command                          *executor.Descriptor
	Process                          *effects.Process
	EnvPath                          string
	Service                          bool
	Events                           []json.RawMessage
	Result                           json.RawMessage
	Error                            string
	Invalid, Interrupted, Unresolved bool
	Completed                        time.Time
	Captures                         map[string]string
}
type savedFile struct {
	Artifact backend.Artifact
	Path     string
}
type disposition struct {
	Request backend.MessageRequest
	Result  *backend.MessageResult
	EventID string
}

func (d *Daemon) statePath(kind, id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(d.stateRoot, kind, fmt.Sprintf("%x.json", sum))
}
func (d *Daemon) writeState(kind, id string, value any) error {
	if d.restoreErr != nil {
		return d.restoreErr
	}
	path := d.statePath(kind, id)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return executor.Write(path, value)
}
func (d *Daemon) loadState(kind string, load func(string) error) error {
	paths, err := filepath.Glob(filepath.Join(d.stateRoot, kind, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := load(path); err != nil {
			return fmt.Errorf("restore %s: %w", kind, err)
		}
	}
	return nil
}
func (d *Daemon) restore() error {
	root, err := executor.Root(d.executorRoot)
	if err != nil {
		return err
	}
	d.stateRoot = filepath.Join(root, "taskdaemon")
	if err = d.loadState("closed", func(path string) error {
		var saved struct {
			Scope  effects.Scope
			Closed time.Time
		}
		if err = executor.Read(path, &saved); err != nil {
			return err
		}
		d.closed[saved.Scope] = saved.Closed
		return nil
	}); err != nil {
		return err
	}
	if err = d.loadState("sessions", func(path string) error {
		var saved savedSession
		if err = executor.Read(path, &saved); err != nil {
			return err
		}
		adapter, err := d.adapter(saved.Harness)
		if err != nil {
			return err
		}
		_, cancel := context.WithCancel(context.Background())
		s := &session{harness: saved.Harness, owner: saved.Request.Owner, adapter: adapter, native: saved.Native, request: saved.Request, ready: make(chan struct{}), cancel: cancel}
		close(s.ready)
		d.sessions[key(s.owner.Run, s.owner.Task)] = s
		return nil
	}); err != nil {
		return err
	}
	if err = d.loadState("files", func(path string) error {
		var saved savedFile
		if err = executor.Read(path, &saved); err != nil {
			return err
		}
		d.files[saved.Artifact] = saved.Path
		return nil
	}); err != nil {
		return err
	}
	if err = d.loadState("messages", func(path string) error {
		var saved disposition
		if err = executor.Read(path, &saved); err != nil {
			return err
		}
		d.messages[saved.Request.ID] = &saved
		return nil
	}); err != nil {
		return err
	}
	return d.loadState("tasks", func(path string) error {
		var saved savedTask
		if err = executor.Read(path, &saved); err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(context.Background())
		t := &task{input: saved.Input, released: saved.Released, owner: saved.Owner, service: saved.Service, cancel: cancel, done: make(chan struct{}), events: saved.Events, result: saved.Result, captures: saved.Captures, completed: saved.Completed, turn: saved.Turn, command: saved.Command, process: saved.Process, envPath: saved.EnvPath}
		if saved.Error != "" {
			t.err = errors.New(saved.Error)
		}
		if saved.Invalid {
			t.err = fmt.Errorf("%w: %v", gimbal.ErrInvalidOutput, t.err)
		}
		if saved.Interrupted {
			t.err = context.Canceled
		}
		if saved.Unresolved {
			t.err = &backend.DeliveryUncertain{Err: t.err}
		}
		d.tasks[key(t.owner.Run, t.owner.Task)] = t
		if !saved.Completed.IsZero() {
			close(t.done)
			return nil
		}
		if saved.Service {
			t.err = &backend.DeliveryUncertain{Err: errors.New("service restoration is unavailable")}
			t.completed = time.Now()
			close(t.done)
			return nil
		}
		if saved.Turn != nil {
			s := d.sessions[key(saved.Owner.Run, saved.Turn.Session)]
			if s == nil {
				return errors.New("accepted turn session mapping missing")
			}
			ctx = effects.WithReconstruction(effects.WithExecutionIdentity(effects.WithOwner(ctx, saved.Owner), saved.Turn.ExecutionIdentity), true)
			if !saved.Turn.Deadline.IsZero() {
				cancel()
				ctx, cancel = context.WithDeadline(ctx, saved.Turn.Deadline)
				t.cancel = cancel
			}
			d.recoverTasks = append(d.recoverTasks, func() { go d.runTurn(ctx, s, *saved.Turn, t, nil, func() {}) })
		} else if saved.Command != nil {
			descriptor := *saved.Command
			t.stop = func(ctx context.Context) error { return commandexecutor.Cancel(ctx, descriptor) }
			d.recoverTasks = append(d.recoverTasks, func() {
				go func() {
					if !executor.Exists(filepath.Join(filepath.Dir(descriptor.Result), "request.json")) {
						d.completeTask(t, nil, &backend.DeliveryUncertain{Err: errors.New("accepted command evidence unavailable")})
						return
					}
					result, err := commandexecutor.Wait(context.Background(), descriptor)
					raw, _ := json.Marshal(effects.ProcessResult{ExitCode: result.ExitCode, Interrupted: result.Cancelled || result.TimedOut, Error: result.Error})
					if saved.Process != nil && saved.EnvPath != "" && result.ExitCode == 0 && !result.Cancelled && !result.TimedOut && result.Error == "" && err == nil {
						err = d.publishProducer(ctx, *saved.Process, saved.EnvPath)
					}
					d.completeTask(t, raw, err)
				}()
			})
			d.recoverTasks = append(d.recoverTasks, func() {
				go func() {
					select {
					case <-ctx.Done():
						stop, c := context.WithTimeout(context.Background(), 12*time.Second)
						defer c()
						_ = t.stop(stop)
					case <-t.done:
					}
				}()
			})
		} else {
			return errors.New("accepted task has no executor mapping")
		}
		return nil
	})
}
func (d *Daemon) saveTaskLocked(t *task) error {
	saved := savedTask{Input: t.input, Released: t.released, Owner: t.owner, Turn: t.turn, Command: t.command, Process: t.process, EnvPath: t.envPath, Service: t.service, Events: t.events, Result: t.result, Captures: t.captures, Completed: t.completed}
	if t.err != nil {
		saved.Error = t.err.Error()
		saved.Invalid = errors.Is(t.err, gimbal.ErrInvalidOutput)
		saved.Interrupted = errors.Is(t.err, context.Canceled)
		var uncertain *backend.DeliveryUncertain
		saved.Unresolved = errors.As(t.err, &uncertain)
	}
	return d.writeState("tasks", key(t.owner.Run, t.owner.Task), saved)
}
func (d *Daemon) completeTask(t *task, raw json.RawMessage, err error) {
	t.mu.Lock()
	t.result, t.err, t.completed = raw, err, time.Now()
	if saveErr := d.saveTaskLocked(t); saveErr != nil {
		t.err = &backend.DeliveryUncertain{Err: errors.Join(err, saveErr)}
	}
	t.mu.Unlock()
	close(t.done)
}

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func (d *Daemon) steer(ctx context.Context, req effects.Target) (bool, error) {
	d.ensureRecovery()
	if req.InputID == "" {
		return d.steerNative(ctx, req)
	}
	d.controlMu.Lock()
	defer d.controlMu.Unlock()
	path := d.statePath("controls", req.InputID)
	lease, err := executor.Lock(path + ".owner")
	if err != nil {
		return false, err
	}
	if lease == nil {
		return false, &backend.DeliveryUncertain{RequestID: req.InputID, Err: errors.New("steering outcome is still owned by another handler")}
	}
	defer lease.Close()
	var saved disposition
	err = executor.Read(path, &saved)
	request := backend.MessageRequest{ID: req.InputID, Kind: "steer", Target: &req}
	if err == nil {
		if !sameJSON(saved.Request, request) {
			return false, errors.New("input identity conflicts with accepted steering")
		}
		if saved.Result == nil {
			return false, &backend.DeliveryUncertain{RequestID: req.InputID, Err: errors.New("steering accepted; native delivery outcome unavailable")}
		}
		if saved.Result.Error != "" {
			return saved.Result.Value, &backend.DeliveryUncertain{RequestID: req.InputID, Err: errors.New(saved.Result.Error)}
		}
		return saved.Result.Value, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	saved.Request = request
	if err = d.writeState("controls", req.InputID, saved); err != nil {
		return false, err
	}
	value, err := d.steerNative(ctx, req)
	saved.Result = &backend.MessageResult{Value: value}
	if err != nil {
		saved.Result.Error = err.Error()
		saved.Result.Unresolved = true
	}
	if saveErr := d.writeState("controls", req.InputID, saved); saveErr != nil {
		return false, &backend.DeliveryUncertain{RequestID: req.InputID, Err: saveErr}
	}
	if err != nil {
		return value, &backend.DeliveryUncertain{RequestID: req.InputID, Err: err}
	}
	return value, nil
}

func (d *Daemon) existingTask(owner effects.Owner, input any) (bool, error) {
	d.ensureRecovery()
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.tasks[key(owner.Run, owner.Task)]
	if t == nil {
		return false, nil
	}
	raw, _ := json.Marshal(input)
	if string(t.input) != string(raw) {
		return true, errors.New("task identity conflicts with accepted inputs")
	}
	return true, nil
}

func (d *Daemon) ensureRecovery() {
	d.recoveryOnce.Do(func() {
		if d.restoreErr == nil {
			for _, start := range d.recoverTasks {
				start()
			}
		}
		d.recoverTasks = nil
	})
}
