package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCancelCommand() *cobra.Command {
	var workDir string
	command := &cobra.Command{
		Use: "cancel RUN", Short: "Cancel an owned run or retry its pending remote cleanup",
		Long:    "Cancel a run through its owning instance. A terminal run with failed remote cleanup retains stop control while that instance is alive; cancel retries cleanup of the exact owned sessions without restarting the assignment. Accepted means the request was delivered. Observe the run for cancellation and cleanup results; an unreachable backend leaves cleanup pending. The original workflow error remains in history.",
		Example: "  gimbal cancel RUN_ID --work-dir /abs/project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			owner, found, err := findRun(ctx, workDir, args[0])
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("no running Gimbal instance retains control of run %q", args[0])
			}
			raw, err := json.Marshal(map[string]string{"run": args[0]})
			if err != nil {
				return err
			}
			response, cleanup, err := owner.request(ctx, http.MethodPost, "/control/cancel", bytes.NewReader(raw))
			if err != nil {
				return err
			}
			defer cleanup()
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				return fmt.Errorf("cancel: %s: %s", response.Status, strings.TrimSpace(string(body)))
			}
			_, err = io.Copy(cmd.OutOrStdout(), response.Body)
			return err
		},
	}
	command.Flags().StringVar(&workDir, "work-dir", ".", "repository directory owning this project's .gimbal state")
	return command
}
