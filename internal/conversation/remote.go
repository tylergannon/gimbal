package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/environments"
	"github.com/tylergannon/gimbal/internal/sessionstate"
	"github.com/tylergannon/polytype"
)

func remotePlacement(item *Conversation) bool {
	return item.Placement.Present && item.Placement.Value.Machine.ID != "" && item.Placement.Value.Machine.ID != environments.Local
}

func localCapabilities() environments.Capabilities {
	return environments.Capabilities{Steer: true, Fork: true, Worktree: true, Browse: true, Clone: true}
}

func (m *Manager) createNamespace(in NewConversation, provider, model string, target environments.Target, route backend.Route) (Conversation, error) {
	if strings.TrimSpace(target.Directory) == "" {
		return Conversation{}, errors.New("conversation: namespace directory is required")
	}
	id, err := randomID()
	if err != nil {
		return Conversation{}, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = providerLabel(provider) + " conversation"
	}
	now := time.Now().UnixMilli()
	branch := strings.TrimSpace(in.Branch)
	item := &Conversation{ID: id, Title: title, Provider: provider, Model: model, Effort: in.Effort, SharedCheckout: in.SharedCheckout, Base: in.Base, Branch: branch, Worktree: target.Directory, Environment: in.Environment, Live: true, Status: StatusIdle, Created: now, Updated: now, Messages: []Message{}, Runs: []Run{}, Requests: []RequestState{}, Contact: "unknown", Capabilities: environments.Capabilities{Steer: true}, Placement: polytype.Nullable[Placement]{Present: true, Value: Placement{Machine: PlacementMachine{Name: "Pending worker", Kind: "pending"}, Environment: environments.DescribeEnvironment(target.Environment), Directory: target.Directory}}}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return Conversation{}, errors.New("conversation: runtime is shutting down")
	}
	m.namespaceRoutes[id] = route
	if err := m.save(item); err != nil {
		delete(m.namespaceRoutes, id)
		return Conversation{}, err
	}
	m.items[id] = item
	m.projections[id] = sessionstate.New(sessionstate.NewProjectionState())
	return clone(item), nil
}

// SetEnvironments attaches the instance-owned placement service without making
// network calls under the host's project-admission lock.
func (m *Manager) SetEnvironments(service *environments.Service, projectID string) {
	m.mu.Lock()
	m.environments, m.projectID = service, projectID
	m.mu.Unlock()
}

// Cleanup requests ordinary workflow completion through durable ingress.
func (m *Manager) Cleanup(ctx context.Context, id string) error {
	_, err := m.Submit(id, Request{ID: ulid.Make().String(), Kind: "finish"})
	return err
}
