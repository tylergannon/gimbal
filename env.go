package gimbal

import (
	"context"

	"github.com/tylergannon/gimbal/internal/effects"
)

// Env is the Gimbal-owned environment passed to every generated workflow.
// WorkDir is the absolute initial working directory selected for the run.
type Env struct {
	WorkDir string
}

// WithEnvironment selects the logical environment inherited by tasks created in ctx.
func WithEnvironment(ctx context.Context, name string) context.Context {
	return effects.WithEnvironment(ctx, name)
}
