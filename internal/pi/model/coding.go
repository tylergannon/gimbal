package model

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Exact summary-wrapper text from pi (core/messages.ts). These must match
// byte-for-byte so reconstructed context is identical to pi's.
const (
	CompactionSummaryPrefix = "The conversation history before this point was compacted into the following summary:\n\n<summary>\n"
	CompactionSummarySuffix = "\n</summary>"
	BranchSummaryPrefix     = "The following is a summary of a branch that this conversation came back from:\n\n<summary>\n"
	BranchSummarySuffix     = "</summary>"
)

// Coding-agent custom roles.
const (
	RoleBashExecution     Role = "bashExecution"
	RoleCustom            Role = "custom"
	RoleBranchSummary     Role = "branchSummary"
	RoleCompactionSummary Role = "compactionSummary"
)

// BashExecutionMessage is a bash execution via the ! command.
type BashExecutionMessage struct {
	Command   string `json:"command"`
	Output    string `json:"output"`
	ExitCode  *int   `json:"exitCode"`
	Cancelled bool   `json:"cancelled"`
	Truncated bool   `json:"truncated"`
	// FullOutputPath is set when truncated output was spilled to a file.
	FullOutputPath *string `json:"fullOutputPath,omitempty"`
	Timestamp      int64   `json:"timestamp"`
	// ExcludeFromContext excludes the message from LLM context (!! prefix).
	ExcludeFromContext bool `json:"excludeFromContext,omitempty"`
}

// MessageRole implements Message.
func (BashExecutionMessage) MessageRole() Role { return RoleBashExecution }

// CustomMessage is an extension-injected message.
type CustomMessage struct {
	CustomType string      `json:"customType"`
	Content    ContentList `json:"content"`
	Display    bool        `json:"display"`
	Details    any         `json:"details,omitempty"`
	Timestamp  int64       `json:"timestamp"`

	contentWasString bool
}

// MessageRole implements Message.
func (CustomMessage) MessageRole() Role { return RoleCustom }

// NewCustomText builds a custom message with string content.
func NewCustomText(customType, text string, display bool, details any, timestamp int64) CustomMessage {
	return CustomMessage{
		CustomType:       customType,
		Content:          ContentList{TextContent{Text: text}},
		Display:          display,
		Details:          details,
		Timestamp:        timestamp,
		contentWasString: true,
	}
}

// StringContent reports whether the message's content was the plain-string
// form.
func (m CustomMessage) StringContent() (string, bool) {
	if m.contentWasString && len(m.Content) == 1 {
		if t, ok := m.Content[0].(TextContent); ok {
			return t.Text, true
		}
	}
	return "", false
}

// MarshalJSON writes string content in the string form.
func (m CustomMessage) MarshalJSON() ([]byte, error) {
	if s, ok := m.StringContent(); ok {
		return marshalStruct(map[string]any{
			"role":       RoleCustom,
			"customType": m.CustomType,
			"content":    s,
			"display":    m.Display,
			"details":    m.Details,
			"timestamp":  m.Timestamp,
		}, []string{"role", "customType", "content", "display", "details", "timestamp"})
	}
	type alias CustomMessage
	return marshalWithRole(RoleCustom, alias(m))
}

