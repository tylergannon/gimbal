package model

import "encoding/json"

// EventType is the discriminator for AssistantMessageEvent.
type EventType string

const (
	EventStart         EventType = "start"
	EventTextStart     EventType = "text_start"
	EventTextDelta     EventType = "text_delta"
	EventTextEnd       EventType = "text_end"
	EventThinkingStart EventType = "thinking_start"
	EventThinkingDelta EventType = "thinking_delta"
	EventThinkingEnd   EventType = "thinking_end"
	EventToolCallStart EventType = "toolcall_start"
	EventToolCallDelta EventType = "toolcall_delta"
	EventToolCallEnd   EventType = "toolcall_end"
	EventDone          EventType = "done"
	EventError         EventType = "error"
)

// AssistantMessageEvent is one event in the streaming protocol. It is a flat
// struct carrying the union of fields used by pi's variants.
type AssistantMessageEvent struct {
	Type EventType `json:"type"`
	// ContentIndex is set for per-block events.
	ContentIndex int `json:"contentIndex"`
	// Delta is the incremental text for *_delta events.
	Delta string `json:"delta"`
	// Content is the finished text for text_end / thinking_end events.
	Content string `json:"content"`
	// ToolCall is the finished tool call for toolcall_end events.
	ToolCall *ToolCall `json:"toolCall,omitempty"`
	// Partial is the in-progress assistant message.
	Partial *AssistantMessage `json:"partial,omitempty"`
	// Reason is the stop reason for done/error events.
	Reason StopReason `json:"reason"`
	// Message is the final assistant message for "done" events.
	Message *AssistantMessage `json:"message,omitempty"`
	// Error is the final assistant message for "error" events.
	Error *AssistantMessage `json:"error,omitempty"`
}

// MarshalJSON serializes the event with exactly the fields pi's corresponding
// union variant carries.
func (e AssistantMessageEvent) MarshalJSON() ([]byte, error) {
	switch e.Type {
	case EventStart:
		return json.Marshal(struct {
			Type    EventType         `json:"type"`
			Partial *AssistantMessage `json:"partial"`
		}{e.Type, e.Partial})
	case EventTextStart, EventThinkingStart, EventToolCallStart:
		return json.Marshal(struct {
			Type         EventType         `json:"type"`
			ContentIndex int               `json:"contentIndex"`
			Partial      *AssistantMessage `json:"partial"`
		}{e.Type, e.ContentIndex, e.Partial})
	case EventTextDelta, EventThinkingDelta, EventToolCallDelta:
		return json.Marshal(struct {
			Type         EventType         `json:"type"`
			ContentIndex int               `json:"contentIndex"`
			Delta        string            `json:"delta"`
			Partial      *AssistantMessage `json:"partial"`
		}{e.Type, e.ContentIndex, e.Delta, e.Partial})
	case EventTextEnd, EventThinkingEnd:
		return json.Marshal(struct {
			Type         EventType         `json:"type"`
			ContentIndex int               `json:"contentIndex"`
			Content      string            `json:"content"`
			Partial      *AssistantMessage `json:"partial"`
		}{e.Type, e.ContentIndex, e.Content, e.Partial})
	case EventToolCallEnd:
		return json.Marshal(struct {
			Type         EventType         `json:"type"`
			ContentIndex int               `json:"contentIndex"`
			ToolCall     *ToolCall         `json:"toolCall"`
			Partial      *AssistantMessage `json:"partial"`
		}{e.Type, e.ContentIndex, e.ToolCall, e.Partial})
	case EventDone:
		return json.Marshal(struct {
			Type    EventType         `json:"type"`
			Reason  StopReason        `json:"reason"`
			Message *AssistantMessage `json:"message"`
		}{e.Type, e.Reason, e.Message})
	case EventError:
		return json.Marshal(struct {
			Type   EventType         `json:"type"`
			Reason StopReason        `json:"reason"`
			Error  *AssistantMessage `json:"error"`
		}{e.Type, e.Reason, e.Error})
	default:
		type alias AssistantMessageEvent
		return json.Marshal(alias(e))
	}
}

// UnmarshalJSON decodes any assistant message event variant.
func (e *AssistantMessageEvent) UnmarshalJSON(data []byte) error {
	var head struct {
		Type EventType `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	type alias AssistantMessageEvent
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = AssistantMessageEvent(raw)
	return nil
}
