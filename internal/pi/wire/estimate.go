package wire

import (
	"encoding/json"
	"math"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Context token estimation, ported from pi packages/ai/src/utils/estimate.ts
// (upstream d6af72e1).

const (
	charsPerToken       = 4
	estimatedImageChars = 4800
)

// ContextUsageEstimate is an estimated context token breakdown.
type ContextUsageEstimate struct {
	// Tokens is the estimated total context tokens.
	Tokens int
	// UsageTokens is the tokens reported by the most recent applicable
	// assistant usage block.
	UsageTokens int
	// TrailingTokens is the estimated tokens after that block.
	TrailingTokens int
	// LastUsageIndex is the index of the applicable message that provided
	// usage, or nil when none exists.
	LastUsageIndex *int
}

// CalculateContextTokens returns pi's calculateContextTokens: totalTokens when
// reported, else the sum of the parts.
func CalculateContextTokens(usage model.Usage) int {
	if usage.TotalTokens != 0 {
		return usage.TotalTokens
	}
	return usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
}

// EstimateTextTokens estimates tokens from a text length.
func EstimateTextTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / charsPerToken))
}

// EstimateTextAndImageContentTokens estimates tokens for a content list,
// weighting images by a fixed character count.
func EstimateTextAndImageContentTokens(content model.ContentList) int {
	chars := 0
	for _, block := range content {
		switch b := block.(type) {
		case model.TextContent:
			chars += len(b.Text)
		case *model.TextContent:
			if b != nil {
				chars += len(b.Text)
			}
		case model.ImageContent, *model.ImageContent:
			chars += estimatedImageChars
		}
	}
	return int(math.Ceil(float64(chars) / charsPerToken))
}

// EstimateMessageTokens estimates the context tokens one message contributes.
func EstimateMessageTokens(message model.Message) int {
	switch m := message.(type) {
	case model.SystemMessage:
		return estimateSystemTokens(m)
	case *model.SystemMessage:
		if m != nil {
			return estimateSystemTokens(*m)
		}
		return 0
	case model.UserMessage:
		return EstimateTextAndImageContentTokens(m.Content)
	case *model.UserMessage:
		if m != nil {
			return EstimateTextAndImageContentTokens(m.Content)
		}
		return 0
	case model.ToolResultMessage:
		return EstimateTextAndImageContentTokens(m.Content)
	case *model.ToolResultMessage:
		if m != nil {
			return EstimateTextAndImageContentTokens(m.Content)
		}
		return 0
	case model.AssistantMessage:
		return estimateAssistantTokens(m.Content)
	case *model.AssistantMessage:
		if m != nil {
			return estimateAssistantTokens(m.Content)
		}
		return 0
	case model.CustomMessage:
		return EstimateTextAndImageContentTokens(m.Content)
	case *model.CustomMessage:
		if m != nil {
			return EstimateTextAndImageContentTokens(m.Content)
		}
		return 0
	case model.BashExecutionMessage:
		return EstimateTextTokens(m.Command + m.Output)
	case *model.BashExecutionMessage:
		if m != nil {
			return EstimateTextTokens(m.Command + m.Output)
		}
		return 0
	case model.BranchSummaryMessage:
		return EstimateTextTokens(m.Summary)
	case *model.BranchSummaryMessage:
		if m != nil {
			return EstimateTextTokens(m.Summary)
		}
		return 0
	case model.CompactionSummaryMessage:
		return EstimateTextTokens(m.Summary)
	case *model.CompactionSummaryMessage:
		if m != nil {
			return EstimateTextTokens(m.Summary)
		}
		return 0
	default:
		return 0
	}
}

func estimateSystemTokens(message model.SystemMessage) int {
	return EstimateTextTokens(model.GetSystemMessageText(message)) +
		estimateToolsTokens(message.ToolsAdded) +
		estimateToolsTokens(message.ToolsRemoved)
}

