# Go POC workflow source snapshot

Historical source from `program/workflows/workflows.go` at
`4906a98c44930763597ebe015d5b996c120bc1eb`, preserved without code changes.
The `tractor` import path belongs to that pre-rename branch. This is an
inspection copy of the unmerged POC, not code compiled by current Gimbal.
See [POC recovery](POC-RECOVERY.md) for the full library and status.

```go
package workflows

import (
	"context"
	"fmt"

	"github.com/tylergannon/tractor/program"
)

// SprintExecute implements and reviews each sprint until its checks pass.
func SprintExecute(ctx context.Context, runtime *program.Runtime, input SprintExecuteInput) error {
	run, err := prepareSprint(runtime, input)
	if err != nil {
		return err
	}
	return workLedger(ctx, runtime, run)
}

// ChapterLoop nests a sprint loop within each chapter.
func ChapterLoop(ctx context.Context, runtime *program.Runtime, input ChapterLoopInput) error {
	path, options, err := prepareChapter(runtime, input)
	if err != nil {
		return err
	}
	for chapter, err := range program.Loop(ctx, path, options) {
		if err != nil {
			return fmt.Errorf("chapter loop: %w", err)
		}
		sprints, err := prepareChapterSprint(runtime, input, chapter)
		if err != nil {
			return err
		}
		if err := workLedger(ctx, runtime, sprints); err != nil {
			return fmt.Errorf("chapter %q: %w", chapter.Item.Name, err)
		}
	}
	return nil
}

// DeliveryLoop plans, critiques, updates, then implements and reviews the plan.
func DeliveryLoop(ctx context.Context, runtime *program.Runtime, input DeliveryLoopInput) error {
	run, err := prepareDelivery(runtime, input)
	if err != nil {
		return err
	}
	if err := turn(ctx, runtime, "plan", input.PlanModel, planPrompt(input.Goal, run.Checklist)); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	if err := turn(ctx, runtime, "plan-critique", input.CritiqueModel, critiquePrompt(input.Goal, run.Checklist)); err != nil {
		return fmt.Errorf("plan critique: %w", err)
	}
	if err := turn(ctx, runtime, "plan-update", input.PlanModel, updatePrompt(input.Goal, run.Checklist)); err != nil {
		return fmt.Errorf("plan update: %w", err)
	}
	return workLedger(ctx, runtime, run)
}

func workLedger(ctx context.Context, runtime *program.Runtime, run ledgerRun) error {
	for iteration, err := range program.Loop(ctx, run.Checklist, program.LoopOptions{
		Validate:      runtime.Validate,
		Evaluate:      evaluator(runtime, run.EvaluateModel, run.Goal, "checklist"),
		MaxIterations: run.MaxIterations,
	}) {
		if err != nil {
			return err
		}
		if err := implement(ctx, runtime, run, iteration); err != nil {
			return fmt.Errorf("implement %q: %w", iteration.Item.Name, err)
		}
		verdict, err := review(ctx, runtime, run, iteration)
		if err != nil {
			return fmt.Errorf("review %q: %w", iteration.Item.Name, err)
		}
		for repairs := 0; verdict.MaterialDefect; repairs++ {
			if repairs >= 12 {
				return fmt.Errorf("review %q: reached 12 material-defect repairs", iteration.Item.Name)
			}
			if err := repair(ctx, runtime, run, iteration, verdict.Notes); err != nil {
				return fmt.Errorf("repair %q: %w", iteration.Item.Name, err)
			}
			verdict, err = review(ctx, runtime, run, iteration)
			if err != nil {
				return fmt.Errorf("review repair %q: %w", iteration.Item.Name, err)
			}
		}
	}
	return nil
}
```
