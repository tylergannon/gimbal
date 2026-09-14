package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestartRestoresSnapshotAndOnlyItsDeltaSuffix(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(nil, "run-1", "fixture", dir)
	if err != nil {
		t.Fatal(err)
	}
	fold(t, store, scopeBegan("", "."), sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
	at := Placement{Session: "s1", Turn: "t1"}
	if err := store.Event(at, stepStarted("first", "msg-1", "m"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Event(at, json.RawMessage(`{"id":"text","type":"session.text.started","created":2,"data":{"sessionID":"ses_a","assistantMessageID":"msg-1","ordinal":0}}`), nil); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	if err := store.saveSnapshotLocked(); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	cut := store.position
	offset := store.journalEnd
	store.mu.Unlock()
	if err := store.Event(at, json.RawMessage(`{"id":"second","type":"session.text.delta","created":3,"data":{"sessionID":"ses_a","assistantMessageID":"msg-1","ordinal":0,"delta":"after cut"}}`), nil); err != nil {
		t.Fatal(err)
	}
	want := store.Snapshot()

	restored, ok, err := loadDurable(nil, "run-1", dir)
	if err != nil || !ok {
		t.Fatalf("restore: ok=%v err=%v", ok, err)
	}
	got := restored.Snapshot()
	if got.Position != want.Position || got.Position != cut+1 {
		t.Fatalf("position = %d, want %d", got.Position, want.Position)
	}
	if got.Stream != want.Stream {
		t.Fatalf("stream changed across restart: %q != %q", got.Stream, want.Stream)
	}
	transcript, err := json.Marshal(got.Transcripts["t1"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(transcript), "after cut") {
		t.Fatalf("suffix text missing from restored transcript: %s", transcript)
	}
	info, err := os.Stat(filepath.Join(dir, deltaFile))
	if err != nil {
		t.Fatal(err)
	}
	if offset <= 0 || offset >= info.Size() {
		t.Fatalf("snapshot offset %d does not divide %d-byte journal", offset, info.Size())
	}
}

func TestInterruptedDeltaTailDoesNotAdvanceRecoveredCursor(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(nil, "run-1", "fixture", dir)
	if err != nil {
		t.Fatal(err)
	}
	fold(t, store, sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
	store.mu.Lock()
	if err := store.saveSnapshotLocked(); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	want := store.position
	store.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(dir, deltaFile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"stream":"interrupted"`); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	restored, ok, err := loadDurable(nil, "run-1", dir)
	if err != nil || !ok {
		t.Fatalf("restore: %v", err)
	}
	if got := restored.Snapshot().Position; got != want {
		t.Fatalf("cursor advanced to %d, want %d", got, want)
	}
}

func TestJoinResumesRetainedSuffixOrReplacesFromSnapshot(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"))
	base := store.Snapshot()
	fold(t, store, turnStarted("", "s1", "t1"))
	snapshot, suffix, sub, err := store.Join(base.Stream, base.Position)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if snapshot != nil || len(suffix) != 1 || suffix[0].Position != base.Position+1 {
		t.Fatalf("resume = snapshot:%v suffix:%+v", snapshot != nil, suffix)
	}
	replacement, suffix, stale, err := store.Join("another-process", base.Position)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Close()
	if replacement == nil || len(suffix) != 0 || replacement.Position != store.Snapshot().Position {
		t.Fatalf("stale cursor did not get exact replacement")
	}
}
