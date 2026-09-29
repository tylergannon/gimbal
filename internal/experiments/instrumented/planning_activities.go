package main

import (
	"context"
	"encoding/json"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
)

type planValue struct {
	Tasks []gimbal.Task `json:"tasks"`
	Next  *int          `json:"next"`
}
type planResult struct {
	Value   planValue
	Failure *operationFailure
}

func (r planResult) Err() error {
	if r.Failure == nil {
		return nil
	}
	return r.Failure
}
func (a *Activities) EnterLoopScope(_ context.Context, in ScopeInput) error {
	return a.enterScope(in, compiledscope.OpenLoop)
}
func (a *Activities) EnterTaskScope(_ context.Context, in ScopeInput, inherited compiledscope.Snapshot, task gimbal.Task) (compiledscope.Snapshot, error) {
	raw, err := json.Marshal(task)
	if err != nil {
		return inherited, err
	}
	var snapshot compiledscope.Snapshot
	err = a.enterScope(in, func(ctx context.Context, name string) (context.Context, func(error) error, error) {
		ctx, ref, finish, err := compiledscope.OpenTask(ctx, name, raw, inherited)
		snapshot = ref
		return ctx, finish, err
	})
	if err != nil {
		return inherited, err
	}
	return snapshot, nil
}

func (a *Activities) plan(ctx context.Context, in operationInput, name, goal string, tasks []gimbal.Task, previous string) (planResult, error) {
	scoped, release, err := a.operation(ctx, in.Scope)
	if err != nil {
		return planResult{}, err
	}
	defer release()
	session, err := a.session(in)
	if err != nil {
		return planResult{}, err
	}
	raw, err := json.Marshal(tasks)
	if err != nil {
		return planResult{}, err
	}
	answer, err := compiledscope.Plan(scoped, session, in.Context, goal, raw, previous)
	var out planValue
	if err == nil {
		err = json.Unmarshal(answer, &out)
	}
	return planResult{out, failure(err)}, nil
}
func (a *Activities) RecordPlan(ctx context.Context, id, goal string, p planValue) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return compiledscope.RecordPlan(scoped, goal, raw)
}

type checkResult struct {
	commandResult
	Context compiledscope.Snapshot
}

func (a *Activities) check(ctx context.Context, in operationInput, key, command string, args ...string) (checkResult, error) {
	return a.checkAt(ctx, in, key, a.workdir, command, args...)
}
func (a *Activities) checkAt(ctx context.Context, in operationInput, key, dir, command string, args ...string) (checkResult, error) {
	scoped, release, err := a.operation(ctx, in.Scope)
	if err != nil {
		return checkResult{Context: in.Context}, err
	}
	defer release()
	ref, record, err := compiledscope.CheckContext(scoped, in.Context, key, dir, command, args...)
	return checkResult{commandResult{record.ExitCode, record.Stdout, record.Stderr, failure(err)}, ref}, nil
}
func (a *Activities) TaskFeedback(ctx context.Context, id string) (string, error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return "", err
	}
	defer release()
	return compiledscope.TaskFeedback(scoped)
}
