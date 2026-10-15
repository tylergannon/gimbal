package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/projectfiles"
)

func cacheFixture(t *testing.T, m *Manager, data []byte) string {
	t.Helper()
	id := "0123456789abcdef0123456789abcdef"
	dir := filepath.Join(m.projectDir, "files")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := projectfiles.Document{ID: id, Name: "context.pdf", MediaType: "application/pdf", Size: int64(len(data))}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(m.projectDir, "conversation-files", id)
	if err := os.MkdirAll(cached, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cached, doc.Name), data, 0600); err != nil {
		t.Fatal(err)
	}
	return id
}

type effortChat struct {
	chatAdapter
	model, effort string
}

func (a *effortChat) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	a.model, a.effort = model, effort
	return a.chatAdapter.CreateSession(ctx, model, effort, workdir)
}

func TestSessionKeepsExplicitEffortAndDefaultLeavesHarnessChoice(t *testing.T) {
	for _, tc := range []struct{ provider, model, effort, wantModel string }{
		{"codex", "gpt-6-luna", "", "gpt-6-luna"},
		{"codex", "gpt-6-luna", "high", "gpt-6-luna"},
		{"agy", "gemini-3.8-flash-low", "high", "gemini-3.8-flash-high"},
		{"agy", "gemini-3.8-flash-low", "", "gemini-3.8-flash-low"},
	} {
		t.Run(tc.provider+tc.effort, func(t *testing.T) {
			a := &effortChat{chatAdapter: chatAdapter{sessions: map[string][]string{}}}
			dir := t.TempDir()
			m, err := New(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return a, nil }, "gimbal", "instance", dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			item, err := m.Start(t.Context(), NewConversation{Provider: tc.provider, Model: tc.model, Effort: tc.effort}, "Hello")
			if err != nil {
				t.Fatal(err)
			}
			waitRequest(t, m, item.ID, item.Requests[0].ID, "completed")
			if a.model != tc.wantModel || a.effort != tc.effort {
				t.Fatalf("native model/effort = %q/%q", a.model, a.effort)
			}
			got, _ := m.Get(item.ID)
			if got.Effort != tc.effort {
				t.Fatalf("saved effort = %q", got.Effort)
			}
		})
	}
}

func TestUnsupportedEffortFailsBeforeAdmission(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	m, err := New(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { calls++; return nil, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, provider := range []string{"pi", "opencode"} {
		if _, err := m.Create(t.Context(), NewConversation{Provider: provider, Effort: "high"}); err == nil {
			t.Fatalf("%s accepted fabricated effort", provider)
		}
	}
	if calls != 0 || len(m.List()) != 0 {
		t.Fatal("unsupported effort admitted work")
	}
}

func TestFirstTurnFileContextUsesExactPrivateFileAndSurvivesFollowup(t *testing.T) {
	a := &controlledChat{started: make(chan string, 3), release: make(chan struct{}), landed: true}
	m, unused := managerWithControl(t, a)
	data := append(bytes.Repeat([]byte("long context\n"), 10000), []byte{0, 1, 2, 255}...)
	fileID := cacheFixture(t, m, data)
	item, err := m.Start(t.Context(), NewConversation{Provider: "codex", FileID: fileID}, "Discuss the file")
	if err != nil {
		t.Fatal(err)
	}
	prompt := <-a.started
	path, _, err := m.filePath(fileID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, path) || strings.Contains(prompt, "long context") {
		t.Fatal("file was not passed by absolute local path")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("context was truncated or changed")
	}
	close(a.release)
	waitRequest(t, m, item.ID, item.Requests[0].ID, "completed")
	if _, err = m.Submit(item.ID, Request{ID: "again", Created: 2, Kind: "prompt", Message: "Continue"}); err != nil {
		t.Fatal(err)
	}
	<-a.started
	waitRequest(t, m, item.ID, "again", "completed")
	if _, err = m.Submit(item.ID, Request{ID: "file-steer", Created: 3, Kind: "prompt", Message: "Again", FileID: fileID}); err == nil {
		t.Fatal("followup accepted first-turn file context")
	}
	actual, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("followup removed cached context")
	}
	// A bad first-turn context must be a definite failure, not uncertainty.
	os.Remove(path)
	if _, err = m.Submit(unused.ID, Request{ID: "missing", Created: 4, Kind: "prompt", Message: "Read", FileID: fileID}); err != nil {
		t.Fatal(err)
	}
	failed := waitRequest(t, m, unused.ID, "missing", "failed")
	if failed.Status == "unresolved" {
		t.Fatal("missing local bytes blocked the chat as uncertain")
	}
}

func TestRemoteOutgoingFileIsExactAndJournalRemainsByteFree(t *testing.T) {
	f := &remoteFixture{autoComplete: true}
	m, _, _ := remoteManager(t, f)
	data := append(bytes.Repeat([]byte("full PDF context\n"), 7000), []byte{0, 255}...)
	fileID := cacheFixture(t, m, data)
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true, FileID: fileID}, "Read this")
	if err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, item.Requests[0].ID, "completed")
	f.mu.Lock()
	turn := f.receivedTurns[0]
	f.mu.Unlock()
	if len(turn.Files) != 1 || !bytes.Equal(turn.Files[0].Bytes, data) {
		t.Fatal("remote task omitted or changed binary context")
	}
	m.mu.RLock()
	journal := m.remoteStates[item.ID].Requests[item.Requests[0].ID]
	m.mu.RUnlock()
	if len(journal.Turn.Files) != 0 {
		t.Fatal("journal contains file bytes")
	}
	raw, err := os.ReadFile(filepath.Join(m.projectDir, "conversations", item.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record conversationRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	var state remoteState
	if err = json.Unmarshal(record.Transport, &state); err != nil {
		t.Fatal(err)
	}
	for _, envelope := range state.Requests {
		if envelope.Turn != nil && len(envelope.Turn.Files) > 0 {
			t.Fatal("disk journal contains file bytes")
		}
	}
	if _, _, err = m.filePath(fileID); err != nil {
		t.Fatal("completed turn lost private context")
	}
}

func TestRenameChangesTitleWithoutChangingNativeSession(t *testing.T) {
	a := &controlledChat{started: make(chan string, 1), release: make(chan struct{})}
	m, item := managerWithControl(t, a)
	renamed, err := m.Rename(item.ID, "Renamed chat")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Title != "Renamed chat" || renamed.ID != item.ID || renamed.NativeSession != item.NativeSession || renamed.Revision <= item.Revision {
		t.Fatal("rename changed identity or failed to publish")
	}
	if _, err = m.Rename(item.ID, "  "); err == nil {
		t.Fatal("accepted blank title")
	}
}

func TestEffortChoicesRespectHarnessAndModel(t *testing.T) {
	for _, tc := range []struct {
		provider, model, level string
		available              bool
	}{
		{"pi", "anthropic/claude-opus-5-5", "high", false},
		{"opencode", "opencode/gpt-6-sol", "high", false},
		{"claude", "claude-haiku-4-5", "high", false},
		{"claude", "claude-sonnet-4-5", "high", false},
		{"claude", "claude-opus-4-5", "xhigh", false},
		{"claude", "claude-opus-4-6", "max", true},
		{"claude", "claude-opus-4-6", "xhigh", false},
		{"claude", "claude-opus-5-5", "xhigh", true},
		{"codex", "o3", "xhigh", false},
	} {
		if got := slices.Contains(modelEfforts(tc.provider, tc.model), tc.level); got != tc.available {
			t.Errorf("%s %s %s availability=%v", tc.provider, tc.model, tc.level, got)
		}
	}
}
