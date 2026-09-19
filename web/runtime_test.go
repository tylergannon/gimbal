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
)

func TestRuntimeListenerOptionsConflict(t *testing.T) {
	_, err := NewRuntime(t.Context(), t.TempDir(), WithPort(0), WithNoWeb())
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflicting listener options: %v", err)
	}
	if _, err := NewRuntime(t.Context(), t.TempDir(), WithPort(65536)); err == nil {
		t.Fatal("invalid port was accepted")
	}
	if _, err := NewRuntime(t.Context(), t.TempDir(), WithUDS("  ")); err == nil {
		t.Fatal("blank UDS path was accepted")
	}
}

func TestRuntimeServesWebApplicationOverUDSAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	socketDir, err := os.MkdirTemp("/tmp", "gimble-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "gimble.sock")
	runtime, err := NewRuntime(ctx, t.TempDir(), WithUDS(socket))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://gimble/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read response: %v; close response: %v", readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Runs") {
		t.Fatalf("GET /: status %d, body %q", response.StatusCode, body)
	}
	cancel()
	<-runtime.done
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket still exists after shutdown: %v", err)
	}
}

func TestRuntimeUsesSelectedArbitraryPortAndShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, t.TempDir(), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + runtime.address + "/")
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
	<-runtime.done
	if conn, err := net.Dial("tcp", runtime.address); err == nil {
		_ = conn.Close()
		t.Fatal("TCP listener remained open after shutdown")
	}
}

func TestRuntimeRunsWithoutWeb(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, t.TempDir(), WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run(ctx, "headless", nil, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-runtime.done
}
