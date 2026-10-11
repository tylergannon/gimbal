package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fakeDocker(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$DOCKER_CALLS\"\n" + body
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("DOCKER_CALLS", log)
	return log
}
func TestStopChecksReceiptOwnership(t *testing.T) {
	log := fakeDocker(t, `case "$1" in
 inspect) printf '{"gimbal.owner":"someone-else"}\n';;
 rm) exit 0;;
 esac
`)
	raw, _ := json.Marshal(ownership{ID: "owned-id", Label: "our-label"})
	if err := Stop(context.Background(), raw); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected refusal, got %v", err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "rm ") {
		t.Fatalf("removed unowned container: %s", calls)
	}
}
func TestStartFailureDoesNotAcquireConflictingName(t *testing.T) {
	log := fakeDocker(t, `echo 'name already in use' >&2; exit 1`)
	raw := json.RawMessage(`{"Name":"existing","Image":"gimbal","Port":8081}`)
	receipt, err := Start(context.Background(), raw)
	if err == nil || len(receipt) != 0 {
		t.Fatalf("conflict acquired receipt: %s %v", receipt, err)
	}
	if err := Stop(context.Background(), receipt); err == nil {
		t.Fatal("empty receipt accepted")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "rm ") {
		t.Fatalf("removed existing container: %s", calls)
	}
}
func TestStopOwnedAndAbsent(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(strconv.FormatBool(absent), func(t *testing.T) {
			body := `case "$1" in
 inspect) printf '{"gimbal.owner":"our-label"}\n';;
 rm) exit 0;;
 esac
`
			if absent {
				body = `echo 'Error: No such object: owned-id' >&2; exit 1`
			}
			log := fakeDocker(t, body)
			raw, _ := json.Marshal(ownership{ID: "owned-id", Label: "our-label"})
			if err := Stop(context.Background(), raw); err != nil {
				t.Fatal(err)
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "rm -f owned-id") == absent {
				t.Fatalf("unexpected release: %s", calls)
			}
		})
	}
}
func TestStartRetainsReceiptAfterReadinessCancellation(t *testing.T) {
	fakeDocker(t, `echo owned-id`)
	ctx, cancel := context.WithCancel(context.Background())
	// The health handler cancels readiness after creation.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cancel(); w.WriteHeader(503) }))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	raw, _ := json.Marshal(config{Name: "ours", Image: "gimbal", Port: port})
	receipt, err := Start(ctx, raw)
	if err == nil {
		t.Fatal("readiness unexpectedly succeeded")
	}
	var value ownership
	if err := json.Unmarshal(receipt, &value); err != nil {
		t.Fatal(err)
	}
	if value.ID != "owned-id" || value.Label == "" || value.Endpoint == "" {
		t.Fatalf("lost ownership: %+v", value)
	}
}
