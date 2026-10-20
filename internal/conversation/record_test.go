package conversation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

func TestRecordAppendsOnceAndSurvivesRestart(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	factory := func(string) (gimbal.HarnessAdapter, error) { return nil, nil }
	manager, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Create(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Record(item.ID, "", nil); err == nil {
		t.Fatal("recorded without a source")
	}
	entries := []Entry{
		{Seq: 1, Message: Message{Role: "zoom", Text: "Ana: hello", Created: 100}},
		{Seq: 2, Message: Message{Role: "zoom", Text: "Bo: hi", Created: 200}},
	}
	if n, err := manager.Record(item.ID, "a", entries); err != nil || n != 2 {
		t.Fatalf("first record = %d, %v", n, err)
	}
	// Replaying the same events, and a second source at the same sequence.
	if n, err := manager.Record(item.ID, "a", entries); err != nil || n != 0 {
		t.Fatalf("replay = %d, %v", n, err)
	}
	if n, err := manager.Record(item.ID, "b", entries[:1]); err != nil || n != 1 {
		t.Fatalf("second source = %d, %v", n, err)
	}
	manager.Close()

	restarted, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	saved, _ := restarted.Get(item.ID)
	if len(saved.Messages) != 3 || saved.Messages[1].Text != "Bo: hi" || saved.Messages[1].Created != 200 {
		t.Fatalf("saved messages = %+v", saved.Messages)
	}
	if cursor, err := restarted.Cursor(item.ID, "a"); err != nil || cursor != 2 {
		t.Fatalf("cursor a = %d", cursor)
	}
	if n, _ := restarted.Record(item.ID, "a", entries); n != 0 {
		t.Fatalf("restart replay appended %d", n)
	}
	if _, err := restarted.Cursor("missing", "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cursor for a missing conversation: %v", err)
	}
}

func TestRecordRefusesWorkflowBackedConversation(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	manager, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	item, err := manager.Create(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.items[item.ID].WorkflowRun = "run-1"
	manager.mu.Unlock()
	if _, err := manager.Record(item.ID, "a", []Entry{{Seq: 1, Message: Message{Role: "zoom", Text: "x"}}}); !errors.Is(err, ErrWorkflowBacked) {
		t.Fatalf("err = %v, want ErrWorkflowBacked", err)
	}
}

func TestRecordingFindsItsConversationAfterRestart(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	factory := func(string) (gimbal.HarnessAdapter, error) { return nil, nil }
	manager, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.Recording(t.Context(), "zoom-connect:h/c", "Zoom call h/c")
	if err != nil {
		t.Fatal(err)
	}
	again, err := manager.Recording(t.Context(), "zoom-connect:h/c", "Zoom call h/c")
	if err != nil || again != first {
		t.Fatalf("second ask = %q, %v; want %q", again, err, first)
	}
	other, _ := manager.Recording(t.Context(), "zoom-connect:h/other", "Zoom call h/other")
	if other == first {
		t.Fatal("two sources share a conversation")
	}
	manager.Close()
	restarted, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if found, err := restarted.Recording(t.Context(), "zoom-connect:h/c", "x"); err != nil || found != first {
		t.Fatalf("after restart = %q, %v; want %q", found, err, first)
	}
	if len(restarted.List()) != 2 {
		t.Fatalf("conversations = %d, want 2", len(restarted.List()))
	}
}

func TestRecordingIsReadOnlyAndRefusesChat(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	factory := func(string) (gimbal.HarnessAdapter, error) { return nil, nil }
	manager, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	started := false
	manager.WorkflowStart = func(context.Context, Conversation, backend.Route) error { started = true; return nil }
	manager.WorkflowInput = func(string, string, string) error { started = true; return nil }
	id, err := manager.Recording(t.Context(), "zoom-connect:h/c", "Zoom call h/c")
	if err != nil {
		t.Fatal(err)
	}
	if item, _ := manager.Get(id); item.Live {
		t.Fatal("a recording is live; the page would offer a composer")
	}
	if _, err := manager.Record(id, "zoom-connect:h/c", []Entry{{Seq: 1, Message: Message{Role: "zoom", Text: "Ana: hello"}}}); err != nil {
		t.Fatal(err)
	}
	check := func(m *Manager) {
		t.Helper()
		item, _ := m.Get(id)
		if item.Live || item.WorkflowRun != "" || len(item.Messages) != 1 || item.Messages[0].Text != "Ana: hello" {
			t.Fatalf("recording = live %v run %q messages %+v", item.Live, item.WorkflowRun, item.Messages)
		}
	}
	if _, err := manager.Submit(id, Request{ID: "r1", Kind: "prompt", Message: "hi"}); !errors.Is(err, ErrRecordOnly) {
		t.Fatalf("submit = %v, want ErrRecordOnly", err)
	}
	if err := manager.Associate(id, "run-1", "w", filepath.Dir(project)); !errors.Is(err, ErrRecordOnly) {
		t.Fatalf("associate = %v, want ErrRecordOnly", err)
	}
	if started {
		t.Fatal("a workflow was started")
	}
	check(manager)
	manager.Close()
	restarted, err := New(t.Context(), project, factory, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	check(restarted)
	if _, err := restarted.Submit(id, Request{ID: "r2", Kind: "prompt", Message: "hi"}); !errors.Is(err, ErrRecordOnly) {
		t.Fatalf("submit after restart = %v", err)
	}
}

func TestRecordingIsSavedWithItsCursor(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	manager, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	id, err := manager.Recording(t.Context(), "zoom-connect:h/c", "Zoom call h/c")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(project, "conversations", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"zoom-connect:h/c": 0`) || !strings.Contains(string(data), `"live": false`) {
		t.Fatalf("the first save lacks the cursor or live=false:\n%s", data)
	}
}

func TestRecordingCoexistsWithCreatingOrdinaryConversations(t *testing.T) {
	project := filepath.Join(t.TempDir(), ".gimbal")
	manager, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	const source = "zoom-connect:h/c"
	id, err := manager.Recording(t.Context(), source, "Zoom call h/c")
	if err != nil {
		t.Fatal(err)
	}
	const count = 50
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for seq := int64(1); seq <= count; seq++ {
			if _, err := manager.Record(id, source, []Entry{{Seq: seq, Message: Message{Role: "zoom", Text: "line"}}}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range count {
			if _, err := manager.Create(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range count {
			if _, err := manager.Recording(t.Context(), source+"-other", "other"); err != nil {
				t.Error(err)
				return
			}
			manager.List()
		}
	}()
	wg.Wait()
	if item, _ := manager.Get(id); len(item.Messages) != count {
		t.Fatalf("recording has %d messages, want %d", len(item.Messages), count)
	}
	if got := len(manager.List()); got != count+2 {
		t.Fatalf("conversations = %d, want %d", got, count+2)
	}
}
