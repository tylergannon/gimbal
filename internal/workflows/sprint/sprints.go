// Package sprint is the sprint workflow. It builds one sprint of
// ephemeral/research/api/SPRINTS.md in the repository it runs in, gated as
// docs/definition-of-done.md says. A researcher reads the code once; a
// planner forked from it keeps the backlog; each task a coder forked from it
// does the planner's work under a supervisor, and the task is committed when
// the checks pass. When the planner is done, a validator checks that the
// sprint is demonstrated, and its objections are another loop. Then the
// planner files what is left as issues and merges the sprint.
package sprint

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
)

//go:generate go tool polytype --validate

// Input starts a sprint.
type Input struct {
	// The sprint to build: its number in ephemeral/research/api/SPRINTS.md, e.g. 2.
	Sprint int `json:"sprint"`
	// Absolute path of the repository. Each validated task is committed to the branch checked out there, and the sprint ends by merging that branch.
	Repo string `json:"repo"`
	// Codex model for the researcher, the planner, and the coders, e.g. "gpt-5.6-luna".
	Model string `json:"model"`
	// Claude Code model for the supervisors and the validator, e.g. "haiku".
	ReviewModel string `json:"review_model"`
	// The most tasks to run in all. The sprint fails if the planner is not done by then.
	Tasks int `json:"tasks"`
}

// review is the validator's assessment of whether the sprint was demonstrated.
type review struct {
	// Each objection is one requirement whose evidence is invalid, written as an instruction to the builders. Leave the list empty when the sprint is demonstrated.
	Objections []string `json:"objections"`
}

var checks = []string{"go vet ./...", "go test ./..."}

// Sprint builds sprint in.Sprint.
func Sprint(ctx context.Context, in Input) error {
	if err := gimble.SetJSON(ctx, "input", in); err != nil {
		return err
	}
	plan, err := os.ReadFile(filepath.Join(in.Repo, "ephemeral/research/api/SPRINTS.md"))
	if err != nil {
		return err
	}
	text, ok := section(string(plan), in.Sprint)
	if !ok {
		return fmt.Errorf("sprint: SPRINTS.md has no Sprint %d", in.Sprint)
	}
	goal := text + "\n\n" + fmt.Sprintf(done, in.Repo)

	cx, cl := codex.New(), claude.New()
	researcher := gimble.NewSession(ctx, "researcher", cx, in.Model, in.Repo)
	if _, err := researcher.Generate[gimble.Text](ctx, researchPrompt+"\n\n"+goal); err != nil {
		return err
	}
	planner, err := researcher.Fork(ctx, "planner")
	if err != nil {
		return err
	}
	validator := gimble.NewSession(ctx, "validator", cl, in.ReviewModel, in.Repo)

	// Build until the planner is done, then validate. The validator's
	// objections are the goal of another loop, for three rounds at most.
	tasks := 0
	for round := 1; ; round++ {
		loop := gimble.Loop(ctx, "sprint", goal, planner)
		for ctx, task := range loop.Tasks {
			if tasks++; tasks > in.Tasks {
				break
			}
			if err := runTask(ctx, in, researcher, validator, cl, task); err != nil {
				return err
			}
		}
		if err := loop.Err(); err != nil {
			return err
		}
		if tasks > in.Tasks {
			return fmt.Errorf("sprint: the planner was not done after %d tasks", in.Tasks)
		}
		review, err := validator.Generate[review](ctx, fmt.Sprintf(validatePrompt, in.Sprint)+"\n\n"+goal)
		if err != nil {
			return err
		}
		if len(review.Objections) == 0 {
			break
		}
		objections := "- " + strings.Join(review.Objections, "\n- ")
		log.Printf("sprint: round %d: the validator objects:\n%s", round, objections)
		if round == 3 {
			return fmt.Errorf("sprint: the validator still objects after %d rounds", round)
		}
		goal = text + "\n\n" + fmt.Sprintf(done, in.Repo) + "\n\nThe validator found the validation of this goal invalid, so it is also done only when these are answered:\n\n" + objections
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
	if err := gimble.Set(ctx, "worker result", string(result)); err != nil {
		return err
	}
	passed := workErr == nil
	if workErr != nil {
		if err := gimble.Set(ctx, "worker error", workErr.Error()); err != nil {
			return err
		}
		log.Printf("sprint: task %q: %v", task.Name, workErr)
	}

	if strings.TrimSpace(task.Validation.Command) != "" {
		code, output, err := command(ctx, in.Repo, task.Validation.Command)
		if err != nil {
			return err
		}
		if err := gimble.Set(ctx, "task command", commandText(task.Validation.Command, code, output)); err != nil {
			return err
		}
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
	if err := gimble.SetJSON(ctx, "task assessment", assessment); err != nil {
		return err
	}
	if len(assessment.Objections) != 0 {
		passed = false
	}
	for i, check := range checks {
		code, output, err := command(ctx, in.Repo, check)
		if err != nil {
			return err
		}
		if err := gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), commandText(check, code, output)); err != nil {
			return err
		}
		if code != 0 {
			passed = false
		}
	}
	if !passed {
		log.Printf("sprint: task %q did not validate, so its work stays uncommitted", task.Name)
		return nil
	}
	if _, err := git(ctx, in.Repo, "add", "-A"); err != nil {
		return err
	}
	if status, _ := git(ctx, in.Repo, "status", "--porcelain"); status == "" {
		log.Printf("sprint: task %q: nothing to commit", task.Name)
		return nil
	}
	message := fmt.Sprintf("Sprint %d: %s\n\n%s", in.Sprint, task.Name, task.Description)
	if _, err := git(ctx, in.Repo, "commit", "-m", message); err != nil {
		return err
	}
	log.Printf("sprint: task %q: committed", task.Name)
	return nil
}

