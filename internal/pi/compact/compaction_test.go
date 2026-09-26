package compact

import (
	"reflect"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestCalculateContextTokens(t *testing.T) {
	if got := CalculateContextTokens(mockUsage(1000, 500, 200, 100)); got != 1800 {
		t.Fatalf("calculateContextTokens = %d, want 1800", got)
	}
	if got := CalculateContextTokens(mockUsage(0, 0, 0, 0)); got != 0 {
		t.Fatalf("calculateContextTokens zero = %d, want 0", got)
	}
}

func TestGetLastAssistantUsage(t *testing.T) {
	t.Run("finds the last non-aborted assistant usage", func(t *testing.T) {
		b := newEntryBuilder()
		entries := []model.SessionEntry{
			b.message(userMessage("Hello")),
			b.message(assistantMessageWithUsage("Hi", mockUsage(100, 50))),
			b.message(userMessage("How are you?")),
			b.message(assistantMessageWithUsage("Good", mockUsage(200, 100))),
		}
		usage, ok := GetLastAssistantUsage(entries)
		if !ok {
			t.Fatal("expected usage")
		}
		if usage.Input != 200 {
			t.Fatalf("input = %d, want 200", usage.Input)
		}
	})

	t.Run("skips aborted messages", func(t *testing.T) {
		b := newEntryBuilder()
		aborted := assistantMessageWithUsage("Aborted", mockUsage(300, 150))
		aborted.StopReason = model.StopAborted
		entries := []model.SessionEntry{
			b.message(userMessage("Hello")),
			b.message(assistantMessageWithUsage("Hi", mockUsage(100, 50))),
			b.message(userMessage("How are you?")),
			b.message(aborted),
		}
		usage, ok := GetLastAssistantUsage(entries)
		if !ok {
			t.Fatal("expected usage")
		}
		if usage.Input != 100 {
			t.Fatalf("input = %d, want 100", usage.Input)
		}
	})

	t.Run("skips all-zero assistant usage", func(t *testing.T) {
		b := newEntryBuilder()
		entries := []model.SessionEntry{
			b.message(userMessage("Hello")),
			b.message(assistantMessageWithUsage("Hi", mockUsage(100, 50))),
			b.message(userMessage("continue")),
			b.message(assistantMessageWithUsage("Partial", mockUsage(0, 0))),
		}
		usage, ok := GetLastAssistantUsage(entries)
		if !ok {
			t.Fatal("expected usage")
		}
		if usage.Input != 100 {
			t.Fatalf("input = %d, want 100", usage.Input)
		}
	})

	t.Run("returns false without assistant messages", func(t *testing.T) {
		b := newEntryBuilder()
		entries := []model.SessionEntry{b.message(userMessage("Hello"))}
		if _, ok := GetLastAssistantUsage(entries); ok {
			t.Fatal("expected no usage")
		}
	})
}

func TestEstimateContextTokensUsageAnchor(t *testing.T) {
	messages := []model.AgentMessage{
		userMessage("Hello"),
		assistantMessageWithUsage("Hi", mockUsage(100, 50)),
		userMessage("continue"),
		assistantMessageWithUsage("Partial thinking", mockUsage(0, 0)),
	}

	estimate := EstimateContextTokens(messages)

	if estimate.UsageTokens != 150 {
		t.Fatalf("usageTokens = %d, want 150", estimate.UsageTokens)
	}
	if estimate.LastUsageIndex == nil || *estimate.LastUsageIndex != 1 {
		t.Fatalf("lastUsageIndex = %v, want 1", estimate.LastUsageIndex)
	}
	if estimate.TrailingTokens <= 0 {
		t.Fatalf("trailingTokens = %d, want > 0", estimate.TrailingTokens)
	}
	if estimate.Tokens != 150+estimate.TrailingTokens {
		t.Fatalf("tokens = %d, want %d", estimate.Tokens, 150+estimate.TrailingTokens)
	}
}

func TestShouldCompact(t *testing.T) {
	settings := CompactionSettings{Enabled: true, ReserveTokens: 10000, KeepRecentTokens: 20000}
	if !ShouldCompact(95000, 100000, settings) {
		t.Fatal("expected compaction at 95000")
	}
	if ShouldCompact(89000, 100000, settings) {
		t.Fatal("did not expect compaction at 89000")
	}

	disabled := CompactionSettings{Enabled: false, ReserveTokens: 10000, KeepRecentTokens: 20000}
	if ShouldCompact(95000, 100000, disabled) {
		t.Fatal("did not expect compaction when disabled")
	}
}

func TestFindCutPointBasedOnTokenDifferences(t *testing.T) {
	b := newEntryBuilder()
	var entries []model.SessionEntry
	for i := range 10 {
		entries = append(entries, b.message(userMessage("User")))
		entries = append(entries, b.message(assistantMessageWithUsage("Assistant", mockUsage(0, 100, (i+1)*1000, 0))))
	}

	result := FindCutPoint(entries, 0, len(entries), 2500)

	if entries[result.FirstKeptEntryIndex].EntryType() != "message" {
		t.Fatalf("cut entry type = %q, want message", entries[result.FirstKeptEntryIndex].EntryType())
	}
	role := entries[result.FirstKeptEntryIndex].(*model.SessionMessageEntry).Message.MessageRole()
	if role != model.RoleUser && role != model.RoleAssistant {
		t.Fatalf("cut role = %q, want user or assistant", role)
	}
}

func TestFindCutPointReturnsStartWithoutCutPoints(t *testing.T) {
	b := newEntryBuilder()
	entries := []model.SessionEntry{b.message(assistantMessage("a"))}
	result := FindCutPoint(entries, 0, len(entries), 1000)
	if result.FirstKeptEntryIndex != 0 {
		t.Fatalf("firstKeptEntryIndex = %d, want 0", result.FirstKeptEntryIndex)
	}
}

func TestFindCutPointKeepsEverythingWithinBudget(t *testing.T) {
	b := newEntryBuilder()
	entries := []model.SessionEntry{
		b.message(userMessage("1")),
		b.message(assistantMessageWithUsage("a", mockUsage(0, 50, 500, 0))),
		b.message(userMessage("2")),
		b.message(assistantMessageWithUsage("b", mockUsage(0, 50, 1000, 0))),
	}

	result := FindCutPoint(entries, 0, len(entries), 50000)
	if result.FirstKeptEntryIndex != 0 {
		t.Fatalf("firstKeptEntryIndex = %d, want 0", result.FirstKeptEntryIndex)
	}
}

func TestFindCutPointIndicatesSplitTurn(t *testing.T) {
	b := newEntryBuilder()
	entries := []model.SessionEntry{
		b.message(userMessage("Turn 1")),
		b.message(assistantMessageWithUsage("A1", mockUsage(0, 100, 1000, 0))),
		b.message(userMessage("Turn 2")),
		b.message(assistantMessageWithUsage("A2-1", mockUsage(0, 100, 5000, 0))),
		b.message(assistantMessageWithUsage("A2-2", mockUsage(0, 100, 8000, 0))),
		b.message(assistantMessageWithUsage("A2-3", mockUsage(0, 100, 10000, 0))),
	}

	result := FindCutPoint(entries, 0, len(entries), 3000)

	cutEntry := entries[result.FirstKeptEntryIndex].(*model.SessionMessageEntry)
	if cutEntry.Message.MessageRole() == model.RoleAssistant {
		if !result.IsSplitTurn {
			t.Fatal("expected split turn")
		}
		if result.TurnStartIndex != 2 {
			t.Fatalf("turnStartIndex = %d, want 2", result.TurnStartIndex)
		}
	}
}

func TestFindCutPointBudgetsContextVisibleCustomMessages(t *testing.T) {
	b := newEntryBuilder()
	entries := []model.SessionEntry{
		b.message(userMessage("hi")),
		b.message(assistantMessage("hello")),
		b.customMessage("test", repeat("x", 4000)),
		b.message(assistantMessage("ok")),
	}

	tinyBudget := FindCutPoint(entries, 0, len(entries), 1)
	if tinyBudget.FirstKeptEntryIndex != 3 {
		t.Fatalf("tiny budget firstKept = %d, want 3", tinyBudget.FirstKeptEntryIndex)
	}
	if !tinyBudget.IsSplitTurn {
		t.Fatal("tiny budget expected split turn")
	}
	if tinyBudget.TurnStartIndex != 2 {
		t.Fatalf("tiny budget turnStart = %d, want 2", tinyBudget.TurnStartIndex)
	}

	customFitsBudget := FindCutPoint(entries, 0, len(entries), 2)
	if customFitsBudget.FirstKeptEntryIndex != 2 {
		t.Fatalf("custom fits firstKept = %d, want 2", customFitsBudget.FirstKeptEntryIndex)
	}
	if customFitsBudget.IsSplitTurn {
		t.Fatal("custom fits did not expect split turn")
	}
	if customFitsBudget.TurnStartIndex != -1 {
		t.Fatalf("custom fits turnStart = %d, want -1", customFitsBudget.TurnStartIndex)
	}
}

// Regression test for #9740.
func TestFindCutPointFallsBackBeforeOversizedTrailingToolResults(t *testing.T) {
	b := newEntryBuilder()
	oldUser := b.message(userMessage("old history"))
	oldAssistant := b.message(assistantMessage("old answer"))
	currentUser := b.message(userMessage("read the large file"))
	toolCall := b.message(model.AssistantMessage{
		Content:    model.ContentList{model.ToolCall{ID: "call-1", Name: "read", Arguments: map[string]any{"path": "big.txt"}}},
		StopReason: model.StopToolUse,
		Timestamp:  1,
	})
	toolResult := b.message(model.ToolResultMessage{
		ToolCallID: "call-1",
		ToolName:   "read",
		Content:    model.ContentList{model.TextContent{Text: repeat("x", 8000)}},
		Timestamp:  1,
	})
	entries := []model.SessionEntry{oldUser, oldAssistant, currentUser, toolCall, toolResult}

	result := FindCutPoint(entries, 0, len(entries), 1000)
	want := CutPointResult{FirstKeptEntryIndex: 3, TurnStartIndex: 2, IsSplitTurn: true}
	if result != want {
		t.Fatalf("cut point = %+v, want %+v", result, want)
	}

	preparation, ok := PrepareCompaction(entries, CompactionSettings{
		Enabled:          DefaultCompactionSettings.Enabled,
		ReserveTokens:    DefaultCompactionSettings.ReserveTokens,
		KeepRecentTokens: 1000,
	})
	if !ok {
		t.Fatal("expected preparation")
	}
	if preparation.FirstKeptEntryID != toolCall.ID {
		t.Fatalf("firstKeptEntryId = %q, want %q", preparation.FirstKeptEntryID, toolCall.ID)
	}
	wantMessages := []model.AgentMessage{oldUser.Message, oldAssistant.Message}
	if !reflect.DeepEqual(preparation.MessagesToSummarize, wantMessages) {
		t.Fatalf("messagesToSummarize = %#v, want %#v", preparation.MessagesToSummarize, wantMessages)
	}
	wantPrefix := []model.AgentMessage{currentUser.Message}
	if !reflect.DeepEqual(preparation.TurnPrefixMessages, wantPrefix) {
		t.Fatalf("turnPrefixMessages = %#v, want %#v", preparation.TurnPrefixMessages, wantPrefix)
	}
}

func TestPrepareCompactionDoesNotTreatSystemMessagesAsConversation(t *testing.T) {
	b := newEntryBuilder()
	prompt := "current prompt"
	system := b.message(model.SystemMessage{
		Sections:  model.SystemSections{{Name: "preamble", Value: &prompt}},
		Timestamp: 1,
	})
	user := b.message(userMessage("one long turn"))
	assistant := b.message(assistantMessage("assistant suffix"))

	preparation, ok := PrepareCompaction([]model.SessionEntry{system, user, assistant}, CompactionSettings{
		Enabled:          DefaultCompactionSettings.Enabled,
		ReserveTokens:    DefaultCompactionSettings.ReserveTokens,
		KeepRecentTokens: 1,
	})
	if !ok {
		t.Fatal("expected preparation")
	}
	if preparation.FirstKeptEntryID != assistant.ID {
		t.Fatalf("firstKeptEntryId = %q, want %q", preparation.FirstKeptEntryID, assistant.ID)
	}
	if !preparation.IsSplitTurn {
		t.Fatal("expected split turn")
	}
	if len(preparation.MessagesToSummarize) != 0 {
		t.Fatalf("messagesToSummarize = %#v, want empty", preparation.MessagesToSummarize)
	}
	wantPrefix := []model.AgentMessage{user.Message}
	if !reflect.DeepEqual(preparation.TurnPrefixMessages, wantPrefix) {
		t.Fatalf("turnPrefixMessages = %#v, want %#v", preparation.TurnPrefixMessages, wantPrefix)
	}
}

func TestPrepareCompactionSkipsRepeatedWhenKeptMessagesFit(t *testing.T) {
	b := newEntryBuilder()
	u1 := b.message(userMessage("user msg 1 (summarized by compaction1)"))
	a1 := b.message(assistantMessage("assistant msg 1"))
	u2 := b.message(userMessage("user msg 2 - kept by compaction1"))
	a2 := b.message(assistantMessage("assistant msg 2"))
	u3 := b.message(userMessage("user msg 3 - kept by compaction1"))
	a3 := b.message(assistantMessageWithUsage("assistant msg 3", mockUsage(5000, 1000)))
	compaction1 := b.compaction("First summary", u2.ID)
	u4 := b.message(userMessage("user msg 4 (new after compaction1)"))
	a4 := b.message(assistantMessageWithUsage("assistant msg 4", mockUsage(8000, 2000)))

	pathEntries := []model.SessionEntry{u1, a1, u2, a2, u3, a3, compaction1, u4, a4}
	if _, ok := PrepareCompaction(pathEntries, DefaultCompactionSettings); ok {
		t.Fatal("expected no preparation")
	}
}

func TestPrepareCompactionResummarizesKeptMessages(t *testing.T) {
	b := newEntryBuilder()
	u1 := b.message(userMessage(repeat("user msg 1 (summarized by compaction1)", 4)))
	a1 := b.message(assistantMessage(repeat("assistant msg 1", 4)))
	u2 := b.message(userMessage(repeat("user msg 2 - kept by compaction1 ", 12)))
	a2 := b.message(assistantMessage(repeat("assistant msg 2 ", 12)))
	u3 := b.message(userMessage(repeat("user msg 3 - kept by compaction1 ", 12)))
	a3 := b.message(assistantMessageWithUsage(repeat("assistant msg 3 ", 12), mockUsage(5000, 1000)))
	compaction1 := b.compaction("First summary", u2.ID)
	u4 := b.message(userMessage(repeat("user msg 4 (new after compaction1) ", 12)))
	a4 := b.message(assistantMessageWithUsage(repeat("assistant msg 4 ", 12), mockUsage(8000, 2000)))

	settings := DefaultCompactionSettings
	settings.KeepRecentTokens = 100
	preparation, ok := PrepareCompaction([]model.SessionEntry{u1, a1, u2, a2, u3, a3, compaction1, u4, a4}, settings)
	if !ok {
		t.Fatal("expected preparation")
	}

	summarizedText := extractText(preparation.MessagesToSummarize)
	if !contains(summarizedText, "user msg 2 - kept by compaction1") {
		t.Fatalf("summarized text = %q", summarizedText)
	}
	if !contains(summarizedText, "user msg 3 - kept by compaction1") {
		t.Fatalf("summarized text = %q", summarizedText)
	}
	if contains(summarizedText, "First summary") {
		t.Fatalf("summarized text unexpectedly contains previous summary: %q", summarizedText)
	}
	if preparation.PreviousSummary == nil || *preparation.PreviousSummary != "First summary" {
		t.Fatalf("previousSummary = %v, want First summary", preparation.PreviousSummary)
	}
}
