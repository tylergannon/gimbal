package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/experiments/instrumented/planning"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/host"
	"go.temporal.io/sdk/testsuite"
)

func newTestActivities(t *testing.T) *Activities {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	owner := host.New(ctx, t.TempDir())
	t.Cleanup(owner.Close)
	dir := t.TempDir()
	project, err := owner.AdmitProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Activities{project: project, workdir: dir, store: compiledscope.Store{Root: t.TempDir()}}

	return a
}
func testActivities(t *testing.T, name string) *Activities {
	a := newTestActivities(t)
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
type recordedTurn struct{ prompt, result string }

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
			Event                struct{ Kind, Prompt, Result string }
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
		}
	}
	return turns
}
func assertContinuityRecords(t *testing.T, project string) {
	t.Helper()
	turns := recordedTurns(t, project)
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
