// Package delivery implements a small planner driven delivery workflow.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimble"
)

//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Delivery -name delivery

// Input describes the local delivery contract.
type Input struct {
	// WorkDir is the repository in which the work and validation command run.
	WorkDir string
	// Brief names the local requirements and design specification.
	Brief string
	// Validation is the fixed shell command that must pass for delivery.
	Validation string
	// MaxTasks bounds planner assignments.
	MaxTasks int
}

// Verdict is the independent validator's assessment of the whole brief.
type Verdict struct {
	// Satisfied is true only when the validator observed the whole brief working.
	Satisfied bool `json:"satisfied"`
	// Findings lists unmet required behavior or invalid proof.
	Findings []string `json:"findings"`
}

// Delivery plans and executes the brief, then independently validates the
// resulting repository and its fixed validation command.
func Delivery(ctx context.Context, in Input) error {
	workDir, err := filepath.Abs(in.WorkDir)
	if err != nil {
		return err
	}
	if in.MaxTasks <= 0 {
		return errors.New("delivery: MaxTasks must be positive")
	}
	briefPath := in.Brief
	if !filepath.IsAbs(briefPath) {
		briefPath = filepath.Join(workDir, briefPath)
	}
	brief, err := filepath.Abs(briefPath)
	if err != nil {
		return err
	}
	briefInfo, err := os.Stat(brief)
	if err != nil {
		return fmt.Errorf("delivery: brief: %w", err)
	}
	if briefInfo.IsDir() {
		return fmt.Errorf("delivery: brief %q is a directory", brief)
	}
	if in.Validation == "" {
		return errors.New("delivery: Validation must not be empty")
	}

	gimble.Set(ctx, "brief", brief)
	gimble.Set(ctx, "validation", in.Validation)
	gimble.Set(ctx, "workdir", workDir)
	planner := gimble.NewSession(ctx, "planner", workDir)
	loop := gimble.Loop(ctx, "delivery")
	goal := "Implement the complete brief in " + brief
	count := 0
	satisfied := false
	for taskCtx := range loop.Tasks(goal, planner) {
		count++
		implementer := gimble.NewSession(taskCtx, "implementer", workDir)
		if _, err := implementer.Generate[gimble.Text](taskCtx, implementPrompt); err != nil {
			return err
		}
		exit, stdout, stderr, err := gimble.RunCommand(taskCtx, "validation", workDir, "sh", "-c", in.Validation)
		if err != nil {
			return err
		}
		gimble.Set(taskCtx, "validation_exit", exit)
		gimble.Set(taskCtx, "validation_stdout", stdout)
		gimble.Set(taskCtx, "validation_stderr", stderr)
		validator := gimble.NewSession(taskCtx, "validator", workDir)
		verdict, err := validator.Generate[Verdict](taskCtx, validatorPrompt)
		if err != nil {
			return err
		}
		gimble.SetJSON(taskCtx, "verdict", verdict)
		if exit == 0 && verdict.Satisfied {
			satisfied = true
			break
		}
		if count == in.MaxTasks {
			break
		}
	}
	if err := loop.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	if satisfied {
		return nil
	}
	if count == in.MaxTasks {
		return fmt.Errorf("delivery: task limit %d exhausted without satisfying the brief", in.MaxTasks)
	}
	return errors.New("delivery: planner ended without satisfying the brief")
}

const implementPrompt = "Implement the current task in the working directory. Read the brief and make the necessary changes. Do not edit the brief or weaken the fixed validation command. Do not claim completion without doing the work."

const validatorPrompt = "Independently inspect the whole brief and the current working directory. Run checks as needed. Check actual files and behavior against every requirement, and check that the fixed validation command is a legitimate check. Require evidence that the required behavior works. Report only unmet requirements or invalid evidence, not style preferences or requests for enhancements. Do not modify source files or weaken the check. Return satisfied=true only when the brief is complete; list concrete findings otherwise."
