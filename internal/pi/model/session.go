package model

import (
	"encoding/json"
	"fmt"
)

// CurrentSessionVersion is the current pi session file version.
const CurrentSessionVersion = 3

// SessionHeader is the first line of a session file.
type SessionHeader struct {
	Type          string `json:"type"`
	Version       *int   `json:"version,omitempty"`
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	Cwd           string `json:"cwd"`
	ParentSession string `json:"parentSession,omitempty"`
}

// NewSessionHeader builds a session header with the current version.
func NewSessionHeader(id, timestamp, cwd string) SessionHeader {
	version := CurrentSessionVersion
	return SessionHeader{Type: "session", Version: &version, ID: id, Timestamp: timestamp, Cwd: cwd}
}

// SessionEntryBase carries the tree fields of every session entry.
type SessionEntryBase struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	ParentID  *string `json:"parentId"`
	Timestamp string  `json:"timestamp"`
}

// Base returns the entry's base fields.
func (b SessionEntryBase) Base() SessionEntryBase { return b }

// EntryType returns the entry's discriminating type.
func (b SessionEntryBase) EntryType() string { return b.Type }

// SessionEntry is one append-only entry in a session file.
type SessionEntry interface {
	EntryType() string
	Base() SessionEntryBase
}

// SessionMessageEntry stores an agent message.
type SessionMessageEntry struct {
	SessionEntryBase
	Message AgentMessage `json:"message"`
}

// NewSessionMessageEntry builds a message entry.
func NewSessionMessageEntry(id string, parentID *string, timestamp string, message AgentMessage) SessionMessageEntry {
	return SessionMessageEntry{
		Type: "message", ID: id, ParentID: parentID, Timestamp: timestamp,
		Message: message,
	}
}

// MarshalJSON adds the type discriminator.
func (e SessionMessageEntry) MarshalJSON() ([]byte, error) {
	e.Type = "message"
	type alias SessionMessageEntry
	return json.Marshal(alias(e))
}

// UnmarshalJSON decodes a message entry.
func (e *SessionMessageEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type      string          `json:"type"`
		ID        string          `json:"id"`
		ParentID  *string         `json:"parentId"`
		Timestamp string          `json:"timestamp"`
		Message   json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	message, err := UnmarshalMessage(raw.Message)
	if err != nil {
		return err
	}
	e.SessionEntryBase = SessionEntryBase{Type: "message", ID: raw.ID, ParentID: raw.ParentID, Timestamp: raw.Timestamp}
	e.Message = message
	return nil
}

// ThinkingLevelChangeEntry records a thinking-level change.
type ThinkingLevelChangeEntry struct {
	SessionEntryBase
	ThinkingLevel string `json:"thinkingLevel"`
}

// MarshalJSON adds the type discriminator.
func (e ThinkingLevelChangeEntry) MarshalJSON() ([]byte, error) {
	e.Type = "thinking_level_change"
	type alias ThinkingLevelChangeEntry
	return json.Marshal(alias(e))
}

