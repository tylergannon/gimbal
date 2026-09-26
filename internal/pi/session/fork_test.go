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

func TestForkPersistedSession(t *testing.T) {
	cwd := t.TempDir()
	sessionDir := filepath.Join(cwd, "sessions")
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	manager, err := history.Create(cwd, sessionDir, nil)
	if err != nil {
		t.Fatalf("create persisted session: %v", err)
	}
	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingOff},
		StreamFn:     doneStream(testAssistant("done", model.StopStop)),
	})
	parent, err := New(Config{Agent: builtAgent, SessionManager: manager, Settings: settings, Cwd: cwd})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(parent.Dispose)
	if err := parent.Prompt(context.Background(), "parent", nil); err != nil {
		t.Fatalf("parent prompt: %v", err)
	}
	parentMessages := len(parent.Messages())

	child, err := parent.Fork(context.Background())
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Cleanup(child.Dispose)
	if child.IsStreaming() {
		t.Fatalf("child should not be streaming")
	}
	if child.SessionFile() == parent.SessionFile() {
		t.Fatalf("child shares parent session file")
	}
	if len(child.Messages()) != parentMessages {
		t.Fatalf("child messages = %d, want %d", len(child.Messages()), parentMessages)
	}
	if err := child.Prompt(context.Background(), "child", nil); err != nil {
		t.Fatalf("child prompt: %v", err)
	}
	if len(parent.Messages()) != parentMessages {
		t.Fatalf("parent transcript changed by child: %d -> %d", parentMessages, len(parent.Messages()))
	}
}
