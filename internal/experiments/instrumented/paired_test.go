package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/planning"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// The source uses Project.Run and the actual authored function. The target uses
// Temporal's scheduler and real activities; only environment provisioning and
// the external agent are substituted. Commands and files are real in both.
type pairedAdapter struct {
	specimenAdapter
	tokens map[string]string
	trace  []string
}

func (a *pairedAdapter) Fork(ctx context.Context, parent string) (string, error) {
	id, err := a.specimenAdapter.Fork(ctx, parent)
	a.tokens[id] = a.tokens[parent]
	a.trace = append(a.trace, "fork:"+parent+":"+id)
	return id, err
}

func (a *pairedAdapter) Close(ctx context.Context, id string) error {
	a.trace = append(a.trace, "close:"+id)
	return a.specimenAdapter.Close(ctx, id)
}

type pairedRun struct {
	a          *Activities
	adapter    *pairedAdapter
	err        error
	activities []string
}

func runPairSide(t *testing.T, name, mode string, target bool) pairedRun {
	t.Helper()
	a := newTestActivities(t)
	adapter := &pairedAdapter{tokens: map[string]string{}}
	decisions, workers := 0, 0
	variation := os.Getenv("GIMBAL_GENERATION_CASE")
	resultKey := "implementation"
	if variation == "rename" {
		resultKey = "delivery"
	}
	adapter.turn = func(_ context.Context, id, prompt string) (any, error) {
		var operation string
		switch {
		case strings.Contains(prompt, "EXTRA SOURCE TURN"):
			operation = "extra"
		case strings.Contains(prompt, continuity.RememberPrompt):
			operation = "remember"
		case strings.Contains(prompt, continuity.EditPrompt):
			operation = "edit"
		case strings.Contains(prompt, continuity.ForkPrompt):
			operation = "diverge"
		case strings.Contains(prompt, continuity.ResumePrompt):
			operation = "resume"
		case strings.Contains(prompt, "You plan the loop"):
			operation = "plan"
		case strings.Contains(prompt, planning.WorkPrompt):
			operation = "work"
		default:
			return nil, fmt.Errorf("unexpected prompt: %s", prompt)
		}
		adapter.trace = append(adapter.trace, operation+":"+id)
		if operation == "plan" && mode == "planner-failure" {
			return nil, errors.New("planner unavailable")
		}
		if operation == "remember" || operation == "work" {
			switch mode {
			case "failure":
				return nil, errors.New("agent unavailable")
			case "cancelled":
				return nil, context.Canceled
			case "deadline":
				return nil, context.DeadlineExceeded
			}
		}
		switch operation {
		case "extra":
			return Report{Summary: "extra", Receipt: "done"}, nil
		case "remember":
			adapter.tokens[id] = "amber-17"
			decision := "edit"
			if mode == "skip" {
				decision = "skip"
			}
			return Report{Summary: decision, File: "continuity.txt", Receipt: "amber-17"}, nil
		case "edit":
			if !strings.Contains(prompt, "## layer\n\nchild") {
				t.Error("child context missing")
			}
			if err := os.WriteFile(filepath.Join(a.workdir, "continuity.txt"), []byte(adapter.tokens[id]), 0644); err != nil {
				return nil, err
			}
			return Report{Summary: "child", File: "continuity.txt", Receipt: adapter.tokens[id]}, nil
		case "diverge":
			inherited := adapter.tokens[id]
			adapter.tokens[id] = "violet-29"
			if !strings.Contains(prompt, `"summary": "child"`) {
				t.Error("fork lost typed child result")
			}
			return Report{Summary: "violet-29", File: "continuity.txt", Receipt: inherited}, nil
		case "resume":
			if !strings.Contains(prompt, "## layer\n\nparent") || strings.Contains(prompt, "## result") {
				t.Error("child context leaked into parent")
			}
			if mode != "skip" {
				b, err := os.ReadFile(filepath.Join(a.workdir, "continuity.txt"))
				if err != nil || string(b) != "amber-17" {
					t.Errorf("child file unavailable: %q %v", b, err)
				}
				if !slices.Equal(adapter.closed, []string{"2"}) {
					t.Errorf("fork not closed before resume: %v", adapter.closed)
				}
			}
			b, err := os.ReadFile(filepath.Join(a.workdir, "recovery.txt"))
			if err != nil || string(b) != "recovered" {
				t.Errorf("recovery missing: %q %v", b, err)
			}
			if !strings.Contains(prompt, `"exit_code": 7`) || !strings.Contains(prompt, `"stdout": "observed"`) || !strings.Contains(prompt, `"stderr": "diagnostic"`) {
				t.Error("diagnostic fields lost")
			}
			return Report{Summary: "parent", File: "continuity.txt", Receipt: adapter.tokens[id]}, nil
		case "plan":
			decisions++
			_, scoped, _ := strings.Cut(prompt, "Scoped context:\n\n")
			scoped, _, _ = strings.Cut(scoped, "Previous task record:")
			scoped, _, _ = strings.Cut(scoped, "Backlog now:")
			if !strings.Contains(scoped, "## review\n\nparent") || strings.Contains(scoped, "## "+resultKey) || strings.Contains(scoped, "## receipt") || strings.Contains(scoped, "## task") {
				t.Error("task locals promoted to planner current context")
			}
			if decisions > 1 {
				_, feedback, ok := strings.Cut(prompt, "Previous task record")
				if !ok || !strings.Contains(feedback, "## "+resultKey) || strings.Contains(feedback, "## result") || !strings.Contains(feedback, "## receipt\n\nrecorded:done") {
					t.Error("renamed/additional feedback lost")
				}
				if variation == "rename" && strings.Contains(feedback, "## implementation") {
					t.Error("obsolete feedback key retained")
				}
				if variation == "additional" && !strings.Contains(feedback, "## proof_note\n\nadditional evidence") {
					t.Error("added local write missing")
				}
				if variation == "command" && !strings.Contains(feedback, `"stdout": "source-check"`) {
					t.Error("changed command evidence missing")
				}
				shadowed := decisions == 2
				if variation == "conditional" {
					shadowed = decisions == 3
				}
				if shadowed && !strings.Contains(feedback, "## review\n\ntask") {
					t.Error("executed shadow missing")
				}
				if !shadowed && strings.Contains(feedback, "## review") {
					t.Error("unwritten inherited value presented as task-local")
				}
				if !strings.Contains(prompt, "Previous task record") || !strings.Contains(prompt, `"exit_code": 0`) || !strings.Contains(prompt, `"receipt": "done"`) {
					t.Error("planner lost completed task evidence")
				}
			}
			if mode == "stop" || decisions == 3 {
				return planValue{Tasks: []gimbal.Task{}}, nil
			}
			i := 0
			return planValue{Tasks: []gimbal.Task{{Name: fmt.Sprintf("task-%d", decisions), Description: "Write planned.txt containing done", DefinitionOfDone: "file contains done"}}, Next: &i}, nil
		case "work":
			if variation == "prompt" && !strings.Contains(prompt, "Changed by source.") {
				t.Error("changed prompt missing")
			}
			workers++
			if !strings.Contains(prompt, fmt.Sprintf("task-%d", workers)) {
				t.Error("wrong selected assignment")
			}
			if err := os.WriteFile(filepath.Join(a.workdir, "planned.txt"), []byte("done\n"), 0644); err != nil {
				return nil, err
			}
			summary := "done"
			if workers == 1 {
				summary = "reviewed"
			}
			return Report{Summary: summary, File: "planned.txt", Receipt: "done"}, nil
		}
		panic("unreachable")
	}
	if mode == "cleanup-failure" {
		adapter.closeErr = errors.New("close failed")
	}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "deterministic"}}
	r := pairedRun{a: a, adapter: adapter}
	if !target {
		body := continuity.Continuity
		if name == "planning" {
			body = planning.Planning
		}
		r.err = a.project.Run(t.Context(), name, a.models, func(ctx context.Context) error { return body(ctx, gimbal.Env{WorkDir: a.workdir}) })
		return r
	}
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestWorkflowEnvironment()
	e.RegisterActivity(a)
	e.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: "test"}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
	e.RegisterActivityWithOptions(func(context.Context) error {
		if len(a.scopes) != 0 {
			t.Error("target released environment before closing scopes")
		}
		return nil
	}, activity.RegisterOptions{Name: "ReleaseEnvironment"})
	e.SetOnActivityCompletedListener(func(info *activity.Info, encoded converter.EncodedValue, activityErr error) {
		if info.ActivityType.Name != "ContinuityGenerate1" && info.ActivityType.Name != "PlanningGenerate1" {
			return
		}
		if activityErr != nil {
			t.Errorf("agent failure escaped operation result: %v", activityErr)
			return
		}
		var result generateResult
		if err := encoded.Get(&result); err != nil {
			t.Error(err)
			return
		}
		wantKind := map[string]string{"failure": "execution", "cancelled": "cancelled", "deadline": "deadline"}[mode]
		gotKind := ""
		if result.Failure != nil {
			gotKind = result.Failure.Kind
		}
		if gotKind != wantKind {
			t.Errorf("operation failure kind %q want %q", gotKind, wantKind)
		}
		if errors.Is(result.Err(), context.Canceled) != (mode == "cancelled") || errors.Is(result.Err(), context.DeadlineExceeded) != (mode == "deadline") {
			t.Errorf("operation failure classification changed: %+v", result.Failure)
		}
	})
	e.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		r.activities = append(r.activities, info.ActivityType.Name)
	})
	if name == "continuity" {
		e.ExecuteWorkflow(ContinuityWorkflow, Input{})
	} else {
		e.ExecuteWorkflow(PlanningWorkflow, Input{})
	}
	r.err = e.GetWorkflowError()
	return r
}

