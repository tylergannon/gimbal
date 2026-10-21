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
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestManagerRecordsCLIAssociationBesideOrdinaryChat(t *testing.T) {
	repository := newRepository(t)
	project := filepath.Join(repository, ".gimbal")
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	manager, err := newMetadataManager(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repository, nil)
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
	replied, err := sendAndWait(t, manager, item.ID, "Remember cobalt")
	if err != nil {
		t.Fatal(err)
	}
	if len(replied.Runs) != 1 || replied.Runs[0].ID != "run-1" || replied.Messages[len(replied.Messages)-1].Text != "I will remember cobalt." {
		t.Fatalf("conversation: %+v", replied)
	}
	if got := manager.Purpose(item); !strings.Contains(got, `--instance-dir "/instance"`) || !strings.Contains(got, `--project "`+repository+`"`) || !strings.Contains(got, `--conversation "`+item.ID+`"`) {
		t.Fatalf("CLI instructions: %s", got)
	}
	restarted, err := newMetadataManager(t.Context(), project, func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := restarted.Get(item.ID)
	if !ok || len(saved.Runs) != 1 || saved.Runs[0].ID != "run-1" {
		t.Fatalf("saved association: %+v", saved)
	}
	if saved.Updated != replied.Updated {
		t.Fatalf("reopening moved interaction time from %d to %d", replied.Updated, saved.Updated)
	}
}

func TestManagerStartTitlesByFirstMessageAndRepliesInBackground(t *testing.T) {
	repository := newRepository(t)
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	manager, err := newMetadataManager(t.Context(), filepath.Join(repository, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.Start(t.Context(), NewConversation{Provider: "claude"}, "  "); err == nil {
		t.Fatal("started a conversation without a message")
	}
	started, err := manager.Start(t.Context(), NewConversation{Provider: "claude"}, "Remember cobalt\nand nothing else.")
	if err != nil {
		t.Fatal(err)
	}
	claudeDefault, err := modelalias.Resolve(modelalias.Selection{Name: "haiku"})
	if err != nil {
		t.Fatal(err)
	}
	if started.Title != "Remember cobalt" || started.Model != claudeDefault.Model {
		t.Fatalf("started = %+v", started)
	}
	if len(started.Requests) != 1 || started.Requests[0].Kind != "prompt" || started.Requests[0].Message != "Remember cobalt\nand nothing else." {
		t.Fatalf("first message not admitted: %+v", started.Requests)
	}
	first := waitRequest(t, manager, started.ID, started.Requests[0].ID, "completed")
	if len(first.Messages) != 2 || first.Messages[0].Role != "user" || first.Messages[0].Text != "Remember cobalt\nand nothing else." {
		t.Fatalf("first turn not recorded: %+v", first.Messages)
	}
	continued, err := sendAndWait(t, manager, started.ID, "What word did I ask you to remember?")
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, message := range continued.Messages {
		texts = append(texts, message.Role+": "+message.Text)
	}
	want := "user: Remember cobalt\nand nothing else.|assistant: I will remember cobalt.|user: What word did I ask you to remember?|assistant: You asked me to remember cobalt."
	if got := strings.Join(texts, "|"); got != want {
		t.Fatalf("transcript = %q", got)
	}
}

func TestTitleFromFirstMessage(t *testing.T) {
	for text, want := range map[string]string{
		"Fix the tab strip":      "Fix the tab strip",
		"  First line\nsecond  ": "First line",
		"Fold extra tabs into a more menu when the window is one thousand pixels wide": "Fold extra tabs into a more menu when the window is one…",
	} {
		if got := titleFrom(text); got != want {
			t.Errorf("titleFrom(%q) = %q, want %q", text, got, want)
		}
	}
}

type chatAdapter struct {
	mu       sync.Mutex
	sessions map[string][]string
	turns    int
	next     int
	replies  []conversationReply
	prompts  []string
	workdirs []string
}

func (a *chatAdapter) CreateSession(_ context.Context, _, _, workdir string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.workdirs = append(a.workdirs, workdir)
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
	output, _ := json.Marshal(reply.Message)
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

func TestConversationRefreshFollowsExternalRun(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "runs", "child")
	writer, err := observation.Open(nil, "child", "review", dir, func() error { return os.MkdirAll(dir, 0755) })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	if err := os.MkdirAll(filepath.Join(state, "conversations"), 0755); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{projectDir: state, registry: observation.NewRegistry(state)}
	item := &Conversation{ID: "chat", Runs: []Run{{ID: "child", Workflow: "review", Status: RunStatusRunning}}}
	manager.refresh(item)
	if item.Runs[0].Status != RunStatusRunning {
		t.Fatal("running child settled early")
	}
	if err := writer.Lifecycle([]byte(`{"seq":1,"time":"2026-10-10T10:00:00Z","scope":"","event":{"kind":"run_ended","name":"review"}}`)); err != nil {
		t.Fatal(err)
	}
	manager.refresh(item)
	if item.Revision == 0 {
		t.Fatal("run settlement did not advance the conversation stream")
	}
	if item.Runs[0].Status != RunStatusCompleted {
		t.Fatal("conversation did not catch up")
	}
	raw, err := os.ReadFile(filepath.Join(state, "conversations", "chat.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Conversation
	if err := json.Unmarshal(raw, &saved); err != nil || saved.Runs[0].Status != RunStatusCompleted {
		t.Fatalf("saved conversation: %s, %v", raw, err)
	}
}

func TestManagerNonGitConversationsShareProjectDirectory(t *testing.T) {
	directory := t.TempDir()
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	manager, err := newMetadataManager(t.Context(), filepath.Join(directory, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for range 2 {
		item, err := manager.Create(t.Context(), NewConversation{Provider: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		if item.Branch != "" || item.Worktree != directory {
			t.Fatalf("unexpected Git association: %+v", item)
		}
		if _, err := sendAndWait(t, manager, item.ID, "Remember cobalt."); err != nil {
			t.Fatal(err)
		}
		if err := manager.Associate(item.ID, "run-"+item.ID, "review", directory); err != nil {
			t.Fatal(err)
		}
		if err := manager.Associate(item.ID, "wrong-directory", "review", filepath.Dir(directory)); err == nil {
			t.Fatal("accepted a different directory")
		}
	}
	if len(manager.List()) != 2 {
		t.Fatal("conversations did not remain distinct")
	}
	for _, name := range []string{".git", ".gimbal/conversation-worktrees"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s: %v", name, err)
		}
	}
	manager.Close()
	restarted, err := newMetadataManager(t.Context(), filepath.Join(directory, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	for _, item := range restarted.List() {
		if item.Branch != "" || item.Worktree != directory || len(item.Messages) != 2 || len(item.Runs) != 1 {
			t.Fatalf("lost saved conversation: %+v", item)
		}
	}
}

func TestManagerChatsWithoutCommittedGitHistory(t *testing.T) {
	for _, location := range []string{"ordinary-folder", "unborn-repository", "folder-under-unborn-repository", "folder-under-committed-repository"} {
		t.Run(location, func(t *testing.T) {
			directory := t.TempDir()
			if location == "folder-under-committed-repository" {
				directory = newRepository(t)
			} else if location != "ordinary-folder" {
				git(t, directory, "init", "-q")
			}
			if strings.HasPrefix(location, "folder-under-") {
				directory = filepath.Join(directory, "project")
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			adapter := &chatAdapter{sessions: make(map[string][]string)}
			manager, err := newMetadataManager(t.Context(), filepath.Join(directory, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", directory, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			setup, err := manager.Setup(t.Context())
			if err != nil || setup.Git || setup.Branch != "" {
				t.Fatalf("setup = %+v, err = %v", setup, err)
			}
			item, err := manager.Start(t.Context(), NewConversation{Provider: "claude"}, "Remember cobalt.")
			if err != nil {
				t.Fatal(err)
			}
			if item.Branch != "" || item.Worktree != directory {
				t.Fatalf("unexpected workspace: %+v", item)
			}
			waitRequest(t, manager, item.ID, item.Requests[0].ID, "completed")
			item, err = sendAndWait(t, manager, item.ID, "What word did I ask you to remember?")
			if err != nil {
				t.Fatal(err)
			}
			if len(item.Messages) != 4 || item.Messages[3].Text != "You asked me to remember cobalt." {
				t.Fatalf("chat did not complete both turns: %+v", item.Messages)
			}
			if _, err := os.Stat(filepath.Join(directory, ".gimbal", "conversation-worktrees")); !os.IsNotExist(err) {
				t.Fatalf("unexpected worktree directory: %v", err)
			}
		})
	}
}

func sendAndWait(t *testing.T, m *Manager, id, text string) (Conversation, error) {
	t.Helper()
	requestID, _ := randomID()
	_, err := m.Submit(id, Request{ID: requestID, Created: time.Now().UnixMilli(), Kind: "prompt", Message: text})
	if err != nil {
		return Conversation{}, err
	}
	deadline := time.After(5 * time.Second)
	for {
		item, _ := m.Get(id)
		for _, r := range item.Requests {
			if r.ID == requestID && (r.Status == "completed" || r.Status == "failed") {
				if r.Error != "" {
					return item, errors.New(r.Error)
				}
				return item, nil
			}
		}
		select {
		case <-deadline:
			t.Fatal("request did not settle")
		case <-time.After(time.Millisecond):
		}
	}
}

type conversationReply struct {
	Message string `json:"message"`
}

func TestListRefreshesWorkflowFailureAndKeepsUndeliveredInputQueued(t *testing.T) {
	repository := newRepository(t)
	manager, err := newMetadataManager(t.Context(), filepath.Join(repository, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		return &chatAdapter{sessions: make(map[string][]string)}, nil
	}, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	item, err := manager.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	stored := manager.items[item.ID]
	stored.WorkflowRun = "run"
	stored.Requests = []RequestState{{Request: request("queued", "hello")}}
	stored.Requests[0].Status = "accepted"
	manager.mu.Unlock()
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "waiting", nil, nil
	}
	waiting := manager.List()[0]
	if waiting.Status != "waiting" || waiting.Requests[0].Status != "accepted" {
		t.Fatalf("initial wait lost queued input: %+v", waiting)
	}
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "error", nil, errors.New("native schema rejected")
	}
	failed := manager.List()[0]
	if waiting.Updated != item.Updated || failed.Updated != waiting.Updated {
		t.Fatalf("observation changed interaction time: created=%d waiting=%d failed=%d", item.Updated, waiting.Updated, failed.Updated)
	}
	if failed.Status != "error" || failed.Error != "native schema rejected" || failed.Revision <= waiting.Revision || failed.Requests[0].Status != "failed" {
		t.Fatalf("list did not refresh failure: %+v", failed)
	}
}

func TestWorkflowControlSettlesOnlyAfterItsDurableConsumption(t *testing.T) {
	repository := newRepository(t)
	manager, err := newMetadataManager(t.Context(), filepath.Join(repository, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		return &chatAdapter{sessions: make(map[string][]string)}, nil
	}, "/bin/gimbal", "/instance", repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	item, err := manager.Create(t.Context(), NewConversation{Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	stored := manager.items[item.ID]
	stored.WorkflowRun = "run"
	stored.Requests = []RequestState{{Request: Request{ID: "stop", Kind: "interrupt"}, Status: "accepted"}, {Request: Request{ID: "another", Kind: "interrupt"}, Status: "accepted"}}
	manager.mu.Unlock()
	var consumed []string
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "waiting", consumed, nil
	}
	if got := manager.List()[0]; got.Requests[0].Status != "accepted" {
		t.Fatal("waiting alone settled control")
	}
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "unavailable", nil, errors.New("transient observation failure")
	}
	if got := manager.List()[0]; got.Contact != "unreachable" {
		t.Fatal("missing contact loss")
	}
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "working", nil, nil
	}
	if got := manager.List()[0]; got.Contact != "connected" || got.Status != "working" {
		t.Fatalf("contact did not recover: %+v", got)
	}
	manager.WorkflowObserve = func(string) (gimbal.InteractiveTranscript, string, []string, error) {
		return gimbal.InteractiveTranscript{}, "waiting", consumed, nil
	}
	consumed = []string{"stop"}
	got := manager.List()[0]
	if got.Requests[0].Status != "completed" || got.Requests[1].Status != "accepted" {
		t.Fatalf("control dispositions=%+v", got.Requests)
	}
}
