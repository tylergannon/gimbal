package history

import (
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func newMemory(t *testing.T, cwd string) *SessionManager {
	t.Helper()
	session, err := InMemory(cwd, nil, nil, nil)
	if err != nil {
		t.Fatalf("in memory: %v", err)
	}
	return session
}

func TestAppendChain(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("first"))
	id2 := mustAppendMessage(t, session, assistantMsg("second"))
	id3 := mustAppendMessage(t, session, userMsg("third"))

	entries := session.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].Base().ID != id1 || entries[0].Base().ParentID != nil {
		t.Fatalf("first entry = %#v", entries[0].Base())
	}
	if entries[1].Base().ID != id2 || ptrValue(entries[1].Base().ParentID) != id1 {
		t.Fatalf("second entry = %#v", entries[1].Base())
	}
	if entries[2].Base().ID != id3 || ptrValue(entries[2].Base().ParentID) != id2 {
		t.Fatalf("third entry = %#v", entries[2].Base())
	}
}

func TestAppendIntegratesIntoTree(t *testing.T) {
	session := newMemory(t, "/project")
	msgID := mustAppendMessage(t, session, userMsg("hello"))
	thinkingID, err := session.AppendThinkingLevelChange("high")
	if err != nil {
		t.Fatal(err)
	}
	modelID, err := session.AppendModelChange("openai", "gpt-4")
	if err != nil {
		t.Fatal(err)
	}
	customID, err := session.AppendCustomEntry("my_data", map[string]any{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}

	entries := session.GetEntries()
	if ptrValue(entries[1].Base().ParentID) != msgID || entries[1].Base().ID != thinkingID {
		t.Fatalf("thinking entry = %#v", entries[1].Base())
	}
	if ptrValue(entries[2].Base().ParentID) != thinkingID || entries[2].Base().ID != modelID {
		t.Fatalf("model entry = %#v", entries[2].Base())
	}
	if ptrValue(entries[3].Base().ParentID) != modelID || entries[3].Base().ID != customID {
		t.Fatalf("custom entry = %#v", entries[3].Base())
	}
}

func TestAppendCompactionTree(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("1"))
	id2 := mustAppendMessage(t, session, assistantMsg("2"))
	usage := &model.Usage{Input: 10, Output: 20, TotalTokens: 100}
	compactionID, err := session.AppendCompaction("summary", &id1, 1000, nil, false, usage)
	if err != nil {
		t.Fatal(err)
	}
	entries := session.GetEntries()
	compaction, ok := entries[2].(*model.CompactionEntry)
	if !ok {
		t.Fatalf("entry type = %T", entries[2])
	}
	if compaction.Base().ID != compactionID || ptrValue(compaction.Base().ParentID) != id2 {
		t.Fatalf("compaction base = %#v", compaction.Base())
	}
	if compaction.Summary != "summary" || compaction.FirstKeptEntryID != id1 || compaction.TokensBefore != 1000 {
		t.Fatalf("compaction = %#v", compaction)
	}
	if compaction.Usage == nil || compaction.Usage.TotalTokens != 100 {
		t.Fatalf("usage = %#v", compaction.Usage)
	}
}

func TestLeafAdvancesAfterAppend(t *testing.T) {
	session := newMemory(t, "/project")
	if session.GetLeafID() != nil {
		t.Fatal("new session should have no leaf")
	}
	id1 := mustAppendMessage(t, session, userMsg("1"))
	if leaf := session.GetLeafID(); leaf == nil || *leaf != id1 {
		t.Fatalf("leaf = %v", leaf)
	}
	id2 := mustAppendMessage(t, session, assistantMsg("2"))
	if leaf := session.GetLeafID(); leaf == nil || *leaf != id2 {
		t.Fatalf("leaf = %v", leaf)
	}
}

func TestGetBranch(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("1"))
	id2 := mustAppendMessage(t, session, assistantMsg("2"))
	mustAppendMessage(t, session, userMsg("3"))

	if got := session.GetBranch(); len(got) != 3 {
		t.Fatalf("branch length = %d", len(got))
	}
	path := session.GetBranch(id2)
	if len(path) != 2 || path[0].Base().ID != id1 || path[1].Base().ID != id2 {
		t.Fatalf("branch from id2 = %#v", path)
	}
}

func TestGetTreeBranches(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("1"))
	id2 := mustAppendMessage(t, session, assistantMsg("2"))
	id3 := mustAppendMessage(t, session, userMsg("3"))
	if err := session.Branch(id2); err != nil {
		t.Fatal(err)
	}
	id4 := mustAppendMessage(t, session, userMsg("4-branch"))

	tree := session.GetTree()
	if len(tree) != 1 || tree[0].Entry.Base().ID != id1 {
		t.Fatalf("tree roots = %#v", tree)
	}
	node2 := tree[0].Children[0]
	if node2.Entry.Base().ID != id2 || len(node2.Children) != 2 {
		t.Fatalf("node2 = %#v", node2)
	}
	ids := []string{node2.Children[0].Entry.Base().ID, node2.Children[1].Entry.Base().ID}
	if !contains(ids, id3) || !contains(ids, id4) {
		t.Fatalf("children = %v", ids)
	}
}

