package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tylergannon/gimbal"
	"strings"
	"testing"
)

type scriptedHarness struct {
	answers  []any
	prompts  []string
	sessions int
	closed   int
}

func (a *scriptedHarness) CreateSession(context.Context, string, string, string) (string, error) {
	a.sessions++
	return fmt.Sprint(a.sessions), nil
}
func (a *scriptedHarness) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.prompts = append(a.prompts, prompt)
	if len(a.answers) == 0 {
		return gimbal.TurnResult{}, fmt.Errorf("unexpected extra turn")
	}
	raw, err := json.Marshal(a.answers[0])
	a.answers = a.answers[1:]
	return gimbal.TurnResult{Output: raw}, err
}
func (*scriptedHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*scriptedHarness) Fork(context.Context, string) (string, error) {
	return "", fmt.Errorf("not used")
}
func (a *scriptedHarness) Close(context.Context, string) error { a.closed++; return nil }

func TestSprintRevisionUsesOriginalBriefAndReview(t *testing.T) {
	plan := Sprint{Goal: "local trial", Tasks: []Task{{"baseline", "local", []string{}, "blocked infrastructure"}, {"trial", "two stacks", []string{"baseline"}, "no cross access"}, {"cleanup", "stop", []string{"trial"}, "within 10 seconds"}}}
	a := &scriptedHarness{answers: []any{plan, Review{Findings: []string{"observe the trial live"}}, plan, Review{Findings: []string{}}}}
	bindings := map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleSprintPlanning: {Adapter: a}, gimbal.RoleCodeReview: {Adapter: a}}
	err := gimbal.Run(gimbal.Project(context.Background(), t.TempDir()), "test-sprint", bindings, func(ctx context.Context) error {
		_, err := SprintPlan(ctx, gimbal.Env{WorkDir: t.TempDir()}, "original bounded fixture")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.prompts) != 4 || a.sessions != 3 || a.closed != 3 {
		t.Fatalf("turns/sessions/cleanup = %d/%d/%d", len(a.prompts), a.sessions, a.closed)
	}
	if !strings.Contains(a.prompts[2], "original bounded fixture") || !strings.Contains(a.prompts[2], "observe the trial live") {
		t.Fatal("revision lost original input or review")
	}
}
func TestSprintStopsAfterOneRejectedRevision(t *testing.T) {
	plan := Sprint{Goal: "trial", Tasks: []Task{{"a", "baseline", []string{}, "run"}, {"b", "trial", []string{"a"}, "check"}, {"c", "cleanup", []string{"b"}, "stop"}}}
	a := &scriptedHarness{answers: []any{plan, Review{Findings: []string{"gap"}}, plan, Review{Findings: []string{"still unresolved"}}}}
	bindings := map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleSprintPlanning: {Adapter: a}, gimbal.RoleCodeReview: {Adapter: a}}
	err := gimbal.Run(gimbal.Project(context.Background(), t.TempDir()), "test-sprint-rejected", bindings, func(ctx context.Context) error {
		result, err := SprintPlan(ctx, gimbal.Env{WorkDir: t.TempDir()}, "fixture")
		if len(result.Tasks) != 3 {
			t.Fatal("rejected candidate lost")
		}
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "still unresolved") {
		t.Fatalf("want final rejection, got %v", err)
	}
	if len(a.prompts) != 4 {
		t.Fatal("unbounded repair turns")
	}
}
