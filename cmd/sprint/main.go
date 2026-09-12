// Command sprint runs the sprint workflow on the repository in the current
// directory, as in: go run ./cmd/sprint -sprint 2
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tylergannon/gimble/internal/workflows/sprint"
	"github.com/tylergannon/gimble/web"
)

func main() {
	var in sprint.Input
	flag.IntVar(&in.Sprint, "sprint", 0, "the sprint of SPRINTS.md to build")
	flag.StringVar(&in.Model, "model", "gpt-5.6-luna", "Codex model for the researcher, the planner, and the coders")
	flag.StringVar(&in.ReviewModel, "review-model", "haiku", "Claude Code model for the supervisors and the validator")
	flag.IntVar(&in.Tasks, "tasks", 10, "the most tasks to run in all")
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	uds := flag.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flag.Bool("no-web", false, "run without the web application")
	flag.Parse()
	repo, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	in.Repo = repo

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var runtimeOptions []web.Option
	switch {
	case *noWeb:
		runtimeOptions = append(runtimeOptions, web.WithNoWeb())
	case *uds != "":
		runtimeOptions = append(runtimeOptions, web.WithUDS(*uds))
	default:
		runtimeOptions = append(runtimeOptions, web.WithPort(*port))
	}
	runtime, err := web.NewRuntime(ctx, filepath.Join(repo, ".gimble"), runtimeOptions...)
	if err != nil {
		log.Fatal(err)
	}
	err = runtime.Run(ctx, "sprint", func(ctx context.Context) error {
		return sprint.Sprint(ctx, in)
	})
	if err != nil {
		log.Fatal(err)
	}
}
