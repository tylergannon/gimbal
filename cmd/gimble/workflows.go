package main

import (
	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/builtin"
)

func workflowDefaults() map[gimble.WorkflowRole]string {
	return builtin.Defaults()
}

// newRunCommand is gimble run: the workflows built into this binary, each
// the Command generated beside this application.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Submit a compiled workflow to a running Gimble instance",
		Long:  "Submit a workflow compiled into both this CLI and the selected persistent instance. To add one, author it under internal/workflows/ in the Gimble checkout, generate its graph and Form handler with just build, and restart the serving binary. The stock workflow selection in internal/builtin/workflows.go generates the matching CLI command. A CLI command does not transport a Go closure. The instance owns accepted runs after this client exits. Each workflow accepts --instance-dir (or GIMBLE_INSTANCE_DIR, default .gimble), --project for its owning repository, --work-dir for execution, and --follow for terminal success or failure. Model and effort flags select each role; executables, PATH, and provider configuration come from the instance startup environment.",
	}
	addStockCommands(run, workflowDefaults())
	return run
}
