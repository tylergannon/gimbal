package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/workflows/intake"
	"github.com/tylergannon/gimble/internal/workflows/lfg"
	"github.com/tylergannon/gimble/internal/workflows/planning"
	"github.com/tylergannon/gimble/internal/workflows/sprint"
	"github.com/tylergannon/gimble/web"
)

type stringFlags []string

func (s *stringFlags) String() string { return strings.Join(*s, ", ") }
func (s *stringFlags) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value must not be blank")
	}
	*s = append(*s, value)
	return nil
}

// builtinRequest is CLI input, not an executable workflow graph.
type builtinRequest struct {
	Repo, Goal, Plan, Acceptance, Constraints string
	SemanticIndex, TokenCache                 string
	ContextFiles, Checks                      stringFlags
	Model, ReviewModel, PlanningModel         string
	Tasks, SupervisorIntervalSeconds          int
	Finish                                    string
	DryRun                                    bool
}

func runBuiltin(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	joinInput := closeBuiltinInputOnCancel(ctx, stdin)
	defer joinInput()
	flags := flag.NewFlagSet("gimble "+name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	var in builtinRequest
	flags.StringVar(&in.Repo, "repo", ".", "repository to work in")
	flags.StringVar(&in.Goal, "goal", "", "goal, claim, or description of the work")
	flags.StringVar(&in.Plan, "plan", "", "local plan or design file (relative to -repo)")
	flags.StringVar(&in.SemanticIndex, "semantic-index", "", "semantic index entrypoint for Sprint Plan (relative to -repo)")
	flags.StringVar(&in.TokenCache, "token-cache", "", "local source cache for Sprint Plan (relative to -repo)")
	flags.Var(&in.ContextFiles, "file", "local context file (repeatable, relative to -repo)")
	flags.StringVar(&in.Acceptance, "acceptance", "", "what must be true when the work is done")
	flags.StringVar(&in.Constraints, "constraints", "", "constraints and things to leave alone")
	flags.Var(&in.Checks, "check", "verification command to run in the repository (repeatable)")
	flags.StringVar(&in.Model, "model", "gpt-5.6-luna", "Codex worker/planner model")
	flags.StringVar(&in.ReviewModel, "review-model", "haiku", "Claude supervisor/reviewer model")
	flags.StringVar(&in.PlanningModel, "planning-model", "gemini-3.8-flash-low", "Gemini model for the third planning perspective")
	flags.IntVar(&in.Tasks, "tasks", 10, "maximum sprint tasks")
	interval := flags.Duration("supervisor-interval", 30*time.Second, "time between supervisor looks (whole seconds)")
	flags.StringVar(&in.Finish, "finish", "local", "sprint delivery: local, pr, or merge")
	flags.BoolVar(&in.DryRun, "dry-run", false, "preview prompts without models or execution commands")
	yes := flags.Bool("yes", false, "accept the guide's workflow choice and execute after planning without further input")
	noWeb := flags.Bool("no-web", false, "record the run without starting the web server")
	port := flags.Int("port", 0, "web server port; 0 chooses a free port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		if in.Goal != "" {
			return errors.New("give the goal with -goal or as trailing text, not both")
		}
		in.Goal = strings.Join(flags.Args(), " ")
	}
	if *interval < time.Second || *interval%time.Second != 0 {
		return errors.New("supervisor interval must be a positive whole number of seconds")
	}
	in.SupervisorIntervalSeconds = int(*interval / time.Second)
	if in.Tasks <= 0 {
		return errors.New("tasks must be positive")
	}
	switch in.Finish {
	case "local", "pr", "merge":
	default:
		return errors.New("finish must be local, pr, or merge")
	}
	if name != "sprint" && name != "work" && in.Finish != "local" {
		return errors.New("-finish applies to sprint execution")
	}
	reader := bufio.NewReader(stdin)
	if strings.TrimSpace(in.Goal) == "" && in.Plan == "" && len(in.ContextFiles) == 0 {
		if *yes || in.DryRun {
			return errors.New("provide a goal, -plan, or -file")
		}
		_, _ = fmt.Fprintln(stdout, "What do you want to accomplish? A sentence, a claim, or a design is enough to start.")
		answer, err := readBuiltinLine(ctx, reader)
		if err != nil && (!errors.Is(err, io.EOF) || answer == "") {
			return fmt.Errorf("goal: %w", err)
		}
		in.Goal = strings.TrimSpace(answer)
	}
	if err := normalizeBuiltin(&in); err != nil {
		return err
	}
	project := filepath.Join(in.Repo, ".gimble")
	if in.DryRun {
		project = filepath.Join(project, "previews")
	}
	if err := os.MkdirAll(filepath.Join(project, "requests"), 0o755); err != nil {
		return err
	}
	artifacts, err := os.MkdirTemp(filepath.Join(project, "requests"), name+"-")
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifacts, "request.json"), data, 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Request and planning artifacts:", artifacts)
	option := web.WithPort(*port)
	if *noWeb || in.DryRun {
		option = web.WithNoWeb()
	}
	runtime, err := web.NewRuntime(ctx, project, option)
	if err != nil {
		return err
	}

	switch name {
	case "lfg":
		return runtime.Run(ctx, "lfg", func(ctx context.Context) error { return runLFG(ctx, in, stdout) })
	case "plan":
		return runtime.Run(ctx, "plan", func(ctx context.Context) error {
			if *yes {
				reader = nil
			}
			_, err := makePlan(ctx, in, artifacts, reader, stdout)
			return err
		})
	case "sprint":
		return runtime.Run(ctx, "sprint", func(ctx context.Context) error { return executeSprint(ctx, in) })
	case "work":
		return runtime.Run(ctx, "work", func(ctx context.Context) error {
			if in.DryRun {
				_, _ = fmt.Fprintln(stdout, "Guide preview: inspect the request; clarify only material unknowns; choose lfg, plan, or sprint. Use an explicit subcommand with -dry-run to preview that workflow.")
				return nil
			}
			var advice intake.Advice
			err := gimble.Scope(ctx, "intake", func(ctx context.Context) error {
				var err error
				interview := reader
				if *yes {
					interview = nil
				}
				advice, err = intake.Guide(ctx, intake.Input{Repo: in.Repo, Goal: in.Goal, Plan: in.Plan, Acceptance: in.Acceptance, Constraints: in.Constraints, ContextFiles: in.ContextFiles, Model: in.Model}, interview, stdout)
				return err
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "Recommended: %s — %s\n", advice.Workflow, advice.Reason)
			choice := advice.Workflow
			if !*yes {
				_, _ = fmt.Fprintln(stdout, "Press Enter to use that workflow, or type lfg, plan, sprint, or cancel:")
				answer, err := readBuiltinLine(ctx, reader)
				if err != nil && (!errors.Is(err, io.EOF) || answer == "") {
					return fmt.Errorf("workflow choice: %w", err)
				}
				if answer = strings.TrimSpace(answer); answer != "" {
					choice = answer
				}
			}
			in.Constraints += advice.Clarifications
			if err := os.WriteFile(filepath.Join(artifacts, "clarifications.md"), []byte(advice.Clarifications), 0o644); err != nil {
				return err
			}
			switch choice {
			case "lfg":
				return gimble.Scope(ctx, "lfg", func(ctx context.Context) error { return runLFG(ctx, in, stdout) })
			case "plan":
				var plan string
				err := gimble.Scope(ctx, "planning", func(ctx context.Context) error {
					var err error
					interview := reader
					if *yes {
						interview = nil
					}
					plan, err = makePlan(ctx, in, artifacts, interview, stdout)
					return err
				})
				if err != nil {
					return err
				}
				if !*yes {
					_, _ = fmt.Fprintln(stdout, "Run this plan now? [y/N]")
					answer, err := readBuiltinLine(ctx, reader)
					if err != nil && (!errors.Is(err, io.EOF) || answer == "") {
						return fmt.Errorf("plan saved; execution choice: %w", err)
					}
					if strings.TrimSpace(strings.ToLower(answer)) != "y" {
						return nil
					}
				}
				in.Plan = plan
				return gimble.Scope(ctx, "execution", func(ctx context.Context) error { return executeSprint(ctx, in) })
			case "sprint":
				return gimble.Scope(ctx, "execution", func(ctx context.Context) error { return executeSprint(ctx, in) })
			case "cancel":
				return errors.New("cancelled before execution")
			default:
				return fmt.Errorf("unknown workflow %q; choose lfg, plan, or sprint", choice)
			}
		})
	default:
		return fmt.Errorf("unknown builtin %q", name)
	}
}

