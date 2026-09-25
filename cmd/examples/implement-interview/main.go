// Command implement-interview runs the issue 249 example in this process.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/workflows/implementinterview"
)

func main() {
	projectFlag := flag.String("project", ".", "project directory for the standalone run")
	workDirFlag := flag.String("work-dir", "", "execution directory (default: project)")
	requirementsFile := flag.String("requirements-file", "", "absolute path to the locally saved requirements file (required)")
	referenceDir := flag.String("reference-dir", "", "absolute path to the research directory (required)")
	maxTasks := flag.Int("max-tasks", 0, "maximum planner assignments (required, at least 1)")
	apiResearch := flag.String("api-research", "gpt-5.6-terra:high", "model for API research")
	frontendResearch := flag.String("frontend-research", "claude-sonnet-5:high", "model for frontend research")
	planning := flag.String("sprint-planning", "claude-opus-5-5:max", "model for sprint planning")
	coding := flag.String("coding", "gpt-6-sol:xhigh", "model for coding")
	scopeReview := flag.String("implementation-scope-review", "gpt-5.6-terra:high", "model for implementation scope review")
	architecture := flag.String("architectural-critique", "claude-sonnet-5:high", "model for architectural critique")
	qa := flag.String("qa-orchestration", "claude-opus-5-5:max", "model for QA orchestration")
	flag.Parse()
	if *requirementsFile == "" || *referenceDir == "" || *maxTasks < 1 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "implement-interview: give --requirements-file, --reference-dir, --max-tasks (at least 1), and no positional arguments")
		os.Exit(2)
	}
	project, err := filepath.Abs(*projectFlag)
	if err == nil {
		workDir := *workDirFlag
		if workDir == "" {
			workDir = project
		}
		workDir, err = filepath.Abs(workDir)
		if err == nil {
			var models map[gimbal.WorkflowRole]gimbal.ModelBinding
			models, err = binding.Roles(map[gimbal.WorkflowRole]string{
				"api-research": *apiResearch, "frontend-research": *frontendResearch,
				gimbal.RoleSprintPlanning: *planning, "coding": *coding,
				"implementation-scope-review": *scopeReview, gimbal.RoleArchitecturalCritique: *architecture,
				gimbal.RoleQAOrchestration: *qa,
			})
			if err == nil {
				ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
				defer stop()
				err = gimbal.Run(gimbal.Project(ctx, project), "implement-interview", models, func(ctx context.Context) error {
					return implementinterview.ImplementInterview(ctx, gimbal.Env{WorkDir: workDir}, implementinterview.InterviewBuildParams{
						RequirementsFile: *requirementsFile, ReferenceDir: *referenceDir, MaxTasks: *maxTasks,
					})
				})
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "implement-interview:", err)
		os.Exit(1)
	}
}
