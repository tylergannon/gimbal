package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/sessionstate"
)

const fixtureSession = "agentic-dialogues.1"

func wrappedPrompt(message string) string {
	return "You are the agent in a Gimbal conversation.\n\nHuman input:\n" + message + "\n\nRespond with {\"decision\":{\"type\":\"chat\",\"message\":\"your reply\"}}, or {\"decision\":{\"type\":\"end\"}} when finished."
}

// journalFixture is the literal record a conversation run keeps: lifecycle
// records for run.jsonl and native events for sessions/agentic-dialogues.1.jsonl.
type journalFixture struct {
	lifecycle, session []string
	events             int
}

func (f *journalFixture) event(kind, data string) string {
	f.events++
	return fmt.Sprintf(`{"id":"e%d","type":"%s","created":1000,"data":%s}`, f.events, kind, data)
}

// turnID is the id the n-th native turn of the session has in the journal.
func turnID(n int) string { return fmt.Sprintf("turn-%d", n) }

func (f *journalFixture) lifecycleRecord(n int, kind, fields string) {
	f.lifecycle = append(f.lifecycle, fmt.Sprintf(`{"seq":%d,"time":"2026-10-20T00:00:%02dZ","scope":"","session":"%s","turn":"%s","event":{"kind":"%s"%s}}`, len(f.lifecycle)+1, n, fixtureSession, turnID(n), kind, fields))
}

func (f *journalFixture) native(n int, kind, data string) {
	f.session = append(f.session, fmt.Sprintf(`{"scope":"","session":"%s","turn":"%s","event":%s}`, fixtureSession, turnID(n), f.event(kind, data)))
}

// newJournal is a conversation run that has started its harness session.
func newJournal() *journalFixture {
	return &journalFixture{lifecycle: []string{
		`{"seq":1,"time":"2026-10-20T00:00:00Z","scope":"","event":{"kind":"run_started","name":"conversation"}}`,
		`{"seq":2,"time":"2026-10-20T00:00:00Z","scope":"","session":"` + fixtureSession + `","event":{"kind":"session_created","name":"` + fixtureSession + `","adapter":"codex","model":"gpt-6-luna","parent":"","workdir":"/w"}}`,
	}}
}

// consumed is the interactive's record of the inputs it has taken so far.
func (f *journalFixture) consumed(ids ...string) {
	raw, _ := json.Marshal(ids)
	f.lifecycle = append(f.lifecycle, fmt.Sprintf(`{"seq":%d,"time":"2026-10-20T00:00:00Z","scope":"","session":"%s","event":{"kind":"interactive_changed","id":"question-1","name":"conversation","address":"conversation","ordinal":1,"consumed":%s,"status":"active","waiting":false,"transcript":{"messages":[],"ended_by":""},"error":""}}`, len(f.lifecycle)+1, fixtureSession, raw))
}

// startTurn is the start of one native turn for the wrapped message: the
// interactive's prompt, delivered to the harness.
func (f *journalFixture) startTurn(n int, message string) {
	prompt, _ := json.Marshal(wrappedPrompt(message))
	f.lifecycleRecord(n, "turn_started", `,"prompt":`+string(prompt)+`,"output_type":"gimbal.interactiveOutput"`)
	f.native(n, "session.inbox.enqueued", `{"sessionID":"ses_a","inboxID":"`+turnID(n)+`/prompt","item":{"type":"user","payload":{"text":`+string(prompt)+`},"delivery":"queue"}}`)
	f.native(n, "session.inbox.delivered", `{"sessionID":"ses_a","inboxID":"`+turnID(n)+`/prompt"}`)
	f.native(n, "session.step.started", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","agent":"codex","model":{"id":"gpt-6-luna","providerID":"openai"}}`, n))
}

func (f *journalFixture) endTurn(n int) {
	f.lifecycleRecord(n, "turn_ended", `,"result":"{}","error":"","usage":[],"duration":1000000,"interrupted":false`)
}

// firstTurn is a Codex-style turn: the wrapped prompt, a streamed structured
// decision that has not finished, and a command that is still running.
func firstTurn(message string) *journalFixture {
	f := newJournal()
	f.consumed("req-1")
	f.startTurn(1, message)
	f.native(1, "session.tool.input.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"call_1","name":"commandExecution"}`)
	f.native(1, "session.tool.called", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"call_1","input":{"command":"sleep 40"},"executed":true}`)
	f.native(1, "session.text.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1"}`)
	f.native(1, "session.text.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","delta":"{\"decision\":{\"type\":\"chat\",\"message\":\"Wait"}`)
	f.native(1, "session.text.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","delta":"ing for the\\nmarker"}`)
	return f
}

