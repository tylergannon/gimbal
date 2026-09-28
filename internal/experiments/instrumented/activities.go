package main

import (
	"context"
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
	"go.temporal.io/sdk/temporal"
)

// Activities owns worker-local resources, never workflow scheduling. Every
// frame is explicitly opened/closed by generated lexical code. No callback or
// goroutine waits between activities to keep a scope alive.
type Activities struct {
	project *host.Project
	workdir string
	mu      sync.Mutex
	scopes  map[string]*scopeFrame
}

type scopeFrame struct {
	parent  string
	ctx     context.Context
	cancel  context.CancelFunc
	finish  func(error) error
	busy    sync.WaitGroup
	closing bool
}

type ScopeInput struct{ ID, Parent, Name string }

func (a *Activities) Initialize(_ context.Context, data Data) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scopes != nil {
		return errors.New("specimen already initialized")
	}
	root, finish, err := a.project.OpenCompiledRun(a.project.Context(), "instrumented", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		coder: {Adapter: claude.New(), Model: "claude-haiku-4-5"},
		coach: {Adapter: pi.New(), Model: "diffusion/deepseek-4.1-flash"},
	})
	if err != nil {
		return err
	}
	root, cancel := context.WithCancel(root)
	a.scopes = map[string]*scopeFrame{"": {ctx: root, cancel: cancel, finish: finish}}
	gimbal.Set(root, "task", data.Task)
	return nil
}

func (a *Activities) EnterScope(_ context.Context, in ScopeInput) error {
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
	child, finish, err := compiledscope.OpenScope(ctx, in.Name)
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

func (a *Activities) Prepare(ctx context.Context) error {
	_, release, err := a.operation(ctx, "")
	if err != nil {
		return err
	}
	defer release()
	return prepareFixture(a.workdir)
}

func (a *Activities) SetIteration(ctx context.Context, id string, data IterationData) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	gimbal.Set(scoped, "iteration", fmt.Sprint(data.Pair.Number))
	gimbal.SetJSON(scoped, "previous", data.Previous)
	gimbal.Set(scoped, "layer", "iteration-layer")
	return nil
}

// These activities keep the authored Set operations in the worker. Only small
// values are exercised; they do not define a context-spill transport protocol.
func (a *Activities) SetOuterContext(ctx context.Context, id string) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	gimbal.Set(scoped, "layer", "outer-layer")
	gimbal.Set(scoped, "inherited", "outer-inherited")
	return nil
}

func (a *Activities) SetInnerContext(ctx context.Context, id string) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	gimbal.Set(scoped, "layer", "inner-layer")
	gimbal.Set(scoped, "child-only", "inner-private")
	return nil
}

func (a *Activities) Repair(ctx context.Context, id string, assignment Assignment) (out Report, err error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return out, err
	}
	defer release()
	gimbal.SetJSON(scoped, "assignment", assignment)
	principal := gimbal.NewSession(scoped, coder, a.workdir)
	supervisor := gimbal.NewSession(scoped, coach, a.workdir)
	out, err = principal.Generate[Report](scoped, repairPrompt, gimbal.WithSupervisor(supervisor, coachPrompt))
	if _, ok := errors.AsType[gimbal.Killed](err); ok {
		err = temporal.NewNonRetryableApplicationError(err.Error(), "Killed", err)
	}
	return
}

func (a *Activities) IterationTests(ctx context.Context, id string, data IterationData) (Checks, error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return Checks{}, err
	}
	defer release()
	code, stdout, stderr, err := gimbal.RunCommand(scoped, "tests", a.workdir, "go", "test", "-count=1", "-run", data.Pair.Test, "./...")
	return checked(code, stdout, stderr, err)
}

func (a *Activities) FinalTests(ctx context.Context) (Checks, error) {
	scoped, release, err := a.operation(ctx, "")
	if err != nil {
		return Checks{}, err
	}
	defer release()
	code, stdout, stderr, err := gimbal.RunCommand(scoped, "final-tests", a.workdir, "go", "test", "-count=1", "./...")
	return checked(code, stdout, stderr, err)
}

func checked(code int, stdout, stderr string, err error) (Checks, error) {
	if err == nil && code != 0 {
		err = fmt.Errorf("checks exited %d: %s%s", code, stdout, stderr)
	}
	return Checks{code, stdout, stderr}, err
}

func (a *Activities) Finish(ctx context.Context, reason string) error {
	return a.ExitScope(ctx, "", reason)
}
