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
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
)

// This authenticated stand-in covers the manager's workflow handoff only.
// Runtime protocol, interruption and native continuity are tested at their owners.
func newMetadataManager(ctx context.Context, dir string, factory AdapterFactory, cli, instance, project string, registry *observation.Registry) (*Manager, error) {
	m, err := New(ctx, dir, factory, cli, instance, project, registry)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	histories := map[string]gimbal.InteractiveTranscript{}
	states := map[string]string{}
	m.mu.Lock()
	for _, item := range m.items {
		if item.WorkflowRun == "" {
			continue
		}
		history := gimbal.InteractiveTranscript{}
		for _, message := range item.Messages {
			history.Messages = append(history.Messages, gimbal.InteractiveMessage{Role: message.Role, Message: message.Text})
		}
		histories[item.WorkflowRun], states[item.WorkflowRun] = history, item.Status
	}
	m.mu.Unlock()
	m.WorkflowStart = func(_ context.Context, item Conversation, _ backend.Route) error {
		mu.Lock()
		defer mu.Unlock()
		histories[item.WorkflowRun] = gimbal.InteractiveTranscript{}
		states[item.WorkflowRun] = "waiting"
		return nil
	}
	m.WorkflowInput = func(run, address, raw string) error {
		if address != "conversation" {
			return errors.New("wrong stable address")
		}
		var input runstate.InteractionInput
		if err := json.Unmarshal([]byte(raw), &input); err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		history := histories[run]
		if input.Finish {
			history.EndedBy = "human"
			states[run] = "completed"
		} else if !input.Stop && !input.Configure {
			reply := "I will remember cobalt."
			if strings.Contains(input.Message, "What word") {
				reply = "You asked me to remember cobalt."
			}
			history.Messages = append(history.Messages, gimbal.InteractiveMessage{Role: "user", Message: input.Message}, gimbal.InteractiveMessage{Role: "assistant", Message: reply})
			states[run] = "waiting"
		}
		histories[run] = history
		return nil
	}
	m.WorkflowObserve = func(run string) (gimbal.InteractiveTranscript, string, []string, error) {
		mu.Lock()
		defer mu.Unlock()
		history, ok := histories[run]
		if !ok {
			return gimbal.InteractiveTranscript{}, "", nil, errors.New("fixture observation unavailable")
		}
		return history, states[run], nil, nil
	}
	return m, nil
}

type controlledChat struct {
	chatAdapter
	started chan string
	release chan struct{}
	landed  bool
}

func managerWithControl(t *testing.T, a *controlledChat) (*Manager, Conversation) {
	t.Helper()
	dir := t.TempDir()
	m, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return a, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	item, err := m.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	return m, item
}
func request(id, message string) Request {
	return Request{ID: id, Created: 1, Kind: "prompt", Message: message}
}
func waitRequest(t *testing.T, m *Manager, id, rid, status string) Conversation {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		item, _ := m.Get(id)
		for _, request := range item.Requests {
			if request.ID == rid && request.Status == status {
				return item
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("request %s not %s: %+v", rid, status, item)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestRequestIDRejectsConflictingPayload(t *testing.T) {
	m, item := managerWithControl(t, &controlledChat{})
	first := request("stable-id", "First")
	a, err := m.Submit(item.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Submit(item.ID, first)
	if err != nil || a != b {
		t.Fatal("identical retry changed acknowledgement")
	}
	if _, err = m.Submit(item.ID, request("stable-id", "Different")); err == nil {
		t.Fatal("conflicting retry admitted")
	}
}
func TestStopAndFinishUseOrdinaryWorkflowIngress(t *testing.T) {
	m, item := managerWithControl(t, &controlledChat{})
	var inputs []runstate.InteractionInput
	m.WorkflowInput = func(_ string, address, raw string) error {
		if address != "conversation" {
			t.Fatal(address)
		}
		var input runstate.InteractionInput
		_ = json.Unmarshal([]byte(raw), &input)
		inputs = append(inputs, input)
		return nil
	}
	for _, r := range []Request{request("message", "Hello"), {ID: "stop", Kind: "interrupt"}, {ID: "archive", Kind: "finish"}} {
		if _, err := m.Submit(item.ID, r); err != nil {
			t.Fatal(err)
		}
	}
	if len(inputs) != 3 || !inputs[1].Stop || inputs[1].Finish || !inputs[2].Finish {
		t.Fatalf("controls %+v", inputs)
	}
}

func TestFirstInputWaitsForItsOneWorkflowOwner(t *testing.T) {
	m, item := managerWithControl(t, &controlledChat{})
	starts, deliveries := 0, 0
	var originalRun, originalInput string
	m.WorkflowStart = func(_ context.Context, item Conversation, _ backend.Route) error {
		starts++
		originalRun = item.WorkflowRun
		return nil
	}
	m.WorkflowInput = func(run, address, raw string) error {
		deliveries++
		if run != originalRun || address != "conversation" {
			t.Fatal("handoff changed run or address")
		}
		if originalInput == "" {
			originalInput = raw
		} else if originalInput != raw {
			t.Fatal("startup retry changed input identity/content")
		}
		if deliveries < 3 {
			return errors.New("gimbal: interaction owner is starting or unavailable; retry input with the same ID")
		}
		return nil
	}
	first := request("first-message", "Retain this exact message")
	state, err := m.Submit(item.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	if starts != 1 || deliveries != 3 || state.Status != "accepted" {
		t.Fatalf("starts=%d deliveries=%d state=%+v", starts, deliveries, state)
	}
	if _, err := m.Submit(item.ID, first); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || deliveries != 3 {
		t.Fatal("accepted retry restarted or redelivered")
	}
}
