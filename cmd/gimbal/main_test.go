package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRouteAnalysis(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	vetConfig := filepath.Join(dir, "vet.cfg")
	if err := os.WriteFile(vetConfig, []byte(`{"ImportPath":"example.com/p","GoFiles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ordinaryConfig := filepath.Join(dir, "settings.cfg")
	if err := os.WriteFile(ordinaryConfig, []byte("ordinary settings"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want []string
		ok   bool
	}{
		{name: "standalone", args: []string{"lint", "./..."}, want: []string{"./..."}, ok: true},
		{name: "vet version", args: []string{"-V=full"}, want: []string{"-V=full"}, ok: true},
		{name: "CLI long version", args: []string{"--version"}},
		{name: "CLI short version", args: []string{"-v"}},
		{name: "vet flags", args: []string{"-flags"}, want: []string{"-flags"}, ok: true},
		{name: "vet unit", args: []string{"-json", vetConfig}, want: []string{"-json", vetConfig}, ok: true},
		{name: "run prompt help with cfg argument", args: []string{"run-prompt", "-h", ordinaryConfig}},
		{name: "run prompt wins over real vet config", args: []string{"run-prompt", "-h", vetConfig}},
		{name: "upload artifact wins over real vet config", args: []string{"upload-artifact", "--provider", "r2", vetConfig}},
		{name: "server help with cfg socket", args: []string{"-h", "-uds", ordinaryConfig}},
		{name: "server wins over real vet config", args: []string{"-h", "-uds", vetConfig}},
		{name: "ordinary server", args: []string{"-no-web"}},
		{name: "run a workflow", args: []string{"run", "review", "--goal", "check the code"}},
		{name: "manage opencode", args: []string{"opencode", "start"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := routeAnalysis(test.args)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeAnalysis(%q) = (%q, %v), want (%q, %v)", test.args, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestCLIReportsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.2.3", "abc1234", "2026-10-12T17:00:00Z"
	t.Cleanup(func() { version, commit, date = oldVersion, oldCommit, oldDate })

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runWithUploaders([]string{flag}, &stdout, &stderr, func(string) string { return "" }, artifactUploaderConstructors{})
			if err != nil {
				t.Fatalf("run version command: %v (stderr: %q)", err, stderr.String())
			}
			const want = "gimbal version v1.2.3 (commit abc1234, built 2026-10-12T17:00:00Z)\n"
			if got := stdout.String(); got != want {
				t.Fatalf("version output = %q, want %q", got, want)
			}
		})
	}
}

type readyLineWriter chan string

func (w readyLineWriter) Write(data []byte) (int, error) {
	w <- string(data)
	return len(data), nil
}

func TestBundledServerReadyBindsEphemeralPortWithoutAdmittingCWD(t *testing.T) {
	instanceDir := filepath.Join(t.TempDir(), "desktop")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(readyLineWriter, 1)
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, serverFlags{
			instanceDir: instanceDir,
			noProject:   true,
			port:        0,
			ready:       true,
		}, ready)
	}()

	var line string
	select {
	case line = <-ready:
	case err := <-done:
		t.Fatalf("server exited before readiness: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not report readiness")
	}
	if !strings.HasPrefix(line, "READY http://127.0.0.1:") || !strings.HasSuffix(line, "\n") {
		t.Fatalf("readiness line = %q", line)
	}
	endpoint, err := url.Parse(strings.TrimSpace(strings.TrimPrefix(line, "READY ")))
	if err != nil {
		t.Fatalf("parse readiness URL: %v", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(endpoint.String())
	if err != nil {
		t.Fatalf("request bundled server at %s: %v", endpoint, err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatalf("read projects page: %v", readErr)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Open a project folder") {
		t.Fatalf("empty server response = %d, body contains empty state: %v", response.StatusCode, strings.Contains(string(body), "Open a project folder"))
	}
	if _, err := os.Stat(filepath.Join(instanceDir, "projects.json")); !os.IsNotExist(err) {
		t.Fatalf("empty desktop server unexpectedly persisted an implicit project catalog: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after its owner context was cancelled")
	}
}

func TestServerFlagConflictsFailBeforeStarting(t *testing.T) {
	tests := []struct {
		name  string
		flags serverFlags
		want  string
	}{
		{
			name:  "no project with explicit project",
			flags: serverFlags{noProject: true, projects: []string{"/tmp/project"}},
			want:  "--no-project cannot be combined with --project",
		},
		{
			name:  "ready with no web listener",
			flags: serverFlags{ready: true, noWeb: true},
			want:  "--ready requires the TCP web listener",
		},
		{
			name:  "ready with unix listener",
			flags: serverFlags{ready: true, uds: "/tmp/gimbal.sock"},
			want:  "--ready requires the TCP web listener",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := serve(context.Background(), test.flags, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("serve error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
