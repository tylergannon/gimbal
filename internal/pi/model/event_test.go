package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAssistantMessageEventTerminalFields(t *testing.T) {
	message := &AssistantMessage{StopReason: StopStop, Timestamp: 1}
	done := AssistantMessageEvent{Type: EventDone, Reason: StopStop, Message: message}
	raw, err := json.Marshal(done)
	if err != nil {
		t.Fatalf("marshal done: %v", err)
	}
	if strings.Contains(string(raw), "contentIndex") {
		t.Fatalf("done should not carry contentIndex: %s", raw)
	}
	if !strings.Contains(string(raw), `"reason":"stop"`) || !strings.Contains(string(raw), `"type":"done"`) {
		t.Fatalf("done missing terminal fields: %s", raw)
	}

	errorMessage := &AssistantMessage{StopReason: StopError, ErrorMessage: "boom", Timestamp: 2}
	failed := AssistantMessageEvent{Type: EventError, Reason: StopError, Error: errorMessage}
	raw, err = json.Marshal(failed)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(raw), `"errorMessage":"boom"`) {
		t.Fatalf("error event missing error message: %s", raw)
	}
	if strings.Contains(string(raw), `"message"`) {
		t.Fatalf("error event should not carry message: %s", raw)
	}
}

func TestAssistantMessageEventDelta(t *testing.T) {
	partial := &AssistantMessage{StopReason: StopPending, Timestamp: 1}
	delta := AssistantMessageEvent{Type: EventTextDelta, ContentIndex: 1, Delta: "hi", Partial: partial}
	raw, err := json.Marshal(delta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"type":"text_delta"`, `"contentIndex":1`, `"delta":"hi"`, `"partial"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("delta event missing %s: %s", want, raw)
		}
	}
}

func TestSessionEntryCodecRoundTrip(t *testing.T) {
	parent := "p1"
	entries := []SessionEntry{
		NewSessionMessageEntry("e1", &parent, "2025-01-01T00:00:00Z", NewUserText("hi", 1)),
		ThinkingLevelChangeEntry{Type: "thinking_level_change", ID: "e2", ThinkingLevel: "high"},
		ModelChangeEntry{Type: "model_change", ID: "e3", Provider: "anthropic", ModelID: "claude"},
		UsageEntry{Type: "usage", ID: "e4", Kind: "cache_warm", Provider: "anthropic", Model: "claude", Usage: Usage{Input: 1}},
		CompactionEntry{Type: "compaction", ID: "e5", Summary: "s", FirstKeptEntryID: "e1", TokensBefore: 10},
		BranchSummaryEntry{Type: "branch_summary", ID: "e6", FromID: "e1", Summary: "bs"},
		CustomEntry{Type: "custom", ID: "e7", CustomType: "state", Data: map[string]any{"a": 1.0}},
		LabelEntry{Type: "label", ID: "e8", TargetID: "e1", Label: new("bookmark")},
		SessionInfoEntry{Type: "session_info", ID: "e9", Name: "named"},
	}
	for _, entry := range entries {
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal %s: %v", entry.EntryType(), err)
		}
		decoded, err := UnmarshalSessionEntry(raw)
		if err != nil {
			t.Fatalf("unmarshal %s: %v", entry.EntryType(), err)
		}
		if decoded.EntryType() != entry.EntryType() {
			t.Fatalf("type = %q, want %q", decoded.EntryType(), entry.EntryType())
		}
	}
}

func TestSessionMessageEntryCustomMessage(t *testing.T) {
	entry := NewSessionMessageEntry("e1", nil, "2025-01-01T00:00:00Z", NewCustomText("note", "hi", true, nil, 1))
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := UnmarshalSessionEntry(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	messageEntry, ok := decoded.(*SessionMessageEntry)
	if !ok {
		t.Fatalf("entry type %T", decoded)
	}
	custom, ok := messageEntry.Message.(CustomMessage)
	if !ok {
		t.Fatalf("message type %T", messageEntry.Message)
	}
	if custom.CustomType != "note" {
		t.Fatalf("customType = %q", custom.CustomType)
	}
}

func TestUnmarshalSessionEntryRejectsUnknownType(t *testing.T) {
	if _, err := UnmarshalSessionEntry([]byte(`{"type":"mystery","id":"1"}`)); err == nil {
		t.Fatal("expected an error for an unknown entry type")
	}
}

func TestSessionHeaderVersion(t *testing.T) {
	header := NewSessionHeader("id1", "2025-01-01T00:00:00Z", "/tmp")
	if header.Version == nil || *header.Version != CurrentSessionVersion {
		t.Fatalf("version = %v", header.Version)
	}
	raw, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := UnmarshalFileEntry(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := decoded.(SessionHeader)
	if !ok {
		t.Fatalf("decoded type %T", decoded)
	}
	if got.ID != "id1" {
		t.Fatalf("id = %q", got.ID)
	}
}

func TestSchemaCodecRoundTrip(t *testing.T) {
	schema := Object(
		Prop("path", String()),
		Opt("limit", Integer()),
	)
	schema.Description = "read a file"
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Schema
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "object" || decoded.Description != "read a file" {
		t.Fatalf("decoded = %#v", decoded)
	}
	if got := decoded.OrderedProperties(); len(got) != 2 || got[0] != "path" || got[1] != "limit" {
		t.Fatalf("properties = %v", got)
	}
	if len(decoded.Required) != 1 || decoded.Required[0] != "path" {
		t.Fatalf("required = %v", decoded.Required)
	}
	if !decoded.Check(map[string]any{"path": "x"}) {
		t.Fatal("schema rejected valid object")
	}
	if decoded.Check(map[string]any{"limit": 1.0}) {
		t.Fatal("schema accepted object missing a required property")
	}
}
