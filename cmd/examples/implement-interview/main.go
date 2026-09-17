// Command implement-interview builds and demonstrates Gimble issue 249.
package main

import (
	"fmt"
	"os"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/workflows/implementinterview"
)

func main() {
	cmd := implementinterview.Command(map[gimble.WorkflowRole]string{
		"api-research":                   "gpt-5.6-terra:high",
		"frontend-research":              "claude-sonnet-5:high",
		gimble.RoleSprintPlanning:        "claude-opus-5:max",
		"coding":                         "gpt-5.6-sol:xhigh",
		"implementation-scope-review":    "gpt-5.6-terra:high",
		gimble.RoleArchitecturalCritique: "claude-sonnet-5:high",
		gimble.RoleQAOrchestration:       "claude-opus-5:max",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "implement-interview:", err)
		os.Exit(1)
	}
}
