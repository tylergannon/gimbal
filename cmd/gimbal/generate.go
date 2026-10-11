package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/generate"
)

func newGenerateCommand() *cobra.Command {
	var dir, entry, name string
	var choices []string
	cmd := &cobra.Command{Use: "generate", Short: "Generate a workflow definition and typed schemas in its Go module", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		defaults := map[string]string{}
		for _, choice := range choices {
			role, model, ok := strings.Cut(choice, "=")
			if !ok || role == "" || model == "" {
				return fmt.Errorf("--role-default must be role=model")
			}
			defaults[role] = model
		}
		return generate.Definition(dir, entry, name, defaults)
	}}
	cmd.Flags().StringVar(&dir, "dir", ".", "workflow package directory")
	cmd.Flags().StringVar(&entry, "entry", "", "ordinary Go workflow entry")
	cmd.Flags().StringVar(&name, "name", "", "catalog workflow name")
	cmd.Flags().StringArrayVar(&choices, "role-default", nil, "role=model default (repeat for each workflow role)")
	_ = cmd.MarkFlagRequired("entry")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
