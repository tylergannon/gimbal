package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/pi"
	"go.temporal.io/sdk/activity"
)

// Activities owns worker-local resources, never workflow scheduling. Every
// frame is explicitly opened/closed by generated lexical code. The root
// cancellation watcher drains resources if the controller becomes unavailable;
// it does not execute workflow control flow.
type Activities struct {
	models    map[gimbal.WorkflowRole]gimbal.ModelBinding
	project   *host.Project
	controls  host.CompiledControls
	workdir   string
	store     compiledscope.Store
	mu        sync.Mutex
	scopes    map[string]*scopeFrame
	completed map[string]*scopeFrame
	draining  bool
	drainErr  error
	drainOnce sync.Once
	drainDone chan struct{}
}

type scopeFrame struct {
	sessions map[string]*gimbal.Session
	parent   string
	ctx      context.Context
	cancel   context.CancelFunc
	finish   func(error) error
	busy     sync.WaitGroup
	closing  bool
	done     chan struct{}
	outcome  error
}

type ScopeInput struct{ ID, Parent, Name string }

func (a *Activities) Initialize(_ context.Context, data Data) (compiledscope.Snapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scopes != nil {
		return "", errors.New("specimen already initialized")
	}
	name := data.Name
	if name == "" {
		name = "continuity"
	}
	models := a.models
	if models == nil {
		models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: claude.New(), Model: "claude-haiku-4-5"}, coach: {Adapter: pi.New(), Model: "diffusion/deepseek-4.1-flash"}}
	}
	localDir := a.store.LocalDir
	if localDir == "" {
		localDir = filepath.Join(a.store.Root, "materialized")
	}
	root, finish, err := a.project.OpenCompiledRun(a.project.Context(), name, models, data.Context, localDir, a.controls)
	if err != nil {
		return "", err
	}
	root, cancel := context.WithCancel(root)
	a.scopes = map[string]*scopeFrame{"": {ctx: root, cancel: cancel, finish: finish, done: make(chan struct{})}}
	a.completed = make(map[string]*scopeFrame)
	a.drainDone = make(chan struct{})

	go func() {
		<-root.Done()
		a.mu.Lock()
		frame := a.scopes[""]
		closing := frame == nil || frame.closing
		a.mu.Unlock()
		if !closing {
			a.drain(context.Cause(root))
		}
	}()
	return data.Context, nil
}

func (a *Activities) EnterScope(_ context.Context, in ScopeInput) error {
	return a.enterScope(in, compiledscope.OpenScope)
}
func (a *Activities) enterScope(in ScopeInput, open func(context.Context, string) (context.Context, func(error) error, error)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	parent := a.scopes[in.Parent]
	if a.draining || parent == nil || parent.closing {
		return fmt.Errorf("parent scope %q unavailable", in.Parent)
	}
	if err := parent.ctx.Err(); err != nil {
		return err
	}
	if _, exists := a.scopes[in.ID]; exists || in.ID == "" {
		return fmt.Errorf("scope %q already exists", in.ID)
	}
	ctx, cancel := context.WithCancel(parent.ctx)
	child, finish, err := open(ctx, in.Name)
	if err != nil {
		cancel()
		return err
	}
	a.scopes[in.ID] = &scopeFrame{parent: in.Parent, ctx: child, cancel: cancel, finish: finish, done: make(chan struct{})}
	return nil
}

// operation leases a scope only for this activity. ExitScope prevents new
// leases, cancels existing ones and joins them before resource cleanup.
func (a *Activities) operation(ctx context.Context, id string) (context.Context, func(), error) {
	a.mu.Lock()
	frame := a.scopes[id]
	if a.draining || frame == nil || frame.closing {
		a.mu.Unlock()
		return nil, nil, fmt.Errorf("scope %q unavailable", id)
	}
	if err := frame.ctx.Err(); err != nil {
		a.mu.Unlock()
		return nil, nil, err
	}
	frame.busy.Add(1)
	a.mu.Unlock()
	scoped, cancel := context.WithCancel(frame.ctx)
	stop := context.AfterFunc(ctx, cancel)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-scoped.Done():
				return
			case <-ticker.C:
				activity.RecordHeartbeat(ctx)
			}
		}
	}()
	return scoped, func() { stop(); cancel(); <-done; frame.busy.Done() }, nil
}

