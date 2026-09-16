// Command easyloop runs the easy loop workflow, the translation of the
// df-easy-loop-simple skill, on the repository in the current directory, as
// in: go run ./cmd/easyloop -spec /path/to/spec.md.
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
	"github.com/tylergannon/gimble/internal/workflows/easyloop"
	"github.com/tylergannon/gimble/web"
)

func main() {
	var in easyloop.Input
	flag.StringVar(&in.Spec, "spec", "", "the spec document that says what is being built")
	flag.IntVar(&in.Tasks, "tasks", 50, "the most tasks to run in all")
	planner := flag.String("planner", "claude-haiku-4-5-20251001", "model for the planner, who writes the plan and dispatches its work, as model or model:effort")
	critic := flag.String("critic", "gpt-5.6-luna", "model for the critic of the plan, as model or model:effort")
	coder := flag.String("coder", "gpt-5.6-luna", "model for the coder, as model or model:effort")
	reviewer := flag.String("reviewer", "claude-haiku-4-5-20251001", "model for the reviewer, as model or model:effort")
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	uds := flag.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flag.Bool("no-web", false, "run without the web application")
	flag.Parse()
	if in.Spec == "" {
		log.Fatal("easyloop: give -spec")
	}
	models, err := bind(map[string]string{"planner": *planner, "critic": *critic, "coder": *coder, "reviewer": *reviewer})
	if err != nil {
		log.Fatal("easyloop: ", err)
	}
	repo, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	in.Repo = repo
	if in.Spec, err = filepath.Abs(in.Spec); err != nil {
		log.Fatal(err)
	}

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
	err = runtime.Run(ctx, "easyloop", models, func(ctx context.Context) error {
		return easyloop.EasyLoop(ctx, in)
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
