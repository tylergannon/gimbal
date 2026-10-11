package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"plugin"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/runner"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func serveEnvironment(ctx context.Context, address, root string, dirs []string) (func(), error) {
	return serveEnvironmentOptions(ctx, address, root, dirs, "", "")
}
func serveEnvironmentOptions(ctx context.Context, address, root string, dirs []string, workdir, publicHost string) (func(), error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	daemon := taskdaemon.New()
	daemon.Workdir = workdir
	daemon.PublicHost = publicHost
	mux := http.NewServeMux()
	mux.Handle("/tasks/", daemon.Handler())
	workflowDaemon := &runner.WorkflowDaemon{Root: root, PluginDirs: dirs}
	mux.Handle("/workflows/", workflowDaemon.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close() }, nil
}
func newDaemonCommand() *cobra.Command {
	var address, root, workdir, publicHost string
	var dirs []string
	c := &cobra.Command{Use: "daemon", Short: "Execute agent sessions, commands and scope-owned services", Long: "Run a task daemon independently of central. Pass its HTTP endpoint as --task-endpoint when starting a workflow. Workflow and task workspaces belong to this environment; remote command artifacts are transferred as bytes. Provider configuration belongs to this daemon. Use loopback or a private container network; this endpoint executes trusted workflow requests. Release active scopes before stopping this daemon; task-daemon crash recovery is not supported.", RunE: func(c *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := invocation.LoadPlugins(dirs); err != nil {
			return err
		}
		stop, err := serveEnvironmentOptions(ctx, address, root, dirs, workdir, publicHost)
		if err != nil {
			return err
		}
		defer stop()
		if _, err := fmt.Fprintln(c.OutOrStdout(), "Task daemon listening on", address); err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	}}
	c.Flags().StringVar(&workdir, "workdir", "", "task repository workdir in this environment")
	c.Flags().StringVar(&publicHost, "public-host", "", "service host reachable by the declared consumer")
	c.Flags().StringVar(&root, "workspace-root", "", "workflow workspace root on this daemon")
	c.Flags().StringSliceVar(&dirs, "plugin-dir", nil, "native workflow/backend plugin directories on this daemon")
	c.Flags().StringVar(&address, "listen", "127.0.0.1:8081", "task HTTP listen address")
	return c
}
func newEnvironmentCommand() *cobra.Command {
	var filename, configPath, receiptPath string
	c := &cobra.Command{Use: "environment", Short: "Start or stop an execution environment using a native Go plugin", Long: "Load a native environment plugin built with the same Go toolchain and sources as this binary. The Docker plugin lives in plugins/docker. Its JSON config names Name, Image, Port, explicit Mounts, and Env variable names. Start prints the daemon endpoint for --task-endpoint. Start writes an ownership receipt with --receipt, including after readiness failure. Stop releases only the receipt-owned environment and interrupts its tasks; scope cleanup releases services without stopping the environment."}
	c.PersistentFlags().StringVar(&filename, "plugin", "", "native Go plugin (.so)")
	c.PersistentFlags().StringVar(&configPath, "config", "", "environment JSON configuration file")
	c.PersistentFlags().StringVar(&receiptPath, "receipt", "", "ownership receipt file outside Git (required)")
	for _, action := range []string{"start", "stop"} {
		c.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if receiptPath == "" {
				return fmt.Errorf("--receipt is required to retain environment ownership")
			}
			inputPath := configPath
			if action == "stop" {
				inputPath = receiptPath
			}
			raw, err := os.ReadFile(inputPath)
			if err != nil {
				return err
			}
			p, err := plugin.Open(filename)
			if err != nil {
				return err
			}
			if action == "start" {
				symbol, err := p.Lookup("Start")
				if err != nil {
					return err
				}
				start, ok := symbol.(func(context.Context, json.RawMessage) (json.RawMessage, error))
				if !ok {
					return fmt.Errorf("invalid environment Start signature")
				}
				receipt, startErr := start(cmd.Context(), raw)
				if len(receipt) > 0 {
					if err := os.WriteFile(receiptPath, receipt, 0600); err != nil {
						return errors.Join(startErr, fmt.Errorf("retain ownership receipt: %w", err))
					}
					var value struct{ Endpoint string }
					if err := json.Unmarshal(receipt, &value); err != nil {
						return errors.Join(startErr, err)
					}
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), value.Endpoint); err != nil {
						return errors.Join(startErr, err)
					}
				}
				return startErr
			}
			symbol, err := p.Lookup("Stop")
			if err != nil {
				return err
			}
			stop, ok := symbol.(func(context.Context, json.RawMessage) error)
			if !ok {
				return fmt.Errorf("invalid environment Stop signature")
			}
			return stop(cmd.Context(), raw)
		}})
	}
	return c
}
