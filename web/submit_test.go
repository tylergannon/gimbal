package web

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/conversation"
	"github.com/tylergannon/gimble/internal/observation"
)

func TestHostedSubmissionOwnsRunBeyondClient(t *testing.T) {
	base := t.TempDir()
	projectA, projectB := filepath.Join(base, "a"), filepath.Join(base, "b")
	workdir := filepath.Join(base, "execution")
	for _, dir := range []string{projectA, projectB, workdir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	instanceDir := filepath.Join(base, "instance")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	entry := func(ctx context.Context, env gimble.Env, raw json.RawMessage) error {
		var params struct{ Choice string }
		if err := json.Unmarshal(raw, &params); err != nil {
			return err
		}
		gimble.Set(ctx, "choice", params.Choice)
		gimble.Set(ctx, "execution directory", env.WorkDir)
		close(started)
		select {
		case <-release:
			return errors.New("requested failure")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb(), WithWorkflows(map[string]WorkflowEntry{"fixture": entry}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := instance.Owner.AdmitProject(projectA); err != nil {
		t.Fatal(err)
	}
	projectRuntime, err := instance.Owner.AdmitProject(projectB)
	if err != nil {
		t.Fatal(err)
	}
	clientCtx, stopClient := context.WithCancel(t.Context())
	admitted, err := Submit(clientCtx, instanceDir, projectB, Submission{Name: "fixture", Params: json.RawMessage(`{"Choice":"kept"}`), WorkDir: workdir})
	if err != nil {
		t.Fatal(err)
	}
	stopClient()
	<-started
	if _, err := os.Stat(filepath.Join(projectB, ".gimble", "runs", admitted.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectA, ".gimble", "runs", admitted.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("run appeared under wrong project: %v", err)
	}
	close(release)
	followCtx, stopFollow := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopFollow()
	row, err := Follow(followCtx, instanceDir, projectB, admitted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" || !strings.Contains(row.Error, "requested failure") {
		t.Fatalf("terminal row: %+v", row)
	}
	snapshot, err := projectRuntime.Registry().Snapshot(admitted.ID)
	if err != nil {
		t.Fatal(err)
	}
	values := snapshot.Scopes[""].Values
	if string(values["choice"].Value) != `"kept"` || string(values["execution directory"].Value) != `"`+workdir+`"` {
		t.Fatalf("submitted choices: %+v", values)
	}
	if _, err := Follow(followCtx, instanceDir, projectA, admitted.ID); err == nil {
		t.Fatal("wrong project followed run")
	}
	if _, err := Submit(t.Context(), filepath.Join(base, "missing"), projectB, Submission{}); err == nil || !strings.Contains(err.Error(), "start gimble") {
		t.Fatalf("unavailable instance: %v", err)
	}
}

func TestConversationSubmissionPersistsProjectRunAndTerminalFailure(t *testing.T) {
	base := t.TempDir()
	project, worktree := filepath.Join(base, "owner"), filepath.Join(base, "conversation-worktree")
	for _, dir := range []string{project, worktree, filepath.Join(project, ".gimble", "conversations")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item := conversation.Conversation{ID: "chat-1", Worktree: worktree, Messages: []conversation.Message{}, Runs: []conversation.Run{}}
	encoded, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(project, ".gimble", "conversations", "chat-1.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instanceDir := filepath.Join(base, "instance")
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb(), WithWorkflows(map[string]WorkflowEntry{
		"fixture": func(ctx context.Context, env gimble.Env, _ json.RawMessage) error {
			gimble.Set(ctx, "workdir", env.WorkDir)
			return errors.New("fixture failed")
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := instance.Owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Submit(t.Context(), instanceDir, project, Submission{Name: "fixture", WorkDir: project, Conversation: item.ID}); err == nil {
		t.Fatal("accepted conversation run in owning project instead of worktree")
	}
	admitted, err := Submit(t.Context(), instanceDir, project, Submission{Name: "fixture", WorkDir: worktree, Conversation: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		stored, _ := runtime.Conversations().Get(item.ID)
		if len(stored.Runs) == 1 && stored.Runs[0].Status == conversation.RunStatusError {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal association: %+v", stored.Runs)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(project, ".gimble", "runs", admitted.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, ".gimble", "runs", admitted.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree became run owner: %v", err)
	}
	snapshot, err := runtime.Registry().Snapshot(admitted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Scopes[""].Values["workdir"].Value); got != `"`+worktree+`"` {
		t.Fatalf("execution workdir = %s", got)
	}
	reopened, err := conversation.New(t.Context(), filepath.Join(project, ".gimble"), func(string) (gimble.HarnessAdapter, error) { return nil, errors.New("no provider needed") }, "", instanceDir, project, observation.NewRegistry(filepath.Join(project, ".gimble")))
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := reopened.Get(item.ID)
	if !ok || len(saved.Runs) != 1 || saved.Runs[0].ID != admitted.ID || saved.Runs[0].Status != conversation.RunStatusError || !strings.Contains(saved.Runs[0].Error, "fixture failed") {
		t.Fatalf("reopened conversation: %+v", saved)
	}
}