func TestPairedWorkflows(t *testing.T) {
	for _, name := range []string{"continuity", "planning"} {
		modes := []string{"success", "failure", "cancelled", "deadline", "cleanup-failure"}
		if name == "continuity" {
			modes = append(modes, "skip")
		} else {
			modes = append(modes, "stop", "planner-failure")
		}
		for _, mode := range modes {
			t.Run(name+"/"+mode, func(t *testing.T) {
				source := runPairSide(t, name, mode, false)
				target := runPairSide(t, name, mode, true)
				wantErr := mode == "failure" || mode == "cancelled" || mode == "deadline" || mode == "cleanup-failure" || mode == "planner-failure"
				if (source.err != nil) != wantErr || (target.err != nil) != wantErr {
					t.Fatalf("source=%v target=%v", source.err, target.err)
				}
				if !slices.Equal(source.adapter.trace, target.adapter.trace) {
					t.Errorf("operation/resource trace differs:\nsource %v\ntarget %v", source.adapter.trace, target.adapter.trace)
				}
				if !slices.Equal(normalizedPrompts(source), normalizedPrompts(target)) {
					t.Errorf("prompts differ:\nsource %q\ntarget %q", normalizedPrompts(source), normalizedPrompts(target))
				}
				assertPairedOperations(t, name, mode, target)
				if mode == "success" {
					for _, r := range []pairedRun{source, target} {
						if name == "continuity" {
							assertContinuityRecords(t, r.a.project.Dir())
						} else {
							plans, tasks := assertPlanningRecords(t, r.a.project.Dir())
							if plans != 3 || tasks != 2 {
								t.Fatalf("recorded plans=%d tasks=%d", plans, tasks)
							}
						}
					}
				}
				if errors.Is(source.err, context.Canceled) != (mode == "cancelled") || errors.Is(source.err, context.DeadlineExceeded) != (mode == "deadline") {
					t.Errorf("source error classification: %v", source.err)
				}
				left, right := pairedEvents(t, source), pairedEvents(t, target)
				if !slices.Equal(left, right) {
					t.Errorf("semantic events differ:\nsource %s\ntarget %s", strings.Join(left, "\n"), strings.Join(right, "\n"))
				}
			})
		}
	}
}

