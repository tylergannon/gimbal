package zoomconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/conversation"
)

// fakeServer serves the real Zoom Connect stream contract: bearer auth, ?from=
// replay, "id/event/data" frames, ": keepalive" comments, and "event: end".
type fakeServer struct {
	mu     sync.Mutex
	events []Event
	// dropAfter closes the first connection after this many frames; zero never drops.
	dropAfter int
	// ignoreFrom replays every event regardless of ?from=, as a lazy server might.
	ignoreFrom bool
	opens      int
	froms      []int64
}

func (f *fakeServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/calls/call1/stream" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		f.mu.Lock()
		f.opens++
		first := f.opens == 1
		f.froms = append(f.froms, from)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, ": connected\n\n")
		sent := 0
		for _, event := range f.events {
			if event.Seq <= from && !f.ignoreFrom {
				continue
			}
			if first && f.dropAfter > 0 && sent == f.dropAfter {
				return
			}
			raw, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Kind, raw)
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			sent++
		}
		_, _ = fmt.Fprint(w, "event: end\ndata: {}\n\n")
	})
}

func meeting() []Event {
	ana, bo := &Person{ID: "p1", Name: "Ana"}, &Person{ID: "p2", Name: "Bo"}
	return []Event{
		{Seq: 1, TS: 1000, Kind: "stream.started", Data: map[string]any{"source": "zoom"}},
		{Seq: 2, TS: 2000, Kind: "participant.joined", Participant: ana, Data: map[string]any{}},
		{Seq: 3, TS: 3000, Kind: "speaker.active", Participant: ana, Data: map[string]any{}},
		{Seq: 4, TS: 4000, Recv: 4001, Kind: "transcript", Participant: ana, Data: map[string]any{"text": "Ship it on Friday.", "final": true, "source": "zoom", "zoom_timestamp": 1760550003987}},
		{Seq: 5, TS: 5000, Kind: "transcript", Participant: bo, Data: map[string]any{"text": "Ship it on Fri", "final": false}},
		{Seq: 6, TS: 6000, Recv: 6001, Kind: "chat", Participant: bo, Data: map[string]any{"text": "Friday works."}},
		{Seq: 7, TS: 7000, Kind: "stream.stopped", Data: map[string]any{"reason": "meeting ended"}},
	}
}

func openManager(t *testing.T, project string) *conversation.Manager {
	t.Helper()
	manager, err := conversation.New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", project, "project", nil)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func texts(item conversation.Conversation) []string {
	var out []string
	for _, message := range item.Messages {
		first, _, _ := strings.Cut(message.Text, "\n")
		out = append(out, first)
	}
	return out
}

func want(t *testing.T, item conversation.Conversation, expected ...string) {
	t.Helper()
	got := texts(item)
	if fmt.Sprint(got) != fmt.Sprint(expected) {
		t.Fatalf("messages = %q, want %q", got, expected)
	}
}

var wantMeeting = []string{
	"[stream.started]",
	"Ana joined",
	"Ana: Ship it on Friday.",
	"Bo (chat): Friday works.",
	"[stream.stopped] meeting ended",
}

