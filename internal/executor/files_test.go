package executor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcceptedOperationWithoutOwnerOrResultPanics(t *testing.T) {
	dir := t.TempDir()
	if err := Accept(dir, map[string]string{"command": "sh"}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		p := recover()
		if p == nil || !strings.Contains(p.(string), "neither a live owner nor a terminal result") {
			t.Fatalf("panic = %v", p)
		}
	}()
	AssertOwner(dir)
}
func TestConflictRejectedWhileOwnerLivesAndAfterCompletion(t *testing.T) {
	dir := t.TempDir()
	request := map[string]string{"prompt": "first"}
	lease, err := Claim(dir, request)
	if err != nil {
		t.Fatal(err)
	}
	if err = Accept(dir, request); err != nil {
		t.Fatal(err)
	}
	if other, err := Claim(dir, request); err != nil || other != nil {
		t.Fatalf("duplicate: %v, %v", other, err)
	}
	if _, err = Claim(dir, map[string]string{"prompt": "different"}); err == nil {
		t.Fatal("conflict accepted")
	}
	if err = Write(filepath.Join(dir, "result.json"), map[string]bool{"done": true}); err != nil {
		t.Fatal(err)
	}
	lease.Close()
	if _, err = Claim(dir, map[string]string{"prompt": "different"}); err == nil {
		t.Fatal("completed conflict accepted")
	}
	if _, err = Claim(dir, request); err != nil {
		t.Fatal(err)
	}
}
func TestWaitChecksFileAfterSubscribing(t *testing.T) {
	dir := t.TempDir()
	lease, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err = Accept(dir, struct{}{}); err != nil {
		t.Fatal(err)
	}
	server, err := Listen(dir, func(Control) Reply { return Reply{} })
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	// A change before the subscription must return immediately from size/result.
	if err = Write(filepath.Join(dir, "result.json"), struct{}{}); err != nil {
		t.Fatal(err)
	}
	server.Notify()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if _, err = Call(ctx, dir, Control{Op: "wait"}); err != nil {
		t.Fatal(err)
	}
}
func TestVacuumRetainsActiveAndSessionState(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	old := now.Add(-8 * 24 * time.Hour)
	names := map[string]string{"expired": string(NewCommandID()), "active": string(NewTurnID()), "recent": string(NewCommandID()), "session": "sessions/kept"}
	for label, name := range names {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := Write(filepath.Join(dir, "result.json"), json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
		if label != "recent" {
			if err := os.Chtimes(filepath.Join(dir, "result.json"), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	lease, err := Lock(filepath.Join(root, names["active"]))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	submissions := filepath.Join(root, "submissions")
	if err = os.MkdirAll(submissions, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(submissions, names["expired"]+".json")
	if err = Write(manifest, names["expired"]); err != nil {
		t.Fatal(err)
	}
	if err = Vacuum(root, now); err != nil {
		t.Fatal(err)
	}
	if Exists(filepath.Join(root, names["expired"])) || Exists(manifest) {
		t.Fatal("expired operation or submission retained")
	}
	for _, label := range []string{"active", "recent", "session"} {
		if !Exists(filepath.Join(root, names[label], "result.json")) {
			t.Fatalf("removed %s", label)
		}
	}
}

func TestIdentityPrefixes(t *testing.T) {
	for id, prefix := range map[string]string{string(NewCommandID()): "command_", string(NewSessionID()): "session_", string(NewTurnID()): "turn_"} {
		if err := Validate(id, prefix); err != nil {
			t.Fatal(err)
		}
		if Validate("../"+id, prefix) == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestWriteReplacesSavedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	for _, value := range []string{"accepted", "completed"} {
		if err := Write(path, value); err != nil {
			t.Fatal(err)
		}
		var got string
		if err := Read(path, &got); err != nil || got != value {
			t.Fatalf("state=%q err=%v", got, err)
		}
	}
}
