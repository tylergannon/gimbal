// Package authored holds the ordinary Go source owned by this consumer.
package authored

import (
	"context"

	"github.com/tylergannon/gimbal"
)

// Delivery records an agent response, then checks a local deliverable.
func Delivery(ctx context.Context, env gimbal.Env) error {
	gimbal.Set(ctx, "assignment", "Create a small, checked delivery.")
	worker := gimbal.NewSession(ctx, "coder", env.WorkDir)
	summary, err := worker.Generate[gimbal.Text](ctx, "Reply with exactly one sentence describing a small deliverable ready for verification. Only provide the sentence: do not run commands, read or write files, invoke tools, start workflows, or delegate work. The caller creates and checks the deliverable after your reply.")
	if err != nil {
		return err
	}
	gimbal.Set(ctx, "summary", summary)
	err = gimbal.Scope(ctx, "verify", func(ctx context.Context) error {
		err := gimbal.Check(ctx, "delivery", env.WorkDir, "sh", "-c", "printf 'done\n' > delivery.txt && test -s delivery.txt")
		return err
	})
	return err
}
