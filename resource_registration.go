package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal/backend"
)

// OrchestratorDefinition installs environment provisioning independently of messaging.
// Construct receives the selected secrets bootstrap configuration for its workers.
type OrchestratorDefinition struct {
	Name                  string
	ValidateConfiguration func(json.RawMessage) error
	RequiresSharedSecrets bool
	Construct             func(context.Context, json.RawMessage, backend.Selection) (backend.OrchestratorBackend, error)
}

// SecretsDefinition installs a store independently of messaging and orchestration.
// SharedAcrossProcesses means independently constructed clients share scoped values.
type SecretsDefinition struct {
	Name                  string
	ValidateConfiguration func(json.RawMessage) error
	SharedAcrossProcesses bool
	Construct             func(context.Context, json.RawMessage) (backend.SecretsBackend, error)
}

func (r *Registry) RegisterOrchestrator(d OrchestratorDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(d.Name) == "" || d.ValidateConfiguration == nil || d.Construct == nil {
		return fmt.Errorf("malformed orchestrator definition %q", d.Name)
	}
	if _, ok := r.orchestrators[d.Name]; ok {
		return fmt.Errorf("duplicate orchestrator %q", d.Name)
	}
	if r.orchestrators == nil {
		r.orchestrators = map[string]OrchestratorDefinition{}
	}
	r.orchestrators[d.Name] = d
	return nil
}
func (r *Registry) RegisterSecrets(d SecretsDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(d.Name) == "" || d.ValidateConfiguration == nil || d.Construct == nil {
		return fmt.Errorf("malformed secrets definition %q", d.Name)
	}
	if _, ok := r.secrets[d.Name]; ok {
		return fmt.Errorf("duplicate secrets backend %q", d.Name)
	}
	if r.secrets == nil {
		r.secrets = map[string]SecretsDefinition{}
	}
	r.secrets[d.Name] = d
	return nil
}
func (r *Registry) Orchestrator(name string) (OrchestratorDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.orchestrators[name]
	return d, ok
}
func (r *Registry) Secrets(name string) (SecretsDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.secrets[name]
	return d, ok
}

func (r *Registry) constructDependencies(ctx context.Context, selections backend.BackendSelection) (backend.Dependencies, error) {
	var deps backend.Dependencies
	secrets, ok := r.Secrets(selections.Secrets.Name)
	if !ok {
		return deps, fmt.Errorf("unknown secrets backend %q", selections.Secrets.Name)
	}
	if err := secrets.ValidateConfiguration(selections.Secrets.Configuration); err != nil {
		return deps, fmt.Errorf("secrets backend %s configuration: %w", selections.Secrets.Name, err)
	}
	var orchestrator OrchestratorDefinition
	if selections.Orchestrator.Name != "" {
		orchestrator, ok = r.Orchestrator(selections.Orchestrator.Name)
		if !ok {
			return deps, fmt.Errorf("unknown orchestrator %q", selections.Orchestrator.Name)
		}
		if err := orchestrator.ValidateConfiguration(selections.Orchestrator.Configuration); err != nil {
			return deps, fmt.Errorf("orchestrator %s configuration: %w", selections.Orchestrator.Name, err)
		}
		if orchestrator.RequiresSharedSecrets && !secrets.SharedAcrossProcesses {
			return deps, fmt.Errorf("orchestrator %s requires a secrets backend shared across processes", orchestrator.Name)
		}
	}
	var err error
	deps.Secrets, err = secrets.Construct(ctx, selections.Secrets.Configuration)
	if err != nil {
		return backend.Dependencies{}, err
	}
	if selections.Orchestrator.Name != "" {
		deps.Orchestrator, err = orchestrator.Construct(ctx, selections.Orchestrator.Configuration, selections.Secrets)
		if err != nil {
			return backend.Dependencies{}, errors.Join(err, (backend.Connection{Orchestrator: deps.Orchestrator, Secrets: deps.Secrets}).Close())
		}
		var routed bool
		deps.Routing, routed = deps.Orchestrator.(backend.EnvironmentRouting)
		if !routed {
			return backend.Dependencies{}, errors.Join(fmt.Errorf("orchestrator %s must supply environment routing", orchestrator.Name), (backend.Connection{Orchestrator: deps.Orchestrator, Secrets: deps.Secrets}).Close())
		}
	}
	return deps, nil
}
