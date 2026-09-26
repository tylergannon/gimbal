package session

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
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

// TestForkChildRunsWithParentRequestConfig proves the child agent carries the
// parent's provider stream, API-key callback and request options rather than
// only the compaction stream.
func TestForkChildRunsWithParentRequestConfig(t *testing.T) {
	cwd := t.TempDir()
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory session manager: %v", err)
	}

	temperature := 0.25
	type request struct {
		apiKey      string
		temperature *float64
		metadata    map[string]any
	}
	var mu sync.Mutex
	var requests []request
	stream := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		requests = append(requests, request{apiKey: options.APIKey, temperature: options.Temperature, metadata: options.Metadata})
		mu.Unlock()
		eventStream := wire.NewAssistantMessageEventStream()
		eventStream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistant("reply", model.StopStop)})
		return eventStream, nil
	}

	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingOff},
		StreamFn:     stream,
		GetApiKey:    func(provider string) string { return "parent-key" },
		Temperature:  &temperature,
		Metadata:     map[string]any{"fork": "parent"},
	})
	parent, err := New(Config{Agent: builtAgent, SessionManager: manager, Settings: settings, Cwd: cwd})
	if err != nil {
		t.Fatalf("new parent: %v", err)
	}
	t.Cleanup(parent.Dispose)
	if err := parent.Prompt(context.Background(), "parent", nil); err != nil {
		t.Fatalf("parent prompt: %v", err)
	}

	child, err := parent.Fork(context.Background())
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Cleanup(child.Dispose)
	if err := child.Prompt(context.Background(), "child", nil); err != nil {
		t.Fatalf("child prompt: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("stream calls = %d, want 2", len(requests))
	}
	childRequest := requests[1]
	if childRequest.apiKey != "parent-key" {
		t.Fatalf("child API key = %q, want parent-key", childRequest.apiKey)
	}
	if childRequest.temperature == nil || *childRequest.temperature != temperature {
		t.Fatalf("child temperature = %v, want %v", childRequest.temperature, temperature)
	}
	if childRequest.metadata["fork"] != "parent" {
		t.Fatalf("child metadata = %v, want fork=parent", childRequest.metadata)
	}
}

// TestForkDropsUnfinishedToolBatch proves the child snapshot excludes an
// assistant tool batch whose results are incomplete, including results that
// already arrived, in both runtime and history.
func TestForkDropsUnfinishedToolBatch(t *testing.T) {
	cwd := t.TempDir()
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory session manager: %v", err)
	}
	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingOff},
		StreamFn:     doneStream(testAssistant("done", model.StopStop)),
	})
	parent, err := New(Config{Agent: builtAgent, SessionManager: manager, Settings: settings, Cwd: cwd})
	if err != nil {
		t.Fatalf("new parent: %v", err)
	}
	t.Cleanup(parent.Dispose)

	now := time.Now().UnixMilli()
	if _, err := manager.AppendMessage(model.UserMessage{Content: model.ContentList{model.TextContent{Text: "run both"}}, Timestamp: now}); err != nil {
		t.Fatalf("append user: %v", err)
	}
	assistant := model.AssistantMessage{
		Content: model.ContentList{
			model.TextContent{Text: "calling"},
			model.ToolCall{ID: "call-1", Name: "dummy", Arguments: map[string]any{}},
			model.ToolCall{ID: "call-2", Name: "dummy", Arguments: map[string]any{}},
		},
		Api: testModel().Api, Provider: testModel().Provider, Model: testModel().ID,
		StopReason: model.StopToolUse, Timestamp: now,
	}
	if _, err := manager.AppendMessage(assistant); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	if _, err := manager.AppendMessage(model.ToolResultMessage{ToolCallID: "call-1", ToolName: "dummy", Content: model.ContentList{model.TextContent{Text: "one"}}, Timestamp: now}); err != nil {
		t.Fatalf("append tool result: %v", err)
	}
	parent.RefreshContext()

	parentBefore := rolesOf(parent.Messages())
	unfinishedChild, err := parent.Fork(context.Background())
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Cleanup(unfinishedChild.Dispose)
	wantComplete := []model.Role{model.RoleSystem, model.RoleUser}
	if got := rolesOf(unfinishedChild.Messages()); !reflect.DeepEqual(got, wantComplete) {
		t.Fatalf("child runtime roles = %v, want %v", got, wantComplete)
	}
	if got := rolesOf(unfinishedChild.SessionManager().BuildSessionProjection().Messages); !reflect.DeepEqual(got, wantComplete) {
		t.Fatalf("child history roles = %v, want %v", got, wantComplete)
	}
	if got := rolesOf(parent.Messages()); !reflect.DeepEqual(got, parentBefore) {
		t.Fatalf("parent transcript changed by fork: %v -> %v", parentBefore, got)
	}

	// Once the second result lands the batch is complete and the child keeps it.
	if _, err := manager.AppendMessage(model.ToolResultMessage{ToolCallID: "call-2", ToolName: "dummy", Content: model.ContentList{model.TextContent{Text: "two"}}, Timestamp: now}); err != nil {
		t.Fatalf("append second tool result: %v", err)
	}
	parent.RefreshContext()
	completeChild, err := parent.Fork(context.Background())
	if err != nil {
		t.Fatalf("second fork: %v", err)
	}
	t.Cleanup(completeChild.Dispose)
	wantFinished := []model.Role{model.RoleSystem, model.RoleUser, model.RoleAssistant, model.RoleToolResult, model.RoleToolResult}
	if got := rolesOf(completeChild.Messages()); !reflect.DeepEqual(got, wantFinished) {
		t.Fatalf("child roles after completion = %v, want %v", got, wantFinished)
	}
}