// chatReply is a turn that answered with the Codex-style streamed decision.
func (f *journalFixture) chatReply(n int, reply string) {
	f.native(n, "session.text.started", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d"}`, n))
	delta, _ := json.Marshal(`{"decision":{"type":"chat","message":"` + reply + `"}}`)
	f.native(n, "session.text.delta", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","delta":%s}`, n, delta))
}

// structuredReply is a Claude turn: the reply exists only as the input of the
// StructuredOutput completion tool, as the Claude adapter emits it.
func (f *journalFixture) structuredReply(n int, envelope string) {
	input, _ := json.Marshal(envelope)
	f.native(n, "session.tool.input.started", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","id":"so_%d","name":"StructuredOutput"}`, n, n))
	f.native(n, "session.tool.input.delta", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","id":"so_%d","delta":%s}`, n, n, input))
	f.native(n, "session.tool.input.ended", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","id":"so_%d","text":%s}`, n, n, input))
	f.native(n, "session.tool.called", fmt.Sprintf(`{"sessionID":"ses_a","assistantMessageID":"msg_%d","id":"so_%d","input":%s,"executed":true}`, n, n, envelope))
}

func (f *journalFixture) steer(n int, request, message string) {
	f.native(n, "session.inbox.enqueued", `{"sessionID":"ses_a","inboxID":"`+turnID(n)+`/steer/`+request+`","item":{"type":"user","payload":{"text":"`+message+`"},"delivery":"steer"}}`)
	f.native(n, "session.inbox.delivered", `{"sessionID":"ses_a","inboxID":"`+turnID(n)+`/steer/`+request+`"}`)
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
			Turn      string          `json:"turn"`
			Event     json.RawMessage `json:"event"`
			NativeRef json.RawMessage `json:"native_ref"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if err := store.Event(observation.Placement{Session: fixtureSession, Turn: record.Turn}, record.Event, record.NativeRef); err != nil {
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
	journal.steer(1, "req-2", "and say hi")
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
	// Delivered into the running turn, the steer is no longer a queued prompt.
	if live.Requests[1].Status == "accepted" || live.ActiveRequest != "req-1" {
		t.Fatalf("steer status=%q active=%q", live.Requests[1].Status, live.ActiveRequest)
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

// observeWith is the workflow's view of a conversation: the interactive
// transcript the run has published, and the run status.
func observeWith(status string, messages ...gimbal.InteractiveMessage) func(string) (gimbal.InteractiveTranscript, string, []string, error) {
	return func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{Messages: messages}, status, nil, nil
	}
}

func submit(t *testing.T, manager *Manager, id string, requests ...Request) {
	t.Helper()
	for _, request := range requests {
		request.Kind = "prompt"
		if _, err := manager.Submit(id, request); err != nil {
			t.Fatal(err)
		}
	}
}

// reopen is a new process that has only the journal files.
func reopen(t *testing.T, project string, item Conversation, observe func(string) (gimbal.InteractiveTranscript, string, []string, error)) Conversation {
	t.Helper()
	reopened, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return &chatAdapter{sessions: map[string][]string{}}, nil }, "/bin/gimbal", "/instance", filepath.Dir(project), observation.NewRegistry(project))
	if err != nil {
		t.Fatal(err)
	}
	reopened.WorkflowObserve = observe
	reopened.mu.Lock()
	reopened.items[item.ID].State = ""
	reopened.mu.Unlock()
	replayed, ok := reopened.Get(item.ID)
	if !ok {
		t.Fatal("conversation lost")
	}
	return replayed
}

func rowIDs(rows []map[string]any) []string {
	var ids []string
	for _, row := range rows {
		ids = append(ids, row["id"].(string))
	}
	return ids
}

func deliveries(item Conversation) string {
	var out []string
	for _, request := range item.Requests {
		out = append(out, request.ID+"="+request.Delivery+"/"+request.Status)
	}
	return strings.Join(out, " ")
}

// goldenFixture holds the conversation the page receives for a journal. The web
// tests render these files, so the page is checked against what Go produces;
// this test fails when the projection changes. GIMBAL_UPDATE_FIXTURES=1 rewrites them.
func goldenFixture(t *testing.T, name string, item Conversation) {
	t.Helper()
	id, run := item.ID, item.WorkflowRun
	item.Worktree = "/project"
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.NewReplacer(id, "chat", run, "run").Replace(string(raw))
	var out Conversation
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	out.Created, out.Updated, out.Revision, out.Branch = 1, 1, 1, "gimbal/conversation"
	want, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "..", "web", "src", "lib", "conversation", "fixtures", name+".json")
	if os.Getenv("GIMBAL_UPDATE_FIXTURES") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(want, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(have)) != string(want) {
		t.Fatalf("%s is out of date with the projection (GIMBAL_UPDATE_FIXTURES=1 go test ./internal/conversation rewrites it): %v", path, err)
	}
}

// req-1 starts turn 1. req-2 "go on" is steered into it. req-3 "go on" is an
// ordinary prompt that starts turn 2. Text alone cannot tell req-2 from req-3.
func TestSteerWithTheSameTextDoesNotStealALaterPrompt(t *testing.T) {
	observe := observeWith("working",
		gimbal.InteractiveMessage{Role: "user", Message: "Start"},
		gimbal.InteractiveMessage{Role: "user", Message: "go on"},
		gimbal.InteractiveMessage{Role: "assistant", Message: "On it"})
	manager, registry, project, item := conversationWithRun(t, observe)
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "Start"}, Request{ID: "req-2", Created: 600, Message: "go on"}, Request{ID: "req-3", Created: 700, Message: "go on"})

	journal := newJournal()
	journal.consumed("req-1")
	journal.startTurn(1, "Start")
	journal.steer(1, "req-2", "go on")
	journal.chatReply(1, "On it")
	journal.endTurn(1)
	journal.consumed("req-1", "req-2", "req-3")
	journal.startTurn(2, "go on")
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)

	live, _ := manager.Get(item.ID)
	rows := chatRows(t, live)
	want := `[req-1/prompt req-1/msg_1 req-1/turn-1/steer/req-2 req-3/prompt req-3/msg_2]`
	if got := fmt.Sprint(rowIDs(rows)); got != want {
		t.Fatalf("rows\n got %s\nwant %s", got, want)
	}
	if got, want := deliveries(live), "req-1=turn/completed req-2=steer/completed req-3=turn/running"; got != want {
		t.Fatalf("requests\n got %s\nwant %s", got, want)
	}
	if live.ActiveRequest != "req-3" {
		t.Fatalf("active request %q, want req-3 (the running turn)", live.ActiveRequest)
	}
	for _, request := range live.Requests {
		if request.Status == "accepted" {
			t.Fatalf("%s still shows as a queued prompt: %s", request.ID, deliveries(live))
		}
	}

	journal.write(t, project, run.WorkflowRun)
	goldenFixture(t, "steered-turns", live)
	replayed := reopen(t, project, live, observe)
	if replayed.State != live.State || deliveries(replayed) != deliveries(live) || replayed.ActiveRequest != live.ActiveRequest {
		t.Fatalf("replay differs\nlive:   %s %s\nreplay: %s %s", live.State, deliveries(live), replayed.State, deliveries(replayed))
	}
}

// Two ordinary turns whose prompts are word for word the same each answer
// their own request.
func TestEndedOrdinaryTurnsWithTheSameTextPairInOrder(t *testing.T) {
	observe := observeWith("waiting",
		gimbal.InteractiveMessage{Role: "user", Message: "go on"},
		gimbal.InteractiveMessage{Role: "assistant", Message: "one"},
		gimbal.InteractiveMessage{Role: "user", Message: "go on"},
		gimbal.InteractiveMessage{Role: "assistant", Message: "two"})
	manager, registry, project, item := conversationWithRun(t, observe)
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "go on"}, Request{ID: "req-2", Created: 600, Message: "go on"})

	journal := newJournal()
	journal.consumed("req-1")
	journal.startTurn(1, "go on")
	journal.chatReply(1, "one")
	journal.endTurn(1)
	journal.consumed("req-1", "req-2")
	journal.startTurn(2, "go on")
	journal.chatReply(2, "two")
	journal.endTurn(2)
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)

	live, _ := manager.Get(item.ID)
	want := `[req-1/prompt req-1/msg_1 req-2/prompt req-2/msg_2]`
	if got := fmt.Sprint(rowIDs(chatRows(t, live))); got != want {
		t.Fatalf("rows\n got %s\nwant %s", got, want)
	}
	if got, want := deliveries(live), "req-1=turn/completed req-2=turn/completed"; got != want || live.ActiveRequest != "" {
		t.Fatalf("requests %s active=%q", got, live.ActiveRequest)
	}
	goldenFixture(t, "codex-turns", live)
}

// A prompt stopped while it was queued is never consumed. Sending the same
// words again starts the turn, which belongs to the resent request.
func TestResentPromptOwnsTheTurnNotTheStoppedOne(t *testing.T) {
	observe := observeWith("working")
	manager, registry, project, item := conversationWithRun(t, observe)
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "Start"}, Request{ID: "req-2", Created: 600, Message: "Start"})
	manager.mu.Lock()
	manager.items[item.ID].Requests[0].Status = "unprocessed"
	manager.mu.Unlock()

	journal := newJournal()
	journal.consumed("req-2")
	journal.startTurn(1, "Start")
	journal.chatReply(1, "hi")
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)

	live, _ := manager.Get(item.ID)
	want := `[req-2/prompt req-2/msg_1]`
	if got := fmt.Sprint(rowIDs(chatRows(t, live))); got != want {
		t.Fatalf("rows\n got %s\nwant %s", got, want)
	}
	if got, want := deliveries(live), "req-1=durable/unprocessed req-2=turn/running"; got != want || live.ActiveRequest != "req-2" {
		t.Fatalf("requests %s active=%q", got, live.ActiveRequest)
	}
}

// Without a record of what the interactive consumed, a turn is not guessed
// from its words; its rows stay but belong to no request.
func TestTurnWithoutConsumedInputIsNotPaired(t *testing.T) {
	manager, registry, project, item := conversationWithRun(t, observeWith("working"))
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "Start"})
	journal := newJournal()
	journal.startTurn(1, "Start")
	journal.chatReply(1, "hi")
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)
	for _, row := range chatRows(t, live) {
		if strings.HasPrefix(row["id"].(string), "req-1/") || row["requestID"] == "req-1" {
			t.Fatalf("turn paired without evidence: %v", row)
		}
	}
	if strings.Contains(string(live.State), "Human input") {
		t.Fatal("the conversation purpose leaked into the readout")
	}
}

// Claude answers through its StructuredOutput completion tool, which the page
// hides. The projection supplies the reply as text.
func TestClaudeStructuredOutputReplyIsVisible(t *testing.T) {
	for name, envelope := range map[string]string{
		"completed": `{"state":"completed","value":{"decision":{"type":"chat","message":"Hello there\nfriend"}}}`,
		"waiting":   `{"state":"waiting","message":"Hello there\nfriend"}`,
	} {
		t.Run(name, func(t *testing.T) {
			manager, registry, project, item := conversationWithRun(t, observeWith("waiting"))
			submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "hi"})
			journal := newJournal()
			journal.consumed("req-1")
			journal.startTurn(1, "hi")
			journal.structuredReply(1, envelope)
			journal.endTurn(1)
			run, _ := manager.Get(item.ID)
			journal.open(t, registry, project, run.WorkflowRun)
			live, _ := manager.Get(item.ID)
			rows := chatRows(t, live)
			if len(rows) != 2 {
				t.Fatalf("rows: %v", rows)
			}
			var texts []string
			for _, part := range parts(rows[1]) {
				if part["type"] == "text" {
					texts = append(texts, part["text"].(string))
				}
			}
			if fmt.Sprint(texts) != "[Hello there\nfriend]" {
				t.Fatalf("visible reply %q in %v", texts, rows[1])
			}
			goldenFixture(t, "claude-"+name, live)
			if strings.Contains(strings.Join(texts, ""), "state") || strings.Contains(strings.Join(texts, ""), "decision") {
				t.Fatalf("completion schema leaked: %q", texts)
			}
		})
	}
}

// Claude's projector puts narration text and the StructuredOutput call on the
// same assistant message. The narration stays, and the real reply is added.
func TestClaudeNarrationDoesNotHideTheStructuredReply(t *testing.T) {
	const envelope = `{"state":"completed","value":{"decision":{"type":"chat","message":"The answer is 42."}}}`
	for name, narration := range map[string]string{"narrated": "I checked the files.", "repeated": "The answer is 42."} {
		t.Run(name, func(t *testing.T) {
			manager, registry, project, item := conversationWithRun(t, observeWith("waiting"))
			submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "what is it"})
			journal := newJournal()
			journal.consumed("req-1")
			journal.startTurn(1, "what is it")
			journal.native(1, "session.text.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1"}`)
			journal.native(1, "session.text.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","delta":"`+narration+`"}`)
			journal.native(1, "session.text.ended", `{"sessionID":"ses_a","assistantMessageID":"msg_1","text":"`+narration+`"}`)
			journal.structuredReply(1, envelope)
			journal.endTurn(1)
			run, _ := manager.Get(item.ID)
			journal.open(t, registry, project, run.WorkflowRun)
			live, _ := manager.Get(item.ID)
			rows := chatRows(t, live)
			var texts, tools []string
			for _, part := range parts(rows[1]) {
				switch part["type"] {
				case "text":
					texts = append(texts, part["text"].(string))
				case "tool":
					tools = append(tools, part["name"].(string))
				}
			}
			want := "[" + narration + " The answer is 42.]"
			if narration == "The answer is 42." {
				want = "[The answer is 42.]"
			}
			if fmt.Sprint(texts) != want || fmt.Sprint(tools) != "[StructuredOutput]" {
				t.Fatalf("texts %q tools %v, want %s", texts, tools, want)
			}
			if name == "narrated" {
				goldenFixture(t, "claude-narrated", live)
			}
		})
	}
}

// The reply streams in as the tool input is written.
func TestClaudeStructuredOutputReplyStreams(t *testing.T) {
	manager, registry, project, item := conversationWithRun(t, observeWith("working"))
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "hi"})
	journal := newJournal()
	journal.consumed("req-1")
	journal.startTurn(1, "hi")
	journal.native(1, "session.tool.input.started", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"so_1","name":"StructuredOutput"}`)
	journal.native(1, "session.tool.input.delta", `{"sessionID":"ses_a","assistantMessageID":"msg_1","id":"so_1","delta":"{\"state\":\"completed\",\"value\":{\"decision\":{\"type\":\"chat\",\"message\":\"Hel"}`)
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)
	rows := chatRows(t, live)
	content := parts(rows[1])
	if last := content[len(content)-1]; last["type"] != "text" || last["text"] != "Hel" {
		t.Fatalf("streaming reply: %v", rows[1])
	}
}

// A run that cannot be read for a moment leaves the transcript on screen.
func TestUnreadableRunKeepsTheTranscriptShown(t *testing.T) {
	manager, registry, project, item := conversationWithRun(t, observeWith("working"))
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "Wait"})
	run, _ := manager.Get(item.ID)
	firstTurn("Wait").open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)
	if len(chatRows(t, live)) != 2 {
		t.Fatalf("rows: %v", chatRows(t, live))
	}

	manager.registry = observation.NewRegistry(project + "-elsewhere")
	for range 2 {
		after, _ := manager.Get(item.ID)
		if after.State != live.State || after.Revision != live.Revision {
			t.Fatalf("readout lost while the run was unreadable:\n%s", after.State)
		}
	}
}

