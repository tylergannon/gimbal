package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/pi"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// Activities retains local runtime ownership across Temporal activity calls.
// It does not choose the iteration, launch branches, or advance the workflow.
// The experiment deliberately has no activity retries or worker recovery.
type Activities struct {
	busy      sync.WaitGroup
	closing   bool
	project   *host.Project
	workdir   string
	mu        sync.Mutex
	root      context.Context
	iteration *iteration
	finish    chan error
	ended     chan struct{}
	runErr    error
}

type iteration struct {
	ctx   context.Context
	group interface {
		Go(string, func(context.Context) error)
		Wait() error
	}
	finish chan error
	ended  chan struct{}
	err    error
	joined bool
}

func (a *Activities) Initialize(ctx context.Context, data Data) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finish != nil {
		return errors.New("specimen already initialized")
	}
	a.finish, a.ended = make(chan error, 1), make(chan struct{})
	ready := make(chan struct{})
	go func() {
		a.runErr = a.project.Run(a.project.Context(), "instrumented", map[gimbal.WorkflowRole]gimbal.ModelBinding{
			coder: {Adapter: claude.New(), Model: "claude-haiku-4-5"},
			coach: {Adapter: pi.New(), Model: "diffusion/deepseek-4.1-flash"},
		}, func(root context.Context) error {
			gimbal.Set(root, "task", data.Task)
			a.root = root
			close(ready)
			var result error
			select {
			case result = <-a.finish:
			case <-root.Done():
				result = root.Err()
			}
			a.mu.Lock()
			a.closing = true
			a.mu.Unlock()
			a.busy.Wait()
			if a.iteration != nil {
				a.iteration.finish <- result
				<-a.iteration.ended
				result = errors.Join(result, a.iteration.err)
			}
			return result
		})
		close(a.ended)
	}()
	select {
	case <-ready:
		return nil
	case <-a.ended:
		return a.runErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// operation binds cancellation and heartbeats to this activity, without making
// the Initialize activity's short-lived context own the whole Gimbal run.
func (a *Activities) operation(ctx, parent context.Context) (context.Context, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if parent == nil || a.closing {
		return nil, nil, errors.New("specimen scope unavailable")
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	a.busy.Add(1)
	scoped, cancel := context.WithCancel(parent)
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
	return scoped, func() { stop(); cancel(); <-done; a.busy.Done() }, nil
}

func (a *Activities) Prepare(ctx context.Context) error {
	_, release, err := a.operation(ctx, a.root)
	if err != nil {
		return err
	}
	defer release()
	return prepareFixture(a.workdir)
}

func (a *Activities) BeginIteration(ctx context.Context, data IterationData) error {
	_, release, err := a.operation(ctx, a.root)
	if err != nil {
		return err
	}
	defer release()
	if a.iteration != nil {
		return errors.New("previous iteration still open")
	}
	it := &iteration{finish: make(chan error, 1), ended: make(chan struct{})}
	ready := make(chan struct{})
	a.iteration = it
	go func() {
		it.err = gimbal.Scope(a.root, "pairs", func(scope context.Context) error {
			it.ctx = scope
			gimbal.Set(scope, "iteration", fmt.Sprint(data.Pair.Number))
			gimbal.SetJSON(scope, "previous", data.Previous)
			it.group = gimbal.Group(scope, "fixes")
			close(ready)
			// Only the owner tears down this frame. Root cancellation first
			// joins active activities, then sends finish; otherwise it could
			// race IterationTests into a second Group.Wait.
			result := <-it.finish
			if !it.joined {
				result = errors.Join(result, it.group.Wait())
			}
			return result
		})
		close(it.ended)
	}()
	select {
	case <-ready:
		return nil
	case <-it.ended:
		return it.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Activities) FixLeft(ctx context.Context, assignment Assignment) (Report, error) {
	var out Report
	done := make(chan error, 1)
	a.iteration.group.Go("left", func(scope context.Context) error {
		var err error
		out, err = a.repair(ctx, scope, assignment)
		done <- err
		return err
	})
	return waitRepair(done, &out)
}

func (a *Activities) FixRight(ctx context.Context, assignment Assignment) (Report, error) {
	var out Report
	done := make(chan error, 1)
	a.iteration.group.Go("right", func(scope context.Context) error {
		var err error
		out, err = a.repair(ctx, scope, assignment)
		done <- err
		return err
	})
	return waitRepair(done, &out)
}

func waitRepair(done <-chan error, out *Report) (Report, error) {
	err := <-done
	// Keep the operator-killed exception recognizable across the Temporal boundary.
	if _, ok := errors.AsType[gimbal.Killed](err); ok {
		err = temporal.NewNonRetryableApplicationError(err.Error(), "Killed", err)
	}
	return *out, err
}

func (a *Activities) repair(ctx, scope context.Context, assignment Assignment) (Report, error) {
	scoped, release, err := a.operation(ctx, scope)
	if err != nil {
		return Report{}, err
	}
	defer release()
	gimbal.SetJSON(scoped, "assignment", assignment)
	principal := gimbal.NewSession(scoped, coder, a.workdir)
	supervisor := gimbal.NewSession(scoped, coach, a.workdir)
	return principal.Generate[Report](scoped, repairPrompt, gimbal.WithSupervisor(supervisor, coachPrompt))
}

func (a *Activities) IterationTests(ctx context.Context, data IterationData) (Checks, error) {
	scoped, release, err := a.operation(ctx, a.iteration.ctx)
	if err != nil {
		return Checks{}, err
	}
	defer release()
	err = a.iteration.group.Wait()
	a.iteration.joined = true
	if err != nil {
		return Checks{}, err
	}
	code, stdout, stderr, err := gimbal.RunCommand(scoped, "tests", a.workdir, "go", "test", "-count=1", "-run", data.Pair.Test, "./...")
	return checked(code, stdout, stderr, err)
}

func (a *Activities) EndIteration(ctx context.Context) error {
	_, release, err := a.operation(ctx, a.root)
	if err != nil {
		return err
	}
	defer release()
	it := a.iteration
	it.finish <- nil
	select {
	case <-it.ended:
		a.iteration = nil
		return it.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Activities) FinalTests(ctx context.Context) (Checks, error) {
	scoped, release, err := a.operation(ctx, a.root)
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
	a.mu.Lock()
	finish, ended := a.finish, a.ended
	a.mu.Unlock()
	if finish == nil {
		return nil
	}
	var result error
	if reason != "" {
		result = errors.New(reason)
	}
	select {
	case finish <- result:
	case <-ended:
	}
	select {
	case <-ended:
		if reason == "" {
			return a.runErr
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
