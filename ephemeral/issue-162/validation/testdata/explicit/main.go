package main

import (
	"context"
	"fmt"
	"os"

	g "github.com/tylergannon/gimble"
)

func worker(ctx context.Context) error {
	const prefix = "worker "
	g.Set(ctx, prefix+"result", fmt.Sprint("dynamic value ", os.Getpid()))
	fmt.Println("worker ran")
	return nil
}

func helper(ctx context.Context, key string) error {
	switch key {
	case "run":
		return worker(ctx)
	case "alias":
		known := worker
		return known(ctx)
	default:
		return fmt.Errorf("unsupported task %q", key)
	}
}

func main() {
	dir, err := os.MkdirTemp("", "gimble-162-independent-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	err = g.Run(g.Project(context.Background(), dir), "explicit", func(ctx context.Context) error {
		group := g.Group(ctx, "workers")
		group.Go("direct", func(ctx context.Context) error { return helper(ctx, "run") })
		group.Go("alias", func(ctx context.Context) error { return helper(ctx, "alias") })
		return group.Wait()
	})
	if err != nil {
		panic(err)
	}
}
