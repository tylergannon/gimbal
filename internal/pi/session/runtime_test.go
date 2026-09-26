package session

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestPersistenceReopen(t *testing.T) {
	cwd := t.TempDir()
	sessionDir := filepath.Join(cwd, "sessions")
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	manager, err := history.Create(cwd, sessionDir, nil)
	if err != nil {
		t.Fatalf("create persisted session: %v", err)
	}

	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingLow},
		StreamFn:     doneStream(testAssistant("done", model.StopStop)),
	})
	session, err := New(Config{
		Agent:          builtAgent,
		SessionManager: manager,
		Settings:       settings,
		Cwd:            cwd,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := session.Prompt(context.Background(), "persist me", nil); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	sessionFile := session.SessionFile()
	session.Dispose()

	reopened, err := history.Open(sessionFile, sessionDir, "")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	result, err := CreateFromServices(context.Background(), FromServicesOptions{
		Services:          &Services{Cwd: cwd, Settings: settings},
		SessionManager:    reopened,
		GetModel:          func(provider, modelID string) *model.Model { return testModel() },
		HasConfiguredAuth: func(provider string) bool { return true },
		StreamFn:          doneStream(testAssistant("done", model.StopStop)),
	})
	if err != nil {
		t.Fatalf("create from reopened services: %v", err)
	}
	t.Cleanup(result.Session.Dispose)

	if result.Session.Model() == nil {
		t.Fatalf("model not restored")
	}
	if result.ModelFallbackMessage != "" {
		t.Fatalf("unexpected fallback: %q", result.ModelFallbackMessage)
	}
	foundUser := false
	for _, message := range result.Session.Messages() {
		if text := messageText(message); text == "persist me" {
			foundUser = true
		}
	}
	if !foundUser {
		t.Fatalf("persisted user message not restored")
	}
}

func TestCreateFromServicesSelectsModelFromSession(t *testing.T) {
	cwd := t.TempDir()
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory manager: %v", err)
	}
	if _, err := manager.AppendModelChange("openai", "mock"); err != nil {
		t.Fatalf("append model change: %v", err)
	}
	if _, err := manager.AppendMessage(model.NewUserText("hello", 0)); err != nil {
		t.Fatalf("append message: %v", err)
	}

	result, err := CreateFromServices(context.Background(), FromServicesOptions{
		Services:          &Services{Cwd: cwd, Settings: settings},
		SessionManager:    manager,
		GetModel:          func(provider, modelID string) *model.Model { return testModel() },
		HasConfiguredAuth: func(provider string) bool { return true },
		StreamFn:          doneStream(testAssistant("done", model.StopStop)),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(result.Session.Dispose)
	if result.Session.Model() == nil || result.Session.Model().ID != "mock" {
		t.Fatalf("model = %v, want mock", result.Session.Model())
	}
}
