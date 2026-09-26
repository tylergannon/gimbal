package pi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/session"
)

func TestProjectedEventsUseGimbalCompatibleNativeRefs(t *testing.T) {
	var events []gimbal.AgentEvent
	run := newTurn("pi-session", "deepseek-4.1-flash", func(event gimbal.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	mustRecord(t, run, messageUpdate(model.EventTextDelta, 0, "hello", nil))
	mustRecord(t, run, session.Event{Type: session.EventType(model.EvToolExecutionStart), Agent: model.AgentEvent{
		Type: model.EvToolExecutionStart, ToolCallID: "call-1", ToolName: "bash",
		Args: map[string]any{"command": "cat seed.txt"},
	}})
	mustRecord(t, run, session.Event{Type: session.EventType(model.EvToolExecutionEnd), Agent: model.AgentEvent{
		Type: model.EvToolExecutionEnd, ToolCallID: "call-1", ToolName: "bash",
		Result: "ORBIT-42",
	}})

	if len(events) != 4 {
		t.Fatalf("projected event count = %d, want 4: %#v", len(events), events)
	}
	wantTypes := []string{"session.text.delta", "session.tool.input.ended", "session.tool.called", "session.tool.success"}
	for i, event := range events {
		if event.Type != wantTypes[i] {
			t.Errorf("event %d type = %q, want %q", i, event.Type, wantTypes[i])
		}
		var ref map[string]string
		if err := json.Unmarshal(event.NativeRef, &ref); err != nil {
			t.Fatalf("event %d NativeRef: %v", i, err)
		}
		if ref["provider"] != "pi" || ref["sessionID"] != "pi-session" {
			t.Errorf("event %d NativeRef = %#v", i, ref)
		}
	}
}

func TestToolEventsAreProjected(t *testing.T) {
	var got []gimbal.AgentEvent
	run := newTurn("s", "m", func(event gimbal.AgentEvent) error {
		got = append(got, event)
		return nil
	})
	partial := &model.AssistantMessage{Content: model.ContentList{model.ToolCall{ID: "call-1", Name: "bash"}}}
	mustRecord(t, run, messageUpdate(model.EventToolCallStart, 0, "", partial))
	mustRecord(t, run, session.Event{Type: session.EventType(model.EvToolExecutionStart), Agent: model.AgentEvent{
		Type: model.EvToolExecutionStart, ToolCallID: "call-1", ToolName: "bash",
		Args: map[string]any{"command": "pwd"},
	}})
	mustRecord(t, run, session.Event{Type: session.EventType(model.EvToolExecutionEnd), Agent: model.AgentEvent{
		Type: model.EvToolExecutionEnd, ToolCallID: "call-1", ToolName: "bash", Result: "/tmp",
	}})
	var types []string
	for _, event := range got {
		types = append(types, event.Type)
	}
	want := []string{"session.tool.input.started", "session.tool.input.ended", "session.tool.called", "session.tool.success"}
	if fmt.Sprint(types) != fmt.Sprint(want) {
		t.Fatalf("projected tool events = %v, want %v", types, want)
	}
}

func TestAgentSettledRequiresFinalAssistantAndAcceptsDiagnosticEventGaps(t *testing.T) {
	run := newTurn("s", "m", func(gimbal.AgentEvent) error { return fmt.Errorf("observer failed") })
	mustRecord(t, run, messageUpdate(model.EventTextDelta, 0, "partial", nil))
	mustRecord(t, run, session.Event{Type: session.EventAgentSettled})
	select {
	case <-run.settled:
	default:
		t.Fatal("agent_settled was not observed")
	}
	if run.final != nil {
		t.Fatal("partial text was incorrectly treated as authoritative output")
	}
}

func messageUpdate(kind model.EventType, index int, delta string, partial *model.AssistantMessage) session.Event {
	return session.Event{
		Type: session.EventType(model.EvMessageUpdate),
		Agent: model.AgentEvent{
			Type:                  model.EvMessageUpdate,
			AssistantMessageEvent: &model.AssistantMessageEvent{Type: kind, ContentIndex: index, Delta: delta, Partial: partial},
		},
	}
}

func mustRecord(t *testing.T, run *turn, event session.Event) {
	t.Helper()
	if err := run.record(event); err != nil {
		t.Fatalf("record: %v", err)
	}
}

func TestEffortMustBeBlankAndModelIDIsBound(t *testing.T) {
	a := newAdapter(Config{APIKey: func() string { return "test-key" }})
	if _, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "high", t.TempDir()); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("CreateSession effort error = %v", err)
	}
	if got := modelID("diffusion/deepseek-4.1-flash"); got != "deepseek-4.1-flash" {
		t.Fatalf("modelID = %q", got)
	}
	if got := modelID("deepseek-4.1-flash"); got != "deepseek-4.1-flash" {
		t.Fatalf("modelID without provider = %q", got)
	}
}

