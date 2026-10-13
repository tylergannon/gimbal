package gimbal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal/auth"
)

// ProviderDefinition supplies verified external identities. Connect receives
// the application-owned exact callback URL and must not register HTTP routes.
// Resource-owning implementations may also implement io.Closer.
type ProviderDefinition struct {
	Name                  string
	ValidateConfiguration func(json.RawMessage) error
	Connect               func(context.Context, json.RawMessage, string) (auth.Provider, error)
}

// UsersDefinition supplies a durable local user and session store.
// Resource-owning implementations may also implement io.Closer.
type UsersDefinition struct {
	Name                  string
	ValidateConfiguration func(json.RawMessage) error
	Connect               func(context.Context, json.RawMessage) (auth.Users, error)
}

// RegisterProvider admits one provider definition before serving requests.
func (r *Registry) RegisterProvider(d ProviderDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(d.Name) == "" || d.ValidateConfiguration == nil || d.Connect == nil {
		return fmt.Errorf("malformed auth provider definition %q", d.Name)
	}
	if _, ok := r.providers[d.Name]; ok {
		return fmt.Errorf("duplicate auth provider %q", d.Name)
	}
	if r.providers == nil {
		r.providers = make(map[string]ProviderDefinition)
	}
	r.providers[d.Name] = d
	return nil
}

// Provider returns a registered external identity provider definition.
func (r *Registry) Provider(name string) (ProviderDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.providers[name]
	return d, ok
}

// RegisterUsers admits one user store definition before serving requests.
func (r *Registry) RegisterUsers(d UsersDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(d.Name) == "" || d.ValidateConfiguration == nil || d.Connect == nil {
		return fmt.Errorf("malformed auth users definition %q", d.Name)
	}
	if _, ok := r.users[d.Name]; ok {
		return fmt.Errorf("duplicate auth users %q", d.Name)
	}
	if r.users == nil {
		r.users = make(map[string]UsersDefinition)
	}
	r.users[d.Name] = d
	return nil
}

// Users returns a registered durable local user store definition.
func (r *Registry) Users(name string) (UsersDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.users[name]
	return d, ok
}
