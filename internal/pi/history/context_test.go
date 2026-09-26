package history

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func msgEntry(id string, parentID *string, message model.AgentMessage) model.SessionEntry {
	return &model.SessionMessageEntry{
		Type: "message", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		Message: message,
	}
}

func compactionEntry(id string, parentID *string, summary, firstKept string) model.SessionEntry {
	return &model.CompactionEntry{
		Type: "compaction", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		Summary:          summary,
		FirstKeptEntryID: firstKept,
		TokensBefore:     1000,
	}
}

func branchEntry(id string, parentID *string, summary, fromID string) model.SessionEntry {
	return &model.BranchSummaryEntry{
		Type: "branch_summary", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		Summary: summary,
		FromID:  fromID,
	}
}

func customEntry(id string, parentID *string, customType string, data any) model.SessionEntry {
	return &model.CustomEntry{
		Type: "custom", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		CustomType: customType,
		Data:       data,
	}
}

func thinkingEntry(id string, parentID *string, level string) model.SessionEntry {
	return &model.ThinkingLevelChangeEntry{
		Type: "thinking_level_change", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		ThinkingLevel: level,
	}
}

func modelChangeEntry(id string, parentID *string, provider, modelID string) model.SessionEntry {
	return &model.ModelChangeEntry{
		Type: "model_change", ID: id, ParentID: parentID, Timestamp: "2025-01-01T00:00:00Z",
		Provider: provider,
		ModelID:  modelID,
	}
}

//go:fix inline
func TestBuildSessionContextTrivial(t *testing.T) {
	ctx := BuildSessionContext(nil, nil, nil)
	if len(ctx.Messages) != 0 {
		t.Fatalf("empty messages = %d", len(ctx.Messages))
	}
	if ctx.ThinkingLevel != "off" {
		t.Fatalf("thinking = %q", ctx.ThinkingLevel)
	}
	if ctx.Model != nil {
		t.Fatalf("model = %v", ctx.Model)
	}

	entries := []model.SessionEntry{msgEntry("1", nil, userMsg("hello"))}
	ctx = BuildSessionContext(entries, nil, nil)
	if len(ctx.Messages) != 1 || ctx.Messages[0].MessageRole() != model.RoleUser {
		t.Fatalf("single user message: %#v", ctx.Messages)
	}

	entries = []model.SessionEntry{
		msgEntry("1", nil, userMsg("hello")),
		msgEntry("2", new("1"), assistantMsg("hi there")),
		msgEntry("3", new("2"), userMsg("how are you")),
		msgEntry("4", new("3"), assistantMsg("great")),
	}
	ctx = BuildSessionContext(entries, nil, nil)
	if len(ctx.Messages) != 4 {
		t.Fatalf("conversation messages = %d", len(ctx.Messages))
	}
}

func TestBuildSessionContextTracksSettings(t *testing.T) {
	entries := []model.SessionEntry{msgEntry("1", nil, userMsg("hello")), thinkingEntry("2", new("1"), "high"), msgEntry("3", new("2"), assistantMsg("thinking hard"))}
	ctx := BuildSessionContext(entries, nil, nil)
	if ctx.ThinkingLevel != "high" {
		t.Fatalf("thinking = %q", ctx.ThinkingLevel)
	}
	if got := ctx.Model; got == nil || got.Provider != "anthropic" || got.ModelID != "test" {
		t.Fatalf("model = %#v", got)
	}

	entries = []model.SessionEntry{msgEntry("1", nil, userMsg("hello")), modelChangeEntry("2", new("1"), "openai", "gpt-4"), msgEntry("3", new("2"), assistantMsg("hi"))}
	ctx = BuildSessionContext(entries, nil, nil)
	if got := ctx.Model; got == nil || got.Provider != "anthropic" {
		t.Fatalf("assistant should overwrite model change: %#v", got)
	}
}

func TestBuildSessionContextWithCompaction(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("first")),
		msgEntry("2", new("1"), assistantMsg("response1")),
		msgEntry("3", new("2"), userMsg("second")),
		msgEntry("4", new("3"), assistantMsg("response2")),
		compactionEntry("5", new("4"), "Summary of first two turns", "3"),
		msgEntry("6", new("5"), userMsg("third")),
		msgEntry("7", new("6"), assistantMsg("response3")),
	}
	ctx := BuildSessionContext(entries, nil, nil)
	if len(ctx.Messages) != 5 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
	summary, ok := ctx.Messages[0].(model.CompactionSummaryMessage)
	if !ok {
		t.Fatalf("first message = %T", ctx.Messages[0])
	}
	if summary.Summary != "Summary of first two turns" {
		t.Fatalf("summary = %q", summary.Summary)
	}
}

func TestBuildSessionContextCompactionKeepsFromFirst(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("first")),
		msgEntry("2", new("1"), assistantMsg("response")),
		compactionEntry("3", new("2"), "Empty summary", "1"),
		msgEntry("4", new("3"), userMsg("second")),
	}
	ctx := BuildSessionContext(entries, nil, nil)
	if len(ctx.Messages) != 4 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
}