func TestBranchErrorsForMissingEntry(t *testing.T) {
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("hello"))
	if err := session.Branch("nonexistent"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestBranchWithSummary(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("1"))
	mustAppendMessage(t, session, assistantMsg("2"))
	id3 := mustAppendMessage(t, session, userMsg("3"))
	usage := &model.Usage{TotalTokens: 100}
	summaryID, err := session.BranchWithSummary(&id1, "Summary of abandoned work", nil, false, usage)
	if err != nil {
		t.Fatal(err)
	}
	if leaf := session.GetLeafID(); leaf == nil || *leaf != summaryID {
		t.Fatalf("leaf = %v", leaf)
	}
	summary, ok := session.GetEntry(summaryID).(*model.BranchSummaryEntry)
	if !ok {
		t.Fatalf("entry = %T", session.GetEntry(summaryID))
	}
	if ptrValue(summary.Base().ParentID) != id1 || summary.FromID != id3 {
		t.Fatalf("summary = %#v", summary)
	}
	if _, err := session.BranchWithSummary(new("nope"), "summary", nil, false, nil); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestBuildSessionContextWithBranches(t *testing.T) {
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("msg1"))
	id2 := mustAppendMessage(t, session, assistantMsg("msg2"))
	mustAppendMessage(t, session, userMsg("msg3"))
	if err := session.Branch(id2); err != nil {
		t.Fatal(err)
	}
	mustAppendMessage(t, session, assistantMsg("msg4-branch"))
	ctx := session.BuildSessionContext()
	if len(ctx.Messages) != 3 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
	if ctx.Messages[2].(model.AssistantMessage).Content[0].(model.TextContent).Text != "msg4-branch" {
		t.Fatalf("messages = %#v", ctx.Messages)
	}
}