func normalizedPrompts(r pairedRun) []string {
	out := make([]string, len(r.adapter.prompts))
	for i, prompt := range r.adapter.prompts {
		out[i] = strings.ReplaceAll(prompt, r.a.workdir, "WORKDIR")
	}
	return out
}

func assertPairedOperations(t *testing.T, name, mode string, r pairedRun) {
	t.Helper()
	want := []string{"ContinuityGenerate1", "ContinuityGenerate2", "ContinuityGenerate3", "ContinuityRunCommand1", "ContinuityRunCommand2", "ContinuityGenerate4"}
	trace := []string{"remember:1", "edit:1", "fork:1:2", "diverge:2", "close:2", "resume:1", "close:1"}
	if mode == "skip" {
		want = []string{"ContinuityGenerate1", "ContinuityRunCommand1", "ContinuityRunCommand2", "ContinuityGenerate4"}
		trace = []string{"remember:1", "resume:1", "close:1"}
	}
	if name == "planning" {
		want = []string{"PlanningPlan1", "PlanningGenerate1", "PlanningCheck1", "PlanningPlan1", "PlanningGenerate1", "PlanningCheck1", "PlanningPlan1"}
		trace = []string{"plan:1", "work:2", "close:2", "plan:1", "work:3", "close:3", "plan:1", "close:1"}
		if mode == "stop" || mode == "planner-failure" {
			want = []string{"PlanningPlan1"}
			trace = []string{"plan:1", "close:1"}
		}
	}
	if mode == "failure" || mode == "cancelled" || mode == "deadline" {
		want = []string{"ContinuityGenerate1"}
		trace = []string{"remember:1", "close:1"}
		if name == "planning" {
			want = []string{"PlanningPlan1", "PlanningGenerate1"}
			trace = []string{"plan:1", "work:2", "close:2", "close:1"}
		}
	}
	if name == "planning" && os.Getenv("GIMBAL_GENERATION_CASE") == "generate" && (mode == "success" || mode == "cleanup-failure") {
		want = []string{"PlanningPlan1", "PlanningGenerate1", "PlanningCheck1", "PlanningGenerate2", "PlanningPlan1", "PlanningGenerate1", "PlanningCheck1", "PlanningGenerate2", "PlanningPlan1"}
		trace = []string{"plan:1", "work:2", "extra:2", "close:2", "plan:1", "work:3", "extra:3", "close:3", "plan:1", "close:1"}
	}
	var operations []string
	for _, a := range r.activities {
		if slices.Contains([]string{"ContinuityGenerate1", "ContinuityGenerate2", "ContinuityGenerate3", "ContinuityRunCommand1", "ContinuityRunCommand2", "ContinuityGenerate4", "PlanningPlan1", "PlanningGenerate1", "PlanningGenerate2", "PlanningCheck1"}, a) {
			operations = append(operations, a)
		}
	}
	if !slices.Equal(want, operations) {
		t.Errorf("orchestrated operations %v want %v", operations, want)
	}
	if !slices.Equal(trace, r.adapter.trace) {
		t.Errorf("resource trace %v want %v", r.adapter.trace, trace)
	}
}

