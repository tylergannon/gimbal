package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

func runWorker(ctx context.Context, config workerConfig, output io.Writer, ready func()) error {
	if config.service {
		if !filepath.IsAbs(config.workdir) || !filepath.IsAbs(config.stateDir) || !filepath.IsAbs(config.envFile) {
			return errors.New("service requires absolute --workdir, --executor-state-dir and --env-file paths")
		}
	}
	if err := applyEnvironment(config.envFile, userEnvironment); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	workdir, err := filepath.Abs(config.workdir)
	if err != nil {
		return err
	}
	info, err := os.Stat(workdir)
	if err != nil {
		return fmt.Errorf("task workdir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("task workdir is not a directory: %s", workdir)
	}
	stateDir, err := executor.Root(config.stateDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	// The bridge lease is independent of the command and agent owner leases.
	lease, err := executor.Lock(filepath.Join(stateDir, "taskdaemon"))
	if err != nil {
		return err
	}
	if lease == nil {
		return fmt.Errorf("another worker owns %s", stateDir)
	}
	defer lease.Close()
	daemon := taskdaemon.New(stateDir)
	daemon.Workdir = workdir
	daemon.Secrets = backend.NewMemorySecrets()
	if err := daemon.Ready(); err != nil {
		return fmt.Errorf("task state: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/tasks/", daemon.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	listener, err := net.Listen("tcp", config.address)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: mux}
	// Close only HTTP, never cancel the daemon's independent execution owners.
	defer server.Close()
	stopped := make(chan error, 1)
	go func() { stopped <- server.Serve(listener) }()
	if _, err := fmt.Fprintf(output, "Task worker listening on %s; workdir=%s; state=%s\n", listener.Addr(), workdir, stateDir); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ready()
	select {
	case <-ctx.Done():
		// Close the old listener before releasing the bridge lease for restart.
		if err := server.Close(); err != nil {
			return err
		}
		<-stopped
		return nil
	case err := <-stopped:
		return err
	}
}
