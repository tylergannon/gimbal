package compact

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func branchTestModel() *model.Model {
	return &model.Model{
		ID:            "test-model",
		Name:          "Test Model",
		Api:           "anthropic-messages",
		Provider:      "anthropic",
		BaseURL:       "https://api.anthropic.com",
		ContextWindow: 200000,
		MaxTokens:     8192,
	}
}

func branchTestEntries() []model.SessionEntry {
	b := newEntryBuilder()
	return []model.SessionEntry{b.message(userMessage("Abandoned request"))}
}

func singleResponseStream(response *model.AssistantMessage) model.StreamFunction {
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		stream := wire.NewAssistantMessageEventStream()
		event := model.AssistantMessageEvent{Type: model.EventDone, Reason: response.StopReason, Message: response}
		if response.StopReason == model.StopError || response.StopReason == model.StopAborted {
			event = model.AssistantMessageEvent{Type: model.EventError, Reason: response.StopReason, Error: response}
		}
		stream.Push(event)
		return stream, nil
	}
}

func TestBranchSummaryDoesNotOverrideToolChoice(t *testing.T) {
	entries := branchTestEntries()
	response := &model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: "summary"}},
		StopReason: model.StopStop,
	}
	var captured *model.SimpleStreamOptions
	streamFn := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		captured = options
		return singleResponseStream(response)(ctx, m, transcript, options)
	}

	result := GenerateBranchSummary(context.Background(), entries, GenerateBranchSummaryOptions{
		Model: branchTestModel(), StreamFn: streamFn,
	})
	if result.Error != "" {
		t.Fatal(result.Error)
	}

	if captured == nil {
		t.Fatal("stream function was not called")
	}
	if captured.MaxTokens == nil || *captured.MaxTokens != 4096 {
		t.Fatalf("maxTokens = %v, want 4096", captured.MaxTokens)
	}
	if captured.ToolChoice != "" {
		t.Fatalf("toolChoice = %q, want empty", captured.ToolChoice)
	}
}

func TestBranchSummaryClampsOutputCapToModelLimit(t *testing.T) {
	entries := branchTestEntries()
	m := branchTestModel()
	m.MaxTokens = 1024
	response := &model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: "summary"}},
		StopReason: model.StopStop,
	}
	var captured *model.SimpleStreamOptions
	streamFn := func(ctx context.Context, mdl *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		captured = options
		return singleResponseStream(response)(ctx, mdl, transcript, options)
	}

	result := GenerateBranchSummary(context.Background(), entries, GenerateBranchSummaryOptions{
		Model: m, StreamFn: streamFn,
	})
	if result.Error != "" {
		t.Fatal(result.Error)
	}
	if captured == nil || captured.MaxTokens == nil || *captured.MaxTokens != 1024 {
		t.Fatalf("maxTokens = %v, want 1024", captured.MaxTokens)
	}
}

func TestBranchSummaryRejectsToolCalls(t *testing.T) {
	entries := branchTestEntries()
	response := &model.AssistantMessage{
		Content: model.ContentList{
			model.ToolCall{ID: "tool-call-1", Name: "read", Arguments: map[string]any{"path": "README.md"}},
		},
		StopReason: model.StopToolUse,
	}

	result := GenerateBranchSummary(context.Background(), entries, GenerateBranchSummaryOptions{
		Model: branchTestModel(), StreamFn: singleResponseStream(response),
	})
	if result.Error != "Branch summarization attempted to call a tool" {
		t.Fatalf("error = %q", result.Error)
	}
}

func TestBranchSummaryRejectsLengthLimitedSummaries(t *testing.T) {
	entries := branchTestEntries()
	response := &model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: "partial"}},
		StopReason: model.StopLength,
	}

	result := GenerateBranchSummary(context.Background(), entries, GenerateBranchSummaryOptions{
		Model: branchTestModel(), StreamFn: singleResponseStream(response),
	})
	want := "Branch summarization failed: generation hit the token cap and the summary is incomplete"
	if result.Error != want {
		t.Fatalf("error = %q, want %q", result.Error, want)
	}
}
