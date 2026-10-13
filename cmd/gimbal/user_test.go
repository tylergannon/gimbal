package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal/auth"
)

func userControlFixture(t *testing.T, dir, filename string, handler http.Handler) {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "gimbal-user-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "control.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	if err := os.MkdirAll(filepath.Join(dir, "control"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(runtimeDiscovery{PID: os.Getpid(), Socket: socket})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "control", filename+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUserCommandsUseLiveSelectedControlSocket(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	var operations []string
	userControlFixture(t, dir, "live", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		operations = append(operations, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/control/live":
			w.WriteHeader(http.StatusOK)
		case "/control/users":
			var identity auth.Identity
			if err := json.NewDecoder(r.Body).Decode(&identity); err != nil {
				t.Error(err)
			}
			if identity != (auth.Identity{Issuer: "https://github.com", Subject: "123456", Name: "Tyler"}) {
				t.Errorf("identity = %#v", identity)
			}
			_, _ = w.Write([]byte(`{"id":"stable-local-id","name":"Tyler"}`))
		case "/control/users/stable-local-id":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	if err := os.WriteFile(filepath.Join(dir, "control", "stale.json"), []byte(`{"pid":1,"socket":"/tmp/gimbal-nonexistent-user.sock"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"user", "add", "--instance-dir", dir, "--issuer", "https://github.com", "--subject", "123456", "--name", "Tyler"},
		{"user", "remove", "--instance-dir", dir, "stable-local-id"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, &stdout, &stderr, os.Getenv); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != "stable-local-id\n" {
			t.Fatalf("output = %q", stdout.String())
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(operations, ","); got != "GET /control/live,POST /control/users,GET /control/live,DELETE /control/users/stable-local-id" {
		t.Fatalf("requests = %s", got)
	}
}

func TestUserCommandsRequireAbsoluteInstanceAndProvisionedIdentity(t *testing.T) {
	for _, args := range [][]string{
		{"user", "remove", "some-id"},
		{"user", "remove", "--instance-dir", "relative", "some-id"},
		{"user", "add", "--instance-dir", t.TempDir(), "--issuer", "https://github.com", "--name", "Tyler"},
	} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}, os.Getenv); err == nil {
			t.Fatalf("%v unexpectedly succeeded", args)
		}
	}
}

func TestUserCommandsRefuseAuthOffAndAmbiguousInstances(t *testing.T) {
	dir := t.TempDir()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/control/live" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "authentication is disabled", http.StatusConflict)
	})
	userControlFixture(t, dir, "one", handler)
	args := []string{"user", "remove", "--instance-dir", dir, "some-id"}
	if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}, os.Getenv); err == nil || !strings.Contains(err.Error(), "authentication is disabled") {
		t.Fatalf("auth-off error = %v", err)
	}
	userControlFixture(t, dir, "two", handler)
	if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}, os.Getenv); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("ambiguous instance error = %v", err)
	}
}

func TestUserHelpExplainsGitHubNumericIdentity(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"user", "--help"}, &out, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "gh api users/LOGIN --jq .id") {
		t.Fatalf("help = %q", out.String())
	}
}

func TestUserCommandsNeedNoCommandSecrets(t *testing.T) {
	if needsSecrets([]string{"user", "add"}) || needsSecrets([]string{"user", "remove", "id"}) {
		t.Fatal("trusted control clients must not load provider secrets")
	}
}