func estimateAssistantTokens(content model.ContentList) int {
	chars := 0
	for _, block := range content {
		switch b := block.(type) {
		case model.TextContent:
			chars += len(b.Text)
		case *model.TextContent:
			if b != nil {
				chars += len(b.Text)
			}
		case model.ThinkingContent:
			chars += len(b.Thinking)
		case *model.ThinkingContent:
			if b != nil {
				chars += len(b.Thinking)
			}
		case model.ToolCall:
			chars += len(b.Name) + len(safeJSONStringify(b.Arguments))
		case *model.ToolCall:
			if b != nil {
				chars += len(b.Name) + len(safeJSONStringify(b.Arguments))
			}
		}
	}
	return int(math.Ceil(float64(chars) / charsPerToken))
}

func estimateToolsTokens[T any](tools []T) int {
	if len(tools) == 0 {
		return 0
	}
	return EstimateTextTokens(safeJSONStringify(tools))
}

func safeJSONStringify(value any) string {
	serialized, err := json.Marshal(value)
	if err != nil {
		return "[unserializable]"
	}
	return string(serialized)
}

func getLastAssistantUsageInfo(messages []model.Message) (model.Usage, int, bool) {
	var latestPrefixTimestamp int64 = math.MinInt64
	var usage model.Usage
	index := -1
	for i, message := range messages {
		if assistant, ok := assistantMessageOf(message); ok {
			if assistant.Timestamp >= latestPrefixTimestamp &&
				assistant.StopReason != model.StopAborted &&
				assistant.StopReason != model.StopError &&
				CalculateContextTokens(assistant.Usage) > 0 {
				usage = assistant.Usage
				index = i
			}
		}
		if timestamp := messageTimestamp(message); timestamp > latestPrefixTimestamp {
			latestPrefixTimestamp = timestamp
		}
	}
	return usage, index, index >= 0
}

func assistantMessageOf(message model.Message) (model.AssistantMessage, bool) {
	switch m := message.(type) {
	case model.AssistantMessage:
		return m, true
	case *model.AssistantMessage:
		if m != nil {
			return *m, true
		}
	}
	return model.AssistantMessage{}, false
}

func messageTimestamp(message model.Message) int64 {
	switch m := message.(type) {
	case model.SystemMessage:
		return m.Timestamp
	case *model.SystemMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.UserMessage:
		return m.Timestamp
	case *model.UserMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.AssistantMessage:
		return m.Timestamp
	case *model.AssistantMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.ToolResultMessage:
		return m.Timestamp
	case *model.ToolResultMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.BashExecutionMessage:
		return m.Timestamp
	case *model.BashExecutionMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.CustomMessage:
		return m.Timestamp
	case *model.CustomMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.BranchSummaryMessage:
		return m.Timestamp
	case *model.BranchSummaryMessage:
		if m != nil {
			return m.Timestamp
		}
	case model.CompactionSummaryMessage:
		return m.Timestamp
	case *model.CompactionSummaryMessage:
		if m != nil {
			return m.Timestamp
		}
	}
	return 0
}

// EstimateContextTokens estimates the tokens in a normalized transcript: the
// most recent applicable assistant usage plus the messages after it, or every
// message when no usage applies.
func EstimateContextTokens(transcript model.TranscriptContext) ContextUsageEstimate {
	return EstimateMessagesTokens(transcript.Messages)
}

// EstimateMessagesTokens estimates the tokens in a message slice.
func EstimateMessagesTokens(messages []model.Message) ContextUsageEstimate {
	if usage, index, ok := getLastAssistantUsageInfo(messages); ok {
		usageTokens := CalculateContextTokens(usage)
		trailingTokens := 0
		for i := index + 1; i < len(messages); i++ {
			trailingTokens += EstimateMessageTokens(messages[i])
		}
		return ContextUsageEstimate{
			Tokens:         usageTokens + trailingTokens,
			UsageTokens:    usageTokens,
			TrailingTokens: trailingTokens,
			LastUsageIndex: &index,
		}
	}

	tokens := 0
	for _, message := range messages {
		tokens += EstimateMessageTokens(message)
	}
	return ContextUsageEstimate{Tokens: tokens, TrailingTokens: tokens}
}
