package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/sessionstate"
)

// Request identifies one prompt or interrupt independently of the conversation.
// Target is the current prompt request for an interrupt, never a session ID.
type Request struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

// RequestState is reduced from admission, delivery and completion events.
type RequestState struct {
	Request
	Status   string `json:"status"`
	Delivery string `json:"delivery"`
	Error    string `json:"error"`
}

type requestEvent struct {
	kind     string
	request  Request
	delivery string
	err      error
}

// Submit admits work and returns its identity before the native operation ends.
// The host context owns execution, so disconnecting the page never cancels it.
func (m *Manager) Submit(id string, r Request) (RequestState, error) {
	if r.ID == "" || r.Created <= 0 {
		return RequestState{}, errors.New("conversation: request ID and creation time required")
	}
	if r.Kind != "prompt" && r.Kind != "interrupt" {
		return RequestState{}, errors.New("conversation: prompt or interrupt required")
	}
	if r.Kind == "prompt" && strings.TrimSpace(r.Message) == "" {
		return RequestState{}, errors.New("conversation: message is blank")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	if item == nil || m.closing || (remotePlacement(item) && !item.Live) {
		return RequestState{}, errors.New("conversation: unavailable")
	}
	for _, previous := range item.Requests {
		if previous.ID == r.ID {
			if previous.Request != r {
				return RequestState{}, errors.New("conversation: request ID already used for different work")
			}
			return previous, nil
		}
	}
	if (item.Status == "unresolved" || item.CleanupPending) && r.Kind == "prompt" {
		return RequestState{}, errors.New("conversation: the previous request's outcome is unconfirmed; do not start replacement work")
	}
	active := m.active[id]
	if active == nil && (item.NativeSession != "" || remotePlacement(item)) {
		active = &activeConversation{session: item.NativeSession, resume: true}
		m.active[id] = active
	}
	if active == nil {
		return RequestState{}, errors.New("conversation: this saved native session is no longer live")
	}
	if r.Kind == "interrupt" && (r.Target == "" || active.request != r.Target) {
		return RequestState{}, errors.New("conversation: that request is no longer running")
	}
	if err := m.reduce(item, requestEvent{kind: "accepted", request: r}); err != nil {
		return RequestState{}, err
	}
	if r.Kind == "interrupt" {
		if err := m.reduce(item, requestEvent{kind: "stopping", request: r}); err != nil {
			return RequestState{}, err
		}
		if remotePlacement(item) {
			m.remoteReaders.Add(1)
			go func() { defer m.remoteReaders.Done(); m.remoteInterrupt(id, r) }()
		} else if active.cancel != nil {
			active.cancel()
		}
	} else {
		target := active.request
		adapter, session := active.adapter, active.session
		if remotePlacement(item) {
			m.remoteReaders.Add(1)
			go func() { defer m.remoteReaders.Done(); m.deliverRemote(id, r, active, target) }()
		} else {
			go m.deliver(id, r, active, target, adapter, session)
		}
	}
	return item.Requests[len(item.Requests)-1], nil
}

func (m *Manager) deliver(id string, r Request, active *activeConversation, target string, adapter gimbal.HarnessAdapter, session string) {
	if target != "" && adapter != nil && session != "" {
		active.steerMu.Lock()
		m.mu.Lock()
		stillCurrent := active.request == target
		m.mu.Unlock()
		if !stillCurrent {
			active.steerMu.Unlock()
		} else {
			m.mu.Lock()
			_ = m.reduce(m.items[id], requestEvent{kind: "delivery", request: r, delivery: "steer"})
			m.mu.Unlock()
			landed, err := adapter.Steer(m.ctx, session, r.Message)
			active.steerMu.Unlock()
			if err != nil {
				m.finish(id, r, "unresolved", err)
				return
			}
			if landed {
				m.mu.Lock()
				item := m.items[id]
				_ = m.userEvent(item, r, "steer")
				_ = m.reduce(item, requestEvent{kind: "completed", request: r, delivery: "steer"})
				m.mu.Unlock()
				return
			}
		}
	}
	active.turnMu.Lock()
	defer active.turnMu.Unlock()
	active.steerMu.Lock()
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()
	m.mu.Lock()
	item := m.items[id]
	if m.closing || item.Status == "unresolved" {
		m.mu.Unlock()
		active.steerMu.Unlock()
		m.finish(id, r, "unresolved", errors.New("conversation: native outcome unconfirmed or server shutting down"))
		return
	}
	active.request, active.cancel = r.ID, cancel
	_ = m.reduce(item, requestEvent{kind: "started", request: r, delivery: "turn"})
	_ = m.userEvent(item, r, "turn")
	provider, model, worktree := item.Provider, item.Model, item.Worktree
	m.mu.Unlock()
	active.steerMu.Unlock()
	if active.adapter == nil {
		resolved, err := m.factory(provider)
		if err != nil {
			m.finish(id, r, "failed", err)
			return
		}
		m.mu.Lock()
		active.adapter = resolved
		m.mu.Unlock()
	}
	_, effort, _ := providerModel(provider, model)
	if active.resume {
		resumer, ok := active.adapter.(persistentSessionAdapter)
		if !ok {
			m.finish(id, r, "failed", errors.New("conversation: this provider cannot resume a saved native session"))
			return
		}
		if err := resumer.ResumeSession(ctx, active.session, model, effort, worktree); err != nil {
			m.finish(id, r, "failed", err)
			return
		}
		active.resume = false
	}
	if active.session == "" {
		native, err := active.adapter.CreateSession(ctx, model, effort, worktree)
		if err != nil {
			m.finish(id, r, "failed", err)
			return
		}
		m.mu.Lock()
		active.session = native
		if _, ok := active.adapter.(persistentSessionAdapter); ok {
			item.NativeSession = native
			_ = m.save(item)
		}
		m.mu.Unlock()
	}
	prompt := fmt.Sprintf(conversationPrompt, m.cli, m.instanceDir, m.project, worktree, id) + r.Message
	result, err := active.adapter.RunTurn(ctx, active.session, prompt, nil, func(e gimbal.AgentEvent) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Provider message IDs may repeat on later turns; request identity isolates them.
		var data map[string]any
		if json.Unmarshal(e.Data, &data) != nil {
			return nil
		}
		data["sessionID"] = id
		if mid, ok := data["assistantMessageID"].(string); ok {
			data["assistantMessageID"] = r.ID + "/" + mid
		}
		e.Data, _ = json.Marshal(data)
		_ = m.activity(item, e)
		return nil // Observation errors never replace native execution's outcome.
	})
	if err != nil {
		var uncertain *backend.DeliveryUncertain
		switch {
		case errors.As(err, &uncertain):
			m.finish(id, r, "unresolved", err)
		case errors.Is(err, context.Canceled):
			m.finish(id, r, "canceled", nil)
		default:
			m.finish(id, r, "failed", err)
		}
		return
	}
	var text string
	if err = json.Unmarshal(result.Output, &text); err != nil || strings.TrimSpace(text) == "" {
		m.finish(id, r, "failed", errors.New("provider returned an invalid or empty reply"))
		return
	}
	m.mu.Lock()
	m.assistantReply(item, r, text)
	m.mu.Unlock()
	m.finish(id, r, "completed", nil)
}

