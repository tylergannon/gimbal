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
	"github.com/tylergannon/gimbal/internal/runstate"
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

func TestFirstTurnFileContextUsesExactPrivateFileAndSurvivesFollowup(t *testing.T) {
	a := &controlledChat{started: make(chan string, 3), release: make(chan struct{}), landed: true}
	m, unused := managerWithControl(t, a)
	original := m.WorkflowInput
	m.WorkflowInput = func(run, address, raw string) error {
		var input runstate.InteractionInput
		_ = json.Unmarshal([]byte(raw), &input)
		a.started <- input.Message
		return original(run, address, raw)
	}

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
	if _, err = m.Submit(unused.ID, Request{ID: "missing", Created: 4, Kind: "prompt", Message: "Read", FileID: fileID}); err == nil {
		t.Fatal("missing local bytes blocked the chat as uncertain")
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

func TestPrivateContextSurvivesNativeSessionResumption(t *testing.T) {
	dir := t.TempDir()
	adapter := &persistentChatAdapter{sessions: map[string][]string{}}
	factory := func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }
	first, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), factory, "/bin/gimbal", "/instance", dir, nil)
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
	resumed, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), factory, "/bin/gimbal", "/instance", dir, nil)
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
