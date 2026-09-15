// Command sprint runs the sprint workflow on the repository in the current
// directory, as in: go run ./cmd/sprint -sprint 2, or go run ./cmd/sprint
// -issue 168. With -dry-run it calls no model and runs no command: it prints
// every prompt with the schema it would send, answered with example values.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble/internal/workflows/sprint"
	"github.com/tylergannon/gimble/web"
)

func main() {
	var in sprint.Input
	flag.IntVar(&in.Sprint, "sprint", 0, "the sprint of SPRINTS.md to build")
	flag.StringVar(&in.Issue, "issue", "", "the issue to build instead of a sprint: a GitHub issue number, or a file holding the issue's text")
	flag.StringVar(&in.Goal, "goal", "", "desired result; replaces -sprint/-issue when supplied")
	flag.StringVar(&in.Plan, "plan", "", "absolute local plan or specification file")
	flag.StringVar(&in.Acceptance, "acceptance", "", "observable acceptance requirements")
	flag.StringVar(&in.Constraints, "constraints", "", "non-negotiable constraints")
	flag.Var((*stringList)(&in.ContextFiles), "context-file", "local context file (repeatable)")
	flag.Var((*stringList)(&in.Checks), "check", "deterministic repository check (repeatable)")
	flag.IntVar(&in.SupervisorIntervalSeconds, "supervisor-interval-seconds", 0, "supervisor look interval in seconds")
	flag.StringVar(&in.Finish, "finish", "local", "finish policy: local, pr, or merge")
	flag.BoolVar(&in.DryRun, "dry-run", false, "call no model and run no command: print every prompt with its schema, answered with example values")
	flag.StringVar(&in.Model, "model", "gpt-5.6-luna", "Codex model for the researcher, the planner, and the coders")
	flag.StringVar(&in.ReviewModel, "review-model", "haiku", "Claude Code model for the supervisors and the validator")
	flag.IntVar(&in.Tasks, "tasks", 10, "the most tasks to run in all")
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	uds := flag.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flag.Bool("no-web", false, "run without the web application")
	flag.Parse()
	if in.Goal == "" && (in.Sprint == 0) == (in.Issue == "") {
		log.Fatal("sprint: give -sprint or -issue, not both")
	}
	repo, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	in.Repo = repo
	project := filepath.Join(repo, ".gimble")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var runtimeOptions []web.Option
	switch {
	case in.DryRun:
		// A dry run is a viewer, so its record is left in a temporary
		// directory rather than among the project's runs.
		if project, err = os.MkdirTemp("", "sprint-dry-run"); err != nil {
			log.Fatal(err)
		}
		runtimeOptions = append(runtimeOptions, web.WithNoWeb())
	case *noWeb:
		runtimeOptions = append(runtimeOptions, web.WithNoWeb())
	case *uds != "":
		runtimeOptions = append(runtimeOptions, web.WithUDS(*uds))
	default:
		runtimeOptions = append(runtimeOptions, web.WithPort(*port))
	}
	runtime, err := web.NewRuntime(ctx, project, runtimeOptions...)
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

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
