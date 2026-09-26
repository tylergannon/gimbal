package config

import (
	"context"
	"maps"
	"sync"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// CredentialType tags a stored credential.
type CredentialType string

const (
	// CredentialAPIKey is an API-key credential.
	CredentialAPIKey CredentialType = "api_key"
	// CredentialOAuth is an OAuth credential.
	CredentialOAuth CredentialType = "oauth"
)

// Credential is one provider's stored credential (pi Credential).
type Credential struct {
	Type    CredentialType    `json:"type"`
	Key     string            `json:"key,omitempty"`
	Env     model.ProviderEnv `json:"env,omitempty"`
	Access  string            `json:"access,omitempty"`
	Refresh string            `json:"refresh,omitempty"`
	Expires int64             `json:"expires,omitempty"`
}

// CredentialInfo is non-secret credential metadata for enumeration.
type CredentialInfo struct {
	ProviderID string         `json:"providerId"`
	Type       CredentialType `json:"type"`
}

// CredentialStore is the app-owned credential storage contract, keyed by
// provider id. Context carries cancellation in place of pi's AbortSignal.
type CredentialStore interface {
	Read(ctx context.Context, providerID string) (*Credential, error)
	List(ctx context.Context) ([]CredentialInfo, error)
	Modify(ctx context.Context, providerID string, fn func(current *Credential) (*Credential, error)) (*Credential, error)
	Delete(ctx context.Context, providerID string) error
}

// RuntimeCredentials is an asynchronous overlay for non-persistent runtime API
// keys. Runtime overrides mask stored credentials in Read and List without
// being persisted; Delete removes both.
type RuntimeCredentials struct {
	store CredentialStore

	mu        sync.RWMutex
	overrides map[string]string
}

// NewRuntimeCredentials wraps a credential store with a runtime overlay.
func NewRuntimeCredentials(store CredentialStore) *RuntimeCredentials {
	return &RuntimeCredentials{store: store, overrides: map[string]string{}}
}

// SetRuntimeAPIKey records a non-persistent API key for a provider.
func (r *RuntimeCredentials) SetRuntimeAPIKey(providerID, apiKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides[providerID] = apiKey
}

// RemoveRuntimeAPIKey drops a runtime override.
func (r *RuntimeCredentials) RemoveRuntimeAPIKey(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.overrides, providerID)
}

// HasRuntimeAPIKey reports whether a provider has a runtime override.
func (r *RuntimeCredentials) HasRuntimeAPIKey(providerID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.overrides[providerID]
	return ok
}

// Read returns the runtime override when present, otherwise the stored
// credential.
func (r *RuntimeCredentials) Read(ctx context.Context, providerID string) (*Credential, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	override, ok := r.overrides[providerID]
	r.mu.RUnlock()
	if ok {
		return &Credential{Type: CredentialAPIKey, Key: override}, nil
	}
	return r.store.Read(ctx, providerID)
}

// List merges the stored credential metadata with the runtime overrides,
// exposing overrides as api_key entries without their keys.
func (r *RuntimeCredentials) List(ctx context.Context) ([]CredentialInfo, error) {
	stored, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries := make(map[string]CredentialInfo, len(stored))
	order := make([]string, 0, len(stored))
	for _, entry := range stored {
		if _, seen := entries[entry.ProviderID]; !seen {
			order = append(order, entry.ProviderID)
		}
		entries[entry.ProviderID] = entry
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	for providerID := range r.overrides {
		if _, seen := entries[providerID]; !seen {
			order = append(order, providerID)
		}
		entries[providerID] = CredentialInfo{ProviderID: providerID, Type: CredentialAPIKey}
	}

	merged := make([]CredentialInfo, 0, len(order))
	for _, providerID := range order {
		merged = append(merged, entries[providerID])
	}
	return merged, nil
}

// Modify forwards a serialized write to the persistent store.
func (r *RuntimeCredentials) Modify(ctx context.Context, providerID string, fn func(current *Credential) (*Credential, error)) (*Credential, error) {
	return r.store.Modify(ctx, providerID, fn)
}

// Delete removes the persisted credential and then the runtime override. A
// cancelled or failed delete leaves the override in place.
func (r *RuntimeCredentials) Delete(ctx context.Context, providerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.store.Delete(ctx, providerID); err != nil {
		return err
	}
	r.RemoveRuntimeAPIKey(providerID)
	return nil
}

// InMemoryCredentialStore is a simple CredentialStore for tests and embedded
// use. It serializes Modify per provider.
type InMemoryCredentialStore struct {
	mu      sync.Mutex
	entries map[string]Credential
	order   []string
}

// NewInMemoryCredentialStore returns a store seeded with the given credentials.
func NewInMemoryCredentialStore(seed map[string]Credential) *InMemoryCredentialStore {
	store := &InMemoryCredentialStore{entries: map[string]Credential{}}
	for providerID, credential := range seed {
		credential.Env = cloneProviderEnv(credential.Env)
		store.entries[providerID] = credential
		store.order = append(store.order, providerID)
	}
	return store
}

// Read returns the stored credential for a provider, or nil when absent.
func (s *InMemoryCredentialStore) Read(ctx context.Context, providerID string) (*Credential, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.entries[providerID]
	if !ok {
		return nil, nil
	}
	out := credential
	out.Env = cloneProviderEnv(credential.Env)
	return &out, nil
}

// List returns credential metadata in insertion order.
func (s *InMemoryCredentialStore) List(ctx context.Context) ([]CredentialInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info := make([]CredentialInfo, 0, len(s.entries))
	for _, providerID := range s.order {
		credential, ok := s.entries[providerID]
		if !ok {
			continue
		}
		info = append(info, CredentialInfo{ProviderID: providerID, Type: credential.Type})
	}
	return info, nil
}

// Modify runs fn against the current credential and stores its result.
func (s *InMemoryCredentialStore) Modify(ctx context.Context, providerID string, fn func(current *Credential) (*Credential, error)) (*Credential, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.entries[providerID]
	var currentPtr *Credential
	if ok {
		clone := current
		clone.Env = cloneProviderEnv(current.Env)
		currentPtr = &clone
	}
	next, err := fn(currentPtr)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return currentPtr, nil
	}
	stored := *next
	stored.Env = cloneProviderEnv(next.Env)
	if _, exists := s.entries[providerID]; !exists {
		s.order = append(s.order, providerID)
	}
	s.entries[providerID] = stored
	return &stored, nil
}

// Delete removes a provider's credential.
func (s *InMemoryCredentialStore) Delete(ctx context.Context, providerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, providerID)
	for i, id := range s.order {
		if id == providerID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

func cloneProviderEnv(env model.ProviderEnv) model.ProviderEnv {
	if env == nil {
		return nil
	}
	out := make(model.ProviderEnv, len(env))
	maps.Copy(out, env)
	return out
}
