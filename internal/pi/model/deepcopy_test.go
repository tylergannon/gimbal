package model

import "testing"

func TestCloneSessionEntryIsolation(t *testing.T) {
	parent := "p1"
	entry := NewSessionMessageEntry("e1", &parent, "2025-01-01T00:00:00Z", NewUserText("hello", 1))
	clone := CloneSessionEntry(entry)
	msg := clone.(SessionMessageEntry).Message.(UserMessage)
	msg.Content[0] = TextContent{Text: "changed"}

	original := entry.Message.(UserMessage)
	if original.Content[0].(TextContent).Text != "hello" {
		t.Fatal("clone mutated the original message")
	}
	if *clone.Base().ParentID != "p1" {
		t.Fatal("parent id lost")
	}
}

func TestCloneCompactionEntryIsolation(t *testing.T) {
	system := NewSystemText("original", 1)
	entry := CompactionEntry{
		Type: "compaction", ID: "c1",
		Summary:          "s",
		FirstKeptEntryID: "e1",
		Details:          map[string]any{"nested": []any{"a"}},
		SystemMessage:    &system,
	}
	clone := CloneSessionEntry(entry).(CompactionEntry)
	clone.Details.(map[string]any)["nested"].([]any)[0] = "changed"
	clone.SystemMessage.Content[0] = TextContent{Text: "changed"}

	if entry.Details.(map[string]any)["nested"].([]any)[0] != "a" {
		t.Fatal("clone mutated details")
	}
	if entry.SystemMessage.Content[0].(TextContent).Text != "original" {
		t.Fatal("clone mutated system message")
	}
}
