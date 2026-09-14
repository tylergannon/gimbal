package setchecks

import (
	"context"

	"github.com/tylergannon/gimble"
)

func duplicate(ctx context.Context) {
	gimble.Set(ctx, "role", "first")
	gimble.SetJSON(ctx, "role", gimble.Text("second"))
}

func repeating(ctx context.Context) {
	for i := 0; i < 2; i++ {
		gimble.Set(ctx, "role", i)
	}
}

func outerTaskContext(ctx context.Context) {
	loop := gimble.Loop(ctx, "tasks", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		_ = taskCtx
		gimble.Set(ctx, "result", task.Name)
	}
}