const routerCatalogJSON = `{
  "object": "list",
  "data": [
    {"id": "deepseek-4.1-flash", "object": "model", "context_window": 1048576, "max_output_tokens": 262144,
     "capabilities": {"openai_chat": true, "reasoning": true, "tools": true, "vision": true},
     "client_compat": {"pi": {"maxTokensField": "max_tokens", "supportsReasoningEffort": true, "thinkingFormat": "deepseek",
       "thinkingLevelMap": {"high": "high", "off": "none"}}}},
    {"id": "glm-5.3-flash", "object": "model", "context_window": 524288, "max_output_tokens": 163840,
     "capabilities": {"openai_chat": true, "reasoning": true, "tools": true, "vision": true},
     "client_compat": {"pi": {"maxTokensField": "max_tokens", "supportsReasoningEffort": true, "thinkingFormat": "zai",
       "thinkingLevelMap": {"high": "high", "off": "none", "xhigh": "max"}}}},
    {"id": "glm-5.3", "object": "model", "context_window": 524288, "max_output_tokens": 131072,
     "capabilities": {"openai_chat": true, "reasoning": true, "tools": true, "vision": true}}
  ]
}`

// serveRouterModels answers the adapter's authenticated catalog lookup.
func serveRouterModels(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/v1/models" {
		return false
	}
	if got := r.Header.Get("Authorization"); got != "Bearer fake-router-key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(routerCatalogJSON))
	return true
}

func TestRouterCatalogModelUsesRouterMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRouterModels(w, r) }))
	defer server.Close()
	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	m, err := a.routerCatalogModel(context.Background(), "glm-5.3-flash", "fake-router-key")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "glm-5.3-flash" || m.Provider != "diffusion" || m.Api != model.APIOpenAICompletions {
		t.Fatalf("router model = %+v", m)
	}
	if m.ContextWindow != 524288 || m.MaxTokens != 163840 || len(m.Input) != 2 || !m.Reasoning {
		t.Fatalf("router model limits/flags = %+v", m)
	}
	if m.Compat == nil || m.ThinkingLevelMap == nil {
		t.Fatalf("router model metadata = %+v", m)
	}
	if value := m.ThinkingLevelMap[model.ModelThinkingLevel("xhigh")]; value == nil || *value != "max" {
		t.Fatalf("thinking level map = %+v", m.ThinkingLevelMap)
	}
	fallback, err := a.routerCatalogModel(context.Background(), "unknown-model", "fake-router-key")
	if err != nil {
		t.Fatal(err)
	}
	if fallback.ContextWindow != 0 || fallback.MaxTokens != 0 || len(fallback.Input) != 0 {
		t.Fatalf("unknown model should keep conservative defaults: %+v", fallback)
	}
}

