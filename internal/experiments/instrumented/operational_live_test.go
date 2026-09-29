package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/observation"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func recordedHistory(ctx context.Context, c client.Client, id string) (*historypb.History, error) {
	it := c.GetWorkflowHistory(ctx, id, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	h := &historypb.History{}
	for it.HasNext() {
		e, err := it.Next()
		if err != nil {
			return nil, err
		}
		h.Events = append(h.Events, e)
	}
	return h, nil
}
func waitActivity(t *testing.T, ctx context.Context, c client.Client, id, name string, completed bool) {
	t.Helper()
	for {
		h, err := recordedHistory(ctx, c, id)
		if err != nil {
			t.Fatal(err)
		}
		var scheduled int64
		for _, e := range h.Events {
			if a := e.GetActivityTaskScheduledEventAttributes(); a != nil && a.ActivityType.Name == name {
				scheduled = e.EventId
			}
			if !completed {
				if a := e.GetActivityTaskStartedEventAttributes(); a != nil && a.ScheduledEventId == scheduled && scheduled != 0 {
					return
				}
			}
			if completed {
				if a := e.GetActivityTaskCompletedEventAttributes(); a != nil && a.ScheduledEventId == scheduled && scheduled != 0 {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func replayRecorded(t *testing.T, c client.Client, id string, root string) {
	t.Helper()
	h, err := recordedHistory(t.Context(), c, id)
	if err != nil {
		t.Fatal(err)
	}
	r, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{DataConverter: dataConverter(root)})
	if err != nil {
		t.Fatal(err)
	}
	r.RegisterWorkflow(ContinuityWorkflow)
	r.RegisterWorkflow(FanoutWorkflow)
	r.RegisterWorkflow(PlanningWorkflow)
	r.RegisterWorkflow(largeResultWorkflow)
	if err = r.ReplayWorkflowHistory(nil, h); err != nil {
		t.Fatal(err)
	}
}
func TestLiveOperationalBoundary(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_FAILURE") != "1" {
		t.Skip("requires dedicated gimbal-instrumented-control container; stops/kills only specimen containers")
	}
	for _, mode := range []string{"control-restart", "activity-loss", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			c, err := client.Dial(client.Options{DataConverter: dataConverter(stateRoot())})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			id := fmt.Sprintf("%s-%d", mode, time.Now().UnixMilli())
			input, err := prepareInput(stateRoot(), id, "Continue the conversation across activities.")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
			defer cancel()
			run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, ContinuityWorkflow, input)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("workflow %s", id)
			target := "ContinuityGenerate2"
			if mode == "control-restart" {
				target = "ContinuityGenerate1"
			}
			waitActivity(t, ctx, c, id, target, false)
			started := time.Now()
			switch mode {
			case "control-restart":
				if b, err := exec.CommandContext(ctx, "docker", "stop", "--time", "3", "gimbal-instrumented-control").CombinedOutput(); err != nil {
					t.Fatalf("stop: %s %v", b, err)
				}
				defer func() { _ = exec.Command("docker", "start", "gimbal-instrumented-control").Run() }()
				waitActivity(t, ctx, c, id, "ContinuityGenerate1", true)
				// That result is durable while no workflow worker exists to schedule Edit.
				if b, err := exec.CommandContext(ctx, "docker", "start", "gimbal-instrumented-control").CombinedOutput(); err != nil {
					t.Fatalf("restart: %s %v", b, err)
				}
			case "activity-loss":
				if b, err := exec.CommandContext(ctx, "docker", "kill", environmentID(id)).CombinedOutput(); err != nil {
					t.Fatalf("kill: %s %v", b, err)
				}
			case "cancel":
				if err = c.CancelWorkflow(ctx, id, run.GetRunID()); err != nil {
					t.Fatal(err)
				}
			}
			err = run.Get(ctx, nil)
			if (err != nil) != (mode != "control-restart") {
				t.Fatalf("outcome %v", err)
			}
			h, herr := recordedHistory(ctx, c, id)
			if herr != nil {
				t.Fatal(herr)
			}
			remembers := 0
			for _, event := range h.Events {
				if a := event.GetActivityTaskScheduledEventAttributes(); a != nil && a.ActivityType.Name == "ContinuityGenerate1" {
					remembers++
				}
			}
			if remembers != 1 {
				t.Fatalf("completed operation repeated %d", remembers)
			}
			if mode == "activity-loss" && time.Since(started) > 90*time.Second {
				t.Fatal("worker loss exceeded failure bound")
			}
			project := filepath.Join(stateRoot(), environmentID(id), "workspace", ".gimbal")
			if mode == "control-restart" {
				assertContinuityRecords(t, project)
			}
			runs, _ := filepath.Glob(filepath.Join(project, "runs", "*"))
			if len(runs) != 1 {
				t.Fatal(runs)
			}
			snapshot, serr := observation.NewRegistry(project).Snapshot(filepath.Base(runs[0]))
			if serr != nil {
				t.Fatal(serr)
			}
			if mode == "activity-loss" {
				if snapshot.Run.Status != observation.StatusFailed || !strings.Contains(snapshot.Run.Error, "cleanup is unconfirmed") {
					t.Fatalf("untruthful lost-worker record: %+v", snapshot.Run)
				}
				if snapshot.Scopes[""].Ended != 0 {
					t.Fatal("dead worker falsely claimed root cleanup")
				}
			}
			if mode == "cancel" && snapshot.Run.Status != observation.StatusCancelled {
				t.Fatalf("cancellation has wrong status: %s", snapshot.Run.Status)
			}
			replayRecorded(t, c, id, stateRoot())
			t.Logf("%s settled in %s; Gimbal status=%s; one Remember; history replay passed; result error=%v", mode, time.Since(started).Round(time.Millisecond), snapshot.Run.Status, err)
		})
	}
}
