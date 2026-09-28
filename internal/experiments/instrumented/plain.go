// Package main is a hand-written specimen of Temporal compiler output.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Plain -name instrumented

const coder gimbal.WorkflowRole = "coder"
const coach gimbal.WorkflowRole = "coach"
const reportPrompt = "Inspect the task and actual check result supplied in context. Read marker.txt in your working directory to confirm that the earlier command's filesystem changes remain available. Return a concise report of the evidence. Make no changes."
const coachPrompt = "Check that the report's conclusions follow from the available evidence."

type Params struct{ Task string }
type Checks struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}
type Report struct {
	Summary string `json:"summary"`
	Marker  string `json:"marker"`
}

// Plain is the ordinary source counterpart; Temporal does not execute this body.
func Plain(ctx context.Context, env gimbal.Env, in Params) error {
	gimbal.Set(ctx, "task", in.Task)
	return gimbal.Scope(ctx, "review", func(ctx context.Context) error {
		principal := gimbal.NewSession(ctx, coder, env.WorkDir)
		supervisor := gimbal.NewSession(ctx, coach, env.WorkDir)
		code, stdout, stderr, err := gimbal.RunCommand(ctx, "tests", env.WorkDir, "sh", "-c", "printf 'workspace-preserved\\n' > marker.txt; cat marker.txt")
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("checks exited %d: %s", code, stderr)
		}
		gimbal.SetJSON(ctx, "checks", Checks{code, stdout, stderr})
		_, err = principal.Generate[Report](ctx, reportPrompt, gimbal.WithSupervisor(supervisor, coachPrompt, gimbal.WithInterval(10*time.Second)))
		return err
	})
}
