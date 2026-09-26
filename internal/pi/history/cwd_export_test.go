package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestMissingSessionCwdIssue(t *testing.T) {
	root := t.TempDir()
	fallbackCwd := filepath.Join(root, "fallback")
	if err := os.MkdirAll(fallbackCwd, 0o755); err != nil {
		t.Fatal(err)
	}
	missingCwd := filepath.Join(fallbackCwd, "does-not-exist")
	sessionDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(sessionDir, "session.jsonl")
	writeFile(t, sessionFile, sessionHeaderLine("session-id", missingCwd, new(3))+"\n")

	session, err := Open(sessionFile, sessionDir, "")
	if err != nil {
		t.Fatal(err)
	}
	issue := GetMissingSessionCwdIssue(session, fallbackCwd)
	if issue == nil || issue.SessionCwd != missingCwd || issue.FallbackCwd != fallbackCwd || issue.SessionFile != sessionFile {
		t.Fatalf("issue = %#v", issue)
	}
	if err := AssertSessionCwdExists(session, fallbackCwd); err == nil {
		t.Fatal("expected missing cwd error")
	} else if !strings.Contains(err.Error(), "Stored session working directory does not exist") {
		t.Fatalf("error = %v", err)
	}

	override, err := Open(sessionFile, sessionDir, fallbackCwd)
	if err != nil {
		t.Fatal(err)
	}
	if override.GetCwd() != fallbackCwd {
		t.Fatalf("override cwd = %q", override.GetCwd())
	}
	if issue := GetMissingSessionCwdIssue(override, fallbackCwd); issue != nil {
		t.Fatalf("issue = %#v", issue)
	}
}

func TestSerializeSessionBranch(t *testing.T) {
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("hello"))
	promptID := mustAppendMessage(t, session, assistantMsg("hi"))

	serialized := SerializeSessionBranch(session, func(parentID *string, timestamp string) []model.SessionEntry {
		if ptrValue(parentID) != promptID {
			t.Fatalf("trailing parent = %v", parentID)
		}
		return []model.SessionEntry{customEntry("trailing", parentID, "export", map[string]any{"x": 1})}
	})
	lines := strings.Split(strings.TrimRight(serialized, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d: %q", len(lines), serialized)
	}
	header, ok := parseSessionLine(lines[0])
	if !ok || header.header == nil || header.header.ID != session.GetSessionID() {
		t.Fatalf("header = %#v", lines[0])
	}
	first, ok := parseSessionLine(lines[1])
	if !ok || first.entry == nil || first.entry.Base().ParentID != nil {
		t.Fatalf("first entry = %#v", lines[1])
	}
	second, ok := parseSessionLine(lines[2])
	if !ok || second.entry == nil || ptrValue(second.entry.Base().ParentID) != first.entry.Base().ID {
		t.Fatalf("second entry = %#v", lines[2])
	}
}

func TestExportSessionToJSONL(t *testing.T) {
	dir := t.TempDir()
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("hello"))
	outputPath := filepath.Join(dir, "nested", "export.jsonl")
	written, err := ExportSessionToJSONL(session, outputPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if written != outputPath {
		t.Fatalf("path = %q", written)
	}
	header, entries, err := LoadEntriesFromFile(written)
	if err != nil || header == nil || len(entries) != 1 {
		t.Fatalf("export = %#v %#v %v", header, entries, err)
	}
}

