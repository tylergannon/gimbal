package easyloop

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/graph"
	"github.com/tylergannon/gimble/workflow"
)

// fake plays every role: prose turns say "done", the planner dispatches one
// task per lap for as many laps as plans allows and then ends dispatch, and
// the reviewer objects until the visit given by metAt, when it sees the spec
// met.
type fake struct {
	mu      sync.Mutex
	prompts []string
	plans   int // dispatches that select a task before the planner ends dispatch
	metAt   int // the review that sees the spec met; 0 for never
	planned int
	reviews int
}

func (*fake) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}

func (f *fake) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prompts = append(f.prompts, prompt)
	switch {
	case len(schema) == 0:
		out, err := json.Marshal("done")
		return gimble.TurnResult{Output: out}, err
	case strings.HasPrefix(prompt, "You plan the loop"):
		f.planned++
		task := `{"name":"hello","description":"Print hello.","definition_of_done":"hello prints.","validation":{"command":"","query":""}}`
		if f.planned <= f.plans {
			return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[` + task + `],"next":0}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[],"next":null}`)}, nil
	default:
		f.reviews++
		if f.reviews == f.metAt {
			return gimble.TurnResult{Output: json.RawMessage(`{"not_seen_working":[],"spec_met":true}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"not_seen_working":["The greeting has no test."],"spec_met":false}`)}, nil
	}
}

func (*fake) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fake) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*fake) Close(context.Context, string) error                 { return nil }

func run(t *testing.T, f *fake, in Input) error {
	t.Helper()
	models := map[string]gimble.ModelBinding{}
	for _, role := range []string{"planner", "critic", "coder", "reviewer"} {
		models[role] = gimble.ModelBinding{Adapter: f, Model: "test"}
	}
	return gimble.Run(gimble.Project(t.Context(), t.TempDir()), "easyloop", models, func(ctx context.Context) error {
		return EasyLoop(ctx, in)
	})
}

// TestTheReviewerEndsTheLoop: the plan is written, critiqued, and revised
// before dispatch; each task is coded then reviewed; the reviewer's verdict
// reaches the planner's next decision; and the loop ends when the reviewer
// sees the spec met, before the planner runs out of tasks.
func TestTheReviewerEndsTheLoop(t *testing.T) {
	repo := t.TempDir()
	f := &fake{plans: 5, metAt: 2}
	if err := run(t, f, Input{Spec: repo + "/spec.md", WorkDir: repo}); err != nil {
		t.Fatal(err)
	}
	if len(f.prompts) != 9 {
		t.Fatalf("turns = %d, want plan, critique, update, and two laps of dispatch, code, review", len(f.prompts))
	}
	for i, want := range []string{planPrompt, critiquePrompt, updatePrompt} {
		if !strings.HasPrefix(f.prompts[i], want) {
			t.Errorf("turn %d is not %q:\n%s", i, want[:20], f.prompts[i])
		}
	}
	code := f.prompts[4]
	if !strings.HasPrefix(code, codePrompt) || !strings.Contains(code, "## spec document\n\n"+repo+"/spec.md") || !strings.Contains(code, "## plan directory\n\n"+repo+"/docs/plans/spec") || !strings.Contains(code, "## task\n\n") {
		t.Errorf("the coder's prompt lacks the spec, the plan directory, or the task:\n%s", code)
	}
	review := f.prompts[5]
	if !strings.HasPrefix(review, reviewPrompt) || !strings.Contains(review, "## coder's report\n\ndone") {
		t.Errorf("the reviewer's prompt lacks the coder's report:\n%s", review)
	}
	second := f.prompts[6]
	if !strings.HasPrefix(second, "You plan the loop") || !strings.Contains(second, "The greeting has no test.") {
		t.Errorf("the planner's second decision was not told the reviewer's findings:\n%s", second)
	}
}

// TestThePlannerStoppingFirstFailsTheRun: dispatch that ends before the
// reviewer has seen the spec met is not success.
func TestThePlannerStoppingFirstFailsTheRun(t *testing.T) {
	repo := t.TempDir()
	err := run(t, &fake{plans: 1}, Input{Spec: repo + "/spec.md", WorkDir: repo})
	if err == nil || !strings.Contains(err.Error(), "had not seen the spec met after 1 tasks") {
		t.Fatalf("err = %v, want the reviewer's verdict to be missing", err)
	}
}

func TestGraphReadsWithoutDiagnostics(t *testing.T) {
	g, err := graph.Extract(".", "EasyLoop", "easyloop")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Diagnostics) != 0 {
		t.Fatalf("the workflow reads without diagnostics, got %v", g.Diagnostics)
	}
	for _, op := range g.Body {
		if loop, ok := op.(workflow.Loop); ok {
			if loop.Name != "work" || loop.Planner != "planner" {
				t.Errorf("loop = %q with planner %q, want work planned by planner", loop.Name, loop.Planner)
			}
			return
		}
	}
	t.Fatal("the entry body holds no Loop")
}
