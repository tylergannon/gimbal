package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	"github.com/tylergannon/gimbal/internal/projectfiles"
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
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	err = gimbal.Run(ctx, runID, "native-messaging", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: h, Model: model.Model}}, func(ctx context.Context) error {
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
		return gimbal.Service(ctx, "scoped-service", project, `: > "$ENV_FILE"; sleep 120`)
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
	t.Log("luna: exact memory-only task bytes read, typed Text accepted twice on one native session, scoped shell service confirmed interrupted")
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
	tr.config.Environments = map[string]string{"local-notes": "conversation-worker", "task-remote": "conversation-worker"}
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
	cliDir := t.TempDir()
	cli := filepath.Join(cliDir, "gimbal")
	build := exec.CommandContext(ctx, "go", "build", "-o", cli, "./cmd/gimbal")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build desktop helper: %s: %v", output, err)
	}
	t.Setenv("PATH", cliDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	route := backend.Route{Name: "river", Configuration: raw}
	config := environments.Config{Version: 1, Machines: []environments.Machine{{ID: "namespace", Name: "namespace", Kind: environments.KindRoute, Route: &route, Executable: cli}}, Environments: []environments.Environment{{ID: "local-notes", Project: project.ID(), Machine: environments.Local, Directory: selectedDir}, {ID: "task-remote", Project: project.ID(), Machine: "namespace", Directory: selectedDir}}}
	configRaw, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(instance, "environments.json"), configRaw, 0600); err != nil {
		t.Fatal(err)
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	manager := project.Conversations()
	for _, environment := range []string{"local-notes", "task-remote"} {
		setup, err := manager.SetupEnvironment(ctx, environment)
		if err != nil {
			t.Fatalf("setup %s: %v", environment, err)
		}
		if setup.Environment != environment || len(setup.Harnesses) == 0 || setup.Directory == "" || !setup.Capabilities.Steer || setup.Capabilities.Fork {
			t.Fatalf("setup facts from worker: %+v", setup)
		}
		var count int
		if err := tr.pool.QueryRow(ctx, `SELECT count(*) FROM gimbal_message_owners WHERE namespace=$1 AND environment=$2 AND worker=$3`, tr.config.Queue, environment, "conversation-worker").Scan(&count); err != nil || count == 0 {
			t.Fatalf("setup %s did not reach named worker: %d %v", environment, count, err)
		}
	}
	token := "host-" + ulid.Make().String()
	fileID := "0123456789abcdef0123456789abcdef"
	fileName := "attachment.txt"
	state := filepath.Join(project.Path(), ".gimbal")
	if err := os.MkdirAll(filepath.Join(state, "files"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state, "conversation-files", fileID), 0700); err != nil {
		t.Fatal(err)
	}
	document, _ := json.Marshal(projectfiles.Document{ID: fileID, Name: fileName, Size: int64(len(token)), MediaType: "text/plain"})
	if err := os.WriteFile(filepath.Join(state, "files", fileID+".json"), document, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "conversation-files", fileID, fileName), []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	chat, err := manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes", SharedCheckout: true, FileID: fileID}, "Read the attached file, remember its exact content, confirm briefly and wait for my next message.")
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
	chat, err = manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "task-remote", SharedCheckout: true}, "Reply briefly, then wait.")
	if err != nil {
		t.Fatal(err)
	}
	finished = false
	remote := wait("remote setup-to-start reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	if remote.Environment != "task-remote" || remote.Placement.Value.Machine.ID != "conversation-worker" {
		t.Fatalf("remote placement: %+v", remote)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish-remote", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("remote setup-to-start finish", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	// The default choice on a plain directory runs there, even though a Git
	// checkout would receive an automatically generated branch.
	chat, err = manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes"}, "Reply briefly, then wait.")
	if err != nil {
		t.Fatal(err)
	}
	finished = false
	plain := wait("plain directory reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	actual, err := filepath.EvalSymlinks(plain.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := filepath.EvalSymlinks(selectedDir)
	if err != nil {
		t.Fatal(err)
	}
	if actual != selected || plain.Branch != "" || plain.Placement.Value.Machine.ID != "conversation-worker" {
		t.Fatalf("plain directory placement: %+v", plain)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish-plain", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("plain finish", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	bad, err := manager.Create(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes", Branch: "feature/explicit"})
	if err != nil {
		t.Fatal(err)
	}
	// The workflow may fail before the first input reaches Interactive.
	_, _ = manager.Submit(bad.ID, conversation.Request{ID: "invalid-branch", Kind: "prompt", Message: "This must not reach a session."})
	var badRun string
	for {
		item, ok := manager.Get(bad.ID)
		if ok && item.Status == "error" {
			if !strings.Contains(item.Error, "directory has no committed Git history") {
				t.Fatalf("explicit branch error: %+v", item)
			}
			badRun = item.WorkflowRun
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("explicit branch did not fail: %+v", item)
		case <-time.After(50 * time.Millisecond):
		}
	}
	badSnapshot, err := project.Registry().Snapshot(badRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(badSnapshot.Sessions) != 0 {
		t.Fatalf("invalid branch started a session: %+v", badSnapshot.Sessions)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = selectedDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	chat, err = manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes"}, "Reply briefly, then wait.")
	if err != nil {
		t.Fatal(err)
	}
	finished = false
	unborn := wait("unborn repository reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	unbornPath, err := filepath.EvalSymlinks(unborn.Worktree)
	if err != nil || unbornPath != selected {
		t.Fatalf("unborn repository placement: %+v %v", unborn, err)
	}
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish-unborn", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("unborn finish", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	git("-c", "user.name=Gimbal Test", "-c", "user.email=gimbal@example.invalid", "commit", "--allow-empty", "-qm", "initial")
	chat, err = manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes", Branch: "feature/namespace-checkout", Base: "HEAD"}, "Reply briefly, then wait.")
	if err != nil {
		t.Fatal(err)
	}
	finished = false
	prepared := wait("isolated checkout reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	if prepared.Worktree == selectedDir || prepared.Placement.Value.Directory != prepared.Worktree {
		t.Fatalf("checkout placement: %+v", prepared)
	}
	if got := git("-C", prepared.Worktree, "branch", "--show-current"); got != "feature/namespace-checkout" {
		t.Fatalf("prepared branch: %q", got)
	}
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", selectedDir, "worktree", "remove", "--force", prepared.Worktree).Run()
	})
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish-isolated", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("isolated finish", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	chat, err = manager.Start(ctx, conversation.NewConversation{Provider: "codex", Model: model.Model, Environment: "local-notes"}, "Reply briefly, then wait.")
	if err != nil {
		t.Fatal(err)
	}
	finished = false
	defaultGit := wait("default Git checkout reply", func(item conversation.Conversation) bool { return item.Status == "waiting" && len(item.Messages) >= 2 })
	if defaultGit.Branch != "gimbal/conversation-"+chat.ID[:12] {
		t.Fatalf("default branch not projected: %+v", defaultGit)
	}
	if got := git("-C", defaultGit.Worktree, "branch", "--show-current"); got != "gimbal/conversation-"+chat.ID[:12] {
		t.Fatalf("default Git branch: %q", got)
	}
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", selectedDir, "worktree", "remove", "--force", defaultGit.Worktree).Run()
	})
	if _, err := manager.Submit(chat.ID, conversation.Request{ID: "finish-default-git", Kind: "finish"}); err != nil {
		t.Fatal(err)
	}
	wait("default Git finish", func(item conversation.Conversation) bool { return !item.Live && item.Status == "completed" })
	finished = true
	t.Log("luna: shared, plain-directory, unborn, explicit Git, and default Git namespace conversations completed; invalid explicit branch failed before session")
}

