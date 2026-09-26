package session

import (
	"context"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

func TestAutoRetryRecovers(t *testing.T) {
	settings := config.NewInMemorySettingsManager(config.Settings{
		"retry": map[string]any{"enabled": true, "maxRetries": 2, "baseDelayMs": 1, "maxAgentDelayMs": 1},
	}, config.CreateOptions{})

	var mu sync.Mutex
	calls := 0
	stream := func(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		eventStream := wire.NewAssistantMessageEventStream()
		if call == 1 {
			failure := testAssistant("", model.StopError)
			failure.ErrorMessage = "overloaded"
			eventStream.Push(model.AssistantMessageEvent{Type: model.EventError, Reason: model.StopError, Error: failure})
			return eventStream, nil
		}
		eventStream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: model.StopStop, Message: testAssistant("recovered", model.StopStop)})
		return eventStream, nil
	}

	session := newTestSession(t, testSessionOptions{settings: settings, stream: stream})
	var retryStarts, retryEnds int
	session.Subscribe(func(event Event) error {
		if event.Type == EventAutoRetryStart {
			retryStarts++
		}
		if event.Type == EventAutoRetryEnd {
			retryEnds++
		}
		return nil
	})

	if err := session.Prompt(context.Background(), "hello", nil); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	mu.Lock()
	total := calls
	mu.Unlock()
	if total != 2 {
		t.Fatalf("model calls = %d, want 2", total)
	}
	if retryStarts != 1 || retryEnds != 1 {
		t.Fatalf("retry starts=%d ends=%d, want 1/1", retryStarts, retryEnds)
	}
}
