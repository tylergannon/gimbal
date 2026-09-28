package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
)

// Handles refer to resources owned by a worker scope, never a transient activity.
type sessionHandle struct{ Owner, ID string }
type operationInput struct {
	Scope   string
	Context compiledscope.Snapshot
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

type generateResult struct {
	Value   Report
	Failure *operationFailure
}

func (r generateResult) Err() error {
	if r.Failure == nil {
		return nil
	}
	return r.Failure
}

func (a *Activities) NewParent(ctx context.Context) (sessionHandle, error) {
	return a.newSession(ctx, "", "parent")
}
func (a *Activities) newSession(ctx context.Context, owner, id string) (sessionHandle, error) {
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
	f.sessions[id] = gimbal.NewSession(scoped, coder, a.workdir)
	return sessionHandle{owner, id}, nil
}
func (a *Activities) session(in operationInput) (*gimbal.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// The creating scope must be this operation's scope or an ancestor.
	for id := in.Scope; ; {
		f := a.scopes[id]
		if f == nil || f.closing {
			break
		}
		if id == in.Session.Owner {
			s := f.sessions[in.Session.ID]
			if s == nil {
				break
			}
			return s, nil
		}
		if id == "" {
			break
		}
		id = f.parent
	}
	return nil, fmt.Errorf("session %s is unavailable in scope %s", in.Session.ID, in.Scope)
}
func (a *Activities) turnInput(ctx context.Context, in operationInput) (context.Context, *gimbal.Session, func(), error) {
	scoped, release, err := a.input(ctx, in.Scope, in.Context)
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
func (a *Activities) Remember(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, continuity.RememberPrompt)
	return generateResult{value, failure(err)}, nil
}
func (a *Activities) Edit(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, continuity.EditPrompt)
	return generateResult{value, failure(err)}, nil
}
func (a *Activities) Diverge(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, continuity.ForkPrompt)
	return generateResult{value, failure(err)}, nil
}
func (a *Activities) Resume(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, continuity.ResumePrompt)
	return generateResult{value, failure(err)}, nil
}
func (a *Activities) ForkSession(ctx context.Context, in operationInput) (sessionHandle, error) {
	scoped, release, err := a.input(ctx, in.Scope, in.Context)
	if err != nil {
		return sessionHandle{}, err
	}
	defer release()
	s, err := a.session(in)
	if err != nil {
		return sessionHandle{}, err
	}
	fork, err := s.Fork(scoped, "fork")
	if err != nil {
		return sessionHandle{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.scopes[in.Scope]
	if f.sessions == nil {
		f.sessions = map[string]*gimbal.Session{}
	}
	f.sessions["fork"] = fork
	return sessionHandle{in.Scope, "fork"}, nil
}
func (a *Activities) SetValue(ctx context.Context, id string, base compiledscope.Snapshot, entry compiledscope.Entry) (compiledscope.Snapshot, error) {
	return a.write(ctx, id, base, entry)
}
func (a *Activities) command(ctx context.Context, in operationInput, name, command string, args ...string) (commandResult, error) {
	scoped, release, err := a.input(ctx, in.Scope, in.Context)
	if err != nil {
		return commandResult{}, err
	}
	defer release()
	code, out, stderr, err := gimbal.RunCommand(scoped, name, a.workdir, command, args...)
	return commandResult{code, out, stderr, failure(err)}, nil
}
func (a *Activities) Diagnostic(ctx context.Context, in operationInput) (commandResult, error) {
	return a.command(ctx, in, "diagnostic", "sh", "-c", "printf observed; printf diagnostic >&2; exit 7")
}
func (a *Activities) Recover(ctx context.Context, in operationInput) (commandResult, error) {
	return a.command(ctx, in, "recovery", "sh", "-c", "printf recovered > recovery.txt")
}
func (a *Activities) MissingCommand(ctx context.Context, in operationInput) (commandResult, error) {
	return a.command(ctx, in, "missing", filepath.Join(a.workdir, "does-not-exist"))
}
