// Package sprint is the sprint workflow. It builds one sprint of
// ephemeral/research/api/SPRINTS.md, or one issue, in the repository it
// runs in, gated as docs/definition-of-done.md says. A researcher reads the
// code once; a planner forked from it keeps the backlog; each task a coder
// forked from it does the planner's work under a supervisor, and the task is
// committed when the checks pass. When the planner is done, a validator
// reports what it did not see working; the planner is told, and plans
// another round. Then the planner files what is left as issues and merges
// the sprint.
package sprint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
)

//go:generate go tool polytype --validate

// Input starts a sprint.
type Input struct {
	// The sprint to build: its number in ephemeral/research/api/SPRINTS.md, e.g. 2. Zero when an issue is built instead.
	Sprint int `json:"sprint"`
	// The issue to build instead of a sprint: a GitHub issue number, or the path of a file holding the issue's text. Empty when a sprint is built.
	Issue string `json:"issue"`
	// Absolute path of the repository. Each validated task is committed to the branch checked out there, and the sprint ends by merging that branch.
	Repo string `json:"repo"`
	// Codex model for the researcher, the planner, and the coders, e.g. "gpt-5.6-luna".
	Model string `json:"model"`
	// Claude Code model for the supervisors and the validator, e.g. "haiku".
	ReviewModel string `json:"review_model"`
	// The most tasks to run in all. The sprint fails if the planner is not done by then.
	Tasks int `json:"tasks"`
	// Call no model and run no command: print each prompt with the schema it would send, answered with an example value, so every prompt can be read before a real run. A viewer, never proof.
	DryRun bool `json:"dry_run"`
}

// review is what the validator reports.
type review struct {
	// What the validator did not see working, one finding each. Empty when everything asked for was seen working.
	NotSeenWorking []string `json:"not_seen_working"`
}

var checks = []string{"go vet ./...", "go test ./..."}

// Sprint builds sprint in.Sprint, or issue in.Issue.
func Sprint(ctx context.Context, in Input) error {
	if in.DryRun {
		d := newDryRun(os.Stdout)
		return run(ctx, in, d, d)
	}
	return run(ctx, in, codex.New(), claude.New())
}

// run is the workflow on the given harnesses: cx for the researcher, the
// planner, and the coders; cl for the supervisors and the validator.
func run(ctx context.Context, in Input, cx, cl gimble.HarnessAdapter) error {
	gimble.SetJSON(ctx, "input", in)
	text, err := goalText(ctx, in)
	if err != nil {
		return err
	}
	goal := text + "\n\n" + fmt.Sprintf(done, in.Repo)
	validation, err := validationSection(in.Repo)
	if err != nil {
		return err
	}

	researcher := gimble.NewSession(ctx, "researcher", cx, in.Model, in.Repo)
	if _, err := researcher.Generate[gimble.Text](ctx, researchPrompt+"\n\n"+goal); err != nil {
		return err
	}
	planner, err := researcher.Fork(ctx, "planner")
	if err != nil {
		return err
	}
	validator := gimble.NewSession(ctx, "validator", cl, in.ReviewModel, in.Repo)

	// Build until the planner is done, then validate. What the validator
	// did not see working is information for the planner's next round,
	// never part of the goal; three rounds at most.
	tasks := 0
	var findings []string
	for round := 1; ; round++ {
		err := gimble.Scope(ctx, "round", func(ctx context.Context) error {
			if len(findings) > 0 {
				gimble.Set(ctx, "what the validator did not see working", "- "+strings.Join(findings, "\n- "))
			}
			loop := gimble.Loop(ctx, "sprint", goal, planner)
			for ctx, task := range loop.Tasks {
				if tasks++; tasks > in.Tasks {
					break
				}
				if err := runTask(ctx, in, researcher, validator, cl, task); err != nil {
					return err
				}
			}
			return loop.Err()
		})
		if err != nil {
			return err
		}
		if tasks > in.Tasks {
			return fmt.Errorf("sprint: the planner was not done after %d tasks", in.Tasks)
		}
		review, err := validator.Generate[review](ctx, validatePrompt+"\n\n## Goal\n\n"+withoutProof(text)+"\n\n"+validation)
		if err != nil {
			return err
		}
		findings = review.NotSeenWorking
		if len(findings) == 0 {
			break
		}
		log.Printf("sprint: round %d: the validator did not see working:\n- %s", round, strings.Join(findings, "\n- "))
		if round == 3 {
			return fmt.Errorf("sprint: the validator still found work missing after %d rounds", round)
		}
	}

	// Exit and merge: the planner, who drove the sprint, files what is left
	// and merges the branch.
	summary, err := planner.Generate[gimble.Text](ctx, mergePrompt)
	if err != nil {
		return err
	}
	log.Printf("sprint: done:\n%s", summary)
	return nil
}

