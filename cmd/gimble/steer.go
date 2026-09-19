package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSteerCommand() *cobra.Command {
	var workDir, session, loop string
	command := &cobra.Command{
		Use:   "steer RUN MESSAGE",
		Short: "Steer an agent session or queue a message for a loop planner",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (session == "") == (loop == "") {
				return errors.New("select exactly one target with --session or --loop")
			}
			if strings.TrimSpace(args[1]) == "" {
				return errors.New("steering message must not be blank")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			runtime, found, err := findRun(ctx, workDir, args[0])
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("no running Gimble instance owns run %q", args[0])
			}
			body := map[string]string{"run": args[0], "message": args[1]}
			path := "/control/steer"
			if session != "" {
				body["session"] = session
			} else {
				path = "/control/steer-loop"
				body["scope"] = loop
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				return err
			}
			response, cleanup, err := runtime.request(ctx, http.MethodPost, path, bytes.NewReader(encoded))
			if err != nil {
				return err
			}
			defer cleanup()
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK {
				message, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				return fmt.Errorf("steer: %s: %s", response.Status, strings.TrimSpace(string(message)))
			}
			_, err = io.Copy(cmd.OutOrStdout(), response.Body)
			return err
		},
	}
	command.Flags().StringVar(&workDir, "work-dir", ".", "the working directory for this project")
	command.Flags().StringVar(&session, "session", "", "session ID to steer")
	command.Flags().StringVar(&loop, "loop", "", "loop scope ID whose planner should receive the message")
	return command
}
