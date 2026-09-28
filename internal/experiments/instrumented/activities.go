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
)

// Activities owns one specimen run. Temporal owns the sequence; this object
// only preserves the local runtime's scope, sessions and single event writer.
type Activities struct {
	busy                  sync.WaitGroup
	closing               bool
	project               *host.Project
	workdir               string
	mu                    sync.Mutex
	scope                 context.Context
	principal, supervisor *gimbal.Session
	finish                chan error
	ended                 chan struct{}
	runErr                error
}

func (a *Activities) Initialize(ctx context.Context, data Data) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finish != nil {
		return errors.New("specimen is already initialized")
	}
	a.finish, a.ended = make(chan error, 1), make(chan struct{})
	ready := make(chan struct{})
	go func() {
		a.runErr = a.project.Run(a.project.Context(), "instrumented", map[gimbal.WorkflowRole]gimbal.ModelBinding{
			coder: {Adapter: claude.New(), Model: "claude-haiku-4-5"},
			coach: {Adapter: pi.New(), Model: "diffusion/deepseek-4.1-flash"},
		}, func(root context.Context) error {
			gimbal.Set(root, "task", data.Task)
			return gimbal.Scope(root, "review", func(scope context.Context) error {
				a.scope = scope
				a.principal = gimbal.NewSession(scope, coder, a.workdir)
				a.supervisor = gimbal.NewSession(scope, coach, a.workdir)
				close(ready)
				var result error
				select {
				case result = <-a.finish:
				case <-scope.Done():
					result = scope.Err()
				}
				a.mu.Lock()
				a.closing = true
				a.mu.Unlock()
				a.busy.Wait()
				return result
			})
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

func (a *Activities) operation(ctx context.Context) (context.Context, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scope == nil || a.closing {
		return nil, nil, errors.New("specimen not initialized")
	}
	if err := a.scope.Err(); err != nil {
		return nil, nil, err
	}
	a.busy.Add(1)
	scoped, cancel := context.WithCancel(a.scope)
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

func (a *Activities) Review_RunTests(ctx context.Context, data Data) (Checks, error) {
	scoped, release, err := a.operation(ctx)
	if err != nil {
		return Checks{}, err
	}
	defer release()
	code, stdout, stderr, err := gimbal.RunCommand(scoped, "tests", a.workdir, "sh", "-c", "printf 'workspace-preserved\\n' > marker.txt; cat marker.txt")
	if err == nil && code != 0 {
		err = fmt.Errorf("checks exited %d: %s", code, stderr)
	}
	return Checks{code, stdout, stderr}, err
}

func (a *Activities) Review_GenerateReport(ctx context.Context, data Data) (Report, error) {
	scoped, release, err := a.operation(ctx)
	if err != nil {
		return Report{}, err
	}
	defer release()
	gimbal.SetJSON(scoped, "checks", data.Checks)
	return a.principal.Generate[Report](scoped, reportPrompt, gimbal.WithSupervisor(a.supervisor, coachPrompt))
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
