package planning

import (
	"context"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Planning -name planning
const Goal = "Create planned.txt containing exactly done, with a newline. One worker assignment should suffice. After a recorded passing check, end dispatch."
const WorkPrompt = "Perform the assignment in task. Edit only planned.txt. Return a concise summary, file planned.txt, and receipt done."

// Planning hands command evidence back to a continuing planner.
func Planning(ctx context.Context, env gimbal.Env) error {
	planner := gimbal.NewSession(ctx, "coder", env.WorkDir)
	loop := gimbal.PromiseLoop(ctx, "plan", Goal, planner)
	for ctx, _ := range loop.Tasks {
		worker := gimbal.NewSession(ctx, "coder", env.WorkDir)
		result, err := worker.Generate[continuity.Report](ctx, WorkPrompt)
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "result", result)
		if err = gimbal.Check(ctx, "check", env.WorkDir, "sh", "-c", "test \"$(cat planned.txt)\" = done"); err != nil {
			return err
		}
	}
	return loop.Err()
}
