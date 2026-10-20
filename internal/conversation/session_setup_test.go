package conversation

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

type effortChat struct {
	chatAdapter
	model, effort string
}

func (a *effortChat) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	a.model, a.effort = model, effort
	return a.chatAdapter.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), model, effort, workdir)
}

func TestSessionKeepsExplicitEffortAndDefaultLeavesHarnessChoice(t *testing.T) {
	for _, tc := range []struct{ provider, model, effort, wantModel string }{
		{"codex", "gpt-6-luna", "", "gpt-6-luna"},
		{"codex", "gpt-6-luna", "high", "gpt-6-luna"},
		{"agy", "gemini-3.8-flash-low", "low", "gemini-3.8-flash-low"},
		{"agy", "gemini-3.8-flash-low", "", "gemini-3.8-flash-low"},
	} {
		t.Run(tc.provider+tc.effort, func(t *testing.T) {
			a := &effortChat{chatAdapter: chatAdapter{sessions: map[string][]string{}}}
			dir := t.TempDir()
			m, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return a, nil }, "gimbal", "instance", dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			item, err := m.Start(t.Context(), NewConversation{Provider: tc.provider, Model: tc.model, Effort: tc.effort}, "Hello")
			if err != nil {
				t.Fatal(err)
			}
			waitRequest(t, m, item.ID, item.Requests[0].ID, "completed")
			if item.Model != tc.wantModel || item.Effort != tc.effort {
				t.Fatalf("native model/effort = %q/%q", a.model, a.effort)
			}
			got, _ := m.Get(item.ID)
			if got.Effort != tc.effort {
				t.Fatalf("saved effort = %q", got.Effort)
			}
		})
	}
}

func TestUnsupportedEffortFailsBeforeAdmission(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	m, err := newMetadataManager(t.Context(), filepath.Join(dir, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { calls++; return nil, nil }, "gimbal", "instance", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, provider := range []string{"pi", "opencode"} {
		if _, err := m.Create(t.Context(), NewConversation{Provider: provider, Effort: "high"}); err == nil {
			t.Fatalf("%s accepted fabricated effort", provider)
		}
	}
	if calls != 0 || len(m.List()) != 0 {
		t.Fatal("unsupported effort admitted work")
	}
}

func TestRenameChangesTitleWithoutChangingNativeSession(t *testing.T) {
	a := &controlledChat{started: make(chan string, 1), release: make(chan struct{})}
	m, item := managerWithControl(t, a)
	renamed, err := m.Rename(item.ID, "Renamed chat")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Title != "Renamed chat" || renamed.ID != item.ID || renamed.NativeSession != item.NativeSession || renamed.Revision <= item.Revision {
		t.Fatal("rename changed identity or failed to publish")
	}
	if _, err = m.Rename(item.ID, "  "); err == nil {
		t.Fatal("accepted blank title")
	}
}
