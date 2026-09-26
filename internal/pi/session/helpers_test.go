package session

import (
	"context"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/agent"
	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func testModel() *model.Model {
	return &model.Model{
		ID: "mock", Name: "mock", Api: model.APIOpenAIResponses, Provider: model.ProviderOpenAI,
		BaseURL: "https://example.invalid", Input: []string{"text"},
		ContextWindow: 200_000, MaxTokens: 4096,
	}
}

func testAssistant(text string, stop model.StopReason) *model.AssistantMessage {
	return &model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: text}},
		Api:        model.APIOpenAIResponses,
		Provider:   model.ProviderOpenAI,
		Model:      "mock",
		Usage:      model.Usage{},
		StopReason: stop,
		Timestamp:  time.Now().UnixMilli(),
	}
}

func doneStream(message *model.AssistantMessage) model.StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		stream.Push(model.AssistantMessageEvent{Type: model.EventStart, Partial: testAssistant("", model.StopPending)})
		stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: message.StopReason, Message: message})
		return stream, nil
	}
}

// blockingStream pushes a start event, then waits for context cancellation
// before emitting an aborted error, mimicking the upstream concurrent test.
func blockingStream() model.StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		stream.Push(model.AssistantMessageEvent{Type: model.EventStart, Partial: testAssistant("", model.StopPending)})
		go func() {
			<-ctx.Done()
			errorMessage := testAssistant("Aborted", model.StopAborted)
			errorMessage.ErrorMessage = "aborted"
			stream.Push(model.AssistantMessageEvent{Type: model.EventError, Reason: model.StopAborted, Error: errorMessage})
		}()
		return stream, nil
	}
}

type testSessionOptions struct {
	stream       model.StreamFunction
	summary      model.StreamFunction
	settings     *config.SettingsManager
	baseTools    map[string]model.AgentTool
	customTools  []model.ToolDefinition
	activeNames  []string
	allowedNames []string
	excluded     []string
}

func newTestSession(t *testing.T, opts testSessionOptions) *Session {
	t.Helper()
	cwd := t.TempDir()
	settings := opts.settings
	if settings == nil {
		settings = config.NewInMemorySettingsManager(nil, config.CreateOptions{})
	}
	manager, err := history.InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in-memory session manager: %v", err)
	}
	stream := opts.stream
	if stream == nil {
		stream = doneStream(testAssistant("done", model.StopStop))
	}
	builtAgent := agent.NewAgent(agent.AgentOptions{
		InitialState: &model.AgentState{Model: testModel(), ThinkingLevel: model.ThinkingOff},
		StreamFn:     stream,
	})
	session, err := New(Config{
		Agent:                  builtAgent,
		SessionManager:         manager,
		Settings:               settings,
		Cwd:                    cwd,
		StreamFn:               opts.summary,
		InitialActiveToolNames: opts.activeNames,
		AllowedToolNames:       opts.allowedNames,
		ExcludedToolNames:      opts.excluded,
		BaseTools:              opts.baseTools,
		CustomTools:            opts.customTools,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(session.Dispose)
	return session
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within deadline")
}
