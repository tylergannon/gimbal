package web

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestDirectRunDoesNotJoinAnInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	root := t.TempDir()
	hostedProject := filepath.Join(root, "hosted")
	directProject := filepath.Join(root, "direct")
	for _, dir := range []string{hostedProject, directProject} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	instance, err := NewInstance(ctx, filepath.Join(root, "instance"), nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	project, err := instance.Owner.AdmitProject(hostedProject)
	if err != nil {
		t.Fatal(err)
	}
	if err := gimbal.Run(gimbal.Project(ctx, directProject), "direct", nil, func(ctx context.Context) error {
		gimbal.Set(ctx, "message", "direct-ok")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runs, err := os.ReadDir(filepath.Join(directProject, "runs"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("direct durable runs = %v, %v", runs, err)
	}
	id := runs[0].Name()
	if _, err := os.Stat(filepath.Join(directProject, "runs", id, "run.json")); err != nil {
		t.Fatalf("direct run row: %v", err)
	}
	if _, err := project.Registry().Snapshot(id); !errors.Is(err, observation.ErrNoRun) {
		t.Fatalf("instance unexpectedly observed direct run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hostedProject, ".gimbal", "runs", id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("direct run appeared in hosted project: %v", err)
	}
}

func TestRuntimeListenerOptionsConflict(t *testing.T) {
	_, _, err := newProject(t.Context(), t.TempDir(), WithPort(0), WithNoWeb())
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflicting listener options: %v", err)
	}
	if _, _, err := newProject(t.Context(), t.TempDir(), WithPort(65536)); err == nil {
		t.Fatal("invalid port was accepted")
	}
	if _, _, err := newProject(t.Context(), t.TempDir(), WithUDS("  ")); err == nil {
		t.Fatal("blank UDS path was accepted")
	}
}

func TestRuntimeServesWebApplicationOverUDSAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	socketDir, err := os.MkdirTemp("/tmp", "gimbal-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "gimbal.sock")
	instance, _, err := newProject(ctx, t.TempDir(), WithUDS(socket))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://gimbal/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read response: %v; close response: %v", readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Projects") {
		t.Fatalf("GET /: status %d, body %q", response.StatusCode, body)
	}
	cancel()
	<-instance.done
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket still exists after shutdown: %v", err)
	}
}

func TestRuntimeUsesSelectedArbitraryPortAndShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	instance, _, err := newProject(ctx, t.TempDir(), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + instance.address + "/")
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", response.StatusCode)
	}
	cancel()
	<-instance.done
	if conn, err := net.Dial("tcp", instance.address); err == nil {
		_ = conn.Close()
		t.Fatal("TCP listener remained open after shutdown")
	}
}

func TestRuntimeRunsWithoutWeb(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	instance, runtime, err := newProject(ctx, t.TempDir(), WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run(ctx, "headless", nil, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-instance.done
}