func (m *Manager) assistantReply(item *Conversation, r Request, text string) {
	id := item.ID
	// Some harnesses return a final answer without streaming it. Preserve that answer.
	state := m.projections[id].Snapshot().State
	hasText := false
	for _, row := range state.Message[id] {
		mid, _ := row.Get("id").(string)
		if !strings.HasPrefix(mid, r.ID+"/") {
			continue
		}
		parts, _ := row.Get("content").([]any)
		for _, part := range parts {
			if p, ok := part.(*sessionstate.Obj); ok && p.Get("type") == "text" && p.Get("text") == text {
				hasText = true
			}
		}
	}
	if !hasText {
		mid := r.ID + "/reply"
		for _, event := range []struct {
			kind string
			data map[string]any
		}{
			{"session.step.started", map[string]any{"assistantMessageID": mid}},
			{"session.text.started", map[string]any{"assistantMessageID": mid}},
			{"session.text.ended", map[string]any{"assistantMessageID": mid, "text": text}},
		} {
			event.data["sessionID"] = id
			raw, _ := json.Marshal(event.data)
			_ = m.activity(item, gimbal.AgentEvent{Type: event.kind, Data: raw})
		}
	}
	item.Messages = append(item.Messages, Message{Role: "assistant", Text: text, Created: time.Now().UnixMilli()})
}

func (m *Manager) finish(id string, r Request, kind string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finishLocked(id, r, kind, err)
}

