package main

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

type worker func(context.Context) error

func implement(context.Context) error { fmt.Println("direct: implement"); return nil }
func review(context.Context) error    { fmt.Println("alias: review"); return nil }

func workflow(ctx context.Context, kind string) error {
	workers := map[string]worker{"implement": implement, "review": review}
	if err := workers[kind](ctx); err != nil {
		return err
	}
	selected := workers["review"]
	alias := selected
	return alias(ctx)
}

// markWorkflow gives the analyzer the real Gimble callback boundary. The
// deterministic executable calls the same callback directly, without needing
// a live run store.
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
