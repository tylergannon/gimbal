package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
)

func TestControlRecordWritesThroughTheHostManager(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dir := t.TempDir()
	_, project, err := newProject(ctx, dir, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := host.CanonicalProject(dir)
	discoveries, _ := filepath.Glob(filepath.Join(dir, ".gimbal", "control", "*.json"))
	if len(discoveries) != 1 {
		t.Fatalf("discoveries = %v", discoveries)
	}
	var discovery controlDiscovery
	raw, _ := os.ReadFile(discoveries[0])
	if err := json.Unmarshal(raw, &discovery); err != nil {
		t.Fatal(err)
	}
	client := controlClient(discovery.Socket)
	defer client.CloseIdleConnections()
	post := func(projectName string, body ControlRecord) (int, ControlRecorded) {
		t.Helper()
		encoded, _ := json.Marshal(body)
		request, _ := http.NewRequest(http.MethodPost, "http://control/control/conversations/record", bytes.NewReader(encoded))
		request.Header.Set("X-Gimbal-Project", projectName)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		var recorded ControlRecorded
		_ = json.NewDecoder(response.Body).Decode(&recorded)
		return response.StatusCode, recorded
	}
	source := "zoom-connect:host/call"
	status, created := post(canonical, ControlRecord{Source: source, Title: "Zoom call host/call"})
	if status != http.StatusOK || created.Conversation == "" || created.Cursor != 0 {
		t.Fatalf("create = %d %+v", status, created)
	}
	// A repeated create, as after a lost reply, finds the same conversation.
	if _, again := post(canonical, ControlRecord{Source: source, Title: "Zoom call host/call"}); again.Conversation != created.Conversation {
		t.Fatalf("second create made %q, want %q", again.Conversation, created.Conversation)
	}
	entries := []conversation.Entry{
		{Seq: 1, Message: conversation.Message{Role: "zoom", Text: "Ana: one", Created: 10}},
		{Seq: 2, Message: conversation.Message{Role: "zoom", Text: "Ana: two", Created: 20}},
	}
	if _, first := post(canonical, ControlRecord{Conversation: created.Conversation, Source: source, Entries: entries}); first.Appended != 2 || first.Cursor != 2 {
		t.Fatalf("first write = %+v", first)
	}
	// The same write again, as a client does when the reply was lost.
	if _, replay := post(canonical, ControlRecord{Conversation: created.Conversation, Source: source, Entries: entries}); replay.Appended != 0 || replay.Cursor != 2 {
		t.Fatalf("replayed write = %+v", replay)
	}
	// The host's own manager, not a copy, holds the messages.
	held, ok := project.Conversations().Get(created.Conversation)
	if !ok || len(held.Messages) != 2 || held.Messages[1].Text != "Ana: two" || held.Messages[1].Created != 20 {
		t.Fatalf("host manager = %+v", held)
	}
	if status, _ := post(canonical, ControlRecord{Conversation: "missing", Source: source}); status != http.StatusNotFound {
		t.Fatalf("missing conversation status = %d", status)
	}
	if status, _ := post(filepath.Join(dir, "elsewhere"), ControlRecord{Source: source}); status != http.StatusNotFound {
		t.Fatalf("unadmitted project status = %d", status)
	}
	if status, _ := post(canonical, ControlRecord{}); status != http.StatusBadRequest {
		t.Fatalf("blank source status = %d", status)
	}
}
