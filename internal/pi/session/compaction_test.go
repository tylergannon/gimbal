package session

import (
	"context"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

func compactionSettingsManager(reserve, keepRecent int) *config.SettingsManager {
	return config.NewInMemorySettingsManager(config.Settings{
		"compaction": map[string]any{
			"reserveTokens":    reserve,
			"keepRecentTokens": keepRecent,
		},
	}, config.CreateOptions{})
}

func countingStream(calls *int) model.StreamFunction {
	base := doneStream(testAssistant("compacted", model.StopStop))
	return func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		*calls++
		return base(ctx, m, transcript, options)
	}
}

func TestAutoCompactionQueueResume(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	manager := session.sessionManager
	_, _ = manager.AppendMessage(model.NewUserText("message to compact", time.Now().UnixMilli()))
	assistant := testAssistant("assistant response to compact", model.StopStop)
	assistant.Usage = model.Usage{Input: 100, TotalTokens: 100}
	_, _ = manager.AppendMessage(assistant)
	session.agent.SetMessages(manager.BuildSessionContext().Messages)

	session.agent.FollowUp(model.NewCustomText("test", "Queued custom", false, nil, time.Now().UnixMilli()))
	if session.PendingMessageCount() != 0 {
		t.Fatalf("pending = %d, want 0", session.PendingMessageCount())
	}
	if !session.agent.HasQueuedMessages() {
		t.Fatalf("agent has no queued messages")
	}

	cont, err := session.runAutoCompaction(context.Background(), CompactionThreshold, false)
	if err != nil {
		t.Fatalf("run auto compaction: %v", err)
	}
	if !cont {
		t.Fatalf("run auto compaction did not request continuation")
	}
	if calls != 1 {
		t.Fatalf("summary calls = %d, want 1", calls)
	}
}

func TestOverflowRecoverySingleAttempt(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	manager := session.sessionManager
	_, _ = manager.AppendMessage(model.NewUserText("before overflow", time.Now().UnixMilli()))
	assistant := testAssistant("prior", model.StopStop)
	assistant.Usage = model.Usage{Input: 100, TotalTokens: 100}
	_, _ = manager.AppendMessage(assistant)

	events := make([]Event, 0)
	session.Subscribe(func(event Event) error {
		if event.Type == EventCompactionEnd {
			events = append(events, event)
		}
		return nil
	})

	overflow := testAssistant("", model.StopError)
	overflow.ErrorMessage = "prompt is too long"
	overflow.Timestamp = time.Now().UnixMilli()

	if _, err := session.checkCompaction(context.Background(), overflow, "", nil, nil, true); err != nil {
		t.Fatalf("first check: %v", err)
	}
	second := testAssistant("", model.StopError)
	second.ErrorMessage = "prompt is too long"
	second.Timestamp = time.Now().UnixMilli() + 100_000
	if _, err := session.checkCompaction(context.Background(), second, "", nil, nil, true); err != nil {
		t.Fatalf("second check: %v", err)
	}

	if calls != 1 {
		t.Fatalf("summary calls = %d, want 1", calls)
	}
	found := false
	for _, event := range events {
		if event.ErrorMessage == "Context overflow recovery failed after one compact-and-retry attempt. Try reducing context or switching to a larger-context model." {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected overflow failure event, got %+v", events)
	}
}

func TestThresholdCompactionForErrorMessageUsesLastUsage(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	successful := testAssistant("large successful response", model.StopStop)
	successful.Usage = model.Usage{Input: 180_000, Output: 10_000, TotalTokens: 190_000}
	errorMessage := testAssistant("", model.StopError)
	errorMessage.ErrorMessage = "529 overloaded"
	errorMessage.Timestamp = successful.Timestamp + 1000

	session.agent.SetMessages([]model.AgentMessage{
		model.NewUserText("hello", successful.Timestamp-1000),
		successful,
		model.NewUserText("another prompt", successful.Timestamp+500),
		errorMessage,
	})
	for _, message := range session.agent.State().Messages {
		_, _ = session.sessionManager.AppendMessage(message)
	}

	if _, err := session.checkCompaction(context.Background(), errorMessage, "", nil, nil, true); err != nil {
		t.Fatalf("check: %v", err)
	}
	if calls != 1 {
		t.Fatalf("summary calls = %d, want 1", calls)
	}
}

func TestThresholdCompactionSkippedWithoutUsage(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	errorMessage := testAssistant("", model.StopError)
	errorMessage.ErrorMessage = "529 overloaded"
	session.agent.SetMessages([]model.AgentMessage{
		model.NewUserText("hello", errorMessage.Timestamp-1000),
		errorMessage,
	})

	if _, err := session.checkCompaction(context.Background(), errorMessage, "", nil, nil, true); err != nil {
		t.Fatalf("check: %v", err)
	}
	if calls != 0 {
		t.Fatalf("summary calls = %d, want 0", calls)
	}
}

func TestThresholdCompactionIgnoresPreCompactionUsage(t *testing.T) {
	var calls int
	settings := compactionSettingsManager(16384, 1)
	session := newTestSession(t, testSessionOptions{settings: settings, summary: countingStream(&calls)})

	manager := session.sessionManager
	preTimestamp := time.Now().UnixMilli() - 10_000
	kept := testAssistant("kept response", model.StopStop)
	kept.Usage = model.Usage{Input: 180_000, Output: 10_000, TotalTokens: 190_000}
	kept.Timestamp = preTimestamp
	_, _ = manager.AppendMessage(model.NewUserText("before compaction", preTimestamp-1000))
	_, _ = manager.AppendMessage(kept)
	entries := manager.GetEntries()
	firstKept := entries[0].Base().ID
	_, _ = manager.AppendCompaction("summary", &firstKept, kept.Usage.TotalTokens, nil, false, nil)

	errorMessage := testAssistant("", model.StopError)
	errorMessage.ErrorMessage = "529 overloaded"
	errorMessage.Timestamp = time.Now().UnixMilli() + 100_000

	session.agent.SetMessages([]model.AgentMessage{
		model.NewUserText("kept user msg", preTimestamp-1000),
		kept,
		model.NewUserText("new prompt", time.Now().UnixMilli()-500),
		errorMessage,
	})

	if _, err := session.checkCompaction(context.Background(), errorMessage, "", nil, nil, true); err != nil {
		t.Fatalf("check: %v", err)
	}
	if calls != 0 {
		t.Fatalf("summary calls = %d, want 0", calls)
	}
}