func TestCreateBranchedSessionInMemory(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("1"))
	id2 := mustAppendMessage(t, session, assistantMsg("2"))
	id3 := mustAppendMessage(t, session, userMsg("3"))
	mustAppendMessage(t, session, assistantMsg("4"))
	if err := session.Branch(id3); err != nil {
		t.Fatal(err)
	}
	mustAppendMessage(t, session, userMsg("5"))

	path, err := session.CreateBranchedSession(id2)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("in-memory path = %q", path)
	}
	entries := session.GetEntries()
	if len(entries) != 2 || entries[0].Base().ID != id1 || entries[1].Base().ID != id2 {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestCreateBranchedSessionMissingEntry(t *testing.T) {
	session := newMemory(t, "/project")
	mustAppendMessage(t, session, userMsg("hello"))
	if _, err := session.CreateBranchedSession("nonexistent"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestLabelsSetGetClear(t *testing.T) {
	session := newMemory(t, "/project")
	msgID := mustAppendMessage(t, session, userMsg("hello"))
	if _, ok := session.GetLabel(msgID); ok {
		t.Fatal("unexpected label")
	}
	labelID, err := session.AppendLabelChange(msgID, new("checkpoint"))
	if err != nil {
		t.Fatal(err)
	}
	if label, ok := session.GetLabel(msgID); !ok || label != "checkpoint" {
		t.Fatalf("label = %q ok=%v", label, ok)
	}
	labelEntry, ok := session.GetEntry(labelID).(*model.LabelEntry)
	if !ok || labelEntry.TargetID != msgID || labelEntry.Label == nil || *labelEntry.Label != "checkpoint" {
		t.Fatalf("label entry = %#v", session.GetEntry(labelID))
	}
	if _, err := session.AppendLabelChange(msgID, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := session.GetLabel(msgID); ok {
		t.Fatal("label should be cleared")
	}
}

func TestLabelsLastWinsAndTree(t *testing.T) {
	session := newMemory(t, "/project")
	msg1 := mustAppendMessage(t, session, userMsg("hello"))
	msg2 := mustAppendMessage(t, session, assistantMsg("hi"))
	if _, err := session.AppendLabelChange(msg1, new("start")); err != nil {
		t.Fatal(err)
	}
	lastID, err := session.AppendLabelChange(msg2, new("response"))
	if err != nil {
		t.Fatal(err)
	}
	tree := session.GetTree()
	msg2Node := tree[0].Children[0]
	if msg2Node.Entry.Base().ID != msg2 || msg2Node.Label == nil || *msg2Node.Label != "response" {
		t.Fatalf("node = %#v", msg2Node)
	}
	lastEntry := session.GetEntry(lastID)
	if msg2Node.LabelTimestamp == nil || *msg2Node.LabelTimestamp != lastEntry.Base().Timestamp {
		t.Fatalf("label timestamp = %v", msg2Node.LabelTimestamp)
	}
}

func TestLabelsNotOnBranchDropped(t *testing.T) {
	session := newMemory(t, "/project")
	msg1 := mustAppendMessage(t, session, userMsg("1"))
	msg2 := mustAppendMessage(t, session, assistantMsg("2"))
	msg3 := mustAppendMessage(t, session, userMsg("3"))
	for _, item := range []struct {
		id    string
		label string
	}{{msg1, "first"}, {msg2, "second"}, {msg3, "third"}} {
		if _, err := session.AppendLabelChange(item.id, new(item.label)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := session.CreateBranchedSession(msg2); err != nil {
		t.Fatal(err)
	}
	if label, _ := session.GetLabel(msg1); label != "first" {
		t.Fatalf("msg1 label = %q", label)
	}
	if label, _ := session.GetLabel(msg2); label != "second" {
		t.Fatalf("msg2 label = %q", label)
	}
	if _, ok := session.GetLabel(msg3); ok {
		t.Fatal("msg3 label should be dropped")
	}
}

func TestLabelsRewireRemovedChildren(t *testing.T) {
	session := newMemory(t, "/project")
	msg1 := mustAppendMessage(t, session, userMsg("hello"))
	if _, err := session.AppendLabelChange(msg1, new("checkpoint")); err != nil {
		t.Fatal(err)
	}
	modelChangeID, err := session.AppendModelChange("anthropic", "claude-test")
	if err != nil {
		t.Fatal(err)
	}
	msg2 := mustAppendMessage(t, session, userMsg("followup"))
	if _, err := session.CreateBranchedSession(msg2); err != nil {
		t.Fatal(err)
	}
	if ptrValue(session.GetEntry(modelChangeID).Base().ParentID) != msg1 {
		t.Fatalf("model change parent = %v", session.GetEntry(modelChangeID).Base().ParentID)
	}
}

func TestLabelErrors(t *testing.T) {
	session := newMemory(t, "/project")
	if _, err := session.AppendLabelChange("non-existent", new("label")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestCustomEntryTraversal(t *testing.T) {
	session := newMemory(t, "/project")
	msgID := mustAppendMessage(t, session, userMsg("hello"))
	customID, err := session.AppendCustomEntry("my_data", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	msg2ID := mustAppendMessage(t, session, assistantMsg("hi"))

	entries := session.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("entries = %d", len(entries))
	}
	custom, ok := entries[1].(*model.CustomEntry)
	if !ok || custom.CustomType != "my_data" || custom.Base().ID != customID || ptrValue(custom.Base().ParentID) != msgID {
		t.Fatalf("custom = %#v", entries[1])
	}
	path := session.GetBranch()
	if len(path) != 3 || path[0].Base().ID != msgID || path[1].Base().ID != customID || path[2].Base().ID != msg2ID {
		t.Fatalf("path = %#v", path)
	}
	if ctx := session.BuildSessionContext(); len(ctx.Messages) != 2 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
}

func TestAppendUsageAndSessionInfo(t *testing.T) {
	session := newMemory(t, "/project")
	if _, err := session.AppendMessage(userMsg("hello")); err != nil {
		t.Fatal(err)
	}
	usageEntry, err := session.AppendUsage("cache_warm", "anthropic", "test", model.Usage{TotalTokens: 7}, "note")
	if err != nil {
		t.Fatal(err)
	}
	if usageEntry.Kind != "cache_warm" || usageEntry.Usage.TotalTokens != 7 || usageEntry.Note != "note" {
		t.Fatalf("usage = %#v", usageEntry)
	}
	if _, err := session.AppendSessionInfo("  My  Session\nName "); err != nil {
		t.Fatal(err)
	}
	if name := session.GetSessionName(); name != "My  Session Name" {
		t.Fatalf("name = %q", name)
	}
	if _, err := session.AppendSessionInfo(""); err != nil {
		t.Fatal(err)
	}
	if name := session.GetSessionName(); name != "" {
		t.Fatalf("cleared name = %q", name)
	}
}

func TestCustomMessageEntryProjectionAndChildren(t *testing.T) {
	session := newMemory(t, "/project")
	id1 := mustAppendMessage(t, session, userMsg("hello"))
	customID, err := session.AppendCustomMessageEntry("injected", model.ContentList{model.TextContent{Text: "context"}}, true, map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	children := session.GetChildren(id1)
	if len(children) != 1 || children[0].Base().ID != customID {
		t.Fatalf("children = %#v", children)
	}
	projection := session.BuildSessionProjection()
	if len(projection.Messages) != 2 {
		t.Fatalf("messages = %d", len(projection.Messages))
	}
	custom, ok := projection.Messages[1].(model.CustomMessage)
	if !ok || custom.CustomType != "injected" || contentText(custom.Content) != "context" {
		t.Fatalf("custom message = %#v", projection.Messages[1])
	}
}

func TestGetLatestCompactionEntry(t *testing.T) {
	entries := []model.SessionEntry{
		compactionEntry("1", nil, "first", "1"),
		msgEntry("2", new("1"), userMsg("hi")),
		compactionEntry("3", new("2"), "second", "2"),
	}
	latest := GetLatestCompactionEntry(entries)
	if latest == nil || latest.Summary != "second" {
		t.Fatalf("latest = %#v", latest)
	}
	if got := GetLatestCompactionEntry([]model.SessionEntry{msgEntry("1", nil, userMsg("hi"))}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}
