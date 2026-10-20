package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/skgo/params"

	routes "github.com/tylergannon/gimbal/internal/skgo/links/onzggl3sn52xizlt"
)

func TestConversationPageRendersProviderAndRealWorktreeFacts(t *testing.T) {
	repository := t.TempDir()
	gitForConversationPage(t, repository, "init", "-q")
	gitForConversationPage(t, repository, "config", "user.email", "gimbal-test@example.invalid")
	gitForConversationPage(t, repository, "config", "user.name", "Gimbal Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForConversationPage(t, repository, "add", "README.md")
	gitForConversationPage(t, repository, "commit", "-qm", "initial")

	ctx, cancel := context.WithCancel(t.Context())
	instance, runtime, err := newProject(ctx, repository, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-instance.done
	}()

	items := make([]conversation.Conversation, 0, 3)
	for _, provider := range []string{"codex", "claude", "agy"} {
		item, err := runtime.Conversations().Create(ctx, conversation.NewConversation{
			Title: provider + " page", Provider: provider,
		})
		if err != nil {
			t.Fatalf("create %s: %v", provider, err)
		}
		items = append(items, item)
	}
	selected := items[2]
	response, err := http.Get("http://" + instance.address + "/projects/" + runtime.ID() + "/conversations/" + selected.ID)
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
		"Sessions", "New tab", "Antigravity", "Antigravity · Flash 3.8 Low",
		selected.Title, selected.Model, selected.Branch, selected.Worktree,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("conversation page does not contain %q", want)
		}
	}
	for _, item := range items {
		if !strings.Contains(string(body), item.Title) {
			t.Errorf("workspace inventory does not contain saved chat %q", item.Title)
		}
	}
	if !strings.Contains(string(body), "workspace-panel-/projects/"+runtime.ID()+"/conversations/"+selected.ID) {
		t.Error("workspace selected tab does not carry its canonical route identity")
	}
}

func TestConversationPageRendersLinkedWorkflowStatusAndSavedContext(t *testing.T) {
	repository := t.TempDir()
	gitForConversationPage(t, repository, "init", "-q")
	gitForConversationPage(t, repository, "config", "user.email", "gimbal-test@example.invalid")
	gitForConversationPage(t, repository, "config", "user.name", "Gimbal Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForConversationPage(t, repository, "add", "README.md")
	gitForConversationPage(t, repository, "commit", "-qm", "initial")

	project := filepath.Join(repository, ".gimbal")
	store := filepath.Join(project, "conversations")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	item := conversation.Conversation{
		ID: "saved-conversation", Title: "Saved launch", Provider: "codex", Model: "gpt-5.6-luna",
		Branch: "gimbal/conversation-saved", Worktree: repository, Status: conversation.StatusIdle,
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
	instance, runtime, err := newProject(ctx, repository, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-instance.done
	}()
	response, err := http.Get("http://" + instance.address + "/projects/" + runtime.ID() + "/conversations/" + item.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source turn unavailable", "/projects/" + runtime.ID() + "/runs/01RUN.review", "Ended", "Keep this context"} {
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

func TestNewSessionFormNeedsAFirstMessageAndOffersResolvedDefaults(t *testing.T) {
	repository := t.TempDir()
	gitForConversationPage(t, repository, "init", "-q", "-b", "main")
	gitForConversationPage(t, repository, "config", "user.email", "gimbal-test@example.invalid")
	gitForConversationPage(t, repository, "config", "user.name", "Gimbal Test")
	gitForConversationPage(t, repository, "commit", "-q", "--allow-empty", "-m", "initial")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, runtime, err := newProject(ctx, repository, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}

	_, err = routes.Skgo_createConversation(runtime.Context(), app.RequestEvent[params.Params]{}, routes.CreateConversation{Provider: "claude", Message: "  "})
	if err == nil || !strings.Contains(err.Error(), "Describe the work") {
		t.Fatalf("blank first message: %v", err)
	}
	if items := runtime.Conversations().List(); len(items) != 0 {
		t.Fatalf("a blank first message created %d conversations", len(items))
	}

	setup, err := routes.Skgo_conversationSetup(host.WithOwner(runtime.Context(), runtime.Owner()), routes.ConversationSetup{Project: runtime.ID()})
	if err != nil {
		t.Fatal(err)
	}
	if !setup.Git || setup.Branch != "main" || setup.Base != "main" || !slices.Contains(setup.Bases, "main") || setup.Directory == "" {
		t.Fatalf("setup environment = %+v", setup)
	}
	for _, choice := range []routes.CreateConversation{
		{Provider: "claude", Message: "Keep the draft", Branch: "main", Base: "main"},
		{Provider: "claude", Message: "Keep the draft", Branch: "new-branch", Base: "missing-base"},
	} {
		if _, err := routes.Skgo_createConversation(runtime.Context(), app.RequestEvent[params.Params]{}, choice); err == nil {
			t.Fatalf("invalid checkout admitted: %+v", choice)
		}
	}
	if len(runtime.Conversations().List()) != 0 {
		t.Fatal("invalid form choices admitted a conversation")
	}
	for _, harness := range setup.Harnesses {
		if harness.Default == "" || !slices.Contains(harness.Models, harness.Default) {
			t.Errorf("%s offers no resolved default model: %+v", harness.ID, harness)
		}
	}
	if len(setup.Harnesses) == 0 || setup.Harnesses[0].ID != "claude" || !strings.HasPrefix(setup.Harnesses[0].Default, "claude-haiku-") {
		t.Fatalf("first harness = %+v", setup.Harnesses)
	}
}
