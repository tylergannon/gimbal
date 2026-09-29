package main

import (
	"time"

	"github.com/tylergannon/gimbal/contextdata"
	"go.temporal.io/sdk/workflow"
)

var controlQueue = "temporal-consumer"

type Input struct{ Context contextdata.Snapshot }
type Environment struct {
	Name  string
	Queue string
	URL   string
}

// Generated control flow carries immutable context references, not live scopes.
type Data struct {
	Context contextdata.Snapshot
	Name    string
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
