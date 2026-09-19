package implementation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/workflow"
)

type implementationHarness struct {
	mu          sync.Mutex
	created     int
	plans       int
	prompts     []string
	assessments []Assessment
	closed      []string
}

func (h *implementationHarness) CreateSession(context.Context, string, string, string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.created++
	return "session-" + strconv.Itoa(h.created), nil
}

func (*implementationHarness) Fork(context.Context, string) (string, error) { return "fork", nil }

func (*implementationHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }

func (h *implementationHarness) Close(_ context.Context, session string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = append(h.closed, session)
	return nil
}

func (h *implementationHarness) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	switch {
	case strings.Contains(prompt, "You plan the loop"):
		h.mu.Lock()
		h.plans++
		h.prompts = append(h.prompts, prompt)
		h.mu.Unlock()
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement the next part of the promise","definition_of_done":"the selected behavior is directly demonstrated","validation":{"command":"printf task-check-output","query":"Observe whether the selected behavior works"}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently validate the selected task and the overall promise"):
		h.mu.Lock()
		assessment := Assessment{ValidationPassed: true, Observed: "promise works", SubstantialGaps: []string{}, SmallGaps: []string{"optional polish"}}
		if len(h.assessments) > 0 {
			assessment = h.assessments[0]
			h.assessments = h.assessments[1:]
		}
		h.mu.Unlock()
		raw, err := json.Marshal(assessment)
		if err != nil {
			return gimble.TurnResult{}, err
		}
		return gimble.TurnResult{Output: raw}, nil
	default:
		if len(schema) == 0 {
			return gimble.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`"no objection"`)}, nil
	}
}

func implementationParams(t *testing.T, maxTasks int) (gimble.Env, Params) {
	t.Helper()
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "done.md"), []byte("The feature works in the real application."), 0o644); err != nil {
		t.Fatal(err)
	}
	return gimble.Env{WorkDir: workDir}, Params{
		Promise:              "Implement the feature",
		DefinitionOfDoneFile: "done.md",
		MaxTasks:             maxTasks,
	}
}

func runImplementation(t *testing.T, h *implementationHarness, env gimble.Env, params Params) error {
	t.Helper()
	models := map[gimble.WorkflowRole]gimble.ModelBinding{
		gimble.RoleSprintPlanning:        {Adapter: h, Model: "model"},
		roleCoding:                       {Adapter: h, Model: "model"},
		gimble.RoleArchitecturalCritique: {Adapter: h, Model: "model"},
		gimble.RoleQAOrchestration:       {Adapter: h, Model: "model"},
	}
	return gimble.Run(gimble.Project(t.Context(), t.TempDir()), "implementation-test", models,
		func(ctx context.Context) error { return Implement(ctx, env, params) })
}

func TestImplementStopsImmediatelyWhenValidationPassesWithSmallGaps(t *testing.T) {
	h := &implementationHarness{assessments: []Assessment{{ValidationPassed: true, Observed: "saw it work at 90–95%", SubstantialGaps: []string{}, SmallGaps: []string{"optional polish remains"}}}}
	env, params := implementationParams(t, 3)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
	if h.plans != 1 {
		t.Fatalf("planner turns = %d, want 1; a passing validation with small gaps must not take another lap", h.plans)
	}
}

func TestImplementReplansOnlyForSubstantialGaps(t *testing.T) {
	h := &implementationHarness{assessments: []Assessment{
		{ValidationPassed: false, Observed: "inspected repository", SubstantialGaps: []string{"still incomplete"}, SmallGaps: []string{}},
		{ValidationPassed: true, Observed: "promise works", SubstantialGaps: []string{}, SmallGaps: []string{"optional polish"}},
	}}
	env, params := implementationParams(t, 2)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
	if h.plans != 2 {
		t.Fatalf("planner turns = %d, want 2", h.plans)
	}
	if !strings.Contains(h.prompts[1], "still incomplete") || !strings.Contains(h.prompts[1], "task-check-output") {
		t.Fatalf("second planner prompt lacks validation or check evidence:\n%s", h.prompts[1])
	}
}

func TestGeneratedGraphShowsTheImplementationLoop(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("generated graph diagnostics = %+v", Graph.Diagnostics)
	}
	found := false
	for _, operation := range Graph.Body {
		if loop, ok := operation.(workflow.PromiseLoop); ok && loop.Name == "implementation" {
			found = true
		}
	}
	if !found {
		t.Fatal("generated graph has no implementation PromiseLoop")
	}
}
