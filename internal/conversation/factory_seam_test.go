package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal"
)

type denyingAdapter struct{ calls atomic.Int32 }

func (a *denyingAdapter) deny() error { a.calls.Add(1); return errors.New("harness denied") }
func (a *denyingAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "", a.deny()
}
func (a *denyingAdapter) RunTurn(context.Context, string, string, json.RawMessage, func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	return gimbal.TurnResult{}, a.deny()
}
func (a *denyingAdapter) Steer(context.Context, string, string) (bool, error) { return false, a.deny() }
func (a *denyingAdapter) Fork(context.Context, string) (string, error)        { return "", a.deny() }
func (a *denyingAdapter) Close(context.Context, string) error                 { return a.deny() }

// Seam H audit: the Manager's AdapterFactory only validates that a provider is
// known. Conversation turns run through the conversation workflow's own role
// binding, so a denying adapter returned by the factory is never asked to work.
func TestConversationFactoryAdapterIsValidationOnly(t *testing.T) {
	repo := newRepository(t)
	adapter := &denyingAdapter{}
	var resolved atomic.Int32
	factory := func(string) (gimbal.HarnessAdapter, error) { resolved.Add(1); return adapter, nil }
	m, err := newMetadataManager(t.Context(), filepath.Join(repo, ".gimbal"), factory, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude"}, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sendAndWait(t, m, item.ID, "Continue"); err != nil {
		t.Fatal(err)
	}
	if resolved.Load() == 0 {
		t.Fatal("factory was never consulted")
	}
	if got := adapter.calls.Load(); got != 0 {
		t.Fatalf("factory adapter received %d session calls", got)
	}
}

func TestConversationFactoryRefusalRejectsStart(t *testing.T) {
	repo := newRepository(t)
	factory := func(string) (gimbal.HarnessAdapter, error) { return nil, errors.New("unknown harness") }
	m, err := newMetadataManager(t.Context(), filepath.Join(repo, ".gimbal"), factory, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.Start(t.Context(), NewConversation{Provider: "claude"}, "Hello"); err == nil {
		t.Fatal("refused provider started a conversation")
	}
}
