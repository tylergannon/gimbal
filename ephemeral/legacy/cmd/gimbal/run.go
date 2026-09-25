package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/program"
	"github.com/tylergannon/gimbal/program/workflows"
)

type workflowEntry struct {
	name    string
	summary string
	command func() *cobra.Command
}

var workflowCatalog = []workflowEntry{
	{"sprint-execute", "Work a planned sprint ledger to done: implement, review, repair, validate each item", newSprintExecuteCommand},
	{"chapter-loop", "Work a chapter ledger; each chapter runs its own sprint ledger", newChapterLoopCommand},
	{"delivery-loop", "Plan a checklist from a specification, critique it, then work it", newDeliveryLoopCommand},
}

func newLsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List workflows",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 3, ' ', 0)
			for _, entry := range workflowCatalog {
				_, _ = fmt.Fprintf(w, "%s\t%s\n", entry.name, entry.summary)
			}
			return w.Flush()
		},
	}
}

func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run [workflow]",
		Short: "Run a workflow; `gimbal run <workflow> --help` shows its arguments",
	}
	for _, entry := range workflowCatalog {
		command := entry.command()
		command.Use = entry.name
		command.Short = entry.summary
		command.Args = cobra.NoArgs
		run.AddCommand(command)
	}
	return run
}

// runtimeFlags are shared by every workflow: where it works, where it logs,
// and how long the whole run may take.
type runtimeFlags struct {
	workdir string
	logs    string
	timeout time.Duration
}

func (f *runtimeFlags) bind(command *cobra.Command) {
	command.Flags().StringVar(&f.workdir, "workdir", ".", "application workspace")
	command.Flags().StringVar(&f.logs, "logs", "", "new run artifact directory (default: a temporary directory)")
	command.Flags().DurationVar(&f.timeout, "timeout", 2*time.Hour, "whole run timeout")
}

func (f *runtimeFlags) execute(command *cobra.Command, run func(context.Context, *program.Runtime) error) error {
	runtime, err := program.NewRuntime(program.Config{Workdir: f.workdir, RunDir: f.logs})
	if err != nil {
		return err
	}
	defer runtime.Close()
	if _, err := fmt.Fprintf(command.ErrOrStderr(), "Workdir: %s\nLogs: %s\n", runtime.Workdir, runtime.RunDir); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()
	if err := run(ctx, runtime); err != nil {
		return err
	}
	_, err = fmt.Fprintln(command.OutOrStdout(), "completed")
	return err
}

func newSprintExecuteCommand() *cobra.Command {
	var flags runtimeFlags
	var input workflows.SprintExecuteInput
	command := &cobra.Command{
		RunE: func(command *cobra.Command, _ []string) error {
			return flags.execute(command, func(ctx context.Context, runtime *program.Runtime) error {
				return workflows.SprintExecute(ctx, runtime, input)
			})
		},
	}
	flags.bind(command)
	command.Flags().StringVar(&input.Goal, "goal", "", "the outcome that frames each sprint")
	command.Flags().StringVar(&input.Checklist, "checklist", "", "sprint checklist path, relative to workdir")
	command.Flags().StringVar(&input.ImplementModel, "implement-model", "", "model for implementation turns")
	command.Flags().StringVar(&input.ReviewModel, "review-model", "", "model for independent review turns")
	command.Flags().StringVar(&input.EvaluateModel, "evaluate-model", "", "model for checklist evaluation turns")
	command.Flags().IntVar(&input.MaxIterations, "max-iterations", 0, "maximum loop arrivals")
	_ = command.MarkFlagRequired("goal")
	_ = command.MarkFlagRequired("checklist")
	return command
}

func newChapterLoopCommand() *cobra.Command {
	var flags runtimeFlags
	var input workflows.ChapterLoopInput
	command := &cobra.Command{
		RunE: func(command *cobra.Command, _ []string) error {
			return flags.execute(command, func(ctx context.Context, runtime *program.Runtime) error {
				return workflows.ChapterLoop(ctx, runtime, input)
			})
		},
	}
	flags.bind(command)
	command.Flags().StringVar(&input.Goal, "goal", "", "the outcome that frames the chapters")
	command.Flags().StringVar(&input.Checklist, "checklist", "", "chapter checklist path, relative to workdir")
	command.Flags().StringVar(&input.ImplementModel, "implement-model", "", "model for implementation turns")
	command.Flags().StringVar(&input.ReviewModel, "review-model", "", "model for independent review turns")
	command.Flags().StringVar(&input.EvaluateModel, "evaluate-model", "", "model for checklist evaluation turns")
	command.Flags().IntVar(&input.ChapterMaxIterations, "chapter-max-iterations", 0, "maximum chapter loop arrivals")
	command.Flags().IntVar(&input.SprintMaxIterations, "sprint-max-iterations", 0, "maximum sprint loop arrivals per chapter")
	_ = command.MarkFlagRequired("goal")
	_ = command.MarkFlagRequired("checklist")
	return command
}

func newDeliveryLoopCommand() *cobra.Command {
	var flags runtimeFlags
	var input workflows.DeliveryLoopInput
	command := &cobra.Command{
		RunE: func(command *cobra.Command, _ []string) error {
			return flags.execute(command, func(ctx context.Context, runtime *program.Runtime) error {
				return workflows.DeliveryLoop(ctx, runtime, input)
			})
		},
	}
	flags.bind(command)
	command.Flags().StringVar(&input.Goal, "goal", "", "specification path or outcome to deliver")
	command.Flags().StringVar(&input.Checklist, "checklist", "", "plan checklist output path (default plan.md)")
	command.Flags().StringVar(&input.PlanModel, "plan-model", "", "model for planning")
	command.Flags().StringVar(&input.CritiqueModel, "critique-model", "", "model for plan critique")
	command.Flags().StringVar(&input.CodingModel, "coding-model", "", "model for coding turns")
	command.Flags().StringVar(&input.ReviewModel, "review-model", "", "model for independent review turns")
	command.Flags().StringVar(&input.EvaluateModel, "evaluate-model", "", "model for checklist evaluation turns")
	command.Flags().IntVar(&input.MaxIterations, "max-iterations", 0, "maximum work loop arrivals")
	_ = command.MarkFlagRequired("goal")
	return command
}
