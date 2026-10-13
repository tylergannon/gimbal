package backend

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ScopedValues contains only private backend data, never workflow events.
type ScopedValues map[Scope]map[string]map[string]string

func WithinScope(parent, child string) bool {
	return parent == "" || child == parent || strings.HasPrefix(child, parent+"/")
}

func (v ScopedValues) Publish(scope Scope, write StoreWrite) error {
	seen := map[string]bool{}
	for _, value := range write.Values {
		if seen[value.Name] {
			return fmt.Errorf("duplicate name %q in write", value.Name)
		}
		seen[value.Name] = true
		for _, existing := range v[scope] {
			if _, ok := existing[value.Name]; ok {
				return fmt.Errorf("name %q already defined in this scope", value.Name)
			}
		}
	}
	if v[scope] == nil {
		v[scope] = map[string]map[string]string{}
	}
	if v[scope][write.Store] == nil {
		v[scope][write.Store] = map[string]string{}
	}
	for _, value := range write.Values {
		v[scope][write.Store][value.Name] = value.Value
	}
	return nil
}
func (v ScopedValues) Acquire(scope Scope, requirements Requirements) (map[string]string, error) {
	type definition struct {
		depth        int
		store, value string
	}
	visible := map[string]definition{}
	for ancestor, stores := range v {
		if ancestor.Run != scope.Run || !WithinScope(ancestor.Scope, scope.Scope) {
			continue
		}
		for store, values := range stores {
			for name, value := range values {
				previous, ok := visible[name]
				if !ok || len(ancestor.Scope) > previous.depth {
					visible[name] = definition{len(ancestor.Scope), store, value}
				}
			}
		}
	}
	allowed := map[string]bool{}
	for _, requirement := range requirements {
		allowed[requirement.Store] = true
		for _, name := range requirement.Names {
			value, ok := visible[name]
			if !ok || value.store != requirement.Store {
				return nil, fmt.Errorf("required name %q unavailable in store %q", name, requirement.Store)
			}
		}
	}
	result := map[string]string{}
	for name, value := range visible {
		if allowed[value.store] {
			result[name] = value.value
		}
	}
	return result, nil
}
func (v ScopedValues) Release(scope Scope) {
	for candidate := range v {
		if candidate.Run == scope.Run && WithinScope(scope.Scope, candidate.Scope) {
			delete(v, candidate)
		}
	}
}

type memorySecrets struct {
	mu     sync.Mutex
	values ScopedValues
}

func NewMemorySecrets() SecretsBackend { return &memorySecrets{values: ScopedValues{}} }
func (m *memorySecrets) Publish(ctx context.Context, s Scope, write StoreWrite) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values.Publish(s, write)
}
func (m *memorySecrets) Acquire(ctx context.Context, s Scope, r Requirements) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values.Acquire(s, r)
}
func (m *memorySecrets) Release(ctx context.Context, s Scope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values.Release(s)
	return nil
}
