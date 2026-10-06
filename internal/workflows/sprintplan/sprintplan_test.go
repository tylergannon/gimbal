package sprintplan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/workflow"
)

// The adapter inspects actual on-disk handoffs and requires each fan-out to
// overlap. It proves orchestration, not the quality of a model's research.
type handoffHarness struct {
	dir, fail string
	mu        sync.Mutex
	next      int
	models    map[string]string
	arrivals  map[string]int
	barriers  map[string]chan struct{}
}

func (a *handoffHarness) CreateSession(_ context.Context, model, _, _ string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next++
	id := fmt.Sprint(a.next)
	a.models[id] = model
	return id, nil
}
func (a *handoffHarness) RunTurn(ctx context.Context, id, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.mu.Lock()
	role := a.models[id]
	a.mu.Unlock()
	if role == a.fail {
		return gimbal.TurnResult{}, fmt.Errorf("fixture failure in %s", role)
	}
	stage, count := "", 0
	var required []string
	switch role {
	case "research-indexing":
		stage, count = "research", 2
	case "index-curation":
		required = []string{"working-set/project/INDEX.md", "working-set/prior-art/INDEX.md"}
	case "sprint-planning":
		for _, lane := range []string{"claude", "codex", "gemini"} {
			required = append(required, "draft/"+lane+".md", "critique/"+lane+".md")
		}
	default:
		stage, count = "draft", 3
		required = []string{"working-set/INDEX.md"}
		if strings.Contains(prompt, "\ncritique\n") {
			stage = "critique"
			for _, lane := range []string{"claude", "codex", "gemini"} {
				required = append(required, "draft/"+lane+".md")
			}
		}
	}
	for _, name := range append(required, "intent.md") {
		if data, err := os.ReadFile(filepath.Join(a.dir, name)); err != nil || len(data) == 0 {
			return gimbal.TurnResult{}, fmt.Errorf("%s reached before %s was saved: %v", role, name, err)
		}
	}
	if stage != "" {
		a.mu.Lock()
		a.arrivals[stage]++
		barrier := a.barriers[stage]
		if a.arrivals[stage] == count {
			close(barrier)
		}
		a.mu.Unlock()
		select {
		case <-barrier:
		case <-ctx.Done():
			return gimbal.TurnResult{}, ctx.Err()
		}
	}
	raw, _ := json.Marshal("# " + role + " " + stage + "\n\nProposed work; see the original intent and local evidence.")
	return gimbal.TurnResult{Output: raw}, nil
}
func (*handoffHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*handoffHarness) Fork(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected fork")
}
func (*handoffHarness) Close(context.Context, string) error { return nil }

func TestSprintHandoffsAndFailure(t *testing.T) {
	for _, failedRole := range []string{"", "research-indexing", "sprint-plan-codex"} {
		t.Run("failure="+failedRole, func(t *testing.T) {
			project := t.TempDir()
			if err := os.WriteFile(filepath.Join(project, "intent.md"), []byte("Users can export their records. Verify a complete export."), 0o644); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(project, "ephemeral", "sprints", "export")
			a := &handoffHarness{dir: dir, fail: failedRole, models: map[string]string{}, arrivals: map[string]int{}, barriers: map[string]chan struct{}{"research": make(chan struct{}), "draft": make(chan struct{}), "critique": make(chan struct{})}}
			bindings := map[gimbal.WorkflowRole]gimbal.ModelBinding{}
			for _, role := range []gimbal.WorkflowRole{"research-indexing", "index-curation", "sprint-plan-claude", "sprint-plan-codex", "sprint-plan-gemini", gimbal.RoleSprintPlanning} {
				bindings[role] = gimbal.ModelBinding{Adapter: a, Model: string(role)}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			err := gimbal.Run(gimbal.Project(ctx, project), "sprint-plan", bindings, func(ctx context.Context) error {
				return SprintPlan(ctx, gimbal.Env{WorkDir: project}, Params{Intent: "intent.md", SprintDir: "ephemeral/sprints/export"})
			})
			if failedRole != "" {
				if err == nil || !strings.Contains(err.Error(), "fixture failure") {
					t.Fatalf("want branch failure, got %v", err)
				}
				if _, err := os.Stat(filepath.Join(dir, "plan.md")); !os.IsNotExist(err) {
					t.Fatalf("failed run published plan: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.next != 10 {
				t.Fatalf("created %d sessions, want ten independent turns", a.next)
			}
			if data, err := os.ReadFile(filepath.Join(dir, "plan.md")); err != nil || !strings.Contains(string(data), "sprint-planning") {
				t.Fatalf("plan = %q, %v", data, err)
			}
			if _, err := prepare(project, Params{Intent: "intent.md", SprintDir: dir}); err == nil {
				t.Fatal("existing sprint was overwritten")
			}
		})
	}
}

func TestEmptyInputsAndOutputsDoNotBecomeArtifacts(t *testing.T) {
	dir := t.TempDir()
	if _, err := prepare(dir, Params{}); err == nil {
		t.Fatal("accepted empty input")
	}
	path := filepath.Join(dir, "plan.md")
	if err := save(path, "  ", nil); err == nil {
		t.Fatal("accepted empty plan")
	}
	if err := save(path, "partial", fmt.Errorf("interrupted")); err == nil {
		t.Fatal("accepted failed turn")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid output published: %v", err)
	}
}

func TestGraphShowsResearchAndBothPlanningPhases(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("graph diagnostics: %+v", Graph.Diagnostics)
	}
	var research, plans bool
	for _, op := range Graph.Body {
		if group, ok := op.(workflow.Group); ok && group.Name == "research" {
			research = len(group.Children) == 2
		}
		if iteration, ok := op.(workflow.Iterate); ok && iteration.Name == "planning" {
			for _, child := range iteration.Body {
				if group, ok := child.(workflow.Group); ok && group.Name == "plans" {
					plans = len(group.Children) == 3
				}
			}
		}
	}
	if !research || !plans {
		t.Fatal("missing visible research or three-lane planning fan-out")
	}
}
