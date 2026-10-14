//go:build darwin || linux

package gimbal

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

func TestDefaultLocalServicePublishesAndReleases(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	workdir := t.TempDir()
	if err = os.WriteFile(filepath.Join(workdir, "value.txt"), []byte("local-service-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	var address string
	err = Run(Project(t.Context(), t.TempDir()), "local-service", nil, func(ctx context.Context) error {
		return Scope(ctx, "http", func(ctx context.Context) error {

			command := "exec env GIMBAL_LOCAL_SERVICE_FIXTURE=1 '" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "' -test.run '^TestLocalServiceHTTPFixture$'"
			if err := Service(WithYields(ctx, DefaultStore, "SERVICE_URL"), "server", workdir, command); err != nil {
				return err
			}
			values, err := effects.Connection(ctx).Secrets.Acquire(ctx, backend.Scope{Run: effectOwner(mustScope(ctx), "").Run, Scope: mustScope(ctx).key}, backend.Requirements{{Names: []string{"SERVICE_URL"}}})
			if err != nil {
				return err
			}
			address = values["SERVICE_URL"]
			response, err := http.Get(address + "/value.txt")
			if err != nil {
				return err
			}
			defer func() { _ = response.Body.Close() }()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if string(raw) != "local-service-bytes" {
				return fmt.Errorf("service returned %q", raw)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if response, err := http.Get(address + "/value.txt"); err == nil {
		_ = response.Body.Close()
		t.Fatal("scope-owned service remained reachable")
	}
}

func mustScope(ctx context.Context) *scope {
	s, err := current(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

// The service fixture is the same Go test executable, with no external runtime.
func TestLocalServiceHTTPFixture(t *testing.T) {
	if os.Getenv("GIMBAL_LOCAL_SERVICE_FIXTURE") != "1" {
		t.Skip("subprocess fixture")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	server := &http.Server{Handler: http.FileServer(http.Dir("."))}
	defer func() { _ = server.Close() }()
	failures := make(chan error, 1)
	go func() { failures <- server.Serve(listener) }()
	path := os.Getenv("ENV_FILE")
	if path == "" {
		t.Fatal("ENV_FILE is required")
	}
	if err := os.WriteFile(path+".tmp", []byte("SERVICE_URL=http://"+listener.Addr().String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case err := <-failures:
		t.Fatal(err)
	}
}
