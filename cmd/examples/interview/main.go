// Command interview runs the human interview example.
package main

import (
	"fmt"
	"os"

	"github.com/tylergannon/gimble"
	interviewworkflow "github.com/tylergannon/gimble/internal/workflows/interview"
)

func main() {
	cmd := interviewworkflow.Command(map[gimble.WorkflowRole]string{
		"interviewer": "gpt-5.6-terra",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "interview:", err)
		os.Exit(1)
	}
}
