package effects

import (
	"context"

	"github.com/tylergannon/gimbal/backend"
)

type taskFilesKey struct{}

// WithTaskFiles carries runtime-collected inputs to the execution bridge.
func WithTaskFiles(ctx context.Context, files []backend.TaskFile) context.Context {
	return context.WithValue(ctx, taskFilesKey{}, files)
}

// TaskFiles returns the runtime-collected inputs for this turn.
func TaskFiles(ctx context.Context) []backend.TaskFile {
	files, _ := ctx.Value(taskFilesKey{}).([]backend.TaskFile)
	return files
}
