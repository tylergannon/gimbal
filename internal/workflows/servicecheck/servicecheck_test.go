package servicecheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNodeServiceAndConsumer(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	// Check the actual argument/SQL contract, then supply a database response.
	sql := `#!/bin/sh
set -eu
test "$1" = -X
test "$2" = fixture-database
test "$3" = -Atc
test "$4" = "SELECT 'GIMBAL_SERVICE_OK'"
printf 'GIMBAL_SERVICE_OK\n'
`
	if err := os.WriteFile(filepath.Join(bin, "psql"), []byte(sql), 0700); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(root, "service.env")
	command := exec.Command("sh", "-c", web)
	command.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "DATABASE_URL=fixture-database", "ENV_FILE="+envFile)
	command.Dir = root
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		if err := command.Wait(); err != nil {
			t.Errorf("service shutdown: %v", err)
		}
	})
	var address string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(envFile)
		if err == nil {
			address = strings.TrimSpace(strings.TrimPrefix(string(raw), "WEB_URL="))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if address == "" {
		t.Fatal("service did not publish its bound address")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	consumer := exec.CommandContext(ctx, "node", "--input-type=module", "-e", consume)
	consumer.Env = append(os.Environ(), "WEB_URL="+address)
	raw, err := consumer.CombinedOutput()
	if err != nil {
		t.Fatalf("consumer: %v: %s", err, raw)
	}
	if !strings.Contains(string(raw), "Command fetched database-backed service: GIMBAL_SERVICE_OK") {
		t.Fatal(string(raw))
	}
}
func TestNodeConsumerRejectsBadResponse(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
	}{{500, "GIMBAL_SERVICE_OK"}, {200, "wrong response"}} {
		t.Run(fmt.Sprint(test.status, test.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "node", "--input-type=module", "-e", consume)
			command.Env = append(os.Environ(), "WEB_URL="+server.URL)
			if raw, err := command.CombinedOutput(); err == nil {
				t.Fatalf("bad response accepted: %s", raw)
			}
		})
	}
}
