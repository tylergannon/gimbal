package main

import (
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:               "gimbal",
		Short:             "Run coding agents from ordinary Go workflows",
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.AddCommand(newRunCommand(), newLsCommand(), newRunPromptCommand())
	return root
}
