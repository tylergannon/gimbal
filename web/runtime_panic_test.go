package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/observation"
)

func TestHostedPanicsLeaveOtherRunsAndPageUsable(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	worktree := filepath.Join(base, "worktree")
	for _, dir := range []string{project, worktree, filepath.Join(project, ".gimble", "conversations")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	chat := conversation.Conversation{ID: "chat-1", Worktree: worktree, Messages: []conversation.Message{}, Runs: []conversation.Run{}}
	data, err := json.Marshal(chat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".gimble", "conversations", chat.ID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	busyEntered := make(chan struct{}, 1)
	entries := map[string]WorkflowEntry{
		"busy": func(ctx context.Context, _ gimble.Env, _ json.RawMessage) error {
			return gimble.Scope(ctx, "wait", func(ctx context.Context) error {
				busyEntered <- struct{}{}
				<-ctx.Done()
				return context.Cause(ctx)
			})
		},
		"root-panic": func(context.Context, gimble.Env, json.RawMessage) error {
			panic("root exploded")
		},
		"group-panic": func(ctx context.Context, _ gimble.Env, _ json.RawMessage) error {
			group := gimble.Group(ctx, "children")
			group.Go("child", func(context.Context) error { panic("child exploded") })
			return group.Wait()
		},
		"after": func(context.Context, gimble.Env, json.RawMessage) error { return nil },
	}
	instanceDir := filepath.Join(base, "instance")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := NewInstance(ctx, instanceDir, nil, WithPort(0), WithWorkflows(entries))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := instance.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	busy, err := Submit(t.Context(), instanceDir, project, Submission{Name: "busy", WorkDir: project})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-busyEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent run did not enter its scope")
	}

	failedIDs := map[string]string{}
	for _, tc := range []struct {
		name, panicText, workdir, conversation string
	}{
		{"root-panic", "root exploded", project, ""},
		{"group-panic", "child exploded", worktree, chat.ID},
	} {
		admitted, err := Submit(t.Context(), instanceDir, project, Submission{Name: tc.name, WorkDir: tc.workdir, Conversation: tc.conversation})
		if err != nil {
			t.Fatalf("submit %s: %v", tc.name, err)
		}
		followCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
		row, err := Follow(followCtx, instanceDir, project, admitted.ID)
		stop()
		if err != nil || row.Status != observation.StatusFailed || !strings.Contains(row.Error, tc.panicText) {
			t.Fatalf("follow %s: row %+v, error %v", tc.name, row, err)
		}
		failedIDs[tc.name] = admitted.ID
		page, err := http.Get("http://" + instance.address + "/projects/" + runtime.id + "/runs/" + admitted.ID)
		if err != nil {
			t.Fatal(err)
		}
		_ = page.Body.Close()
		if page.StatusCode != http.StatusOK {
			t.Fatalf("failed run page %s: HTTP %d", admitted.ID, page.StatusCode)
		}
	}

	busySnapshot, err := runtime.registry.Snapshot(busy.ID)
	if err != nil || busySnapshot.Run.Status != observation.StatusRunning {
		t.Fatalf("unrelated run: %+v, %v", busySnapshot.Run, err)
	}
	page, err := http.Get("http://" + instance.address + "/projects/" + runtime.id)
	if err != nil {
		t.Fatal(err)
	}
	_ = page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("project page: HTTP %d", page.StatusCode)
	}
	if err := runtime.KillScope(busy.ID, "wait.1", "test", "finished observing"); err != nil {
		t.Fatalf("control unrelated run: %v", err)
	}
	after, err := Submit(t.Context(), instanceDir, project, Submission{Name: "after", WorkDir: project})
	if err != nil {
		t.Fatalf("submit after panic: %v", err)
	}
	followCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	row, err := Follow(followCtx, instanceDir, project, after.ID)
	stop()
	if err != nil || row.Status != observation.StatusCompleted {
		t.Fatalf("run after panic: %+v, %v", row, err)
	}

	cancel()
	<-instance.done
	restartCtx, stopRestart := context.WithCancel(t.Context())
	defer stopRestart()
	restarted, err := NewInstance(restartCtx, instanceDir, nil, WithNoWeb(), WithWorkflows(entries))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := restarted.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range failedIDs {
		followCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
		row, err := Follow(followCtx, instanceDir, project, id)
		stop()
		if err != nil || row.Status != observation.StatusFailed {
			t.Fatalf("follow reopened %s: %+v, %v", name, row, err)
		}
	}
	saved, ok := reopened.conversations.Get(chat.ID)
	if !ok || len(saved.Runs) != 1 || saved.Runs[0].Status != conversation.RunStatusError || !strings.Contains(saved.Runs[0].Error, "child exploded") {
		t.Fatalf("reopened conversation run: %+v", saved.Runs)
	}
}

func TestDirectRunStillPanicsAfterRecordingFailure(t *testing.T) {
	project := t.TempDir()
	for _, tc := range []struct {
		name, panicText string
		body            func(context.Context) error
	}{
		{"direct-root", "root exploded", func(context.Context) error { panic("root exploded") }},
		{"direct-group", "child exploded", func(ctx context.Context) error {
			group := gimble.Group(ctx, "children")
			group.Go("child", func(context.Context) error { panic("child exploded") })
			return group.Wait()
		}},
	} {
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = gimble.Run(gimble.Project(t.Context(), project), tc.name, nil, tc.body)
		}()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), tc.panicText) {
			t.Fatalf("direct %s panic = %v", tc.name, recovered)
		}
		entries, err := os.ReadDir(filepath.Join(project, "runs"))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), "."+tc.name) {
				continue
			}
			snapshot, err := observation.NewRegistry(project).Snapshot(entry.Name())
			if err != nil || snapshot.Run.Status != observation.StatusFailed || !strings.Contains(snapshot.Run.Error, tc.panicText) {
				t.Fatalf("direct %s history: %+v, %v", tc.name, snapshot.Run, err)
			}
			found = true
		}
		if !found {
			t.Fatalf("missing direct %s run", tc.name)
		}
	}
}
