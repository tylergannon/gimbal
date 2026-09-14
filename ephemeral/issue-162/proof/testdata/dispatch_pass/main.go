package main

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

type worker func(context.Context) error

func implement(context.Context) error { fmt.Println("switch: implement"); return nil }
func review(context.Context) error    { fmt.Println("helper: review"); return nil }
func alias(context.Context) error     { fmt.Println("alias: implement"); return nil }

func helper(ctx context.Context) error { return review(ctx) }

func groupShape(ctx context.Context) error {
	group := gimble.Group(ctx, "workers")
	group.Go("implement", func(ctx context.Context) error { return implement(ctx) })
	return group.Wait()
}

func workflow(ctx context.Context, kind string) error {
	switch kind {
	case "implement":
		if err := implement(ctx); err != nil {
			return err
		}
	case "review":
		if err := review(ctx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported task kind %q", kind)
	}
	if err := helper(ctx); err != nil {
		return err
	}
	singleTarget := alias
	if err := singleTarget(ctx); err != nil {
		return err
	}
	if false {
		return groupShape(ctx)
	}
	return nil
}

func markWorkflow(ctx context.Context) error {
	return gimble.Run(ctx, "dispatch", func(ctx context.Context) error {
		return workflow(ctx, "implement")
	})
}

func main() {
	if err := workflow(context.Background(), "implement"); err != nil {
		panic(err)
	}
}
