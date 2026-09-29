package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/experiments/instrumented/planning"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	graph "github.com/tylergannon/gimbal/workflow"
	"go.temporal.io/sdk/testsuite"
)

func newTestActivities(t *testing.T) *Activities {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := t.TempDir()
	a := &Activities{workdir: dir, store: compiledscope.Store{Root: t.TempDir()}, controls: host.CompiledControls{CancelRun: func(context.Context, gimbal.Killed) error { return nil }}}
	a.store.LocalDir = filepath.Join(a.store.Root, "materialized")
	ctx, err := host.WithContextStore(ctx, dir, &a.store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owner := host.New(ctx, t.TempDir())
	t.Cleanup(owner.Close)
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.project = project

	return a
}
func testActivities(t *testing.T, name string) *Activities {
	a := newTestActivities(t)
	registerTestGraph(name)
	var s testsuite.WorkflowTestSuite
	e := s.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	if _, err := e.ExecuteActivity(a.Initialize, Data{Name: name}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Finish(t.Context(), ""); err != nil {
			t.Error(err)
		}
	})
	return a
}

// Observe source-authored results from the durable turn records rather than
// adding observation-only return fields to a generated workflow's contract.
type recordedTurn struct {
	prompt, result, err string
	completed           bool
}

func recordedTurns(t *testing.T, project string) []recordedTurn {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("run records: %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var turns []recordedTurn
	indexes := map[string]int{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var record struct {
			Scope, Session, Turn string
			Event                struct{ Kind, Prompt, Result, Error string }
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		key := record.Scope + "/" + record.Session + "/" + record.Turn
		switch record.Event.Kind {
		case "turn_started":
			indexes[key] = len(turns)
			turns = append(turns, recordedTurn{prompt: record.Event.Prompt})
		case "turn_ended":
			index, ok := indexes[key]
			if !ok {
				t.Fatal("result without started turn")
			}
			turns[index].result = record.Event.Result
			turns[index].err = record.Event.Error
			turns[index].completed = true
		}
	}
	return turns
}
func assertContinuityRecords(t *testing.T, project string) {
	t.Helper()
	var turns []recordedTurn
	for _, turn := range recordedTurns(t, project) {
		if turn.completed && turn.err == "" {
			turns = append(turns, turn)
		}
	}
	if len(turns) != 4 {
		t.Fatalf("continuity turns=%d want 4", len(turns))
	}
	reports := make([]Report, len(turns))
	for i, turn := range turns {
		if err := json.Unmarshal([]byte(turn.result), &reports[i]); err != nil {
			t.Fatal(err)
		}
	}
	if reports[0].Summary != "edit" || reports[1].Summary != "child" || reports[1].Receipt != "amber-17" || reports[2].Receipt != "amber-17" || reports[2].Summary != "violet-29" || reports[3].Receipt != "amber-17" || reports[3].Summary != "parent" {
		t.Fatalf("continuation results: %+v", reports)
	}
}
func assertPlanningRecords(t *testing.T, project string) (decisions, tasks int) {
	t.Helper()
	var finalPrompt string
	for _, turn := range recordedTurns(t, project) {
		if !turn.completed || turn.err != "" {
			continue
		}
		if strings.Contains(turn.prompt, "You plan the loop") {
			decisions++
			finalPrompt = turn.prompt
			continue
		}
		if strings.Contains(turn.prompt, planning.WorkPrompt) {
			tasks++
		}
	}
	if decisions < 2 || tasks < 1 || !strings.Contains(finalPrompt, "Previous task record") || !strings.Contains(finalPrompt, "exit_code") {
		t.Fatalf("planning decisions=%d tasks=%d; command feedback missing=%v", decisions, tasks, !strings.Contains(finalPrompt, "exit_code"))
	}
	return
}

func TestRecordedTurnsRetainValidationAttempts(t *testing.T) {
	a := newTestActivities(t)
	attempts := 0
	adapter := &specimenAdapter{turn: func(context.Context, string, string) (any, error) {
		attempts++
		if attempts == 1 {
			return map[string]any{"summary": 123}, nil
		}
		return Report{Summary: "valid", File: "report.txt", Receipt: "done"}, nil
	}}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "deterministic"}}
	err := a.project.Run(t.Context(), "record-retry", a.models, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, coder, a.workdir)
		_, err := session.Generate[Report](ctx, "Return a valid report.")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	turns := recordedTurns(t, a.project.Dir())
	if len(turns) != 2 || !turns[0].completed || turns[0].err == "" || !turns[1].completed || turns[1].err != "" {
		t.Fatalf("attempt records: %+v", turns)
	}
	var result Report
	if err := json.Unmarshal([]byte(turns[1].result), &result); err != nil || result.Summary != "valid" {
		t.Fatalf("validated result %+v: %v", result, err)
	}
}

// Synthetic workflows used by operation tests still satisfy hosted entry's
// graph-registration contract. Real specimen graph registrations come from
// their authored packages and are not replaced here.
func registerTestGraph(name string) {
	if _, ok := gimbal.RegisteredGraph(name); !ok {
		gimbal.RegisterGraph(graph.Graph{Name: name})
	}
}
