package sprint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

// taskAdapter answers a worker's prose turn, a planner's dispatch (one task,
// then dispatch ends), and a validator's assessment (always an objection),
// so runTask can be exercised the way Loop actually drives it: with the
// task record already in the task scope under "task".
type taskAdapter struct {
	task           gimble.Task
	prompts        []string
	plans          int
	validatorFails bool // the validator's turn errors instead of answering
}

func (a *taskAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}

func (a *taskAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.prompts = append(a.prompts, prompt)
	switch {
	case len(schema) == 0:
		out, err := json.Marshal("worker finished")
		return gimble.TurnResult{Output: out}, err
	case strings.HasPrefix(prompt, "You plan the loop"):
		a.plans++
		next := 0
		p := struct {
			Tasks []gimble.Task `json:"tasks"`
			Next  *int          `json:"next"`
		}{Tasks: []gimble.Task{a.task}}
		if a.plans == 1 {
			p.Next = &next
		}
		raw, err := json.Marshal(p)
		return gimble.TurnResult{Output: raw}, err
	case a.validatorFails:
		return gimble.TurnResult{}, errors.New("adapter: the validator's turn failed")
	default:
		return gimble.TurnResult{Output: json.RawMessage(`{"not_seen_working":["The task has no evidence for its definition of done."]}`)}, nil
	}
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

	oldChecks := repositoryChecks
	repositoryChecks = struct{ vet, test string }{}
	t.Cleanup(func() { repositoryChecks = oldChecks })

	adapter := &taskAdapter{task: gimble.Task{
		Name:             "prove-result",
		Description:      "Produce the expected result.",
		DefinitionOfDone: "The expected result is demonstrated.",
	}}
	models := sprintModels(adapter, "test")
	models["planner"] = gimble.ModelBinding{Adapter: adapter, Model: "test"}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", models, func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", repo)
		validator := gimble.NewSession(ctx, "validator", repo)
		planner := gimble.NewSession(ctx, "planner", repo)
		loop := gimble.Loop(ctx, "sprint", "ship", planner)
		for ctx, task := range loop.Tasks {
			if err := runTask(ctx, Input{Sprint: 1, Repo: repo}, researcher, validator, task); err != nil {
				return err
			}
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(adapter.prompts) != 4 {
		t.Fatalf("turns = %d, want two planner dispatches, the worker, and the task assessment", len(adapter.prompts))
	}
	assessment := adapter.prompts[2]
	if !strings.Contains(assessment, "Done when: The expected result is demonstrated.") {
		t.Fatalf("assessment prompt omits the task's definition of done:\n%s", assessment)
	}
	if strings.Contains(assessment, `"definition_of_done"`) {
		t.Fatalf("assessment prompt shows the task as its JSON record, not as its own terms:\n%s", assessment)
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
	in := Input{Issue: issue, Repo: repo, Tasks: 10, DryRun: true}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", sprintModels(d, "dry"), func(ctx context.Context) error {
		return Sprint(ctx, in)
	})
	if err != nil {
		t.Fatal(err, "\n", out.String())
	}

	prompts := dryPrompts(t, out.String())
	want := "Read and implement the issue in " + issue + "."
	var planner, validations, assessments []string
	for _, prompt := range prompts {
		if strings.Contains(prompt, "gh issue view") {
			t.Errorf("a prompt points at a remote source:\n%s", prompt)
		}
		switch {
		case strings.HasPrefix(prompt, "You plan the loop"):
			planner = append(planner, prompt)
		case strings.HasPrefix(prompt, validatePrompt):
			validations = append(validations, prompt)
		case strings.HasPrefix(prompt, taskValidationPrompt):
			assessments = append(assessments, prompt)
		}
	}
	if len(assessments) == 0 {
		t.Fatalf("no task assessment among %d prompts", len(prompts))
	}
	for _, prompt := range assessments {
		if !strings.Contains(prompt, "## the task under assessment") || !strings.Contains(prompt, "Answer this about it too:") {
			t.Errorf("the task assessment is not shaped by its own template:\n%s", prompt)
		}
		if strings.Contains(prompt, "## input") || strings.Contains(prompt, `"dry_run"`) {
			t.Errorf("the task assessment carries the run's own input record:\n%s", prompt)
		}
		if !strings.Contains(prompt, "## definition of done") {
			t.Errorf("the task assessment lost a scoped value its template keeps:\n%s", prompt)
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
		if strings.Contains(prompt, "Proof:") {
			t.Errorf("the validator's prompt carries a Proof line:\n%s", prompt)
		}
		if !strings.Contains(prompt, "## definition of done\n\nDefinition of done (docs") {
			t.Errorf("the validator's prompt lacks the definition of done, now delivered through scope:\n%s", prompt)
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

// sprintModels binds the three roles the sprint names to one harness.
func sprintModels(adapter gimble.HarnessAdapter, model string) map[string]gimble.ModelBinding {
	models := map[string]gimble.ModelBinding{}
	for _, role := range []string{"researcher", "validator", "supervisor"} {
		models[role] = gimble.ModelBinding{Adapter: adapter, Model: model}
	}
	return models
}

// TestAValidatorTurnErrorFailsTheTaskAndTheLoopGoesOn: an error inside a
// task is not the sprint's end. The validator's turn fails here; the task's
// work stays uncommitted, the reason is recorded on the task's scope where
// the planner reads it before its next decision, and dispatch continues.
func TestAValidatorTurnErrorFailsTheTaskAndTheLoopGoesOn(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "before"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "before")
	runGit(t, repo, "commit", "-qm", "before")

	oldChecks := repositoryChecks
	repositoryChecks = struct{ vet, test string }{}
	t.Cleanup(func() { repositoryChecks = oldChecks })

	adapter := &taskAdapter{validatorFails: true, task: gimble.Task{
		Name:             "prove-result",
		Description:      "Produce the expected result.",
		DefinitionOfDone: "The expected result is demonstrated.",
	}}
	models := sprintModels(adapter, "test")
	models["planner"] = gimble.ModelBinding{Adapter: adapter, Model: "test"}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", models, func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", repo)
		validator := gimble.NewSession(ctx, "validator", repo)
		planner := gimble.NewSession(ctx, "planner", repo)
		loop := gimble.Loop(ctx, "sprint", "ship", planner)
		for ctx, task := range loop.Tasks {
			if err := runTask(ctx, Input{Sprint: 1, Repo: repo}, researcher, validator, task); err != nil {
				return err
			}
		}
		return loop.Err()
	})
	if err != nil {
		t.Fatalf("the sprint ended on a failed validator turn: %v", err)
	}
	if adapter.plans != 2 {
		t.Fatalf("planner dispatches = %d, want the loop to plan again after the failed task", adapter.plans)
	}
	second := adapter.prompts[len(adapter.prompts)-1]
	if !strings.Contains(second, "## task error") || !strings.Contains(second, "the validator's turn failed") {
		t.Fatalf("the planner was not told why the task failed:\n%s", second)
	}
	if got := runGit(t, repo, "rev-list", "--count", "HEAD"); got != "1" {
		t.Fatalf("commit count = %s, want 1: a failed task must stay uncommitted", got)
	}
}

// TestACancelledContextStillEndsTheSprint: a cancelled ctx is the one thing
// that ends a sprint from inside a task.
func TestACancelledContextStillEndsTheSprint(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")

	adapter := &taskAdapter{task: gimble.Task{
		Name:             "prove-result",
		Description:      "Produce the expected result.",
		DefinitionOfDone: "The expected result is demonstrated.",
	}}
	models := sprintModels(adapter, "test")
	models["planner"] = gimble.ModelBinding{Adapter: adapter, Model: "test"}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "test", models, func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", repo)
		validator := gimble.NewSession(ctx, "validator", repo)
		planner := gimble.NewSession(ctx, "planner", repo)
		loop := gimble.Loop(ctx, "sprint", "ship", planner)
		for taskCtx, task := range loop.Tasks {
			cancelled, cancel := context.WithCancel(taskCtx)
			cancel()
			if err := runTask(cancelled, Input{Sprint: 1, Repo: repo}, researcher, validator, task); err != nil {
				return err
			}
		}
		return loop.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("the sprint ended with %v, want context.Canceled", err)
	}
}
