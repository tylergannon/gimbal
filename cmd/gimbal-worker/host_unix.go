//go:build !windows

package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func runHost(command *cobra.Command, config workerConfig) error {
	if config.service {
		return errors.New("--service requires Windows SCM; omit it for foreground mode")
	}
	ctx, cancel := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runWorker(ctx, config, command.OutOrStdout(), func() {})
}
func userEnvironment(string) (string, error) {
	return "", errors.New("user environment imports require Windows")
}
