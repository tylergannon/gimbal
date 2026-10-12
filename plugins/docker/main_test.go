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

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
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

func TestEnvironmentConfigurationReachesDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	log := fakeDocker(t, `echo owned-id`)
	raw, _ := json.Marshal(config{Name: "tasks", Image: "gimbal", Port: port, Network: "private", PublicHost: "tasks", ServicePorts: []int{19091}, Workdir: "/workspace/repo", WorkspaceRoot: "/workspace", PluginDirs: []string{"/plugins", "/project-plugins"}})
	receipt, err := Start(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	for _, want := range []string{"--network private", "-p 127.0.0.1:19091:19091", "--plugin-dir /plugins --plugin-dir /project-plugins", "--public-host tasks", "--workdir /workspace/repo", "--workspace-root /workspace"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("missing %q: %s", want, calls)
		}
	}
	var owner ownership
	if err := json.Unmarshal(receipt, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.Network != "private" || owner.PublicHost != "tasks" || len(owner.ServicePorts) != 1 || owner.ServicePorts[0] != 19091 {
		t.Fatalf("lost service route: %+v", owner)
	}
	registry := &gimbal.Registry{}
	if err := Register(registry); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Connect(context.Background(), backend.Route{Name: "docker", Configuration: receipt}); err != nil {
		t.Fatalf("Start receipt cannot be used as a backend route: %v", err)
	}
}

func TestInvalidConfigurationDoesNotStartContainer(t *testing.T) {
	for _, fields := range []string{
		`"Port":65536`, `"ServicePorts":[8081],"PublicHost":"localhost"`,
		`"ServicePorts":[18081],"PublicHost":"localhost"`, `"ServicePorts":[19091,19091],"PublicHost":"localhost"`,
		`"ServicePorts":[-1],"PublicHost":"localhost"`, `"ServicePorts":[19091]`,
		`"PublicHost":"localhost"`, `"PluginDirs":["relative"]`, `"Workdir":"relative"`,
	} {
		t.Run(fields, func(t *testing.T) {
			log := fakeDocker(t, `echo should-not-start`)
			raw := json.RawMessage(`{"Name":"tasks","Image":"gimbal","Port":18081,` + fields + `}`)
			if receipt, err := Start(context.Background(), raw); err == nil || len(receipt) != 0 {
				t.Fatalf("invalid config accepted: %s %v", receipt, err)
			}
			if calls, err := os.ReadFile(log); err == nil {
				t.Fatalf("docker called: %s", calls)
			}
		})
	}
}
