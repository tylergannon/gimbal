package backend

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
)

// ScopedValues contains only private backend data, never workflow events.
type ScopedValues map[Scope]map[string]map[string]string

func WithinScope(parent, child string) bool {
	return parent == "" || child == parent || strings.HasPrefix(child, parent+"/")
}

func (v ScopedValues) Publish(scope Scope, store string, values map[string]string) error {
	if v[scope] == nil {
		v[scope] = map[string]map[string]string{}
	}
	for name := range values {
		for _, existing := range v[scope] {
			if _, ok := existing[name]; ok {
				return fmt.Errorf("name %q already defined in this scope", name)
			}
		}
	}
	if v[scope][store] == nil {
		v[scope][store] = map[string]string{}
	}
	maps.Copy(v[scope][store], values)
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
func (m *memorySecrets) Publish(ctx context.Context, s Scope, store string, values map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values.Publish(s, store, values)
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
