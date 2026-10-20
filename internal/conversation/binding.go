package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/runstore"
	"time"
)

// RoleSpec is the model the conversation workflow's role runs on. It refuses a
// saved model that resolves to a different harness than the selected provider,
// so a mismatch fails before any adapter starts.
func RoleSpec(item Conversation) (string, error) {
	resolved, err := modelalias.Resolve(modelalias.Selection{Name: item.Model})
	if err != nil {
		return "", fmt.Errorf("conversation: %s model %q: %w", item.Provider, item.Model, err)
	}
	if resolved.Harness != item.Provider {
		return "", fmt.Errorf("conversation: %s session has model %q, which runs on %s; start a new session to choose its model again", item.Provider, item.Model, resolved.Harness)
	}
	if item.Effort != "" {
		return item.Model + ":" + item.Effort, nil
	}
	return item.Model, nil
}

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
