package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/observation"
	hooks "github.com/tylergannon/gimble/web/src"
)

func TestRunsPageRendersEveryRunAndPendingInterview(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	registry := observation.NewRegistry(project)
	running := openTestRun(t, registry, project, "run-running", "plan-trip")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-18T12:00:00Z","scope":"","event":{"kind":"run_started","name":"plan-trip"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-18T12:00:10Z","scope":"research.1/lodging.1","session":"researcher.1","event":{"kind":"interview_question_asked","name":"preferences","question_id":"question-1","question":"Would you trade reliable Wi-Fi for a secluded cabin?"}}`),
	} {
		if err := running.Lifecycle(record); err != nil {
			t.Fatalf("fold running record %s: %v", record, err)
		}
	}

	ended := openTestRun(t, registry, project, "run-ended", "release")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-18T11:00:00Z","scope":"","event":{"kind":"run_started","name":"release"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-18T11:07:30Z","scope":"","event":{"kind":"run_ended","name":"release","error":""}}`),
	} {
		if err := ended.Lifecycle(record); err != nil {
			t.Fatalf("fold ended record %s: %v", record, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := observation.WithRegistry(request.Context(), registry)
	request = request.WithContext(hooks.WithProjectDir(ctx, project))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /: status %d, body %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"Runs",
		"run-running",
		"plan-trip",
		"Running",
		"Needs your answer",
		"Would you trade reliable Wi-Fi for a secluded cabin?",
		"run-ended",
		"release",
		"Ended",
		"7m 30s",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the runs page does not contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Agent workflows in Go") {
		t.Fatal("the old guide landing page is still rendered at /")
	}
}

func openTestRun(t *testing.T, registry *observation.Registry, project, id, name string) *observation.Store {
	t.Helper()
	dir := filepath.Join(project, "runs", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, id, name, dir)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
