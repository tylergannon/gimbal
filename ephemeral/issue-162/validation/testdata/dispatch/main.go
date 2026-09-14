package main

import (
	"context"
	"fmt"
	"os"

	g "github.com/tylergannon/gimble"
)

func worker(ctx context.Context) error {
	g.Set(ctx, "result", "deterministic worker ran")
	fmt.Println("worker ran")
	return nil
}

func helper(ctx context.Context, key string) error {
	workers := map[string]func(context.Context) error{"run": worker}
	selected := workers[key]
	return selected(ctx)
}

func main() {
	dir, err := os.MkdirTemp("", "gimble-162-independent-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	err = g.Run(g.Project(context.Background(), dir), "dispatch", func(ctx context.Context) error {
		if err := g.Scope(ctx, "direct", func(ctx context.Context) error {
			workers := map[string]func(context.Context) error{"run": worker}
			return workers["run"](ctx)
		}); err != nil {
			return err
		}
		return g.Scope(ctx, "helper", func(ctx context.Context) error {
			return helper(ctx, "run")
		})
	})
	if err != nil {
		panic(err)
	}
}