func TestNativeSessionRunTurnWithFakeCompletions(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		number := requests.Add(1)
		answer := "Pi first answer"
		switch number {
		case 3:
			answer = `{"wrong":"no"}`
		case 4:
			answer = `{"answer":"yes"}`
		}
		writeFakeCompletion(w, answer, number)
	}))
	defer server.Close()

	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	workdir := t.TempDir()
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close(context.Background(), sessionID) }()

	var events []gimbal.AgentEvent
	first, err := a.RunTurn(context.Background(), sessionID, "Say hello", nil, func(event gimbal.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Output) != `"Pi first answer"` {
		t.Fatalf("first output = %s", first.Output)
	}
	if first.Usage["deepseek-4.1-flash"].Tokens.Input != 3 || first.Usage["deepseek-4.1-flash"].Tokens.Output != 2 {
		t.Fatalf("first usage = %#v", first.Usage)
	}
	if len(events) == 0 || events[0].Type != "session.text.delta" {
		t.Fatalf("projected events = %#v", events)
	}

	second, err := a.RunTurn(context.Background(), sessionID, "second turn on this conversation", nil, nil)
	if err != nil || string(second.Output) != `"Pi first answer"` {
		t.Fatalf("second turn output=%s err=%v", second.Output, err)
	}

	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`)
	candidate, err := a.RunTurn(context.Background(), sessionID, "Return an answer", schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(candidate.Output) != `{"wrong":"no"}` {
		t.Fatalf("candidate schema output = %s", candidate.Output)
	}
	valid, err := a.RunTurn(context.Background(), sessionID, "Return an answer", schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal(valid.Output, &decoded); err != nil || decoded.Answer != "yes" {
		t.Fatalf("schema output = %s err=%v", valid.Output, err)
	}
	if requests.Load() != 4 {
		t.Fatalf("request count = %d, want 4", requests.Load())
	}
}

func TestSteerLandsInRunningTurnAndMissesWithoutOne(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		lastUser := lastUserText(t, r)
		if strings.Contains(lastUser, "hold for steer") {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		answer := "answer-" + lastUser
		if strings.Contains(lastUser, "STEER THIS TURN") {
			answer = "steer was delivered"
		}
		writeFakeCompletion(w, answer, 1)
	}))
	defer server.Close()

	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	workdir := t.TempDir()
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close(context.Background(), sessionID) }()

	type outcome struct {
		result gimbal.TurnResult
		err    error
	}
	turnDone := make(chan outcome, 1)
	go func() {
		result, err := a.RunTurn(context.Background(), sessionID, "hold for steer", nil, nil)
		turnDone <- outcome{result, err}
	}()
	select {
	case <-entered:
	case <-time.After(8 * time.Second):
		t.Fatal("native session did not reach the blocked provider request")
	}

	steered := make(chan struct {
		landed bool
		err    error
	}, 1)
	go func() {
		landed, err := a.Steer(context.Background(), sessionID, "STEER THIS TURN")
		steered <- struct {
			landed bool
			err    error
		}{landed, err}
	}()
	close(release)
	result := <-turnDone
	got := <-steered
	if got.err != nil || !got.landed {
		t.Fatalf("Steer landed=%t err=%v", got.landed, got.err)
	}
	if result.err != nil || string(result.result.Output) != `"steer was delivered"` {
		t.Fatalf("steered turn output=%s err=%v", result.result.Output, result.err)
	}

	if landed, err := a.Steer(context.Background(), sessionID, "must not be sent"); err != nil || landed {
		t.Fatalf("Steer without active turn landed=%t err=%v", landed, err)
	}
}

func TestForkIsIndependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		writeFakeCompletion(w, "answer-"+lastUserText(t, r), 1)
	}))
	defer server.Close()

	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	workdir := t.TempDir()
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close(context.Background(), sessionID) }()
	if _, err := a.RunTurn(context.Background(), sessionID, "shared parent history", nil, nil); err != nil {
		t.Fatal(err)
	}
	childID, err := a.Fork(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	defer func() { _ = a.Close(context.Background(), childID) }()

	parentSession, _ := a.session(sessionID)
	childSession, _ := a.session(childID)
	if parentSession.session == childSession.session {
		t.Fatal("fork shares the parent native session")
	}

	var wg sync.WaitGroup
	results := make(chan string, 2)
	errorsCh := make(chan error, 2)
	for _, item := range []struct{ id, prompt string }{
		{sessionID, "parent divergent follow-up"},
		{childID, "child divergent follow-up"},
	} {
		wg.Add(1)
		go func(id, prompt string) {
			defer wg.Done()
			result, err := a.RunTurn(context.Background(), id, prompt, nil, nil)
			results <- string(result.Output)
			errorsCh <- err
		}(item.id, item.prompt)
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent fork turn: %v", err)
		}
	}
	got := map[string]bool{}
	for result := range results {
		got[result] = true
	}
	if !got[`"answer-parent divergent follow-up"`] || !got[`"answer-child divergent follow-up"`] {
		t.Fatalf("fork results = %v", got)
	}
}

func TestCancellationReturnsContextError(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	workdir := t.TempDir()
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close(context.Background(), sessionID) }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := a.RunTurn(ctx, sessionID, "hold for cancellation", nil, nil); done <- err }()
	select {
	case <-entered:
	case <-time.After(8 * time.Second):
		t.Fatal("native session did not reach the cancellation request")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled turn err=%v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("cancelled turn did not return")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRouterModels(w, r) }))
	defer server.Close()
	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(context.Background(), sessionID); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(context.Background(), sessionID); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := a.RunTurn(context.Background(), sessionID, "after close", nil, nil); err == nil {
		t.Fatal("RunTurn on a closed session succeeded")
	}
}

func TestSchemaCandidateFeedsGimbalValidation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		answer := `{"wrong":"no"}`
		if requests.Add(1) > 1 {
			answer = `{"answer":"yes"}`
		}
		writeFakeCompletion(w, answer, 1)
	}))
	defer server.Close()

	a := newAdapter(Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }})
	workdir := t.TempDir()
	var got schemaAnswer
	err := gimbal.Run(gimbal.Project(context.Background(), workdir), "pi-schema-reask", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"pi": {Adapter: a, Model: "diffusion/deepseek-4.1-flash"},
	}, func(ctx context.Context) error {
		var err error
		got, err = gimbal.NewSession(ctx, "pi", workdir).Generate[schemaAnswer](ctx, "Return an answer")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "yes" {
		t.Fatalf("schema answer = %#v", got)
	}
	if requests.Load() != 2 {
		t.Fatalf("request count = %d, want 2", requests.Load())
	}
}

type schemaAnswer struct {
	Answer string `json:"answer"`
}

func (schemaAnswer) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`)
}

