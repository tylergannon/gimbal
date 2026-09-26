package wire

import (
	"slices"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Transcript message transformation, ported from pi
// packages/ai/src/api/transform-messages.ts (upstream d6af72e1).

const (
	nonVisionUserImagePlaceholder = "(image omitted: model does not support images)"
	nonVisionToolImagePlaceholder = "(tool image omitted: model does not support images)"
)

// TransformMessages normalizes a transcript for a target model: unsupported
// images are downgraded to placeholders, cross-model thinking blocks are
// converted to plain text, tool-call IDs can be normalized, and synthetic empty
// tool results are inserted for orphaned tool calls. The normalizeToolCallID
// callback may be nil.
func TransformMessages(messages []model.Message, m *model.Model, normalizeToolCallID func(id string, m *model.Model, source model.AssistantMessage) string) []model.Message {
	toolCallIDMap := map[string]string{}

	normalizedMessages := make([]model.Message, len(messages))
	for i, message := range messages {
		normalizedMessages[i] = ensureMessageContent(message)
	}
	imageAwareMessages := downgradeUnsupportedImages(normalizedMessages, m)

	transformed := make([]model.Message, 0, len(imageAwareMessages))
	for _, message := range imageAwareMessages {
		switch message.MessageRole() {
		case model.RoleSystem, model.RoleUser:
			transformed = append(transformed, message)
		case model.RoleToolResult:
			result, ok := toolResultMessageOf(message)
			if !ok {
				transformed = append(transformed, message)
				continue
			}
			if normalizedID, ok := toolCallIDMap[result.ToolCallID]; ok && normalizedID != result.ToolCallID {
				result.ToolCallID = normalizedID
				transformed = append(transformed, result)
				continue
			}
			transformed = append(transformed, message)
		case model.RoleAssistant:
			assistant, ok := assistantMessageOf(message)
			if !ok {
				transformed = append(transformed, message)
				continue
			}
			isSameModel := assistant.Provider == m.Provider && assistant.Api == m.Api && assistant.Model == m.ID
			assistant.Content = transformAssistantContent(assistant.Content, isSameModel, m, assistant, normalizeToolCallID, toolCallIDMap)
			transformed = append(transformed, assistant)
		default:
			transformed = append(transformed, message)
		}
	}

	// Second pass: insert synthetic empty tool results for orphaned tool calls.
	result := make([]model.Message, 0, len(transformed))
	var pendingToolCalls []model.ToolCall
	existingToolResultIDs := map[string]bool{}
	var heldSystemMessages []model.Message
	closePendingToolCalls := func() {
		if len(pendingToolCalls) > 0 {
			for _, toolCall := range pendingToolCalls {
				if !existingToolResultIDs[toolCall.ID] {
					result = append(result, model.ToolResultMessage{
						ToolCallID: toolCall.ID,
						ToolName:   toolCall.Name,
						Content:    model.ContentList{model.TextContent{Text: "No result provided"}},
						IsError:    true,
						Timestamp:  time.Now().UnixMilli(),
					})
				}
			}
			pendingToolCalls = nil
			existingToolResultIDs = map[string]bool{}
		}
		result = append(result, heldSystemMessages...)
		heldSystemMessages = nil
	}

	for _, message := range transformed {
		switch message.MessageRole() {
		case model.RoleAssistant:
			closePendingToolCalls()
			assistant, ok := assistantMessageOf(message)
			if !ok {
				result = append(result, message)
				continue
			}
			if assistant.StopReason == model.StopError || assistant.StopReason == model.StopAborted {
				continue
			}
			if toolCalls := toolCallsOf(assistant.Content); len(toolCalls) > 0 {
				pendingToolCalls = toolCalls
				existingToolResultIDs = map[string]bool{}
			}
			result = append(result, message)
		case model.RoleToolResult:
			if toolResult, ok := toolResultMessageOf(message); ok {
				existingToolResultIDs[toolResult.ToolCallID] = true
			}
			result = append(result, message)
		case model.RoleSystem:
			if len(pendingToolCalls) > 0 {
				heldSystemMessages = append(heldSystemMessages, message)
			} else {
				result = append(result, message)
			}
		case model.RoleUser:
			closePendingToolCalls()
			result = append(result, message)
		default:
			result = append(result, message)
		}
	}
	closePendingToolCalls()

	return result
}

func transformAssistantContent(content model.ContentList, isSameModel bool, m *model.Model, source model.AssistantMessage, normalizeToolCallID func(string, *model.Model, model.AssistantMessage) string, toolCallIDMap map[string]string) model.ContentList {
	var out model.ContentList
	for _, block := range content {
		switch b := block.(type) {
		case model.ThinkingContent:
			if transformed, keep := transformThinking(b, isSameModel); keep {
				out = append(out, transformed)
			}
		case *model.ThinkingContent:
			if b == nil {
				continue
			}
			if transformed, keep := transformThinking(*b, isSameModel); keep {
				out = append(out, transformed)
			}
		case model.ToolCall:
			out = append(out, transformToolCall(b, isSameModel, m, source, normalizeToolCallID, toolCallIDMap))
		case *model.ToolCall:
			if b == nil {
				continue
			}
			out = append(out, transformToolCall(*b, isSameModel, m, source, normalizeToolCallID, toolCallIDMap))
		default:
			out = append(out, block)
		}
	}
	return out
}

// transformThinking returns the replacement block and whether it should be kept.
func transformThinking(block model.ThinkingContent, isSameModel bool) (model.Content, bool) {
	if block.Redacted {
		if isSameModel {
			return block, true
		}
		return nil, false
	}
	if isSameModel && block.ThinkingSignature != "" {
		return block, true
	}
	if strings.TrimSpace(block.Thinking) == "" {
		return nil, false
	}
	if isSameModel {
		return block, true
	}
	return model.TextContent{Text: block.Thinking}, true
}

func transformToolCall(call model.ToolCall, isSameModel bool, m *model.Model, source model.AssistantMessage, normalizeToolCallID func(string, *model.Model, model.AssistantMessage) string, toolCallIDMap map[string]string) model.ToolCall {
	normalized := call
	if !isSameModel && normalized.ThoughtSignature != "" {
		normalized.ThoughtSignature = ""
	}
	if !isSameModel && normalizeToolCallID != nil {
		normalizedID := normalizeToolCallID(call.ID, m, source)
		if normalizedID != call.ID {
			toolCallIDMap[call.ID] = normalizedID
			normalized.ID = normalizedID
		}
	}
	return normalized
}

func downgradeUnsupportedImages(messages []model.Message, m *model.Model) []model.Message {
	if slices.Contains(m.Input, "image") {
		return messages
	}
	out := make([]model.Message, len(messages))
	for i, message := range messages {
		switch message.MessageRole() {
		case model.RoleUser:
			user, ok := userMessageOf(message)
			if !ok {
				out[i] = message
				continue
			}
			// pi only downgrades array content; the plain-string form passes
			// through untouched, preserving its encoding.
			if _, isString := user.StringContent(); isString {
				out[i] = message
				continue
			}
			user.Content = replaceImagesWithPlaceholder(user.Content, nonVisionUserImagePlaceholder)
			out[i] = user
		case model.RoleToolResult:
			toolResult, ok := toolResultMessageOf(message)
			if !ok {
				out[i] = message
				continue
			}
			toolResult.Content = replaceImagesWithPlaceholder(toolResult.Content, nonVisionToolImagePlaceholder)
			out[i] = toolResult
		default:
			out[i] = message
		}
	}
	return out
}

func replaceImagesWithPlaceholder(content model.ContentList, placeholder string) model.ContentList {
	result := make(model.ContentList, 0, len(content))
	previousWasPlaceholder := false
	for _, block := range content {
		if _, isImage := block.(model.ImageContent); isImage {
			if !previousWasPlaceholder {
				result = append(result, model.TextContent{Text: placeholder})
			}
			previousWasPlaceholder = true
			continue
		}
		if b, isPointerImage := block.(*model.ImageContent); isPointerImage && b != nil {
			if !previousWasPlaceholder {
				result = append(result, model.TextContent{Text: placeholder})
			}
			previousWasPlaceholder = true
			continue
		}
		result = append(result, block)
		switch b := block.(type) {
		case model.TextContent:
			previousWasPlaceholder = b.Text == placeholder
		case *model.TextContent:
			previousWasPlaceholder = b != nil && b.Text == placeholder
		default:
			previousWasPlaceholder = false
		}
	}
	return result
}

func toolCallsOf(content model.ContentList) []model.ToolCall {
	var calls []model.ToolCall
	for _, block := range content {
		switch b := block.(type) {
		case model.ToolCall:
			calls = append(calls, b)
		case *model.ToolCall:
			if b != nil {
				calls = append(calls, *b)
			}
		}
	}
	return calls
}

func ensureMessageContent(message model.Message) model.Message {
	content, ok := contentListOf(message)
	if !ok || content != nil {
		return message
	}
	return withContent(message, model.ContentList{})
}

func contentListOf(message model.Message) (model.ContentList, bool) {
	switch m := message.(type) {
	case model.SystemMessage:
		return m.Content, true
	case *model.SystemMessage:
		if m == nil {
			return nil, false
		}
		return m.Content, true
	case model.UserMessage:
		return m.Content, true
	case *model.UserMessage:
		if m == nil {
			return nil, false
		}
		return m.Content, true
	case model.AssistantMessage:
		return m.Content, true
	case *model.AssistantMessage:
		if m == nil {
			return nil, false
		}
		return m.Content, true
	case model.ToolResultMessage:
		return m.Content, true
	case *model.ToolResultMessage:
		if m == nil {
			return nil, false
		}
		return m.Content, true
	case model.CustomMessage:
		return m.Content, true
	case *model.CustomMessage:
		if m == nil {
			return nil, false
		}
		return m.Content, true
	case model.BranchSummaryMessage, *model.BranchSummaryMessage,
		model.CompactionSummaryMessage, *model.CompactionSummaryMessage,
		model.BashExecutionMessage, *model.BashExecutionMessage:
		return nil, false
	default:
		return nil, false
	}
}

func withContent(message model.Message, content model.ContentList) model.Message {
	switch m := message.(type) {
	case model.SystemMessage:
		m.Content = content
		return m
	case *model.SystemMessage:
		if m == nil {
			return message
		}
		out := *m
		out.Content = content
		return &out
	case model.UserMessage:
		m.Content = content
		return m
	case *model.UserMessage:
		if m == nil {
			return message
		}
		out := *m
		out.Content = content
		return &out
	case model.AssistantMessage:
		m.Content = content
		return m
	case *model.AssistantMessage:
		if m == nil {
			return message
		}
		out := *m
		out.Content = content
		return &out
	case model.ToolResultMessage:
		m.Content = content
		return m
	case *model.ToolResultMessage:
		if m == nil {
			return message
		}
		out := *m
		out.Content = content
		return &out
	case model.CustomMessage:
		m.Content = content
		return m
	case *model.CustomMessage:
		if m == nil {
			return message
		}
		out := *m
		out.Content = content
		return &out
	default:
		return message
	}
}

func userMessageOf(message model.Message) (model.UserMessage, bool) {
	switch m := message.(type) {
	case model.UserMessage:
		return m, true
	case *model.UserMessage:
		if m != nil {
			return *m, true
		}
	}
	return model.UserMessage{}, false
}

func toolResultMessageOf(message model.Message) (model.ToolResultMessage, bool) {
	switch m := message.(type) {
	case model.ToolResultMessage:
		return m, true
	case *model.ToolResultMessage:
		if m != nil {
			return *m, true
		}
	}
	return model.ToolResultMessage{}, false
}
