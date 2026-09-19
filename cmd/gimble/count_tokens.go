package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tiktoken-go/tokenizer"
)

func newCountTokensCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "count-tokens FILE",
		Short: "Count a file's o200k_base tokens",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			codec, err := tokenizer.Get(tokenizer.O200kBase)
			if err != nil {
				return fmt.Errorf("initialize tokenizer: %w", err)
			}
			count, err := codec.Count(string(data))
			if err != nil {
				return fmt.Errorf("count tokens: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), count)
			return err
		},
	}
}