func (m *Manager) finishLocked(id string, r Request, kind string, err error) {
	item := m.items[id]
	var uncertain *backend.DeliveryUncertain
	if errors.Is(err, context.Canceled) && !errors.As(err, &uncertain) {
		kind, err = "canceled", nil
	}
	_ = m.reduce(item, requestEvent{kind: kind, request: r, err: err})
	active := m.active[id]
	if active == nil {
		return
	}
	if kind == "unresolved" && remotePlacement(item) {
		return
	}
	if active.request != r.ID {
		return
	}
	active.request, active.cancel = "", nil
	for _, control := range item.Requests {
		if control.Kind == "interrupt" && control.Target == r.ID && (control.Status == "accepted" || control.Status == "unresolved") {
			result := "completed"
			if kind == "unresolved" {
				result = "unresolved"
			}
			_ = m.reduce(item, requestEvent{kind: result, request: control.Request, err: err})
		}
	}
}

// reduce is the single lifecycle state transition path. Native transcript
// events use the same session projection as workflow agent transcripts.
func (m *Manager) reduce(item *Conversation, e requestEvent) error {
	var row *RequestState
	for i := range item.Requests {
		if item.Requests[i].ID == e.request.ID {
			row = &item.Requests[i]
			break
		}
	}
	if e.kind == "accepted" {
		item.Requests = append(item.Requests, RequestState{Request: e.request, Status: "accepted"})
		row = &item.Requests[len(item.Requests)-1]
	}
	if row == nil {
		return errors.New("conversation: event for unknown request")
	}
	switch e.kind {
	case "delivery":
		row.Delivery = e.delivery
	case "started":
		row.Status, row.Delivery = "running", e.delivery
		item.Status, item.Error, item.ActiveRequest = StatusWorking, "", row.ID
	case "stopping":
		item.Status = "stopping"
	default:
		if e.kind != "accepted" {
			row.Status = e.kind
		}
		if e.delivery != "" {
			row.Delivery = e.delivery
		}
		if e.err != nil {
			row.Error = e.err.Error()
		}
		if item.ActiveRequest == row.ID {
			if e.kind != "unresolved" {
				item.ActiveRequest = ""
			}
			item.Status, item.Error = StatusIdle, ""
			if e.kind == "failed" {
				item.Status, item.Error = StatusError, row.Error
			}
			if e.kind == "unresolved" {
				item.Status, item.Error = "unresolved", row.Error
			}
			if e.kind == "canceled" {
				item.Status = "canceled"
			}
			ending := "session.execution.succeeded"
			if e.kind == "canceled" {
				ending = "session.execution.interrupted"
			}
			if e.kind == "failed" {
				ending = "session.execution.failed"
			}
			if e.kind != "unresolved" {
				raw, _ := json.Marshal(map[string]any{"sessionID": item.ID})
				_ = m.activity(item, gimbal.AgentEvent{Type: ending, Data: raw})
			}
		}
	}
	item.Revision++
	item.Updated = time.Now().UnixMilli()
	return m.save(item)
}

func (m *Manager) userEvent(item *Conversation, r Request, delivery string) error {
	data, _ := json.Marshal(map[string]any{"sessionID": item.ID, "inboxID": r.ID, "item": map[string]any{"type": "user", "payload": map[string]any{"text": r.Message, "steered": delivery == "steer", "requestID": r.ID}, "delivery": delivery}})
	_ = m.activity(item, gimbal.AgentEvent{Type: "session.inbox.enqueued", Data: data})
	data, _ = json.Marshal(map[string]any{"sessionID": item.ID, "inboxID": r.ID})
	_ = m.activity(item, gimbal.AgentEvent{Type: "session.inbox.delivered", Data: data})
	item.Messages = append(item.Messages, Message{Role: "user", Text: r.Message, Created: time.Now().UnixMilli()})
	return m.save(item)
}

func (m *Manager) activity(item *Conversation, e gimbal.AgentEvent) error {
	item.Revision++
	item.Updated = time.Now().UnixMilli()
	e.ID = fmt.Sprintf("evt_%s_%020d", item.ID, item.Revision)
	e.Created = item.Updated
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var event sessionstate.Obj
	if err = json.Unmarshal(raw, &event); err != nil {
		return err
	}
	m.projections[item.ID].Apply(&event)
	state, err := json.Marshal(m.projections[item.ID].Snapshot().State)
	if err != nil {
		return err
	}
	item.State = gimbal.JSONText(state)
	return m.save(item)
}
