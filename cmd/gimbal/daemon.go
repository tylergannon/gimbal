package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/runner"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func serveEnvironment(ctx context.Context, address, root string, dirs []string) (func(), error) {
	return serveEnvironmentOptions(ctx, address, root, dirs, "", "")
}
func serveEnvironmentOptions(ctx context.Context, address, root string, dirs []string, workdir, executorRoot string) (func(), error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	daemon := taskdaemon.New(executorRoot)
	daemon.Workdir = workdir
	daemon.Secrets = backend.NewMemorySecrets()
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
	var address, root, workdir, executorRoot string
	var dirs []string
	c := &cobra.Command{Use: "daemon", Short: "Execute agent sessions, commands and scope-owned services", Long: "Run a task daemon independently of central. Pass its HTTP endpoint as --task-endpoint when starting a workflow. Workflow and task workspaces belong to this environment; remote command artifacts are transferred as bytes. Provider configuration belongs to this daemon. Use loopback or a private container network; this endpoint executes trusted workflow requests. Release active scopes before stopping this daemon; task-daemon crash recovery is not supported.", RunE: func(c *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := invocation.LoadPlugins(dirs); err != nil {
			return err
		}
		stop, err := serveEnvironmentOptions(ctx, address, root, dirs, workdir, executorRoot)
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
	c.Flags().StringVar(&executorRoot, "executor-state-dir", "", "independent executor state directory")
	c.Flags().StringVar(&workdir, "workdir", "", "task repository workdir in this environment")
	c.Flags().StringVar(&root, "workspace-root", "", "workflow workspace root on this daemon")
	c.Flags().StringSliceVar(&dirs, "plugin-dir", nil, "native workflow/backend plugin directories on this daemon")
	c.Flags().StringVar(&address, "listen", "127.0.0.1:8081", "task HTTP listen address")
	return c
}
