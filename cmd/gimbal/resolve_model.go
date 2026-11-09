package main

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/modelalias"
)

func newResolveModelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve-model ALIAS[:EFFORT]",
		Short: "Resolve a model selection through Gimbal's central alias catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, effort, explicit := strings.Cut(args[0], ":")
			resolved, err := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: explicit})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(resolved)
		},
	}
}
