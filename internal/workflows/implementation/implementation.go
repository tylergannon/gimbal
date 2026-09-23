// Package implementation works through an ordered list of outcomes. Each
// outcome has its own bounded PromiseLoop and independent validation; there is
// no planner deciding which outcome comes next.
//
// The caller supplies a local JSON file containing an array of outcome
// strings. The workflow takes them in file order. Within an outcome, a planner
// may choose another task only when validation finds a substantial gap. A
// passing judgment at 90–95% with only small gaps advances immediately to the
// next outcome. An incomplete outcome stops the run; later outcomes do not
// start.
//
// Scope supervisors watch the planner, coder, and validator for unnecessary
// complexity, gold-plating, and work outside the selected outcome. They steer
// but never gate completion. The workflow changes the working tree but does
// not commit, push, merge, deploy, or edit the outcomes file. Checks gather
// evidence; the independent validator judges whether the behavior was seen.
//
// Example outcomes.json:
//
//	["The CLI starts work in the selected instance", "The page observes that work live"]
//
// Example invocation:
//
//	gimble run implement --outcomes-file ./outcomes.json --max-tasks-per-outcome 3
package implementation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Implement -name implement -command ../../../cmd/gimble/implementation_gen.go -mermaid ../../../docs-site/src/lib/generated/workflows/implement.mmd

const roleCoding gimble.WorkflowRole = "coding"

// Params identify the ordered outcomes and the per-outcome task bound.
type Params struct {
	// OutcomesFile is a local JSON array of outcome strings, in execution order.
	OutcomesFile string
	// MaxTasksPerOutcome bounds planner assignments for each outcome.
	MaxTasksPerOutcome int
}

// Assessment is an independent validator's bounded completion judgment.
type Assessment struct {
	// ValidationPassed is true when the current outcome holds at 90–95% completion with no substantial gap.
	ValidationPassed bool `json:"validation_passed"`
	// Observed states what the validator personally inspected or ran.
	Observed string `json:"observed"`
	// SubstantialGaps lists unmet requirements that justify another task in this outcome.
	SubstantialGaps []string `json:"substantial_gaps"`
	// SmallGaps lists optional finishing work that must not block this outcome or the next.
	SmallGaps []string `json:"small_gaps"`
}

// Implement works through supplied outcomes until each is demonstrated.
func Implement(ctx context.Context, env gimble.Env, params Params) error {
	if params.MaxTasksPerOutcome < 1 {
		return fmt.Errorf("max-tasks-per-outcome must be at least 1")
	}
	outcomesFile, err := absoluteFrom(env.WorkDir, params.OutcomesFile)
	if err != nil {
		return fmt.Errorf("outcomes-file: %w", err)
	}
	raw, err := os.ReadFile(outcomesFile)
	if err != nil {
		return fmt.Errorf("outcomes file: %w", err)
	}
	var outcomes []string
	if err := json.Unmarshal(raw, &outcomes); err != nil {
		return fmt.Errorf("outcomes file must be a JSON array of strings: %w", err)
	}
	if len(outcomes) == 0 {
		return fmt.Errorf("outcomes file must contain at least one outcome")
	}
	for i, outcome := range outcomes {
		if strings.TrimSpace(outcome) == "" {
			return fmt.Errorf("outcome %d is blank", i+1)
		}
	}

	gimble.Set(ctx, "outcomes file", outcomesFile)
	gimble.Set(ctx, "repository", env.WorkDir)
	gimble.Set(ctx, "maximum tasks per outcome", fmt.Sprint(params.MaxTasksPerOutcome))
	gimble.Set(ctx, "completion rule", completionRule)

	outcomeNumber := 0
	for outcomeCtx, outcome := range gimble.Iterate(ctx, "outcome", outcomes) {
		outcomeNumber++
		gimble.Set(outcomeCtx, "outcome", outcome)
		planner := gimble.NewSession(outcomeCtx, gimble.RoleSprintPlanning, env.WorkDir)
		plannerCoach := gimble.NewSession(outcomeCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
		loop := gimble.PromiseLoop(outcomeCtx, "implementation", outcome, planner,
			gimble.WithSupervisor(plannerCoach, scopePrompt))

		completed := false
		tasksRun := 0
		var operationalErr error
		for taskCtx, task := range loop.Tasks {
			tasksRun++
			worker := gimble.NewSession(taskCtx, roleCoding, env.WorkDir)
			workerCoach := gimble.NewSession(taskCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
			workerReport, err := worker.Generate[gimble.Text](taskCtx, implementationPrompt,
				gimble.WithSupervisor(workerCoach, scopePrompt))
			if err != nil {
				operationalErr = err
				break
			}
			gimble.Set(taskCtx, "worker report", string(workerReport))

			if command := strings.TrimSpace(task.Validation.Command); command != "" {
				if err := gimble.Check(taskCtx, "task check", env.WorkDir, "sh", "-lc", command); err != nil {
					operationalErr = err
					break
				}
			}

			validator := gimble.NewSession(taskCtx, gimble.RoleQAOrchestration, env.WorkDir)
			validatorCoach := gimble.NewSession(taskCtx, gimble.RoleArchitecturalCritique, env.WorkDir)
			assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt,
				gimble.WithSupervisor(validatorCoach, scopePrompt))
			if err != nil {
				operationalErr = err
				break
			}
			gimble.SetJSON(taskCtx, "independent assessment", assessment)
			if assessment.ValidationPassed && len(assessment.SubstantialGaps) == 0 {
				completed = true
				break
			}
			if tasksRun >= params.MaxTasksPerOutcome {
				break
			}
		}
		if err := loop.Err(); err != nil {
			return fmt.Errorf("outcome %d: %w", outcomeNumber, err)
		}
		if operationalErr != nil {
			return fmt.Errorf("outcome %d: %w", outcomeNumber, operationalErr)
		}
		if !completed {
			return fmt.Errorf("outcome %d incomplete after %d tasks", outcomeNumber, tasksRun)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
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

const completionRule = `Advance to the next outcome when independent validation sees the current outcome working at 90–95% with only small gaps. Never take another lap to polish the last 5%. A substantial unmet requirement may justify another task within the current outcome; do not start later outcomes while it remains unmet.`

const scopePrompt = `Steer only on concrete over-engineering, gold-plating, unrequested behavior, or work beyond the selected outcome. Do not edit source, create new requirements, or treat style preferences and optional polish as blockers.`

const implementationPrompt = `Read the selected outcome and task with its definition of done. Implement only that task in the repository and gather useful evidence. Do not edit the outcomes file or this workflow; do not commit, push, merge, deploy, or add proof scripts or run output. Preserve unrelated work. Report what changed and what you personally observed.`

const validationPrompt = `Independently validate the selected task and current outcome. Inspect the repository and recorded evidence, then personally observe behavior that checks alone do not establish. Make no source changes and do not treat the worker report as proof. ValidationPassed is true when the outcome holds at 90–95% with no substantial gap. List optional polish in SmallGaps; list only genuinely unmet requirements or invalid evidence in SubstantialGaps. Do not demand 100% or work from later outcomes.`
