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
}

func (h *implementationHarness) CreateSession(context.Context, string, string, string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.created++
	return "session-" + strconv.Itoa(h.created), nil
}

func (*implementationHarness) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*implementationHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*implementationHarness) Close(context.Context, string) error                 { return nil }

func (h *implementationHarness) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	switch {
	case strings.Contains(prompt, "You plan the loop"):
		h.mu.Lock()
		h.plans++
		h.prompts = append(h.prompts, prompt)
		h.mu.Unlock()
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement the next part of this outcome","definition_of_done":"the selected behavior is directly demonstrated","validation":{"command":"printf task-check-output","query":"Observe whether it works"}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently validate the selected task and current outcome"):
		h.mu.Lock()
		assessment := Assessment{ValidationPassed: true, Observed: "saw the outcome work", SmallGaps: []string{"optional polish"}}
		if len(h.assessments) > 0 {
			assessment = h.assessments[0]
			h.assessments = h.assessments[1:]
		}
		h.mu.Unlock()
		if assessment.SubstantialGaps == nil {
			assessment.SubstantialGaps = []string{}
		}
		if assessment.SmallGaps == nil {
			assessment.SmallGaps = []string{}
		}
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

func implementationParams(t *testing.T, outcomes []string, maxTasks int) (gimble.Env, Params) {
	t.Helper()
	workDir := t.TempDir()
	raw, err := json.Marshal(outcomes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "outcomes.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return gimble.Env{WorkDir: workDir}, Params{OutcomesFile: "outcomes.json", MaxTasksPerOutcome: maxTasks}
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

func TestImplementAdvancesThroughOutcomesInOrderWithoutExtraPlanning(t *testing.T) {
	h := &implementationHarness{assessments: []Assessment{
		{ValidationPassed: true, Observed: "first works", SmallGaps: []string{"polish"}},
		{ValidationPassed: true, Observed: "second works"},
	}}
	env, params := implementationParams(t, []string{"First outcome", "Second outcome"}, 3)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
	if h.plans != 2 {
		t.Fatalf("planner turns = %d, want one per outcome", h.plans)
	}
	if !strings.Contains(h.prompts[0], "First outcome") || !strings.Contains(h.prompts[1], "Second outcome") {
		t.Fatalf("outcomes not planned in order: %q", h.prompts)
	}
}

func TestImplementReplansInsideOutcomeAndStopsBeforeLaterOutcomeOnFailure(t *testing.T) {
	h := &implementationHarness{assessments: []Assessment{
		{ValidationPassed: true, Observed: "first works"},
		{ValidationPassed: false, Observed: "second incomplete", SubstantialGaps: []string{"still missing"}},
		{ValidationPassed: false, Observed: "second incomplete", SubstantialGaps: []string{"still missing"}},
	}}
	env, params := implementationParams(t, []string{"First", "Second", "Third"}, 2)
	err := runImplementation(t, h, env, params)
	if err == nil || !strings.Contains(err.Error(), "outcome 2 incomplete after 2 tasks") {
		t.Fatalf("result = %v, want bounded outcome-2 failure", err)
	}
	if h.plans != 3 {
		t.Fatalf("planner turns = %d, want 1 for first and 2 for second", h.plans)
	}
	if !strings.Contains(h.prompts[2], "still missing") || !strings.Contains(h.prompts[2], "task-check-output") {
		t.Fatalf("second lap lacks feedback: %s", h.prompts[2])
	}
	for _, prompt := range h.prompts {
		if strings.Contains(prompt, "Third") {
			t.Fatalf("later outcome started after failure: %s", prompt)
		}
	}
}

func TestImplementRejectsEmptyOutcomesBeforeStartingAgents(t *testing.T) {
	env, params := implementationParams(t, []string{"First", " "}, 2)
	h := &implementationHarness{}
	if err := runImplementation(t, h, env, params); err == nil || !strings.Contains(err.Error(), "outcome 2 is blank") {
		t.Fatalf("result = %v, want blank-outcome error", err)
	}
	if h.created != 0 {
		t.Fatalf("created %d sessions before validating input", h.created)
	}
}

func TestGeneratedGraphShowsOutcomesAndScopeSupervisors(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("generated graph diagnostics = %+v", Graph.Diagnostics)
	}
	var outcomes workflow.Iterate
	for _, operation := range Graph.Body {
		if item, ok := operation.(workflow.Iterate); ok {
			outcomes = item
		}
	}
	if outcomes.Name != "outcome" {
		t.Fatal("generated graph has no ordered outcome iterator")
	}
	for _, operation := range outcomes.Body {
		if loop, ok := operation.(workflow.PromiseLoop); ok && loop.Name == "implementation" {
			if len(loop.Supervisors) != 1 {
				t.Fatalf("planner supervisors = %d, want 1", len(loop.Supervisors))
			}
			var workerSupervised, validatorSupervised bool
			for _, taskOp := range loop.Body {
				if call, ok := taskOp.(workflow.AgentCall); ok {
					switch call.Role {
					case "coding":
						workerSupervised = len(call.Supervisors) == 1
					case "qa-orchestration":
						validatorSupervised = len(call.Supervisors) == 1
					}
				}
			}
			if !workerSupervised || !validatorSupervised {
				t.Fatalf("missing worker or validator supervisor: worker=%t validator=%t", workerSupervised, validatorSupervised)
			}
			return
		}
	}
	t.Fatal("generated graph has no per-outcome PromiseLoop")
}
