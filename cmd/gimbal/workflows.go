package main

import (
	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/builtin"
)

func workflowDefaults() map[gimbal.WorkflowRole]string {
	return builtin.Defaults()
}

// newRunCommand is gimbal run: the workflows built into this binary, each
// the Command generated beside this application.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Submit a compiled workflow to a running Gimbal instance",
		Long:  "Submit a workflow compiled into both this CLI and the selected persistent instance. To add one, author it under internal/workflows/ in the Gimbal checkout, generate its graph and JSON invocation adapter with just build, and restart the serving binary. The stock workflow selection in internal/builtin/workflows.go generates the matching CLI command. A CLI command does not transport a Go closure. Each accepted run executes in an independent process and survives instance shutdown and restart. Supply parameter flags or --input with one JSON object. Each workflow accepts --instance-dir (or GIMBAL_INSTANCE_DIR, default .gimbal), --project for its owning repository, --work-dir for execution, and --follow for terminal success or failure. Model and effort flags select each role; executables, PATH, and provider configuration come from the instance startup environment.",
	}
	addStockCommands(run, workflowDefaults())
	return run
}
