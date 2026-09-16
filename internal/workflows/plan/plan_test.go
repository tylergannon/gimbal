package plan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/graph"
	"github.com/tylergannon/gimble/workflow"
)

// fake answers every prose turn with "done" and keeps each prompt.
type fake struct {
	mu      sync.Mutex
	prompts []string
}

func (*fake) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}

func (f *fake) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prompts = append(f.prompts, prompt)
	out, err := json.Marshal("done")
	return gimble.TurnResult{Output: out}, err
}

func (*fake) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fake) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*fake) Close(context.Context, string) error                 { return nil }

// TestEveryLaneDraftsThenCritiquesTheOtherTwo: the drafts are written to
// their own files, each critique names the other two drafts and not its own,
// and the merge waits for the person's answers.
func TestEveryLaneDraftsThenCritiquesTheOtherTwo(t *testing.T) {
	repo := t.TempDir()
	drafts := filepath.Join(repo, "docs", "sprints", "drafts")
	if err := os.MkdirAll(drafts, 0o755); err != nil {
		t.Fatal(err)
	}
	// The person has already answered, so the run's wait ends at once.
	if err := os.WriteFile(filepath.Join(drafts, "SPRINT-002-ANSWERS.md"), []byte("The Claude draft.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fake{}
	models := map[string]gimble.ModelBinding{}
	for _, role := range []string{"planner", "claude", "codex", "gemini"} {
		models[role] = gimble.ModelBinding{Adapter: f, Model: "test"}
	}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "plan", models, func(ctx context.Context) error {
		return Plan(ctx, Input{Sprint: 2, Seed: "Draw the graph.", WorkDir: repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.prompts) != 9 {
		t.Fatalf("turns = %d, want the intent, three drafts, three critiques, the questions, and the merge", len(f.prompts))
	}
	var seen []string
	for _, prompt := range f.prompts {
		switch {
		case strings.HasPrefix(prompt, intentPrompt):
			seen = append(seen, "intent")
			if !strings.Contains(prompt, "## seed\n\nDraw the graph.") {
				t.Errorf("the intent prompt lacks the seed:\n%s", prompt)
			}
		case strings.HasPrefix(prompt, draftPrompt):
			seen = append(seen, "draft")
		case strings.HasPrefix(prompt, critiquePrompt):
			seen = append(seen, "critique")
		case strings.HasPrefix(prompt, questionsPrompt):
			seen = append(seen, "questions")
		case strings.HasPrefix(prompt, mergePrompt):
			seen = append(seen, "merge")
			if !strings.Contains(prompt, "## answers file\n\n"+filepath.Join(drafts, "SPRINT-002-ANSWERS.md")) {
				t.Errorf("the merge prompt does not name the answers:\n%s", prompt)
			}
		}
	}
	if got := strings.Join(seen, " "); got != "intent draft draft draft critique critique critique questions merge" {
		t.Errorf("turns in order: %s", got)
	}
	for _, lane := range []string{"CLAUDE", "CODEX", "GEMINI"} {
		draft := filepath.Join(drafts, "SPRINT-002-"+lane+"-DRAFT.md")
		critique := filepath.Join(drafts, "SPRINT-002-"+lane+"-CRITIQUE.md")
		if !has(f.prompts, "## your draft\n\n"+draft) {
			t.Errorf("no turn writes %s", draft)
		}
		i := index(f.prompts, "## your critique\n\n"+critique)
		if i < 0 {
			t.Fatalf("no turn writes %s", critique)
		}
		if strings.Contains(f.prompts[i], draft) {
			t.Errorf("the %s critique reviews its own draft:\n%s", lane, f.prompts[i])
		}
		if strings.Count(f.prompts[i], "-DRAFT.md") != 2 {
			t.Errorf("the %s critique does not review two drafts:\n%s", lane, f.prompts[i])
		}
	}
}

func has(prompts []string, text string) bool { return index(prompts, text) >= 0 }

func index(prompts []string, text string) int {
	for i, prompt := range prompts {
		if strings.Contains(prompt, text) {
			return i
		}
	}
	return -1
}

func TestGraphReadsWithoutDiagnostics(t *testing.T) {
	g, err := graph.Extract(".", "Plan", "plan")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Diagnostics) != 0 {
		t.Fatalf("the workflow reads without diagnostics, got %v", g.Diagnostics)
	}
	groups := 0
	for _, op := range g.Body {
		if group, ok := op.(workflow.Group); ok {
			groups++
			if len(group.Children) != 3 {
				t.Errorf("group %q has %d children, want the three lanes", group.Name, len(group.Children))
			}
		}
	}
	if groups != 2 {
		t.Errorf("groups = %d, want the drafts and the critiques", groups)
	}
}
