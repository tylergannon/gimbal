package implementation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/runlog"
	"github.com/tylergannon/gimbal/workflow"
)

type implementationHarness struct {
	mu          sync.Mutex
	created     int
	plans       int
	prompts     []string
	assessments []Assessment
	qaFailures  int
	qaAttempts  int
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

func (h *implementationHarness) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	switch {
	case strings.Contains(prompt, "You plan the loop"):
		h.mu.Lock()
		h.plans++
		h.prompts = append(h.prompts, prompt)
		h.mu.Unlock()
		return gimbal.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement the next part of this outcome","definition_of_done":"the selected behavior is directly demonstrated","validation":{"command":"printf task-check-output","query":"Observe whether it works"}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently validate the selected task and current outcome"):
		h.mu.Lock()
		h.qaAttempts++
		if h.qaAttempts <= h.qaFailures {
			h.mu.Unlock()
			return gimbal.TurnResult{}, errors.New("claude: result for unknown tool_use_id toolu_test")
		}
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
			return gimbal.TurnResult{}, err
		}
		return gimbal.TurnResult{Output: raw}, nil
	default:
		if len(schema) == 0 {
			return gimbal.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
		}
		return gimbal.TurnResult{Output: json.RawMessage(`"no objection"`)}, nil
	}
}

func TestImplementRecoversQAProtocolFailureAndRecordsEachAttempt(t *testing.T) {
	for _, test := range []struct {
		name        string
		qaFailures  int
		wantSuccess bool
	}{
		{"recovers", 1, true},
		{"exhausted", 2, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := &implementationHarness{qaFailures: test.qaFailures}
			env, params := implementationParams(t, []string{"QA must validate the outcome"}, 1)
			project := t.TempDir()
			models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
				gimbal.RoleSprintPlanning:        {Adapter: h, Model: "model"},
				roleCoding:                       {Adapter: h, Model: "model"},
				gimbal.RoleArchitecturalCritique: {Adapter: h, Model: "model"},
				gimbal.RoleQAOrchestration:       {Adapter: h, Model: "model"},
			}
			err := gimbal.Run(gimbal.Project(t.Context(), project), "implementation-test", models,
				func(ctx context.Context) error { return Implement(ctx, env, params) })
			if test.wantSuccess && err != nil {
				t.Fatal(err)
			}
			if !test.wantSuccess && (err == nil || !strings.Contains(err.Error(), "provider/session error after 2 attempts") || strings.Contains(err.Error(), "incomplete")) {
				t.Fatalf("run error = %v, want explicit provider/session failure", err)
			}
			if h.qaAttempts != 2 || h.plans != 1 {
				t.Fatalf("QA attempts = %d, planner turns = %d; want 2 and 1", h.qaAttempts, h.plans)
			}
			runs, err := filepath.Glob(filepath.Join(project, "runs", "*"))
			if err != nil || len(runs) != 1 {
				t.Fatalf("run directories = %v, error = %v", runs, err)
			}
			var qaTurns []gimbal.TurnEnded
			var judgments, passedChecks int
			if err := runlog.Read[gimbal.LifecycleRecord](t.Context(), runs[0], func(record gimbal.LifecycleRecord) error {
				if strings.Contains(record.Turn.Value, "/qa-orchestration.") {
					if ended, ok := record.Event.(gimbal.TurnEnded); ok {
						qaTurns = append(qaTurns, ended)
					}
				}
				if set, ok := record.Event.(gimbal.ValueSet); ok && set.Key == "independent assessment" {
					judgments++
				}
				if check, ok := record.Event.(gimbal.CommandEnded); ok && strings.Contains(check.Stdout, "task-check-output") && check.ExitCode == 0 {
					passedChecks++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if passedChecks != 1 {
				t.Fatalf("passed task checks = %d, want one before QA recovery", passedChecks)
			}
			if len(qaTurns) != 2 || !strings.Contains(qaTurns[0].Error, "unknown tool_use_id") {
				t.Fatalf("QA turn records = %+v, want original error and retry", qaTurns)
			}
			if test.wantSuccess && (qaTurns[1].Error != "" || judgments != 1) {
				t.Fatalf("recovered QA turn = %+v, judgments = %d", qaTurns[1], judgments)
			}
			if !test.wantSuccess && (!strings.Contains(qaTurns[1].Error, "unknown tool_use_id") || judgments != 0) {
				t.Fatalf("exhausted QA turn = %+v, judgments = %d", qaTurns[1], judgments)
			}
		})
	}
}

func implementationParams(t *testing.T, outcomes []string, maxTasks int) (gimbal.Env, Params) {
	t.Helper()
	workDir := t.TempDir()
	raw, err := json.Marshal(outcomes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "outcomes.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return gimbal.Env{WorkDir: workDir}, Params{OutcomesFile: "outcomes.json", MaxTasksPerOutcome: maxTasks}
}

func runImplementation(t *testing.T, h *implementationHarness, env gimbal.Env, params Params) error {
	t.Helper()
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
		gimbal.RoleSprintPlanning:        {Adapter: h, Model: "model"},
		roleCoding:                       {Adapter: h, Model: "model"},
		gimbal.RoleArchitecturalCritique: {Adapter: h, Model: "model"},
		gimbal.RoleQAOrchestration:       {Adapter: h, Model: "model"},
	}
	return gimbal.Run(gimbal.Project(t.Context(), t.TempDir()), "implementation-test", models,
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

func TestImplementDoesNotAdvanceOnContradictoryValidation(t *testing.T) {
	h := &implementationHarness{assessments: []Assessment{
		{ValidationPassed: true, Observed: "partial", SubstantialGaps: []string{"project B controls are broken"}},
		{ValidationPassed: true, Observed: "both projects work"},
		{ValidationPassed: true, Observed: "next outcome works"},
	}}
	env, params := implementationParams(t, []string{"First", "Second"}, 2)
	if err := runImplementation(t, h, env, params); err != nil {
		t.Fatal(err)
	}
	if h.plans != 3 {
		t.Fatalf("planner turns = %d, want two for first outcome and one for second", h.plans)
	}
	if !strings.Contains(h.prompts[1], "project B controls are broken") || !strings.Contains(h.prompts[2], "Second") {
		t.Fatalf("contradictory assessment advanced the outcome: %q", h.prompts)
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