func TestRecordPersistsMeetingAndResumesAfterRestart(t *testing.T) {
	fake := &fakeServer{events: meeting()}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()
	project := filepath.Join(t.TempDir(), ".gimbal")
	manager := openManager(t, project)
	item, err := manager.Create(t.Context(), conversation.NewConversation{Provider: "claude", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{BaseURL: server.URL, Token: "key", CallID: "call1", Retry: time.Millisecond}
	if err := Record(t.Context(), cfg, manager, item.ID, nil); err != nil {
		t.Fatal(err)
	}
	saved, _ := manager.Get(item.ID)
	want(t, saved, wantMeeting...)
	address := strings.TrimPrefix(server.URL, "http://") + "/call1"
	spoken := saved.Messages[2]
	if spoken.Role != "zoom" || spoken.Created != 4000 || spoken.Text != "Ana: Ship it on Friday.\n("+address+"#4 transcript ts=4000 recv=4001 zoom_timestamp=1760550003987 participant=p1)" {
		t.Fatalf("speaker line lost its provenance: %+v", spoken)
	}
	if cursor, _ := manager.Cursor(item.ID, cfg.Source()); cursor != 7 || !strings.HasPrefix(cfg.Source(), "zoom-connect:127.0.0.1:") {
		t.Fatalf("cursor %d under source %q", cursor, cfg.Source())
	}
	manager.Close()

	// A restarted recorder reopens the saved conversation and resumes after seq 7.
	restarted := openManager(t, project)
	defer restarted.Close()
	reopened, ok := restarted.Get(item.ID)
	if !ok {
		t.Fatal("conversation not saved")
	}
	want(t, reopened, wantMeeting...)
	if err := Record(t.Context(), cfg, restarted, item.ID, nil); err != nil {
		t.Fatal(err)
	}
	again, _ := restarted.Get(item.ID)
	want(t, again, wantMeeting...)
	if last := fake.froms[len(fake.froms)-1]; last != 7 {
		t.Fatalf("restart resumed from %d, want 7", last)
	}
}

func TestRecordReconnectsAfterDropWithoutDuplicates(t *testing.T) {
	// The server drops after three frames and then replays everything, ignoring ?from.
	fake := &fakeServer{events: meeting(), dropAfter: 3, ignoreFrom: true}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()
	manager := openManager(t, filepath.Join(t.TempDir(), ".gimbal"))
	defer manager.Close()
	item, err := manager.Create(t.Context(), conversation.NewConversation{Provider: "claude", SharedCheckout: true})
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{BaseURL: server.URL, Token: "key", CallID: "call1", Retry: time.Millisecond}
	if err := Record(t.Context(), cfg, manager, item.ID, nil); err != nil {
		t.Fatal(err)
	}
	saved, _ := manager.Get(item.ID)
	want(t, saved, wantMeeting...)
	if fake.opens != 2 || fake.froms[1] != 2 {
		t.Fatalf("opens = %d, froms = %v; want a second connection resuming from 2", fake.opens, fake.froms)
	}
}

func TestRecordStopsOnRejectedKey(t *testing.T) {
	server := httptest.NewServer((&fakeServer{events: meeting()}).handler(t))
	defer server.Close()
	manager := openManager(t, filepath.Join(t.TempDir(), ".gimbal"))
	defer manager.Close()
	item, _ := manager.Create(t.Context(), conversation.NewConversation{Provider: "claude", SharedCheckout: true})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := Record(ctx, Config{BaseURL: server.URL, Token: "wrong", CallID: "call1", Retry: time.Millisecond}, manager, item.ID, nil)
	if fatal, ok := err.(*FatalError); !ok || fatal.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, want 401 FatalError", err)
	}
	if err := Record(ctx, Config{BaseURL: server.URL}, manager, item.ID, nil); err == nil {
		t.Fatal("recorded without a call ID")
	}
	if err := Record(ctx, Config{BaseURL: server.URL, CallID: "call1"}, manager, "missing", nil); err == nil {
		t.Fatal("recorded into a missing conversation")
	}
}

func TestEntryKeepsOnlyReadableEvents(t *testing.T) {
	cfg := Config{BaseURL: "https://zc.example.com/", CallID: "call1"}
	kept := 0
	for _, event := range meeting() {
		if _, ok := Entry(cfg, event); ok {
			kept++
		}
	}
	if kept != 5 {
		t.Fatalf("kept %d of %d events, want 5 (speaker.active and the interim transcript are dropped)", kept, len(meeting()))
	}
}

func TestEntryInventsNothingForMissingSpeakerOrTime(t *testing.T) {
	cfg := Config{BaseURL: "https://zc.example.com/", CallID: "call1"}
	entry, ok := Entry(cfg, Event{Seq: 9, Kind: "transcript", Data: map[string]any{"text": "no speaker"}})
	if !ok || entry.Text != "no speaker\n(zc.example.com/call1#9 transcript)" || entry.Created != 0 {
		t.Fatalf("anonymous transcript = %+v, %v", entry, ok)
	}
	chat, _ := Entry(cfg, Event{Seq: 10, TS: 5, Kind: "chat", Participant: &Person{Email: "a@example.com"}, Data: map[string]any{"text": "hi", "zoom_timestamp": nil}})
	if chat.Text != "(chat): hi\n(zc.example.com/call1#10 chat ts=5 email=a@example.com)" || chat.Created != 5 {
		t.Fatalf("chat = %+v", chat)
	}
}

// lossyStore applies each write and then loses the reply, once in three.
type lossyStore struct {
	*conversation.Manager
	writes int
}

func (s *lossyStore) Record(id, source string, entries []conversation.Entry) (int, error) {
	n, err := s.Manager.Record(id, source, entries)
	s.writes++
	if err == nil && s.writes%3 == 0 {
		return 0, ErrUnavailable
	}
	return n, err
}

func TestRecordRetriesLostReplyWithoutDuplicatesOrSecondConversation(t *testing.T) {
	fake := &fakeServer{events: meeting()}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()
	manager := openManager(t, filepath.Join(t.TempDir(), ".gimbal"))
	defer manager.Close()
	cfg := Config{BaseURL: server.URL, Token: "key", CallID: "call1", Retry: time.Millisecond}
	if err := Record(t.Context(), cfg, &lossyStore{Manager: manager}, "", nil); err != nil {
		t.Fatal(err)
	}
	items := manager.List()
	if len(items) != 1 {
		t.Fatalf("conversations = %d, want 1", len(items))
	}
	want(t, items[0], wantMeeting...)
	if fake.opens < 2 {
		t.Fatalf("opens = %d; a lost reply should have forced a reconnect", fake.opens)
	}
}
