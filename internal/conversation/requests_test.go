package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

type controlledChat struct {
	chatAdapter
	started      chan string
	release      chan struct{}
	steerEntered chan struct{}
	steerRelease chan struct{}
	landed       bool
	failure      error
	muControl    sync.Mutex
	calls        int
}

func (a *controlledChat) RunTurn(ctx context.Context, session, prompt string, _ json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.muControl.Lock()
	a.calls++
	call := a.calls
	a.muControl.Unlock()
	for _, e := range []struct{ kind, data string }{
		{"session.step.started", `{"sessionID":"native","assistantMessageID":"same-id"}`},
		{"session.text.started", `{"sessionID":"native","assistantMessageID":"same-id"}`},
		{"session.text.delta", `{"sessionID":"native","assistantMessageID":"same-id","delta":"Partial reply"}`},
	} {
		if err := emit(gimbal.AgentEvent{Type: e.kind, Data: json.RawMessage(e.data)}); err != nil {
			return gimbal.TurnResult{}, err
		}
	}
	a.started <- prompt
	if call == 1 {
		select {
		case <-ctx.Done():
			return gimbal.TurnResult{}, ctx.Err()
		case <-a.release:
		}
	}
	if a.failure != nil {
		return gimbal.TurnResult{}, a.failure
	}
	raw, _ := json.Marshal("Final reply")
	return gimbal.TurnResult{Output: raw}, nil
}
func (a *controlledChat) Steer(context.Context, string, string) (bool, error) {
	if a.steerEntered != nil {
		close(a.steerEntered)
		<-a.steerRelease
	}
	return a.landed, nil
}
func request(id, message string) Request {
	return Request{ID: id, Created: 1, Kind: "prompt", Message: message}
}
func managerWithControl(t *testing.T, a *controlledChat) (*Manager, Conversation) {
	t.Helper()
	a.sessions = map[string][]string{}
	root := t.TempDir()
	m, err := New(t.Context(), filepath.Join(root, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return a, nil }, "/bin/gimbal", "/instance", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := m.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, item
}
func waitRequest(t *testing.T, m *Manager, id, rid, status string) Conversation {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		item, _ := m.Get(id)
		for _, r := range item.Requests {
			if r.ID == rid && r.Status == status {
				return item
			}
		}
		select {
		case <-deadline:
			t.Fatalf("request %s never reached %s", rid, status)
		case <-time.After(time.Millisecond):
		}
	}
}
func TestInteractiveRequestStreamsSteersStopsAndContinues(t *testing.T) {
	a := &controlledChat{started: make(chan string, 3), release: make(chan struct{}), landed: true}
	m, item := managerWithControl(t, a)
	first := request("first", "Start working")
	if _, err := m.Submit(item.ID, first); err != nil {
		t.Fatal(err)
	}
	select {
	case <-a.started:
	case <-time.After(time.Second):
		t.Fatal("admission blocked")
	}
	partial := waitRequest(t, m, item.ID, "first", "running")
	if partial.ActiveRequest != "first" || !containsState(partial, "Partial reply") {
		t.Fatalf("missing live reduction: %+v", partial)
	}
	if _, err := m.Submit(item.ID, first); err != nil {
		t.Fatal(err)
	} // Same admission, never a second native turn.
	if _, err := m.Submit(item.ID, request("steer", "Keep it concise")); err != nil {
		t.Fatal(err)
	}
	steered := waitRequest(t, m, item.ID, "steer", "completed")
	if !containsState(steered, `"steered":true`) || steered.Requests[1].Delivery != "steer" {
		t.Fatalf("steering not reduced: %s", steered.State)
	}
	if _, err := m.Submit(item.ID, Request{ID: "stop", Created: 2, Kind: "interrupt", Target: "first"}); err != nil {
		t.Fatal(err)
	}
	stopped := waitRequest(t, m, item.ID, "first", "canceled")
	if stopped.Status != "canceled" || !containsState(stopped, "Partial reply") || !stopped.Live {
		t.Fatalf("stop lost partial reply/session: %+v", stopped)
	}
	waitRequest(t, m, item.ID, "stop", "completed")
	if _, err := m.Submit(item.ID, request("next", "Continue")); err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, "next", "completed")
	a.muControl.Lock()
	defer a.muControl.Unlock()
	if a.calls != 2 {
		t.Fatalf("native turns = %d", a.calls)
	}
}
func TestSteerThatLosesRaceBecomesNextTurn(t *testing.T) {
	a := &controlledChat{started: make(chan string, 3), release: make(chan struct{}), steerEntered: make(chan struct{}), steerRelease: make(chan struct{})}
	m, item := managerWithControl(t, a)
	_, _ = m.Submit(item.ID, request("first", "Start"))
	<-a.started
	_, err := m.Submit(item.ID, request("late", "Next instruction"))
	if err != nil {
		t.Fatal(err)
	}
	<-a.steerEntered
	close(a.release)
	waitRequest(t, m, item.ID, "first", "completed")
	close(a.steerRelease)
	final := waitRequest(t, m, item.ID, "late", "completed")
	if final.Requests[1].Delivery != "turn" || len(final.Messages) != 4 {
		t.Fatalf("late message lost: %+v", final)
	}
	// A stale stop must never interrupt the next turn.
	if _, err = m.Submit(item.ID, Request{ID: "stale-stop", Created: 2, Kind: "interrupt", Target: "first"}); err == nil {
		t.Fatal("accepted stale stop")
	}
}
func TestUncertainOutcomeDoesNotClaimFailureOrReplay(t *testing.T) {
	a := &controlledChat{started: make(chan string, 2), release: make(chan struct{}), failure: &backend.DeliveryUncertain{RequestID: "first", Err: errors.New("contact lost")}}
	m, item := managerWithControl(t, a)
	_, _ = m.Submit(item.ID, request("first", "Start"))
	<-a.started
	close(a.release)
	result := waitRequest(t, m, item.ID, "first", "unresolved")
	if result.Status != "unresolved" || !containsState(result, "Partial reply") {
		t.Fatalf("uncertainty hidden: %+v", result)
	}
	if _, err := m.Submit(item.ID, request("retry", "Replay")); err == nil {
		t.Fatal("started replacement with unconfirmed outcome")
	}
}
func TestRequestIDRejectsConflictingPayload(t *testing.T) {
	a := &controlledChat{started: make(chan string, 2), release: make(chan struct{})}
	m, item := managerWithControl(t, a)
	_, _ = m.Submit(item.ID, request("first", "Start"))
	<-a.started
	if _, err := m.Submit(item.ID, request("first", "Different work")); err == nil {
		t.Fatal("reused ID for different request")
	}
}
func containsState(item Conversation, text string) bool {
	return strings.Contains(string(item.State), text)
}

func TestUnconfirmedStopRetainsCurrentRequest(t *testing.T) {
	a := &controlledChat{started: make(chan string, 2), release: make(chan struct{}), failure: &backend.DeliveryUncertain{RequestID: "first", Err: context.Canceled}}
	m, item := managerWithControl(t, a)
	_, _ = m.Submit(item.ID, request("first", "Start"))
	<-a.started
	close(a.release)
	current := waitRequest(t, m, item.ID, "first", "unresolved")
	if current.ActiveRequest != "first" || current.Status != "unresolved" {
		t.Fatalf("unconfirmed stop presented as ended: %+v", current)
	}
}
