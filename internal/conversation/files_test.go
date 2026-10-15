package conversation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
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

func TestMissingFilePreventsConversationAdmission(t *testing.T) {
	a := &controlledChat{started: make(chan string, 1), release: make(chan struct{})}
	m, _ := managerWithControl(t, a)
	before := len(m.List())
	id := cacheFixture(t, m, []byte("private context"))
	path, _, _ := m.filePath(id)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(t.Context(), NewConversation{Provider: "claude", FileID: id}, "Read this"); err == nil {
		t.Fatal("missing file admitted a conversation")
	}
	if len(m.List()) != before {
		t.Fatal("file failure left an admitted conversation")
	}
	if err := m.CacheFile(t.Context(), "invalid"); err == nil {
		t.Fatal("invalid identity retrieved a file")
	}
	select {
	case <-a.started:
		t.Fatal("file failure started agent work")
	default:
	}
}

func TestMissingRemoteFileFailsBeforeNativeAdmission(t *testing.T) {
	f := &remoteFixture{autoComplete: true}
	m, _, _ := remoteManager(t, f)
	item, err := m.Create(t.Context(), NewConversation{Provider: "claude", Environment: "remote-project", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	id := cacheFixture(t, m, []byte("private remote context"))
	path, _, _ := m.filePath(id)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Submit(item.ID, Request{ID: "missing-file", Created: 1, Kind: "prompt", Message: "Read", FileID: id}); err != nil {
		t.Fatal(err)
	}
	waitRequest(t, m, item.ID, "missing-file", "failed")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions != 0 || f.admissions != 0 {
		t.Fatal("missing file started remote agent work")
	}
}

func TestPrivateContextSurvivesNativeSessionResumption(t *testing.T) {
	dir := t.TempDir()
	adapter := &persistentChatAdapter{sessions: map[string][]string{}}
	factory := func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }
	first, err := New(t.Context(), filepath.Join(dir, ".gimbal"), factory, "/bin/gimbal", "/instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("context retained across native detach and resume")
	fileID := cacheFixture(t, first, data)
	item, err := first.Start(t.Context(), NewConversation{Provider: "claude", FileID: fileID}, "Read")
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	waitRequest(t, first, item.ID, item.Requests[0].ID, "completed")
	first.Close()
	resumed, err := New(t.Context(), filepath.Join(dir, ".gimbal"), factory, "/bin/gimbal", "/instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	if _, err := sendAndWait(t, resumed, item.ID, "Continue discussing the file"); err != nil {
		t.Fatal(err)
	}
	path, _, err := resumed.filePath(fileID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("resumption lost private context")
	}
}
