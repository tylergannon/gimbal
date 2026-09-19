// Package implementation implements a local requirements file through a
// bounded, planner-directed loop and independently validates the result.
//
// The requirements file is the authoritative goal. On each lap, a planner
// chooses the next coherent assignment, a fresh coding agent implements it,
// and the workflow runs both the planner's optional task check and one fixed
// caller-supplied validation command. A fresh validator then inspects the
// complete requirements and the actual repository. Failed deterministic
// validation cannot be overruled by an agent's judgment.
//
// The workflow does not commit, push, merge, deploy, or rewrite the
// requirements. It succeeds only when the fixed command passes and the
// independent validator finds the complete requirements satisfied. Otherwise
// it replans until the task limit is exhausted or the planner stops honestly.
//
// Example:
//
//	gimble run implement \
//	  --requirements-file ./issue.md \
//	  --validation-command "just build && just test" \
//	  --max-tasks 8
package implementation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Implement -name implement

const roleCoding gimble.WorkflowRole = "coding"

// Params identify the local requirements, deterministic gate, and task bound.
type Params struct {
	// RequirementsFile is the local file whose complete requirements the run implements.
	RequirementsFile string
	// ValidationCommand is the fixed shell command that must pass before the run can succeed.
	ValidationCommand string
	// MaxTasks is the maximum number of planner assignments the run may execute.
	MaxTasks int
}

// Assessment is an independent validator's judgment of the complete requirements.
type Assessment struct {
	// Complete is true only when the validator observed that every requirement is satisfied.
	Complete bool `json:"complete"`
	// Observed states what the validator personally inspected or ran.
	Observed string `json:"observed"`
	// UnmetRequirements lists required behavior or proof that remains missing.
	UnmetRequirements []string `json:"unmet_requirements"`
}

// Implement modifies a repository until its requirements are independently demonstrated.
func Implement(ctx context.Context, env gimble.Env, params Params) error {
	if params.MaxTasks < 1 {
		return fmt.Errorf("max-tasks must be at least 1")
	}
	validationCommand := strings.TrimSpace(params.ValidationCommand)
	if validationCommand == "" {
		return fmt.Errorf("validation-command must not be blank")
	}
	requirementsFile, err := absoluteFrom(env.WorkDir, params.RequirementsFile)
	if err != nil {
		return fmt.Errorf("requirements-file: %w", err)
	}
	info, err := os.Stat(requirementsFile)
	if err != nil {
		return fmt.Errorf("requirements file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("requirements file is not a regular file: %s", requirementsFile)
	}

	gimble.Set(ctx, "requirements file", requirementsFile)
	gimble.Set(ctx, "repository", env.WorkDir)
	gimble.Set(ctx, "fixed validation command", validationCommand)
	gimble.Set(ctx, "maximum tasks", params.MaxTasks)

	planner := gimble.NewSession(ctx, gimble.RoleSprintPlanning, env.WorkDir)
	plannerCoach := gimble.NewSession(ctx, gimble.RoleArchitecturalCritique, env.WorkDir)
	goal := "Implement every requirement in " + requirementsFile +
		" in the repository and demonstrate that the fixed validation command passes and an independent validator finds the complete requirements satisfied."
	loop := gimble.PromiseLoop(ctx, "implementation", goal, planner,
		gimble.WithSupervisor(plannerCoach, planningScopePrompt))

	completed := false
	exhausted := false
	var operationalErr error
	tasksRun := 0
	for taskCtx, task := range loop.Tasks {
		tasksRun++
		worker := gimble.NewSession(taskCtx, roleCoding, env.WorkDir)
		workerCoach := gimble.NewSession(taskCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
		workerReport, err := worker.Generate[gimble.Text](taskCtx, implementationPrompt,
			gimble.WithSupervisor(workerCoach, implementationScopePrompt))
		if err != nil {
			operationalErr = err
			break
		}
		gimble.Set(taskCtx, "worker report", string(workerReport))

		taskCheckPassed := true
		if command := strings.TrimSpace(task.Validation.Command); command != "" {
			exit, stdout, stderr, err := gimble.RunCommand(taskCtx, "task-check", env.WorkDir, "sh", "-lc", command)
			if err != nil {
				operationalErr = err
				break
			}
			gimble.Set(taskCtx, "task check", commandResult(exit, stdout, stderr))
			taskCheckPassed = exit == 0
		}

		exit, stdout, stderr, err := gimble.RunCommand(taskCtx, "fixed-validation", env.WorkDir, "sh", "-lc", validationCommand)
		if err != nil {
			operationalErr = err
			break
		}
		gimble.Set(taskCtx, "fixed validation", commandResult(exit, stdout, stderr))

		validator := gimble.NewSession(taskCtx, gimble.RoleQAOrchestration, env.WorkDir)
		assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt)
		if err != nil {
			operationalErr = err
			break
		}
		gimble.SetJSON(taskCtx, "independent assessment", assessment)
		checksPassed := taskCheckPassed && exit == 0
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
		return fmt.Errorf("implementation incomplete after the maximum %d tasks", params.MaxTasks)
	}
	return fmt.Errorf("implementation incomplete: planner stopped before the requirements were demonstrated")
}

func absoluteFrom(workDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("path must not be blank")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(workDir, name)
	}
	return filepath.Abs(name)
}

func commandResult(exit int, stdout, stderr string) string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", exit, stdout, stderr)
}

const planningScopePrompt = `Keep the plan inside the requirements file. Object to invented features, speculative infrastructure, polishing, or unrelated repairs. Preserve failed checks and validator findings as evidence for replanning; do not declare the overall goal complete merely because dispatch can stop.`

const implementationScopePrompt = `Watch only for concrete work beyond the selected assignment or the requirements file. Object to invented features, speculative abstractions, weakened validation, edits to the requirements, and unrelated cleanup. Do not edit source or demand optional polish.`

const implementationPrompt = `Read the requirements file and implement only the selected task in the repository. Make the actual source changes and run useful focused checks. Do not edit the requirements, this workflow, or the fixed validation command; do not commit, push, merge, deploy, or add proof scripts and run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed.`

const validationPrompt = `Independently read the complete requirements file, inspect the actual repository changes and recorded command results, and run any additional focused checks needed to assess required behavior. Make no source changes and do not treat the worker report as proof. Complete is true only when every requirement is satisfied and credibly demonstrated; a passing fixed command establishes only what it actually exercises. Report concrete unmet requirements or invalid evidence, not style preferences, optional polish, or unrelated improvements.`
