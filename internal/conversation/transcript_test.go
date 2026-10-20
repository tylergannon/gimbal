package conversation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/sessionstate"
)

const (
	fixtureSession = "agentic-dialogues.1"
	fixtureTurn    = "turn-1"
)

func wrappedPrompt(message string) string {
	return "You are the agent in a Gimbal conversation.\n\nHuman input:\n" + message + "\n\nRespond with {\"decision\":{\"type\":\"chat\",\"message\":\"your reply\"}}, or {\"decision\":{\"type\":\"end\"}} when finished."
}

// journalFixture is the literal record a conversation run keeps: lifecycle
// records for run.jsonl and native events for sessions/agentic-dialogues.1.jsonl.
type journalFixture struct{ lifecycle, session []string }

func event(id, kind, data string) string {
	return `{"id":"` + id + `","type":"` + kind + `","created":1000,"data":` + data + `}`
}

func (f *journalFixture) lifecycleRecord(turn, kind, fields string) {
	f.lifecycle = append(f.lifecycle, `{"seq":1,"time":"2026-10-20T00:00:00Z","scope":"","session":"`+fixtureSession+`","turn":"`+turn+`","event":{"kind":"`+kind+`"`+fields+`}}`)
}

func (f *journalFixture) native(envelope string) {
	f.session = append(f.session, `{"scope":"","session":"`+fixtureSession+`","turn":"`+fixtureTurn+`","event":`+envelope+`}`)
}

// firstTurn is a Codex-style turn: the wrapped prompt, a streamed structured
// decision that has not finished, and a command that is still running.
func firstTurn(message string) *journalFixture {
	f := &journalFixture{}
	f.lifecycle = append(f.lifecycle,
		`{"seq":1,"time":"2026-10-20T00:00:00Z","scope":"","event":{"kind":"run_started","name":"conversation"}}`,
		`{"seq":2,"time":"2026-10-20T00:00:00Z","scope":"","session":"`+fixtureSession+`","event":{"kind":"session_created","name":"`+fixtureSession+`","adapter":"codex","model":"gpt-6-luna","parent":"","workdir":"/w"}}`)
	prompt, _ := json.Marshal(wrappedPrompt(message))
	f.lifecycleRecord(fixtureTurn, "turn_started", `,"prompt":`+string(prompt)+`,"output_type":"gimbal.interactiveOutput"`)
	f.native(event("e1", "session.inbox.enqueued", `{"sessionID":"ses_a","inboxID":"turn-1/prompt","item":{"type":"user","payload":{"text":`+string(prompt)+`},"delivery":"queue"}}`))
	f.native(event("e2", "session.inbox.delivered", `{"sessionID":"ses_a","inboxID":"turn-1/prompt"}`))
	f.native(event("e3", "session.step.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1","agent":"codex","model":{"id":"gpt-6-luna","providerID":"openai"}}`))
	f.native(event("e4", "session.tool.input.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"call_1","name":"commandExecution"}`))
	f.native(event("e5", "session.tool.called", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"call_1","input":{"command":"sleep 40"},"executed":true}`))
	f.native(event("e6", "session.text.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1"}`))
	f.native(event("e7", "session.text.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","delta":"{\"decision\":{\"type\":\"chat\",\"message\":\"Wait"}`))
	f.native(event("e8", "session.text.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","delta":"ing for the\\nmarker"}`))
	return f
}

func (f *journalFixture) steer(request, message string) {
	f.native(event("s1", "session.inbox.enqueued", `{"sessionID":"ses_a","inboxID":"turn-1/steer/`+request+`","item":{"type":"user","payload":{"text":"`+message+`"},"delivery":"steer"}}`))
	f.native(event("s2", "session.inbox.delivered", `{"sessionID":"ses_a","inboxID":"turn-1/steer/`+request+`"}`))
}

