package history

import (
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func mustAppendMessage(t *testing.T, session *SessionManager, message model.AgentMessage) string {
	t.Helper()
	id, err := session.AppendMessage(message)
	if err != nil {
		t.Fatalf("append message: %v", err)
	}
	return id
}

func userMsg(text string) model.AgentMessage {
	return model.NewUserText(text, time.Now().UnixMilli())
}

func assistantMsg(text string) model.AgentMessage {
	return model.AssistantMessage{
		Content:  model.ContentList{model.TextContent{Text: text}},
		Api:      model.APIAnthropicMessages,
		Provider: model.ProviderAnthropic,
		Model:    "test",
		Usage: model.Usage{
			Input: 1, Output: 1, TotalTokens: 2,
			Cost: model.CostBreakdown{},
		},
		StopReason: model.StopStop,
		Timestamp:  time.Now().UnixMilli(),
	}
}

func toolResultMsg(text string) model.AgentMessage {
	return model.ToolResultMessage{
		ToolCallID: "call-1",
		ToolName:   "read",
		Content:    model.ContentList{model.TextContent{Text: text}},
		Timestamp:  time.Now().UnixMilli(),
	}
}

func contentText(content model.ContentList) string {
	text := ""
	for _, block := range content {
		if typed, ok := block.(model.TextContent); ok {
			text += typed.Text
		}
	}
	return text
}
