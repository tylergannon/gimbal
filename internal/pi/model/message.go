package model

import (
	"encoding/json"
	"fmt"
)

// DiagnosticErrorInfo describes an error captured in a diagnostic.
type DiagnosticErrorInfo struct {
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
	Code    any    `json:"code,omitempty"`
}

// AssistantMessageDiagnostic is a redacted provider/runtime diagnostic.
type AssistantMessageDiagnostic struct {
	Type      string               `json:"type"`
	Timestamp int64                `json:"timestamp"`
	Error     *DiagnosticErrorInfo `json:"error,omitempty"`
	Details   JsonObject           `json:"details,omitempty"`
}

// Message is a SystemMessage, UserMessage, AssistantMessage or
// ToolResultMessage.
type Message interface {
	MessageRole() Role
}

// SystemMessage carries system instructions and tool declarations at one point
// in the transcript.
type SystemMessage struct {
	// Content is instruction text. On the leading message this is the base
	// prompt; later, additional instructions.
	Content ContentList
	// Sections are named, ordered prompt sections rendered after Content.
	Sections SystemSections
	// ToolsAdded holds complete definitions of tools that become available.
	ToolsAdded []Tool
	// ToolsRemoved names tools that stop being available.
	ToolsRemoved []ToolReference
	// Timestamp is the Unix timestamp in milliseconds.
	Timestamp int64

	// contentWasString records that Content is pi's string form.
	contentWasString bool
}

// MessageRole implements Message.
func (SystemMessage) MessageRole() Role { return RoleSystem }

// NewSystemText builds a system message whose content is the plain-string form.
func NewSystemText(text string, timestamp int64) SystemMessage {
	return SystemMessage{Content: ContentList{TextContent{Text: text}}, Timestamp: timestamp, contentWasString: true}
}

// StringContent reports whether the message's content is the plain-string form.
func (m SystemMessage) StringContent() (string, bool) {
	if m.Content == nil {
		return "", true
	}
	if m.contentWasString && len(m.Content) == 1 {
		if t, ok := m.Content[0].(TextContent); ok {
			return t.Text, true
		}
	}
	return "", false
}

// MarshalJSON writes the role discriminator and the message's fields.
func (m SystemMessage) MarshalJSON() ([]byte, error) {
	var content any
	if s, ok := m.StringContent(); ok {
		content = s
	} else {
		content = m.Content
	}
	return json.Marshal(struct {
		Role         Role            `json:"role"`
		Content      any             `json:"content"`
		Sections     SystemSections  `json:"sections,omitempty"`
		ToolsAdded   []Tool          `json:"toolsAdded,omitempty"`
		ToolsRemoved []ToolReference `json:"toolsRemoved,omitempty"`
		Timestamp    int64           `json:"timestamp"`
	}{RoleSystem, content, m.Sections, m.ToolsAdded, m.ToolsRemoved, m.Timestamp})
}

// UnmarshalJSON accepts content as a string, a text-block array, or null.
func (m *SystemMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		Content      json.RawMessage `json:"content"`
		Sections     SystemSections  `json:"sections"`
		ToolsAdded   []Tool          `json:"toolsAdded"`
		ToolsRemoved []ToolReference `json:"toolsRemoved"`
		Timestamp    int64           `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := SystemMessage{
		Sections:     raw.Sections,
		ToolsAdded:   raw.ToolsAdded,
		ToolsRemoved: raw.ToolsRemoved,
		Timestamp:    raw.Timestamp,
	}
	switch {
	case len(raw.Content) == 0 || string(trimSpace(raw.Content)) == "null":
		out.Content = nil
		out.contentWasString = true
	case raw.Content[0] == '"':
		var s string
		if err := json.Unmarshal(raw.Content, &s); err != nil {
			return err
		}
		out.Content = ContentList{TextContent{Text: s}}
		out.contentWasString = true
	default:
		if err := json.Unmarshal(raw.Content, &out.Content); err != nil {
			return fmt.Errorf("model: system message content: %w", err)
		}
	}
	*m = out
	return nil
}

// Clone returns a deep copy of the message.
func (m SystemMessage) Clone() SystemMessage {
	out := m
	out.Content = m.Content.Clone()
	out.Sections = m.Sections.Clone()
	out.ToolsAdded = cloneTools(m.ToolsAdded)
	out.ToolsRemoved = append([]ToolReference(nil), m.ToolsRemoved...)
	return out
}

// UserMessage is a message authored by the user.
type UserMessage struct {
	Content   ContentList `json:"content"`
	Timestamp int64       `json:"timestamp"`

	contentWasString bool
}

// MessageRole implements Message.
func (UserMessage) MessageRole() Role { return RoleUser }

// StringContent reports whether the message's content was the plain-string
// form, returning that string.
func (m UserMessage) StringContent() (string, bool) {
	if m.contentWasString && len(m.Content) == 1 {
		if t, ok := m.Content[0].(TextContent); ok {
			return t.Text, true
		}
	}
	return "", false
}

// NewUserText builds a user message from plain text in string form.
func NewUserText(text string, timestamp int64) UserMessage {
	return UserMessage{Content: ContentList{TextContent{Text: text}}, Timestamp: timestamp, contentWasString: true}
}

// MarshalJSON adds the role discriminator, re-emitting string content.
func (m UserMessage) MarshalJSON() ([]byte, error) {
	if s, ok := m.StringContent(); ok {
		return json.Marshal(struct {
			Role      Role   `json:"role"`
			Content   string `json:"content"`
			Timestamp int64  `json:"timestamp"`
		}{RoleUser, s, m.Timestamp})
	}
	type alias UserMessage
	return json.Marshal(struct {
		Role Role `json:"role"`
		alias
	}{RoleUser, alias(m)})
}

// UnmarshalJSON accepts content as either a string or a discriminated array.
func (m *UserMessage) UnmarshalJSON(data []byte) error {
	var probe struct {
		Content   json.RawMessage `json:"content"`
		Timestamp int64           `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	m.Timestamp = probe.Timestamp
	m.contentWasString = false
	if len(probe.Content) == 0 || string(trimSpace(probe.Content)) == "null" {
		m.Content = nil
		return nil
	}
	if probe.Content[0] == '"' {
		var s string
		if err := json.Unmarshal(probe.Content, &s); err != nil {
			return err
		}
		m.Content = ContentList{TextContent{Text: s}}
		m.contentWasString = true
		return nil
	}
	return json.Unmarshal(probe.Content, &m.Content)
}

