package issueseries

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

type seriesHarness struct {
	mu          sync.Mutex
	created     int
	plans       int
	validations int
	checkpoints int
	prompts     []string
}

func (h *seriesHarness) CreateSession(context.Context, string, string, string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.created++
	return "session-" + strconv.Itoa(h.created), nil
}

func (*seriesHarness) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*seriesHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*seriesHarness) Close(context.Context, string) error                 { return nil }

func (h *seriesHarness) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	h.mu.Lock()
	h.prompts = append(h.prompts, prompt)
	h.mu.Unlock()
	switch {
	case strings.Contains(prompt, "You plan the loop"):
		h.mu.Lock()
		h.plans++
		h.mu.Unlock()
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement this issue","definition_of_done":"the issue works","validation":{"command":"printf checked","query":"observe it"}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently validate the selected task and current issue"):
		h.mu.Lock()
		h.validations++
		h.mu.Unlock()
		return gimble.TurnResult{Output: json.RawMessage(`{"validation_passed":true,"observed":"saw it work","substantial_gaps":[],"small_gaps":[]}`)}, nil
	case strings.Contains(prompt, "has passed independent validation"):
		h.mu.Lock()
		h.checkpoints++
		h.mu.Unlock()
		return gimble.TurnResult{Output: json.RawMessage(`"committed and pushed"`)}, nil
	default:
		if len(schema) == 0 {
			return gimble.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"objections":[]}`)}, nil
	}
}

func TestImplementSeriesRunsIssuesInOrderWithResearchVisible(t *testing.T) {
	workDir := t.TempDir()
	issueA := filepath.Join(workDir, "a.md")
	issueB := filepath.Join(workDir, "b.md")
	research := filepath.Join(workDir, "research.md")
	list := filepath.Join(workDir, "issues.txt")
	for path, content := range map[string]string{
		issueA:   "# A\n",
		issueB:   "# B\n",
		research: "# Research index\n",
		list:     "a.md\nb.md\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &seriesHarness{}
	models := map[gimble.WorkflowRole]gimble.ModelBinding{
		gimble.RoleSprintPlanning:        {Adapter: h, Model: "planner"},
		gimble.RoleArchitecturalCritique: {Adapter: h, Model: "coach"},
		roleCoding:                       {Adapter: h, Model: "coder"},
		gimble.RoleQAOrchestration:       {Adapter: h, Model: "validator"},
	}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "issue-series-test", models, func(ctx context.Context) error {
		return ImplementSeries(ctx, gimble.Env{WorkDir: workDir}, Params{
			IssueListFile: list, ResearchIndexFile: research, MaxTasksPerIssue: 2,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.plans != 2 || h.validations != 2 || h.checkpoints != 2 {
		t.Fatalf("plans=%d validations=%d checkpoints=%d, want 2 each", h.plans, h.validations, h.checkpoints)
	}
	joined := strings.Join(h.prompts, "\n")
	for _, required := range []string{issueA, issueB, research} {
		if !strings.Contains(joined, required) {
			t.Fatalf("rendered prompts do not contain %s", required)
		}
	}
	if strings.Index(joined, issueA) > strings.LastIndex(joined, issueB) {
		t.Fatal("issue B appeared before issue A")
	}
}

func TestReadIssueListRejectsDuplicates(t *testing.T) {
	workDir := t.TempDir()
	issue := filepath.Join(workDir, "issue.md")
	list := filepath.Join(workDir, "issues.txt")
	if err := os.WriteFile(issue, []byte("issue"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(list, []byte("issue.md\n./issue.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readIssueList(workDir, list); err == nil || !strings.Contains(err.Error(), "duplicate issue file") {
		t.Fatalf("readIssueList error = %v", err)
	}
}

func TestGeneratedGraphShowsOuterIterationAndInnerPromiseLoop(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("generated graph diagnostics = %+v", Graph.Diagnostics)
	}
	if len(Graph.Body) == 0 {
		t.Fatal("generated graph is empty")
	}
	var found bool
	for _, operation := range Graph.Body {
		iteration, ok := operation.(workflow.Iterate)
		if !ok || iteration.Name != "issue" {
			continue
		}
		for _, nested := range iteration.Body {
			if loop, ok := nested.(workflow.PromiseLoop); ok && loop.Name == "implementation" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("generated graph has no implementation PromiseLoop inside issue Iterate")
	}
}
