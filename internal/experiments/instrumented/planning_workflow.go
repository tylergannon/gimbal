package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type planningOutcome struct {
	Tasks     []Report
	Decisions int
	Feedback  string
}

func PlanningWorkflow(ctx workflow.Context, in Input) (out planningOutcome, err error) {
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
	if err = workflow.ExecuteActivity(ops, "Initialize", Data{Context: in.Context, Name: "planning"}).Get(wait, &root); err != nil {
		return
	}
	var planner sessionHandle
	if err = workflow.ExecuteActivity(ops, "NewParent").Get(wait, &planner); err != nil {
		return
	}
	if err = workflow.ExecuteActivity(ops, "EnterLoopScope", ScopeInput{"plan.1", "", "plan"}).Get(wait, nil); err != nil {
		return
	}
	var loopErr error
	defer func() {
		cleanup := cleanupContext(ops)
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", "plan.1", errorText(loopErr)).Get(cleanup, nil))
	}()
	tasks := []gimbal.Task{}
	for iteration := 1; ; iteration++ {
		var plan planResult
		if err = workflow.ExecuteActivity(ops, "Plan", operationInput{"plan.1", root, planner}, tasks, out.Feedback).Get(wait, &plan); err != nil {
			loopErr = err
			return
		}
		if err = plan.Err(); err != nil {
			loopErr = err
			return
		}
		out.Decisions++
		if err = workflow.ExecuteActivity(ops, "RecordPlan", "plan.1", plan.Value).Get(wait, nil); err != nil {
			loopErr = err
			return
		}
		tasks = plan.Value.Tasks
		if plan.Value.Next == nil {
			break
		}
		// RecordPlan validates the typed plan before indexing; the workflow owns
		// the selection and the loop, not the worker-side planner dispatch.
		task := tasks[*plan.Value.Next]
		id := fmt.Sprintf("plan.1/task.%d", iteration)
		if err = workflow.ExecuteActivity(ops, "EnterTaskScope", ScopeInput{id, "plan.1", "task"}, task).Get(wait, nil); err != nil {
			loopErr = err
			return
		}
		snapshot := root
		err = workflow.ExecuteActivity(ops, "SetValue", id, snapshot, contextEntry("task", task)).Get(wait, &snapshot)
		var session sessionHandle
		if err == nil {
			err = workflow.ExecuteActivity(ops, "NewBranch", id).Get(wait, &session)
		}
		var result generateResult
		if err == nil {
			err = workflow.ExecuteActivity(ops, "TaskTurn", operationInput{id, snapshot, session}).Get(wait, &result)
		}
		if err == nil {
			err = result.Err()
		}
		if err == nil {
			err = workflow.ExecuteActivity(ops, "SetValue", id, snapshot, contextEntry("result", result.Value)).Get(wait, &snapshot)
		}
		var check checkResult
		if err == nil {
			err = workflow.ExecuteActivity(ops, "TaskCheck", operationInput{id, snapshot, session}).Get(wait, &check)
			if err == nil {
				snapshot = check.Context
				err = check.Err()
			}
		}
		if err == nil {
			err = workflow.ExecuteActivity(ops, "TaskFeedback", operationInput{id, snapshot, session}, []string{"task", "result", "check"}).Get(wait, &out.Feedback)
		}
		// Explicit iterator exit: no synthetic function boundary around its body.
		// A source body return is not an error returned to the iterator callback.
		cleanup := cleanupContext(ops)
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", id, "").Get(cleanup, nil))
		if err != nil {
			return
		}
		out.Tasks = append(out.Tasks, result.Value)
	}
	return
}
