package main

import (
	_ "embed"
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/web"
)

//go:embed defaults.json
var workflowDefaultsJSON []byte

func workflowDefaults() map[gimble.WorkflowRole]string {
	var defaults map[gimble.WorkflowRole]string
	if err := json.Unmarshal(workflowDefaultsJSON, &defaults); err != nil {
		panic(err)
	}
	return defaults
}

// newRunCommand is gimble run: the workflows built into this binary, each
// the Command generated beside this application.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Submit a compiled workflow to a running Gimble instance",
		Long:  "Submit a workflow compiled into both this CLI and the selected persistent instance. To add one, author it under internal/workflows/ in the Gimble checkout, generate its graph and application command with just build, register the generated command and hosted entry in cmd/gimble/workflows.go, and restart the serving binary. A CLI command does not transport a Go closure. The instance owns accepted runs after this client exits. Each workflow accepts --instance-dir (or GIMBLE_INSTANCE_DIR, default .gimble), --project for the admitted owner, --work-dir for execution, and --follow for terminal success or failure. Model and effort flags select each role; executables, PATH, and provider configuration come from the instance startup environment.",
	}
	run.AddCommand(reviewCommand(workflowDefaults()))
	run.AddCommand(validateproductCommand(workflowDefaults()))
	run.AddCommand(implementationCommand(workflowDefaults()))
	run.AddCommand(researchdocumentCommand(workflowDefaults()))
	run.AddCommand(pyramidsummaryCommand(workflowDefaults()))
	return run
}

func builtInWorkflows() web.Option {
	return web.WithWorkflows(map[string]web.WorkflowEntry{
		"review":            reviewHosted(),
		"validate-product":  validateproductHosted(),
		"implement":         implementationHosted(),
		"research-document": researchdocumentHosted(),
		"pyramid-summary":   pyramidsummaryHosted(),
	})
}
