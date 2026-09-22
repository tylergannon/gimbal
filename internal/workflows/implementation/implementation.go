// Package implementation fulfills a promise through a bounded,
// planner-directed loop and independently validates the result.
//
// The caller supplies the promise and a local definition-of-done file. On each
// lap, a planner chooses the next coherent assignment and gives it its own
// definition of done. A fresh coding agent implements that assignment. The
// planner may request a command as task evidence; its result is recorded, but
// a passing check is never validation by itself.
//
// The workflow does not commit, push, merge, deploy, or rewrite the
// definition of done. After each task, a fresh independent validator judges
// both that task's definition of done and the overall promise against actual
// evidence and observed behavior. The loop exits when validation passes at
// 90–95% completion with only small gaps. Those gaps are recorded for later
// and must not cause another lap. Only a substantial unmet requirement permits
// replanning, until the task limit is exhausted or the planner stops honestly.
//
// Model cost guidance: the configured defaults use Astra for sprint-planning
// and architectural-critique, while coding and independent validation already
// use Sol. Keep Astra for ambiguous, cross-cutting, or architecture-heavy
// changes. For a small, local, well-specified change, deliberately tune the two
// Astra roles down to Sol or Opus:
//
//	--sprint-planning gpt-6-sol:high \
//	--architectural-critique claude-opus-5-5:high
//
// For low-risk background implementation where elapsed time and rejected
// validation laps are cheap, keep the frontier sprint-planning default and use
// Terra and Sonnet for most repeated work:
//
//	--coding gpt-5.6-terra:high \
//	--architectural-critique claude-sonnet-5:high \
//	--qa-orchestration claude-sonnet-5:high
//
// A failed validation can send substantial gaps back to the frontier planner.
// Provider, harness, and execution errors still end the run rather than
// retrying.
//
// Example:
//
//	gimble run implement \
//	  --promise "Ship the local issue without unrelated changes" \
//	  --definition-of-done-file ./issue.md \
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
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Implement -name implement -mermaid ../../../docs-site/src/lib/generated/workflows/implement.mmd

const roleCoding gimble.WorkflowRole = "coding"

// Params identify the promise, its local definition of done, and the task bound.
type Params struct {
	// Promise is the outcome the implementation loop must fulfill.
	Promise string
	// DefinitionOfDoneFile is the local file the validator uses to judge fulfillment.
	DefinitionOfDoneFile string
	// MaxTasks is the maximum number of planner assignments the run may execute.
	MaxTasks int
}

// Assessment is an independent validator's bounded completion judgment.
type Assessment struct {
	// ValidationPassed is true when the promise and definition of done hold at 90–95% completion with no substantial gap.
	ValidationPassed bool `json:"validation_passed"`
	// Observed states what the validator personally inspected or ran.
	Observed string `json:"observed"`
	// SubstantialGaps lists unmet requirements that justify another implementation lap.
	SubstantialGaps []string `json:"substantial_gaps"`
	// SmallGaps lists optional finishing work that must not block exit or trigger another lap.
	SmallGaps []string `json:"small_gaps"`
}

// Implement modifies a repository until its promise is independently demonstrated.
func Implement(ctx context.Context, env gimble.Env, params Params) error {
	if params.MaxTasks < 1 {
		return fmt.Errorf("max-tasks must be at least 1")
	}
	promise := strings.TrimSpace(params.Promise)
	if promise == "" {
		return fmt.Errorf("promise must not be blank")
	}
	definitionOfDoneFile, err := absoluteFrom(env.WorkDir, params.DefinitionOfDoneFile)
	if err != nil {
		return fmt.Errorf("definition-of-done-file: %w", err)
	}
	info, err := os.Stat(definitionOfDoneFile)
	if err != nil {
		return fmt.Errorf("definition of done file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("definition of done file is not a regular file: %s", definitionOfDoneFile)
	}

	gimble.Set(ctx, "promise", promise)
	gimble.Set(ctx, "definition of done file", definitionOfDoneFile)
	gimble.Set(ctx, "repository", env.WorkDir)
	gimble.Set(ctx, "maximum tasks", params.MaxTasks)
	gimble.Set(ctx, "completion rule", completionRule)

	planner := gimble.NewSession(ctx, gimble.RoleSprintPlanning, env.WorkDir)
	plannerCoach := gimble.NewSession(ctx, gimble.RoleArchitecturalCritique, env.WorkDir)
	loop := gimble.PromiseLoop(ctx, "implementation", promise, planner,
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

		if command := strings.TrimSpace(task.Validation.Command); command != "" {
			if err := gimble.Check(taskCtx, "task check", env.WorkDir, "sh", "-lc", command); err != nil {
				operationalErr = err
				break
			}
		}

		validator := gimble.NewSession(taskCtx, gimble.RoleQAOrchestration, env.WorkDir)
		assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt)
		if err != nil {
			operationalErr = err
			break
		}
		gimble.SetJSON(taskCtx, "independent assessment", assessment)
		if assessment.ValidationPassed {
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
	return fmt.Errorf("implementation incomplete: planner stopped before the promise was demonstrated")
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

const completionRule = `Exit when independent validation passes with the promise and definition of done 90–95% complete and only small gaps remaining. Record those small gaps for later; never take another lap to polish the last 5%. Replan only for substantial unmet requirements or invalid evidence.`

const planningScopePrompt = `Keep the plan inside the promise and its definition-of-done file. Every selected task needs a concrete definition of done that advances the promise. Object to invented features, speculative infrastructure, polishing, or unrelated repairs. Preserve failed checks and substantial validator findings as evidence for replanning. When validation passes at 90–95% with only small gaps, end immediately; never plan work for the last 5%.`

const implementationScopePrompt = `Watch only for concrete work beyond the selected assignment, the promise, or the definition-of-done file. Object to invented features, speculative abstractions, edits to the definition of done, unrelated cleanup, and polishing toward 100%. Do not edit source or demand optional finishing work once the selected result is at 90–95%.`

const implementationPrompt = `Read the promise, the definition-of-done file, and the selected task with its definition of done. Implement only that task in the repository. Make the actual source changes and gather useful evidence. Do not edit the definition of done or this workflow; do not commit, push, merge, deploy, or add proof scripts and run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed.`

const validationPrompt = `Independently validate the selected task and the overall promise. Read the promise, the supplied definition-of-done file, the task's own definition of done, and the completion rule. Inspect the actual repository and recorded evidence, then personally observe any behavior needed by the definitions of done that checks do not establish. Make no source changes and do not treat the worker report as proof. Unit tests and other commands are evidence, never validation by themselves. ValidationPassed is true when the promise and definition of done hold at 90–95% with no substantial gap. Put remaining optional finishing work in SmallGaps; small gaps must not make validation fail or trigger another lap. Set ValidationPassed false only for a substantial unmet requirement or invalid evidence, and list each reason in SubstantialGaps. Do not demand 100%, style preferences, polish, or unrelated improvements.`
