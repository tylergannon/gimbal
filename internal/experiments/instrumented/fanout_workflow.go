package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/fanout"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type fanoutInput struct {
	Input
	Items [2]fanout.Work
}

func (a *Activities) NewBranch(ctx context.Context, id string) (sessionHandle, error) {
	return a.newSession(ctx, id, "worker")
}
func (a *Activities) FanoutLeft(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, fanout.Prompt)
	return generateResult{value, failure(err)}, nil
}
func (a *Activities) FanoutRight(ctx context.Context, in operationInput) (generateResult, error) {
	scoped, session, release, err := a.turnInput(ctx, in)
	if err != nil {
		return generateResult{}, err
	}
	defer release()
	value, err := session.Generate[Report](scoped, fanout.Prompt)
	return generateResult{value, failure(err)}, nil
}
func FanoutWorkflow(ctx workflow.Context, in fanoutInput) (out [2]Report, err error) {
	defer func() {
		if ctx.Err() != nil {
			err = temporal.NewCanceledError(errorText(err))
		}
	}()
	control := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: controlQueue, StartToCloseTimeout: 2 * time.Minute, WaitForCancellation: true, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	defer func() {
		cleanup := cleanupContext(control)
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ReleaseEnvironment").Get(cleanup, nil))
	}()
	var env Environment
	if err = workflow.ExecuteActivity(control, "ProvisionEnvironment").Get(control, &env); err != nil {
		return
	}
	ops := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: env.Queue, ScheduleToStartTimeout: time.Minute, StartToCloseTimeout: 5 * time.Minute, HeartbeatTimeout: 15 * time.Second, WaitForCancellation: true, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	wait, _ := workflow.NewDisconnectedContext(ops)
	defer func() {
		cleanup := cleanupContext(ops)
		finish := "Finish"
		if ctx.Err() != nil {
			finish = "FinishCancelled"
			err = errors.Join(err, ctx.Err())
		}
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, finish, errorText(err)).Get(cleanup, nil))
	}()
	var root compiledscope.Snapshot
	if err = workflow.ExecuteActivity(ops, "Initialize", Data{Context: in.Context, Name: "fanout"}).Get(wait, &root); err != nil {
		return
	}
	if err = workflow.ExecuteActivity(ops, "EnterScope", ScopeInput{"workers.1", "", "workers"}).Get(wait, nil); err != nil {
		return
	}
	branches, cancel := workflow.WithCancel(ops)
	futures := make([]workflow.Future, 0, 2)
	// Two source-authored branches, each with its own Generate activity type.
	if err == nil {
		id := "workers.1/left.1"
		item := in.Items[0]
		if err = workflow.ExecuteActivity(ops, "EnterScope", ScopeInput{id, "workers.1", "left"}).Get(wait, nil); err != nil {
			cancel()
		} else {
			future, done := workflow.NewFuture(ctx)
			futures = append(futures, future)
			workflow.Go(branches, func(branch workflow.Context) {
				drain, _ := workflow.NewDisconnectedContext(branch)
				var value generateResult
				branchErr := func() (bodyErr error) { // authored group callback
					cleanup := cleanupContext(branch)
					defer func() {
						bodyErr = errors.Join(bodyErr, workflow.ExecuteActivity(cleanup, "ExitScope", id, errorText(bodyErr)).Get(cleanup, nil))
					}()
					snapshot := root
					assignment := fmt.Sprintf("file=%s receipt=%s delay=%d", item.File, item.Receipt, item.Delay)
					if bodyErr = workflow.ExecuteActivity(branch, "SetValue", id, snapshot, contextEntry("assignment", assignment)).Get(drain, &snapshot); bodyErr != nil {
						return
					}
					var session sessionHandle
					if bodyErr = workflow.ExecuteActivity(branch, "NewBranch", id).Get(drain, &session); bodyErr != nil {
						return
					}
					if bodyErr = workflow.ExecuteActivity(branch, "FanoutLeft", operationInput{id, snapshot, session}).Get(drain, &value); bodyErr != nil {
						return
					}
					return value.Err()
				}()
				if branchErr != nil {
					cancel()
				}
				done.Set(value.Value, branchErr)
			})
		}
	}
	if err == nil {
		id := "workers.1/right.1"
		item := in.Items[1]
		if err = workflow.ExecuteActivity(ops, "EnterScope", ScopeInput{id, "workers.1", "right"}).Get(wait, nil); err != nil {
			cancel()
		} else {
			future, done := workflow.NewFuture(ctx)
			futures = append(futures, future)
			workflow.Go(branches, func(branch workflow.Context) {
				drain, _ := workflow.NewDisconnectedContext(branch)
				var value generateResult
				branchErr := func() (bodyErr error) { // authored group callback
					cleanup := cleanupContext(branch)
					defer func() {
						bodyErr = errors.Join(bodyErr, workflow.ExecuteActivity(cleanup, "ExitScope", id, errorText(bodyErr)).Get(cleanup, nil))
					}()
					snapshot := root
					assignment := fmt.Sprintf("file=%s receipt=%s delay=%d", item.File, item.Receipt, item.Delay)
					if bodyErr = workflow.ExecuteActivity(branch, "SetValue", id, snapshot, contextEntry("assignment", assignment)).Get(drain, &snapshot); bodyErr != nil {
						return
					}
					var session sessionHandle
					if bodyErr = workflow.ExecuteActivity(branch, "NewBranch", id).Get(drain, &session); bodyErr != nil {
						return
					}
					if bodyErr = workflow.ExecuteActivity(branch, "FanoutRight", operationInput{id, snapshot, session}).Get(drain, &value); bodyErr != nil {
						return
					}
					return value.Err()
				}()
				if branchErr != nil {
					cancel()
				}
				done.Set(value.Value, branchErr)
			})
		}
	}
	selector := workflow.NewSelector(wait)
	for i, future := range futures {
		selector.AddFuture(future, func(f workflow.Future) {
			if e := f.Get(wait, &out[i]); e != nil && err == nil {
				err = e
				cancel()
			}
		})
	}
	for range futures {
		selector.Select(wait)
	}
	cancel()
	cleanup := cleanupContext(ops)
	err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", "workers.1", errorText(err)).Get(cleanup, nil))
	if err != nil {
		return
	}
	for i, result := range out {
		if result.Receipt != in.Items[i].Receipt || result.File != in.Items[i].File {
			err = fmt.Errorf("misassigned result %d", i)
			return
		}
	}
	return
}
