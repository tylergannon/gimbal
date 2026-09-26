package openai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// convertMessages ports openai-completions.ts convertMessages: it transforms
// the transcript for the model, converts every message to the Chat Completions
// shape, and returns them in order.
func convertMessages(m *model.Model, transcript model.TranscriptContext, c compat, grammarProps map[string]string) ([]map[string]any, error) {
	normalized := model.ResolveTranscript(transcript, c.SupportsMidConvoSystemMessages)
	transformed := wire.TransformMessages(normalized.Messages, m, func(id string, target *model.Model, _ model.AssistantMessage) string {
		return normalizeToolCallID(target, id)
	})
	supportsToolAdditions := c.SupportsMidConvoSystemMessages && c.SupportsMidConvoToolAdditions
	transcriptTools := model.ResolveTranscriptTools(normalized.Messages, supportsToolAdditions)
	anchorsAdditions := transcriptTools.AnchorsAdditions

	instructionRole := "system"
	if m.Reasoning && c.SupportsDeveloperRole {
		instructionRole = "developer"
	}
	modelHasImageInput := slices.Contains(m.Input, "image")

	var params []map[string]any
	lastRole := ""

	for i := 0; i < len(transformed); i++ {
		message := transformed[i]

		if c.RequiresAssistantAfterToolResult && lastRole == "toolResult" {
			if _, ok := asUserMessage(message); ok {
				params = append(params, map[string]any{
					"role":    "assistant",
					"content": "I have processed the tool results.",
				})
			}
		}

		switch {
		case isSystemRole(message):
			system, _ := asSystemMessage(message)
			if i > 0 && anchorsAdditions && len(system.ToolsAdded) > 0 {
				tools, err := convertTools(system.ToolsAdded, c)
				if err != nil {
					return nil, err
				}
				params = append(params, map[string]any{"role": "system", "tools": tools})
			}
			text := ""
			if i == 0 {
				text = model.GetSystemMessageText(system)
			} else {
				text = model.RenderSystemMessageUpdate(system)
			}
			if text != "" {
				params = append(params, map[string]any{"role": instructionRole, "content": sanitizeSurrogates(text)})
			}
			lastRole = "system"
		case isUserRole(message):
			user, _ := asUserMessage(message)
			if text, isString := user.StringContent(); isString {
				params = append(params, map[string]any{"role": "user", "content": sanitizeSurrogates(text)})
				lastRole = "user"
				continue
			}
			content := openAIUserContent(user.Content)
			if len(content) == 0 {
				continue
			}
			params = append(params, map[string]any{"role": "user", "content": content})
			lastRole = "user"
		case isAssistantRole(message):
			assistant, _ := asAssistantMessage(message)
			msg, err := convertAssistantMessage(m, assistant, c, grammarProps)
			if err != nil {
				return nil, err
			}
			if msg == nil {
				continue
			}
			params = append(params, msg)
			lastRole = "assistant"
		case isToolResultRole(message):
			var imageBlocks []any
			j := i
			for ; j < len(transformed) && isToolResultRole(transformed[j]); j++ {
				toolResult, _ := asToolResultMessage(transformed[j])
				var texts []string
				hasImages := false
				for _, block := range toolResult.Content {
					switch b := block.(type) {
					case model.TextContent:
						texts = append(texts, b.Text)
					case *model.TextContent:
						if b != nil {
							texts = append(texts, b.Text)
						}
					case model.ImageContent, *model.ImageContent:
						hasImages = true
					}
				}
				textResult := strings.Join(texts, "\n")
				content := textResult
				if content == "" {
					if hasImages {
						content = "(see attached image)"
					} else {
						content = "(no tool output)"
					}
				}
				toolMsg := map[string]any{
					"role":         "tool",
					"content":      sanitizeSurrogates(content),
					"tool_call_id": toolResult.ToolCallID,
				}
				if c.RequiresToolResultName && toolResult.ToolName != "" {
					toolMsg["name"] = toolResult.ToolName
				}
				params = append(params, toolMsg)

				if hasImages && modelHasImageInput {
					for _, block := range toolResult.Content {
						var image model.ImageContent
						switch b := block.(type) {
						case model.ImageContent:
							image = b
						case *model.ImageContent:
							if b == nil {
								continue
							}
							image = *b
						default:
							continue
						}
						imageBlocks = append(imageBlocks, map[string]any{
							"type":      "image_url",
							"image_url": map[string]any{"url": fmt.Sprintf("data:%s;base64,%s", image.MimeType, image.Data)},
						})
					}
				}
			}
			i = j - 1
			if len(imageBlocks) > 0 {
				if c.RequiresAssistantAfterToolResult {
					params = append(params, map[string]any{
						"role":    "assistant",
						"content": "I have processed the tool results.",
					})
				}
				content := []any{map[string]any{"type": "text", "text": "Attached image(s) from tool result:"}}
				content = append(content, imageBlocks...)
				params = append(params, map[string]any{"role": "user", "content": content})
				lastRole = "user"
			} else {
				lastRole = "toolResult"
			}
			continue
		default:
			lastRole = string(message.MessageRole())
		}
	}
	return params, nil
}

