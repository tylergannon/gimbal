package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/gimble/workflow"
)

func TestRunPageKeepsInterviewHistoryAcrossQuestionsAndReload(t *testing.T) {
	const workflowName = "interview-ssr-fixture"
	gimble.RegisterGraph(workflow.Graph{
		Name: workflowName,
		Body: []workflow.Operation{
			workflow.Scope{Name: "preferences", Body: []workflow.Operation{
				workflow.Session{Name: "interviewer"},
				workflow.Interview{Name: "preferences", Session: "interviewer"},
			}},
		},
	})
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
	runDir := filepath.Join(project, "runs", "run-interview")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, "run-interview", workflowName, runDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":1,"time":"2026-09-17T17:00:00Z","scope":"","event":{"kind":"run_started","name":"interview-ssr-fixture"}}`),
		json.RawMessage(`{"seq":2,"time":"2026-09-17T17:00:01Z","scope":"preferences.1","event":{"kind":"scope_began","name":"preferences.1"}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-17T17:00:02Z","scope":"preferences.1","session":"preferences.1/interviewer.1","event":{"kind":"session_created","name":"interviewer","adapter":"fixture","model":"test-model","workdir":"/w"}}`),
		json.RawMessage(`{"seq":4,"time":"2026-09-17T17:00:03Z","scope":"preferences.1","session":"preferences.1/interviewer.1","turn":"preferences.1/interviewer.1/turn.1","event":{"kind":"turn_started","prompt":"first","output_type":"gimble.interviewDecision"}}`),
		json.RawMessage(`{"seq":5,"time":"2026-09-17T17:00:04Z","scope":"preferences.1","session":"preferences.1/interviewer.1","turn":"preferences.1/interviewer.1/turn.1","event":{"kind":"turn_ended","result":"{\"question\":\"Which color?\"}"}}`),
		json.RawMessage(`{"seq":6,"time":"2026-09-17T17:00:05Z","scope":"preferences.1","session":"preferences.1/interviewer.1","event":{"kind":"interview_question_asked","name":"preferences","question_id":"question-1","question":"Which color?"}}`),
		json.RawMessage(`{"seq":7,"time":"2026-09-17T17:00:06Z","scope":"preferences.1","session":"preferences.1/interviewer.1","event":{"kind":"interview_question_answered","question_id":"question-1","answer":"Blue"}}`),
		json.RawMessage(`{"seq":8,"time":"2026-09-17T17:00:07Z","scope":"preferences.1","session":"preferences.1/interviewer.1","turn":"preferences.1/interviewer.1/turn.2","event":{"kind":"turn_started","prompt":"second","output_type":"gimble.interviewDecision"}}`),
	} {
		if err := store.Lifecycle(record); err != nil {
			t.Fatalf("fold %s: %v", record, err)
		}
	}
	for turn, text := range map[string]string{
		"preferences.1/interviewer.1/turn.1": "first interview turn",
		"preferences.1/interviewer.1/turn.2": "second interview turn",
	} {
		at := observation.Placement{Scope: "preferences.1", Session: "preferences.1/interviewer.1", Turn: turn}
		for _, event := range []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`{"id":%q,"type":"session.step.started","created":10,"data":{"sessionID":"native-interviewer","assistantMessageID":%q,"agent":"fixture","model":{"providerID":"fixture","id":"test-model"}}}`, turn+"-step", turn+"-message")),
			json.RawMessage(fmt.Sprintf(`{"id":%q,"type":"session.text.started","created":11,"data":{"sessionID":"native-interviewer","assistantMessageID":%q,"ordinal":0}}`, turn+"-start", turn+"-message")),
			json.RawMessage(fmt.Sprintf(`{"id":%q,"type":"session.text.ended","created":12,"data":{"sessionID":"native-interviewer","assistantMessageID":%q,"ordinal":0,"text":%q}}`, turn+"-text", turn+"-message", text)),
		} {
			if err := store.Event(at, event, nil); err != nil {
				t.Fatal(err)
			}
		}
	}

	render := func() string {
		request := httptest.NewRequest(http.MethodGet, "/runs/run-interview", nil)
		request = request.WithContext(observation.WithRegistry(request.Context(), registry))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET /runs/run-interview: status %d, body %s", recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}
	assertGraph := func(body string) {
		t.Helper()
		if !strings.Contains(body, workflowName+" workflow map") {
			t.Fatal("run with a matching compiled graph did not render the workflow map")
		}
		if strings.Contains(body, "Workflow graph required") {
			t.Fatal("run with a matching compiled graph rendered the corrective state")
		}
		for _, want := range []string{"preferences.1", "first interview turn", "second interview turn", "Which color?", "Blue"} {
			if !strings.Contains(body, want) {
				t.Fatalf("interview history does not contain %q", want)
			}
		}
	}

	thinking := render()
	assertGraph(thinking)
	if !strings.Contains(thinking, "Stop turn") {
		t.Fatal("the workspace did not expose a stop control for the running interview turn")
	}

	for _, record := range []json.RawMessage{
		json.RawMessage(`{"seq":9,"time":"2026-09-17T17:00:08Z","scope":"preferences.1","session":"preferences.1/interviewer.1","turn":"preferences.1/interviewer.1/turn.2","event":{"kind":"turn_ended","result":"{\"question\":\"What shade?\"}"}}`),
		json.RawMessage(`{"seq":10,"time":"2026-09-17T17:00:09Z","scope":"preferences.1","session":"preferences.1/interviewer.1","event":{"kind":"interview_question_asked","name":"preferences","question_id":"question-2","question":"What shade?"}}`),
	} {
		if err := store.Lifecycle(record); err != nil {
			t.Fatalf("fold %s: %v", record, err)
		}
	}

	for reload := range 2 {
		pending := render()
		assertGraph(pending)
		for _, want := range []string{"What shade?", "answerInterview"} {
			if !strings.Contains(pending, want) {
				t.Fatalf("reload %d does not show %q", reload, want)
			}
		}
	}
}
