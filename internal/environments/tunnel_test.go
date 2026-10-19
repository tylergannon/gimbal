package environments

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

// This child is a controlled tunnel command for the normal unit suite. It
// forwards real TCP without requiring an installed or configured SSH server.
// The opt-in test below exercises the same contract through system OpenSSH.
func runTunnelFixture(args []string) {
	forward := ""
	for i, arg := range args {
		if arg == "-L" && i+1 < len(args) {
			forward = args[i+1]
			break
		}
	}
	parts := strings.Split(forward, ":")
	if len(parts) != 4 {
		fmt.Fprintln(os.Stderr, "missing fixture forwarding argument")
		os.Exit(2)
	}
	listener, e := net.Listen("tcp", net.JoinHostPort(parts[0], parts[1]))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	destination := net.JoinHostPort(parts[2], parts[3])
	for {
		source, e := listener.Accept()
		if e != nil {
			return
		}
		go func() {
			defer func() { _ = source.Close() }()
			target, e := net.Dial("tcp", destination)
			if e != nil {
				return
			}
			defer func() { _ = target.Close() }()
			go func() {
				_, _ = io.Copy(target, source)
				if conn, ok := target.(*net.TCPConn); ok {
					_ = conn.CloseWrite()
				}
			}()
			_, _ = io.Copy(source, target)
		}()
	}
}
func TestAcquireRebindsSavedSSHPlacementAfterTunnelExit(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nexec " + shellQuote(os.Args[0]) + " --test-ssh-tunnel \"$@\"\n"
	if e := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	verifyTunnelReplacement(t, "controlled-tunnel")
}
func TestSystemSSHRebindsSavedPlacementAfterTunnelExit(t *testing.T) {
	config := os.Getenv("GIMBAL_TEST_SSH_CONFIG")
	if config == "" {
		t.Skip("set GIMBAL_TEST_SSH_CONFIG to a private loopback SSH fixture config")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nexec /usr/bin/ssh -F " + shellQuote(config) + " \"$@\"\n"
	if e := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	verifyTunnelReplacement(t, "gimbal-proof")
}
func verifyTunnelReplacement(t *testing.T, host string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	daemon := taskdaemon.New(t.TempDir())
	handler := daemon.Handler()
	var admissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		if r.URL.Path == "/tasks/process" {
			admissions.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	parsed, e := url.Parse(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	_, port, e := net.SplitHostPort(parsed.Host)
	if e != nil {
		t.Fatal(e)
	}
	var daemonPort int
	if _, e = fmt.Sscanf(port, "%d", &daemonPort); e != nil {
		t.Fatal(e)
	}
	s := testService(t)
	saved := Target{Machine: Machine{ID: "ssh-machine", Name: "SSH fixture", Kind: KindSSH, SSH: &SSH{Host: host, DaemonPort: daemonPort}}, Directory: t.TempDir()}
	owner := backend.Owner{Run: "retained-conversation", Scope: "retained-conversation", Task: "original-request"}
	first, e := s.Acquire(ctx, saved, owner)
	if e != nil {
		t.Fatal(e)
	}
	command := backend.Process{Owner: owner, Command: "/bin/sh", Args: []string{"-c", "sleep 0.2; printf retained-result"}}
	if e = first.Connection.Execution.Process(ctx, command); e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	var previous *tunnel
	for _, tunnel := range s.tunnels {
		previous = tunnel
		break
	}
	s.mu.Unlock()
	if previous == nil {
		t.Fatal("SSH did not create an owned tunnel")
	}
	if e = previous.close(); e != nil {
		t.Fatal(e)
	}
	oldEndpoint, e := url.Parse(previous.endpoint)
	if e != nil {
		t.Fatal(e)
	}
	// Reserve the old port so the replacement must expose a different transient
	// endpoint, rather than accidentally passing by reusing the same address.
	reserved, e := net.Listen("tcp", oldEndpoint.Host)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = reserved.Close() }()
	// A persisted target may contain the previous derived route, but SSH machine
	// identity must still determine the connection used for the original request.
	saved.Route = first.Target.Route
	next, e := s.Acquire(ctx, saved, owner)
	if e != nil {
		t.Fatal(e)
	}
	if string(first.Target.Route.Configuration) == string(next.Target.Route.Configuration) {
		t.Fatal("SSH reconnect reused an expired local port")
	}
	readCtx := effects.WithConnection(ctx, next.Connection)
	result, e := effects.Wait(readCtx, "backend", backend.Target{Run: owner.Run, ID: owner.Task}, nil)
	if e != nil || len(result) == 0 {
		t.Fatalf("original request did not reconcile on replacement tunnel: %s %v", result, e)
	}
	reader, e := next.Connection.Execution.ReadBytes(ctx, backend.Artifact{Owner: owner, Name: "stdout"})
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(reader)
	_ = reader.Close()
	if e != nil || string(data) != "retained-result" {
		t.Fatalf("retained result lost: %s %v", data, e)
	}
	if admissions.Load() != 1 {
		t.Fatalf("tunnel replacement replayed native work: %d admissions", admissions.Load())
	}
	if e = next.Connection.Execution.Cleanup(ctx, backend.Scope{Run: owner.Run, Scope: owner.Scope}); e != nil {
		t.Fatal(e)
	}
}
