package wire

import (
	"regexp"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

var anthropicIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func anthropicNormalizeToolCallID(id string, _ *model.Model, _ model.AssistantMessage) string {
	normalized := anthropicIDPattern.ReplaceAllString(id, "_")
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	return normalized
}

func copilotClaudeModel() *model.Model {
	return &model.Model{
		ID:            "claude-sonnet-4.6",
		Api:           model.APIAnthropicMessages,
		Provider:      model.ProviderId("github-copilot"),
		Input:         []string{"text", "image"},
		ContextWindow: 128000,
		MaxTokens:     16000,
	}
}

func transformAssistantMessage(content model.ContentList) model.AssistantMessage {
	return model.AssistantMessage{
		Content:    content,
		Api:        model.APIOpenAIResponses,
		Provider:   model.ProviderId("github-copilot"),
		Model:      "gpt-5",
		StopReason: model.StopToolUse,
		Timestamp:  1,
	}
}

func findAssistant(t *testing.T, messages []model.Message) model.AssistantMessage {
	t.Helper()
	for _, message := range messages {
		if message.MessageRole() == model.RoleAssistant {
			if assistant, ok := assistantMessageOf(message); ok {
				return assistant
			}
		}
	}
	t.Fatal("no assistant message in result")
	return model.AssistantMessage{}
}

func TestTransformMessagesConvertsThinkingAcrossModels(t *testing.T) {
	m := copilotClaudeModel()
	messages := []model.Message{
		model.NewUserText("hello", 0),
		model.AssistantMessage{
			Content: model.ContentList{
				model.ThinkingContent{Thinking: "Let me think about this...", ThinkingSignature: "reasoning_content"},
				model.TextContent{Text: "Hi there!"},
			},
			Api:        model.APIOpenAICompletions,
			Provider:   model.ProviderId("github-copilot"),
			Model:      "gpt-4o",
			StopReason: model.StopStop,
			Timestamp:  1,
		},
	}

	result := TransformMessages(messages, m, anthropicNormalizeToolCallID)
	assistant := findAssistant(t, result)

	thinking, text := 0, 0
	for _, block := range assistant.Content {
		switch block.(type) {
		case model.ThinkingContent, *model.ThinkingContent:
			thinking++
		case model.TextContent, *model.TextContent:
			text++
		}
	}
	if thinking != 0 {
		t.Fatalf("thinking blocks = %d, want 0", thinking)
	}
	if text < 2 {
		t.Fatalf("text blocks = %d, want at least 2", text)
	}
}

func TestTransformMessagesRemovesThoughtSignature(t *testing.T) {
	m := copilotClaudeModel()
	messages := []model.Message{
		model.NewUserText("run a command", 0),
		model.AssistantMessage{
			Content: model.ContentList{
				model.ToolCall{ID: "call_123", Name: "bash", Arguments: map[string]any{"command": "ls"}, ThoughtSignature: `{"type":"reasoning.encrypted"}`},
			},
			Api:        model.APIOpenAIResponses,
			Provider:   model.ProviderId("github-copilot"),
			Model:      "gpt-5",
			StopReason: model.StopToolUse,
			Timestamp:  1,
		},
		model.ToolResultMessage{
			ToolCallID: "call_123",
			ToolName:   "bash",
			Content:    model.ContentList{model.TextContent{Text: "output"}},
			Timestamp:  2,
		},
	}

	result := TransformMessages(messages, m, anthropicNormalizeToolCallID)
	assistant := findAssistant(t, result)
	for _, block := range assistant.Content {
		if call, ok := block.(model.ToolCall); ok {
			if call.ThoughtSignature != "" {
				t.Fatalf("thoughtSignature = %q, want empty", call.ThoughtSignature)
			}
			return
		}
	}
	t.Fatal("no tool call in transformed assistant")
}

func TestTransformMessagesAddsSyntheticTrailingResult(t *testing.T) {
	m := copilotClaudeModel()
	messages := []model.Message{
		model.NewUserText("read the file", 0),
		transformAssistantMessage(model.ContentList{
			model.ToolCall{ID: "call_123|fc_123", Name: "read", Arguments: map[string]any{"path": "README.md"}},
		}),
	}

	result := TransformMessages(messages, m, anthropicNormalizeToolCallID)
	last, ok := toolResultMessageOf(result[len(result)-1])
	if !ok {
		t.Fatalf("last message is %T, want tool result", result[len(result)-1])
	}
	if last.ToolCallID != "call_123_fc_123" || last.ToolName != "read" || !last.IsError {
		t.Fatalf("synthetic result = %+v", last)
	}
	if text, _ := last.Content.TextContentText(); text != "No result provided" {
		t.Fatalf("synthetic text = %q, want No result provided", text)
	}
}

func TestTransformMessagesAddsSyntheticOnlyForMissingResults(t *testing.T) {
	m := copilotClaudeModel()
	messages := []model.Message{
		model.NewUserText("run commands", 0),
		transformAssistantMessage(model.ContentList{
			model.ToolCall{ID: "call_1|fc_1", Name: "read", Arguments: map[string]any{"path": "README.md"}},
			model.ToolCall{ID: "call_2|fc_2", Name: "bash", Arguments: map[string]any{"command": "pwd"}},
		}),
		model.ToolResultMessage{
			ToolCallID: "call_1|fc_1",
			ToolName:   "read",
			Content:    model.ContentList{model.TextContent{Text: "done"}},
			Timestamp:  2,
		},
	}

	result := TransformMessages(messages, m, anthropicNormalizeToolCallID)
	var synthetic []model.ToolResultMessage
	for _, message := range result {
		if toolResult, ok := toolResultMessageOf(message); ok && toolResult.IsError {
			synthetic = append(synthetic, toolResult)
		}
	}
	if len(synthetic) != 1 {
		t.Fatalf("synthetic results = %d, want 1", len(synthetic))
	}
	if synthetic[0].ToolCallID != "call_2_fc_2" || synthetic[0].ToolName != "bash" {
		t.Fatalf("synthetic result = %+v", synthetic[0])
	}
}

func TestTransformMessagesDowngradesImagesForNonVisionModels(t *testing.T) {
	m := &model.Model{ID: "text-only", Api: model.APIOpenAICompletions, Provider: "test", Input: []string{"text"}}
	messages := []model.Message{
		model.UserMessage{
			Content: model.ContentList{
				model.TextContent{Text: "look"},
				model.ImageContent{Data: "a", MimeType: "image/png"},
				model.ImageContent{Data: "b", MimeType: "image/png"},
				model.TextContent{Text: "end"},
			},
			Timestamp: 0,
		},
	}

	result := TransformMessages(messages, m, nil)
	user, ok := userMessageOf(result[0])
	if !ok {
		t.Fatalf("result[0] = %T, want user message", result[0])
	}
	if len(user.Content) != 3 {
		t.Fatalf("content = %#v, want text, placeholder, text", user.Content)
	}
	if text, _ := user.Content[0].(model.TextContent); text.Text != "look" {
		t.Fatalf("first block = %#v, want look", user.Content[0])
	}
	if text, _ := user.Content[1].(model.TextContent); text.Text != nonVisionUserImagePlaceholder {
		t.Fatalf("second block = %#v, want placeholder", user.Content[1])
	}
}

func TestTransformMessagesPreservesStringUserContent(t *testing.T) {
	m := &model.Model{ID: "text-only", Api: model.APIOpenAICompletions, Provider: "test", Input: []string{"text"}}
	result := TransformMessages([]model.Message{model.NewUserText("hello", 0)}, m, nil)
	user, ok := userMessageOf(result[0])
	if !ok {
		t.Fatalf("result[0] = %T, want user message", result[0])
	}
	if text, isString := user.StringContent(); !isString || text != "hello" {
		t.Fatalf("StringContent() = %q, %v, want hello, true", text, isString)
	}
}

func TestTransformMessagesDropsErroredAssistantTurns(t *testing.T) {
	m := copilotClaudeModel()
	messages := []model.Message{
		model.NewUserText("hi", 0),
		model.AssistantMessage{StopReason: model.StopError, ErrorMessage: "boom", Timestamp: 1},
		model.NewUserText("again", 2),
	}

	result := TransformMessages(messages, m, nil)
	for _, message := range result {
		if message.MessageRole() == model.RoleAssistant {
			t.Fatal("errored assistant turn must be dropped")
		}
	}
	if len(result) != 2 {
		t.Fatalf("result length = %d, want 2", len(result))
	}
}