func TestPairedCheckFailureRecording(t *testing.T) {
	for _, mode := range []string{"missing", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			var records []string
			for _, target := range []bool{false, true} {
				a := newTestActivities(t)
				command, args := filepath.Join(a.workdir, "does-not-exist"), []string{}
				if mode == "cancelled" {
					command, args = "sh", []string{"-c", "sleep 10"}
				}
				var commandErr error
				if !target {
					commandErr = a.project.Run(t.Context(), "check-pair", nil, func(ctx context.Context) error {
						ctx, cancel := context.WithCancel(ctx)
						defer cancel()
						if mode == "cancelled" {
							timer := time.AfterFunc(30*time.Millisecond, cancel)
							defer timer.Stop()
						}
						return gimbal.Check(ctx, "check", a.workdir, command, args...)
					})
				} else {
					var suite testsuite.WorkflowTestSuite
					e := suite.NewTestActivityEnvironment()
					e.RegisterActivity(a)
					if _, err := e.ExecuteActivity(a.Initialize, Data{Name: "check-pair"}); err != nil {
						t.Fatal(err)
					}
					e.RegisterActivityWithOptions(func(ctx context.Context) (checkResult, error) {
						ctx, cancel := context.WithCancel(ctx)
						defer cancel()
						if mode == "cancelled" {
							timer := time.AfterFunc(30*time.Millisecond, cancel)
							defer timer.Stop()
						}
						return a.check(ctx, operationInput{}, "check", command, args...)
					}, activity.RegisterOptions{Name: "pairedCheck"})
					encoded, err := e.ExecuteActivity("pairedCheck")
					if err != nil {
						t.Fatal(err)
					}
					var out checkResult
					if err := encoded.Get(&out); err != nil {
						t.Fatal(err)
					}
					commandErr = out.Err()
					entries, err := a.store.Load(out.Context)
					if err != nil || len(entries) != 1 || entries[0].Key != "check" {
						t.Fatalf("record missing at activity return: %v %v", entries, err)
					}
					if err := a.Finish(t.Context(), errorText(commandErr)); err != nil {
						t.Fatal(err)
					}
				}
				if commandErr == nil || errors.Is(commandErr, context.Canceled) != (mode == "cancelled") {
					t.Fatalf("check failure: %v", commandErr)
				}
				paths, err := filepath.Glob(filepath.Join(a.project.Dir(), "runs", "*"))
				if err != nil || len(paths) != 1 {
					t.Fatal(paths, err)
				}
				snapshot, err := a.project.Registry().Snapshot(filepath.Base(paths[0]))
				if err != nil {
					t.Fatal(err)
				}
				raw := snapshot.Scopes[""].Values["check"].Value
				var record checkRecord
				if err := json.Unmarshal(raw, &record); err != nil {
					t.Fatal(err)
				}
				if record.Error == "" || record.Command != command || record.Workdir != a.workdir {
					t.Fatalf("incomplete Check record: %+v", record)
				}
				records = append(records, strings.ReplaceAll(string(raw), a.workdir, "WORKDIR"))
			}
			if records[0] != records[1] {
				t.Errorf("implicit Check records differ:\n%s\n%s", records[0], records[1])
			}
		})
	}
}

