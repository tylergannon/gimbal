package main

import (
	_ "embed"
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/workflows/review"
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
// the Command its package generated.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a workflow built into this binary; gimble run --help lists them",
	}
	run.AddCommand(review.Command(workflowDefaults()))
	return run
}