func (schemaAnswer) ValidateJSON(raw []byte) error {
	var answer schemaAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return err
	}
	if answer.Answer == "" {
		return errors.New("answer is required")
	}
	return nil
}

func lastUserText(t *testing.T, r *http.Request) string {
	t.Helper()
	var body struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Errorf("decode request: %v", err)
		return ""
	}
	last := ""
	for _, message := range body.Messages {
		if message.Role != "user" {
			continue
		}
		var text string
		if json.Unmarshal(message.Content, &text) == nil {
			last = text
			continue
		}
		var blocks []struct{ Type, Text string }
		if json.Unmarshal(message.Content, &blocks) == nil {
			var parts []string
			for _, block := range blocks {
				if block.Type == "text" {
					parts = append(parts, block.Text)
				}
			}
			last = strings.Join(parts, " ")
		}
	}
	return last
}

func writeFakeCompletion(w http.ResponseWriter, answer string, number int32) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	chunks := []map[string]any{
		{"id": fmt.Sprintf("chatcmpl-%d", number), "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}}},
		{"id": fmt.Sprintf("chatcmpl-%d", number), "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": answer}, "finish_reason": nil}}},
		{"id": fmt.Sprintf("chatcmpl-%d", number), "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}},
	}
	for _, chunk := range chunks {
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	w.(http.Flusher).Flush()
}
