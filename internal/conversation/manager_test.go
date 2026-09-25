package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
)

func TestManagerRoutesAllProvidersAndKeepsConversationHistory(t *testing.T) {
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimbal")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	adapters := map[string]*chatAdapter{
		"codex":  {sessions: make(map[string][]string)},
		"claude": {sessions: make(map[string][]string)},
		"agy":    {sessions: make(map[string][]string)},
	}
	var routed []string
	manager, err := New(ctx, project, func(provider string) (gimbal.HarnessAdapter, error) {
		routed = append(routed, provider)
		return adapters[provider], nil
	}, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}

	providers := []struct {
		name, modelPrefix string
	}{
		{"codex", "gpt-"},
		{"claude", "claude-"},
		{"agy", "gemini-"},
	}
	created := make([]Conversation, 0, len(providers))
	for _, provider := range providers {
		item, err := manager.Create(ctx, NewConversation{Provider: provider.name})
		if err != nil {
			t.Fatalf("create %s: %v", provider.name, err)
		}
		if !strings.HasPrefix(item.Model, provider.modelPrefix) {
			t.Errorf("%s default model = %q, want prefix %q", provider.name, item.Model, provider.modelPrefix)
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
	restarted, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) {
		t.Fatal("reading saved history should not create an adapter")
		return nil, nil
	}, "/bin/gimbal", "/instance", repository, nil)
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

func TestManagerResumesPersistentNativeSessionAfterRestart(t *testing.T) {
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimbal")
	adapter := &persistentChatAdapter{sessions: make(map[string][]string)}
	createdAdapters := 0
	factory := func(string) (gimbal.HarnessAdapter, error) {
		createdAdapters++
		return adapter, nil
	}

	first, err := New(t.Context(), project, factory, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := first.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	item, err = first.Send(item.ID, "Remember cobalt for the next turn.")
	if err != nil {
		t.Fatal(err)
	}
	if item.NativeSession == "" {
		t.Fatal("first turn did not persist the native session id")
	}
	nativeSession := item.NativeSession
	first.Close()
	if adapter.detached != nativeSession {
		t.Fatalf("detached session = %q, want %q", adapter.detached, nativeSession)
	}

	createdBeforeRestart := createdAdapters
	restarted, err := New(t.Context(), project, factory, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	if createdAdapters != createdBeforeRestart {
		t.Fatal("loading saved conversations contacted the provider")
	}
	saved, ok := restarted.Get(item.ID)
	if !ok || !saved.Live {
		t.Fatalf("saved persistent conversation is not sendable after restart: %+v", saved)
	}
	continued, err := restarted.Send(item.ID, "What word did I ask you to remember?")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.resumed != nativeSession {
		t.Fatalf("resumed session = %q, want %q", adapter.resumed, nativeSession)
	}
	if continued.NativeSession != nativeSession {
		t.Fatalf("continued native session = %q, want unchanged %q", continued.NativeSession, nativeSession)
	}
	if got := continued.Messages[len(continued.Messages)-1].Text; got != "You asked me to remember cobalt." {
		t.Fatalf("resumed turn lost native context: %q", got)
	}
}

func TestManagerRecordsCLIAssociationBesideOrdinaryChat(t *testing.T) {
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimbal")
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	manager, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Associate(item.ID, "run-1", "review", repository); err == nil {
		t.Fatal("accepted wrong worktree")
	}
	if err := manager.Associate(item.ID, "run-1", "review", item.Worktree); err != nil {
		t.Fatal(err)
	}
	replied, err := manager.Send(item.ID, "Remember cobalt")
	if err != nil {
		t.Fatal(err)
	}
	if len(replied.Runs) != 1 || replied.Runs[0].ID != "run-1" || replied.Messages[len(replied.Messages)-1].Text != "I will remember cobalt." {
		t.Fatalf("conversation: %+v", replied)
	}
	if got := adapter.prompts[0]; !strings.Contains(got, `--instance-dir "/instance"`) || !strings.Contains(got, `--project "`+repository+`"`) || !strings.Contains(got, `--conversation "`+item.ID+`"`) {
		t.Fatalf("CLI instructions: %s", got)
	}
	restarted, err := New(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := restarted.Get(item.ID)
	if !ok || len(saved.Runs) != 1 || saved.Runs[0].ID != "run-1" {
		t.Fatalf("saved association: %+v", saved)
	}
}

type chatAdapter struct {
	mu       sync.Mutex
	sessions map[string][]string
	turns    int
	next     int
	replies  []conversationReply
	prompts  []string
}

func (a *chatAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next++
	id := "session-" + string(rune('0'+a.next))
	a.sessions[id] = nil
	return id, nil
}

func (a *chatAdapter) RunTurn(_ context.Context, session, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	history := a.sessions[session]
	a.sessions[session] = append(history, prompt)
	a.prompts = append(a.prompts, prompt)
	a.turns++
	answer := "I will remember cobalt."
	if strings.Contains(prompt, "What word") && len(history) > 0 {
		answer = "You asked me to remember cobalt."
	}
	reply := conversationReply{Message: answer}
	if len(a.replies) > 0 {
		reply = a.replies[0]
		a.replies = a.replies[1:]
	}
	output, _ := json.Marshal(reply)
	return gimbal.TurnResult{Output: output}, nil
}

func (*chatAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*chatAdapter) Fork(context.Context, string) (string, error)        { return "", nil }
func (*chatAdapter) Close(context.Context, string) error                 { return nil }

type persistentChatAdapter struct {
	chatAdapter
	resumed  string
	detached string
}

func (a *persistentChatAdapter) ResumeSession(_ context.Context, session, _, _, _ string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessions[session]; !ok {
		return errors.New("unknown persistent session")
	}
	a.resumed = session
	return nil
}

func (a *persistentChatAdapter) DetachSession(_ context.Context, session string) error {
	a.detached = session
	return nil
}

func newRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	git(t, repository, "init", "-q")
	git(t, repository, "config", "user.email", "gimbal-test@example.invalid")
	git(t, repository, "config", "user.name", "Gimbal Test")
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
