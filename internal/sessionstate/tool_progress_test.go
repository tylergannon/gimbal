package sessionstate

import "testing"

func TestToolProgressAfterCompletionRetainsNestedTranscript(t *testing.T) {
	projection := New(NewProjectionState())
	for _, event := range []*Obj{
		nativeEvent("step", "session.step.started", "sessionID", "ses", "assistantMessageID", "message", "agent", "codex", "model", obj("providerID", "openai", "id", "model")),
		nativeEvent("input-started", "session.tool.input.started", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "name", "collabAgentToolCall"),
		nativeEvent("input-ended", "session.tool.input.ended", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "text", "{}"),
		nativeEvent("called", "session.tool.called", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "input", NewObj(), "executed", true),
		nativeEvent("progress-one", "session.tool.progress", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "metadata", obj("mode", "append", "transcript", []any{obj("type", "session.text.delta", "data", obj("delta", "one"))})),
		nativeEvent("success", "session.tool.success", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "content", []any{obj("type", "text", "text", "launched"), obj("type", "transcript", "events", []any{obj("type", "session.text.delta", "data", obj("delta", "one"))})}, "executed", true),
		nativeEvent("progress-two", "session.tool.progress", "sessionID", "ses", "assistantMessageID", "message", "id", "tool", "metadata", obj("mode", "append", "transcript", []any{obj("type", "session.text.delta", "data", obj("delta", "two"))})),
	} {
		projection.Apply(event)
	}

	message := projection.message("ses", "message")
	tool := objOf(arrOf(message.Get("content"))[0])
	requireJSON(t, "completed tool transcript", objOf(tool.Get("state")).Get("metadata"), `{"transcript":[{"type":"session.text.delta","data":{"delta":"one"}},{"type":"session.text.delta","data":{"delta":"two"}}]}`)
}
