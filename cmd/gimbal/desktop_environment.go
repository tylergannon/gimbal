package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/environments"
)

func newDesktopEnvironmentCommand() *cobra.Command {
	return &cobra.Command{Use: "desktop-environment JSON", Short: "Inspect or prepare a desktop session directory on this machine", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		var in environments.HelperRequest
		if e := json.Unmarshal([]byte(args[0]), &in); e != nil {
			return fmt.Errorf("desktop-environment request: %w", e)
		}
		out, e := environments.RunHelper(c.Context(), in)
		if e != nil {
			return e
		}
		return json.NewEncoder(c.OutOrStdout()).Encode(out)
	}}
}
