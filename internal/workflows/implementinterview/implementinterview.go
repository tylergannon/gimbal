// Package implementinterview builds and demonstrates Gimbal issue 249.
package implementinterview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ImplementInterview -name implement-interview

const (
	roleAPIResearch               gimbal.WorkflowRole = "api-research"
	roleFrontendResearch          gimbal.WorkflowRole = "frontend-research"
	roleCoding                    gimbal.WorkflowRole = "coding"
	roleImplementationScopeReview gimbal.WorkflowRole = "implementation-scope-review"
)

// InterviewBuildParams are the local inputs and task bound for the build.
type InterviewBuildParams struct {
	// RequirementsFile is the absolute path to the locally saved issue 249.
	RequirementsFile string
	// ReferenceDir is the absolute untracked directory where researchers write their notes and downloaded primary sources.
	ReferenceDir string
	// MaxTasks is the maximum number of planner assignments the workflow may run.
	MaxTasks int
}

// Assessment is an independent validator's judgment of the implementation.
type Assessment struct {
	// Complete is true only after the full issue requirements were demonstrated through the real running browser interface.
	Complete bool `json:"complete"`
	// Observed states what the validator personally saw in the code, checks, and live browser interaction.
	Observed string `json:"observed"`
	// UnmetRequirements lists each issue requirement that was not demonstrated; use an empty list only when Complete is true.
	UnmetRequirements []string `json:"unmet_requirements"`
}

