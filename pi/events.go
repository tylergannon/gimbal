package pi

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/session"
)

// turn projects one native run's events into Gimbal agent events and records
// the final assistant message and normalized usage.
type turn struct {
	mu      sync.Mutex
	session string
	model   string
	emit    func(gimbal.AgentEvent) error

	settled     chan struct{}
	settledOnce sync.Once
	final       *model.AssistantMessage
	usage       map[string]gimbal.Usage
	err         error
	waiters     []*steerWaiter
}

// steerWaiter is one Steer call waiting for its message to be consumed.
type steerWaiter struct {
	text      string
	delivered chan struct{}
	once      sync.Once
}

func newTurn(sessionID, modelName string, emit func(gimbal.AgentEvent) error) *turn {
	return &turn{
		session: sessionID,
		model:   modelName,
		emit:    emit,
		settled: make(chan struct{}),
		usage:   map[string]gimbal.Usage{},
	}
}

// record consumes one session event. It always returns nil: an observer
// failure never aborts the native run.
func (t *turn) record(event session.Event) error {
	switch event.Type {
	case session.EventAgentSettled:
		t.mu.Lock()
		t.settledOnce.Do(func() { close(t.settled) })
		t.mu.Unlock()
		return nil
	case session.EventQueueUpdate:
		t.deliverQueued(event.Steering, event.FollowUp)
		return nil
	}
	switch event.Agent.Type {
	case model.EvMessageUpdate:
		t.recordMessageUpdate(event.Agent)
	case model.EvToolExecutionStart:
		t.recordToolStart(event.Agent)
	case model.EvToolExecutionEnd:
		t.recordToolEnd(event.Agent)
	case model.EvMessageEnd:
		t.recordMessageEnd(event.Agent)
	case model.EvAgentEnd:
		t.recordAgentEnd(event.Agent)
	}
	return nil
}

func (t *turn) recordMessageUpdate(event model.AgentEvent) {
	update := event.AssistantMessageEvent
	if update == nil {
		return
	}
	switch update.Type {
	case model.EventTextDelta:
		t.emitEvent("session.text.delta", map[string]any{
			"sessionID": t.session, "ordinal": update.ContentIndex, "delta": update.Delta,
		})
	case model.EventToolCallStart:
		id, name := toolCallIdentity(update.Partial, update.ContentIndex)
		t.emitEvent("session.tool.input.started", map[string]any{
			"sessionID": t.session, "id": id, "name": name,
		})
	}
}

func toolCallIdentity(partial *model.AssistantMessage, index int) (string, string) {
	if partial == nil || index < 0 || index >= len(partial.Content) {
		return "", ""
	}
	if call, ok := partial.Content[index].(model.ToolCall); ok {
		return call.ID, call.Name
	}
	return "", ""
}

func (t *turn) recordToolStart(event model.AgentEvent) {
	input, _ := json.Marshal(event.Args)
	t.emitEvent("session.tool.input.ended", map[string]any{
		"sessionID": t.session, "id": event.ToolCallID, "text": string(input),
	})
	t.emitEvent("session.tool.called", map[string]any{
		"sessionID": t.session, "id": event.ToolCallID, "name": event.ToolName,
		"input": event.Args, "executed": true,
	})
}

func (t *turn) recordToolEnd(event model.AgentEvent) {
	if event.IsError {
		t.emitEvent("session.tool.failed", map[string]any{
			"sessionID": t.session, "id": event.ToolCallID, "name": event.ToolName, "error": event.Result,
		})
		return
	}
	t.emitEvent("session.tool.success", map[string]any{
		"sessionID": t.session, "id": event.ToolCallID, "name": event.ToolName, "result": event.Result,
	})
}

func (t *turn) recordMessageEnd(event model.AgentEvent) {
	message, ok := assistantOf(event.Message)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	clone := *message
	t.final = &clone
	modelName := message.Model
	if modelName == "" {
		modelName = t.model
	}
	t.usage[modelName] = addUsage(t.usage[modelName], normalizedUsage(message.Usage))
}

func (t *turn) recordAgentEnd(event model.AgentEvent) {
	for _, message := range event.Messages {
		assistant, ok := assistantOf(message)
		if !ok || assistant.ErrorMessage == "" {
			continue
		}
		t.mu.Lock()
		t.err = fmt.Errorf("pi: provider error: %s", assistant.ErrorMessage)
		t.mu.Unlock()
	}
}

func assistantOf(message model.AgentMessage) (*model.AssistantMessage, bool) {
	switch value := message.(type) {
	case model.AssistantMessage:
		clone := value
		return &clone, true
	case *model.AssistantMessage:
		return value, true
	default:
		return nil, false
	}
}

func (t *turn) emitEvent(kind string, data any) {
	if t.emit == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	ref, err := json.Marshal(map[string]string{"provider": "pi", "sessionID": t.session})
	if err != nil {
		return
	}
	_ = t.emit(gimbal.AgentEvent{Type: kind, Data: payload, NativeRef: ref})
}

func (t *turn) addWaiter(waiter *steerWaiter) {
	t.mu.Lock()
	t.waiters = append(t.waiters, waiter)
	t.mu.Unlock()
}

func (t *turn) removeWaiter(waiter *steerWaiter) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, candidate := range t.waiters {
		if candidate == waiter {
			t.waiters = append(t.waiters[:i], t.waiters[i+1:]...)
			return
		}
	}
}

// deliverQueued marks every waiter whose message has left the native queues.
func (t *turn) deliverQueued(steering, followUp []string) {
	pending := append(append([]string(nil), steering...), followUp...)
	t.mu.Lock()
	defer t.mu.Unlock()
	remaining := t.waiters[:0]
	for _, waiter := range t.waiters {
		if slices.Contains(pending, waiter.text) {
			remaining = append(remaining, waiter)
			continue
		}
		waiter.once.Do(func() { close(waiter.delivered) })
	}
	t.waiters = remaining
}

func (t *turn) finalMessage() *model.AssistantMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.final
}

func (t *turn) usageReport() map[string]gimbal.Usage {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.usage) == 0 {
		return nil
	}
	out := make(map[string]gimbal.Usage, len(t.usage))
	maps.Copy(out, t.usage)
	return out
}

func normalizedUsage(usage model.Usage) gimbal.Usage {
	var tokens gimbal.Tokens
	tokens.Input = float64(usage.Input)
	tokens.Output = max(0, float64(usage.Output-usage.Reasoning))
	tokens.Reasoning = float64(usage.Reasoning)
	tokens.Cache.Read = float64(usage.CacheRead)
	tokens.Cache.Write = float64(usage.CacheWrite)
	return gimbal.Usage{Cost: usage.Cost.Total, Tokens: tokens}
}

func addUsage(a, b gimbal.Usage) gimbal.Usage {
	a.Cost += b.Cost
	a.Tokens.Input += b.Tokens.Input
	a.Tokens.Output += b.Tokens.Output
	a.Tokens.Reasoning += b.Tokens.Reasoning
	a.Tokens.Cache.Read += b.Tokens.Cache.Read
	a.Tokens.Cache.Write += b.Tokens.Cache.Write
	return a
}
