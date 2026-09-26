package compact

import (
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

type entryBuilder struct {
	counter int
	lastID  *string
}

func newEntryBuilder() *entryBuilder { return &entryBuilder{} }

func (b *entryBuilder) nextID() string {
	id := fmt.Sprintf("test-id-%d", b.counter)
	b.counter++
	return id
}

func (b *entryBuilder) base(entryType string) model.SessionEntryBase {
	return model.SessionEntryBase{
		Type:      entryType,
		ID:        b.nextID(),
		ParentID:  b.lastID,
		Timestamp: "2024-01-01T00:00:00.000Z",
	}
}

func (b *entryBuilder) message(message model.AgentMessage) *model.SessionMessageEntry {
	base := b.base("message")
	entry := &model.SessionMessageEntry{SessionEntryBase: base, Message: message}
	b.lastID = &base.ID
	return entry
}

func (b *entryBuilder) compaction(summary, firstKeptEntryID string) *model.CompactionEntry {
	base := b.base("compaction")
	entry := &model.CompactionEntry{
		SessionEntryBase: base,
		Summary:          summary,
		FirstKeptEntryID: firstKeptEntryID,
		TokensBefore:     10000,
	}
	b.lastID = &base.ID
	return entry
}

func (b *entryBuilder) customMessage(customType, content string) *model.CustomMessageEntry {
	base := b.base("custom_message")
	entry := &model.CustomMessageEntry{
		SessionEntryBase: base,
		CustomType:       customType,
		Content:          model.ContentList{model.TextContent{Text: content}},
		Display:          true,
	}
	b.lastID = &base.ID
	return entry
}

func mockUsage(input, output int, cacheReadWrite ...int) model.Usage {
	cacheRead, cacheWrite := 0, 0
	if len(cacheReadWrite) > 0 {
		cacheRead = cacheReadWrite[0]
	}
	if len(cacheReadWrite) > 1 {
		cacheWrite = cacheReadWrite[1]
	}
	return model.Usage{
		Input:       input,
		Output:      output,
		CacheRead:   cacheRead,
		CacheWrite:  cacheWrite,
		TotalTokens: input + output + cacheRead + cacheWrite,
	}
}

func userMessage(text string) model.AgentMessage {
	return model.NewUserText(text, 1)
}

func assistantMessage(text string) model.AssistantMessage {
	return assistantMessageWithUsage(text, mockUsage(100, 50))
}

func assistantMessageWithUsage(text string, usage model.Usage) model.AssistantMessage {
	return model.AssistantMessage{
		Content:    model.ContentList{model.TextContent{Text: text}},
		Usage:      usage,
		StopReason: model.StopStop,
		Timestamp:  1,
	}
}

func repeat(text string, count int) string { return strings.Repeat(text, count) }

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func extractText(messages []model.AgentMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		switch typed := message.(type) {
		case model.UserMessage:
			parts = append(parts, model.ContentText(typed.Content))
		case model.AssistantMessage:
			parts = append(parts, model.ContentText(typed.Content))
		case model.ToolResultMessage:
			parts = append(parts, model.ContentText(typed.Content))
		case model.CustomMessage:
			parts = append(parts, model.ContentText(typed.Content))
		case model.BashExecutionMessage:
			parts = append(parts, typed.Command+"\n"+typed.Output)
		case model.BranchSummaryMessage:
			parts = append(parts, typed.Summary)
		case model.CompactionSummaryMessage:
			parts = append(parts, typed.Summary)
		}
	}
	return strings.Join(parts, "\n")
}
