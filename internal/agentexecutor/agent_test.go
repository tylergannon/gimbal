package agentexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/executor"
)

func TestEventCatchUpToleratesReplayAndPartialTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	first := []byte("{\"sequence\":1,\"type\":\"message\",\"data\":{}}\n")
	second := []byte("{\"sequence\":2,\"type\":\"message\",\"data\":{}}\n")
	partial := append(append(append([]byte{}, first...), first...), second[:len(second)/2]...)
	if err := os.WriteFile(path, partial, 0600); err != nil {
		t.Fatal(err)
	}
	var offset, sequence int64
	var seen []int64
	apply := func(e Event) error { seen = append(seen, e.Sequence); return nil }
	if err := ReadEvents(path, &offset, &sequence, apply); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || offset != int64(2*len(first)) {
		t.Fatalf("%v %d", seen, offset)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(second[len(second)/2:])
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err = ReadEvents(path, &offset, &sequence, apply); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[1] != 2 {
		t.Fatal(seen)
	}
}
func TestTerminalPublicationDrainsEventsAndPreservesInvalidAnswer(t *testing.T) {
	dir := t.TempDir()
	d := executor.Describe(dir, true)
	event := Event{Sequence: 1, AgentEvent: gimbal.AgentEvent{Type: "message", Data: json.RawMessage(`{}`)}, Native: json.RawMessage(`{"session":"native"}`)}
	raw, _ := json.Marshal(event)
	if err := os.WriteFile(d.Events, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if err := executor.Write(d.Result, Result{Output: []byte("{"), Error: "invalid", InvalidOutput: true, Completed: time.Now()}); err != nil {
		t.Fatal(err)
	}
	count := 0
	out, err := Wait(context.Background(), d, func(e gimbal.AgentEvent) error {
		count++
		if len(e.NativeRef) == 0 {
			t.Fatal("lost native identity")
		}
		return nil
	})
	if !errors.Is(err, gimbal.ErrInvalidOutput) || string(out.Output) != "{" || count != 1 {
		t.Fatalf("%q %v count %d", out.Output, err, count)
	}
}
func TestSessionConflictsAndValidation(t *testing.T) {
	root := t.TempDir()
	s := Session{ID: executor.NewSessionID(), Harness: "claude", Model: "haiku", Workdir: root}
	if err := Create(root, s); err != nil {
		t.Fatal(err)
	}
	if err := Create(root, s); err != nil {
		t.Fatal(err)
	}
	s.Model = "different"
	if err := Create(root, s); err == nil {
		t.Fatal("session conflict accepted")
	}
	s.ID = "../bad"
	if err := Create(root, s); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestCloseUnusedSessionIsIdempotentAndRejectsLaterTurns(t *testing.T) {
	root := t.TempDir()
	a := Adapter{root: root, harness: "claude"}
	id, err := a.CreateSession(t.Context(), "unused", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Close(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if !executor.Exists(filepath.Join(sessionDir(root, executor.SessionID(id)), "closed.json")) {
		t.Fatal("no closed marker")
	}
	_, err = Submit(root, Request{ID: executor.NewTurnID(), Session: executor.SessionID(id), Prompt: "must not run", Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed session accepted a turn: %v", err)
	}
}

func TestIdleSessionCanSteerAndCloseAfterTurnVacuum(t *testing.T) {
	root := t.TempDir()
	a := Adapter{root: root, harness: "claude"}
	id, err := a.CreateSession(t.Context(), "unused", "", root)
	if err != nil {
		t.Fatal(err)
	}
	turn := executor.NewTurnID()
	dir := filepath.Join(root, string(turn))
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = executor.Accept(dir, Request{ID: turn, Session: executor.SessionID(id)}); err != nil {
		t.Fatal(err)
	}
	if err = executor.Write(filepath.Join(dir, "result.json"), Result{Completed: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err = executor.Write(filepath.Join(sessionDir(root, executor.SessionID(id)), "active.json"), turn); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err = os.Chtimes(filepath.Join(dir, "result.json"), old, old); err != nil {
		t.Fatal(err)
	}
	if err = executor.Vacuum(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	if landed, err := a.Steer(t.Context(), id, "no turn"); err != nil || landed {
		t.Fatalf("steer=%v %v", landed, err)
	}
	executor.AssertOwner(dir)
	if err = a.Close(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if executor.Exists(dir) {
		t.Fatal("vacuumed turn directory was recreated")
	}
}