func TestDecisionMessageShowsOnlyTheReply(t *testing.T) {
	for text, want := range map[string]string{
		`{"decision":{"type":"chat","message":"hi"}}`:                               "hi",
		`{"decision":{"type":"chat","message":"say \"hi\""}}`:                       `say "hi"`,
		`{"decision":{"type":"chat","message":"par`:                                 "par",
		`{"decision":{"type":"chat","message":"tab\`:                                "tab",
		`{"decision":{"type":"chat","message":"a\u00`:                               "a",
		`{"decision":{"type":"chat"`:                                                "",
		`{"decision":{"type":"end"}}`:                                               "",
		`{"state":"completed","value":{"decision":{"type":"chat","message":"hi"}}}`: "hi",
		`{"state":"waiting","message":"so far`:                                      "so far",
	} {
		if got := decisionMessage(text); got != want {
			t.Errorf("%s: got %q want %q", text, got, want)
		}
	}
}

// The runtime normalizes inbox IDs before journaling. Their original prompt
// and steer identities survive in native_ref, not in the projected row ID.
func TestNormalizedInboxIDsKeepHumanPromptsAndSteerOwnership(t *testing.T) {
	observe := observeWith("working", gimbal.InteractiveMessage{Role: "user", Message: "Start"})
	manager, registry, project, item := conversationWithRun(t, observe)
	submit(t, manager, item.ID, Request{ID: "req-1", Created: 500, Message: "Start"}, Request{ID: "req-2", Created: 600, Message: "go on"}, Request{ID: "req-3", Created: 700, Message: "go on"})
	journal := newJournal()
	journal.consumed("req-1")
	journal.startTurn(1, "Start")
	journal.steer(1, "req-2", "go on")
	journal.chatReply(1, "On it")
	journal.endTurn(1)
	journal.consumed("req-1", "req-2", "req-3")
	journal.startTurn(2, "go on")
	ids := map[string]string{}
	for i, line := range journal.session {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		event := rec["event"].(map[string]any)
		data := event["data"].(map[string]any)
		id, ok := data["inboxID"].(string)
		if !ok {
			continue
		}
		normalized := ids[id]
		if normalized == "" {
			normalized = fmt.Sprintf("msg_normalized_%d", len(ids)+1)
			ids[id] = normalized
		}
		data["inboxID"] = normalized
		rec["native_ref"] = map[string]any{"provider": "gimbal", "messageID": id, "normalizedMessageID": normalized, "turnID": rec["turn"]}
		raw, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		journal.session[i] = string(raw)
	}
	// Reopening a native session may replay the same prompt with a new
	// normalized ID. Both sidecars still identify one admitted user input.
	for _, line := range append([]string(nil), journal.session[:2]...) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		event := rec["event"].(map[string]any)
		event["id"] = fmt.Sprint(event["id"]) + "-replayed"
		event["data"].(map[string]any)["inboxID"] = "msg_normalized_replayed_prompt"
		rec["native_ref"].(map[string]any)["normalizedMessageID"] = "msg_normalized_replayed_prompt"
		raw, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		journal.session = append(journal.session, string(raw))
	}
	run, _ := manager.Get(item.ID)
	journal.open(t, registry, project, run.WorkflowRun)
	live, _ := manager.Get(item.ID)
	rows := chatRows(t, live)
	if got, want := fmt.Sprint(rowIDs(rows)), `[req-1/prompt req-1/msg_1 req-1/turn-1/steer/req-2 req-3/prompt req-3/msg_2]`; got != want {
		t.Fatalf("rows got %s want %s", got, want)
	}
	if got, want := deliveries(live), "req-1=turn/completed req-2=steer/completed req-3=turn/running"; got != want {
		t.Fatalf("requests got %s want %s", got, want)
	}
	if rows[0]["text"] != "Start" || rows[2]["text"] != "go on" || rows[3]["text"] != "go on" {
		t.Fatalf("human prompts: %v", rows)
	}
	journal.write(t, project, run.WorkflowRun)
	replayed := reopen(t, project, live, observe)
	if replayed.State != live.State || deliveries(replayed) != deliveries(live) {
		t.Fatalf("normalized input replay differs")
	}
}
