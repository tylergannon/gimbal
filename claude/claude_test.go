package claude

import (
	"context"
	"strings"
	"testing"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// TestCloseIsIdempotent covers acceptance item 4: closing a Claude session
// twice returns nil both times. CreateSession only mints an id (Claude
// Code's process is launched per turn, never held by the adapter), so this
// needs no live process either.
func TestCloseIsIdempotent(t *testing.T) {
	ad := New().(*adapter)
	id, err := ad.CreateSession(context.Background(), "model", "", ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := ad.Close(context.Background(), id); err != nil {
		t.Fatalf("first Close = %v, want nil", err)
	}
	if err := ad.Close(context.Background(), id); err != nil {
		t.Fatalf("second Close = %v, want nil", err)
	}
}

func TestAssistantErrorPreservesProviderRequestID(t *testing.T) {
	message := claudeagent.AssistantMessage{
		Error:     claudeagent.AssistantMessageErrorInvalidRequest,
		RequestID: "req_217",
	}
	err := assistantError(message)
	if err == nil || !strings.Contains(err.Error(), "invalid_request") || !strings.Contains(err.Error(), "req_217") {
		t.Fatalf("assistantError = %v, want code and request ID", err)
	}
}

// TestSteerWithNoStreamLiveIsDropped: a steer on a session with no turn
// running has no live SDK stream to send on. The adapter reports it
// dropped, false and no error, without launching Claude Code. A steer on an
// unknown session is the error it always was.
func TestSteerWithNoStreamLiveIsDropped(t *testing.T) {
	ad := New().(*adapter)
	id, err := ad.CreateSession(context.Background(), "model", "", ".")
	if err != nil {
		t.Fatal(err)
	}
	landed, err := ad.Steer(context.Background(), id, "change course")
	if err != nil {
		t.Fatalf("Steer = %v, want nil: a dropped steer is not an error", err)
	}
	if landed {
		t.Fatal("Steer landed with no stream live")
	}
	if _, err := ad.Steer(context.Background(), "unknown-session", "change course"); err == nil {
		t.Fatal("Steer on an unknown session = nil, want an error")
	}
}

// TestSessionKeepsItsEffortAndForksInheritIt: the reasoning effort a role is
// bound to is held for every turn of the session, and a fork continues its
// parent's conversation on the parent's effort.
func TestSessionKeepsItsEffortAndForksInheritIt(t *testing.T) {
	ad := New().(*adapter)
	id, err := ad.CreateSession(context.Background(), "model", "xhigh", ".")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := ad.session(id)
	if err != nil {
		t.Fatal(err)
	}
	if parent.effort != "xhigh" {
		t.Fatalf("session effort = %q, want xhigh", parent.effort)
	}
	forkID, err := ad.Fork(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	fork, err := ad.session(forkID)
	if err != nil {
		t.Fatal(err)
	}
	if fork.effort != "xhigh" || fork.model != "model" {
		t.Fatalf("fork = %q/%q, want model/xhigh", fork.model, fork.effort)
	}
}

// TestAssistantErrorCarriesTheCLIsExplanation: when a turn fails, Claude
// Code states the code on the assistant message and writes why it failed as
// the message's text. The error carries both, so a reader of the run never
// has to open the CLI's own transcript to learn the reason.
func TestAssistantErrorCarriesTheCLIsExplanation(t *testing.T) {
	message := claudeagent.AssistantMessage{Error: claudeagent.AssistantMessageErrorInvalidRequest}
	message.Message.Content = []claudeagent.ContentBlock{{
		Type: "text",
		Text: "Autocompact is thrashing: the context refilled to the limit\nwithin 3 turns of the previous compact, 3 times in a row.",
	}}
	err := assistantError(message)
	if err == nil {
		t.Fatal("assistantError = nil, want an error")
	}
	want := "claude: assistant error: invalid_request: Autocompact is thrashing: the context refilled to the limit within 3 turns of the previous compact, 3 times in a row."
	if err.Error() != want {
		t.Fatalf("assistantError = %q, want %q", err, want)
	}
}