// UnmarshalJSON accepts content as a string or a discriminated array.
func (m *CustomMessage) UnmarshalJSON(data []byte) error {
	var probe struct {
		CustomType string          `json:"customType"`
		Content    json.RawMessage `json:"content"`
		Display    bool            `json:"display"`
		Details    any             `json:"details"`
		Timestamp  int64           `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	m.CustomType = probe.CustomType
	m.Display = probe.Display
	m.Details = probe.Details
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

// BranchSummaryMessage is a summary of an abandoned branch.
type BranchSummaryMessage struct {
	Summary   string  `json:"summary"`
	FromID    *string `json:"fromId"`
	Timestamp int64   `json:"timestamp"`
}

// MessageRole implements Message.
func (BranchSummaryMessage) MessageRole() Role { return RoleBranchSummary }

// MarshalJSON adds the role discriminator.
func (m BranchSummaryMessage) MarshalJSON() ([]byte, error) {
	type alias BranchSummaryMessage
	return marshalWithRole(RoleBranchSummary, alias(m))
}

// CompactionSummaryMessage is a summary of compacted history.
type CompactionSummaryMessage struct {
	Summary      string `json:"summary"`
	TokensBefore int    `json:"tokensBefore"`
	Timestamp    int64  `json:"timestamp"`
}

// MessageRole implements Message.
func (CompactionSummaryMessage) MessageRole() Role { return RoleCompactionSummary }

// MarshalJSON adds the role discriminator.
func (m CompactionSummaryMessage) MarshalJSON() ([]byte, error) {
	type alias CompactionSummaryMessage
	return marshalWithRole(RoleCompactionSummary, alias(m))
}

// CreateBranchSummaryMessage builds a branch summary message.
func CreateBranchSummaryMessage(summary, fromID string, timestamp int64) BranchSummaryMessage {
	return BranchSummaryMessage{Summary: summary, FromID: &fromID, Timestamp: timestamp}
}

// CreateCompactionSummaryMessage builds a compaction summary message.
func CreateCompactionSummaryMessage(summary string, tokensBefore int, timestamp int64) CompactionSummaryMessage {
	return CompactionSummaryMessage{Summary: summary, TokensBefore: tokensBefore, Timestamp: timestamp}
}

// BashExecutionToText converts a bash execution message to model-visible text.
func BashExecutionToText(msg BashExecutionMessage) string {
	text := "Ran `" + msg.Command + "`\n"
	if msg.Output != "" {
		text += "```\n" + msg.Output + "\n```"
	} else {
		text += "(no output)"
	}
	if msg.Cancelled {
		text += "\n\n(command cancelled)"
	} else if msg.ExitCode != nil && *msg.ExitCode != 0 {
		text += "\n\nCommand exited with code " + itoa(*msg.ExitCode)
	}
	if msg.Truncated && msg.FullOutputPath != nil {
		text += "\n\n[Output truncated. Full output: " + *msg.FullOutputPath + "]"
	}
	return text
}

// ConvertToLlm transforms agent messages into LLM-compatible messages. Custom
// messages are converted; UI-only messages are dropped.
func ConvertToLlm(messages []AgentMessage) []Message {
	out := make([]Message, 0, len(messages))
	for _, m := range messages {
		switch t := m.(type) {
		case BashExecutionMessage:
			if t.ExcludeFromContext {
				continue
			}
			out = append(out, NewUserText(BashExecutionToText(t), t.Timestamp))
		case CustomMessage:
			if s, ok := t.StringContent(); ok {
				out = append(out, NewUserText(s, t.Timestamp))
				continue
			}
			out = append(out, UserMessage{Content: t.Content.Clone(), Timestamp: t.Timestamp})
		case BranchSummaryMessage:
			out = append(out, UserMessage{
				Content:   ContentList{TextContent{Text: BranchSummaryPrefix + t.Summary + BranchSummarySuffix}},
				Timestamp: t.Timestamp,
			})
		case CompactionSummaryMessage:
			out = append(out, UserMessage{
				Content:   ContentList{TextContent{Text: CompactionSummaryPrefix + t.Summary + CompactionSummarySuffix}},
				Timestamp: t.Timestamp,
			})
		case SystemMessage, UserMessage, AssistantMessage, ToolResultMessage:
			out = append(out, m)
		default:
			// Unknown roles are filtered out.
		}
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

func marshalWithRole(role Role, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(`{"role":`)
	r, _ := json.Marshal(role)
	buf.Write(r)
	if fields := bytes.TrimSpace(raw[1 : len(raw)-1]); len(fields) > 0 {
		buf.WriteByte(',')
		buf.Write(fields)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func marshalStruct(fields map[string]any, order []string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range order {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(key)
		buf.Write(k)
		buf.WriteByte(':')
		raw, err := json.Marshal(fields[key])
		if err != nil {
			return nil, err
		}
		buf.Write(raw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
