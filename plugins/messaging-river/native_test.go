package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

// Opt in because this exercises the authenticated native Codex harness.
func TestNativeMessagingWorkflowInputsContinuityAndCleanup(t *testing.T) {
	if os.Getenv("GIMBAL_MESSAGING_NATIVE_TEST") != "1" {
		t.Skip("set GIMBAL_MESSAGING_NATIVE_TEST=1 for authenticated Codex parity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	tr, _ := integration(t)
	raw, _ := json.Marshal(tr.config)
	bus := backend.Messaging(tr)
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan error, 1)
	go func() {
		done <- serve(workerCtx, raw, backend.Dependencies{}, taskdaemon.New(t.TempDir()).Messages(bus))
	}()
	defer func() {
		stop()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("worker shutdown: %v", err)
		}
	}()
	token := "input-" + ulid.Make().String()
	project := t.TempDir()
	runID := ulid.Make().String()
	ctx = effects.WithConnection(gimbal.Project(ctx, project), backend.Connection{Messages: bus})
	h := &taskdaemon.Harness{Name: "codex", Connection: backend.Connection{Messages: bus}}
	err := gimbal.Run(ctx, runID, "native-messaging", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: h, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "agent", project)
		first, err := session.Generate[gimbal.Text](gimbal.WithTaskFiles(ctx, backend.TaskFile{Name: "unique", Bytes: []byte(token)}), "Read the supplied task file unique. Remember its entire content for later. Return exactly that content, with no quotes or explanation.")
		if err != nil {
			return err
		}
		if string(first) != token {
			t.Errorf("native input use: %q want %q", first, token)
		}
		second, err := session.Generate[gimbal.Text](ctx, "Return exactly the entire file content you remembered in the previous turn, with no quotes or explanation.")
		if err != nil {
			return err
		}
		if string(second) != token {
			t.Errorf("session continuity: %q want %q", second, token)
		}
		return gimbal.Service(ctx, "scoped-service", project, "printf ready; sleep 120")
	})
	if err != nil {
		t.Fatal(err)
	}
	stopObserved := errors.New("service completion observed")
	err = bus.ListenEvents(ctx, backend.MessageSelection{Run: runID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Kind == "completed" && e.Result != nil && e.Result.Process != nil {
			if !e.Result.Process.Interrupted {
				t.Errorf("service cessation not confirmed: %+v", e.Result.Process)
			}
			return stopObserved
		}
		return nil
	})
	if !errors.Is(err, stopObserved) {
		t.Fatal(err)
	}
	t.Log("gpt-5.6-luna: exact memory-only task bytes read, typed Text accepted twice on one native session, scoped shell service confirmed interrupted")
}

