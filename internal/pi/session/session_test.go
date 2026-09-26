package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func TestPromptConcurrentGuard(t *testing.T) {
	session := newTestSession(t, testSessionOptions{stream: blockingStream()})

	done := make(chan error, 1)
	go func() { done <- session.Prompt(context.Background(), "First message", nil) }()
	waitFor(t, session.IsStreaming)

	err := session.Prompt(context.Background(), "Second message", nil)
	if err == nil || !strings.Contains(err.Error(), "already processing") {
		t.Fatalf("second prompt error = %v, want already processing", err)
	}

	if err := session.Steer("Steering message"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	if got := session.PendingMessageCount(); got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}

	session.Abort()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("first prompt did not finish after abort")
	}
	if session.IsStreaming() {
		t.Fatalf("still streaming after abort")
	}
}

func TestFollowUpWhileStreaming(t *testing.T) {
	session := newTestSession(t, testSessionOptions{stream: blockingStream()})
	done := make(chan error, 1)
	go func() { done <- session.Prompt(context.Background(), "First message", nil) }()
	waitFor(t, session.IsStreaming)

	if err := session.FollowUp("Follow-up message"); err != nil {
		t.Fatalf("follow up: %v", err)
	}
	if got := session.PendingMessageCount(); got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}
	session.Abort()
	<-done
}

func TestPromptAfterPreviousCompletes(t *testing.T) {
	session := newTestSession(t, testSessionOptions{})
	if err := session.Prompt(context.Background(), "First message", nil); err != nil {
		t.Fatalf("first prompt: %v", err)
	}
	if session.IsStreaming() {
		t.Fatalf("streaming after first prompt")
	}
	if err := session.Prompt(context.Background(), "Second message", nil); err != nil {
		t.Fatalf("second prompt: %v", err)
	}
}

// controllableStream waits for release before finishing the first call and
// records the transcript of the second.
type controllableStream struct {
	started    chan struct{}
	release    chan struct{}
	secondSeen chan string
	mu         sync.Mutex
	calls      int
}

func (c *controllableStream) fn() model.StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		c.mu.Lock()
		c.calls++
		call := c.calls
		c.mu.Unlock()
		stream := wire.NewAssistantMessageEventStream()
		if call == 1 {
			if c.started != nil {
				close(c.started)
			}
			stream.Push(model.AssistantMessageEvent{Type: model.EventStart, Partial: testAssistant("", model.StopPending)})
			go func() {
				select {
				case <-c.release:
				case <-ctx.Done():
				}
				stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistant("first", model.StopStop)})
			}()
			return stream, nil
		}
		var texts []string
		for _, message := range transcript.Messages {
			switch value := message.(type) {
			case model.UserMessage:
				texts = append(texts, model.ContentText(value.Content))
			case *model.UserMessage:
				if value != nil {
					texts = append(texts, model.ContentText(value.Content))
				}
			}
		}
		select {
		case c.secondSeen <- strings.Join(texts, "\n"):
		default:
		}
		stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistant("second", model.StopStop)})
		return stream, nil
	}
}

func TestSteerIsDeliveredToNextModelCall(t *testing.T) {
	control := &controllableStream{started: make(chan struct{}), release: make(chan struct{}), secondSeen: make(chan string, 1)}
	session := newTestSession(t, testSessionOptions{stream: control.fn()})

	done := make(chan error, 1)
	go func() { done <- session.Prompt(context.Background(), "First message", nil) }()
	select {
	case <-control.started:
	case <-time.After(3 * time.Second):
		t.Fatal("first model call did not start")
	}
	if err := session.Steer("Steering message"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	close(control.release)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("prompt did not finish")
	}
	select {
	case seen := <-control.secondSeen:
		if !strings.Contains(seen, "Steering message") {
			t.Fatalf("steering message not delivered to next call: %q", seen)
		}
	default:
		t.Fatal("second model call did not happen")
	}
	if session.PendingMessageCount() != 0 {
		t.Fatalf("pending = %d, want 0", session.PendingMessageCount())
	}
}

func TestMessagePersistenceOrder(t *testing.T) {
	cwd := t.TempDir()
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory manager: %v", err)
	}
	tool := model.AgentTool{
		Name: "dummy", Label: "dummy", Description: "Dummy tool", Parameters: model.Object(),
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: "result"}}, Details: map[string]any{}}, nil
		},
	}
	var mu sync.Mutex
	calls := 0
	stream := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		eventStream := wire.NewAssistantMessageEventStream()
		if call == 1 {
			message := testAssistant("calling tool", model.StopToolUse)
			message.Content = append(message.Content, model.ToolCall{ID: "toolu_1", Name: "dummy", Arguments: map[string]any{"q": "x"}})
			eventStream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopToolUse, Message: message})
			return eventStream, nil
		}
		eventStream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistant("done", model.StopStop)})
		return eventStream, nil
	}
	settings := config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingOff, Tools: []model.AgentTool{tool}},
		StreamFn:     stream,
	})
	session, err := New(Config{Agent: builtAgent, SessionManager: manager, Settings: settings, Cwd: cwd})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(session.Dispose)

	if err := session.Prompt(context.Background(), "hi", nil); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	var roles []model.Role
	for _, entry := range manager.GetEntries() {
		if messageEntry, ok := entry.(*model.SessionMessageEntry); ok {
			roles = append(roles, messageEntry.Message.MessageRole())
		}
	}
	want := []model.Role{model.RoleSystem, model.RoleUser, model.RoleAssistant, model.RoleToolResult, model.RoleAssistant}
	if len(roles) != len(want) {
		t.Fatalf("roles = %v, want %v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("roles = %v, want %v", roles, want)
		}
	}
}

func TestForkIsIndependent(t *testing.T) {
	session := newTestSession(t, testSessionOptions{})
	if err := session.Prompt(context.Background(), "parent prompt", nil); err != nil {
		t.Fatalf("parent prompt: %v", err)
	}

	child, err := session.Fork(context.Background())
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Cleanup(child.Dispose)
	if child.SessionID() == session.SessionID() {
		t.Fatalf("child session id equals parent")
	}
	if len(child.Messages()) != len(session.Messages()) {
		t.Fatalf("child messages = %d, parent = %d", len(child.Messages()), len(session.Messages()))
	}

	// A prompt on the child must not change the parent transcript.
	if err := child.Prompt(context.Background(), "child prompt", nil); err != nil {
		t.Fatalf("child prompt: %v", err)
	}
	if len(session.Messages()) != len(child.Messages())-2 {
		t.Fatalf("parent changed after child prompt: parent=%d child=%d", len(session.Messages()), len(child.Messages()))
	}
}

func TestDisposeIsIdempotent(t *testing.T) {
	session := newTestSession(t, testSessionOptions{})
	session.Dispose()
	session.Dispose()
	if !session.IsIdle() {
		t.Fatalf("session not idle after dispose")
	}
}