func TestInfrastructureFailureStopsBeforeAuthoredOperations(t *testing.T) {
	for _, name := range []string{"continuity", "planning"} {
		t.Run(name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			e := suite.NewTestWorkflowEnvironment()
			var scheduled []string
			e.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
				scheduled = append(scheduled, info.ActivityType.Name)
			})
			e.RegisterActivityWithOptions(func(context.Context) (Environment, error) {
				return Environment{}, errors.New("environment unavailable")
			}, activity.RegisterOptions{Name: "ProvisionEnvironment"})
			e.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: "ReleaseEnvironment"})
			e.ExecuteWorkflow(func(ctx workflow.Context) error {
				var err error
				if name == "continuity" {
					err = ContinuityWorkflow(ctx, Input{})
				} else {
					err = PlanningWorkflow(ctx, Input{})
				}
				// Test the authored caller's error boundary, before Temporal
				// serializes the final joined error for an external client.
				var failure *temporal.ActivityError
				if !errors.As(err, &failure) || failure.ActivityType().Name != "ProvisionEnvironment" {
					t.Errorf("infrastructure error lost at workflow boundary: %v", err)
				}
				return err
			})
			if err := e.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "environment unavailable") {
				t.Fatalf("infrastructure failure missing: %v", err)
			}
			if !slices.Equal(scheduled, []string{"ProvisionEnvironment", "ReleaseEnvironment"}) {
				t.Fatalf("work scheduled after failed provisioning: %v", scheduled)
			}
		})
	}
}

// Compare semantic event fields and their ordering, retaining scope/session/turn
// associations. Omit clocks, duration and usage; normalize only workspace paths.
func pairedEvents(t *testing.T, r pairedRun) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(r.a.project.Dir(), "runs", "*", "run.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var record map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		delete(record, "seq")
		delete(record, "time")
		var event map[string]json.RawMessage
		if err := json.Unmarshal(record["event"], &event); err != nil {
			t.Fatal(err)
		}
		delete(event, "duration")
		delete(event, "usage")
		record["event"], err = json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, strings.ReplaceAll(string(b), r.a.workdir, "WORKDIR"))
	}
	return out
}
