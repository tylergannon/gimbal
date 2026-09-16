// Command plan runs the plan workflow, the translation of the df-sprint-plan
// skill, on the repository in the current directory, as in:
// go run ./cmd/plan -sprint 2 -seed "the run page draws the workflow graph".
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
	"github.com/tylergannon/gimble/internal/workflows/plan"
	"github.com/tylergannon/gimble/web"
)

func main() {
	var in plan.Input
	flag.IntVar(&in.Sprint, "sprint", 0, "the sprint to plan: NNN of the docs/sprints/SPRINT-NNN.md it writes")
	flag.StringVar(&in.Seed, "seed", "", "what the sprint should be about")
	planner := flag.String("planner", "claude-haiku-4-5-20251001", "model for the planner, who writes the intent, the questions, and the merge, as model or model:effort")
	claude := flag.String("claude", "claude-haiku-4-5-20251001", "model for the Claude lane, as model or model:effort")
	codex := flag.String("codex", "gpt-5.6-luna", "model for the Codex lane, as model or model:effort")
	gemini := flag.String("gemini", "gemini-3.8-flash-low", "model for the Gemini lane, as model or model:effort")
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	uds := flag.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flag.Bool("no-web", false, "run without the web application")
	flag.Parse()
	if in.Sprint == 0 || in.Seed == "" {
		log.Fatal("plan: give -sprint and -seed")
	}
	models, err := bind(map[string]string{"planner": *planner, "claude": *claude, "codex": *codex, "gemini": *gemini})
	if err != nil {
		log.Fatal("plan: ", err)
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
	err = runtime.Run(ctx, "plan", models, func(ctx context.Context) error {
		return plan.Plan(ctx, in)
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
