package main

import (
	"context"
	"os"

	"github.com/tylergannon/gimble"
)

func worker(context.Context) error { return nil }

func helper() error {
	workers := map[string]func(context.Context) error{"run": worker}
	return workers["run"](context.Background())
}

func main() {
	dir, err := os.MkdirTemp("", "gimble-162-helper-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := gimble.Run(gimble.Project(context.Background(), dir), "helper", func(context.Context) error {
		return helper()
	}); err != nil {
		panic(err)
	}
}
