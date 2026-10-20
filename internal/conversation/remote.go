package conversation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/sessionstate"
	"github.com/tylergannon/polytype"
)

type remoteState struct {
	Target           environments.Target               `json:"target"`
	Session          backend.MessageRequest            `json:"session"`
	Preparation      *NewConversation                  `json:"preparation,omitempty"`
	PrepareAttempted bool                              `json:"prepare_attempted"`
	SessionAttempted bool                              `json:"session_attempted"`
	SessionCreated   bool                              `json:"session_created"`
	CleanupFailed    bool                              `json:"cleanup_failed,omitempty"`
	TurnAttempted    map[string]bool                   `json:"turn_attempted,omitempty"`
	Requests         map[string]backend.MessageRequest `json:"requests"`
	Cursors          map[string]int                    `json:"cursors"`
	Seen             map[string]bool                   `json:"seen"`
}

func remotePlacement(item *Conversation) bool {
	return item.Placement.Present && item.Placement.Value.Machine.ID != "" && item.Placement.Value.Machine.ID != environments.Local
}

func localCapabilities() environments.Capabilities {
	return environments.Capabilities{Steer: true, Fork: true, Worktree: true, Browse: true, Clone: true}
}

// SetEnvironments attaches the instance-owned placement service without making
// network calls under the host's project-admission lock.
func (m *Manager) SetEnvironments(service *environments.Service, projectID string) {
	m.mu.Lock()
	m.environments, m.projectID = service, projectID
	m.mu.Unlock()
}

func (m *Manager) createRemote(ctx context.Context, in NewConversation, provider, model string) (Conversation, error) {
	m.mu.RLock()
	service, projectID := m.environments, m.projectID
	m.mu.RUnlock()
	if service == nil {
		return Conversation{}, errors.New("conversation: environment service unavailable")
	}
	normalized := in
	normalized.Provider = provider
	normalized.Model = model
	normalized.Title = strings.TrimSpace(in.Title)
	var item *Conversation
	var state *remoteState
	id := m.pendingRemotePreparation(in, provider, model)
	var err error
	if id != "" {
		m.mu.RLock()
		item = m.items[id]
		state = m.remoteStates[id]
		m.mu.RUnlock()
	} else {
		target, resolveErr := service.Resolve(ctx, projectID, in.Environment)
		if resolveErr != nil {
			return Conversation{}, resolveErr
		}
		id, err = randomID()
		if err != nil {
			return Conversation{}, err
		}
		now := time.Now().UnixMilli()
		item = &Conversation{ID: id, Title: normalized.Title, Provider: provider, Model: model, Effort: in.Effort, SharedCheckout: in.SharedCheckout, Base: in.Base, Environment: in.Environment, Placement: polytype.Nullable[Placement]{Present: true, Value: Placement{Machine: PlacementMachine{ID: target.Machine.ID, Name: target.Machine.Name, Kind: target.Machine.Kind}, Environment: environments.DescribeEnvironment(target.Environment), Directory: target.Directory}}, Contact: "unknown", Capabilities: target.Capabilities, CleanupPending: true, Status: "preparing", Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{}}
		if item.Title == "" {
			item.Title = providerLabel(provider) + " conversation"
		}
		state = &remoteState{Target: target, Preparation: &normalized, Requests: map[string]backend.MessageRequest{}, Cursors: map[string]int{}, Seen: map[string]bool{}}
		m.mu.Lock()
		m.items[id] = item
		m.remoteStates[id] = state
		m.active[id] = &activeConversation{}
		m.projections[id] = sessionstate.New(sessionstate.NewProjectionState())
		err = m.save(item)
		m.mu.Unlock()
		if err != nil {
			return Conversation{}, err
		}
	}
	target := state.Target
	now := item.Created
	m.mu.RLock()
	admission := m.active[id]
	m.mu.RUnlock()
	if admission == nil {
		m.mu.Lock()
		admission = &activeConversation{}
		m.active[id] = admission
		m.mu.Unlock()
	}
	admission.admitMu.Lock()
	defer admission.admitMu.Unlock()
	m.mu.RLock()
	if state.Session.ID != "" {
		snapshot := clone(m.items[id])
		m.mu.RUnlock()
		return snapshot, nil
	}
	unavailable := m.closing || (item.Status != "preparing" && item.Status != "unresolved")
	m.mu.RUnlock()
	if unavailable {
		return Conversation{}, errors.New("conversation: retained preparation is no longer available")
	}
	owner := backend.Owner{Run: id, Scope: id, Task: "session"}
	binding, err := service.Acquire(ctx, target, owner)
	if err != nil {
		m.remotePreparationFailed(id, err, &binding)
		return Conversation{}, err
	}
	m.mu.Lock()
	state.PrepareAttempted = true
	err = m.save(item)
	m.mu.Unlock()
	if err != nil {
		m.remotePreparationFailed(id, err, &binding)
		return Conversation{}, err
	}
	branch := in.Branch
	choice := environments.Checkout{SharedCheckout: in.SharedCheckout, Branch: branch, Base: in.Base}
	prepared, err := service.Prepare(ctx, target, choice, id)
	var uncertain *backend.DeliveryUncertain
	if errors.As(err, &uncertain) && ctx.Err() == nil {
		// Re-read the persisted helper request once. Service.Prepare preserves its
		// envelope and never replays an attempted mutation to recover a lost result.
		prepared, err = service.Prepare(ctx, target, choice, id)
	}
	if err != nil {
		m.remotePreparationFailed(id, err, &binding)
		return Conversation{}, err
	}
	effort := in.Effort
	identity := backend.NewExecutionIdentity("session")
	session := backend.MessageRequest{ExecutionIdentity: identity, ID: identity.RequestID, Created: time.UnixMilli(now), Owner: owner, Kind: "identified-session", SessionID: "session", Session: &backend.Session{ExecutionIdentity: identity, Owner: owner, Harness: provider, Model: model, Effort: effort, Workdir: prepared.Directory}}
	m.mu.Lock()
	item = m.items[id]
	item.Worktree, item.Branch = prepared.Directory, prepared.Branch
	item.Placement.Value.Directory = prepared.Directory
	item.Error = ""
	item.Revision++
	item.Status, item.Contact, item.CleanupPending = StatusIdle, "connected", false
	item.Live = true
	item.Capabilities = prepared.Target.Capabilities
	state.Session = session
	active := m.active[id]
	active.remote = &binding
	err = m.save(item)
	snapshot := clone(item)
	m.mu.Unlock()
	return snapshot, err
}

