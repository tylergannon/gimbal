package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

// TestNativeConversationDetachResumeLive uses the host's ordinary conversation manager and workflow.
func TestNativeConversationDetachResumeLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	owner := New(ctx, t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("luna resolves to %s", model.Model)
	token := "cobalt-" + string(runstore.NewRunID())
	manager := project.Conversations()
	chat, err := manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, SharedCheckout: true}, "Remember this token: "+token+". Confirm briefly, and wait for my next message.")
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		snapshot, ok := manager.Get(chat.ID)
		if ok && !snapshot.Live {
			return
		}
		if _, err := manager.Submit(chat.ID, conversation.Request{ID: "cleanup-finish", Kind: "finish"}); err != nil {
			t.Errorf("finish test conversation: %v", err)
			return
		}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			snapshot, ok := manager.Get(chat.ID)
			if ok && !snapshot.Live {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Error("test conversation did not finish during cleanup")
	}()
	wait := func(description string, ready func() bool) {
		t.Helper()
		for !ready() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", description, ctx.Err())
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	var first conversation.Conversation
	wait("first answer and idle", func() bool {
		var ok bool
		first, ok = manager.Get(chat.ID)
		return ok && first.Status == "waiting" && len(first.Messages) == 2
	})
	if first.Messages[0].Role != "user" || first.Messages[1].Role != "assistant" || strings.TrimSpace(first.Messages[1].Text) == "" {
		t.Fatalf("first messages: %+v", first.Messages)
	}
	bindingID := func() (string, map[string]string) {
		t.Helper()
		file, err := os.Open(filepath.Join(project.Dir(), "runs", chat.WorkflowRun, "sessions", "agentic-dialogues.1.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		var id string
		nativeTurns := map[string]string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var record gimbal.AgentRecord
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			var ref struct {
				Provider  string
				SessionID string
			}
			if len(record.NativeRef) == 0 {
				continue
			}
			if err := json.Unmarshal(record.NativeRef, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.Provider == "codex" && ref.SessionID != "" {
				if previous := nativeTurns[record.Turn]; previous != "" && previous != ref.SessionID {
					t.Fatalf("turn %s changed native session: %s -> %s", record.Turn, previous, ref.SessionID)
				}
				nativeTurns[record.Turn] = ref.SessionID
			}
			if ref.Provider != "gimbal" || ref.SessionID == "" {
				continue
			}
			if id != "" && id != ref.SessionID {
				t.Fatalf("multiple executor sessions: %s, %s", id, ref.SessionID)
			}
			id = ref.SessionID
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		if err := executor.Validate(id, "session_"); err != nil {
			t.Fatalf("observed executor session: %v", err)
		}
		return id, nativeTurns
	}
	binding, firstNativeTurns := bindingID()
	root, err := executor.Root("")
	if err != nil {
		t.Fatal(err)
	}
	sessionDir := filepath.Join(root, "sessions", binding)
	nativeID := func() string {
		t.Helper()
		var saved struct{ ID string }
		if err := executor.Read(filepath.Join(sessionDir, "native.json"), &saved); err != nil || saved.ID == "" {
			t.Fatalf("native checkpoint: %q %v", saved.ID, err)
		}
		return saved.ID
	}
	originalNative := nativeID()
	if len(firstNativeTurns) != 1 {
		t.Fatalf("first native turns: %v", firstNativeTurns)
	}
	for _, id := range firstNativeTurns {
		if id != originalNative {
			t.Fatalf("first native observation %s differs from checkpoint %s", id, originalNative)
		}
	}
	wait("first worker release", func() bool {
		lease, err := executor.Lock(sessionDir)
		if err != nil {
			t.Fatal(err)
		}
		if lease == nil {
			return false
		}
		lease.Close()
		return true
	})
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "recall", Kind: "prompt", Message: "What token did I give you? Echo only the token."}); err != nil {
		t.Fatal(err)
	}
	var second conversation.Conversation
	wait("second answer and idle", func() bool {
		var ok bool
		second, ok = manager.Get(chat.ID)
		return ok && second.Status == "waiting" && len(second.Messages) == 4
	})
	if second.Messages[1].Text != first.Messages[1].Text || !strings.Contains(second.Messages[3].Text, token) {
		t.Fatalf("resumed messages: %+v", second.Messages)
	}
	nextBinding, nativeTurns := bindingID()
	if nextBinding != binding {
		t.Fatalf("executor session changed: %s -> %s", binding, nextBinding)
	}
	if len(nativeTurns) != 2 {
		t.Fatalf("native turns: %v", nativeTurns)
	}
	for _, id := range nativeTurns {
		if id != originalNative {
			t.Fatalf("native turn session %s differs from checkpoint %s", id, originalNative)
		}
	}
	if next := nativeID(); next != originalNative {
		t.Fatalf("native session changed: %s -> %s", originalNative, next)
	}
	requests, err := filepath.Glob(filepath.Join(sessionDir, "submissions", "*.json"))
	if err != nil || len(requests) != 2 {
		t.Fatalf("turn submissions=%v: %v", requests, err)
	}
	remember, recall := 0, 0
	for _, path := range requests {
		var request struct{ Prompt string }
		if err := executor.Read(path, &request); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(request.Prompt, token) {
			remember++
		}
		if strings.Contains(request.Prompt, "What token did I give you?") {
			recall++
			if strings.Contains(request.Prompt, token) {
				t.Fatal("recall prompt replayed the token")
			}
		}
	}
	if remember != 1 || recall != 1 {
		t.Fatalf("turn prompts: remember=%d recall=%d", remember, recall)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("conversation completion", func() bool {
		snapshot, ok := manager.Get(chat.ID)
		return ok && snapshot.Status == "completed" && !snapshot.Live
	})
	finished = true
	// The completed workflow releases its writer lease. Corroborate the live
	// observation with the authoritative native-binding record afterward.
	var records runstore.Run
	wait("journal writer release", func() bool {
		var err error
		records, err = runstate.ForProject(project.Dir()).Open(ctx, runstore.RunID(chat.WorkflowRun))
		if errors.Is(err, runstore.ErrBusy) {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		return true
	})
	defer func() { _ = records.Close() }()
	rows, err := records.Read(ctx, 0, 4096)
	if err != nil {
		t.Fatal(err)
	}
	var journalBinding string
	for _, row := range rows {
		if row.Kind != "native-binding" {
			continue
		}
		var value struct{ Native string }
		if err := json.Unmarshal(row.Payload, &value); err != nil {
			t.Fatal(err)
		}
		if journalBinding != "" && journalBinding != value.Native {
			t.Fatalf("multiple journal bindings: %s, %s", journalBinding, value.Native)
		}
		journalBinding = value.Native
	}
	if journalBinding != binding {
		t.Fatalf("observed executor session %s differs from journal binding %s", binding, journalBinding)
	}
	t.Logf("conversation=%s run=%s executor session=%s native session=%s completed after detach and recall", chat.ID, chat.WorkflowRun, binding, originalNative)
}