// write puts the journal where a registry replays a run from.
func (f *journalFixture) write(t *testing.T, project, run string) {
	t.Helper()
	dir := filepath.Join(project, "runs", run)
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, lines := range map[string][]string{"run.jsonl": f.lifecycle, filepath.Join("sessions", fixtureSession+".jsonl"): f.session} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// open feeds the journal to a live run the way gimbal.Run does.
func (f *journalFixture) open(t *testing.T, registry *observation.Registry, project, run string) {
	t.Helper()
	dir := filepath.Join(project, "runs", run)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := observation.Open(registry, run, "conversation", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, line := range f.lifecycle {
		if err := store.Lifecycle(json.RawMessage(line)); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range f.session {
		var record struct {
			Event json.RawMessage `json:"event"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if err := store.Event(observation.Placement{Session: fixtureSession, Turn: fixtureTurn}, record.Event, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func chatRows(t *testing.T, item Conversation) []map[string]any {
	t.Helper()
	var state sessionstate.ProjectionState
	if err := json.Unmarshal([]byte(item.State), &state); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	for _, row := range state.Message[item.ID] {
		raw, _ := json.Marshal(row)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		rows = append(rows, decoded)
	}
	return rows
}

func parts(row map[string]any) []map[string]any {
	var out []map[string]any
	for _, part := range row["content"].([]any) {
		out = append(out, part.(map[string]any))
	}
	return out
}

func conversationWithRun(t *testing.T, observe func(string) (gimbal.InteractiveTranscript, string, []string, error)) (*Manager, *observation.Registry, string, Conversation) {
	t.Helper()
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimbal")
	registry := observation.NewRegistry(project)
	manager, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }, "/bin/gimbal", "/instance", repository, registry)
	if err != nil {
		t.Fatal(err)
	}
	manager.WorkflowStart = func(context.Context, Conversation, backend.Route) error { return nil }
	manager.WorkflowInput = func(string, string, string) error { return nil }
	manager.WorkflowObserve = observe
	item, err := manager.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	return manager, registry, project, item
}

// The native conversation tab reads Conversation.State. Before the fix nothing
// wrote it after Interactive moved conversations onto the workflow path, so an
// active turn showed no prompt, no tool row and "turn 0".
func TestActiveTurnProjectsPromptToolAndAnswerFromTheRunJournal(t *testing.T) {
	working := func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{Messages: []gimbal.InteractiveMessage{}}, "working", nil, nil
	}
	manager, registry, project, item := conversationWithRun(t, working)

	// Admitted, but the harness has said nothing yet: the prompt already shows.
	if _, err := manager.Submit(item.ID, Request{ID: "req-1", Created: 500, Kind: "prompt", Message: "Wait 40 seconds then print the marker"}); err != nil {
		t.Fatal(err)
	}
	pending, _ := manager.Get(item.ID)
	rows := chatRows(t, pending)
	if len(rows) != 1 || rows[0]["text"] != "Wait 40 seconds then print the marker" || rows[0]["requestID"] != "req-1" || pending.Requests[0].Delivery != "turn" {
		t.Fatalf("admitted prompt: rows=%v requests=%+v", rows, pending.Requests)
	}

	journal := firstTurn("Wait 40 seconds then print the marker")
	journal.open(t, registry, project, pending.WorkflowRun)
	live, _ := manager.Get(item.ID)
	rows = chatRows(t, live)
	if len(rows) != 2 {
		t.Fatalf("rows: %v", rows)
	}
	if rows[0]["type"] != "user" || rows[0]["text"] != "Wait 40 seconds then print the marker" || rows[0]["requestID"] != "req-1" || rows[0]["id"] != "req-1/prompt" {
		t.Fatalf("current prompt: %v", rows[0])
	}
	if strings.Contains(string(live.State), "Human input") {
		t.Fatal("the conversation purpose leaked into the readout")
	}
	content := parts(rows[1])
	if rows[1]["id"] != "req-1/msg_1" || len(content) != 2 || content[0]["type"] != "tool" || content[0]["state"].(map[string]any)["status"] != "running" {
		t.Fatalf("tool row: %v", rows[1])
	}
	if content[1]["text"] != "Waiting for the\nmarker" {
		t.Fatalf("incremental answer: %q", content[1]["text"])
	}
	if live.ActiveRequest != "req-1" || live.Status != "working" || live.Requests[0].Delivery != "turn" || live.Revision <= pending.Revision {
		t.Fatalf("turn state: active=%q status=%q requests=%+v revision=%d>%d", live.ActiveRequest, live.Status, live.Requests, live.Revision, pending.Revision)
	}
}

func TestSteeredPromptJoinsTheRunningTurn(t *testing.T) {
	working := func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{Messages: []gimbal.InteractiveMessage{}}, "working", nil, nil
	}
	manager, registry, project, item := conversationWithRun(t, working)
	for _, request := range []Request{{ID: "req-1", Created: 500, Kind: "prompt", Message: "Wait"}, {ID: "req-2", Created: 600, Kind: "prompt", Message: "and say hi"}} {
		if _, err := manager.Submit(item.ID, request); err != nil {
			t.Fatal(err)
		}
	}
	journal := firstTurn("Wait")
	journal.steer("req-2", "and say hi")
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)
	var steered map[string]any
	for _, row := range chatRows(t, live) {
		if row["requestID"] == "req-2" {
			steered = row
		}
	}
	if steered == nil || steered["steered"] != true || steered["text"] != "and say hi" || !strings.HasPrefix(steered["id"].(string), "req-1/") {
		t.Fatalf("steered row: %v", steered)
	}
	if live.Requests[0].Delivery != "turn" || live.Requests[1].Delivery != "steer" {
		t.Fatalf("deliveries: %+v", live.Requests)
	}
}

// Reopening reads only the journal: the saved projection is cleared first.
func TestReopenedConversationReplaysTheTranscriptFromTheJournal(t *testing.T) {
	working := func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{Messages: []gimbal.InteractiveMessage{}}, "working", nil, nil
	}
	manager, registry, project, item := conversationWithRun(t, working)
	if _, err := manager.Submit(item.ID, Request{ID: "req-1", Created: 500, Kind: "prompt", Message: "Wait"}); err != nil {
		t.Fatal(err)
	}
	run, _ := manager.Get(item.ID)
	journal := firstTurn("Wait")
	journal.open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)

	// A new process: no live store, a registry that only has the files.
	journal.write(t, project, run.WorkflowRun)
	reopened, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }, "/bin/gimbal", "/instance", filepath.Dir(project), observation.NewRegistry(project))
	if err != nil {
		t.Fatal(err)
	}
	reopened.WorkflowObserve = working
	reopened.mu.Lock()
	reopened.items[item.ID].State = ""
	reopened.mu.Unlock()
	replayed, ok := reopened.Get(item.ID)
	if !ok {
		t.Fatal("conversation lost")
	}
	if replayed.State != live.State {
		t.Fatalf("replay differs\nlive:   %s\nreplay: %s", live.State, replayed.State)
	}
	if len(chatRows(t, replayed)) != 2 || replayed.Requests[0].Delivery != "turn" {
		t.Fatalf("replayed: %v %+v", chatRows(t, replayed), replayed.Requests)
	}
}

func TestDecisionMessageShowsOnlyTheReply(t *testing.T) {
	for text, want := range map[string]string{
		`{"decision":{"type":"chat","message":"hi"}}`:         "hi",
		`{"decision":{"type":"chat","message":"say \"hi\""}}`: `say "hi"`,
		`{"decision":{"type":"chat","message":"par`:           "par",
		`{"decision":{"type":"chat","message":"tab\`:          "tab",
		`{"decision":{"type":"chat","message":"a\u00`:         "a",
		`{"decision":{"type":"chat"`:                          "",
		`{"decision":{"type":"end"}}`:                         "",
	} {
		if got := decisionMessage(text); got != want {
			t.Errorf("%s: got %q want %q", text, got, want)
		}
	}
}