// ModelChangeEntry records a model change.
type ModelChangeEntry struct {
	SessionEntryBase
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// MarshalJSON adds the type discriminator.
func (e ModelChangeEntry) MarshalJSON() ([]byte, error) {
	e.Type = "model_change"
	type alias ModelChangeEntry
	return json.Marshal(alias(e))
}

// UsageEntry records usage for an arbitrary category.
type UsageEntry struct {
	SessionEntryBase
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Usage    Usage  `json:"usage"`
	Note     string `json:"note,omitempty"`
}

// MarshalJSON adds the type discriminator.
func (e UsageEntry) MarshalJSON() ([]byte, error) {
	e.Type = "usage"
	type alias UsageEntry
	return json.Marshal(alias(e))
}

// CompactionEntry records a compaction boundary.
type CompactionEntry struct {
	SessionEntryBase
	Summary          string         `json:"summary"`
	FirstKeptEntryID string         `json:"firstKeptEntryId"`
	TokensBefore     int            `json:"tokensBefore"`
	Details          any            `json:"details,omitempty"`
	Usage            *Usage         `json:"usage,omitempty"`
	FromHook         bool           `json:"fromHook,omitempty"`
	SystemMessage    *SystemMessage `json:"systemMessage,omitempty"`
}

// MarshalJSON adds the type discriminator.
func (e CompactionEntry) MarshalJSON() ([]byte, error) {
	e.Type = "compaction"
	type alias CompactionEntry
	return json.Marshal(alias(e))
}

// BranchSummaryEntry records a branch summary.
type BranchSummaryEntry struct {
	SessionEntryBase
	FromID   string `json:"fromId"`
	Summary  string `json:"summary"`
	Details  any    `json:"details,omitempty"`
	Usage    *Usage `json:"usage,omitempty"`
	FromHook bool   `json:"fromHook,omitempty"`
}

// MarshalJSON adds the type discriminator.
func (e BranchSummaryEntry) MarshalJSON() ([]byte, error) {
	e.Type = "branch_summary"
	type alias BranchSummaryEntry
	return json.Marshal(alias(e))
}

// CustomEntry stores extension-specific data. It does not participate in LLM
// context.
type CustomEntry struct {
	SessionEntryBase
	CustomType string `json:"customType"`
	Data       any    `json:"data,omitempty"`
}

// MarshalJSON adds the type discriminator.
func (e CustomEntry) MarshalJSON() ([]byte, error) {
	e.Type = "custom"
	type alias CustomEntry
	return json.Marshal(alias(e))
}

// LabelEntry is a user-defined bookmark on an entry.
type LabelEntry struct {
	SessionEntryBase
	TargetID string  `json:"targetId"`
	Label    *string `json:"label"`
}

// MarshalJSON adds the type discriminator.
func (e LabelEntry) MarshalJSON() ([]byte, error) {
	e.Type = "label"
	type alias LabelEntry
	return json.Marshal(alias(e))
}

// SessionInfoEntry is session metadata such as a display name.
type SessionInfoEntry struct {
	SessionEntryBase
	Name string `json:"name,omitempty"`
}

// MarshalJSON adds the type discriminator.
func (e SessionInfoEntry) MarshalJSON() ([]byte, error) {
	e.Type = "session_info"
	type alias SessionInfoEntry
	return json.Marshal(alias(e))
}

// CustomMessageEntry injects an extension message into LLM context.
type CustomMessageEntry struct {
	SessionEntryBase
	CustomType string      `json:"customType"`
	Content    ContentList `json:"content"`
	Details    any         `json:"details,omitempty"`
	Display    bool        `json:"display"`

	contentWasString bool
}

// MarshalJSON preserves the string content form.
func (e CustomMessageEntry) MarshalJSON() ([]byte, error) {
	e.Type = "custom_message"
	var content any
	if e.contentWasString && len(e.Content) == 1 {
		if t, ok := e.Content[0].(TextContent); ok {
			content = t.Text
		} else {
			content = e.Content
		}
	} else {
		content = e.Content
	}
	return json.Marshal(struct {
		Type       string  `json:"type"`
		ID         string  `json:"id"`
		ParentID   *string `json:"parentId"`
		Timestamp  string  `json:"timestamp"`
		CustomType string  `json:"customType"`
		Content    any     `json:"content"`
		Details    any     `json:"details,omitempty"`
		Display    bool    `json:"display"`
	}{"custom_message", e.ID, e.ParentID, e.Timestamp, e.CustomType, content, e.Details, e.Display})
}

// UnmarshalJSON accepts content as a string or a discriminated array.
func (e *CustomMessageEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID         string          `json:"id"`
		ParentID   *string         `json:"parentId"`
		Timestamp  string          `json:"timestamp"`
		CustomType string          `json:"customType"`
		Content    json.RawMessage `json:"content"`
		Details    any             `json:"details"`
		Display    bool            `json:"display"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.SessionEntryBase = SessionEntryBase{Type: "custom_message", ID: raw.ID, ParentID: raw.ParentID, Timestamp: raw.Timestamp}
	e.CustomType = raw.CustomType
	e.Details = raw.Details
	e.Display = raw.Display
	e.contentWasString = false
	if len(raw.Content) > 0 && raw.Content[0] == '"' {
		var s string
		if err := json.Unmarshal(raw.Content, &s); err != nil {
			return err
		}
		e.Content = ContentList{TextContent{Text: s}}
		e.contentWasString = true
		return nil
	}
	return json.Unmarshal(raw.Content, &e.Content)
}

// ContextEditableContent is content an append-only context edit may replace.
type ContextEditableContent struct {
	Content ContentList `json:"content"`
}

// ContextEditEntry is an append-only change to one earlier entry's context.
type ContextEditEntry struct {
	SessionEntryBase
	TargetID string `json:"targetId"`
	// Replacement is null to omit the target from context, or a content value
	// to replace only its content.
	Replacement *ContextEditableContent `json:"replacement"`
}

// MarshalJSON adds the type discriminator.
func (e ContextEditEntry) MarshalJSON() ([]byte, error) {
	e.Type = "context_edit"
	type alias ContextEditEntry
	return json.Marshal(alias(e))
}

// NewSessionEntry returns an empty entry value for the given type, or false.
func NewSessionEntry(entryType string) (SessionEntry, bool) {
	switch entryType {
	case "message":
		return &SessionMessageEntry{}, true
	case "thinking_level_change":
		return &ThinkingLevelChangeEntry{}, true
	case "model_change":
		return &ModelChangeEntry{}, true
	case "usage":
		return &UsageEntry{}, true
	case "compaction":
		return &CompactionEntry{}, true
	case "branch_summary":
		return &BranchSummaryEntry{}, true
	case "custom":
		return &CustomEntry{}, true
	case "custom_message":
		return &CustomMessageEntry{}, true
	case "context_edit":
		return &ContextEditEntry{}, true
	case "label":
		return &LabelEntry{}, true
	case "session_info":
		return &SessionInfoEntry{}, true
	default:
		return nil, false
	}
}

// UnmarshalSessionEntry decodes a session entry by its type discriminator.
func UnmarshalSessionEntry(data []byte) (SessionEntry, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	entry, ok := NewSessionEntry(head.Type)
	if !ok {
		return nil, fmt.Errorf("model: unknown session entry type: %q", head.Type)
	}
	if err := json.Unmarshal(data, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// FileEntry is a session header or a session entry.
type FileEntry interface {
	FileEntryType() string
}

// FileEntryType implements FileEntry for the header.
func (SessionHeader) FileEntryType() string { return "session" }

// FileEntryType implements FileEntry for entries.
func (e headerEntry) FileEntryType() string { return e.entry.EntryType() }

type headerEntry struct{ entry SessionEntry }

// Entry returns the wrapped session entry.
func (e headerEntry) Entry() SessionEntry { return e.entry }

// UnmarshalFileEntry decodes a session header or session entry.
func UnmarshalFileEntry(data []byte) (FileEntry, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	if head.Type == "session" {
		var header SessionHeader
		if err := json.Unmarshal(data, &header); err != nil {
			return nil, err
		}
		return header, nil
	}
	entry, err := UnmarshalSessionEntry(data)
	if err != nil {
		return nil, err
	}
	return headerEntry{entry: entry}, nil
}

// SessionModelRef names the provider and model of a session context.
type SessionModelRef struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// SessionContext is the model-visible context of a session.
type SessionContext struct {
	Messages      []AgentMessage   `json:"messages"`
	ThinkingLevel string           `json:"thinkingLevel"`
	Model         *SessionModelRef `json:"model"`
}

// ProjectedSessionEntry is one append-only entry's contribution to context.
type ProjectedSessionEntry struct {
	SourceEntry SessionEntry   `json:"sourceEntry"`
	Messages    []AgentMessage `json:"messages"`
}

// SessionProjection is the projected session state.
type SessionProjection struct {
	Entries       []ProjectedSessionEntry `json:"entries"`
	Messages      []AgentMessage          `json:"messages"`
	ThinkingLevel string                  `json:"thinkingLevel"`
	Model         *SessionModelRef        `json:"model"`
}

// SessionTreeNode is a defensive copy of session structure.
type SessionTreeNode struct {
	Entry          SessionEntry      `json:"entry"`
	Children       []SessionTreeNode `json:"children"`
	Label          *string           `json:"label,omitempty"`
	LabelTimestamp *string           `json:"labelTimestamp,omitempty"`
}

// SessionInfo describes a stored session.
type SessionInfo struct {
	Path              string `json:"path"`
	ID                string `json:"id"`
	Cwd               string `json:"cwd"`
	Name              string `json:"name,omitempty"`
	ParentSessionPath string `json:"parentSessionPath,omitempty"`
	Created           string `json:"created"`
	Modified          string `json:"modified"`
	MessageCount      int    `json:"messageCount"`
	FirstMessage      string `json:"firstMessage"`
	AllMessagesText   string `json:"allMessagesText"`
}
