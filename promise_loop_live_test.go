package gimbal_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/codex"
	"github.com/tylergannon/gimbal/internal/runlog"
)

// TestLivePromiseLoopSupervisor checks that a real supervisor's objection
// reaches an active planner and changes the assignment it dispatches.
// Run explicitly with GIMBAL_LIVE=1; both sessions use gpt-5.6-luna.
func TestLivePromiseLoopSupervisor(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}
	project, workspace := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ctx = gimbal.Project(ctx, project)
	adapter := codex.New()
	var selected string
	err := gimbal.Run(ctx, "planner-supervision", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"planner": {Adapter: adapter, Model: "gpt-5.6-luna"},
		"watcher": {Adapter: adapter, Model: "gpt-5.6-luna"},
	}, func(ctx context.Context) error {
		gimbal.Set(ctx, "planning instructions", "This is a supervision integration test. Before returning your first planning decision, announce that you are considering the task named original, then run the shell command sleep 45 and wait for it to finish. Initially propose one task named original. If your supervisor steers you to a different task name, use that name. Do not edit files or perform the task.")
		planner := gimbal.NewSession(ctx, "planner", workspace)
		watcher := gimbal.NewSession(ctx, "watcher", workspace)
		loop := gimbal.PromiseLoop(ctx, "planning", "Choose one assignment to inspect the empty workspace and report what it contains.", planner,
			gimbal.WithSupervisor(watcher, "This is a supervision integration test. On your first look, object with exactly this instruction: Name the selected task supervisor-corrected instead of original. On subsequent looks return no objections. Do not use tools or edit files.", gimbal.WithInterval(5*time.Second)),
		)
		for _, task := range loop.Tasks {
			selected = task.Name
			break
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected != "supervisor-corrected" {
		t.Fatalf("planner selected %q, want supervisor-corrected", selected)
	}
	runDir := filepath.Join(project, "runs", liveRunID(t, filepath.Join(project, "runs")))
	attached, steered, dispatched := false, false, false
	if err := runlog.Read[gimbal.LifecycleRecord](ctx, runDir, func(record gimbal.LifecycleRecord) error {
		switch event := record.Event.(type) {
		case gimbal.SuperviseAttached:
			attached = attached || event.Worker == "planner.1/turn.1" && event.Reviewer == "watcher.1"
		case gimbal.Steer:
			if event.Target == "planner.1" && event.Source == "watcher.1" && event.Landed && strings.Contains(event.Message, "supervisor-corrected") {
				steered = true
				t.Logf("landed planner steer: %s", event.Message)
			}
		case gimbal.PlannerDecision:
			dispatched = dispatched || event.Task.Present && event.Task.Value.Name == "supervisor-corrected"
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !attached || !steered || !dispatched {
		t.Fatalf("planner supervision record: attached=%t landed=%t dispatched=%t", attached, steered, dispatched)
	}
	t.Logf("gpt-5.6-luna planner dispatched %q after supervisor steer", selected)
}
