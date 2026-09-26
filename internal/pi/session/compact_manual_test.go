package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestManualCompactEmptyHistory(t *testing.T) {
	s := newTestSession(t, testSessionOptions{settings: compactionSettingsManager(16384, 1)})
	s.sessionManager.ResetLeaf()
	if _, err := s.Compact(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "Nothing to compact") {
		t.Fatalf("empty history compaction = %v", err)
	}
}

func TestFreshUserMessageResetsOverflowRecovery(t *testing.T) {
	s := newTestSession(t, testSessionOptions{})
	s.overflowAttempted = true
	if err := s.handleAgentEvent(context.Background(), model.AgentEvent{Type: model.EvMessageStart, Message: model.NewUserText("fresh prompt", 1)}); err != nil {
		t.Fatal(err)
	}
	if s.overflowAttempted {
		t.Fatal("fresh prompt retained previous overflow recovery state")
	}
}

func TestManualCompact(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	manager := session.sessionManager
	_, _ = manager.AppendMessage(model.NewUserText("first message", time.Now().UnixMilli()))
	first := testAssistant("first response", model.StopStop)
	first.Usage = model.Usage{Input: 100, TotalTokens: 100}
	_, _ = manager.AppendMessage(first)
	_, _ = manager.AppendMessage(model.NewUserText("second message", time.Now().UnixMilli()))
	second := testAssistant("second response", model.StopStop)
	second.Usage = model.Usage{Input: 100, TotalTokens: 100}
	_, _ = manager.AppendMessage(second)
	session.agent.SetMessages(manager.BuildSessionContext().Messages)

	result, err := session.Compact(context.Background(), "")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if result == nil || !strings.Contains(result.Summary, "compacted") {
		t.Fatalf("result = %+v", result)
	}
	if calls == 0 {
		t.Fatalf("summary calls = %d, want at least 1", calls)
	}
	last := session.sessionManager.GetBranch()
	lastEntry := last[len(last)-1]
	if lastEntry.EntryType() != "compaction" {
		t.Fatalf("last entry type = %s, want compaction", lastEntry.EntryType())
	}
}