// goalText is what the sprint builds: the sprint's section of SPRINTS.md,
// or one line naming the local file that holds the issue. An issue given by
// number is fetched with gh and written under the repository's .gimble
// directory first, so every agent reads it from the file.
func goalText(ctx context.Context, in Input) (string, error) {
	if in.Issue == "" {
		plan, err := os.ReadFile(filepath.Join(in.Repo, "ephemeral/research/api/SPRINTS.md"))
		if err != nil {
			return "", err
		}
		text, ok := section(string(plan), fmt.Sprintf("## Sprint %d:", in.Sprint))
		if !ok {
			return "", fmt.Errorf("sprint: SPRINTS.md has no Sprint %d", in.Sprint)
		}
		return text, nil
	}
	file := in.Issue
	if number, err := strconv.Atoi(in.Issue); err == nil {
		cmd := exec.CommandContext(ctx, "gh", "issue", "view", strconv.Itoa(number), "--json", "title,body", "-t", `# {{.title}}{{"\n\n"}}{{.body}}{{"\n"}}`)
		cmd.Dir = in.Repo
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("sprint: gh issue view %d: %w", number, err)
		}
		file = filepath.Join(in.Repo, ".gimble", "issues", fmt.Sprintf("%d.md", number))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(file, out, 0o644); err != nil {
			return "", err
		}
	}
	file, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(file); err != nil {
		return "", fmt.Errorf("sprint: issue: %w", err)
	}
	return fmt.Sprintf("Read and implement the issue in %s.", file), nil
}

// validationSection is the Validation section of docs/definition-of-done.md,
// read from the repository when the workflow runs.
func validationSection(repo string) (string, error) {
	doc, err := os.ReadFile(filepath.Join(repo, "docs/definition-of-done.md"))
	if err != nil {
		return "", err
	}
	text, ok := section(string(doc), "## Validation")
	if !ok {
		return "", errors.New("sprint: docs/definition-of-done.md has no Validation section")
	}
	return text, nil
}

// withoutProof drops a sprint's "Proof:" paragraph: it says how the sprint
// is proven from outside, which nothing inside the sprint can show.
func withoutProof(text string) string {
	var kept []string
	for _, paragraph := range strings.Split(text, "\n\n") {
		if !strings.HasPrefix(paragraph, "Proof:") {
			kept = append(kept, paragraph)
		}
	}
	return strings.Join(kept, "\n\n")
}

// runTask does one assignment, records the evidence requested by the planner,
// and commits only when that evidence and the workflow's repository checks pass.
func runTask(ctx context.Context, in Input, researcher, validator *gimble.Session, cl gimble.HarnessAdapter, task gimble.Task) error {
	coder, err := researcher.Fork(ctx, "coder")
	if err != nil {
		return err
	}
	supervisor := gimble.NewSession(ctx, "supervisor", cl, in.ReviewModel, in.Repo)
	result, workErr := coder.Generate[gimble.Text](ctx, codePrompt+"\n\n"+gimble.ScopeText(ctx),
		gimble.WithSupervisor(supervisor, superviseInstruction),
	)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	gimble.Set(ctx, "worker result", string(result))
	passed := workErr == nil
	if workErr != nil {
		gimble.Set(ctx, "worker error", workErr.Error())
		log.Printf("sprint: task %q: %v", task.Name, workErr)
	}

	if strings.TrimSpace(task.Validation.Command) != "" {
		code, output, err := command(ctx, in, task.Validation.Command)
		if err != nil {
			return err
		}
		gimble.Set(ctx, "task command", commandText(task.Validation.Command, code, output))
		if code != 0 {
			passed = false
		}
	}
	assessmentPrompt := fmt.Sprintf(taskValidationPrompt, task.DefinitionOfDone)
	if query := strings.TrimSpace(task.Validation.Query); query != "" {
		assessmentPrompt += "\n\nAdditional validation question:\n" + query
	}
	assessment, err := validator.Generate[review](ctx, assessmentPrompt+"\n\n"+gimble.ScopeText(ctx))
	if err != nil {
		return err
	}
	gimble.SetJSON(ctx, "task assessment", assessment)
	if len(assessment.NotSeenWorking) != 0 {
		passed = false
	}
	for i, check := range checks {
		code, output, err := command(ctx, in, check)
		if err != nil {
			return err
		}
		gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), commandText(check, code, output))
		if code != 0 {
			passed = false
		}
	}
	if !passed {
		log.Printf("sprint: task %q did not validate, so its work stays uncommitted", task.Name)
		return nil
	}
	if _, err := git(ctx, in, "add", "-A"); err != nil {
		return err
	}
	if status, _ := git(ctx, in, "status", "--porcelain"); status == "" {
		log.Printf("sprint: task %q: nothing to commit", task.Name)
		return nil
	}
	message := fmt.Sprintf("Sprint %d: %s\n\n%s", in.Sprint, task.Name, task.Description)
	if in.Issue != "" {
		message = fmt.Sprintf("Issue %s: %s\n\n%s", in.Issue, task.Name, task.Description)
	}
	if _, err := git(ctx, in, "commit", "-m", message); err != nil {
		return err
	}
	log.Printf("sprint: task %q: committed", task.Name)
	return nil
}

