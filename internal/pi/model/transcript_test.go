package model

import "testing"

func tool(name, desc string) Tool {
	return Tool{Name: name, Description: desc, Parameters: Object(Prop("x", String()))}
}

func TestGetCurrentToolsAppliesDeltas(t *testing.T) {
	messages := []Message{
		SystemMessage{ToolsAdded: []Tool{tool("read", "read"), tool("write", "write")}},
		SystemMessage{ToolsAdded: []Tool{tool("grep", "grep")}, ToolsRemoved: []ToolReference{{Name: "write"}}},
	}
	got := GetCurrentTools(messages)
	want := []string{"read", "grep"}
	if len(got) != len(want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("tool[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestGetCurrentSystemMessageReplaysPromptAndSections(t *testing.T) {
	messages := []Message{
		NewSystemText("base", 1),
		SystemMessage{Content: ContentList{TextContent{Text: "extra"}}, Timestamp: 2,
			Sections: SystemSections{{Name: "rules", Value: new("new rules")}}},
		SystemMessage{Sections: SystemSections{{Name: "rules", Value: nil}}, Timestamp: 3},
	}
	got, ok := GetCurrentSystemMessage(messages)
	if !ok {
		t.Fatal("expected a current system message")
	}
	if text := GetSystemMessageText(got); text != "base\n\nextra" {
		t.Fatalf("prompt = %q", text)
	}
	if got.Sections.Len() != 0 {
		t.Fatalf("removed section still present: %v", got.Sections)
	}
}

func TestNormalizeContextFoldsShorthand(t *testing.T) {
	ctx := NormalizeContext(Context{
		SystemPrompt: "system",
		Messages:     []Message{NewUserText("hi", 1)},
		Tools:        []Tool{tool("read", "read")},
	})
	if len(ctx.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(ctx.Messages))
	}
	head, ok := GetInitialSystemMessage(ctx.Messages)
	if !ok {
		t.Fatal("missing leading system message")
	}
	if len(head.ToolsAdded) != 1 {
		t.Fatalf("tools = %v", head.ToolsAdded)
	}
	if collapsed := CollapseSystemMessages(ctx); len(collapsed.Messages) != 2 {
		t.Fatalf("collapse changed message count: %d", len(collapsed.Messages))
	}
}

func TestToolStateProjection(t *testing.T) {
	previous := []Tool{tool("read", "read"), tool("write", "write")}
	current := []Tool{tool("read", "read v2"), tool("grep", "grep")}
	changes := GetToolStateChanges(previous, current)

	if len(changes.ToolsAdded) != 2 {
		t.Fatalf("added = %v", changes.ToolsAdded)
	}
	if changes.ToolsAdded[0].Name != "read" || changes.ToolsAdded[1].Name != "grep" {
		t.Fatalf("added order = %v", changes.ToolsAdded)
	}
	if len(changes.ToolsRemoved) != 2 {
		t.Fatalf("removed = %v", changes.ToolsRemoved)
	}
	if changes.ToolsRemoved[0].Name != "read" || changes.ToolsRemoved[1].Name != "write" {
		t.Fatalf("removed order = %v", changes.ToolsRemoved)
	}
	if Dec(previous[0], current[0]) {
		t.Fatal("changed definition reported equal")
	}
	if !Dec(tool("a", "same"), tool("a", "same")) {
		t.Fatal("identical definitions reported different")
	}
}

// Dec is a short alias for DeclarationsEqual in tests.
func Dec(a, b Tool) bool { return DeclarationsEqual(a, b) }

func TestResolveTranscriptTools(t *testing.T) {
	additive := []Message{
		SystemMessage{ToolsAdded: []Tool{tool("read", "read")}},
		SystemMessage{ToolsAdded: []Tool{tool("grep", "grep")}},
	}
	result := ResolveTranscriptTools(additive, true)
	if !result.AnchorsAdditions {
		t.Fatal("expected anchored additions")
	}
	if len(result.RequestTools) != 1 || result.RequestTools[0].Name != "read" {
		t.Fatalf("request tools = %v", result.RequestTools)
	}

	nonAdditive := []Message{
		SystemMessage{ToolsAdded: []Tool{tool("read", "read")}},
		SystemMessage{ToolsRemoved: []ToolReference{{Name: "read"}}},
	}
	result = ResolveTranscriptTools(nonAdditive, true)
	if result.AnchorsAdditions {
		t.Fatal("removal should disable anchored additions")
	}
	if len(result.RequestTools) != 0 {
		t.Fatalf("request tools = %v", result.RequestTools)
	}
}

func TestHasToolRedefinitions(t *testing.T) {
	redefined := []Message{
		SystemMessage{ToolsAdded: []Tool{tool("read", "v1")}},
		SystemMessage{ToolsAdded: []Tool{tool("read", "v2")}},
	}
	if !HasToolRedefinitions(redefined) {
		t.Fatal("expected redefinition")
	}
	if HasNonAdditiveToolChanges([]Message{SystemMessage{ToolsAdded: []Tool{tool("read", "read")}}}) {
		t.Fatal("plain addition is non-additive")
	}
}

func TestConvertToLlm(t *testing.T) {
	messages := []AgentMessage{
		NewUserText("hello", 1),
		BashExecutionMessage{Command: "ls", Output: "a", Timestamp: 2},
		BashExecutionMessage{Command: "secret", ExcludeFromContext: true, Timestamp: 3},
		NewCustomText("note", "from extension", true, nil, 4),
		BranchSummaryMessage{Summary: "branch", Timestamp: 5},
		CompactionSummaryMessage{Summary: "compact", TokensBefore: 100, Timestamp: 6},
	}
	got := ConvertToLlm(messages)
	if len(got) != 5 {
		t.Fatalf("converted %d messages, want 5", len(got))
	}
	first := got[1].(UserMessage)
	if text := ContentText(first.Content); !contains(text, "Ran `ls`") {
		t.Fatalf("bash text = %q", text)
	}
	branch := ContentText(got[3].(UserMessage).Content)
	if !contains(branch, BranchSummaryPrefix) || !contains(branch, BranchSummarySuffix) {
		t.Fatalf("branch summary = %q", branch)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
