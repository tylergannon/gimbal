// Package cmd stands in for github.com/tylergannon/gimble/cmd at exactly
// that import path, so GIMBLE108's cmd exemption can be tested: this
// package runs a prompt given on the command line and is skipped, unlike
// promptchecks.
package cmd

import (
	"context"

	"github.com/tylergannon/gimble"
)

func runPrompt(ctx context.Context, prompt string) {
	session := &gimble.Session{}
	_, _ = session.Generate[string](ctx, prompt)
}
