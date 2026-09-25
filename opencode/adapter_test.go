package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestAdapterContractAndRawCapture(t *testing.T) {
	t.Parallel()
	fake := newFakeOpenCode(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	ad := newAdapter(adapterConfig{
		stateDir: stateDir,
		connect: func(_ context.Context, workdir, _ string) (*Client, error) {
			return newClient(fake.server.URL, "user", "secret", workdir, fake.server.Client()), nil
		},
	})

	workOne := t.TempDir()
	workTwo := t.TempDir()
	one, err := ad.CreateSession(t.Context(), "anthropic/claude-test", "low", workOne)
	if err != nil {
		t.Fatal(err)
	}
	two, err := ad.CreateSession(t.Context(), "native-test", "", workTwo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ad.Close(context.Background(), one)
		_ = ad.Close(context.Background(), two)
	})

	t.Run("text schema and continuation", func(t *testing.T) {
		var events []gimbal.AgentEvent
		text, err := ad.RunTurn(t.Context(), one, "remember pear", nil, func(event gimbal.AgentEvent) error {
			events = append(events, event)
			return nil
		})
		if err != nil || string(text.Output) != `"remember pear"` {
			t.Fatalf("text turn = (%s, %v)", text.Output, err)
		}
		continued, err := ad.RunTurn(t.Context(), one, "CONTINUE", nil, func(gimbal.AgentEvent) error { return nil })
		if err != nil || string(continued.Output) != `"pear"` {
			t.Fatalf("continued turn = (%s, %v)", continued.Output, err)
		}
		types := adapterEventTypes(events)
		for _, want := range []string{"session.step.started", "session.text.delta", "session.text.ended", "session.step.ended"} {
			if !slices.Contains(types, want) {
				t.Fatalf("events %v omit %q", types, want)
			}
		}

		schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`)
		structured, err := ad.RunTurn(t.Context(), one, "SCHEMA", schema, func(gimbal.AgentEvent) error { return nil })
		if err != nil || string(structured.Output) != `{"answer":"valid"}` {
			t.Fatalf("schema turn = (%s, %v)", structured.Output, err)
		}
	})

	t.Run("concurrent routing fork and isolated close", func(t *testing.T) {
		var mu sync.Mutex
		routed := map[string][]string{}
		group := sync.WaitGroup{}
		for _, item := range []struct {
			id, prompt string
		}{{one, "ONE"}, {two, "TWO"}} {
			group.Go(func() {
				_, runErr := ad.RunTurn(t.Context(), item.id, item.prompt, nil, func(event gimbal.AgentEvent) error {
					var data struct {
						SessionID string `json:"sessionID"`
					}
					_ = json.Unmarshal(event.Data, &data)
					mu.Lock()
					routed[item.id] = append(routed[item.id], data.SessionID)
					mu.Unlock()
					return nil
				})
				if runErr != nil {
					t.Errorf("RunTurn(%s): %v", item.id, runErr)
				}
			})
		}
		group.Wait()
		for id, sessions := range routed {
			if len(sessions) == 0 || slices.ContainsFunc(sessions, func(got string) bool { return got != id }) {
				t.Fatalf("session %s received routes %v", id, sessions)
			}
		}

		fork, err := ad.Fork(t.Context(), one)
		if err != nil || fork == one {
			t.Fatalf("Fork() = (%q, %v)", fork, err)
		}
		forked, err := ad.RunTurn(t.Context(), fork, "CONTINUE", nil, func(gimbal.AgentEvent) error { return nil })
		if err != nil || string(forked.Output) != `"pear"` {
			t.Fatalf("fork continuation = (%s, %v)", forked.Output, err)
		}
		if err := ad.Close(t.Context(), fork); err != nil {
			t.Fatal(err)
		}
		if err := ad.Close(t.Context(), one); err != nil {
			t.Fatal(err)
		}
		stillUsable, err := ad.RunTurn(t.Context(), two, "STILL", nil, func(gimbal.AgentEvent) error { return nil })
		if err != nil || string(stillUsable.Output) != `"STILL"` {
			t.Fatalf("other session after close = (%s, %v)", stillUsable.Output, err)
		}
	})

	t.Run("raw unknown event is correlated with POST", func(t *testing.T) {
		ad.mu.Lock()
		capturePath := ad.capture.path
		ad.mu.Unlock()
		if _, err := ad.RunTurn(t.Context(), two, "UNKNOWN", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			data, err := os.ReadFile(capturePath)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(`future.event`)) &&
				bytes.Contains(data, []byte(`{not-json}`)) &&
				bytes.Contains(data, []byte(`"kind":"request"`)) &&
				bytes.Contains(data, []byte(`"kind":"result"`)) &&
				bytes.Contains(data, []byte(`"request_id":"req_`)) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("capture lacks correlated unknown event and POST records:\n%s", data)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("observation failure does not overrule POST success", func(t *testing.T) {
		observationErr := errors.New("recording unavailable")
		result, err := ad.RunTurn(t.Context(), two, "GAP", nil, func(gimbal.AgentEvent) error {
			return observationErr
		})
		if err != nil || string(result.Output) != `"GAP"` {
			t.Fatalf("turn with observation failure = (%s, %v)", result.Output, err)
		}
	})
}

