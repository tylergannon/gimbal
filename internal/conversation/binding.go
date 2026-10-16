package conversation

import (
	"errors"
	"time"
)

// ConfigureModel changes the next idle local Codex turn while preserving its native thread.
func (m *Manager) ConfigureModel(id, model, effort string) (Conversation, error) {
	model, err := selectModel("codex", model, effort)
	if err != nil {
		return Conversation{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	if item == nil || m.closing {
		return Conversation{}, errors.New("conversation: unavailable")
	}
	if item.Provider != "codex" || remotePlacement(item) || !item.Live {
		return Conversation{}, errors.New("conversation: model changes are available only for live local Codex sessions")
	}
	if item.ActiveRequest != "" || item.CleanupPending || item.Contact == "unreachable" || (item.Status != "idle" && item.Status != "canceled") {
		return Conversation{}, errors.New("conversation: wait for a settled idle session before changing model")
	}
	for _, request := range item.Requests {
		if request.Status == "accepted" || request.Status == "running" {
			return Conversation{}, errors.New("conversation: wait for queued requests before changing model")
		}
	}
	active := m.active[id]
	if active != nil && active.request != "" {
		return Conversation{}, errors.New("conversation: a turn is still running")
	}
	if active != nil && active.adapter != nil && active.session != "" {
		if _, ok := active.adapter.(persistentSessionAdapter); !ok {
			return Conversation{}, errors.New("conversation: native session cannot update its model")
		}
	}
	before := *item
	item.Model, item.Effort, item.Updated = model, effort, time.Now().UnixMilli()
	item.Revision++
	if err := m.save(item); err != nil {
		*item = before
		return Conversation{}, err
	}
	// Existing ResumeSession refreshes the adapter cache under the same native ID.
	// Do not detach/close the thread or discard its conversation context.
	if active != nil && active.session != "" {
		active.resume = true
	}
	return clone(item), nil
}