func convertAssistantMessage(m *model.Model, assistant model.AssistantMessage, c compat, grammarProps map[string]string) (map[string]any, error) {
	msg := map[string]any{"role": "assistant"}
	if c.RequiresAssistantAfterToolResult {
		msg["content"] = ""
	} else {
		msg["content"] = nil
	}

	var assistantTextParts []string
	var toolCalls []map[string]any
	var thinkingBlocks []model.ThinkingContent
	var nonEmptyThinkingBlocks []model.ThinkingContent
	var toolCallBlocks []model.ToolCall

	for _, block := range assistant.Content {
		switch b := block.(type) {
		case model.TextContent:
			if strings.TrimSpace(b.Text) != "" {
				assistantTextParts = append(assistantTextParts, sanitizeSurrogates(b.Text))
			}
		case *model.TextContent:
			if b != nil && strings.TrimSpace(b.Text) != "" {
				assistantTextParts = append(assistantTextParts, sanitizeSurrogates(b.Text))
			}
		case model.ThinkingContent:
			thinkingBlocks = append(thinkingBlocks, b)
			if strings.TrimSpace(b.Thinking) != "" {
				nonEmptyThinkingBlocks = append(nonEmptyThinkingBlocks, b)
			}
		case *model.ThinkingContent:
			if b != nil {
				thinkingBlocks = append(thinkingBlocks, *b)
				if strings.TrimSpace(b.Thinking) != "" {
					nonEmptyThinkingBlocks = append(nonEmptyThinkingBlocks, *b)
				}
			}
		case model.ToolCall:
			toolCallBlocks = append(toolCallBlocks, b)
			call, err := convertToolCall(b, grammarProps)
			if err != nil {
				return nil, err
			}
			toolCalls = append(toolCalls, call)
		case *model.ToolCall:
			if b != nil {
				toolCallBlocks = append(toolCallBlocks, *b)
				call, err := convertToolCall(*b, grammarProps)
				if err != nil {
					return nil, err
				}
				toolCalls = append(toolCalls, call)
			}
		}
	}
	assistantText := strings.Join(assistantTextParts, "")

	var preservedReasoningDetails []json.RawMessage
	for _, block := range thinkingBlocks {
		if details := parseOpenAIReasoningDetails(block.ThinkingSignature); details != nil {
			preservedReasoningDetails = details
			break
		}
	}
	if len(preservedReasoningDetails) == 0 {
		for _, call := range toolCallBlocks {
			if detail, ok := parseLegacyEncryptedReasoningDetail(call.ThoughtSignature); ok {
				preservedReasoningDetails = append(preservedReasoningDetails, detail)
			}
		}
	}

	if len(nonEmptyThinkingBlocks) > 0 {
		if c.RequiresThinkingAsText {
			var texts []string
			for _, block := range nonEmptyThinkingBlocks {
				texts = append(texts, sanitizeSurrogates(block.Thinking))
			}
			contentBlocks := []any{map[string]any{"type": "text", "text": strings.Join(texts, "\n\n")}}
			for _, part := range assistantTextParts {
				contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": part})
			}
			msg["content"] = contentBlocks
		} else {
			if assistantText != "" {
				msg["content"] = assistantText
			}
			if len(preservedReasoningDetails) == 0 {
				signature := nonEmptyThinkingBlocks[0].ThinkingSignature
				if m.Provider == "opencode-go" && signature == "reasoning" {
					signature = "reasoning_content"
				}
				if isOpenAICompletionsReasoningField(signature) {
					var thoughts []string
					for _, block := range nonEmptyThinkingBlocks {
						thoughts = append(thoughts, block.Thinking)
					}
					msg[signature] = strings.Join(thoughts, "\n")
				}
			}
		}
	} else if assistantText != "" {
		msg["content"] = assistantText
	}

	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	if len(preservedReasoningDetails) > 0 {
		msg["reasoning_details"] = preservedReasoningDetails
	}
	if c.RequiresReasoningContentOnAssistantMessages && m.Reasoning {
		if _, ok := msg["reasoning_content"]; !ok {
			msg["reasoning_content"] = ""
		}
	}

	content := msg["content"]
	hasContent := false
	switch value := content.(type) {
	case string:
		hasContent = value != ""
	case []any:
		hasContent = len(value) > 0
	}
	_, hasToolCalls := msg["tool_calls"]
	if !hasContent && !hasToolCalls {
		return nil, nil
	}
	return msg, nil
}

func convertToolCall(call model.ToolCall, grammarProps map[string]string) (map[string]any, error) {
	if property, ok := grammarProps[call.Name]; ok {
		input, err := getGrammarToolInput(call.Name, call.Arguments, property)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":   call.ID,
			"type": "custom",
			"custom": map[string]any{
				"name":  call.Name,
				"input": sanitizeSurrogates(input),
			},
		}, nil
	}
	arguments, err := json.Marshal(call.Arguments)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":   call.ID,
		"type": "function",
		"function": map[string]any{
			"name":      call.Name,
			"arguments": string(arguments),
		},
	}, nil
}

