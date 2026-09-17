package gimble

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/runlog"
)

const plannerRole = "planner"

// loopFake answers a planner's dispatch: the first decision takes on the
// one task, and the next ends dispatch. Every prompt is kept so a test can
// read what the planner was told.
func loopFake(prompts *[]string) *fake {
	plans := 0
	return &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		*prompts = append(*prompts, prompt)
		plans++
		task := map[string]any{
			"name": "do the thing", "description": "Do it.", "definition_of_done": "It is done.",
			"validation": map[string]any{"command": "", "query": ""},
		}
		p := map[string]any{"tasks": []any{task}}
		if plans == 1 {
			p["next"] = 0
		} else {
			p["next"] = nil
		}
		raw, err := json.Marshal(p)
		return string(raw), err
	}}
}

// steers reads every Steer record of the run in dir.
func steers(t *testing.T, dir string) []Steer {
	t.Helper()
	var found []Steer
	if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(record LifecycleRecord) error {
		if steer, ok := record.Event.(Steer); ok {
			found = append(found, steer)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return found
}

// TestAMessageToALoopReachesThePlannerAtItsNextDecision: an operator's
// message to a loop waits for the next planning turn instead of being
// dropped, because a planner is not always in one. Here it is sent while a
// task runs and no turn is running at all.
func TestAMessageToALoopReachesThePlannerAtItsNextDecision(t *testing.T) {
	var prompts []string
	var dir string
	err := runTest(t, bind(loopFake(&prompts), "model", plannerRole), func(ctx context.Context) error {
		dir = runDir(ctx)
		planner := NewSession(ctx, plannerRole, ".")
		loop := PromiseLoop(ctx, "sprint", "ship it", planner)
		for taskCtx, task := range loop.Tasks {
			scope, err := current(taskCtx)
			if err != nil {
				return err
			}
			if err := scope.run.SteerLoop("sprint.1", WrapUp); err != nil {
				return err
			}
			if err := scope.run.SteerLoop("sprint.1", "that finding is a misreading, ignore it"); err != nil {
				return err
			}
			// Only a loop takes messages: a task's own scope is not one.
			if err := scope.run.SteerLoop("sprint.1/task.1", "hello"); err == nil {
				t.Errorf("a message to the task scope of %q was accepted", task.Name)
			}
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 {
		t.Fatalf("planning turns = %d, want two", len(prompts))
	}
	if strings.Contains(prompts[0], WrapUp) {
		t.Fatalf("the first decision was told of a message sent after it:\n%s", prompts[0])
	}
	next := prompts[1]
	if !strings.Contains(next, WrapUp) || !strings.Contains(next, "that finding is a misreading, ignore it") {
		t.Fatalf("the planner was not told what the operator sent:\n%s", next)
	}
	if !strings.Contains(next, "end dispatch at this decision") {
		t.Fatalf("the prompt does not state what wrapping up means:\n%s", next)
	}
	recorded := steers(t, dir)
	if len(recorded) != 2 {
		t.Fatalf("Steer records = %+v, want two on the loop", recorded)
	}
	for _, steer := range recorded {
		if steer.Target != "sprint.1" || steer.Source != "person" || !steer.Landed {
			t.Errorf("Steer record = %+v, want it landed on sprint.1 from the person", steer)
		}
	}
}

// TestAMessageALoopNeverReadIsRecordedAsDropped: the workflow stops ranging
// over the tasks, so there is no next decision. The message is recorded as
// what it was, unread, rather than silently lost.
func TestAMessageALoopNeverReadIsRecordedAsDropped(t *testing.T) {
	var prompts []string
	var dir string
	err := runTest(t, bind(loopFake(&prompts), "model", plannerRole), func(ctx context.Context) error {
		dir = runDir(ctx)
		planner := NewSession(ctx, plannerRole, ".")
		loop := PromiseLoop(ctx, "sprint", "ship it", planner)
		for taskCtx := range loop.Tasks {
			scope, err := current(taskCtx)
			if err != nil {
				return err
			}
			if err := scope.run.SteerLoop("sprint.1", "one more thing"); err != nil {
				return err
			}
			break // the workflow is done with the loop, so nothing plans again
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	recorded := steers(t, dir)
	if len(recorded) != 1 || recorded[0].Landed || recorded[0].Message != "one more thing" {
		t.Fatalf("Steer records = %+v, want one recorded unread", recorded)
	}
}