func TestBuildSessionContextUsesLatestCompaction(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("a")),
		msgEntry("2", new("1"), assistantMsg("b")),
		compactionEntry("3", new("2"), "First summary", "1"),
		msgEntry("4", new("3"), userMsg("c")),
		msgEntry("5", new("4"), assistantMsg("d")),
		compactionEntry("6", new("5"), "Second summary", "4"),
		msgEntry("7", new("6"), userMsg("e")),
	}
	ctx := BuildSessionContext(entries, nil, nil)
	if len(ctx.Messages) != 4 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
	if summary, ok := ctx.Messages[0].(model.CompactionSummaryMessage); !ok || summary.Summary != "Second summary" {
		t.Fatalf("first = %#v", ctx.Messages[0])
	}
}

func TestBuildContextEntriesIncludesCustomEntries(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("first")),
		customEntry("2", new("1"), "old-state", map[string]any{"hidden": true}),
		msgEntry("3", new("2"), assistantMsg("response1")),
		customEntry("4", new("3"), "kept-card", map[string]any{"title": "Kept"}),
		msgEntry("5", new("4"), userMsg("second")),
		compactionEntry("6", new("5"), "Summary", "4"),
		customEntry("7", new("6"), "after-card", map[string]any{"title": "After"}),
		msgEntry("8", new("7"), assistantMsg("response2")),
	}
	contextEntries := BuildContextEntries(entries, nil, nil)
	var ids []string
	for _, entry := range contextEntries {
		ids = append(ids, entry.Base().ID)
	}
	want := []string{"6", "4", "5", "7", "8"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v", ids)
		}
	}
	ctx := BuildSessionContext(entries, nil, nil)
	var roles []model.Role
	for _, message := range ctx.Messages {
		roles = append(roles, message.MessageRole())
	}
	if len(roles) != 3 || roles[0] != model.RoleCompactionSummary || roles[1] != model.RoleUser || roles[2] != model.RoleAssistant {
		t.Fatalf("roles = %v", roles)
	}
}

func TestBuildSessionContextKeepsSettingsAfterCompaction(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("first")),
		thinkingEntry("2", new("1"), "high"),
		msgEntry("3", new("2"), assistantMsg("response1")),
		msgEntry("4", new("3"), userMsg("second")),
		compactionEntry("5", new("4"), "Summary", "4"),
	}
	ctx := BuildSessionContext(entries, nil, nil)
	if ctx.ThinkingLevel != "high" {
		t.Fatalf("thinking = %q", ctx.ThinkingLevel)
	}
	if len(ctx.Messages) != 2 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
}

func TestBuildSessionContextBranches(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("start")),
		msgEntry("2", new("1"), assistantMsg("response")),
		msgEntry("3", new("2"), userMsg("branch A")),
		msgEntry("4", new("2"), userMsg("branch B")),
	}
	ctxA := BuildSessionContext(entries, new("3"), nil)
	if len(ctxA.Messages) != 3 {
		t.Fatalf("branch A messages = %d", len(ctxA.Messages))
	}
	ctxB := BuildSessionContext(entries, new("4"), nil)
	if len(ctxB.Messages) != 3 {
		t.Fatalf("branch B messages = %d", len(ctxB.Messages))
	}
}

func TestBuildSessionContextIncludesBranchSummary(t *testing.T) {
	entries := []model.SessionEntry{
		msgEntry("1", nil, userMsg("start")),
		msgEntry("2", new("1"), assistantMsg("response")),
		msgEntry("3", new("2"), userMsg("abandoned path")),
		branchEntry("4", new("2"), "Summary of abandoned work", "3"),
		msgEntry("5", new("4"), userMsg("new direction")),
	}
	ctx := BuildSessionContext(entries, new("5"), nil)
	if len(ctx.Messages) != 4 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
	if summary, ok := ctx.Messages[2].(model.BranchSummaryMessage); !ok || summary.Summary != "Summary of abandoned work" {
		t.Fatalf("summary = %#v", ctx.Messages[2])
	}
}

func TestBuildSessionContextUsesLastEntryWhenLeafMissing(t *testing.T) {
	entries := []model.SessionEntry{msgEntry("1", nil, userMsg("hello")), msgEntry("2", new("1"), assistantMsg("hi"))}
	ctx := BuildSessionContext(entries, new("nonexistent"), nil)
	if len(ctx.Messages) != 2 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
}

func TestBuildSessionContextHandlesOrphans(t *testing.T) {
	entries := []model.SessionEntry{msgEntry("1", nil, userMsg("hello")), msgEntry("2", new("missing"), assistantMsg("orphan"))}
	ctx := BuildSessionContext(entries, new("2"), nil)
	if len(ctx.Messages) != 1 {
		t.Fatalf("messages = %d", len(ctx.Messages))
	}
}
