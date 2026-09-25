package pi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

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
		return fmt.Errorf("answer is required")
	}
	return nil
}

func TestProjectedEventsUseGimbalCompatibleNativeRefs(t *testing.T) {
	var events []gimbal.AgentEvent
	run := newTurn("pi-session", "deepseek-4.1-flash", func(event gimbal.AgentEvent) error {
		events = append(events, event)
		return nil
	})
	run.record(wireRecord{Type: "message_update", Event: json.RawMessage(`{"type":"text_delta","contentIndex":0,"delta":"hello"}`)})
	run.record(wireRecord{Type: "tool_execution_start", ToolID: "call-1", ToolName: "bash", Args: json.RawMessage(`{"command":"cat seed.txt"}`)})
	run.record(wireRecord{Type: "tool_execution_end", ToolID: "call-1", ToolName: "bash", Result: json.RawMessage(`{"content":[{"type":"text","text":"ORBIT-42"}]}`)})

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

func TestEffortMustBeBlankAndModelIDIsBound(t *testing.T) {
	a := newAdapter(adapterConfig{apiKey: func() string { return "test-key" }})
	if _, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "high", t.TempDir()); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("CreateSession effort error = %v", err)
	}
	if got := modelID("diffusion/deepseek-4.1-flash"); got != "deepseek-4.1-flash" {
		t.Fatalf("modelID = %q", got)
	}
}

