package opencode

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
	"syscall"
	"testing"
	"time"
)

func TestClientUsesLegacyProjectScopedSynchronousPrompt(t *testing.T) {
	t.Parallel()
	workdir := t.TempDir()
	promptEntered := make(chan struct{})
	releasePrompt := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "user" || password != "secret" {
			t.Errorf("basic auth = (%q, %q, %v)", username, password, ok)
		}
		if request.URL.Query().Get("directory") != workdir || request.Header.Get("X-OpenCode-Directory") != workdir {
			t.Errorf("project routing query=%q header=%q", request.URL.Query().Get("directory"), request.Header.Get("X-OpenCode-Directory"))
		}
		switch request.URL.Path {
		case "/session":
			_, _ = fmt.Fprintf(response, `{"id":"ses_one","slug":"one","projectID":"p","directory":%q,"title":"one","version":"1.18.31","time":{"created":1,"updated":1}}`, workdir)
		case "/session/ses_one/message":
			close(promptEntered)
			<-releasePrompt
			_, _ = response.Write([]byte(`{"info":{"role":"assistant"},"parts":[]}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := newClient(server.URL, "user", "secret", workdir, server.Client())
	session, err := client.CreateSession(context.Background(), SessionCreateInput{})
	if err != nil || session.Id != "ses_one" {
		t.Fatalf("CreateSession() = (%q, %v)", session.Id, err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := client.Prompt(context.Background(), session.Id, PromptInput{
			Parts: []TextPartInput{{Type: TextPartInputTypeText, Text: "hello"}},
		})
		done <- err
	}()
	select {
	case <-promptEntered:
	case <-time.After(time.Second):
		t.Fatal("prompt request did not arrive")
	}
	select {
	case err := <-done:
		t.Fatalf("Prompt returned before the synchronous response: %v", err)
	default:
	}
	close(releasePrompt)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEventsPreserveUnknownRawEvent(t *testing.T) {
	t.Parallel()
	raw := `{"directory":"/tmp/project","payload":{"type":"future.event","unknown":{"keep":true}}}`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(response, "event: message\ndata: %s\n\n", raw)
	}))
	defer server.Close()

	client := newClient(server.URL, "user", "secret", "/tmp/project", server.Client())
	stop := errors.New("observed")
	var got RawEvent
	err := client.Events(context.Background(), func(event RawEvent) error {
		got = event
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("Events error = %v", err)
	}
	if string(got.Raw) != raw || got.Directory != "/tmp/project" {
		t.Fatalf("raw event = %s, directory = %q", got.Raw, got.Directory)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Payload, &payload); err != nil || payload["type"] != "future.event" {
		t.Fatalf("payload = %s (%v)", got.Payload, err)
	}
}

func TestEventsPassMalformedRawFrameWithoutEndingTheStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(response, "data: {not-json}\n\ndata: {\"payload\":{\"type\":\"after.malformed\"}}\n\n")
	}))
	defer server.Close()

	client := newClient(server.URL, "user", "secret", "/tmp/project", server.Client())
	stop := errors.New("observed both")
	var events []RawEvent
	err := client.Events(context.Background(), func(event RawEvent) error {
		events = append(events, event)
		if len(events) == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("Events error = %v", err)
	}
	if len(events) != 2 || string(events[0].Raw) != "{not-json}" || events[0].Malformed == "" {
		t.Fatalf("malformed event = %+v", events)
	}
	if events[1].Malformed != "" || string(events[1].Payload) != `{"type":"after.malformed"}` {
		t.Fatalf("event after malformed = %+v", events[1])
	}
}

func TestGeneratedMessageAndPartUnionsKeepUpstreamVariants(t *testing.T) {
	t.Parallel()
	var message Message
	if err := json.Unmarshal([]byte(`{"id":"msg_1","sessionID":"ses_1","role":"assistant","time":{"created":1},"parentID":"msg_0","modelID":"m","providerID":"p","mode":"build","agent":"build","path":{"cwd":"/tmp","root":"/tmp"},"cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}`), &message); err != nil {
		t.Fatal(err)
	}
	assistant, err := message.AsAssistantMessage()
	if err != nil || assistant.Id != "msg_1" {
		t.Fatalf("assistant union = (%q, %v)", assistant.Id, err)
	}

	var part Part
	if err := json.Unmarshal([]byte(`{"id":"prt_1","sessionID":"ses_1","messageID":"msg_1","type":"text","text":"done"}`), &part); err != nil {
		t.Fatal(err)
	}
	text, err := part.AsTextPart()
	if err != nil || text.Text != "done" {
		t.Fatalf("text union = (%q, %v)", text.Text, err)
	}
}

func TestStopWithoutStateDoesNotStartServer(t *testing.T) {
	t.Parallel()
	stateDir := filepath.Join(t.TempDir(), "state")
	stopped, err := StopServer(context.Background(), stateDir)
	if err != nil || stopped {
		t.Fatalf("StopServer() = (%v, %v)", stopped, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, stateFileName)); !os.IsNotExist(err) {
		t.Fatalf("stop unexpectedly published server state: %v", err)
	}
}

func TestCancelledDiscoveryDoesNotStopExistingServer(t *testing.T) {
	t.Parallel()
	process := exec.Command("sleep", "30")
	prepareServerProcess(process)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = signalProcessGroup(process.Process.Pid, syscall.SIGTERM)
		_ = process.Wait()
	})
	token, alive, err := processIdentity(process.Process.Pid)
	if err != nil || !alive {
		t.Fatalf("process identity = (%q, %v, %v)", token, alive, err)
	}
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := prepareStateDir(stateDir); err != nil {
		t.Fatal(err)
	}
	state := serverState{
		PID:          process.Process.Pid,
		ProcessToken: token,
		URL:          "http://127.0.0.1:1",
		Username:     "gimble",
		Password:     "secret",
	}
	if err := writeServerState(stateDir, state); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := ensureServer(ctx, stateDir); !errors.Is(err, context.Canceled) {
		t.Fatalf("ensureServer error = %v", err)
	}
	current, alive, err := processIdentity(process.Process.Pid)
	if err != nil || !alive || current != token {
		t.Fatalf("discovery changed shared process: token=%q alive=%v err=%v", current, alive, err)
	}
	got, err := readServerState(stateDir)
	if err != nil || got != state {
		t.Fatalf("discovery changed shared state: got=%+v err=%v", got, err)
	}
}