func command(ctx context.Context, dir, text string) (int, string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", text)
	cmd.Dir = dir
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

const done = "Definition of done (docs/definition-of-done.md): the software actually works, and what this section asks for is 90-95%% built and committed in %s, with `go vet ./...` and `go test ./...` exiting 0 there and each part seen working in a test or a command. Gate on these requirements only, never on code quality. When this is true the goal is met, even with quirks left: they are filed as issues after the loop, not fixed in it. Give the two checks steps of their own, with those commands."

const researchPrompt = `You are about to lead the build of one sprint on this repository, Gimble, a Go library. Read AGENTS.md, docs/definition-of-done.md, ephemeral/research/api/API.md, ephemeral/research/api/SPRINTS.md, and the code the sprint below touches, until you know where everything it needs is. Change no files. Answer with a short summary of what exists and what the sprint needs.`

const codePrompt = `Complete the task in the scoped context, following AGENTS.md and ephemeral/research/api/API.md. Demonstrate the result and leave the work uncommitted. Answer with a short summary of what changed and the evidence you gathered.`

const superviseInstruction = "Don't let it build what its task does not ask for, over-engineer what it does build, or break a rule in AGENTS.md. Object to nothing else: code quality and style are not yours to judge."

const taskValidationPrompt = `Assess the task using the recorded result and evidence.

Definition of done: %s

Object only when that evidence does not establish a working result at the repository's 90-95%% readiness standard. A passing agent judgment cannot override a failed deterministic check.`

const validatePrompt = `You are the validator of one sprint of this repository, Gimble. Its goal is below, and the agents that built it say it is done. Read docs/definition-of-done.md, then decide whether the sprint's validation legitimately demonstrates the goal: that the software actually works and what the goal asks for is implemented and has been seen working.

- Find what demonstrates each part of the goal: the tests and the commands. Check that each really exercises what it claims to, and that none was weakened, skipped, or faked to pass. The sprint's commits are titled "Sprint %d: ..."; git log -p shows what they changed.
- Run the checks yourself, and run the software by hand where that shows more.
- Change no files and commit nothing.

Object only where the validation is invalid: the software does not work, a large part of the goal is missing (the sprint is done at 90-95%%, and the rest is filed as issues), a demonstration does not show what it claims, or a check was tampered with. Code quality, style, enhancements, and bugs that do not stop the goal being demonstrated are not objections; file the ones that matter as GitHub issues with gh issue create. Each objection becomes work for the builders, so write it as an instruction to them. Answer with an empty list when the goal is demonstrated.`

const mergePrompt = `The validator found the sprint demonstrated, so finish it as docs/definition-of-done.md says. File each quirk and bug the sprint leaves as a GitHub issue with gh issue create, skipping any that gh issue list already has. Then push this branch, open a pull request for it with gh pr create that says what the sprint built, how it was seen working, and which issues it left, and merge it with gh pr merge --squash. Answer with the pull request's URL and the issues you filed.`

// section returns the "## Sprint n:" section of SPRINTS.md, up to the next
// heading of its level.
func section(doc string, n int) (string, bool) {
	heading := fmt.Sprintf("## Sprint %d:", n)
	_, rest, ok := strings.Cut(doc, "\n"+heading)
	if !ok {
		return "", false
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return heading + body, true
}

// git runs git in dir and returns its trimmed output.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
