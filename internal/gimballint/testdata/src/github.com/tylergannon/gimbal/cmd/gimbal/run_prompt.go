// Package cmd stands in for github.com/tylergannon/gimbal/cmd/gimbal at exactly
// that import path, so GIMBAL108's cmd exemption can be tested: this
// package runs a prompt given on the command line and is skipped, unlike
// promptchecks.
package cmd

import (
	"context"

	"github.com/tylergannon/gimbal"
)

func runPrompt(ctx context.Context, prompt string) {
	session := &gimbal.Session{}
	_, _ = session.Generate[string](ctx, prompt)
}