// Retrying an explicitly submitted setup reconciles its retained preparation;
// it does not mint another checkout identity after a lost helper result.
func (m *Manager) pendingRemotePreparation(in NewConversation, provider, model string) string {
	in.Provider = provider
	in.Model = model
	in.Title = strings.TrimSpace(in.Title)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, state := range m.remoteStates {
		item := m.items[id]
		if item != nil && item.Status == "unresolved" && item.CleanupPending && state.Session.ID == "" && state.Preparation != nil && *state.Preparation == in {
			return id
		}
	}
	return ""
}

func (m *Manager) remotePreparationFailed(id string, cause error, binding *environments.Binding) {
	var uncertain *backend.DeliveryUncertain
	if errors.As(cause, &uncertain) {
		m.mu.Lock()
		defer m.mu.Unlock()
		item := m.items[id]
		if binding != nil {
			m.active[id].remote = binding
		}
		item.Status, item.Contact, item.CleanupPending, item.Live = "unresolved", "unreachable", true, true
		item.Error = fmt.Sprintf("Preparation for conversation %s is unconfirmed. Retry the same session setup to reconcile it, or end this session to release ownership: %v", id, cause)
		item.Revision++
		_ = m.save(item)
		return
	}

	cleanupErr := error(nil)
	if binding != nil {
		m.mu.Lock()
		prepared := m.remoteStates[id].PrepareAttempted
		m.active[id].remote = binding
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(m.ctx), 15*time.Second)
		if prepared {
			ctx = effects.WithConnection(ctx, binding.Connection)
			cleanupErr = effects.Cleanup(ctx, "backend", id, id, backend.NewExecutionIdentity("control").RequestID)
		}
		// No helper/session mutation was attempted when acquisition itself failed.
		// Releasing only this owner is safe even when readiness never succeeded.
		if cleanupErr == nil {
			cleanupErr = m.environments.Release(ctx, *binding)
		}
		cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	if cleanupErr == nil && binding != nil {
		delete(m.items, id)
		delete(m.remoteStates, id)
		delete(m.active, id)
		delete(m.projections, id)
		_ = os.Remove(filepath.Join(m.projectDir, "conversations", id+".json"))
		return
	}
	item.Status, item.Error, item.CleanupPending = "unresolved", errors.Join(cause, cleanupErr).Error(), true
	item.Contact = "unreachable"
	item.Revision++
	_ = m.save(item)
}

// Cleanup requests ordinary workflow completion through durable ingress.
func (m *Manager) Cleanup(ctx context.Context, id string) error {
	_, err := m.Submit(id, Request{ID: ulid.Make().String(), Kind: "finish"})
	return err
}

func (m *Manager) createLocalEnvironment(ctx context.Context, in NewConversation, provider, model string, target environments.Target) (Conversation, error) {
	id, err := randomID()
	if err != nil {
		return Conversation{}, err
	}
	branch := in.Branch
	prepared, err := m.environments.Prepare(ctx, target, environments.Checkout{SharedCheckout: in.SharedCheckout, Branch: branch, Base: in.Base}, id)
	if err != nil {
		return Conversation{}, err
	}
	if _, err = m.factory(provider); err != nil {
		return Conversation{}, err
	}
	now := time.Now().UnixMilli()
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = providerLabel(provider) + " conversation"
	}
	item := &Conversation{ID: id, Title: title, Provider: provider, Model: model, Effort: in.Effort, SharedCheckout: in.SharedCheckout, Base: in.Base, Branch: prepared.Branch, Worktree: prepared.Directory, Live: true, Status: StatusIdle, Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{}, Environment: in.Environment, Contact: "connected", Capabilities: prepared.Target.Capabilities, Placement: polytype.Nullable[Placement]{Present: true, Value: Placement{Machine: PlacementMachine{ID: environments.Local, Name: "This Mac", Kind: "local"}, Environment: environments.DescribeEnvironment(target.Environment), Directory: prepared.Directory}}}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return Conversation{}, errors.New("conversation: runtime is shutting down")
	}
	if err = m.save(item); err != nil {
		return Conversation{}, err
	}
	m.items[id] = item
	m.active[id] = &activeConversation{}
	m.projections[id] = sessionstate.New(sessionstate.NewProjectionState())
	return clone(item), nil
}
