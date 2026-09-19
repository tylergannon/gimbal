package web

import (
	"context"
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
	runtime, err := NewRuntime(ctx, filepath.Join(repository, ".gimble"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-runtime.done
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
	response, err := http.Get("http://" + runtime.address + "/conversations?conversation=" + selected.ID)
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
	if got := strings.Count(string(body), "data-sveltekit-reload"); got != len(items) {
		t.Errorf("saved conversation links with document navigation = %d, want %d", got, len(items))
	}
}

func gitForConversationPage(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
