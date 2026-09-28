package main

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const controlQueue = "instrumented-control"

type Input struct{ Task string }
type Environment struct {
	Name  string
	Queue string
	URL   string
}

// Data initializes root context; iteration and assignment inputs travel inline
// at their respective scope boundaries.
type Data struct{ Task string }
type IterationData struct {
	Pair     Pair
	Previous Checks
}
type Outcome struct {
	Reports    []Report
	Iterations []Checks
	Final      Checks
}

func ReviewWorkflow(ctx workflow.Context, in Input) (out Outcome, err error) {
	control := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue: controlQueue, StartToCloseTimeout: 2 * time.Minute, WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	var env Environment
	// Cleanup addresses the deterministic container name even if provisioning fails.
	defer func() {
		cleanup, _ := workflow.NewDisconnectedContext(control)
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ReleaseEnvironment").Get(cleanup, nil))
	}()
	if err = workflow.ExecuteActivity(control, "ProvisionEnvironment").Get(control, &env); err != nil {
		return
	}
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue: env.Queue, ScheduleToStartTimeout: time.Minute, StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout: 15 * time.Second, WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	data := Data(in)
	if err = workflow.ExecuteActivity(activityCtx, "Initialize", data).Get(activityCtx, nil); err != nil {
		return
	}
	defer func() {
		cleanup, _ := workflow.NewDisconnectedContext(activityCtx)
		cleanup = workflow.WithActivityOptions(cleanup, workflow.ActivityOptions{TaskQueue: env.Queue, ScheduleToStartTimeout: 10 * time.Second, StartToCloseTimeout: 20 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "Finish", reason).Get(cleanup, nil))
	}()
	if err = workflow.ExecuteActivity(activityCtx, "Prepare").Get(activityCtx, nil); err != nil {
		return
	}
	previous := Checks{}
	for _, pair := range pairs {
		data := IterationData{Pair: pair, Previous: previous}
		if err = workflow.ExecuteActivity(activityCtx, "BeginIteration", data).Get(activityCtx, nil); err != nil {
			return
		}
		branches, cancel := workflow.WithCancel(activityCtx)
		left := workflow.ExecuteActivity(branches, "FixLeft", pair.Left)
		right := workflow.ExecuteActivity(branches, "FixRight", pair.Right)
		var reports [2]Report
		var branchErr error
		selector := workflow.NewSelector(ctx)
		// A disconnected wait drains both activities even when the parent is cancelled.
		join, _ := workflow.NewDisconnectedContext(ctx)
		for i, future := range []workflow.Future{left, right} {
			selector.AddFuture(future, func(f workflow.Future) {
				e := f.Get(join, &reports[i])
				if e != nil {
					branchErr = errors.Join(branchErr, e)
					application, killed := errors.AsType[*temporal.ApplicationError](e)
					if !killed || application.Type() != "Killed" {
						cancel()
					}
				}
			})
		}
		selector.Select(join)
		selector.Select(join)
		cancel()
		if branchErr != nil {
			err = branchErr
			return
		}
		out.Reports = append(out.Reports, reports[:]...)
		if err = workflow.ExecuteActivity(activityCtx, "IterationTests", data).Get(activityCtx, &previous); err != nil {
			return
		}
		out.Iterations = append(out.Iterations, previous)
		if err = workflow.ExecuteActivity(activityCtx, "EndIteration").Get(activityCtx, nil); err != nil {
			return
		}
	}
	err = workflow.ExecuteActivity(activityCtx, "FinalTests").Get(activityCtx, &out.Final)
	if err == nil {
		err = ctx.Err()
	}
	return
}
