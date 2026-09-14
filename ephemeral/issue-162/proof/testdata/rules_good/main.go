package main

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

const namedKey = "named"
const expressionKey = "constant-" + "expression"

func implement(context.Context) error { return nil }
func review(context.Context) error    { return nil }

func workflow(ctx context.Context, kind string, dynamicValue any) error {
	gimble.Set(ctx, "literal", fmt.Sprint(dynamicValue))
	gimble.Set(ctx, namedKey, "value")
	gimble.SetJSON(ctx, expressionKey, gimble.Text(fmt.Sprint(dynamicValue)))
	if kind == "left" {
		gimble.Set(ctx, "one branch", "left")
	} else {
		gimble.Set(ctx, "one branch", "right")
	}

	loop := gimble.Loop(ctx, "tasks", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		gimble.Set(taskCtx, "result", task.Name)
	}

	group := gimble.Group(ctx, "workers")
	group.Go("implement", func(child context.Context) error { return implement(child) })
	if err := group.Wait(); err != nil {
		return err
	}

	switch kind {
	case "implement":
		return implement(ctx)
	case "review":
		return review(ctx)
	default:
		singleTarget := implement
		return singleTarget(ctx)
	}
}

func markWorkflow(ctx context.Context) error {
	return gimble.Run(ctx, "good", func(ctx context.Context) error {
		return workflow(ctx, "implement", "runtime")
	})
}

func main() {}
