package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/conversation"
)

func TestConversationPageRendersProviderAndRealWorktreeFacts(t *testing.T) {
	repository := t.TempDir()
	gitForConversationPage(t, repository, "init", "-q")
	gitForConversationPage(t, repository, "config", "user.email", "gimble-test@example.invalid")
	gitForConversationPage(t, repository, "config", "user.name", "Gimble Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForConversationPage(t, repository, "add", "README.md")
	gitForConversationPage(t, repository, "commit", "-qm", "initial")

	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, repository, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-runtime.instance.done
	}()

	items := make([]conversation.Conversation, 0, 3)
	for _, provider := range []string{"codex", "claude", "agy"} {
		item, err := runtime.conversations.Create(ctx, conversation.NewConversation{
			Title: provider + " page", Provider: provider,
		})
		if err != nil {
			t.Fatalf("create %s: %v", provider, err)
		}
		items = append(items, item)
	}
	selected := items[2]
	response, err := http.Get("http://" + runtime.instance.address + "/projects/" + runtime.id + "/conversations/" + selected.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /conversations: status %d, body %s", response.StatusCode, body)
	}
	for _, want := range []string{
		"Conversations", "Choose a provider", "Codex", "Claude", "agy / Gemini",
		selected.Title, selected.Model, selected.Branch, selected.Worktree,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("conversation page does not contain %q", want)
		}
	}
	if got := strings.Count(string(body), "/projects/"+runtime.id+"/conversations/"); got != len(items) {
		t.Errorf("saved conversation links with document navigation = %d, want %d", got, len(items))
	}
}

func TestConversationPageRendersLinkedWorkflowStatusAndSavedContext(t *testing.T) {
	repository := t.TempDir()
	gitForConversationPage(t, repository, "init", "-q")
	gitForConversationPage(t, repository, "config", "user.email", "gimble-test@example.invalid")
	gitForConversationPage(t, repository, "config", "user.name", "Gimble Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForConversationPage(t, repository, "add", "README.md")
	gitForConversationPage(t, repository, "commit", "-qm", "initial")

	project := filepath.Join(repository, ".gimble")
	store := filepath.Join(project, "conversations")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	item := conversation.Conversation{
		ID: "saved-conversation", Title: "Saved launch", Provider: "codex", Model: "gpt-5.6-luna",
		Branch: "gimble/conversation-saved", Worktree: repository, Status: conversation.StatusIdle,
		Messages: []conversation.Message{{Role: "user", Text: "Keep this context", Created: 1}},
		Runs:     []conversation.Run{{ID: "01RUN.review", Workflow: "review", Status: conversation.RunStatusCompleted}},
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, item.ID+".json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	runtime, err := NewRuntime(ctx, repository, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-runtime.instance.done
	}()
	response, err := http.Get("http://" + runtime.instance.address + "/projects/" + runtime.id + "/conversations/" + item.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Workflow runs", "/projects/" + runtime.id + "/runs/01RUN.review", "completed", "Keep this context"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("conversation page does not contain %q", want)
		}
	}
}

func gitForConversationPage(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
