package conversation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
)

func TestManagerRoutesAllProvidersAndKeepsConversationHistory(t *testing.T) {
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimble")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	adapters := map[string]*chatAdapter{
		"codex":  {sessions: make(map[string][]string)},
		"claude": {sessions: make(map[string][]string)},
		"agy":    {sessions: make(map[string][]string)},
	}
	var routed []string
	manager, err := New(ctx, project, func(provider string) (gimble.HarnessAdapter, error) {
		routed = append(routed, provider)
		return adapters[provider], nil
	})
	if err != nil {
		t.Fatal(err)
	}

	providers := []struct {
		name, model string
	}{
		{"codex", "gpt-5.6-luna"},
		{"claude", "claude-haiku-4-5-20251001"},
		{"agy", "gemini-3.8-flash-low"},
	}
	created := make([]Conversation, 0, len(providers))
	for _, provider := range providers {
		item, err := manager.Create(ctx, NewConversation{Provider: provider.name})
		if err != nil {
			t.Fatalf("create %s: %v", provider.name, err)
		}
		if item.Model != provider.model {
			t.Errorf("%s default model = %q, want %q", provider.name, item.Model, provider.model)
		}
		branch := git(t, item.Worktree, "branch", "--show-current")
		if branch != item.Branch {
			t.Errorf("%s worktree branch = %q, want %q", provider.name, branch, item.Branch)
		}
		canonicalWorktree, err := filepath.EvalSymlinks(item.Worktree)
		if err != nil {
			t.Fatal(err)
		}
		if root := git(t, item.Worktree, "rev-parse", "--show-toplevel"); root != canonicalWorktree {
			t.Errorf("%s worktree root = %q, want %q", provider.name, root, item.Worktree)
		}
		created = append(created, item)
	}
	if created[0].Branch == created[1].Branch || created[1].Branch == created[2].Branch || created[0].Worktree == created[2].Worktree {
		t.Fatal("provider conversations did not get distinct branches and worktrees")
	}

	for _, item := range created {
		if _, err := manager.Send(item.ID, "Remember cobalt for the next turn."); err != nil {
			t.Fatalf("send through %s: %v", item.Provider, err)
		}
	}
	continued, err := manager.Send(created[0].ID, "What word did I ask you to remember?")
	if err != nil {
		t.Fatal(err)
	}
	if got := continued.Messages[len(continued.Messages)-1].Text; got != "You asked me to remember cobalt." {
		t.Fatalf("second turn did not retain context: %q", got)
	}
	for _, provider := range []string{"codex", "claude", "agy"} {
		if adapters[provider].turns == 0 {
			t.Errorf("%s adapter did not receive a turn", provider)
		}
	}
	if got := strings.Join(routed, ","); got != "codex,claude,agy,codex,claude,agy" {
		t.Fatalf("provider routing = %q", got)
	}

	manager.Close()
	restarted, err := New(t.Context(), project, func(string) (gimble.HarnessAdapter, error) {
		t.Fatal("reading saved history should not create an adapter")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := restarted.Get(created[0].ID)
	if !ok {
		t.Fatal("conversation missing after manager restart")
	}
	if saved.Branch != created[0].Branch || saved.Worktree != created[0].Worktree || len(saved.Messages) != 4 {
		t.Fatalf("saved conversation lost association or transcript: %+v", saved)
	}
	if saved.Live {
		t.Fatal("restored conversation claims its prior native session is live")
	}
}

type chatAdapter struct {
	mu       sync.Mutex
	sessions map[string][]string
	turns    int
	next     int
}

func (a *chatAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next++
	id := "session-" + string(rune('0'+a.next))
	a.sessions[id] = nil
	return id, nil
}

func (a *chatAdapter) RunTurn(_ context.Context, session, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	history := a.sessions[session]
	a.sessions[session] = append(history, prompt)
	a.turns++
	answer := "I will remember cobalt."
	if strings.Contains(prompt, "What word") && len(history) > 0 {
		answer = "You asked me to remember cobalt."
	}
	output, _ := json.Marshal(answer)
	return gimble.TurnResult{Output: output}, nil
}

func (*chatAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*chatAdapter) Fork(context.Context, string) (string, error)        { return "", nil }
func (*chatAdapter) Close(context.Context, string) error                 { return nil }

func newRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	git(t, repository, "init", "-q")
	git(t, repository, "config", "user.email", "gimble-test@example.invalid")
	git(t, repository, "config", "user.name", "Gimble Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repository, "add", "README.md")
	git(t, repository, "commit", "-qm", "initial")
	return repository
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
