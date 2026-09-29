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
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

func TestPlannerExpansionFeedbackAndValidation(t *testing.T) {
	for _, mode := range []string{"success", "invalid", "failed"} {
		t.Run(mode, func(t *testing.T) {
			a := newTestActivities(t)
			decisions := 0
			adapter := &specimenAdapter{turn: func(ctx context.Context, id, prompt string) (any, error) {
				if strings.Contains(prompt, "You plan the loop") {
					decisions++
					if mode == "failed" {
						return nil, fmt.Errorf("planner unavailable")
					}
					if mode == "invalid" {
						i := 8
						return planValue{Next: &i}, nil
					}
					if decisions > 1 && (!strings.Contains(prompt, "Previous task record") || !strings.Contains(prompt, "exit_code") || !strings.Contains(prompt, "done")) {
						t.Error("planner lost feedback")
					}
					if decisions == 3 {
						return planValue{Tasks: []gimbal.Task{}}, nil
					}
					i := 0
					return planValue{Tasks: []gimbal.Task{{Name: fmt.Sprintf("task-%d", decisions), Description: "Write planned.txt containing done", DefinitionOfDone: "file contains done"}}, Next: &i}, nil
				}
				if err := os.WriteFile(filepath.Join(a.workdir, "planned.txt"), []byte("done\n"), 0644); err != nil {
					return nil, err
				}
				return Report{File: "planned.txt", Receipt: "done"}, nil
			}}
			a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "test"}}
			var suite testsuite.WorkflowTestSuite
			e := suite.NewTestWorkflowEnvironment()
			e.RegisterActivity(a)
			e.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: "test"}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
			e.RegisterActivityWithOptions(func(context.Context) error {
				if len(a.scopes) != 0 {
					t.Error("leaked scopes")
				}
				return nil
			}, activity.RegisterOptions{Name: "ReleaseEnvironment"})
			e.ExecuteWorkflow(PlanningWorkflow, Input{})
			if (e.GetWorkflowError() != nil) != (mode != "success") {
				t.Fatal(e.GetWorkflowError())
			}
			if mode == "success" {
				plans, tasks := assertPlanningRecords(t, a.project.Dir())
				if plans != 3 || tasks != 2 || decisions != 3 {
					t.Fatalf("decisions=%d recorded=%d tasks=%d", decisions, plans, tasks)
				}
			}
		})
	}
}
func TestLivePlanning(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_WORKFLOW") != "1" {
		t.Skip("requires live workers and paid Haiku access")
	}
	c, err := client.Dial(client.Options{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("planning-%d", time.Now().UnixMilli())
	input, err := prepareInput(stateRoot(), id, "One small planned file.")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, PlanningWorkflow, input)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workflow %s", id)
	if err = run.Get(ctx, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(stateRoot(), environmentID(id), "workspace", "planned.txt"))
	if err != nil || strings.TrimSpace(string(b)) != "done" {
		t.Fatalf("%q %v", b, err)
	}
	plans, tasks := assertPlanningRecords(t, filepath.Join(stateRoot(), environmentID(id), "workspace", ".gimbal"))
	t.Logf("separate planner decisions %d, completed tasks %d; recorded command feedback retained", plans, tasks)
}

func TestCheckRecordsCancelledCommandBeforeReturning(t *testing.T) {
	a := testActivities(t, "cancelled-check")
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestActivityEnvironment()
	e.RegisterActivityWithOptions(func(ctx context.Context) (checkResult, error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		timer := time.AfterFunc(30*time.Millisecond, cancel)
		defer timer.Stop()
		return a.check(ctx, operationInput{}, "check", "sh", "-c", "sleep 10")
	}, activity.RegisterOptions{Name: "cancel-check"})
	encoded, err := e.ExecuteActivity("cancel-check")
	if err != nil {
		t.Fatal(err)
	}
	var result checkResult
	if err = encoded.Get(&result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Kind != "cancelled" {
		t.Fatalf("%+v", result)
	}
	entries, err := a.store.Load(t.Context(), result.Context)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != "check" {
		t.Fatal(entries)
	}
	raw, err := a.store.Value(t.Context(), entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "context canceled") {
		t.Fatalf("cancellation evidence lost: %s", raw)
	}
}
func TestCompiledLoopQueuesAndDropsSteering(t *testing.T) {
	a := newTestActivities(t)
	var prompts []string
	adapter := &specimenAdapter{turn: func(_ context.Context, _, prompt string) (any, error) {
		prompts = append(prompts, prompt)
		return planValue{Tasks: []gimbal.Task{}}, nil
	}}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "test"}}
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	if _, err := e.ExecuteActivity(a.Initialize, Data{Name: "planning"}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Finish(context.Background(), "") }()
	encoded, err := e.ExecuteActivity(a.NewParent)
	if err != nil {
		t.Fatal(err)
	}
	var planner sessionHandle
	if err = encoded.Get(&planner); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ExecuteActivity(a.EnterLoopScope, ScopeInput{"plan.1", "", "plan"}); err != nil {
		t.Fatal(err)
	}
	paths, _ := filepath.Glob(filepath.Join(a.project.Dir(), "runs", "*"))
	run := filepath.Base(paths[0])
	if err = a.project.SteerLoop(run, "plan.1", gimbal.WrapUp); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ExecuteActivity(a.PlanningPlan1, operationInput{Scope: "plan.1", Session: planner}, "plan", "test goal", []gimbal.Task{}, ""); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], gimbal.WrapUp) {
		t.Fatal("queued steer did not enter planner prompt")
	}
	if err = a.project.SteerLoop(run, "plan.1", "late message"); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ExecuteActivity(a.ExitScope, "plan.1", ""); err != nil {
		t.Fatal(err)
	}
	if err = a.project.SteerLoop(run, "plan.1", "too late"); err == nil {
		t.Fatal("ended loop accepted steering")
	}
	raw, err := os.ReadFile(filepath.Join(paths[0], "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var foundLoop, landed, dropped bool
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var r struct {
			Event struct {
				Kind, Message string
				Loop, Landed  bool
			}
		}
		if err = json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		if r.Event.Kind == "scope_began" && r.Event.Loop {
			foundLoop = true
		}
		if r.Event.Kind == "steer" && r.Event.Message == gimbal.WrapUp && r.Event.Landed {
			landed = true
		}
		if r.Event.Kind == "steer" && r.Event.Message == "late message" && !r.Event.Landed {
			dropped = true
		}
	}
	if !foundLoop || !landed || !dropped {
		t.Fatalf("loop=%v landed=%v dropped=%v", foundLoop, landed, dropped)
	}
}
