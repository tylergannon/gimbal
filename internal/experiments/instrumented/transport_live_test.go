package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func largeResultWorkflow(ctx workflow.Context) (out Report, err error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	defer func() {
		cleanup := cleanupContext(ctx)
		_ = workflow.ExecuteActivity(cleanup, "Finish", errorText(err)).Get(cleanup, nil)
	}()
	var snapshot compiledscope.Snapshot
	if err = workflow.ExecuteActivity(ctx, "Initialize", Data{Name: "large-result"}).Get(ctx, &snapshot); err != nil {
		return
	}
	var parent sessionHandle
	if err = workflow.ExecuteActivity(ctx, "NewParent").Get(ctx, &parent); err != nil {
		return
	}
	var result generateResult
	if err = workflow.ExecuteActivity(ctx, "ContinuityGenerate1", operationInput{"", snapshot, parent}).Get(ctx, &result); err != nil {
		return
	}
	if err = result.Err(); err != nil {
		return
	}
	out = result.Value
	// The authored consumer can index, compare, transform and hand off the full
	// typed value without knowing it was externalized by the transport.
	if len(out.Summary) != 3_300_000 || out.Summary[1_650_000:1_650_011] != "whole-value" {
		return out, fmt.Errorf("large result changed")
	}
	if err = workflow.ExecuteActivity(ctx, "SetValue", "", snapshot, contextEntry("result", out)).Get(ctx, &snapshot); err != nil {
		return
	}
	var length int
	if err = workflow.ExecuteActivity(ctx, "ReadLarge", snapshot).Get(ctx, &length); err != nil {
		return
	}
	if length != len(out.Summary) {
		err = fmt.Errorf("context handoff truncated")
	}
	return
}
func TestLiveLargeTypedResult(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_TRANSPORT") != "1" {
		t.Skip("requires running Temporal; no provider calls")
	}
	root := t.TempDir()
	c, err := client.Dial(client.Options{DataConverter: dataConverter(root)})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("large-result-%d", time.Now().UnixMilli())
	a := newTestActivities(t)
	a.store = compiledscope.Store{Root: filepath.Join(root, environmentID(id), "context")}
	want := Report{Summary: strings.Repeat("whole-value", 300000), File: "large", Receipt: "tail"}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Model: "deterministic-large-result", Adapter: &specimenAdapter{turn: func(context.Context, string, string) (any, error) { return want, nil }}}}
	w := worker.New(c, id, worker.Options{})
	w.RegisterWorkflow(largeResultWorkflow)
	w.RegisterActivity(a)
	w.RegisterActivityWithOptions(func(_ context.Context, snapshot compiledscope.Snapshot) (int, error) {
		entries, err := a.store.Load(snapshot)
		if err != nil {
			return 0, err
		}
		raw, err := a.store.Value(entries[0])
		if err != nil {
			return 0, err
		}
		var got Report
		if err = json.Unmarshal(raw, &got); err != nil {
			return 0, err
		}
		if got != want {
			return 0, fmt.Errorf("context differs")
		}
		return len(got.Summary), nil
	}, activity.RegisterOptions{Name: "ReadLarge"})
	if err = w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: id}, largeResultWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	var got Report
	if err = run.Get(ctx, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("client result differs")
	}
	replayRecorded(t, c, id, root)
	t.Logf("workflow %s: %d byte typed Generate result, workflow inspection, Set handoff, later activity read, client return", id, len(got.Summary))
}