func TestAdapterSteeringAndCancellationAbortActivePrompt(t *testing.T) {
	t.Parallel()
	fake := newFakeOpenCode(t)
	ad := newAdapter(adapterConfig{
		stateDir: filepath.Join(t.TempDir(), "state"),
		connect: func(_ context.Context, workdir, _ string) (*Client, error) {
			return newClient(fake.server.URL, "user", "secret", workdir, fake.server.Client()), nil
		},
	})
	sessionID, err := ad.CreateSession(t.Context(), "native-test", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ad.Close(context.Background(), sessionID) })

	result := make(chan struct {
		turn gimbal.TurnResult
		err  error
	}, 1)
	go func() {
		turn, runErr := ad.RunTurn(context.Background(), sessionID, "BLOCK", nil, func(gimbal.AgentEvent) error { return nil })
		result <- struct {
			turn gimbal.TurnResult
			err  error
		}{turn, runErr}
	}()
	fake.waitStarted(t, "BLOCK")
	landed, err := ad.Steer(t.Context(), sessionID, "STEERED")
	if err != nil || !landed {
		t.Fatalf("Steer() = (%v, %v)", landed, err)
	}
	steered := <-result
	if steered.err != nil || string(steered.turn.Output) != `"STEERED"` {
		t.Fatalf("steered turn = (%s, %v)", steered.turn.Output, steered.err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancelled := make(chan error, 1)
	go func() {
		_, runErr := ad.RunTurn(cancelCtx, sessionID, "WAIT", nil, func(gimbal.AgentEvent) error { return nil })
		cancelled <- runErr
	}()
	fake.waitStarted(t, "WAIT")
	cancel()
	select {
	case err := <-cancelled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled RunTurn error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled RunTurn hung")
	}
	fake.waitAborted(t, sessionID)
	if landed, err := ad.Steer(t.Context(), sessionID, "late"); err != nil || landed {
		t.Fatalf("late Steer() = (%v, %v)", landed, err)
	}
}

func TestAdapterSteerWaitsForAbortSettlementBeforeContinuation(t *testing.T) {
	t.Parallel()
	fake := newFakeOpenCode(t)
	fake.delayAbort = true
	ad := newAdapter(adapterConfig{
		stateDir: filepath.Join(t.TempDir(), "state"),
		connect: func(_ context.Context, workdir, _ string) (*Client, error) {
			return newClient(fake.server.URL, "user", "secret", workdir, fake.server.Client()), nil
		},
	})
	sessionID, err := ad.CreateSession(t.Context(), "native-test", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ad.Close(context.Background(), sessionID) })

	type runResult struct {
		turn gimbal.TurnResult
		err  error
	}
	runDone := make(chan runResult, 1)
	go func() {
		turn, runErr := ad.RunTurn(context.Background(), sessionID, "NATURAL", nil, func(gimbal.AgentEvent) error { return nil })
		runDone <- runResult{turn: turn, err: runErr}
	}()
	fake.waitStarted(t, "NATURAL")

	type steerResult struct {
		landed bool
		err    error
	}
	steerDone := make(chan steerResult, 1)
	go func() {
		landed, steerErr := ad.Steer(context.Background(), sessionID, "LATE_STEER")
		steerDone <- steerResult{landed: landed, err: steerErr}
	}()
	select {
	case got := <-fake.abortEntered:
		if got != sessionID {
			t.Fatalf("Abort session = %q, want %q", got, sessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Abort request did not arrive")
	}

	// Let the original POST finish while its Abort request is still delayed.
	close(fake.releaseNatural)
	earlyContinuation := false
	select {
	case got := <-fake.started:
		if got != "LATE_STEER" {
			t.Fatalf("started prompt = %q, want LATE_STEER", got)
		}
		earlyContinuation = true
	case <-time.After(100 * time.Millisecond):
	}
	close(fake.releaseAbort)

	steered := <-steerDone
	if steered.err != nil || !steered.landed {
		t.Fatalf("Steer() = (%v, %v)", steered.landed, steered.err)
	}
	if !earlyContinuation {
		fake.waitStarted(t, "LATE_STEER")
	}
	result := <-runDone
	if earlyContinuation {
		t.Fatalf("continuation started before Abort settled; turn = (%s, %v)", result.turn.Output, result.err)
	}
	if result.err != nil || string(result.turn.Output) != `"LATE_STEER"` {
		t.Fatalf("steered turn = (%s, %v)", result.turn.Output, result.err)
	}
}

type fakeOpenCode struct {
	t      *testing.T
	server *httptest.Server

	mu             sync.Mutex
	nextSession    int
	sessions       map[string]*fakeSession
	subscribers    map[chan []byte]struct{}
	active         map[string]chan struct{}
	started        chan string
	aborted        chan string
	delayAbort     bool
	abortEntered   chan string
	releaseAbort   chan struct{}
	releaseNatural chan struct{}
}

type fakeSession struct {
	directory string
	history   []string
}

func newFakeOpenCode(t *testing.T) *fakeOpenCode {
	t.Helper()
	fake := &fakeOpenCode{
		t: t, sessions: make(map[string]*fakeSession), subscribers: make(map[chan []byte]struct{}),
		active: make(map[string]chan struct{}), started: make(chan string, 20), aborted: make(chan string, 20),
		abortEntered: make(chan string, 1), releaseAbort: make(chan struct{}), releaseNatural: make(chan struct{}),
	}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeOpenCode) serveHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/global/event" {
		f.events(response, request)
		return
	}
	if request.URL.Path == "/session" && request.Method == http.MethodPost {
		f.create(response, request)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "session" {
		http.NotFound(response, request)
		return
	}
	sessionID := parts[1]
	switch parts[2] {
	case "message":
		f.prompt(response, request, sessionID)
	case "abort":
		f.abort(response, sessionID)
	case "fork":
		f.fork(response, request, sessionID)
	default:
		http.NotFound(response, request)
	}
}

func (f *fakeOpenCode) events(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "no flusher", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	stream := make(chan []byte, 64)
	f.mu.Lock()
	f.subscribers[stream] = struct{}{}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.subscribers, stream)
		f.mu.Unlock()
	}()
	_, _ = io.WriteString(response, "event: message\ndata: {\"payload\":{\"type\":\"server.connected\",\"properties\":{}}}\n\n")
	flusher.Flush()
	for {
		select {
		case raw, ok := <-stream:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(response, "event: message\ndata: %s\n\n", raw)
			flusher.Flush()
		case <-request.Context().Done():
			return
		}
	}
}

