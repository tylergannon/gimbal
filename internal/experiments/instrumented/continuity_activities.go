package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
)

// Handles refer to resources owned by a worker scope, never a transient activity.
type sessionHandle struct{ Owner, ID string }
type operationInput struct {
	Scope   string
	Context contextdata.Snapshot
	Session sessionHandle
}
type operationFailure struct{ Kind, Message string }

func (e *operationFailure) Error() string { return e.Message }
func (e *operationFailure) Is(target error) bool {
	return e.Kind == "cancelled" && target == context.Canceled || e.Kind == "deadline" && target == context.DeadlineExceeded
}
func failure(err error) *operationFailure {
	if err == nil {
		return nil
	}
	kind := "execution"
	if errors.Is(err, context.Canceled) {
		kind = "cancelled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		kind = "deadline"
	}
	return &operationFailure{kind, err.Error()}
}

type commandResult struct {
	ExitCode       int
	Stdout, Stderr string
	Failure        *operationFailure
}

func (r commandResult) Err() error {
	if r.Failure == nil {
		return nil
	}
	return r.Failure
}

type operationResult[T any] struct {
	Value   T
	Failure *operationFailure
}

func (r operationResult[T]) Err() error {
	if r.Failure == nil {
		return nil
	}
	return r.Failure
}

type generateResult = operationResult[Report]
type responseResult = operationResult[[]byte]

func (a *Activities) NewParent(ctx context.Context) (sessionHandle, error) {
	return a.newSession(ctx, "", "parent")
}
func (a *Activities) newSession(ctx context.Context, owner, id string) (sessionHandle, error) {
	return a.OpenSession(ctx, owner, id, coder, a.workdir)
}
func (a *Activities) WorkDir(context.Context) (string, error) { return a.workdir, nil }
func (a *Activities) OpenSession(ctx context.Context, owner, id string, role gimbal.WorkflowRole, workdir string) (sessionHandle, error) {
	scoped, release, err := a.operation(ctx, owner)
	if err != nil {
		return sessionHandle{}, err
	}
	defer release()
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.scopes[owner]
	if f.sessions == nil {
		f.sessions = map[string]*gimbal.Session{}
	}
	if f.sessions[id] != nil {
		return sessionHandle{}, fmt.Errorf("duplicate session %s", id)
	}
	f.sessions[id] = gimbal.NewSession(scoped, role, workdir)
	return sessionHandle{owner, id}, nil
}
func (a *Activities) session(in operationInput) (*gimbal.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Handle resolution is consumer-owned. Gimbal checks whether the resolved
	// session is reachable from the operation's actual scope.
	if f := a.scopes[in.Session.Owner]; f != nil && !f.closing {
		if session := f.sessions[in.Session.ID]; session != nil {
			return session, nil
		}
	}

	return nil, fmt.Errorf("session %s is unavailable in scope %s", in.Session.ID, in.Scope)
}
func (a *Activities) turnInput(ctx context.Context, in operationInput) (context.Context, *gimbal.Session, func(), error) {
	scoped, release, err := a.operation(ctx, in.Scope)
	if err != nil {
		return nil, nil, nil, err
	}
	session, err := a.session(in)
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	return scoped, session, release, nil
}
func (a *Activities) OpenFork(ctx context.Context, in operationInput, name string) (operationResult[sessionHandle], error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return operationResult[sessionHandle]{}, err
	}
	defer release()
	fork, err := session.Fork(scoped, name)
	if err != nil {
		return operationResult[sessionHandle]{Failure: failure(err)}, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.scopes[in.Scope]
	if f.sessions == nil {
		f.sessions = map[string]*gimbal.Session{}
	}
	id := fmt.Sprintf("fork-%d", len(f.sessions)+1)
	f.sessions[id] = fork
	return operationResult[sessionHandle]{Value: sessionHandle{in.Scope, id}}, nil
}
func (a *Activities) SetValue(ctx context.Context, id string, base contextdata.Snapshot, entry contextdata.Entry) (contextdata.Snapshot, error) {
	return a.write(ctx, id, base, entry)
}
func (a *Activities) command(ctx context.Context, in operationInput, name, command string, args ...string) (commandResult, error) {
	return a.commandAt(ctx, in, name, a.workdir, command, args...)
}
func (a *Activities) commandAt(ctx context.Context, in operationInput, name, dir, command string, args ...string) (commandResult, error) {
	scoped, release, err := a.operation(ctx, in.Scope)
	if err != nil {
		return commandResult{}, err
	}
	defer release()
	code, out, stderr, err := gimbal.RunCommand(scoped, name, dir, command, args...)
	return commandResult{code, out, stderr, failure(err)}, nil
}
func (a *Activities) MissingCommand(ctx context.Context, in operationInput) (commandResult, error) {
	return a.command(ctx, in, "missing", filepath.Join(a.workdir, "does-not-exist"))
}
