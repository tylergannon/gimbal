package lfg

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
)

type fakeAdapter struct {
	mu      sync.Mutex
	models  []string
	prompts []string
	turns   int
}

func (a *fakeAdapter) CreateSession(_ context.Context, model, _ string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.models = append(a.models, model)
	return "session", nil
}
func (a *fakeAdapter) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.mu.Lock()
	a.prompts = append(a.prompts, prompt)
	a.turns++
	a.mu.Unlock()
	b, _ := json.Marshal("worker complete")
	return gimble.TurnResult{Output: b}, nil
}
func (*fakeAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fakeAdapter) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*fakeAdapter) Close(context.Context, string) error                 { return nil }

func TestRunUsesScopedPromptAndFailedChecksGate(t *testing.T) {
	repo := t.TempDir()
	worker := &fakeAdapter{}
	reviewer := &fakeAdapter{}
	var got string
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "lfg", func(ctx context.Context) error {
		var err error
		got, err = run(ctx, Input{
			Repo: repo, Goal: "make the change", Acceptance: "the behavior works",
			Constraints: "keep it small", Model: "worker-model", ReviewModel: "review-model",
			Checks: []string{"printf check-output; exit 7"}, SupervisorIntervalSeconds: 1,
		}, worker, reviewer)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "exited with code 7") {
		t.Fatalf("error = %v, want failed check", err)
	}
	if got != "worker complete" {
		t.Fatalf("worker result = %q", got)
	}
	if worker.turns != 1 {
		t.Fatalf("worker turns = %d, want one", worker.turns)
	}
	if len(worker.prompts) != 1 || !strings.Contains(worker.prompts[0], "## goal") || !strings.Contains(worker.prompts[0], "make the change") || !strings.Contains(worker.prompts[0], "## checks") || !strings.Contains(worker.prompts[0], "printf check-output; exit 7") || !strings.Contains(worker.prompts[0], repo) {
		t.Fatalf("worker prompt did not include scoped request: %q", worker.prompts)
	}
	if len(worker.models) != 1 || worker.models[0] != "worker-model" {
		t.Fatalf("models = worker %v", worker.models)
	}
}

func TestDryRunDoesNotExecuteCheck(t *testing.T) {
	repo := t.TempDir()
	var out strings.Builder
	result, err := dryRun(Input{Repo: repo, Goal: "goal", Checks: []string{"exit 99"}}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" || !strings.Contains(out.String(), "no model or command was called") || !strings.Contains(out.String(), "exit 99") {
		t.Fatalf("dry run output/result = %q / %q", out.String(), result)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatal(err)
	}
}
