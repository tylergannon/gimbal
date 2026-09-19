// Package buildfrontend implements one milestone of the run interface design
// and has an independent validator demonstrate it in the running software.
package buildfrontend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry BuildFrontend -name build-frontend

const roleCoding gimble.WorkflowRole = "coding"

// BuildParams are the milestone to build and the task bound for the run.
type BuildParams struct {
	// RequirementsFile is the absolute path to the milestone's claims, the file the run implements and demonstrates.
	RequirementsFile string
	// MaxTasks is the maximum number of planner assignments the run may make.
	MaxTasks int
}

// Assessment is the independent validator's judgment of the milestone.
type Assessment struct {
	// Complete is true when the software runs and the claims of the requirements file hold at 90 to 95 percent in your own observation of the running Storybook or web application; small gaps are listed, not disqualifying.
	Complete bool `json:"complete"`
	// Observed states what the validator personally saw in the code, the checks, and the real browser.
	Observed string `json:"observed"`
	// UnmetClaims lists each claim you did not see holding, each marked small or substantial; small gaps may stand beside Complete true.
	UnmetClaims []string `json:"unmet_claims"`
}

// BuildFrontend implements the milestone in the requirements file task by
// task: a coder works under two supervisors, the workflow runs the fixed
// checks and commits the task so the repository's hooks run, and a validator
// judges the milestone in a real browser. The planner sees every result.
func BuildFrontend(ctx context.Context, env gimble.Env, params BuildParams) error {
	if !filepath.IsAbs(params.RequirementsFile) {
		return fmt.Errorf("requirements-file must be an absolute path")
	}
	if params.MaxTasks < 1 {
		return fmt.Errorf("max-tasks must be at least 1")
	}
	if _, err := os.Stat(params.RequirementsFile); err != nil {
		return fmt.Errorf("requirements file: %w", err)
	}
	webDir := filepath.Join(env.WorkDir, "web")
	scratchDir := filepath.Join(env.WorkDir, "tmp", "build-frontend")
	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return err
	}

	gimble.Set(ctx, "requirements-file", params.RequirementsFile)
	gimble.Set(ctx, "repository", env.WorkDir)
	gimble.Set(ctx, "design directory", filepath.Join(env.WorkDir, "docs", "design"))
	gimble.Set(ctx, "scratch directory", scratchDir)

	exit, _, stderr, err := gimble.RunCommand(ctx, "lefthook-install", env.WorkDir, "lefthook", "install")
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("lefthook install exited %d: %s", exit, strings.TrimSpace(stderr))
	}

	planner := gimble.NewSession(ctx, gimble.RoleSprintPlanning, env.WorkDir)
	coder := gimble.NewSession(ctx, roleCoding, env.WorkDir)
	aesthetics := gimble.NewSession(ctx, gimble.RoleFrontendAesthetics, env.WorkDir)
	architecture := gimble.NewSession(ctx, gimble.RoleFrontendArchitecture, env.WorkDir)
	gimble.Set(ctx, "definition of done", doneRule)
	goal := "Implement the claims in " + params.RequirementsFile +
		" in the repository so the software runs and the independent validator sees the claims holding. Done is 90 to 95 percent, never 100: a task ends when its software runs and its claims hold with only small gaps, and then the next task starts; no task is spent polishing what already works, and a small gap the validator lists is noted, not assigned."
	loop := gimble.PromiseLoop(ctx, "build", goal, planner)

	completed := false
	exhausted := false
	var operationalErr error
	tasksRun := 0
	for taskCtx, task := range loop.Tasks {
		tasksRun++
		workerReport, err := coder.Generate[gimble.Text](taskCtx, codingPrompt,
			gimble.WithSupervisor(aesthetics, aestheticsPrompt, gimble.WithInterval(5*time.Minute)),
			gimble.WithSupervisor(architecture, architecturePrompt, gimble.WithInterval(5*time.Minute)),
		)
		if err != nil {
			operationalErr = err
			break
		}

		taskCheckPassed := true
		if command := strings.TrimSpace(task.Validation.Command); command != "" {
			exit, stdout, stderr, err := gimble.RunCommand(taskCtx, "task-check", env.WorkDir, "sh", "-lc", command)
			gimble.Set(taskCtx, "task check", commandResult(exit, stdout, stderr))
			if err != nil {
				operationalErr = err
				break
			}
			taskCheckPassed = exit == 0
		}

		buildExit, stdout, stderr, err := gimble.RunCommand(taskCtx, "build", env.WorkDir, "just", "build")
		gimble.Set(taskCtx, "just build", commandResult(buildExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}
		storybookExit, stdout, stderr, err := gimble.RunCommand(taskCtx, "storybook", webDir,
			"pnpm", "build-storybook", "--quiet", "-o", filepath.Join(scratchDir, "storybook-static"))
		gimble.Set(taskCtx, "storybook build", commandResult(storybookExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}
		commitExit, stdout, stderr, err := gimble.RunCommand(taskCtx, "commit", env.WorkDir,
			"sh", "-lc", commitScript, "commit", "build-frontend: "+task.Name)
		gimble.Set(taskCtx, "commit", commandResult(commitExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}

		validator := gimble.NewSession(taskCtx, gimble.RoleQAOrchestration, env.WorkDir)
		assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt)
		if err != nil {
			operationalErr = err
			break
		}
		gimble.SetJSON(taskCtx, "independent assessment", assessment)
		gimble.Set(taskCtx, "worker report", string(workerReport))
		checksPassed := taskCheckPassed && buildExit == 0 && storybookExit == 0 && commitExit == 0
		gimble.Set(taskCtx, "deterministic checks passed", checksPassed)
		if checksPassed && assessment.Complete {
			completed = true
			break
		}
		if tasksRun >= params.MaxTasks {
			exhausted = true
			break
		}
	}
	if err := loop.Err(); err != nil {
		return err
	}
	if operationalErr != nil {
		return operationalErr
	}
	if completed {
		return nil
	}
	if exhausted {
		return fmt.Errorf("milestone incomplete after the maximum %d tasks", params.MaxTasks)
	}
	return fmt.Errorf("milestone incomplete: planner stopped before every claim was demonstrated")
}

func commandResult(exit int, stdout, stderr string) string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", exit, stdout, stderr)
}