func TestModelsConfigContainsOnlyEnvironmentReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	if err := writeConfig(path, "http://127.0.0.1:1234/v1", "glm-5.3-flash"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test-secret") || !strings.Contains(string(data), `"apiKey":"$DIFFUSION_API_KEY"`) {
		t.Fatalf("models.json did not keep the key as an environment reference: %s", data)
	}
	var config struct {
		Providers map[string]struct {
			Models []struct {
				ID            string   `json:"id"`
				ContextWindow int      `json:"contextWindow"`
				MaxTokens     int      `json:"maxTokens"`
				Input         []string `json:"input"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	models := config.Providers["diffusion"].Models
	if len(models) != 1 || models[0].ID != "glm-5.3-flash" || models[0].ContextWindow != 524288 || models[0].MaxTokens != 163840 || len(models[0].Input) != 2 {
		t.Fatalf("Pi models config = %+v", models)
	}
}

func TestPi0871RPCWithFakeCompletions(t *testing.T) {
	version, err := exec.Command("pi", "--version").CombinedOutput()
	if err != nil {
		t.Skipf("real Pi subprocess test skipped: pi is unavailable: %v", err)
	}
	if got := strings.TrimSpace(string(version)); got != "0.87.1" {
		t.Skipf("real Pi subprocess test skipped: requires pi 0.87.1, found %q", got)
	}
	var requests atomic.Int32
	var authOK atomic.Bool
	var resumeSawHistory atomic.Bool
	var blockNext atomic.Bool
	blocked := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRequest := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseRequest()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") == "Bearer fake-router-key" {
			authOK.Store(true)
		}
		var request struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		number := requests.Add(1)
		if number > 2 {
			for _, message := range request.Messages {
				if message.Role == "assistant" && strings.Contains(string(message.Content), "Pi first answer") {
					resumeSawHistory.Store(true)
				}
			}
		}
		if blockNext.CompareAndSwap(true, false) {
			close(blocked)
			<-release
		}
		answer := "Pi first answer"
		if number == 3 {
			answer = `{"wrong":"no"}`
		} else if number == 4 {
			answer = `{"answer":"yes"}`
		} else if number > 3 {
			answer = "Pi resumed follow-up"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		chunks := []map[string]any{
			{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}}},
			{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": answer}, "finish_reason": nil}}},
			{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-4.1-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 11, "completion_tokens": 4, "total_tokens": 15}},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	workdir := t.TempDir()
	a := newAdapter(adapterConfig{endpoint: server.URL + "/v1", apiKey: func() string { return "fake-router-key" }})
	sessionID, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.session(sessionID)
	config, err := os.ReadFile(filepath.Join(s.dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "fake-router-key") || !strings.Contains(string(config), `"apiKey":"$DIFFUSION_API_KEY"`) {
		t.Fatalf("session config leaked or failed to reference the environment key: %s", config)
	}
	firstProc := s.proc
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
	if first.Usage["deepseek-4.1-flash"].Tokens.Input != 11 || first.Usage["deepseek-4.1-flash"].Tokens.Output != 4 {
		t.Fatalf("first usage = %#v", first.Usage)
	}
	if len(events) == 0 || events[0].Type != "session.text.delta" {
		t.Fatalf("projected events = %#v", events)
	}
	if s.proc != firstProc {
		t.Fatal("Pi process changed between turns")
	}
	second, err := a.RunTurn(context.Background(), sessionID, "second turn on this conversation", nil, nil)
	if err != nil || string(second.Output) != `"Pi first answer"` {
		t.Fatalf("second turn output=%s err=%v", second.Output, err)
	}
	if s.proc != firstProc {
		t.Fatal("second turn did not reuse the Pi process")
	}
	var schemaResult schemaAnswer
	err = gimbal.Run(gimbal.Project(context.Background(), workdir), "pi-schema-reask", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"pi": {Adapter: a, Model: "diffusion/deepseek-4.1-flash"},
	}, func(ctx context.Context) error {
		var err error
		schemaResult, err = gimbal.NewSession(ctx, "pi", workdir).Generate[schemaAnswer](ctx, "Return an answer")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if schemaResult.Answer != "yes" {
		t.Fatalf("schema output = %#v", schemaResult)
	}
	if requests.Load() != 4 {
		t.Fatalf("Pi request count after schema validation/re-ask = %d, want 4", requests.Load())
	}

	// Save a real native conversation, then simulate an unexpected child exit.
	if err := firstProc.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstProc.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Pi child did not exit")
	}
	resumed, err := a.RunTurn(context.Background(), sessionID, "follow-up", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resumed.Output) != `"Pi resumed follow-up"` {
		t.Fatalf("resumed output = %s", resumed.Output)
	}
	if s.proc == firstProc || requests.Load() < 5 {
		t.Fatalf("replacement process not used: process same=%t requests=%d", s.proc == firstProc, requests.Load())
	}
	if !authOK.Load() {
		t.Fatal("Pi did not resolve the key from the child environment")
	}
	if !resumeSawHistory.Load() {
		t.Fatal("replacement Pi process did not resume the saved conversation")
	}
	blockNext.Store(true)
	turnDone := make(chan error, 1)
	go func() {
		_, err := a.RunTurn(context.Background(), sessionID, "wait for interruption", nil, nil)
		turnDone <- err
	}()
	select {
	case <-blocked:
	case <-time.After(10 * time.Second):
		t.Fatal("fake completion endpoint did not receive the pending turn")
	}
	if err := s.proc.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-turnDone:
		if err == nil || !strings.Contains(err.Error(), "child exited") {
			t.Fatalf("pending turn process-exit error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending turn did not return after Pi exited")
	}
	releaseRequest()
	if _, err := a.RunTurn(context.Background(), sessionID, "recover after exit", nil, nil); err != nil {
		t.Fatalf("run after unexpected exit: %v", err)
	}
	if err := a.Close(context.Background(), sessionID); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(context.Background(), sessionID); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestPi0871SteerForkCancellationAndIsolation(t *testing.T) {
	version, err := exec.Command("pi", "--version").CombinedOutput()
	if err != nil {
		t.Skipf("real Pi lifecycle test skipped: pi is unavailable: %v", err)
	}
	if got := strings.TrimSpace(string(version)); got != "0.87.1" {
		t.Skipf("real Pi lifecycle test skipped: requires pi 0.87.1, found %q", got)
	}
	var requests atomic.Int32
	var boundaryMessages atomic.Int32
	var childHasParentHistory atomic.Bool
	var mu sync.Mutex
	var seen []string
	entered := make(chan struct{}, 1)
	raceEntered := make(chan struct{}, 1)
	cancelEntered := make(chan struct{}, 1)
	release := make(chan struct{})
	raceRelease := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Messages []struct {
				Role string          `json:"role"`
				Text json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		encoded, _ := json.Marshal(body.Messages)
		mu.Lock()
		seen = append(seen, string(encoded))
		mu.Unlock()
		n := requests.Add(1)
		lastUser := ""
		for _, message := range body.Messages {
			if message.Role == "user" {
				var text string
				if json.Unmarshal(message.Text, &text) == nil {
					lastUser = text
				} else {
					var blocks []struct{ Type, Text string }
					if json.Unmarshal(message.Text, &blocks) == nil {
						var parts []string
						for _, block := range blocks {
							if block.Type == "text" {
								parts = append(parts, block.Text)
							}
						}
						lastUser = strings.Join(parts, " ")
					} else {
						lastUser = string(message.Text)
					}
				}
			}
		}
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
		if strings.Contains(lastUser, "settle race gate") {
			select {
			case raceEntered <- struct{}{}:
			default:
			}
			select {
			case <-raceRelease:
			case <-r.Context().Done():
				return
			}
		}
		if strings.Contains(lastUser, "hold for cancellation") {
			select {
			case cancelEntered <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(4 * time.Second):
			}
		}
		answer := "answer-" + lastUser
		if strings.Contains(lastUser, "BOUNDARY STEER") {
			boundaryMessages.Add(1)
		}
		if strings.Contains(lastUser, "child divergent follow-up") && strings.Contains(string(encoded), "shared history answer") {
			childHasParentHistory.Store(true)
		}
		switch {
		case strings.Contains(lastUser, "STEER THIS TURN"):
			answer = "steer was delivered"
		case strings.Contains(lastUser, "BOUNDARY STEER"):
			answer = "boundary steer was delivered"
		case strings.Contains(lastUser, "shared parent history"):
			answer = "shared history answer"
		case strings.Contains(lastUser, "child divergent follow-up"):
			answer = "child-only answer"
		case strings.Contains(lastUser, "parent divergent follow-up"):
			answer = "parent-only answer"
		case strings.Contains(lastUser, "child replacement follow-up"):
			answer = "child replacement answer"
		case strings.Contains(lastUser, "parent after cancellation"):
			answer = "session recovered"
		}
		writeFakeCompletion(w, answer, n)
	}))
	defer server.Close()

	a := newAdapter(adapterConfig{endpoint: server.URL + "/v1", apiKey: func() string { return "fake-router-key" }})
	t.Cleanup(func() {
		a.mu.Lock()
		ids := make([]string, 0, len(a.sessions))
		for id := range a.sessions {
			ids = append(ids, id)
		}
		a.mu.Unlock()
		for _, id := range ids {
			_ = a.Close(context.Background(), id)
		}
	})
	workdir := t.TempDir()
	id, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.session(id)
	type turnOutcome struct {
		result gimbal.TurnResult
		err    error
	}
	turnDone := make(chan turnOutcome, 1)
	go func() {
		result, err := a.RunTurn(context.Background(), id, "hold for steer", nil, nil)
		turnDone <- turnOutcome{result, err}
	}()
	select {
	case <-entered:
	case <-time.After(8 * time.Second):
		t.Fatal("Pi did not reach the blocked Router request")
	}
	steered := make(chan struct {
		landed bool
		err    error
	}, 1)
	go func() {
		landed, err := a.Steer(context.Background(), id, "STEER THIS TURN")
		steered <- struct {
			landed bool
			err    error
		}{landed, err}
	}()
	// Wait until the command has been acknowledged by Pi before allowing the
	// current model response to settle. This exercises Pi's own queue draining.
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		select {
		case result := <-steered:
			if result.err != nil || !result.landed {
				t.Fatalf("Steer landed=%t err=%v", result.landed, result.err)
			}
			goto releaseSteer
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("Pi did not acknowledge the queued steer")
releaseSteer:
	releaseOnce.Do(func() { close(release) })
	steerOutcome := <-turnDone
	if steerOutcome.err != nil || string(steerOutcome.result.Output) != `"steer was delivered"` {
		t.Fatalf("steered turn output=%s err=%v", steerOutcome.result.Output, steerOutcome.err)
	}
	beforeNoTurn := requests.Load()
	if landed, err := a.Steer(context.Background(), id, "must not be sent"); err != nil || landed {
		t.Fatalf("Steer without active turn landed=%t err=%v", landed, err)
	}
	if requests.Load() != beforeNoTurn {
		t.Fatal("Steer without an active turn reached Pi")
	}

	boundaryDone := make(chan turnOutcome, 1)
	go func() {
		result, err := a.RunTurn(context.Background(), id, "settle race gate", nil, nil)
		boundaryDone <- turnOutcome{result, err}
	}()
	select {
	case <-raceEntered:
	case <-time.After(8 * time.Second):
		t.Fatal("Pi did not reach the steering-settlement race")
	}
	boundarySteer := make(chan struct {
		landed bool
		err    error
	}, 1)
	go func() {
		landed, err := a.Steer(context.Background(), id, "BOUNDARY STEER")
		boundarySteer <- struct {
			landed bool
			err    error
		}{landed, err}
	}()
	close(raceRelease)
	steerAtBoundary := <-boundarySteer
	if steerAtBoundary.err != nil {
		t.Fatalf("boundary Steer: %v", steerAtBoundary.err)
	}
	boundaryResult := <-boundaryDone
	if boundaryResult.err != nil {
		t.Fatalf("boundary turn: %v", boundaryResult.err)
	}
	if steerAtBoundary.landed && string(boundaryResult.result.Output) != `"boundary steer was delivered"` {
		t.Fatalf("landed boundary steer was absent from its turn result: %s", boundaryResult.result.Output)
	}
	if !steerAtBoundary.landed && string(boundaryResult.result.Output) == `"boundary steer was delivered"` {
		t.Fatal("Steer returned landed=false after its message entered the current turn")
	}
	if !steerAtBoundary.landed && boundaryMessages.Load() != 0 {
		t.Fatal("a steer that missed settlement leaked into a later Pi turn")
	}

	if _, err := a.RunTurn(context.Background(), id, "shared parent history", nil, nil); err != nil {
		t.Fatal(err)
	}
	childID, err := a.Fork(context.Background(), id)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	child, _ := a.session(childID)
	if child.proc == s.proc || child.dir == s.dir || child.sessionDir == s.sessionDir {
		t.Fatal("fork shares parent process or storage")
	}
	parentResult := make(chan gimbal.TurnResult, 1)
	childResult := make(chan gimbal.TurnResult, 1)
	parallelErr := make(chan error, 2)
	go func() {
		result, err := a.RunTurn(context.Background(), id, "parent divergent follow-up", nil, nil)
		parentResult <- result
		parallelErr <- err
	}()
	go func() {
		result, err := a.RunTurn(context.Background(), childID, "child divergent follow-up", nil, nil)
		childResult <- result
		parallelErr <- err
	}()
	parent := <-parentResult
	childAnswer := <-childResult
	if err := <-parallelErr; err != nil {
		t.Fatalf("concurrent parent/child follow-up: %v", err)
	}
	if string(parent.Output) != `"parent-only answer"` || string(childAnswer.Output) != `"child-only answer"` {
		t.Fatalf("fork results parent=%s child=%s", parent.Output, childAnswer.Output)
	}
	oldChildProc := child.proc
	if err := oldChildProc.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-oldChildProc.done:
	case <-time.After(5 * time.Second):
		t.Fatal("forked Pi did not exit")
	}
	resumed, err := a.RunTurn(context.Background(), childID, "child replacement follow-up", nil, nil)
	if err != nil || string(resumed.Output) != `"child replacement answer"` {
		t.Fatalf("child resume output=%s err=%v", resumed.Output, err)
	}
	if child.proc == oldChildProc {
		t.Fatal("child process was not replaced")
	}
	if !childHasParentHistory.Load() {
		t.Fatal("forked child follow-up did not carry the full parent conversation")
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	procBeforeCancel := s.proc
	cancelDone := make(chan error, 1)
	go func() { _, err := a.RunTurn(cancelCtx, id, "hold for cancellation", nil, nil); cancelDone <- err }()
	select {
	case <-cancelEntered:
	case <-time.After(8 * time.Second):
		t.Fatal("Pi did not reach the cancellation request")
	}
	cancel()
	select {
	case err := <-cancelDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled turn err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled Pi turn did not return")
	}
	recovered, err := a.RunTurn(context.Background(), id, "parent after cancellation", nil, nil)
	if err != nil || string(recovered.Output) != `"session recovered"` {
		t.Fatalf("session after cancellation output=%s err=%v", recovered.Output, err)
	}
	if s.proc != procBeforeCancel {
		t.Fatal("session process was torn down despite successful clear_queue and abort")
	}

	// Two more independent sessions run concurrently through the same adapter.
	left, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	right, err := a.CreateSession(context.Background(), "diffusion/deepseek-4.1-flash", "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	type isolatedResult struct {
		session string
		output  string
		usage   gimbal.Usage
		events  []string
		err     error
	}
	results := make(chan isolatedResult, 2)
	for _, item := range []struct{ id, prompt string }{{left, "isolated left"}, {right, "isolated right"}} {
		wg.Add(1)
		go func(id, prompt string) {
			defer wg.Done()
			var eventSessions []string
			result, err := a.RunTurn(context.Background(), id, prompt, nil, func(event gimbal.AgentEvent) error {
				var data struct {
					SessionID string `json:"sessionID"`
				}
				_ = json.Unmarshal(event.Data, &data)
				eventSessions = append(eventSessions, data.SessionID)
				return nil
			})
			results <- isolatedResult{session: id, output: string(result.Output), usage: result.Usage["deepseek-4.1-flash"], events: eventSessions, err: err}
		}(item.id, item.prompt)
	}
	wg.Wait()
	close(results)
	got := map[string]isolatedResult{}
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent isolated session %s: %v", result.session, result.err)
		}
		got[result.session] = result
	}
	if got[left].output != `"answer-isolated left"` || got[right].output != `"answer-isolated right"` {
		t.Fatalf("concurrent session outputs = %v", got)
	}
	for _, id := range []string{left, right} {
		result := got[id]
		if result.usage.Tokens.Input != 3 || result.usage.Tokens.Output != 2 {
			t.Errorf("session %s usage leaked or is missing: %#v", id, result.usage)
		}
		if len(result.events) == 0 {
			t.Errorf("session %s received no projected events", id)
		}
		for _, eventSession := range result.events {
			if eventSession != id {
				t.Errorf("session %s received event for %s", id, eventSession)
			}
		}
	}
	mu.Lock()
	allRequests := strings.Join(seen, "\n")
	mu.Unlock()
	if !strings.Contains(allRequests, "shared history answer") || !strings.Contains(allRequests, "parent divergent follow-up") || !strings.Contains(allRequests, "child divergent follow-up") {
		t.Fatal("fork follow-ups did not retain the full parent conversation independently")
	}
	if !strings.Contains(allRequests, "child divergent follow-up") || !strings.Contains(allRequests, "shared history answer") {
		t.Fatal("forked Pi request did not include parent conversation history")
	}
	if !steerAtBoundary.landed && boundaryMessages.Load() != 0 {
		t.Fatal("a steer reported as missed later appeared in another Pi turn")
	}
	for _, sessionID := range []string{id, childID, left, right} {
		if err := a.Close(context.Background(), sessionID); err != nil {
			t.Fatal(err)
		}
	}
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

func TestAgentSettledRequiresFinalAssistantAndAcceptsDiagnosticEventGaps(t *testing.T) {
	turn := newTurn("s", "m", func(gimbal.AgentEvent) error { return fmt.Errorf("observer failed") })
	turn.record(wireRecord{Type: "message_update", Event: json.RawMessage(`{"type":"text_delta","delta":"partial"}`)})
	turn.record(wireRecord{Type: "agent_settled"})
	select {
	case <-turn.settled:
	default:
		t.Fatal("agent_settled was not observed")
	}
	if turn.final != nil {
		t.Fatal("partial text was incorrectly treated as authoritative output")
	}
}

func TestToolEventsAreProjected(t *testing.T) {
	var got []gimbal.AgentEvent
	turn := newTurn("s", "m", func(event gimbal.AgentEvent) error {
		got = append(got, event)
		return nil
	})
	turn.record(wireRecord{Type: "message_update", Event: json.RawMessage(`{"type":"toolcall_start","id":"call-1","toolName":"bash"}`)})
	turn.record(wireRecord{Type: "tool_execution_start", ToolID: "call-1", ToolName: "bash", Args: json.RawMessage(`{"command":"pwd"}`)})
	turn.record(wireRecord{Type: "tool_execution_end", ToolID: "call-1", ToolName: "bash", Result: json.RawMessage(`{"content":"/tmp"}`)})
	var types []string
	for _, event := range got {
		types = append(types, event.Type)
	}
	want := []string{"session.tool.input.started", "session.tool.input.ended", "session.tool.called", "session.tool.success"}
	if fmt.Sprint(types) != fmt.Sprint(want) {
		t.Fatalf("projected tool events = %v, want %v", types, want)
	}
}
