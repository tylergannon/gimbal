package conversation

import (
	"encoding/json"
	"errors"
	"github.com/tylergannon/gimbal/runstore"
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
	if item.ActiveRequest != "" || item.CleanupPending || item.Contact == "unreachable" || (item.Status != "idle" && item.Status != "canceled" && item.Status != "waiting") {
		return Conversation{}, errors.New("conversation: wait for a settled idle session before changing model")
	}
	active := m.active[id]
	if active != nil && active.request != "" {
		return Conversation{}, errors.New("conversation: a turn is still running")
	}
	if item.WorkflowRun != "" {
		if m.WorkflowInput == nil {
			return Conversation{}, errors.New("conversation: workflow host unavailable")
		}
		raw, _ := json.Marshal(map[string]any{"id": string(runstore.NewRecordID()), "configure": true, "model": model, "effort": effort})
		if err := m.WorkflowInput(item.WorkflowRun, "conversation", string(raw)); err != nil {
			return Conversation{}, err
		}
	}
	before := *item
	item.Model, item.Effort, item.Updated = model, effort, time.Now().UnixMilli()
	item.Revision++
	if err := m.save(item); err != nil {
		*item = before
		return Conversation{}, err
	}
	return clone(item), nil
}