// TestForkPersistedChildKeepsUnfinishedBatchExcludedOnReopen proves the child
// session file is a snapshot of the complete branch: reopening it before the
// child's first message still excludes the unfinished tool batch, carries a
// fresh id, and records the parent session.
func TestForkPersistedChildKeepsUnfinishedBatchExcludedOnReopen(t *testing.T) {
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
		t.Fatalf("new parent: %v", err)
	}
	t.Cleanup(parent.Dispose)

	now := time.Now().UnixMilli()
	if _, err := manager.AppendMessage(model.UserMessage{Content: model.ContentList{model.TextContent{Text: "run both"}}, Timestamp: now}); err != nil {
		t.Fatalf("append user: %v", err)
	}
	assistant := model.AssistantMessage{
		Content: model.ContentList{
			model.TextContent{Text: "calling"},
			model.ToolCall{ID: "call-1", Name: "dummy", Arguments: map[string]any{}},
			model.ToolCall{ID: "call-2", Name: "dummy", Arguments: map[string]any{}},
		},
		Api: testModel().Api, Provider: testModel().Provider, Model: testModel().ID,
		StopReason: model.StopToolUse, Timestamp: now,
	}
	if _, err := manager.AppendMessage(assistant); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	if _, err := manager.AppendMessage(model.ToolResultMessage{ToolCallID: "call-1", ToolName: "dummy", Content: model.ContentList{model.TextContent{Text: "one"}}, Timestamp: now}); err != nil {
		t.Fatalf("append tool result: %v", err)
	}
	parent.RefreshContext()

	parentBefore := rolesOf(parent.Messages())
	child, err := parent.Fork(context.Background())
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Cleanup(child.Dispose)
	if child.SessionFile() == parent.SessionFile() {
		t.Fatalf("child shares parent session file")
	}

	reopened, err := history.Open(child.SessionFile(), filepath.Dir(child.SessionFile()), "")
	if err != nil {
		t.Fatalf("reopen child: %v", err)
	}
	want := []model.Role{model.RoleSystem, model.RoleUser}
	if got := rolesOf(reopened.BuildSessionProjection().Messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened child roles = %v, want %v", got, want)
	}
	header := reopened.GetHeader()
	if header == nil || header.ID == parent.SessionManager().GetSessionID() {
		t.Fatalf("reopened child id = %v, want a fresh id", header)
	}
	if header.ParentSession != parent.SessionFile() {
		t.Fatalf("reopened child parent = %q, want %q", header.ParentSession, parent.SessionFile())
	}
	if got := rolesOf(parent.Messages()); !reflect.DeepEqual(got, parentBefore) {
		t.Fatalf("parent transcript changed by fork: %v -> %v", parentBefore, got)
	}
}

func rolesOf(messages []model.AgentMessage) []model.Role {
	roles := make([]model.Role, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.MessageRole())
	}
	return roles
}