func TestContextEditOmitAndReplace(t *testing.T) {
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("request"))
	assistantID := mustAppendMessage(t, session, assistantMsg("partial"))
	resultID := mustAppendMessage(t, session, toolResultMsg("raw output"))
	if _, err := session.AppendContextEdit(assistantID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendContextEdit(resultID, nil); err != nil {
		t.Fatal(err)
	}
	messageCount := 0
	for _, entry := range session.GetBranch() {
		if entry.EntryType() == "message" {
			messageCount++
		}
	}
	if messageCount != 3 {
		t.Fatalf("message entries = %d", messageCount)
	}
	projection := session.BuildSessionProjection()
	if len(projection.Messages) != 1 || projection.Messages[0].MessageRole() != model.RoleUser {
		t.Fatalf("projection = %#v", projection.Messages)
	}

	replaceSession := newMemory(t, "/project")
	targetID := mustAppendMessage(t, replaceSession, assistantMsg("original"))
	content := model.ContentList{model.TextContent{Text: "first"}}
	if _, err := replaceSession.AppendContextEdit(targetID, &model.ContextEditableContent{Content: content}); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceSession.AppendContextEdit(targetID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceSession.AppendContextEdit(targetID, &model.ContextEditableContent{Content: model.ContentList{model.TextContent{Text: "restored"}}}); err != nil {
		t.Fatal(err)
	}
	projected := replaceSession.BuildSessionProjection().Messages[0].(model.AssistantMessage)
	if contentText(projected.Content) != "restored" {
		t.Fatalf("projected = %q", contentText(projected.Content))
	}
	if projected.Usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v", projected.Usage)
	}
	original := replaceSession.GetEntry(targetID).(*model.SessionMessageEntry)
	if contentText(original.Message.(model.AssistantMessage).Content) != "original" {
		t.Fatalf("original changed")
	}
}

func TestContextEditBranchRelative(t *testing.T) {
	session := newMemory(t, "/project")
	targetID := mustAppendMessage(t, session, userMsg("original"))
	if _, err := session.AppendContextEdit(targetID, &model.ContextEditableContent{Content: model.ContentList{model.TextContent{Text: "edited"}}}); err != nil {
		t.Fatal(err)
	}
	if got := contentText(session.BuildSessionProjection().Messages[0].(model.UserMessage).Content); got != "edited" {
		t.Fatalf("edited = %q", got)
	}
	if err := session.Branch(targetID); err != nil {
		t.Fatal(err)
	}
	if got := contentText(session.BuildSessionProjection().Messages[0].(model.UserMessage).Content); got != "original" {
		t.Fatalf("original = %q", got)
	}
}

func TestContextEditRejectsInvalidTarget(t *testing.T) {
	session := newMemory(t, "/project")
	targetID := mustAppendMessage(t, session, userMsg("hello"))
	if _, err := session.AppendContextEdit("missing", nil); err == nil {
		t.Fatal("expected not found")
	}
	labelID, err := session.AppendLabelChange(targetID, new("label"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendContextEdit(labelID, nil); err == nil || !strings.Contains(err.Error(), "editable model content") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseContextEditStringReplacement(t *testing.T) {
	raw := `{"type":"context_edit","id":"e1","parentId":null,"timestamp":"2025-01-01T00:00:00Z","targetId":"t1","replacement":{"content":"imported replacement"}}`
	entry, err := parseContextEditLine([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	edit, ok := entry.(*model.ContextEditEntry)
	if !ok || edit.Replacement == nil || contentText(edit.Replacement.Content) != "imported replacement" {
		t.Fatalf("edit = %#v", entry)
	}
}

func TestAppendCompactionCapturesSystemMessage(t *testing.T) {
	system := &model.SessionMessageEntry{
		Type: "message", ID: "s1", Timestamp: "2025-01-01T00:00:00Z",
		Message: model.NewSystemText("system prompt", 1),
	}
	session, err := InMemory("/project", nil, nil, []model.SessionEntry{system})
	if err != nil {
		t.Fatal(err)
	}
	compactionID, err := session.AppendCompaction("summary", nil, 100, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	compaction := session.GetEntry(compactionID).(*model.CompactionEntry)
	if compaction.SystemMessage == nil {
		t.Fatal("expected captured system message")
	}
	if model.GetSystemMessageText(*compaction.SystemMessage) != "system prompt" {
		t.Fatalf("system message = %q", model.GetSystemMessageText(*compaction.SystemMessage))
	}
	if compaction.FirstKeptEntryID != compactionID {
		t.Fatalf("first kept = %q", compaction.FirstKeptEntryID)
	}
}