// convertTools serializes tool declarations into the Chat Completions tools
// shape.
func convertTools(tools []model.Tool, c compat) ([]map[string]any, error) {
	var out []map[string]any
	for _, tool := range tools {
		grammar, err := resolveGrammarConstrainedSampling(tool, c.SupportsOpenAIGrammarTools)
		if err != nil {
			return nil, err
		}
		if grammar != nil {
			out = append(out, map[string]any{
				"type": "custom",
				"custom": map[string]any{
					"name":        tool.Name,
					"description": tool.Description,
					"format": map[string]any{
						"type":    "grammar",
						"grammar": map[string]any{"syntax": grammar.Format, "definition": grammar.Definition},
					},
				},
			})
			continue
		}
		strict, err := resolveJSONSchemaStrictSampling(tool, c.SupportsStrictMode)
		if err != nil {
			return nil, err
		}
		parameters, err := getJSONSchemaToolParameters(tool, strict)
		if err != nil {
			return nil, err
		}
		function := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  parameters,
		}
		if c.SupportsStrictMode {
			if strict != nil {
				function["strict"] = *strict
			} else {
				function["strict"] = false
			}
		}
		out = append(out, map[string]any{"type": "function", "function": function})
	}
	return out, nil
}

// hasToolHistory reports whether the conversation contains tool calls or tool
// results.
func hasToolHistory(messages []model.Message) bool {
	for _, message := range messages {
		if isToolResultRole(message) {
			return true
		}
		if assistant, ok := asAssistantMessage(message); ok {
			for _, block := range assistant.Content {
				switch block.(type) {
				case model.ToolCall, *model.ToolCall:
					return true
				}
			}
		}
	}
	return false
}

// openAIUserContent maps user content to OpenAI parts.
func openAIUserContent(content model.ContentList) []any {
	var parts []any
	for _, block := range content {
		switch b := block.(type) {
		case model.TextContent:
			if b.Text == "" {
				continue
			}
			parts = append(parts, map[string]any{"type": "text", "text": sanitizeSurrogates(b.Text)})
		case *model.TextContent:
			if b == nil || b.Text == "" {
				continue
			}
			parts = append(parts, map[string]any{"type": "text", "text": sanitizeSurrogates(b.Text)})
		case model.ImageContent:
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": fmt.Sprintf("data:%s;base64,%s", b.MimeType, b.Data)},
			})
		case *model.ImageContent:
			if b == nil {
				continue
			}
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": fmt.Sprintf("data:%s;base64,%s", b.MimeType, b.Data)},
			})
		}
	}
	return parts
}

var openAIToolCallIDSanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// normalizeToolCallID normalizes a tool-call id for Chat Completions.
func normalizeToolCallID(m *model.Model, id string) string {
	if before, after, ok := strings.Cut(id, "|"); ok {
		callID := openAIToolCallIDSanitizeRe.ReplaceAllString(before, "_")
		itemID := openAIToolCallIDSanitizeRe.ReplaceAllString(after, "_")
		combined := callID
		if itemID != "" {
			combined = callID + "_" + itemID
		}
		if len(combined) <= 40 {
			return combined
		}
		hash := shortHash(id)
		if len(hash) > 8 {
			hash = hash[:8]
		}
		prefixLength := min(max(40-len(hash)-1, 1), len(callID))
		return callID[:prefixLength] + "_" + hash
	}
	if m.Provider == "openai" {
		if runes := []rune(id); len(runes) > 40 {
			return string(runes[:40])
		}
	}
	return id
}

func compatCacheControl(c compat, retention model.CacheRetention) map[string]any {
	if c.CacheControlFormat != "anthropic" || retention == model.CacheNone {
		return nil
	}
	control := map[string]any{"type": "ephemeral"}
	if retention == model.CacheLong && c.SupportsLongCacheRetention {
		control["ttl"] = "1h"
	}
	return control
}

func applyAnthropicCacheControl(messages []map[string]any, tools any, control map[string]any) {
	for _, message := range messages {
		if role, _ := message["role"].(string); role == "system" || role == "developer" {
			addCacheControlToTextContent(message, control)
			break
		}
	}
	if list, ok := tools.([]map[string]any); ok && len(list) > 0 {
		list[len(list)-1]["cache_control"] = control
	}
	for _, message := range slices.Backward(messages) {
		if role, _ := message["role"].(string); role == "user" || role == "assistant" || role == "tool" {
			if addCacheControlToTextContent(message, control) {
				break
			}
		}
	}
}

func addCacheControlToTextContent(message map[string]any, control map[string]any) bool {
	switch content := message["content"].(type) {
	case string:
		if content == "" {
			return false
		}
		message["content"] = []any{map[string]any{"type": "text", "text": content, "cache_control": control}}
		return true
	case []any:
		for _, c := range slices.Backward(content) {
			part, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if part["type"] == "text" {
				part["cache_control"] = control
				return true
			}
		}
	}
	return false
}
