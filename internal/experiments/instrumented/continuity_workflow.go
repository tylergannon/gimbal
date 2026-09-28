package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type continuityOutcome struct {
	Decision, Child, Fork, Final Report
	Diagnostic                   commandResult
}

func ContinuityWorkflow(ctx workflow.Context, in Input) (out continuityOutcome, err error) {
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
	if err = workflow.ExecuteActivity(ops, "Initialize", Data{Context: in.Context, Name: "continuity"}).Get(wait, &root); err != nil {
		return
	}
	if err = workflow.ExecuteActivity(ops, "SetValue", "", root, contextEntry("layer", "parent")).Get(wait, &root); err != nil {
		return
	}
	var parent sessionHandle
	if err = workflow.ExecuteActivity(ops, "NewParent").Get(wait, &parent); err != nil {
		return
	}
	var result generateResult
	if err = workflow.ExecuteActivity(ops, "Remember", operationInput{"", root, parent}).Get(wait, &result); err != nil {
		return
	}
	if err = result.Err(); err != nil {
		return
	}
	out.Decision = result.Value
	if out.Decision.Summary == "edit" {
		// This function boundary was already present in the authored Scope callback.
		err = func() (bodyErr error) {
			cleanup := cleanupContext(ops)
			defer func() {
				bodyErr = errors.Join(bodyErr, workflow.ExecuteActivity(cleanup, "ExitScope", "child.1", errorText(bodyErr)).Get(cleanup, nil))
			}()
			if bodyErr = workflow.ExecuteActivity(ops, "EnterScope", ScopeInput{"child.1", "", "child"}).Get(wait, nil); bodyErr != nil {
				return
			}
			child := root
			if bodyErr = workflow.ExecuteActivity(ops, "SetValue", "child.1", child, contextEntry("layer", "child")).Get(wait, &child); bodyErr != nil {
				return
			}
			if bodyErr = workflow.ExecuteActivity(ops, "Edit", operationInput{"child.1", child, parent}).Get(wait, &result); bodyErr != nil {
				return
			}
			if bodyErr = result.Err(); bodyErr != nil {
				return
			}
			out.Child = result.Value
			if bodyErr = workflow.ExecuteActivity(ops, "SetValue", "child.1", child, contextEntry("result", out.Child)).Get(wait, &child); bodyErr != nil {
				return
			}
			var fork sessionHandle
			if bodyErr = workflow.ExecuteActivity(ops, "ForkSession", operationInput{"child.1", child, parent}).Get(wait, &fork); bodyErr != nil {
				return
			}
			if bodyErr = workflow.ExecuteActivity(ops, "Diverge", operationInput{"child.1", child, fork}).Get(wait, &result); bodyErr != nil {
				return
			}
			out.Fork = result.Value
			return result.Err()
		}()
		if err != nil {
			return
		}
	}
	if err = workflow.ExecuteActivity(ops, "Diagnostic", operationInput{"", root, parent}).Get(wait, &out.Diagnostic); err != nil {
		return
	}
	if err = out.Diagnostic.Err(); err != nil {
		return
	}
	d := out.Diagnostic
	if err = workflow.ExecuteActivity(ops, "SetValue", "", root, contextEntry("diagnostic", Checks{d.ExitCode, d.Stdout, d.Stderr})).Get(wait, &root); err != nil {
		return
	}
	if d.ExitCode == 7 {
		var recovered commandResult
		if err = workflow.ExecuteActivity(ops, "Recover", operationInput{"", root, parent}).Get(wait, &recovered); err != nil {
			return
		}
		if err = recovered.Err(); err != nil {
			return
		}
	}
	if err = workflow.ExecuteActivity(ops, "Resume", operationInput{"", root, parent}).Get(wait, &result); err != nil {
		return
	}
	if err = result.Err(); err != nil {
		return
	}
	out.Final = result.Value
	if out.Final.Receipt != "amber-17" || out.Final.Summary != "parent" {
		err = fmt.Errorf("continuation mismatch: %+v", out.Final)
	}
	return
}