func normalizeBuiltin(in *builtinRequest) error {
	repo, err := filepath.Abs(in.Repo)
	if err != nil {
		return err
	}
	info, err := os.Stat(repo)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("repository is not a directory: %s", repo)
	}
	in.Repo = repo
	if in.SemanticIndex != "" {
		if !filepath.IsAbs(in.SemanticIndex) {
			in.SemanticIndex = filepath.Join(repo, in.SemanticIndex)
		}
		if err := readableInputFile(in.SemanticIndex); err != nil {
			return err
		}
	}
	if in.TokenCache != "" {
		if !filepath.IsAbs(in.TokenCache) {
			in.TokenCache = filepath.Join(repo, in.TokenCache)
		}
		info, err := os.Stat(in.TokenCache)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("token-cache must be a directory")
		}
	}
	for i, file := range in.ContextFiles {
		if !filepath.IsAbs(file) {
			file = filepath.Join(repo, file)
		}
		if err := readableInputFile(file); err != nil {
			return err
		}
		in.ContextFiles[i] = filepath.Clean(file)
	}
	if in.Plan != "" {
		if !filepath.IsAbs(in.Plan) {
			in.Plan = filepath.Join(repo, in.Plan)
		}
		if err := readableInputFile(in.Plan); err != nil {
			return err
		}
	}
	if strings.TrimSpace(in.Goal) == "" {
		switch {
		case in.Plan != "":
			in.Goal = "Realize the work described in " + in.Plan + "."
		case len(in.ContextFiles) > 0:
			in.Goal = "Realize the design or claim described in the supplied context files."
		default:
			return errors.New("goal must not be blank")
		}
	}
	return nil
}

func readableInputFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("input file: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("input is not a regular file: %s", path)
	}
	return nil
}

func runLFG(ctx context.Context, in builtinRequest, out io.Writer) error {
	if in.Finish != "local" {
		return errors.New("lfg leaves changes local; choose sprint for -finish pr or merge")
	}
	files := append([]string(nil), in.ContextFiles...)
	if in.Plan != "" {
		files = append(files, in.Plan)
	}
	result, err := lfg.LFG(ctx, lfg.Input{Repo: in.Repo, Goal: in.Goal, Acceptance: in.Acceptance, Constraints: in.Constraints, ContextFiles: files, Checks: in.Checks, Model: in.Model, ReviewModel: in.ReviewModel, SupervisorIntervalSeconds: in.SupervisorIntervalSeconds, DryRun: in.DryRun}, out)
	if err == nil {
		_, _ = fmt.Fprintln(out, result)
	}
	return err
}

func readBuiltinLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	text, err := reader.ReadString('\n')
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return text, err
}

// The CLI owns stdin. Closing it interrupts a pending question on Ctrl-C;
// workflow scopes read synchronously and never leave a reader goroutine behind.
func closeBuiltinInputOnCancel(ctx context.Context, input io.Reader) func() {
	closer, ok := input.(io.Closer)
	if !ok {
		return func() {}
	}
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			_ = closer.Close()
		case <-stop:
		}
	}()
	return func() { close(stop); <-joined }
}

func makePlan(ctx context.Context, in builtinRequest, artifacts string, reader *bufio.Reader, out io.Writer) (string, error) {
	files := append([]string(nil), in.ContextFiles...)
	if in.Plan != "" {
		files = append(files, in.Plan)
	}
	plan, err := planning.Plan(ctx, planning.Input{Repo: in.Repo, SemanticIndex: in.SemanticIndex, TokenCache: in.TokenCache, Goal: in.Goal, Acceptance: in.Acceptance, Constraints: in.Constraints, ContextFiles: files, Checks: in.Checks, Model: in.Model, ReviewModel: in.ReviewModel, PlanningModel: in.PlanningModel, OutputDir: artifacts, DryRun: in.DryRun}, reader, out)
	if err == nil {
		_, _ = fmt.Fprintln(out, "Plan:", plan)
	}
	return plan, err
}

func executeSprint(ctx context.Context, in builtinRequest) error {
	return sprint.Sprint(ctx, sprint.Input{Repo: in.Repo, Goal: in.Goal, Plan: in.Plan, Acceptance: in.Acceptance, Constraints: in.Constraints, ContextFiles: in.ContextFiles, Checks: in.Checks, Model: in.Model, ReviewModel: in.ReviewModel, Tasks: in.Tasks, SupervisorIntervalSeconds: in.SupervisorIntervalSeconds, Finish: in.Finish, DryRun: in.DryRun})
}
