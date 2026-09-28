package main

import (
	"errors"
	"fmt"
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
	// Scheduling honors cancellation; waiting drains the scheduled activity before
	// lexical cleanup advances, even when the workflow context is cancelled.
	wait, _ := workflow.NewDisconnectedContext(activityCtx)
	// Root cleanup also covers an activity whose initialization completed just
	// as cancellation arrived. The worker's close is harmless if entry failed.
	defer func() {
		cleanup := cleanupContext(activityCtx)
		err = errors.Join(err, workflow.ExecuteActivity(cleanup, "Finish", errorText(err)).Get(cleanup, nil))
	}()
	if err = workflow.ExecuteActivity(activityCtx, "Initialize", Data(in)).Get(wait, nil); err != nil {
		return
	}
	if err = workflow.ExecuteActivity(activityCtx, "Prepare").Get(wait, nil); err != nil {
		return
	}
	previous := Checks{}
	for _, pair := range pairs {
		// One function-return boundary per authored Iterate scope. Its defer runs
		// before the next iteration, including failure and cancellation paths.
		if err = func() (err error) {
			iterationID := fmt.Sprintf("pairs.%d", pair.Number)
			defer func() {
				cleanup := cleanupContext(activityCtx)
				err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", iterationID, errorText(err)).Get(cleanup, nil))
			}()
			if err = workflow.ExecuteActivity(activityCtx, "EnterScope", ScopeInput{iterationID, "", "pairs"}).Get(wait, nil); err != nil {
				return
			}
			data := IterationData{Pair: pair, Previous: previous}
			if err = workflow.ExecuteActivity(activityCtx, "SetIteration", iterationID, data).Get(wait, nil); err != nil {
				return
			}
			if err = func() (err error) { // authored Group / Wait boundary
				groupID := iterationID + "/fixes.1"
				defer func() {
					cleanup := cleanupContext(activityCtx)
					err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", groupID, errorText(err)).Get(cleanup, nil))
				}()
				if err = workflow.ExecuteActivity(activityCtx, "EnterScope", ScopeInput{groupID, iterationID, "fixes"}).Get(wait, nil); err != nil {
					return
				}
				branches, cancel := workflow.WithCancel(activityCtx)
				defer cancel()
				left, leftDone := workflow.NewFuture(ctx)
				right, rightDone := workflow.NewFuture(ctx)
				workflow.Go(branches, func(branch workflow.Context) {
					wait, _ := workflow.NewDisconnectedContext(branch)
					var report Report
					branchErr := func() (err error) { // authored group.Go("left")
						id := groupID + "/left.1"
						defer func() {
							cleanup := cleanupContext(branch)
							err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", id, errorText(err)).Get(cleanup, nil))
						}()
						if err = workflow.ExecuteActivity(branch, "EnterScope", ScopeInput{id, groupID, "left"}).Get(wait, nil); err != nil {
							return
						}
						err = workflow.ExecuteActivity(branch, "Repair", id, pair.Left).Get(wait, &report)
						return
					}()
					leftDone.Set(report, branchErr)
				})
				workflow.Go(branches, func(branch workflow.Context) {
					wait, _ := workflow.NewDisconnectedContext(branch)
					var report Report
					branchErr := func() (err error) { // authored group.Go("right")
						id := groupID + "/right.1"
						defer func() {
							cleanup := cleanupContext(branch)
							err = errors.Join(err, workflow.ExecuteActivity(cleanup, "ExitScope", id, errorText(err)).Get(cleanup, nil))
						}()
						if err = workflow.ExecuteActivity(branch, "EnterScope", ScopeInput{id, groupID, "right"}).Get(wait, nil); err != nil {
							return
						}
						err = workflow.ExecuteActivity(branch, "Repair", id, pair.Right).Get(wait, &report)
						return
					}()
					rightDone.Set(report, branchErr)
				})
				var reports [2]Report
				selector := workflow.NewSelector(ctx)
				join, _ := workflow.NewDisconnectedContext(ctx)
				for i, future := range []workflow.Future{left, right} {
					selector.AddFuture(future, func(f workflow.Future) {
						e := f.Get(join, &reports[i])
						if e != nil {
							if err == nil {
								err = e
							} // Gimbal Group returns the first failure.
							application, killed := errors.AsType[*temporal.ApplicationError](e)
							if !killed || application.Type() != "Killed" {
								cancel()
							}
						}
					})
				}
				selector.Select(join)
				selector.Select(join)
				if err == nil {
					out.Reports = append(out.Reports, reports[:]...)
				}
				return
			}(); err != nil {
				return
			}
			if err = workflow.ExecuteActivity(activityCtx, "IterationTests", iterationID, data).Get(wait, &previous); err != nil {
				return
			}
			out.Iterations = append(out.Iterations, previous)
			return
		}(); err != nil {
			return
		}
	}
	err = workflow.ExecuteActivity(activityCtx, "FinalTests").Get(wait, &out.Final)
	if err == nil {
		err = ctx.Err()
	}
	return
}

// Only error transport formatting is shared. Scope entry, exit and scheduling
// remain visible at each generated call site.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Resource cleanup has its own bounded activity lifetime; it does not inherit
// the turn's heartbeat deadline. Adapter Close already has a per-session bound.
func cleanupContext(ctx workflow.Context) workflow.Context {
	cleanup, _ := workflow.NewDisconnectedContext(ctx)
	options := workflow.GetActivityOptions(ctx)
	options.HeartbeatTimeout = 0
	options.StartToCloseTimeout = time.Minute
	options.ScheduleToStartTimeout = 10 * time.Second
	return workflow.WithActivityOptions(cleanup, options)
}
