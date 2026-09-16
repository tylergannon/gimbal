package main

import (
	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble/internal/workflows/easyloop"
	"github.com/tylergannon/gimble/internal/workflows/execute"
	"github.com/tylergannon/gimble/internal/workflows/plan"
	"github.com/tylergannon/gimble/internal/workflows/sprint"
)

// newRunCommand is gimble run: the workflows built into this binary, each
// the Command its package generated.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a workflow built into this binary; gimble run --help lists them",
	}
	run.AddCommand(easyloop.Command(), execute.Command(), plan.Command(), sprint.Command())
	return run
}
