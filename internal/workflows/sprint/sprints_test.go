package sprint

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

type taskAdapter struct {
	prompts []string
}

func (a *taskAdapter) CreateSession(context.Context, string, string) (string, error) {
	return "session", nil
}

func (a *taskAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.prompts = append(a.prompts, prompt)
	if len(schema) != 0 {
		return gimble.TurnResult{Output: json.RawMessage(`{"not_seen_working":["The task has no evidence for its definition of done."]}`)}, nil
	}
	out, err := json.Marshal("worker finished")
	return gimble.TurnResult{Output: out}, err
}

func (*taskAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*taskAdapter) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*taskAdapter) Close(context.Context, string) error                 { return nil }

func TestRunTaskAssessesDefinitionOfDoneWithoutValidationRecipe(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "before"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "before")
	runGit(t, repo, "commit", "-qm", "before")

	oldChecks := checks
	checks = nil
	t.Cleanup(func() { checks = oldChecks })

	adapter := &taskAdapter{}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", adapter, "test", repo)
		validator := gimble.NewSession(ctx, "validator", adapter, "test", repo)
		return runTask(ctx, Input{Sprint: 1, Repo: repo, ReviewModel: "test"}, researcher, validator, adapter, gimble.Task{
			Name:             "prove-result",
			Description:      "Produce the expected result.",
			DefinitionOfDone: "The expected result is demonstrated.",
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(adapter.prompts) != 2 {
		t.Fatalf("turns = %d, want worker plus task assessment", len(adapter.prompts))
	}
	if !strings.Contains(adapter.prompts[1], "Definition of done: The expected result is demonstrated.") {
		t.Fatalf("assessment prompt omits task definition of done:\n%s", adapter.prompts[1])
	}
	if got := runGit(t, repo, "rev-list", "--count", "HEAD"); got != "1" {
		t.Fatalf("commit count = %s, want 1: an objection must block the task commit", got)
	}
}

// TestDryRunShowsEveryPromptAndKeepsFindingsOutOfTheGoal runs the workflow
// dry on an issue file: no model and no command run, every prompt is printed
// with its schema, the issue reaches each agent as an absolute local path,
// and what the validator did not see working reaches the planner as scoped
// context while the goal stays what it was.
func TestDryRunShowsEveryPromptAndKeepsFindingsOutOfTheGoal(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs/definition-of-done.md"), []byte("# Definition of done\n\n## Development\n\nGated on requirements.\n\n## Validation\n\nThe validation side is an agent. It is not a blank check.\n\n## Exit and merge\n\nMerge at 90-95%.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issue := filepath.Join(repo, "issue.md")
	if err := os.WriteFile(issue, []byte("# Say hello\n\nPrint hello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	d := newDryRun(&out)
	in := Input{Issue: issue, Repo: repo, Model: "codex-model", ReviewModel: "claude-model", Tasks: 10, DryRun: true}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", func(ctx context.Context) error {
		return run(ctx, in, d, d)
	})
	if err != nil {
		t.Fatal(err, "\n", out.String())
	}

	prompts := dryPrompts(t, out.String())
	want := "Read and implement the issue in " + issue + "."
	var planner, validations []string
	for _, prompt := range prompts {
		if strings.Contains(prompt, "gh issue view") {
			t.Errorf("a prompt points at a remote source:\n%s", prompt)
		}
		switch {
		case strings.HasPrefix(prompt, "You plan the loop"):
			planner = append(planner, prompt)
		case strings.HasPrefix(prompt, validatePrompt):
			validations = append(validations, prompt)
		}
	}
	for i, prompt := range []string{prompts[0], planner[0], validations[0]} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt %d does not name the issue file %q:\n%s", i, want, prompt)
		}
	}
	if len(validations) != 2 || len(planner) != 3 {
		t.Fatalf("validator prompts = %d, planner prompts = %d; want 2 and 3 (a round, a second after findings)", len(validations), len(planner))
	}
	if strings.Count(validatePrompt, "\n") != 0 {
		t.Errorf("the validator's prompt is not one line: %q", validatePrompt)
	}
	for _, prompt := range validations {
		if strings.Contains(prompt, "Definition of done (docs") || strings.Contains(prompt, "Proof:") {
			t.Errorf("the validator's prompt carries done boilerplate or a Proof line:\n%s", prompt)
		}
		if !strings.Contains(prompt, "## Validation\n\nThe validation side is an agent.") {
			t.Errorf("the validator's prompt lacks the Validation section:\n%s", prompt)
		}
	}
	finding := "<example not_seen_working>"
	goal, _ := goalText(t.Context(), in)
	for i, prompt := range planner {
		_, backlog, ok := strings.Cut(prompt, "Backlog now:\n\n")
		if !ok {
			t.Fatalf("planner prompt %d has no backlog:\n%s", i, prompt)
		}
		var b struct{ Goal string }
		backlog, _, _ = strings.Cut(backlog, "\n\nReturn the full")
		if err := json.Unmarshal([]byte(backlog), &b); err != nil {
			t.Fatalf("planner prompt %d backlog: %v:\n%s", i, err, backlog)
		}
		if !strings.HasPrefix(b.Goal, goal) || strings.Contains(b.Goal, finding) {
			t.Errorf("planner prompt %d: the goal changed:\n%s", i, b.Goal)
		}
		informed := strings.Contains(prompt, "## what the validator did not see working\n\n- "+finding)
		if informed != (i == 2) {
			t.Errorf("planner prompt %d informed of findings = %v:\n%s", i, informed, prompt)
		}
	}
	if !strings.Contains(out.String(), "\"not_seen_working\"") || !strings.Contains(out.String(), "example answer (no model was called)") {
		t.Errorf("the dry run does not show the schema or mark its answers:\n%s", out.String())
	}
	if entries, _ := os.ReadDir(repo); len(entries) != 2 {
		t.Errorf("the dry run touched the repository: %v", entries)
	}
}

// dryPrompts returns each prompt a dry run printed, in order.
func dryPrompts(t *testing.T, out string) []string {
	t.Helper()
	var prompts []string
	for _, turn := range strings.Split(out, "=== turn ")[1:] {
		_, rest, ok := strings.Cut(turn, "--- prompt ---\n")
		if !ok {
			t.Fatalf("turn without a prompt:\n%s", turn)
		}
		prompt, _, _ := strings.Cut(rest, "\n\n--- schema ---\n")
		prompts = append(prompts, prompt)
	}
	return prompts
}

func TestWithoutProofDropsTheProofParagraph(t *testing.T) {
	text := "## Sprint 3: the app\n\nShip: a page.\n\n- one\n- two\n\nProof: Sprint 4 is built by `cmd/sprint`\nfrom the form.\n\n`API.md`: Run."
	got := withoutProof(text)
	if strings.Contains(got, "Proof") || !strings.Contains(got, "- two\n\n`API.md`: Run.") {
		t.Fatalf("withoutProof = %q", got)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
