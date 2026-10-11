package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"plugin"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func serveTasks(ctx context.Context, address string) (func(), error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	daemon := taskdaemon.New()
	mux := http.NewServeMux()
	mux.Handle("/tasks/", daemon.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close() }, nil
}
func newDaemonCommand() *cobra.Command {
	var address string
	c := &cobra.Command{Use: "daemon", Short: "Execute agent sessions, commands and scope-owned services", Long: "Run a task daemon independently of central. Pass its HTTP endpoint as --task-endpoint when starting a workflow. Workspaces and run artifacts must exist at identical absolute paths in both environments. Provider configuration belongs to this daemon. Use loopback or a private container network; this endpoint executes trusted workflow requests. Release active scopes before stopping this daemon; task-daemon crash recovery is not supported.", RunE: func(c *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		stop, err := serveTasks(ctx, address)
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
	c.Flags().StringVar(&address, "listen", "127.0.0.1:8081", "task HTTP listen address")
	return c
}
func newEnvironmentCommand() *cobra.Command {
	var filename, configPath string
	c := &cobra.Command{Use: "environment", Short: "Start or stop an execution environment using a native Go plugin", Long: "Load a native environment plugin built with the same Go toolchain and sources as this binary. The Docker plugin lives in plugins/docker. Its JSON config names Name, Image, Port, explicit Mounts, and Env variable names. Start prints the daemon endpoint for --task-endpoint. Stop removes that named environment and interrupts its tasks; scope cleanup releases services without stopping the environment."}
	c.PersistentFlags().StringVar(&filename, "plugin", "", "native Go plugin (.so)")
	c.PersistentFlags().StringVar(&configPath, "config", "", "environment JSON configuration file")
	for _, action := range []string{"start", "stop"} {
		c.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := os.ReadFile(configPath)
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
				start, ok := symbol.(func(context.Context, json.RawMessage) (string, error))
				if !ok {
					return fmt.Errorf("invalid environment Start signature")
				}
				endpoint, err := start(cmd.Context(), raw)
				if err == nil {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), endpoint)
				}
				return err
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
