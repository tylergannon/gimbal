package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSystemMessageCodecStringForm(t *testing.T) {
	original := NewSystemText("hello", 0)
	original.Sections = SystemSections{{Name: "rules", Value: new("be kind")}}
	original.ToolsAdded = []Tool{{Name: "read", Description: "read a file"}}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"content":"hello"`) {
		t.Fatalf("string content not preserved: %s", raw)
	}
	var decoded SystemMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	text, ok := decoded.StringContent()
	if !ok || text != "hello" {
		t.Fatalf("StringContent = %q, %v", text, ok)
	}
	if got, want := decoded.ToolsAdded[0].Name, "read"; got != want {
		t.Fatalf("tool name = %q, want %q", got, want)
	}
	if decoded.Sections.Len() != 1 {
		t.Fatalf("sections = %v", decoded.Sections)
	}
}

func TestSystemMessageCodecArrayForm(t *testing.T) {
	msg := SystemMessage{Content: ContentList{TextContent{Text: "a"}, TextContent{Text: "b"}}, Timestamp: 5}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SystemMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded.StringContent(); ok {
		t.Fatalf("array content reported as string")
	}
	if got := ContentText(decoded.Content); got != "a\nb" {
		t.Fatalf("content = %q", got)
	}
}

func TestUserMessageCodecRoundTrip(t *testing.T) {
	for _, msg := range []UserMessage{
		NewUserText("plain", 1),
		{Content: ContentList{TextContent{Text: "part"}, ImageContent{Data: "AA", MimeType: "image/png"}}, Timestamp: 2},
	} {
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		decoded, err := UnmarshalMessage(raw)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got, ok := decoded.(UserMessage)
		if !ok {
			t.Fatalf("decoded type %T", decoded)
		}
		if got.Timestamp != msg.Timestamp {
			t.Fatalf("timestamp = %d, want %d", got.Timestamp, msg.Timestamp)
		}
	}
}

func TestAssistantAndToolResultCodec(t *testing.T) {
	endTurn := false
	assistant := AssistantMessage{
		Content:    ContentList{TextContent{Text: "hi"}, ToolCall{ID: "c1", Name: "read", Arguments: JsonObject{"path": "x"}}},
		Api:        APIAnthropicMessages,
		Provider:   ProviderAnthropic,
		Model:      "claude-test",
		Usage:      Usage{Input: 1, Output: 2, TotalTokens: 3},
		StopReason: StopToolUse,
		EndTurn:    &endTurn,
		Timestamp:  10,
	}
	raw, err := json.Marshal(assistant)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := UnmarshalMessage(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := decoded.(AssistantMessage)
	if _, isToolCall := got.Content[1].(ToolCall); !isToolCall {
		t.Fatalf("tool call not decoded: %#v", got.Content[1])
	}
	if got.EndTurn == nil || *got.EndTurn {
		t.Fatalf("EndTurn = %v, want explicit false", got.EndTurn)
	}

	toolResult := ToolResultMessage{ToolCallID: "c1", ToolName: "read", Content: ContentList{TextContent{Text: "ok"}}, IsError: false, Timestamp: 11}
	traw, err := json.Marshal(toolResult)
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}
	tdecoded, err := UnmarshalMessage(traw)
	if err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if tdecoded.MessageRole() != RoleToolResult {
		t.Fatalf("role = %q", tdecoded.MessageRole())
	}
}

func TestContentListRejectsUnknownType(t *testing.T) {
	var cl ContentList
	if err := json.Unmarshal([]byte(`[{"type":"bogus"}]`), &cl); err == nil {
		t.Fatal("expected an error for an unknown content type")
	}
}

func TestCloneIsolation(t *testing.T) {
	original := AssistantMessage{
		Content: ContentList{ToolCall{ID: "c1", Name: "read", Arguments: JsonObject{"nested": []any{"a", "b"}}}},
		Usage:   Usage{Input: 1},
	}
	clone := original.Clone()
	call := clone.Content[0].(ToolCall)
	call.Arguments["nested"].([]any)[0] = "changed"
	call.Arguments["new"] = "value"

	if got := original.Content[0].(ToolCall).Arguments["nested"].([]any)[0]; got != "a" {
		t.Fatalf("clone mutated original: %v", got)
	}
	if _, present := original.Content[0].(ToolCall).Arguments["new"]; present {
		t.Fatal("clone added a key to the original")
	}
}

func TestCloneMessages(t *testing.T) {
	original := []Message{NewUserText("hello", 1)}
	clone := CloneMessages(original)
	clone[0].(UserMessage).Content[0] = TextContent{Text: "changed"}
	if original[0].(UserMessage).Content[0].(TextContent).Text != "hello" {
		t.Fatal("CloneMessages did not isolate content")
	}
}

func TestModelCloneIsolation(t *testing.T) {
	model := Model{
		ID:               "m1",
		ThinkingLevelMap: ThinkingLevelMap{ModelThinkingLevel("high"): new("high")},
		Headers:          ProviderHeaders{"X": new("1")},
		SamplingParams:   map[string]any{"top_p": 1.0},
	}
	clone := model.Clone()
	*clone.ThinkingLevelMap[ModelThinkingLevel("high")] = "changed"
	*clone.Headers["X"] = "changed"
	clone.SamplingParams["top_p"] = 0.5

	if *model.ThinkingLevelMap[ModelThinkingLevel("high")] != "high" {
		t.Fatal("clone mutated ThinkingLevelMap")
	}
	if *model.Headers["X"] != "1" {
		t.Fatal("clone mutated Headers")
	}
	if model.SamplingParams["top_p"] != 1.0 {
		t.Fatal("clone mutated SamplingParams")
	}
}

//go:fix inline
