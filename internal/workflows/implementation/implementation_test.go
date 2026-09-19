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
	mu       sync.Mutex
	created  int
	plans    int
	prompts  []string
	verdicts []bool
	closed   []string
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
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement the requirement","definition_of_done":"the requirement works","validation":{"command":"","query":""}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently read the complete requirements file"):
		h.mu.Lock()
		complete := true
		if len(h.verdicts) > 0 {
			complete = h.verdicts[0]
			h.verdicts = h.verdicts[1:]
		}
		h.mu.Unlock()
		if complete {
			return gimble.TurnResult{Output: json.RawMessage(`{"complete":true,"observed":"requirement works","unmet_requirements":[]}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"complete":false,"observed":"inspected repository","unmet_requirements":["still incomplete"]}`)}, nil
	default:
		if len(schema) == 0 {
			return gimble.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`"no objection"`)}, nil
	}
}

func implementationParams(t *testing.T, validation string, maxTasks int) (gimble.Env, Params) {
	t.Helper()
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "requirements.md"), []byte("Implement the feature."), 0o644); err != nil {
		t.Fatal(err)
	}
	return gimble.Env{WorkDir: workDir}, Params{
		RequirementsFile:  "requirements.md",
		ValidationCommand: validation,
		MaxTasks:          maxTasks,
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

func TestImplementSucceedsOnlyWithCommandAndIndependentAcceptance(t *testing.T) {
	h := &implementationHarness{verdicts: []bool{true}}
	env, params := implementationParams(t, "true", 1)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
}

func TestImplementFailedValidationCannotBeOverridden(t *testing.T) {
	h := &implementationHarness{verdicts: []bool{true}}
	env, params := implementationParams(t, "false", 1)
	if err := runImplementation(t, h, env, params); err == nil {
		t.Fatal("failed fixed validation was accepted")
	}
}

func TestImplementReplansWithValidatorAndCommandEvidence(t *testing.T) {
	h := &implementationHarness{verdicts: []bool{false, true}}
	env, params := implementationParams(t, "printf fixed-validation-output", 2)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
	if h.plans != 2 {
		t.Fatalf("planner turns = %d, want 2", h.plans)
	}
	if !strings.Contains(h.prompts[1], "still incomplete") || !strings.Contains(h.prompts[1], "fixed-validation-output") {
		t.Fatalf("second planner prompt lacks validator or command evidence:\n%s", h.prompts[1])
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
