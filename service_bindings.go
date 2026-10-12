package gimbal

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal/internal/effects"
)

// WithServices declares named service dependencies for agent turns and commands
// issued with the returned context on messaging routes. The actual producer
// acquires each service for the owning scope and executor before native admission.
// This declaration does not launch a process or establish readiness. Workflow
// source stays unchanged when the externally supplied dependency changes placement.
// Names contain lowercase letters, digits and hyphens, beginning with a letter.
// Non-messaging execution with service dependencies is unsupported.
func WithServices(ctx context.Context, names ...string) context.Context {
	return effects.WithServices(ctx, names)
}

func requireServiceMessaging(ctx context.Context) error {
	if len(effects.Services(ctx)) > 0 && effects.Messages(ctx) == nil {
		return fmt.Errorf("named services require a messaging route")
	}
	return nil
}
