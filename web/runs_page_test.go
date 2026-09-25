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

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
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
		json.RawMessage(`{"seq":2,"time":"2026-09-18T12:00:01Z","scope":"","event":{"kind":"scope_began","name":"plan-trip","loop":false}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-18T12:00:05Z","scope":"research.1","session":"researcher.1","event":{"kind":"session_created","name":"researcher","adapter":"fixture","model":"gpt-5.6-luna","effort":"low","workdir":"/work"}}`),
		json.RawMessage(`{"seq":4,"time":"2026-09-18T12:00:06Z","scope":"research.1","session":"researcher.1","turn":"researcher.1/turn.1","event":{"kind":"turn_started","prompt":"Compare cabins using the traveler's latest preferences.","output_type":"gimbal.Text"}}`),
		json.RawMessage(`{"seq":5,"time":"2026-09-18T12:00:10Z","scope":"research.1/lodging.1","session":"researcher.1","event":{"kind":"interview_question_asked","name":"preferences","question_id":"question-1","question":"Would you trade reliable Wi-Fi for a secluded cabin?"}}`),
		json.RawMessage(`{"seq":6,"time":"2026-09-18T12:00:20Z","scope":"research.1","session":"researcher.1","turn":"researcher.1/turn.1","event":{"kind":"turn_ended","result":"\"done\"","error":"","usage":[{"model":"gpt-5.6-luna","cost":0,"tokens":{"input":1000000,"output":100000,"reasoning":0,"cache":{"read":0,"write":0}}}],"duration":14000000000,"interrupted":false}}`),
	} {
		if err := running.Lifecycle(record); err != nil {
			t.Fatalf("fold running record %s: %v", record, err)
		}
	}

	ended := openTestRun(t, registry, project, "run-ended", "release")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-15T11:00:00Z","scope":"","event":{"kind":"run_started","name":"release"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-15T11:07:30Z","scope":"","event":{"kind":"run_ended","name":"release","error":""}}`),
	} {
		if err := ended.Lifecycle(record); err != nil {
			t.Fatalf("fold ended record %s: %v", record, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/projects/test", nil)
	ctx := observation.WithRegistry(request.Context(), registry)
	request = request.WithContext(host.WithProjectDir(ctx, project))
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
		"Latest activity",
		"2026-09-18T12:00:20.000Z",
		"$0.32",
		"Compare cabins using the traveler's latest preferences.",
		"Sessions",
		"Turns",
		"run-ended",
		"release",
		"Ended",
		"7m 30s",
		"No instruction recorded yet.",
		"Sep ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the runs page does not contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Agent workflows in Go") {
		t.Fatal("the old guide landing page is still rendered at /")
	}
}

func TestRunsPageRendersAnEmptyProject(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	request := httptest.NewRequest(http.MethodGet, "/projects/test", nil)
	ctx := observation.WithRegistry(request.Context(), observation.NewRegistry(project))
	request = request.WithContext(host.WithProjectDir(ctx, project))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /: status %d, body %s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{"Empty project", "No runs yet"} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("the empty runs page does not contain %q:\n%s", want, recorder.Body.String())
		}
	}
}

func TestRunsPageRefreshesAForeignRunFromDisk(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	servedRegistry := observation.NewRegistry(project)
	foreign := openTestRun(t, observation.NewRegistry(project), project, "foreign-review", "review")
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-18T12:00:00Z","scope":"","event":{"kind":"run_started","name":"review"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-18T12:00:01Z","scope":"","event":{"kind":"scope_began","name":"review","loop":false}}`),
	} {
		if err := foreign.Lifecycle(record); err != nil {
			t.Fatalf("fold foreign running record %s: %v", record, err)
		}
	}

	render := func() string {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/projects/test", nil)
		ctx := observation.WithRegistry(request.Context(), servedRegistry)
		request = request.WithContext(host.WithProjectDir(ctx, project))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET /: status %d, body %s", recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}

	first := render()
	if !strings.Contains(first, `aria-label="Open review foreign-review"`) || !strings.Contains(first, "Running") {
		t.Fatalf("first listing did not show the foreign run as running:\n%s", first)
	}

	if err := foreign.Lifecycle(json.RawMessage(`{"seq":3,"time":"2026-09-18T12:00:20Z","scope":"","event":{"kind":"run_ended","name":"review","error":""}}`)); err != nil {
		t.Fatalf("finish foreign run: %v", err)
	}
	if err := foreign.Close(); err != nil {
		t.Fatalf("close foreign run: %v", err)
	}

	second := render()
	if !strings.Contains(second, `aria-label="Open review foreign-review"`) || !strings.Contains(second, "Ended") {
		t.Fatalf("refreshed listing did not show the foreign run as ended:\n%s", second)
	}
	if strings.Contains(second, ">Running<") {
		t.Fatalf("refreshed listing kept the foreign run running:\n%s", second)
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