// commitScript stages everything the task left in the repository and commits
// it under the task's name, which runs the repository's pre-commit hooks, then
// pushes the branch. A task that changed nothing commits nothing and exits 0;
// a push that fails is reported, not counted against the task.
const commitScript = `set -e
git add -A
if git diff --cached --quiet; then
  echo "nothing to commit"
  exit 0
fi
git commit -q -m "$1"
git log -1 --format='committed %h %s'
git push -q -u origin HEAD || echo "push failed; the commit is local"`

// doneRule is the rule every session in the run reads: done is runnable and 90 to 95 percent right, and then the next task.
const doneRule = "Done means the software runs and the claims hold at 90 to 95 percent. At that point the task is finished and the next one starts; nobody polishes to 100, and small gaps are listed for later, not fixed now."

const codingPrompt = "Read the requirements file and the design directory in your context, then implement only the selected task in the repository. Do not expand the specification, modify the requirements, this build workflow, or its fixed checks, and do not commit, push, or merge: after your turn the workflow runs just build and the Storybook build, then commits and pushes the task itself, and the commit hooks are the checks. Anything you produce that is not source, such as screenshots or notes, goes under the scratch directory, and every untracked file you leave in the repository goes into the commit, so leave none you did not mean. Write no proof scripts or run output into the repository. Stop when the task's software runs and its claims hold at 90 to 95 percent; do not polish past that, and leave small gaps for later. Answer with what changed and what you personally ran or observed."

const aestheticsPrompt = "Watch the worker's components against the design directory: the README's rules and the specimen page each component implements. Steer when markup or CSS drifts from its specimen, when text on the map falls under 13px or a node name is not 15px, when a colour is not a token, when contrast drops, when a state from States.html is missing, or when something is drawn that the rules say is not drawn. Do not edit source, object on code style, or ask for anything the requirements do not claim; raise only what would leave a claim unmet, never polish on something that already works."

const architecturePrompt = "Watch for concrete work beyond the requirements file, and for structure that will not survive the move into the application: components that fetch or call remote functions, props that are not the application's own types, legacy Svelte syntax instead of runes, a graph layout library, a second copy of the tokens, or changes under internal/observation or to generated skgo files. Do not edit source, object on style, or demand improvements outside the requirements; raise only what would leave a claim unmet."

const validationPrompt = "Independently read the requirements file, the design README and its specimens, the implementation with its stories and tests, and the recorded command results. Make no source changes and do not treat the worker report as evidence. Start Storybook from the web directory on a free port other than 6006, or the web application when the requirements are about it, open each story in a real browser through the installed Playwright from the shell, screenshot it into the scratch directory, and look at the screenshots yourself beside the specimen pages rendered the same way. Stop what you started afterward and write nothing into the repository. Complete is true when the software runs and the claims hold at 90 to 95 percent by what you personally saw; a green build or a report is not seeing, and a small gap does not make it false. List each claim you did not see holding, marked small or substantial."
