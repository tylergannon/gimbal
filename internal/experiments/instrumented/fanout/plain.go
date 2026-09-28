package fanout

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/continuity"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Fanout -name fanout

const Prompt = "Read assignment. Sleep for its delay seconds, then write its receipt to its file. Return file equal to that filename, receipt equal to that receipt, and a short summary. Edit only that file; other agents share this checkout."

type Work struct {
	File, Receipt string
	Delay         int
}
type Params struct{ ItemsJSON string }

// Fanout collects independently assigned results after every child has joined.
func Fanout(ctx context.Context, env gimbal.Env, in Params) error {
	var items []Work
	if err := json.Unmarshal([]byte(in.ItemsJSON), &items); err != nil {
		return err
	}
	if len(items) != 2 {
		return fmt.Errorf("expected assignments for exactly two authored branches")
	}
	var results [2]continuity.Report
	group := gimbal.Group(ctx, "workers")
	group.Go("left", func(ctx context.Context) error {
		item := items[0]
		gimbal.Set(ctx, "assignment", fmt.Sprintf("file=%s receipt=%s delay=%d", item.File, item.Receipt, item.Delay))
		agent := gimbal.NewSession(ctx, "coder", env.WorkDir)
		result, err := agent.Generate[continuity.Report](ctx, Prompt)
		results[0] = result
		return err
	})
	group.Go("right", func(ctx context.Context) error {
		item := items[1]
		gimbal.Set(ctx, "assignment", fmt.Sprintf("file=%s receipt=%s delay=%d", item.File, item.Receipt, item.Delay))
		agent := gimbal.NewSession(ctx, "coder", env.WorkDir)
		result, err := agent.Generate[continuity.Report](ctx, Prompt)
		results[1] = result
		return err
	})
	if err := group.Wait(); err != nil {
		return err
	}
	for i, result := range results {
		if result.Receipt != items[i].Receipt || result.File != items[i].File {
			return fmt.Errorf("misassigned result %d", i)
		}
	}
	return nil
}
