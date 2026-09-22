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
)

// TestRunPageIsRenderedFromTheRunsObservation is the SSR boundary end to end:
// the Go load reads the registry the request carries, the observation crosses
// as the app's transported type, and the document the browser would receive
// with JavaScript disabled already holds it.
func TestRunPageIsRenderedFromTheRunsObservation(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, mode, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if mode != "prod" {
		t.Fatalf("this test renders the embedded build, but the handler is in %s mode", mode)
	}

	project := t.TempDir()
	registry := observation.NewRegistry(project)
	runDir := filepath.Join(project, "runs", "run-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, "run-1", "demo", runDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-13T00:00:00Z","scope":"","session":"writer","event":{"kind":"session_created","name":"writer","adapter":"fixture","model":"test-model","workdir":"/w"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-13T00:00:01Z","scope":"","session":"writer","turn":"turn-1","event":{"kind":"turn_started","prompt":"write","output_type":"gimble.Text"}}`),
	} {
		if err := store.Lifecycle(record); err != nil {
			t.Fatalf("fold %s: %v", record, err)
		}
	}
	at := observation.Placement{Session: "writer", Turn: "turn-1"}
	for _, event := range []json.RawMessage{
		json.RawMessage(`{"id":"evt1","type":"session.step.started","created":10,"data":{"sessionID":"ses_writer","assistantMessageID":"msg_writer","agent":"fixture","model":{"providerID":"fixture","id":"test-model"}}}`),
		json.RawMessage(`{"id":"evt2","type":"session.text.started","created":11,"data":{"sessionID":"ses_writer","assistantMessageID":"msg_writer","ordinal":0}}`),
		json.RawMessage(`{"id":"evt3","type":"session.text.ended","created":12,"data":{"sessionID":"ses_writer","assistantMessageID":"msg_writer","ordinal":0,"text":"hello from Gimble"}}`),
	} {
		if err := store.Event(at, event, nil); err != nil {
			t.Fatalf("observe event: %v", err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/projects/test/runs/run-1", nil)
	request = request.WithContext(observation.WithRegistry(request.Context(), registry))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /runs/run-1: status %d, body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	// The value travels under the key src/hooks.go and src/hooks.ts both
	// spell; a document without it is one the client could not decode.
	if !strings.Contains(body, "RunSnapshot") {
		t.Fatalf("the rendered document carries no transported snapshot:\n%s", body)
	}
	// And what it carries is this run's actual observation, not an empty one.
	for _, want := range []string{"run-1", "turn-1", "hello from Gimble", "test-model"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the rendered document does not mention %q:\n%s", want, body)
		}
	}

	// An unknown run is an ordinary 404, not an empty page.
	missing := httptest.NewRequest(http.MethodGet, "/projects/test/runs/nope", nil)
	missing = missing.WithContext(observation.WithRegistry(missing.Context(), registry))
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, missing)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET /runs/nope: status %d, want 404", recorder.Code)
	}
}

// TestRunPageRequiresGenerationWhenTheGraphIsUnavailable exercises the
// corrective state without changing the saved run.
func TestRunPageRequiresGenerationWhenTheGraphIsUnavailable(t *testing.T) {
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
	runDir := filepath.Join(project, "runs", "run-loop")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, "run-loop", "looping", runDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-15T00:00:00Z","scope":"","event":{"kind":"run_started","name":"looping"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-15T00:00:01Z","scope":"sprint.1","event":{"kind":"scope_began","name":"sprint.1","loop":true}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-15T00:00:02Z","scope":"quiet.1","event":{"kind":"scope_began","name":"quiet.1","loop":false}}`),
	} {
		if err := store.Lifecycle(record); err != nil {
			t.Fatalf("fold %s: %v", record, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/projects/test/runs/run-loop", nil)
	request = request.WithContext(observation.WithRegistry(request.Context(), registry))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /runs/run-loop: status %d, body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{
		"Workflow graph required",
		"No generated graph is registered for looping",
		"go generate ./...",
		"just build",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the graph guidance does not contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `aria-label="Recorded run history"`) {
		t.Fatal("the missing-graph page still rendered history navigation")
	}
}
