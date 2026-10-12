package gimbal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal/backend"
)

// MessagingDefinition is installed by a native plugin's existing Register
// entrypoint. Worker serves the explicitly configured local deployment; it does
// not provision hosts or orchestrate workflows. Connect must not bind a server.
type MessagingDefinition struct {
	Name                  string
	ValidateConfiguration func(json.RawMessage) error
	Connect               func(context.Context, json.RawMessage) (backend.Messaging, error)
	Worker                func(context.Context, json.RawMessage, backend.MessageHandler) error
}

func (r *Registry) RegisterMessaging(d MessagingDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(d.Name) == "" || d.ValidateConfiguration == nil || d.Connect == nil || d.Worker == nil {
		return fmt.Errorf("malformed messaging definition %q", d.Name)
	}
	if _, ok := r.backends[d.Name]; ok {
		return fmt.Errorf("backend %q already registered", d.Name)
	}
	if r.messaging == nil {
		r.messaging = map[string]MessagingDefinition{}
	}
	if _, ok := r.messaging[d.Name]; ok {
		return fmt.Errorf("messaging backend %q already registered", d.Name)
	}
	r.messaging[d.Name] = d
	return nil
}
func (r *Registry) Messaging(name string) (MessagingDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.messaging[name]
	return d, ok
}