// command runs text with sh in the repository. A dry run runs nothing.
func command(ctx context.Context, in Input, text string) (int, string, error) {
	if in.DryRun {
		return 0, "(dry run: not run)\n", nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", text)
	cmd.Dir = in.Repo
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return 0, "", ctx.Err()
	}
	if err == nil {
		return 0, string(out), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), string(out), nil
	}
	return 0, "", fmt.Errorf("sprint: command %q: %w", text, err)
}

func commandText(command string, code int, output string) string {
	const limit = 3000
	if len(output) > limit {
		output = "[...]" + strings.ToValidUTF8(output[len(output)-limit:], "")
	}
	return fmt.Sprintf("$ %s\nexit %d\n%s", command, code, output)
}

const done = "Definition of done (docs/definition-of-done.md): the software actually works, and what this asks for is 90-95%% built and committed in %s, with `go vet ./...` and `go test ./...` exiting 0 there and each part seen working in a test or a command. Gate on these requirements only, never on code quality. When this is true the goal is met, even with quirks left: they are filed as issues after the loop, not fixed in it."

const researchPrompt = `You are about to lead the build of the goal below on this repository, Gimble, a Go library. Read AGENTS.md, docs/definition-of-done.md, ephemeral/research/api/API.md, ephemeral/research/api/SPRINTS.md, and the code the goal touches, until you know where everything it needs is. Change no files. Answer with a short summary of what exists and what the goal needs.`

const codePrompt = `Complete the task in the scoped context, following AGENTS.md and ephemeral/research/api/API.md. Demonstrate the result and leave the work uncommitted. Answer with a short summary of what changed and the evidence you gathered.`

const superviseInstruction = "Don't let it build what its task does not ask for, over-engineer what it does build, or break a rule in AGENTS.md. Object to nothing else: code quality and style are not yours to judge."

const taskValidationPrompt = `Assess the task using the recorded result and evidence.

Definition of done: %s

List what that evidence does not show working at the repository's 90-95%% readiness standard, and nothing else; an empty list passes the task. A passing agent judgment cannot override a failed deterministic check.`

const validatePrompt = `Check this repository as the Validation section below says, changing no files and committing nothing, and report what you did not see working of the goal below.`

const mergePrompt = `The validator saw the goal working, so finish as docs/definition-of-done.md says. File each quirk and bug left as a GitHub issue with gh issue create, skipping any that gh issue list already has. Then push this branch, open a pull request for it with gh pr create that says what was built, how it was seen working, and which issues it left, and merge it with gh pr merge --squash. Answer with the pull request's URL and the issues you filed.`

// section returns the section of doc under heading, a "## " line, up to
// the next heading of that level.
func section(doc, heading string) (string, bool) {
	_, rest, ok := strings.Cut("\n"+doc, "\n"+heading)
	if !ok {
		return "", false
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return strings.TrimSpace(heading + body), true
}

// git runs git in the repository and returns its trimmed output. A dry run
// runs nothing and returns nothing.
func git(ctx context.Context, in Input, args ...string) (string, error) {
	if in.DryRun {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = in.Repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// newDryRun is the harness of a dry run: it writes each turn to w.
func newDryRun(w io.Writer) *dryRun {
	return &dryRun{w: w, models: make(map[string]string), answered: make(map[string]int)}
}
