package compact

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestSerializeConversationTruncatesLongToolResults(t *testing.T) {
	longContent := strings.Repeat("x", 5000)
	messages := []model.Message{
		model.ToolResultMessage{
			ToolCallID: "tc1",
			ToolName:   "read",
			Content:    model.ContentList{model.TextContent{Text: longContent}},
			Timestamp:  1,
		},
	}

	result := SerializeConversation(messages)

	if !strings.Contains(result, "[Tool result]:") {
		t.Fatalf("result missing tool result marker: %q", result)
	}
	if !strings.Contains(result, "[... 3000 more characters truncated]") {
		t.Fatalf("result missing truncation marker: %q", result)
	}
	if strings.Contains(result, strings.Repeat("x", 3000)) {
		t.Fatalf("result still contains the full content")
	}
	if !strings.Contains(result, strings.Repeat("x", 2000)) {
		t.Fatalf("result missing the first 2000 characters")
	}
}

func TestSerializeConversationKeepsShortToolResults(t *testing.T) {
	shortContent := strings.Repeat("x", 1500)
	messages := []model.Message{
		model.ToolResultMessage{
			ToolCallID: "tc1",
			ToolName:   "read",
			Content:    model.ContentList{model.TextContent{Text: shortContent}},
			Timestamp:  1,
		},
	}

	result := SerializeConversation(messages)

	if result != "[Tool result]: "+shortContent {
		t.Fatalf("result = %q", result)
	}
	if strings.Contains(result, "truncated") {
		t.Fatalf("result unexpectedly truncated")
	}
}

func TestSerializeConversationKeepsAssistantAndUserMessages(t *testing.T) {
	longText := strings.Repeat("y", 5000)
	messages := []model.Message{
		model.NewUserText(longText, 1),
		model.AssistantMessage{
			Content:    model.ContentList{model.TextContent{Text: longText}},
			Usage:      mockUsage(0, 0, 0, 0),
			StopReason: model.StopStop,
			Timestamp:  1,
		},
	}

	result := SerializeConversation(messages)

	if strings.Contains(result, "truncated") {
		t.Fatalf("result unexpectedly truncated")
	}
	if !strings.Contains(result, longText) {
		t.Fatalf("result missing the long text")
	}
}
