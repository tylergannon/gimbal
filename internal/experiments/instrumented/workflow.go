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

// Data is the complete effective context for each operation, carried inline.
type Data struct {
	Task   string
	Checks Checks
}

func ReviewWorkflow(ctx workflow.Context, in Input) (out Report, err error) {
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
	data := Data{Task: in.Task}
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
	if err = workflow.ExecuteActivity(activityCtx, "Review_RunTests", data).Get(activityCtx, &data.Checks); err != nil {
		return
	}
	err = workflow.ExecuteActivity(activityCtx, "Review_GenerateReport", data).Get(activityCtx, &out)
	if err == nil {
		err = ctx.Err()
	}
	return
}
