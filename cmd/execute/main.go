// Command execute runs the execute workflow, the translation of the
// df-sprint-execute skill, on the repository in the current directory, as
// in: go run ./cmd/execute -sprint 1.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/internal/workflows/execute"
	"github.com/tylergannon/gimble/web"
)

func main() {
	var in execute.Input
	flag.IntVar(&in.Sprint, "sprint", 0, "the sprint to execute: NNN of docs/sprints/SPRINT-NNN.md")
	flag.StringVar(&in.Test, "test", "go test ./...", "the repository's test command, run with sh -c after the worker finishes")
	worker := flag.String("worker", "gpt-5.6-luna", "model for the worker, as model or model:effort")
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	uds := flag.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flag.Bool("no-web", false, "run without the web application")
	flag.Parse()
	if in.Sprint == 0 {
		log.Fatal("execute: give -sprint")
	}
	models, err := bind(map[string]string{"worker": *worker})
	if err != nil {
		log.Fatal("execute: ", err)
	}
	repo, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	in.Repo = repo

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var options []web.Option
	switch {
	case *noWeb:
		options = append(options, web.WithNoWeb())
	case *uds != "":
		options = append(options, web.WithUDS(*uds))
	default:
		options = append(options, web.WithPort(*port))
	}
	runtime, err := web.NewRuntime(ctx, filepath.Join(repo, ".gimble"), options...)
	if err != nil {
		log.Fatal(err)
	}
	err = runtime.Run(ctx, "execute", models, func(ctx context.Context) error {
		return execute.Execute(ctx, in)
	})
	if err != nil {
		log.Fatal(err)
	}
}

// bind binds each role the workflow names to the model its flag gave.
func bind(specs map[string]string) (map[string]gimble.ModelBinding, error) {
	models := map[string]gimble.ModelBinding{}
	for role, spec := range specs {
		bound, err := binding.Parse(spec)
		if err != nil {
			return nil, fmt.Errorf("-%s: %w", role, err)
		}
		models[role] = bound
	}
	return models, nil
}
