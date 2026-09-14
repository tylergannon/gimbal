package fresh

import (
	"context"

	"github.com/tylergannon/gimble"
)

func mutuallyExclusive(ctx context.Context, first bool) {
	if first {
		gimble.Set(ctx, "result", "first")
	} else {
		gimble.SetJSON(ctx, "result", gimble.Text("second"))
	}
}

func freshTaskContext(ctx context.Context) {
	loop := gimble.Loop(ctx, "tasks", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		gimble.Set(taskCtx, "result", task.Name)
	}
}