// Clone returns a deep copy of the message.
func (m UserMessage) Clone() UserMessage {
	out := m
	out.Content = m.Content.Clone()
	return out
}

// AssistantMessage is a message authored by the model.
type AssistantMessage struct {
	Content  ContentList `json:"content"`
	Api      Api         `json:"api"`
	Provider ProviderId  `json:"provider"`
	Model    string      `json:"model"`
	// ResponseModel is the concrete model the provider reported when it differs
	// from the requested Model.
	ResponseModel string `json:"responseModel,omitempty"`
	// ResponseID is the provider's own response/message identifier.
	ResponseID string `json:"responseId,omitempty"`
	// ProviderThinkingLevel is the exact provider-native effort level.
	ProviderThinkingLevel string                       `json:"providerThinkingLevel,omitempty"`
	Diagnostics           []AssistantMessageDiagnostic `json:"diagnostics,omitempty"`
	Usage                 Usage                        `json:"usage"`
	StopReason            StopReason                   `json:"stopReason"`
	Deferred              *DeferredHandle              `json:"deferred,omitempty"`
	ErrorMessage          string                       `json:"errorMessage,omitempty"`
	RawStopReason         string                       `json:"rawStopReason,omitempty"`
	// EndTurn is the provider's own indication of whether the model explicitly
	// ended its turn.
	EndTurn   *bool `json:"endTurn,omitempty"`
	Timestamp int64 `json:"timestamp"`
}

// MessageRole implements Message.
func (AssistantMessage) MessageRole() Role { return RoleAssistant }

// MarshalJSON adds the role discriminator.
func (m AssistantMessage) MarshalJSON() ([]byte, error) {
	type alias AssistantMessage
	return json.Marshal(struct {
		Role Role `json:"role"`
		alias
	}{RoleAssistant, alias(m)})
}

// Clone returns a deep copy of the message.
func (m AssistantMessage) Clone() AssistantMessage {
	out := m
	out.Content = m.Content.Clone()
	if m.Diagnostics != nil {
		out.Diagnostics = make([]AssistantMessageDiagnostic, len(m.Diagnostics))
		for i, d := range m.Diagnostics {
			out.Diagnostics[i] = d
			out.Diagnostics[i].Details = cloneMap(d.Details)
			if d.Error != nil {
				e := *d.Error
				out.Diagnostics[i].Error = &e
			}
		}
	}
	if m.Deferred != nil {
		d := *m.Deferred
		d.Data = cloneValue(d.Data)
		out.Deferred = &d
	}
	if m.EndTurn != nil {
		v := *m.EndTurn
		out.EndTurn = &v
	}
	return out
}

// ToolResultMessage is the result of executing a tool call.
type ToolResultMessage struct {
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Content    ContentList `json:"content"`
	Details    any         `json:"details,omitempty"`
	Usage      *Usage      `json:"usage,omitempty"`
	IsError    bool        `json:"isError"`
	Timestamp  int64       `json:"timestamp"`
}

// MessageRole implements Message.
func (ToolResultMessage) MessageRole() Role { return RoleToolResult }

// MarshalJSON adds the role discriminator.
func (m ToolResultMessage) MarshalJSON() ([]byte, error) {
	type alias ToolResultMessage
	return json.Marshal(struct {
		Role Role `json:"role"`
		alias
	}{RoleToolResult, alias(m)})
}