func (a *Activities) ExitScope(_ context.Context, id, reason string) error {
	var cause error
	if reason != "" {
		cause = errors.New(reason)
	}
	return a.closeScope(id, cause)
}

// The worker owns each finish invocation. Controller cleanup and a local
// cancellation drain join the same close and receive its recorded outcome.
func (a *Activities) closeScope(id string, cause error) error {
	a.mu.Lock()
	frame := a.scopes[id]
	if frame == nil {
		completed := a.completed[id]
		a.mu.Unlock()
		if completed != nil {
			<-completed.done
			return completed.outcome
		}
		return nil
	}
	if frame.closing {
		a.mu.Unlock()
		<-frame.done
		return frame.outcome
	}
	for key, child := range a.scopes {
		if key != "" && child.parent == id {
			a.mu.Unlock()
			return fmt.Errorf("scope %q still owns child %q", id, key)
		}
	}
	frame.closing = true
	bodyErr := cause
	if id == "" {
		bodyErr = errors.Join(cause, a.drainErr)
	}
	a.mu.Unlock()
	frame.cancel()
	frame.busy.Wait()
	err := frame.finish(bodyErr)
	a.mu.Lock()
	frame.outcome = withoutCause(err, cause)
	delete(a.scopes, id)
	a.completed[id] = frame
	close(frame.done)
	a.mu.Unlock()
	return frame.outcome
}

// Root cancellation must retire local resources even when the controller is
// unavailable. Stop admissions before taking the leaf-to-root close order.
func (a *Activities) drain(cause error) {
	a.drainOnce.Do(func() {
		a.mu.Lock()
		a.draining = true
		ids := make([]string, 0, len(a.scopes))
		for id, frame := range a.scopes {
			ids = append(ids, id)
			frame.cancel()
		}
		a.mu.Unlock()
		sort.Slice(ids, func(i, j int) bool {
			return strings.Count(ids[i], "/") > strings.Count(ids[j], "/") ||
				strings.Count(ids[i], "/") == strings.Count(ids[j], "/") && len(ids[i]) > len(ids[j])
		})
		for _, id := range ids {
			err := a.closeScope(id, cause)
			if id != "" && err != nil {
				a.mu.Lock()
				a.drainErr = errors.Join(a.drainErr, err)
				a.mu.Unlock()
			}
		}
		close(a.drainDone)
	})
	<-a.drainDone
}

func withoutCause(err, cause error) error {
	if err == nil || err == cause {
		return nil
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		var remaining []error
		for _, child := range many.Unwrap() {
			remaining = append(remaining, withoutCause(child, cause))
		}
		return errors.Join(remaining...)
	}
	return err
}

// Each generated context write is an activity. Values are published here,
// never embedded as multi-megabyte Temporal activity results.
func contextEntry(key string, value any) compiledscope.Entry {
	entry, err := compiledscope.Encode(key, value)
	if err != nil {
		panic(err)
	}
	return entry
}
func (a *Activities) write(ctx context.Context, id string, base compiledscope.Snapshot, entries ...compiledscope.Entry) (compiledscope.Snapshot, error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return "", err
	}
	defer release()
	return compiledscope.WriteContext(scoped, base, entries...)
}
func (a *Activities) Finish(ctx context.Context, reason string) error {
	a.mu.Lock()
	frame, draining := a.scopes[""], a.draining
	cancelled := frame != nil && !frame.closing && frame.ctx.Err() != nil
	a.mu.Unlock()
	if cancelled {
		a.drain(context.Cause(frame.ctx))
	} else if draining {
		<-a.drainDone
	}
	return a.ExitScope(ctx, "", reason)
}

func (a *Activities) FinishCancelled(ctx context.Context, reason string) error {
	a.mu.Lock()
	frame := a.scopes[""]
	a.mu.Unlock()
	if frame != nil {
		// The runtime retains the first effective cause, including a console request.
		if err := compiledscope.CancelRun(frame.ctx); err != nil && frame.ctx.Err() == nil {
			return err
		}
		<-frame.ctx.Done()
		a.drain(context.Cause(frame.ctx))
	}
	return a.Finish(ctx, reason)
}
