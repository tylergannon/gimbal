package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/runstore"
)

// Request identifies one prompt or interrupt independently of the conversation.
// Target is the current prompt request for an interrupt, never a session ID.
type Request struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Target  string `json:"target"`
	FileID  string `json:"file_id"`
}

// RequestState is reduced from admission, delivery and completion events.
type RequestState struct {
	Request
	Status   string `json:"status"`
	Delivery string `json:"delivery"`
	Error    string `json:"error"`
}

// Submit admits work and returns its identity before the native operation ends.
// The host context owns execution, so disconnecting the page never cancels it.
func (m *Manager) Submit(id string, request Request) (RequestState, error) {
	if request.ID == "" {
		return RequestState{}, errors.New("conversation: request ID required")
	}
	if request.Kind != "prompt" && request.Kind != "interrupt" && request.Kind != "finish" {
		return RequestState{}, errors.New("conversation: invalid input kind")
	}
	m.workflowMu.Lock()
	defer m.workflowMu.Unlock()
	m.mu.Lock()
	item := m.items[id]
	if item == nil || m.closing {
		m.mu.Unlock()
		return RequestState{}, errors.New("conversation: unavailable")
	}
	for _, old := range item.Requests {
		if old.ID == request.ID {
			m.mu.Unlock()
			if old.Request != request {
				return RequestState{}, errors.New("conversation: conflicting request")
			}
			return old, nil
		}
	}
	if m.WorkflowStart == nil || m.WorkflowInput == nil {
		m.mu.Unlock()
		return RequestState{}, errors.New("conversation: workflow host unavailable")
	}
	if request.FileID != "" && (request.Kind != "prompt" || len(item.Requests) > 0) {
		m.mu.Unlock()
		return RequestState{}, errors.New("conversation: file context is only available on the first message")
	}
	start := item.WorkflowRun == ""
	if start {
		item.WorkflowRun = string(runstore.NewRunID())
		if err := m.save(item); err != nil {
			m.mu.Unlock()
			return RequestState{}, err
		}
	}
	snapshot := clone(item)
	m.mu.Unlock()
	message := request.Message
	if request.FileID != "" {
		path, name, err := m.filePath(request.FileID)
		if err != nil {
			return RequestState{}, err
		}
		message += "\n\nRead the attached file " + name + " at " + path + "."
	}
	if start {
		if err := m.WorkflowStart(m.ctx, snapshot); err != nil {
			return RequestState{}, err
		}
	}
	raw, _ := json.Marshal(map[string]any{"id": request.ID, "message": message, "finish": request.Kind == "finish", "stop": request.Kind == "interrupt"})
	// The runner announces its controller before its Go body installs the
	// first Interactive owner. Keep this first handoff in the same request,
	// with one run and one input identity; only the ready owner can admit it.
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := m.WorkflowInput(snapshot.WorkflowRun, "conversation", string(raw))
		if err == nil {
			break
		}
		if !start || !strings.Contains(err.Error(), "interaction owner is starting or unavailable") || time.Now().After(deadline) {
			return RequestState{}, err
		}
		select {
		case <-m.ctx.Done():
			return RequestState{}, context.Cause(m.ctx)
		case <-time.After(10 * time.Millisecond):
		}
	}
	state := RequestState{Request: request, Status: "accepted", Delivery: "durable"}
	m.mu.Lock()
	defer m.mu.Unlock()
	item.Requests = append(item.Requests, state)
	if request.Kind == "prompt" {
		item.Messages = append(item.Messages, Message{Role: "user", Text: request.Message, Created: request.Created})
	}
	return state, m.save(item)
}
