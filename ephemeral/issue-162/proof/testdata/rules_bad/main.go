package main

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

type worker func(context.Context) error

func work(context.Context) error { return nil }

func violations(parent context.Context, runtimeKey string) error {
	gimble.Set(parent, runtimeKey, "dynamic key")
	gimble.Set(parent, "duplicate", "first")
	gimble.SetJSON(parent, "duplicate", gimble.Text("second"))
	for range 2 {
		gimble.Set(parent, "loop duplicate", "value")
	}
	gimble.Set(context.Background(), "background", "value")

	if err := gimble.Scope(parent, "child", func(child context.Context) error {
		gimble.Set(parent, "wrong context", "value")
		go func() { gimble.Set(child, "raw go", "value") }()
		return nil
	}); err != nil {
		return err
	}

	loop := gimble.Loop(parent, "tasks", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		gimble.Set(taskCtx, "task", task.Name)
	}

	workers := map[string]worker{"work": work}
	return workers[fmt.Sprint("work")](parent)
}

func markWorkflow(ctx context.Context) error {
	return gimble.Run(ctx, "bad", func(ctx context.Context) error {
		return violations(ctx, "runtime")
	})
}

func main() {}
