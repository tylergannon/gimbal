package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/sessionstate"
	"github.com/tylergannon/polytype"
)

type remoteState struct {
	Target           environments.Target               `json:"target"`
	Session          backend.MessageRequest            `json:"session"`
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
	var restore []string
	for id, item := range m.items {
		if remotePlacement(item) && item.Live {
			restore = append(restore, id)
		}
	}
	m.mu.Unlock()
	for _, id := range restore {
		m.background(func() { m.reconcileRemote(id); m.resumeQueuedRemote(id) })
	}
}

func (m *Manager) createRemote(ctx context.Context, in NewConversation, provider, model string) (Conversation, error) {
	m.mu.RLock()
	service, projectID := m.environments, m.projectID
	m.mu.RUnlock()
	if service == nil {
		return Conversation{}, errors.New("conversation: environment service unavailable")
	}
	target, err := service.Resolve(ctx, projectID, in.Environment)
	if err != nil {
		return Conversation{}, err
	}
	id, err := randomID()
	if err != nil {
		return Conversation{}, err
	}
	now := time.Now().UnixMilli()
	item := &Conversation{ID: id, Title: strings.TrimSpace(in.Title), Provider: provider, Model: model, Environment: in.Environment, Placement: polytype.Nullable[Placement]{Present: true, Value: Placement{Machine: PlacementMachine{ID: target.Machine.ID, Name: target.Machine.Name, Kind: target.Machine.Kind}, Environment: environments.DescribeEnvironment(target.Environment), Directory: target.Directory}}, Contact: "unknown", Capabilities: target.Capabilities, CleanupPending: true, Status: "preparing", Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{}}
	if item.Title == "" {
		item.Title = providerLabel(provider) + " conversation"
	}
	state := &remoteState{Target: target, Requests: map[string]backend.MessageRequest{}, Cursors: map[string]int{}, Seen: map[string]bool{}}
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
	prepared, err := service.Prepare(ctx, target, environments.Checkout{SharedCheckout: in.SharedCheckout, Branch: branch, Base: in.Base}, id)
	if err != nil {
		m.remotePreparationFailed(id, err, &binding)
		return Conversation{}, err
	}
	_, effort, _ := providerModel(provider, model)
	session := backend.MessageRequest{ID: ulid.Make().String(), Created: time.UnixMilli(now), Owner: owner, Kind: "session", SessionID: "session", Session: &backend.Session{Owner: owner, Harness: provider, Model: model, Effort: effort, Workdir: prepared.Directory}}
	m.mu.Lock()
	item = m.items[id]
	item.Worktree, item.Branch = prepared.Directory, prepared.Branch
	item.Placement.Value.Directory = prepared.Directory
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

func (m *Manager) remotePreparationFailed(id string, cause error, binding *environments.Binding) {
	cleanupErr := error(nil)
	if binding != nil {
		m.mu.Lock()
		prepared := m.remoteStates[id].PrepareAttempted
		m.active[id].remote = binding
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(m.ctx), 15*time.Second)
		if prepared {
			ctx = effects.WithConnection(ctx, binding.Connection)
			cleanupErr = effects.Cleanup(ctx, "backend", id, id)
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

func (m *Manager) remoteContext(ctx context.Context, binding *environments.Binding, owner backend.Owner) context.Context {
	ctx = effects.WithConnection(ctx, binding.Connection)
	ctx = effects.WithOwner(ctx, owner)
	return effects.WithEnvironment(ctx, binding.Target.Route.Environment)
}

func (m *Manager) bindRemote(ctx context.Context, id string) (*environments.Binding, error) {
	m.mu.RLock()
	item, service := m.items[id], m.environments
	if item == nil || !remotePlacement(item) || service == nil {
		m.mu.RUnlock()
		return nil, errors.New("conversation: retained placement unavailable")
	}
	state := m.remoteStates[id]
	if state == nil {
		m.mu.RUnlock()
		return nil, errors.New("conversation: retained transport target unavailable")
	}
	target := state.Target
	m.mu.RUnlock()
	binding, err := service.Acquire(ctx, target, backend.Owner{Run: id, Scope: id, Task: "session"})
	if err != nil && binding.Connection.Execution == nil && binding.Connection.Messages == nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active[id] == nil {
		m.active[id] = &activeConversation{}
	}
	// Reacquire the saved target: an SSH tunnel can now have a new local port.
	// Replacing the binding never closes the service-owned shared connection.
	m.active[id].remote = &binding
	return m.active[id].remote, err
}

func (m *Manager) remoteSession(ctx context.Context, id string, binding *environments.Binding) error {
	m.mu.RLock()
	state := m.remoteStates[id]
	created := state != nil && state.SessionCreated
	var r backend.MessageRequest
	attempted := false
	if state != nil {
		r = state.Session
		attempted = state.SessionAttempted
	}
	m.mu.RUnlock()
	if created {
		return nil
	}
	if r.ID == "" {
		return &backend.DeliveryUncertain{RequestID: id, Err: errors.New("retained session admission is unavailable")}
	}
	ctx = m.remoteContext(ctx, binding, r.Owner)
	if attempted && binding.Connection.Messages == nil {
		return &backend.DeliveryUncertain{RequestID: r.ID, Err: errors.New("session admission already attempted; its outcome is unconfirmed")}
	}
	if !attempted {
		m.mu.Lock()
		state.SessionAttempted = true
		err := m.save(m.items[id])
		m.mu.Unlock()
		if err != nil {
			return err
		}
	}
	var err error
	if binding.Connection.Messages != nil {
		// The session envelope was persisted before the first publication.
		if attempted {
			_, err = effects.Await(ctx, r, "completed", nil)
		} else {
			_, err = effects.MessageCall(ctx, r)
		}
	} else {
		_, err = binding.Connection.Execution.Session(ctx, *r.Session)
	}
	if err != nil {
		return &backend.DeliveryUncertain{RequestID: r.ID, Err: fmt.Errorf("session admission unconfirmed; do not start replacement work: %w", err)}
	}
	m.mu.Lock()
	state.SessionCreated = true
	m.items[id].NativeSession = id + "\x00session"
	err = m.save(m.items[id])
	m.mu.Unlock()
	return err
}

func (m *Manager) remoteTurn(id string, r Request, active *activeConversation) {
	active.turnMu.Lock()
	defer active.turnMu.Unlock()
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()
	m.mu.Lock()
	item := m.items[id]
	if m.closing || item.Status == "unresolved" || item.CleanupPending {
		m.mu.Unlock()
		return
	}
	accepted := false
	for _, row := range item.Requests {
		if row.ID == r.ID {
			accepted = row.Status == "accepted"
			break
		}
	}
	if !accepted {
		m.mu.Unlock()
		return
	}
	active.request, active.cancel = r.ID, cancel
	if err := m.reduce(item, requestEvent{kind: "started", request: r, delivery: "turn"}); err != nil {
		m.mu.Unlock()
		m.finish(id, r, "failed", err)
		return
	}
	if err := m.userEvent(item, r, "turn"); err != nil {
		m.mu.Unlock()
		m.finish(id, r, "failed", err)
		return
	}
	owner := backend.Owner{Run: id, Scope: id, Task: r.ID}
	prompt := fmt.Sprintf("You are the agent in a Gimbal conversation on %s. Work in %s and reply to the person's message. Do not use local desktop paths or claim a workflow is centrally owned unless its admission is actually associated with this conversation. If integrated workflow admission is unavailable here, explain that limitation instead of launching unassociated work. Reply in ordinary prose.\n\nPerson: ", item.Placement.Value.Machine.Name, item.Worktree) + r.Message
	envelope := backend.MessageRequest{ID: ulid.Make().String(), Created: time.UnixMilli(r.Created), Owner: owner, SessionID: "session", Kind: "turn", Turn: &backend.Turn{Owner: owner, Session: "session", Prompt: prompt}}
	state := m.remoteStates[id]
	state.Requests[r.ID] = envelope
	journalErr := m.save(item)
	m.mu.Unlock()
	if journalErr != nil {
		m.finish(id, r, "failed", journalErr)
		return
	}
	// End-session sets CleanupPending before waiting for this short admission
	// section, so cleanup cannot miss a session/turn delivered concurrently.
	active.admitMu.Lock()
	m.mu.RLock()
	ending := item.CleanupPending || !item.Live || m.closing
	m.mu.RUnlock()
	if ending {
		active.admitMu.Unlock()
		return
	}
	binding, err := m.bindRemote(ctx, id)
	if err == nil {
		err = m.remoteSession(ctx, id, binding)
	}
	if err == nil {
		m.mu.RLock()
		ending = item.CleanupPending || !item.Live || m.closing
		m.mu.RUnlock()
		if ending {
			active.admitMu.Unlock()
			return
		}
	}
	if err == nil {
		// Session admission can fail before this prompt is ever submitted.
		// Persist the distinction before a possibly delivered native turn.
		m.mu.Lock()
		if state.TurnAttempted == nil {
			state.TurnAttempted = map[string]bool{}
		}
		state.TurnAttempted[r.ID] = true
		err = m.save(item)
		if err != nil {
			state.TurnAttempted[r.ID] = false
		}
		m.mu.Unlock()
		if err == nil {
			callCtx := m.remoteContext(ctx, binding, owner)
			if binding.Connection.Messages != nil {
				err = effects.Submit(callCtx, envelope)
			} else {
				err = binding.Connection.Execution.Turn(callCtx, *envelope.Turn)
			}
		}
	}
	active.admitMu.Unlock()
	if err != nil {
		m.remoteUncertain(id, r, err)
		m.background(func() { m.reconcileRemote(id) })
		return
	}
	m.mu.Lock()
	active.observing = true
	m.mu.Unlock()
	m.observeRemote(ctx, id, r, binding, envelope)
	m.mu.Lock()
	active.observing = false
	pending := m.items[id].ActiveRequest != ""
	m.mu.Unlock()
	if pending && ctx.Err() == nil {
		m.background(func() { m.reconcileRemote(id) })
	}
}

func (m *Manager) remoteUncertain(id string, r Request, err error) {
	m.mu.Lock()
	item := m.items[id]
	if item.ActiveRequest != r.ID || !item.Live {
		m.mu.Unlock()
		return
	}
	item.Contact = "unreachable"
	m.mu.Unlock()
	m.finish(id, r, "unresolved", &backend.DeliveryUncertain{RequestID: r.ID, Err: err})
}

func (m *Manager) observeRemote(ctx context.Context, id string, r Request, binding *environments.Binding, envelope backend.MessageRequest) {
	callCtx := effects.ObserveDetached(m.remoteContext(ctx, binding, envelope.Owner))
	m.mu.RLock()
	cursor := m.remoteStates[id].Cursors[r.ID]
	m.mu.RUnlock()
	callCtx = effects.ObserveTask(callCtx, cursor, func(next int, raw json.RawMessage) { m.remoteActivity(id, r, next, "", raw) })
	callCtx = effects.ObserveMessages(callCtx, func(event backend.MessageEvent) { m.remoteActivity(id, r, 0, event.ID, event.Activity) })
	var raw json.RawMessage
	var err error
	if binding.Connection.Messages != nil {
		var event backend.MessageEvent
		event, err = effects.Await(callCtx, envelope, "completed", nil)
		if event.Result != nil {
			raw, _ = json.Marshal(backend.TurnResult{Output: event.Result.Output, Usage: event.Result.Usage})
		}
	} else {
		raw, err = effects.Wait(callCtx, "backend", backend.Target{Run: id, ID: r.ID}, func(raw json.RawMessage) { m.remoteActivity(id, r, 0, "", raw) })
	}
	if err != nil {
		var uncertain *backend.DeliveryUncertain
		if errors.As(err, &uncertain) {
			m.remoteUncertain(id, r, err)
			if ctx.Err() == nil {
				m.background(func() { m.reconcileRemote(id) })
			}
			return
		}
		if errors.Is(err, context.Canceled) {
			m.settleRemote(id, r, "canceled", "", nil)
			return
		}
		m.settleRemote(id, r, "failed", "", err)
		return
	}
	var result backend.TurnResult
	if err = json.Unmarshal(raw, &result); err != nil {
		m.finish(id, r, "failed", err)
		return
	}
	var text string
	if err = json.Unmarshal(result.Output, &text); err != nil || strings.TrimSpace(text) == "" {
		m.finish(id, r, "failed", errors.New("provider returned an invalid or empty reply"))
		return
	}
	m.settleRemote(id, r, "completed", text, nil)
}

func (m *Manager) remoteActivity(id string, r Request, cursor int, eventID string, raw json.RawMessage) {
	var event gimbal.AgentEvent
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, state := m.items[id], m.remoteStates[id]
	if item == nil || state == nil {
		return
	}
	if eventID != "" {
		if state.Seen[eventID] {
			return
		}
		state.Seen[eventID] = true
	}
	if cursor > 0 {
		if cursor <= state.Cursors[r.ID] {
			return
		}
		state.Cursors[r.ID] = cursor
	}
	if event.Type == "observation-gap" || event.Type == "session.observation.unavailable" {
		item.Contact = "unreachable"
	} else {
		item.Contact = "connected"
	}
	var data map[string]any
	if json.Unmarshal(event.Data, &data) != nil {
		return
	}
	data["sessionID"] = id
	if mid, ok := data["assistantMessageID"].(string); ok {
		data["assistantMessageID"] = r.ID + "/" + mid
	}
	event.Data, _ = json.Marshal(data)
	_ = m.activity(item, event)
}

func (m *Manager) reconcileRemote(id string) {
	m.mu.Lock()
	active := m.active[id]
	if active == nil {
		active = &activeConversation{}
		m.active[id] = active
	}
	if active.observing || m.closing {
		m.mu.Unlock()
		return
	}
	active.observing = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		active.observing = false
		pending := m.items[id] != nil && m.items[id].ActiveRequest == "" && !m.closing
		m.mu.Unlock()
		if pending {
			m.background(func() { m.resumeQueuedRemote(id) })
		}
	}()
	for m.ctx.Err() == nil {
		m.mu.RLock()
		item := m.items[id]
		if item == nil || item.ActiveRequest == "" {
			m.mu.RUnlock()
			return
		}
		rid := item.ActiveRequest
		state := m.remoteStates[id]
		var envelope backend.MessageRequest
		if state != nil {
			envelope = state.Requests[rid]
		}
		var r Request
		for _, row := range item.Requests {
			if row.ID == rid {
				r = row.Request
				break
			}
		}
		closing := m.closing
		attempted := state != nil && state.TurnAttempted[rid]
		m.mu.RUnlock()
		if closing || envelope.ID == "" || !attempted {
			return
		}
		ctx, cancel := context.WithCancel(m.ctx)
		m.mu.Lock()
		if m.closing {
			m.mu.Unlock()
			cancel()
			return
		}
		active.request, active.cancel = rid, cancel
		m.mu.Unlock()
		binding, err := m.bindRemote(ctx, id)
		if err == nil {
			m.observeRemote(ctx, id, r, binding, envelope)
		} else {
			m.remoteUncertain(id, r, err)
		}
		cancel()
		m.mu.RLock()
		done := m.items[id].ActiveRequest == ""
		m.mu.RUnlock()
		if done {
			return
		}
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (m *Manager) remoteInterrupt(id string, r Request) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(m.ctx), 15*time.Second)
	defer cancel()
	owner := backend.Owner{Run: id, Scope: id, Task: r.ID}
	envelope := backend.MessageRequest{ID: ulid.Make().String(), Created: time.UnixMilli(r.Created), Owner: owner, Kind: "interrupt", Target: &backend.Target{Run: id, ID: r.Target}}
	m.mu.Lock()
	m.remoteStates[id].Requests[r.ID] = envelope
	journalErr := m.save(m.items[id])
	m.mu.Unlock()
	if journalErr != nil {
		m.finish(id, r, "failed", journalErr)
		return
	}
	binding, err := m.bindRemote(ctx, id)
	if err == nil {
		ctx = m.remoteContext(ctx, binding, owner)
		if binding.Connection.Messages != nil {
			_, err = effects.MessageCall(ctx, envelope)
		} else {
			err = binding.Connection.Execution.Interrupt(ctx, *envelope.Target)
		}
	}
	if err != nil {
		m.mu.Lock()
		m.items[id].Contact = "unreachable"
		_ = m.reduce(m.items[id], requestEvent{kind: "unresolved", request: r, err: err})
		m.items[id].Status = "unresolved"
		_ = m.save(m.items[id])
		m.mu.Unlock()
	}
	m.background(func() { m.reconcileRemote(id) })
}

func (m *Manager) deliverRemote(id string, r Request, active *activeConversation, target string) {
	m.mu.RLock()
	canSteer := m.items[id].Capabilities.Steer && target != "" && active.request == target
	m.mu.RUnlock()
	var binding *environments.Binding
	if canSteer {
		var err error
		binding, err = m.bindRemote(m.ctx, id)
		if err != nil {
			m.finish(id, r, "unresolved", &backend.DeliveryUncertain{RequestID: r.ID, Err: err})
			return
		}
	}
	if canSteer && binding != nil && binding.Connection.Execution != nil {
		active.steerMu.Lock()
		m.mu.RLock()
		still := active.request == target && !m.items[id].CleanupPending && m.items[id].Live
		m.mu.RUnlock()
		if still {
			landed, err := binding.Connection.Execution.Steer(m.ctx, backend.Target{Run: id, ID: "session", Message: r.Message})
			active.steerMu.Unlock()
			if err != nil {
				m.finish(id, r, "unresolved", &backend.DeliveryUncertain{RequestID: r.ID, Err: err})
				return
			}
			if landed {
				m.mu.Lock()
				_ = m.userEvent(m.items[id], r, "steer")
				_ = m.reduce(m.items[id], requestEvent{kind: "completed", request: r, delivery: "steer"})
				m.mu.Unlock()
				return
			}
		} else {
			active.steerMu.Unlock()
		}
	}
	m.remoteTurn(id, r, active)
}

// Cleanup ends the retained remote scope explicitly. Closing a tab/window never
// invokes this operation. Failed cleanup remains visible and retryable.
func (m *Manager) Cleanup(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	m.mu.Lock()
	item := m.items[id]
	if item == nil || !remotePlacement(item) {
		m.mu.Unlock()
		return errors.New("conversation: no remote environment to release")
	}
	var original Request
	var originalAttempted bool
	var envelope backend.MessageRequest
	if item.ActiveRequest != "" {
		for _, row := range item.Requests {
			if row.ID == item.ActiveRequest {
				original = row.Request
				break
			}
		}
		if state := m.remoteStates[id]; state != nil {
			envelope = state.Requests[item.ActiveRequest]
			originalAttempted = state.TurnAttempted[item.ActiveRequest]
		}
	}
	item.CleanupPending = true
	item.Revision++
	journalErr := m.save(item)
	m.mu.Unlock()
	if journalErr != nil {
		return journalErr
	}
	m.mu.RLock()
	active := m.active[id]
	m.mu.RUnlock()
	if active != nil {
		active.admitMu.Lock()
		defer active.admitMu.Unlock()
	}
	m.mu.RLock()
	if state := m.remoteStates[id]; state != nil && original.ID != "" {
		envelope = state.Requests[original.ID]
		originalAttempted = state.TurnAttempted[original.ID]
	}
	m.mu.RUnlock()
	binding, err := m.bindRemote(ctx, id)
	if binding != nil {
		callCtx := m.remoteContext(ctx, binding, backend.Owner{Run: id, Scope: id, Task: "cleanup"})
		// Uncertain cleanup delivery retains its envelope. An authoritative failed
		// control result permits the person's next explicit attempt to use a new ID.
		if binding.Connection.Messages != nil {
			m.mu.Lock()
			state := m.remoteStates[id]
			envelope, ok := state.Requests["cleanup"]
			if !ok || state.CleanupFailed {
				state.CleanupFailed = false
				envelope = backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now().UTC(), Owner: backend.Owner{Run: id, Scope: id, Task: "cleanup"}, Kind: "cleanup", ScopeTarget: &backend.Scope{Run: id, Scope: id}}
				state.Requests["cleanup"] = envelope
				journalErr = m.save(item)
			}
			m.mu.Unlock()
			if journalErr != nil {
				return journalErr
			}
			var completed backend.MessageEvent
			completed, err = effects.MessageCall(callCtx, envelope)
			if err != nil && completed.Kind == "completed" && completed.Result != nil {
				m.mu.Lock()
				state.CleanupFailed = true
				_ = m.save(item)
				m.mu.Unlock()
			}
		} else {
			var reply backend.Reply
			err = effects.Call(callCtx, "backend", "cleanup", backend.Scope{Run: id, Scope: id}, &reply)
			if err == nil && reply.Error != "" {
				err = errors.New(reply.Error)
			}
		}
		if err == nil && original.ID != "" {
			if originalAttempted {
				err = m.confirmRemoteOutcome(ctx, id, original, binding, envelope)
			} else {
				// Scope cleanup confirms session resources were released. This
				// prompt was never submitted, so no native turn fact can exist.
				m.settleRemote(id, original, "canceled", "", nil)
			}
		}
		if err == nil {
			err = m.environments.Release(ctx, *binding)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item = m.items[id]
	item.CleanupPending = err != nil
	item.Revision++
	if err != nil {
		item.Error = err.Error()
		item.Contact = "unreachable"
		if item.ActiveRequest != "" {
			item.Status = "unresolved"
		}
	} else {
		for _, row := range item.Requests {
			if row.Kind == "prompt" && row.Status == "accepted" {
				_ = m.reduce(item, requestEvent{kind: "canceled", request: row.Request})
			}
		}
		item.Live = false
		item.NativeSession = ""
		item.Error = ""
		item.Contact = "connected"

		if active := m.active[id]; active != nil && active.cancel != nil {
			active.cancel()
		}
	}
	_ = m.save(item)
	return err
}

// Resume only locally accepted rows that were never admitted to a native turn.
// A running/unresolved transport envelope is observed separately, never replayed.
func (m *Manager) resumeQueuedRemote(id string) {
	for {
		m.mu.Lock()
		item, active := m.items[id], m.active[id]
		if item == nil || !item.Live || m.closing || item.ActiveRequest != "" || (item.Status == "unresolved" || item.CleanupPending) {
			m.mu.Unlock()
			return
		}
		if active == nil {
			active = &activeConversation{}
			m.active[id] = active
		}
		var next Request
		for _, row := range item.Requests {
			if row.Kind == "prompt" && row.Status == "accepted" {
				next = row.Request
				break
			}
		}
		m.mu.Unlock()
		if next.ID == "" {
			return
		}
		m.remoteTurn(id, next, active)
	}
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
	item := &Conversation{ID: id, Title: title, Provider: provider, Model: model, Branch: prepared.Branch, Worktree: prepared.Directory, Live: true, Status: StatusIdle, Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{}, Environment: in.Environment, Contact: "connected", Capabilities: prepared.Target.Capabilities, Placement: polytype.Nullable[Placement]{Present: true, Value: Placement{Machine: PlacementMachine{ID: environments.Local, Name: "This Mac", Kind: "local"}, Environment: environments.DescribeEnvironment(target.Environment), Directory: prepared.Directory}}}
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

func (m *Manager) remoteSettled(id string, r Request) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, row := range m.items[id].Requests {
		if row.ID == r.ID {
			return row.Status == "completed" || row.Status == "canceled" || row.Status == "failed"
		}
	}
	return false
}

// Both the ordinary reader and explicit cleanup can observe the same terminal
// fact. Apply it once, preserving a successful completion that won the Stop race.
func (m *Manager) settleRemote(id string, r Request, kind, text string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	for _, row := range item.Requests {
		if row.ID == r.ID && (row.Status == "completed" || row.Status == "canceled" || row.Status == "failed") {
			return
		}
	}
	item.Contact = "connected"
	if kind == "completed" {
		m.assistantReply(item, r, text)
	}
	m.finishLocked(id, r, kind, err)
}

func (m *Manager) confirmRemoteOutcome(ctx context.Context, id string, r Request, binding *environments.Binding, envelope backend.MessageRequest) error {
	if m.remoteSettled(id, r) {
		return nil
	}
	if envelope.ID == "" {
		return &backend.DeliveryUncertain{RequestID: r.ID, Err: errors.New("original request envelope is missing; cleanup cannot establish its outcome")}
	}
	ctx = effects.ObserveDetached(m.remoteContext(ctx, binding, envelope.Owner))
	var raw json.RawMessage
	var err error
	if binding.Connection.Messages != nil {
		var event backend.MessageEvent
		event, err = effects.Await(ctx, envelope, "completed", nil)
		if event.Result != nil {
			raw, _ = json.Marshal(backend.TurnResult{Output: event.Result.Output, Usage: event.Result.Usage})
		}
	} else {
		raw, err = effects.Wait(ctx, "backend", backend.Target{Run: id, ID: r.ID}, nil)
	}
	if err != nil {
		if m.remoteSettled(id, r) {
			return nil
		}
		var uncertain *backend.DeliveryUncertain
		if errors.As(err, &uncertain) {
			return err
		}
		if errors.Is(err, context.Canceled) {
			m.settleRemote(id, r, "canceled", "", nil)
		} else {
			m.settleRemote(id, r, "failed", "", err)
		}
		return nil
	}
	var result backend.TurnResult
	if err = json.Unmarshal(raw, &result); err != nil {
		return &backend.DeliveryUncertain{RequestID: r.ID, Err: err}
	}
	var text string
	if err = json.Unmarshal(result.Output, &text); err != nil || strings.TrimSpace(text) == "" {
		m.settleRemote(id, r, "failed", "", errors.New("provider returned an invalid or empty reply"))
		return nil
	}
	m.settleRemote(id, r, "completed", text, nil)
	return nil
}
