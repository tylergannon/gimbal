package execute

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
	"github.com/tylergannon/polytype"
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

func run(t *testing.T, in Input) (*fake, error) {
	t.Helper()
	f := &fake{}
	models := map[string]gimble.ModelBinding{"worker": {Adapter: f, Model: "test"}}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "execute", models, func(ctx context.Context) error {
		return Execute(ctx, in)
	})
	return f, err
}

func sprintRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	doc := filepath.Join(repo, "docs", "sprints", "SPRINT-001.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("# Sprint 001\n\n## Implementation Plan\n\n- Say hello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// TestExecuteBuildsThenTestsThenReports: the worker gets the sprint document
// by its absolute path, the tests run after it, and the same worker reports
// with the test output in front of it.
func TestExecuteBuildsThenTestsThenReports(t *testing.T) {
	repo := sprintRepo(t)
	f, err := run(t, Input{Sprint: 1, Repo: repo, Test: present("echo ok")})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.prompts) != 2 {
		t.Fatalf("turns = %d, want the build and the report", len(f.prompts))
	}
	doc := filepath.Join(repo, "docs", "sprints", "SPRINT-001.md")
	if !strings.HasPrefix(f.prompts[0], executePrompt) || !strings.Contains(f.prompts[0], "## sprint document\n\n"+doc) {
		t.Errorf("the build prompt does not name the sprint document:\n%s", f.prompts[0])
	}
	if !strings.Contains(f.prompts[1], "## tests\n\n$ echo ok\nexit 0\nok\n") {
		t.Errorf("the report prompt does not carry the test run:\n%s", f.prompts[1])
	}
}

// TestExecuteFailsWhenTheTestsFail: the report is still made, and then the
// run fails with the tests' exit code.
func TestExecuteFailsWhenTheTestsFail(t *testing.T) {
	f, err := run(t, Input{Sprint: 1, Repo: sprintRepo(t), Test: present("exit 3")})
	if err == nil || !strings.Contains(err.Error(), "the tests exited 3") {
		t.Fatalf("err = %v, want the tests' exit code", err)
	}
	if len(f.prompts) != 2 {
		t.Fatalf("turns = %d, want the report before the failure", len(f.prompts))
	}
}

func TestExecuteNeedsTheSprintDocument(t *testing.T) {
	_, err := run(t, Input{Sprint: 7, Repo: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "no sprint document") {
		t.Fatalf("err = %v, want no sprint document", err)
	}
}

func TestGraphReadsWithoutDiagnostics(t *testing.T) {
	g, err := graph.Extract(".", "Execute", "execute")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Diagnostics) != 0 {
		t.Fatalf("the workflow reads without diagnostics, got %v", g.Diagnostics)
	}
}

// present is an Optional that was given.
func present[T any](v T) polytype.Optional[T] { return polytype.Optional[T]{Present: true, Value: v} }