func TestNativeNamedSessionControlsStayAtRetainedOwner(t *testing.T) {
	if os.Getenv("GIMBAL_MESSAGING_NATIVE_TEST") != "1" {
		t.Skip("set GIMBAL_MESSAGING_NATIVE_TEST=1 for authenticated native execution")
	}
	tr, ctx := integration(t)
	tr.config.Environments = map[string]string{"selected": "control-a"}
	queues := []string{}
	stops := []context.CancelFunc{}
	dones := []<-chan error{}
	for _, worker := range []string{"control-a", "control-b"} {
		cfg := tr.config
		cfg.WorkerID = worker
		raw, _ := json.Marshal(cfg)
		daemon := taskdaemon.New(t.TempDir())
		workerCtx, stop := context.WithCancel(ctx)
		ready := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- serve(workerCtx, raw, backend.Dependencies{Ready: func() error { close(ready); return nil }}, daemon.Messages(tr))
		}()
		select {
		case <-ready:
		case err := <-done:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		stops = append(stops, stop)
		dones = append(dones, done)
		queues = append(queues, workerQueue(tr.config.Queue, worker, false), workerQueue(tr.config.Queue, worker, true))
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
		for _, done := range dones {
			if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("worker: %v", err)
			}
		}
	}()
	t.Cleanup(func() { _, _ = tr.pool.Exec(context.Background(), "DELETE FROM river_job WHERE queue=ANY($1)", queues) })
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	h := &taskdaemon.Harness{Name: "codex", Connection: backend.Connection{Messages: tr}}
	base := effects.WithEnvironment(effects.WithConnection(ctx, backend.Connection{Messages: tr}), "selected")
	sessionOwner := backend.Owner{Run: "control-run", Task: "session"}
	sessionIdentity := backend.NewExecutionIdentity("session")
	sessionCtx := effects.WithExecutionIdentity(effects.WithOwner(base, sessionOwner), sessionIdentity)
	id, err := h.CreateSession(sessionCtx, model.Model, model.Effort, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr.config.Environments = map[string]string{"selected": "control-b"}
	tr.config.RebindFrom = map[string]string{"selected": "control-a"}
	probeOwner := backend.Owner{Run: "control-run", Task: "rebind-probe"}
	probeIdentity := backend.NewExecutionIdentity("process")
	probe := effects.Request(probeIdentity.RequestID, "identified-process", probeOwner)
	probe.ExecutionIdentity = probeIdentity
	probe.Process = &backend.Process{ExecutionIdentity: probeIdentity, Owner: probeOwner, Workdir: t.TempDir(), Command: "true"}
	result, err := effects.MessageCall(base, probe)
	if err != nil || result.WorkerID != "control-b" {
		t.Fatalf("new work did not rebind to control-b: %+v %v", result, err)
	}
	tr.config.RebindFrom = nil
	turnOwner := backend.Owner{Run: "control-run", Task: "turn"}
	turnIdentity := backend.NewExecutionIdentity("turn")
	turnCtx := effects.WithExecutionIdentity(effects.WithOwner(base, turnOwner), turnIdentity)
	turnDone := make(chan error, 1)
	activity := make(chan struct{}, 1)
	go func() {
		_, err := h.RunTurn(turnCtx, id, "Use a shell command to sleep for 20 seconds before answering.", nil, func(gimbal.AgentEvent) error {
			select {
			case activity <- struct{}{}:
			default:
			}
			return nil
		})
		turnDone <- err
	}()
	select {
	case <-activity:
	case err := <-turnDone:
		t.Fatalf("turn ended before control: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	steerID := ulid.Make().String()
	steer := effects.Request(steerID, "steer", turnOwner)
	steer.SessionID = "session"
	steer.Target = &backend.Target{Run: "control-run", ID: "session", Message: "Please stop the delay and finish."}
	event, err := effects.MessageCall(base, steer)
	if err != nil {
		t.Fatal(err)
	}
	if event.WorkerID != "control-a" || event.Environment != "selected" {
		t.Fatalf("steer moved owner: %+v", event)
	}
	interrupt := effects.Request(ulid.Make().String(), "interrupt", turnOwner)
	interrupt.Target = &backend.Target{Run: "control-run", ID: "turn"}
	event, err = effects.MessageCall(base, interrupt)
	if err != nil {
		t.Fatal(err)
	}
	if event.WorkerID != "control-a" {
		t.Fatalf("interrupt moved owner: %+v", event)
	}
	select {
	case <-turnDone:
	case <-ctx.Done():
		t.Fatal("interrupted turn did not settle")
	}
	closeIdentity := backend.NewExecutionIdentity("close")
	closeCtx := effects.WithExecutionIdentity(effects.WithOwner(base, sessionOwner), closeIdentity)
	if err := h.Close(closeCtx, id); err != nil {
		t.Fatal(err)
	}
	doneEvent := errors.New("close observed")
	var closed backend.MessageEvent
	err = tr.ListenEvents(ctx, backend.MessageSelection{RequestID: closeIdentity.RequestID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Kind == "completed" {
			closed = e
			return doneEvent
		}
		return nil
	})
	if !errors.Is(err, doneEvent) || closed.WorkerID != "control-a" {
		t.Fatalf("close moved owner: %+v %v", closed, err)
	}
	if err := effects.Cleanup(base, "", "control-run", "", "control-cleanup"); err != nil {
		t.Fatal(err)
	}
	t.Log("luna: steer, interrupt and identified close reached the original worker after rebinding")
}