// ImplementInterview researches, implements, and independently validates issue 249.
func ImplementInterview(ctx context.Context, env gimbal.Env, params InterviewBuildParams) error {
	if !filepath.IsAbs(params.RequirementsFile) || !filepath.IsAbs(params.ReferenceDir) {
		return fmt.Errorf("requirements-file and reference-dir must be absolute paths")
	}
	if params.MaxTasks < 1 {
		return fmt.Errorf("max-tasks must be at least 1")
	}
	if _, err := os.Stat(params.RequirementsFile); err != nil {
		return fmt.Errorf("requirements file: %w", err)
	}
	if info, err := os.Stat(params.ReferenceDir); err != nil || !info.IsDir() {
		return fmt.Errorf("reference directory is not a directory: %s", params.ReferenceDir)
	}

	gimbal.Set(ctx, "requirements-file", params.RequirementsFile)
	gimbal.Set(ctx, "reference-directory", params.ReferenceDir)
	gimbal.Set(ctx, "repository", env.WorkDir)

	recon := gimbal.Group(ctx, "reconnaissance")
	recon.Go("backend", func(ctx context.Context) error {
		researcher := gimbal.NewSession(ctx, roleAPIResearch, env.WorkDir)
		_, err := researcher.Generate[gimbal.Text](ctx, backendResearchPrompt)
		return err
	})
	recon.Go("frontend", func(ctx context.Context) error {
		researcher := gimbal.NewSession(ctx, roleFrontendResearch, env.WorkDir)
		_, err := researcher.Generate[gimbal.Text](ctx, frontendResearchPrompt)
		return err
	})
	if err := recon.Wait(); err != nil {
		return err
	}

	planner := gimbal.NewSession(ctx, gimbal.RoleSprintPlanning, env.WorkDir)
	coder := gimbal.NewSession(ctx, roleCoding, env.WorkDir)
	backendScope := gimbal.NewSession(ctx, roleImplementationScopeReview, env.WorkDir)
	architectureScope := gimbal.NewSession(ctx, gimbal.RoleArchitecturalCritique, env.WorkDir)
	goal := "Implement exactly the locally saved Gimbal issue 249 at " + params.RequirementsFile +
		" and demonstrate its complete definition of done through the real running web interface."
	loop := gimbal.PromiseLoop(ctx, "implementation", goal, planner)

	completed := false
	exhausted := false
	var operationalErr error
	tasksRun := 0
	for taskCtx, task := range loop.Tasks {
		tasksRun++
		workerReport, err := coder.Generate[gimbal.Text](taskCtx, codingPrompt,
			gimbal.WithSupervisor(backendScope, backendScopePrompt),
			gimbal.WithSupervisor(architectureScope, architectureScopePrompt),
		)
		if err != nil {
			operationalErr = err
			break
		}

		taskCheckPassed := true
		if command := strings.TrimSpace(task.Validation.Command); command != "" {
			exit, stdout, stderr, err := gimbal.RunCommand(taskCtx, "task-check", env.WorkDir, "sh", "-lc", command)
			gimbal.Set(taskCtx, "task check", commandResult(exit, stdout, stderr))
			if err != nil {
				operationalErr = err
				break
			}
			taskCheckPassed = exit == 0
		}

		buildExit, stdout, stderr, err := gimbal.RunCommand(taskCtx, "build", env.WorkDir, "just", "build")
		gimbal.Set(taskCtx, "just build", commandResult(buildExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}
		vetExit, stdout, stderr, err := gimbal.RunCommand(taskCtx, "vet", env.WorkDir, "just", "vet")
		gimbal.Set(taskCtx, "just vet", commandResult(vetExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}
		testExit, stdout, stderr, err := gimbal.RunCommand(taskCtx, "test", env.WorkDir, "just", "test")
		gimbal.Set(taskCtx, "just test", commandResult(testExit, stdout, stderr))
		if err != nil {
			operationalErr = err
			break
		}

		validator := gimbal.NewSession(taskCtx, gimbal.RoleQAOrchestration, env.WorkDir)
		assessment, err := validator.Generate[Assessment](taskCtx, validationPrompt)
		if err != nil {
			operationalErr = err
			break
		}
		gimbal.SetJSON(taskCtx, "independent assessment", assessment)
		gimbal.Set(taskCtx, "worker report", string(workerReport))
		checksPassed := taskCheckPassed && buildExit == 0 && vetExit == 0 && testExit == 0
		gimbal.Set(taskCtx, "deterministic checks passed", checksPassed)
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

func commandResult(exit int, stdout, stderr string) string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", exit, stdout, stderr)
}

const backendResearchPrompt = "Read the saved issue, current Go API and design docs, pinned dependencies, runtime/events/generation code, Polytype use, and skgo Go remote-function APIs. Do not edit source. Under the reference directory, create backend/index.md with deep implementation notes and provenance, and save useful official source documentation under backend/. Distinguish verified facts from proposals."

const frontendResearchPrompt = "Read the saved issue, current live-run UI, pinned frontend dependencies, and installed shadcn-svelte components. Research the actual Svelte, SvelteKit, shadcn-svelte, and Bits UI APIs needed by the issue. Do not edit source. Under the reference directory, create frontend/index.md with deep implementation notes and provenance, and save useful official source documentation under frontend/. Distinguish verified facts from proposals."

const codingPrompt = "Read the saved issue and the local reference directory, then implement only the selected task in the repository. Do not expand the specification, modify the requirements or this build workflow, weaken its fixed checks, commit, merge, or add proof scripts or run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed."

const backendScopePrompt = "Watch only for concrete backend or public-API work beyond the saved issue 249. Steer against unnecessary wrappers, frameworks, speculative APIs, and unrelated features. Do not edit source, object on style, or demand improvements outside the requirements."

const architectureScopePrompt = "Watch only for concrete frontend or general implementation work beyond the saved issue 249. Steer against unnecessary abstractions, frameworks, speculative features, and unrelated polish. Do not edit source, object on style, or demand improvements outside the requirements."

const validationPrompt = "Independently read the saved issue, reference directory, implementation, tests, and recorded command results. Make no source changes and do not treat the worker report as evidence. Start the generated interview example on a separate port with its interviewer bound to Terra or Sonnet, never Gemini. Use the installed Playwright through the shell to conduct successive human answers in a real browser; do not write proof programs or commit run artifacts, and cleanly stop the example process afterward. Complete is true only if you personally demonstrate every requirement in that running interface; tests and reports alone are insufficient."