// Clone returns a deep copy of the message.
func (m ToolResultMessage) Clone() ToolResultMessage {
	out := m
	out.Content = m.Content.Clone()
	out.Details = cloneValue(m.Details)
	if m.Usage != nil {
		u := *m.Usage
		out.Usage = &u
	}
	return out
}

// UnmarshalMessage decodes a Message from JSON based on its "role".
func UnmarshalMessage(data []byte) (Message, error) {
	var head struct {
		Role Role `json:"role"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	switch head.Role {
	case RoleSystem:
		var m SystemMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleUser:
		var m UserMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleAssistant:
		var m AssistantMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleToolResult:
		var m ToolResultMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleBashExecution:
		var m BashExecutionMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleCustom:
		var m CustomMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleBranchSummary:
		var m BranchSummaryMessage
		err := json.Unmarshal(data, &m)
		return m, err
	case RoleCompactionSummary:
		var m CompactionSummaryMessage
		err := json.Unmarshal(data, &m)
		return m, err
	default:
		return nil, fmt.Errorf("model: unknown message role: %q", head.Role)
	}
}

// UnmarshalMessages decodes a JSON array of messages.
func UnmarshalMessages(data []byte) ([]Message, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	return unmarshalMessages(raws)
}

func unmarshalMessages(raws []json.RawMessage) ([]Message, error) {
	out := make([]Message, 0, len(raws))
	for _, raw := range raws {
		m, err := UnmarshalMessage(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// MarshalMessages encodes a message slice as JSON.
func MarshalMessages(messages []Message) ([]byte, error) {
	return json.Marshal(messages)
}

// MessageText returns the text content of a message, or "" for roles without
// text.
func MessageText(m Message) string {
	switch t := m.(type) {
	case SystemMessage:
		return ContentText(t.Content, "\n")
	case *SystemMessage:
		return ContentText(t.Content, "\n")
	case UserMessage:
		return ContentText(t.Content, "\n")
	case *UserMessage:
		return ContentText(t.Content, "\n")
	case AssistantMessage:
		return ContentText(t.Content, "\n")
	case *AssistantMessage:
		return ContentText(t.Content, "\n")
	case ToolResultMessage:
		return ContentText(t.Content, "\n")
	case *ToolResultMessage:
		return ContentText(t.Content, "\n")
	default:
		return ""
	}
}

func cloneTools(tools []Tool) []Tool {
	if tools == nil {
		return nil
	}
	out := make([]Tool, len(tools))
	for i, t := range tools {
		out[i] = t.Clone()
	}
	return out
}

// CloneMessage returns a deep copy of any message value.
func CloneMessage(m Message) Message {
	switch t := m.(type) {
	case SystemMessage:
		return t.Clone()
	case *SystemMessage:
		if t == nil {
			return (*SystemMessage)(nil)
		}
		v := t.Clone()
		return &v
	case UserMessage:
		return t.Clone()
	case *UserMessage:
		if t == nil {
			return (*UserMessage)(nil)
		}
		v := t.Clone()
		return &v
	case AssistantMessage:
		return t.Clone()
	case *AssistantMessage:
		if t == nil {
			return (*AssistantMessage)(nil)
		}
		v := t.Clone()
		return &v
	case ToolResultMessage:
		return t.Clone()
	case *ToolResultMessage:
		if t == nil {
			return (*ToolResultMessage)(nil)
		}
		v := t.Clone()
		return &v
	case BashExecutionMessage:
		out := t
		if t.ExitCode != nil {
			c := *t.ExitCode
			out.ExitCode = &c
		}
		if t.FullOutputPath != nil {
			p := *t.FullOutputPath
			out.FullOutputPath = &p
		}
		return out
	case *BashExecutionMessage:
		if t == nil {
			return (*BashExecutionMessage)(nil)
		}
		v := CloneMessage(*t).(BashExecutionMessage)
		return &v
	case CustomMessage:
		out := t
		out.Content = t.Content.Clone()
		out.Details = cloneValue(t.Details)
		return out
	case *CustomMessage:
		if t == nil {
			return (*CustomMessage)(nil)
		}
		v := CloneMessage(*t).(CustomMessage)
		return &v
	case BranchSummaryMessage:
		out := t
		if t.FromID != nil {
			f := *t.FromID
			out.FromID = &f
		}
		return out
	case *BranchSummaryMessage:
		if t == nil {
			return (*BranchSummaryMessage)(nil)
		}
		v := CloneMessage(*t).(BranchSummaryMessage)
		return &v
	case CompactionSummaryMessage:
		return t
	case *CompactionSummaryMessage:
		if t == nil {
			return (*CompactionSummaryMessage)(nil)
		}
		v := *t
		return &v
	default:
		return m
	}
}

// CloneMessages returns a deep copy of a message slice.
func CloneMessages(messages []Message) []Message {
	if messages == nil {
		return nil
	}
	out := make([]Message, len(messages))
	for i, m := range messages {
		out[i] = CloneMessage(m)
	}
	return out
}
