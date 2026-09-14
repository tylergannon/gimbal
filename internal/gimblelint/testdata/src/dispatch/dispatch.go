package dispatch

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

type worker func(context.Context) error

var globalWorkers = map[string]worker{"implement": implement}

func implement(context.Context) error { return nil }
func review(context.Context) error    { return nil }

func directMap(ctx context.Context, kind string) error {
	workers := map[string]worker{"implement": implement, "review": review}
	return workers[kind](ctx) // want `GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS`
}

func aliasedMap(ctx context.Context, kind string) error {
	workers := map[string]worker{"implement": implement, "review": review}
	selected := workers[kind]
	alias := selected
	return alias(ctx) // want `GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS`
}

func opaque(ctx context.Context, selected worker) error {
	return selected(ctx) // want `GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS`
}

func singleTargetAlias(ctx context.Context) error {
	selected := implement
	return selected(ctx)
}

func explicitSwitch(ctx context.Context, kind string) error {
	switch kind {
	case "implement":
		return implement(ctx)
	case "review":
		return review(ctx)
	default:
		return fmt.Errorf("unsupported task kind %q", kind)
	}
}

func helper(ctx context.Context) error { return implement(ctx) }

func namedBody(ctx context.Context) error {
	workers := map[string]worker{"implement": implement}
	return workers["implement"](ctx) // want `GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS`
}

func namedWorkflow(ctx context.Context) error {
	return gimble.Run(ctx, "named", namedBody)
}

func helperWithoutContext() error {
	return globalWorkers["implement"](context.Background()) // want `GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS`
}

func workflow(ctx context.Context, kind string) error {
	return gimble.Run(ctx, "workflow", func(ctx context.Context) error {
		if err := directMap(ctx, kind); err != nil {
			return err
		}
		if err := aliasedMap(ctx, kind); err != nil {
			return err
		}
		if err := opaque(ctx, implement); err != nil {
			return err
		}
		if err := explicitSwitch(ctx, kind); err != nil {
			return err
		}
		if err := singleTargetAlias(ctx); err != nil {
			return err
		}
		if err := helperWithoutContext(); err != nil {
			return err
		}
		return helper(ctx)
	})
}
