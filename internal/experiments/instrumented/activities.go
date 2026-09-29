package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// frame is explicitly opened/closed by generated lexical code. No callback or
// goroutine waits between activities to keep a scope alive.
type Activities struct {
	models  map[gimbal.WorkflowRole]gimbal.ModelBinding
	project *host.Project
	workdir string
	store   compiledscope.Store
	mu      sync.Mutex
	scopes  map[string]*scopeFrame
}

type scopeFrame struct {
	sessions map[string]*gimbal.Session
	parent   string
	ctx      context.Context
	cancel   context.CancelFunc
	finish   func(error) error
	busy     sync.WaitGroup
	closing  bool
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
	root, finish, err := a.project.OpenCompiledRun(a.project.Context(), name, models)
	if err != nil {
		return "", err
	}
	root, cancel := context.WithCancel(root)
	a.scopes = map[string]*scopeFrame{"": {ctx: root, cancel: cancel, finish: finish}}
	entries, err := a.store.Load(data.Context)
	if err != nil {
		return "", err
	}
	return compiledscope.WriteContext(root, a.store, "", entries...)
}

func (a *Activities) EnterScope(_ context.Context, in ScopeInput) error {
	return a.enterScope(in, compiledscope.OpenScope)
}
func (a *Activities) enterScope(in ScopeInput, open func(context.Context, string) (context.Context, func(error) error, error)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	parent := a.scopes[in.Parent]
	if parent == nil || parent.closing {
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
	a.scopes[in.ID] = &scopeFrame{parent: in.Parent, ctx: child, cancel: cancel, finish: finish}
	return nil
}

// operation leases a scope only for this activity. ExitScope prevents new
// leases, cancels existing ones and joins them before resource cleanup.
func (a *Activities) operation(ctx context.Context, id string) (context.Context, func(), error) {
	a.mu.Lock()
	frame := a.scopes[id]
	if frame == nil || frame.closing {
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
	a.mu.Lock()
	frame := a.scopes[id]
	if frame == nil {
		a.mu.Unlock()
		return nil
	}
	if frame.closing {
		a.mu.Unlock()
		return fmt.Errorf("scope %q already closing", id)
	}
	// The workflow owns nesting: a parent cannot conceal an unclosed child.
	for key, child := range a.scopes {
		if key != "" && child.parent == id {
			a.mu.Unlock()
			return fmt.Errorf("scope %q still owns child %q", id, key)
		}
	}
	frame.closing = true
	a.mu.Unlock()
	frame.cancel()
	frame.busy.Wait()
	var cause error
	if reason != "" {
		cause = errors.New(reason)
	}
	err := frame.finish(cause)
	a.mu.Lock()
	delete(a.scopes, id)
	a.mu.Unlock()
	// The body error already travels through the workflow. Return cleanup errors
	// separately, retaining the full outcome in the scope's terminal record.
	return withoutCause(err, cause)
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
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return compiledscope.Entry{Key: key, Value: raw}
}
func (a *Activities) write(ctx context.Context, id string, base compiledscope.Snapshot, entries ...compiledscope.Entry) (compiledscope.Snapshot, error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return "", err
	}
	defer release()
	return compiledscope.WriteContext(scoped, a.store, base, entries...)
}
func (a *Activities) input(ctx context.Context, id string, ref compiledscope.Snapshot) (context.Context, func(), error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	bound, err := compiledscope.BindContext(scoped, a.store, ref)
	if err != nil {
		release()
		return nil, nil, err
	}
	return bound, release, nil
}
func (a *Activities) Finish(ctx context.Context, reason string) error {
	return a.ExitScope(ctx, "", reason)
}

func (a *Activities) FinishCancelled(ctx context.Context, reason string) error {
	a.mu.Lock()
	frame := a.scopes[""]
	a.mu.Unlock()
	if frame != nil {
		if err := compiledscope.CancelRun(frame.ctx); err != nil {
			return err
		}
	}
	return a.Finish(ctx, reason)
}