// This exercises the real selected transport and native session; no shared
// filesystem supplies the memory-only task file.
func TestNativeNamedSessionBytesContinuityAndCleanup(t *testing.T) {
	if os.Getenv("GIMBAL_MESSAGING_NATIVE_TEST") != "1" {
		t.Skip("set GIMBAL_MESSAGING_NATIVE_TEST=1 for authenticated native execution")
	}
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"selected": "native-session-worker"}
	tr.config.WorkerID = "native-session-worker"
	raw, _ := json.Marshal(tr.config)
	queues := []string{workerQueue(tr.config.Queue, "native-session-worker", false), workerQueue(tr.config.Queue, "native-session-worker", true)}
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues) })
	daemon := taskdaemon.New(t.TempDir())
	workerCtx, stop := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, daemon.Messages(tr))
	}()
	defer func() {
		stop()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("worker: %v", err)
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	model.Adapter = &taskdaemon.Harness{Name: "codex", Connection: backend.Connection{Messages: tr}}
	project := t.TempDir()
	runID := ulid.Make().String()
	token := "file-" + ulid.Make().String()
	contextToken := "context-" + ulid.Make().String()
	ctx = effects.WithConnection(gimbal.Project(ctx, project), backend.Connection{Messages: tr})
	err = gimbal.Run(ctx, runID, "named-native", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": model}, func(ctx context.Context) error {
		ctx = gimbal.WithEnvironment(ctx, "selected")
		gimbal.Set(ctx, "large-value", strings.Repeat("ordinary filler ", 10000)+contextToken+strings.Repeat(" ordinary filler", 10000))
		session := gimbal.NewSession(ctx, "agent", project)
		first, err := session.Generate[gimbal.Text](gimbal.WithTaskFiles(ctx, backend.TaskFile{Name: "unique", Bytes: []byte(token)}), "Read the supplied task file unique and return its entire content exactly, with no explanation.")
		if err != nil {
			return err
		}
		if string(first) != token {
			return fmt.Errorf("input mismatch: %q", first)
		}
		second, err := session.Generate[gimbal.Text](ctx, "Return the exact content of the file from the previous turn, with no explanation.")
		if err != nil {
			return err
		}
		if string(second) != token {
			return fmt.Errorf("continuity mismatch: %q", second)
		}
		third, err := session.Generate[gimbal.Text](ctx, "The context value large-value has a complete file. Read that complete file and return the single token starting with context- exactly, with no explanation.")
		if err != nil {
			return err
		}
		if string(third) != contextToken {
			return fmt.Errorf("complete context mismatch: %q", third)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("luna: named native session read memory-only task bytes, retained them for a second turn, read the large complete context value, and cleaned up")
}

func TestNativeHostConfiguredLocalNamespaceConversation(t *testing.T) {
	if os.Getenv("GIMBAL_MESSAGING_NATIVE_TEST") != "1" {
		t.Skip("set GIMBAL_MESSAGING_NATIVE_TEST=1 for authenticated native execution")
	}
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"local-notes": "conversation-worker"}
	tr.config.WorkerID = "conversation-worker"
	raw, _ := json.Marshal(tr.config)
	queues := []string{workerQueue(tr.config.Queue, "conversation-worker", false), workerQueue(tr.config.Queue, "conversation-worker", true)}
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues) })
	if _, ok := gimbal.DefaultRegistry.Messaging("river"); !ok {
		if err := Register(gimbal.DefaultRegistry); err != nil {
			t.Fatal(err)
		}
	}
	daemon := taskdaemon.New(t.TempDir())
	workerCtx, stop := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, daemon.Messages(tr))
	}()
	defer func() {
		stop()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("worker: %v", err)
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	instance, projectDir := t.TempDir(), t.TempDir()
	owner := host.New(ctx, instance)
	defer owner.Close()
	project, err := owner.AdmitProject(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	selectedDir := t.TempDir()
	route := backend.Route{Name: "river", Configuration: raw}
	config := environments.Config{Version: 1, Machines: []environments.Machine{{ID: "namespace", Name: "namespace", Kind: environments.KindRoute, Route: &route}}, Environments: []environments.Environment{{ID: "local-notes", Project: project.ID(), Machine: environments.Local, Directory: selectedDir}}}
	configRaw, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), configRaw, 0600); err != nil {
		t.Fatal(err)
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	manager := project.Conversations()
	token := "host-" + ulid.Make().String()
	chat, err := manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes", SharedCheckout: true}, "Remember this token: "+token+". Confirm briefly and wait for my next message.")
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_, _ = manager.Submit(chat.ID, conversation.Request{ID: "cleanup", Kind: "finish"})
		}
	}()
	wait := func(label string, predicate func(conversation.Conversation) bool) conversation.Conversation {
		t.Helper()
		for {
			item, ok := manager.Get(chat.ID)
			if ok && predicate(item) {
				return item
			}
			if ok && item.Status == "error" {
				t.Fatalf("%s: %s", label, item.Error)
			}
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v; last=%+v", label, ctx.Err(), item)
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	first := wait("first reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	if !first.Placement.Present || first.Placement.Value.Machine.ID != "conversation-worker" {
		t.Fatalf("conversation placement not projected: %+v", first.Placement)
	}
	snapshot, err := project.Registry().Snapshot(chat.WorkflowRun)
	if err != nil {
		t.Fatal(err)
	}
	session := snapshot.Sessions["agentic-dialogues.1"]
	if session.Environment != "local-notes" || session.WorkerID != "conversation-worker" {
		t.Fatalf("admitted placement not projected: %+v", session)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "recall", Kind: "prompt", Message: "Echo exactly the token I gave you, with no explanation."}); err != nil {
		t.Fatal(err)
	}
	second := wait("follow-up", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 4 })
	if !strings.Contains(second.Messages[len(second.Messages)-1].Text, token) {
		t.Fatalf("session did not retain token: %+v", second.Messages)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("finish and workflow cleanup", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	t.Log("luna: configured local directory launched through River, replied twice in one conversation, and finished")
}
