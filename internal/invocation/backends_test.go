package invocation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

type selectionMessages struct{ backend.Messaging }

func TestIndependentSecretsSelection(t *testing.T) {
	registry := &gimbal.Registry{}
	if err := registerResourceBackends(registry); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := registry.RegisterMessaging(gimbal.MessagingDefinition{ExecutionIdentity: true, Name: "test", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(_ context.Context, _ json.RawMessage, deps backend.Dependencies) (backend.Messaging, error) {
		calls++
		if deps.Secrets == nil {
			t.Fatal("selected secrets missing")
		}
		return selectionMessages{}, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	fileConfig, _ := json.Marshal(map[string]string{"Directory": t.TempDir()})
	for _, selection := range []backend.Selection{{Name: "memory", Configuration: json.RawMessage(`{}`)}, {Name: "file", Configuration: fileConfig}} {
		t.Run(selection.Name, func(t *testing.T) {
			connection, err := registry.Connect(t.Context(), backend.Route{Name: "test", Backends: &backend.BackendSelection{Secrets: selection}})
			if err != nil {
				t.Fatal(err)
			}
			scope := backend.Scope{Run: "run", Scope: "root"}
			if err := connection.Secrets.Publish(t.Context(), scope, backend.StoreWrite{Store: "settings", Values: []backend.NamedValue{{Name: "REQUIRED", Value: "yes"}, {Name: "EXTRA", Value: "whole-store"}}}); err != nil {
				t.Fatal(err)
			}
			values, err := connection.Secrets.Acquire(t.Context(), scope, backend.Requirements{{Store: "settings", Names: []string{"REQUIRED"}}})
			if err != nil || values["EXTRA"] != "whole-store" {
				t.Fatalf("store values=%v error=%v", values, err)
			}
			if err := connection.Secrets.Release(t.Context(), scope); err != nil {
				t.Fatal(err)
			}
		})
	}
	if calls != 2 {
		t.Fatalf("same messaging constructor called %d times", calls)
	}
}
func TestLocalOrchestratorRejectsProcessPrivateSecrets(t *testing.T) {
	registry := &gimbal.Registry{}
	if err := registerResourceBackends(registry); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := registry.RegisterMessaging(gimbal.MessagingDefinition{ExecutionIdentity: true, Name: "test", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		calls++
		return selectionMessages{}, nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"Directory": t.TempDir(), "Environments": map[string]any{}})
	_, err := registry.Connect(t.Context(), backend.Route{Name: "test", Backends: &backend.BackendSelection{Orchestrator: backend.Selection{Name: "local", Configuration: raw}, Secrets: backend.Selection{Name: "memory"}}})
	if err == nil || calls != 0 {
		t.Fatalf("private subprocess store admitted: calls=%d error=%v", calls, err)
	}
}

// Keep the test transport a minimal retained-event implementation.
func (selectionMessages) ListenEvents(context.Context, backend.MessageSelection, time.Time, func(backend.MessageEvent) error) error {
	return nil
}
