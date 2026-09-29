package main

import (
	"context"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
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
	return a.enterScope(in, gimbal.OpenLoop)
}
func (a *Activities) EnterTaskScope(_ context.Context, in ScopeInput, inherited contextdata.Snapshot, task gimbal.Task) (contextdata.Snapshot, error) {
	var snapshot contextdata.Snapshot
	err := a.enterScope(in, func(ctx context.Context, name string) (context.Context, func(error) error, error) {
		ctx, ref, finish, err := gimbal.OpenTask(ctx, name, task, inherited)
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
	answer, err := gimbal.PlanNext(scoped, in.Context, session, goal, tasks, previous)
	var out planValue
	if err == nil {
		out.Tasks = answer.Tasks
		if answer.Next.Present {
			next := answer.Next.Value
			out.Next = &next
		}
	}
	return planResult{out, failure(err)}, nil
}
func (a *Activities) RecordPlan(ctx context.Context, id, goal string, p planValue) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	plan := gimbal.Plan{Tasks: p.Tasks}
	if p.Next != nil {
		plan.Next.Present = true
		plan.Next.Value = *p.Next
	}
	return gimbal.RecordPlan(scoped, goal, plan)
}

type checkResult struct {
	commandResult
	Context contextdata.Snapshot
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
	ref, err := gimbal.CheckContext(scoped, in.Context, key, dir, command, args...)
	return checkResult{commandResult{Failure: failure(err)}, ref}, nil
}
func (a *Activities) TaskFeedback(ctx context.Context, id string) (string, error) {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return "", err
	}
	defer release()
	return gimbal.TaskFeedback(scoped)
}
