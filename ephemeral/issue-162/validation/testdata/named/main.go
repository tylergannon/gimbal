package main

import (
	"context"
	"os"

	"github.com/tylergannon/gimble"
)

func worker(context.Context) error { return nil }

func workflow(ctx context.Context) error {
	workers := map[string]func(context.Context) error{"run": worker}
	return workers["run"](ctx)
}

func main() {
	dir, err := os.MkdirTemp("", "gimble-162-named-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := gimble.Run(gimble.Project(context.Background(), dir), "named", workflow); err != nil {
		panic(err)
	}
}
