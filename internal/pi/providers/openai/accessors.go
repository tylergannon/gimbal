package openai

import "github.com/tylergannon/gimbal/internal/pi/model"

// Typed message accessors used by message conversion. They accept both value
// and pointer forms, as internal/pi/model does.

func asSystemMessage(message model.Message) (model.SystemMessage, bool) {
	switch m := message.(type) {
	case model.SystemMessage:
		return m, true
	case *model.SystemMessage:
		if m != nil {
			return *m, true
		}
	}
	return model.SystemMessage{}, false
}

func asUserMessage(message model.Message) (model.UserMessage, bool) {
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

func asAssistantMessage(message model.Message) (model.AssistantMessage, bool) {
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

func asToolResultMessage(message model.Message) (model.ToolResultMessage, bool) {
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

func isSystemRole(message model.Message) bool    { return message.MessageRole() == model.RoleSystem }
func isUserRole(message model.Message) bool      { return message.MessageRole() == model.RoleUser }
func isAssistantRole(message model.Message) bool { return message.MessageRole() == model.RoleAssistant }
func isToolResultRole(message model.Message) bool {
	return message.MessageRole() == model.RoleToolResult
}
