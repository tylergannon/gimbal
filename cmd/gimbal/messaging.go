package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/backend/secretfiles"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func newMessagingWorkerCommand() *cobra.Command {
	var routeFile, readyFile, secretsDirectory string
	var dirs []string
	c := &cobra.Command{Use: "message-worker", Short: "Serve native actions through a runtime-loaded messaging plugin", Long: `Serve the configured request store/queue or HTTP endpoint on an operator-trusted network using the native task owner. --task-route is a JSON file with Name and Configuration; --plugin-dir contains compatible native .so plugins. Keep this worker alive for its actions and sessions; restarting does not resume them. Delivery budgets belong to the selected implementation and caller context. Missing acceptance or completion is an unresolved outcome, never proof of native failure. No automatic replay or host provisioning is provided. Release workflow scopes before stopping the worker.

Example: gimbal message-worker --plugin-dir /abs/plugins --task-route @/abs/filesystem.json
Filesystem Configuration: {"Directory":"/abs/private-messages"}. Events remain until their configured store is removed.`, RunE: func(c *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := invocation.LoadPlugins(dirs); err != nil {
			return err
		}
		route, err := readRoute(routeFile)
		if err != nil {
			return err
		}
		d, ok := gimbal.DefaultRegistry.Messaging(route.Name)
		if !ok {
			return fmt.Errorf("unknown messaging plugin %q", route.Name)
		}
		if err = d.ValidateConfiguration(route.Configuration); err != nil {
			return err
		}
		connection, err := gimbal.DefaultRegistry.Connect(ctx, route)
		if err != nil {
			return err
		}
		defer func() { _ = connection.Close() }()
		if _, err := fmt.Fprintln(c.OutOrStdout(), "Messaging worker:", route.Name); err != nil {
			return err
		}
		daemon := taskdaemon.New()
		daemon.Secrets = connection.Secrets
		if secretsDirectory != "" {
			daemon.Secrets, err = secretfiles.Open(secretsDirectory)
			if err != nil {
				return err
			}
		}
		dependencies := backend.Dependencies{Orchestrator: connection.Orchestrator, Secrets: daemon.Secrets}
		if readyFile != "" {
			dependencies.Ready = func() error { return os.WriteFile(readyFile, []byte("ready\n"), 0600) }
			defer func() { _ = os.Remove(readyFile) }()
		}
		err = d.Worker(ctx, route.Configuration, dependencies, daemon.Messages(connection.Messages))
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}}
	c.Flags().StringVar(&readyFile, "ready-file", "", "private readiness file written after worker starts listening")
	c.Flags().StringVar(&secretsDirectory, "secrets-directory", "", "private shared scoped secrets directory")
	c.Flags().StringVar(&routeFile, "task-route", "", "messaging route JSON or @file")
	c.Flags().StringSliceVar(&dirs, "plugin-dir", nil, "native messaging plugin directories")
	return c
}
func newMessagingWatchCommand() *cobra.Command {
	var routeFile, run, request, since string
	var dirs []string
	c := &cobra.Command{Use: "message-watch", Short: "Independently observe retained and live messaging facts", Long: `Listen without consuming another caller's events. Optional --since is an inclusive RFC3339 time; omission includes all retained history. Ending this command never cancels native work. Output is JSONL and may contain private agent content; keep captures outside Git.

Example: gimbal message-watch --plugin-dir /abs/plugins --task-route @/abs/filesystem.json --run RUN_ID`, RunE: func(c *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := invocation.LoadPlugins(dirs); err != nil {
			return err
		}
		route, err := readRoute(routeFile)
		if err != nil {
			return err
		}
		connection, err := gimbal.DefaultRegistry.Connect(ctx, route)
		if err != nil {
			return err
		}
		defer func() { _ = connection.Close() }()
		if connection.Messages == nil {
			return fmt.Errorf("route is not a messaging plugin")
		}
		var start time.Time
		if since != "" {
			start, err = time.Parse(time.RFC3339Nano, since)
			if err != nil {
				return err
			}
		}
		encoder := json.NewEncoder(c.OutOrStdout())
		err = connection.Messages.ListenEvents(ctx, backend.MessageSelection{Run: run, RequestID: request}, start, func(e backend.MessageEvent) error { return encoder.Encode(e) })
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}}
	c.Flags().StringVar(&routeFile, "task-route", "", "messaging route JSON or @file")
	c.Flags().StringSliceVar(&dirs, "plugin-dir", nil, "native messaging plugin directories")
	c.Flags().StringVar(&run, "run", "", "run identity filter")
	c.Flags().StringVar(&request, "request", "", "request identity filter")
	c.Flags().StringVar(&since, "since", "", "inclusive RFC3339 starting time")
	return c
}
