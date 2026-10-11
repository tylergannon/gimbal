package observation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandOutputServesRetainedBytesAndConfinesAccess(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "runs", "run")
	output := filepath.Join(dir, "artifacts", "commands", "output.log")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		t.Fatal(err)
	}
	full := strings.Repeat("complete stdout bytes\n", 4096)
	if err := os.WriteFile(output, []byte(full), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(state, "private.txt")
	if err := os.WriteFile(outside, []byte("not run output"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(filepath.Dir(output), "outside.log")); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i, event := range []map[string]any{
		{"kind": "run_started", "name": "run"},
		{"kind": "command_started", "id": "scope.1/check.1", "name": "check", "command": "tool"},
		{"kind": "command_ended", "id": "scope.1/check.1", "stdout": "short excerpt", "stdout_file": "artifacts/commands/output.log", "stderr_file": "artifacts/commands/outside.log"},
	} {
		raw, err := json.Marshal(map[string]any{"seq": i + 1, "time": "2026-10-10T00:00:00Z", "scope": "", "event": event})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(state)
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/api/runs/run/commands/stdout/scope.1%2Fcheck.1", full, 200},
		{"/api/runs/run/commands/stderr/scope.1%2Fcheck.1", "", 404},
		{"/api/runs/run/commands/stdout/missing", "", 404},
		{"/api/runs/run/commands/private.txt/scope.1%2Fcheck.1", "", 404},
		{"/api/runs/other/commands/stdout/scope.1%2Fcheck.1", "", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req = req.WithContext(WithRegistry(req.Context(), registry))
			response := httptest.NewRecorder()
			Routes(http.NotFoundHandler()).ServeHTTP(response, req)
			if response.Code != tc.status || (tc.status == 200 && response.Body.String() != tc.body) {
				t.Fatalf("status=%d bytes=%d, want status=%d bytes=%d", response.Code, response.Body.Len(), tc.status, len(tc.body))
			}
		})
	}
}