func (f *fakeOpenCode) create(response http.ResponseWriter, request *http.Request) {
	var input SessionCreateInput
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if input.Model == nil || input.Model.ProviderID == "" || input.Model.ID == "" {
		http.Error(response, "model required", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.nextSession++
	id := fmt.Sprintf("ses_%d", f.nextSession)
	directory := request.URL.Query().Get("directory")
	f.sessions[id] = &fakeSession{directory: directory}
	f.mu.Unlock()
	f.writeSession(response, id, directory)
}

func (f *fakeOpenCode) prompt(response http.ResponseWriter, request *http.Request, sessionID string) {
	var input PromptInput
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	prompt := input.Parts[0].Text
	f.mu.Lock()
	session := f.sessions[sessionID]
	if session == nil {
		f.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	directory := session.directory
	if prompt != "CONTINUE" {
		session.history = append(session.history, strings.TrimPrefix(prompt, "remember "))
	}
	f.mu.Unlock()

	if prompt == "NATURAL" {
		wait := make(chan struct{})
		f.mu.Lock()
		f.active[sessionID] = wait
		f.mu.Unlock()
		f.started <- prompt
		<-f.releaseNatural
		f.mu.Lock()
		if f.active[sessionID] == wait {
			delete(f.active, sessionID)
		}
		f.mu.Unlock()
		f.writePromptResponse(response, sessionID, "msg_natural", prompt, nil, nil)
		return
	}
	if prompt == "LATE_STEER" {
		select {
		case <-f.releaseAbort:
			f.started <- prompt
			f.writePromptResponse(response, sessionID, "msg_late_steer", prompt, nil, nil)
			return
		default:
		}
		wait := make(chan struct{})
		f.mu.Lock()
		f.active[sessionID] = wait
		f.mu.Unlock()
		f.started <- prompt
		<-wait
		f.writePromptResponse(response, sessionID, "msg_aborted", "", nil, map[string]any{
			"name": "MessageAbortedError", "data": map[string]any{"message": "aborted"},
		})
		return
	}
	if prompt == "BLOCK" || prompt == "WAIT" {
		wait := make(chan struct{})
		f.mu.Lock()
		f.active[sessionID] = wait
		f.mu.Unlock()
		f.started <- prompt
		<-wait
		f.writePromptResponse(response, sessionID, "msg_aborted", "", nil, map[string]any{
			"name": "MessageAbortedError", "data": map[string]any{"message": "aborted"},
		})
		return
	}

	text := prompt
	if prompt == "CONTINUE" {
		f.mu.Lock()
		if len(session.history) > 0 {
			text = session.history[0]
		}
		f.mu.Unlock()
	}
	messageID := "msg_" + strings.ToLower(prompt)
	f.publish(directory, map[string]any{
		"id": "evt_part_" + prompt, "type": "message.part.delta",
		"properties": map[string]any{
			"sessionID": sessionID, "messageID": messageID, "partID": "prt_" + prompt,
			"field": "text", "delta": text,
		},
	})
	if prompt == "UNKNOWN" {
		f.publish(directory, map[string]any{
			"id": "evt_future", "type": "future.event",
			"properties": map[string]any{"sessionID": sessionID, "unknown": map[string]any{"keep": true}},
		})
		f.publishRaw([]byte(`{not-json}`))
	}
	if prompt == "GAP" {
		f.disconnectEvents()
	}
	if prompt == "SCHEMA" {
		f.writePromptResponse(response, sessionID, messageID, "", map[string]any{"answer": "valid"}, nil)
		return
	}
	f.writePromptResponse(response, sessionID, messageID, text, nil, nil)
}

func (f *fakeOpenCode) abort(response http.ResponseWriter, sessionID string) {
	if f.delayAbort {
		f.abortEntered <- sessionID
		<-f.releaseAbort
	}
	f.mu.Lock()
	wait := f.active[sessionID]
	delete(f.active, sessionID)
	f.mu.Unlock()
	if wait == nil {
		_, _ = io.WriteString(response, "false")
		return
	}
	close(wait)
	f.aborted <- sessionID
	_, _ = io.WriteString(response, "true")
}

func (f *fakeOpenCode) fork(response http.ResponseWriter, request *http.Request, sessionID string) {
	f.mu.Lock()
	parent := f.sessions[sessionID]
	if parent == nil {
		f.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	f.nextSession++
	id := fmt.Sprintf("ses_%d", f.nextSession)
	history := append([]string(nil), parent.history...)
	f.sessions[id] = &fakeSession{directory: parent.directory, history: history}
	f.mu.Unlock()
	f.writeSession(response, id, parent.directory)
}

func (f *fakeOpenCode) publish(directory string, payload any) {
	raw, _ := json.Marshal(map[string]any{"directory": directory, "payload": payload})
	f.publishRaw(raw)
}

func (f *fakeOpenCode) publishRaw(raw []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for subscriber := range f.subscribers {
		subscriber <- raw
	}
}

func (f *fakeOpenCode) disconnectEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for subscriber := range f.subscribers {
		delete(f.subscribers, subscriber)
		close(subscriber)
	}
}

func (f *fakeOpenCode) writeSession(response http.ResponseWriter, id, directory string) {
	_, _ = fmt.Fprintf(response, `{"id":%q,"slug":"test","projectID":"p","directory":%q,"title":"test","version":"test","time":{"created":1,"updated":1}}`, id, directory)
}

func (f *fakeOpenCode) writePromptResponse(response http.ResponseWriter, sessionID, messageID, text string, structured, nativeErr any) {
	info := map[string]any{
		"id": messageID, "sessionID": sessionID, "role": "assistant", "parentID": "msg_user",
		"modelID": "native-test", "providerID": "opencode", "mode": "build", "agent": "build",
		"path": map[string]any{"cwd": "/tmp", "root": "/tmp"}, "cost": 0,
		"time": map[string]any{"created": 1, "completed": 2},
		"tokens": map[string]any{
			"input": 1, "output": 1, "reasoning": 0, "cache": map[string]any{"read": 0, "write": 0},
		},
	}
	if structured != nil {
		info["structured"] = structured
	}
	if nativeErr != nil {
		info["error"] = nativeErr
	}
	parts := []any{}
	if text != "" {
		parts = append(parts, map[string]any{
			"id": "prt_result", "sessionID": sessionID, "messageID": messageID,
			"type": "text", "text": text,
		})
	}
	_ = json.NewEncoder(response).Encode(map[string]any{"info": info, "parts": parts})
}

func (f *fakeOpenCode) waitStarted(t *testing.T, prompt string) {
	t.Helper()
	select {
	case got := <-f.started:
		if got != prompt {
			t.Fatalf("started prompt = %q, want %q", got, prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("prompt %q did not start", prompt)
	}
}

func (f *fakeOpenCode) waitAborted(t *testing.T, sessionID string) {
	t.Helper()
	select {
	case got := <-f.aborted:
		if got != sessionID {
			t.Fatalf("aborted session = %q, want %q", got, sessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("session %q was not aborted", sessionID)
	}
}

func adapterEventTypes(events []gimbal.AgentEvent) []string {
	types := make([]string, len(events))
	for index, event := range events {
		types[index] = event.Type
	}
	return types
}
