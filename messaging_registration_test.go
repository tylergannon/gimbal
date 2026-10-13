package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

type constructionFixture struct{ closes, releases int }

func (*constructionFixture) AcquireEnvironment(context.Context, backend.Owner, string) (backend.Environment, error) {
	return backend.Environment{}, nil
}
func (f *constructionFixture) Release(context.Context, backend.Scope) error { f.releases++; return nil }
func (f *constructionFixture) Close() error                                 { f.closes++; return nil }

func (*constructionFixture) Destinations(context.Context, backend.MessageRequest) ([]backend.EnvironmentDestination, error) {
	return nil, nil
}
func (*constructionFixture) ActiveEnvironments(context.Context) ([]backend.EnvironmentDestination, error) {
	return nil, nil
}

type constructionMessages struct{}

func (constructionMessages) PublishRequest(context.Context, backend.MessageRequest) error { return nil }
func (constructionMessages) PublishEvent(context.Context, backend.MessageEvent) error     { return nil }
func (constructionMessages) ListenEvents(context.Context, backend.MessageSelection, time.Time, func(backend.MessageEvent) error) error {
	return nil
}
func fixtureRegistry(t *testing.T, fixture *constructionFixture, connect func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error)) *Registry {
	t.Helper()
	r := &Registry{}
	if err := r.RegisterSecrets(SecretsDefinition{Name: "memory", ValidateConfiguration: func(json.RawMessage) error { return nil }, Construct: func(context.Context, json.RawMessage) (backend.SecretsBackend, error) {
		return backend.NewMemorySecrets(), nil
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterOrchestrator(OrchestratorDefinition{Name: "fixture", ValidateConfiguration: func(raw json.RawMessage) error {
		if string(raw) != `{"placement":"fixture"}` {
			return errors.New("construction data lost")
		}
		return nil
	}, Construct: func(context.Context, json.RawMessage, backend.Selection) (backend.OrchestratorBackend, error) {
		return fixture, nil
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterMessaging(MessagingDefinition{Name: "fixture", ValidateConfiguration: func(json.RawMessage) error { return nil }, Connect: connect, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	return r
}
func fixtureRoute() backend.Route {
	return backend.Route{Name: "fixture", Backends: &backend.BackendSelection{Orchestrator: backend.Selection{Name: "fixture", Configuration: json.RawMessage(`{"placement":"fixture"}`)}, Secrets: backend.Selection{Name: "memory"}}}
}
func TestMessagingReceivesIndependentlyConstructedDependencies(t *testing.T) {
	fixture := &constructionFixture{}
	var supplied backend.Dependencies
	r := fixtureRegistry(t, fixture, func(_ context.Context, _ json.RawMessage, deps backend.Dependencies) (backend.Messaging, error) {
		supplied = deps
		return constructionMessages{}, nil
	})
	connection, err := r.Connect(t.Context(), fixtureRoute())
	if err != nil || connection.Orchestrator != fixture || supplied.Orchestrator != fixture || supplied.Secrets == nil {
		t.Fatalf("dependencies lost: %+v %v", supplied, err)
	}
	if _, err := r.Connect(t.Context(), backend.Route{Name: "fixture"}); err != nil || supplied.Orchestrator != nil {
		t.Fatal("ordinary deployment", err)
	}
}
func TestFailedMessagingConnectClosesDependencyWithoutReleasingUse(t *testing.T) {
	fixture := &constructionFixture{}
	r := fixtureRegistry(t, fixture, func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		return nil, errors.New("unavailable")
	})
	if _, err := r.Connect(t.Context(), fixtureRoute()); err == nil {
		t.Fatal("missing failure")
	}
	if fixture.closes != 1 || fixture.releases != 0 {
		t.Fatalf("ownership confused: %+v", fixture)
	}
}
func TestBackendSelectionsValidateBeforeConstruction(t *testing.T) {
	calls := 0
	r := fixtureRegistry(t, &constructionFixture{}, func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		calls++
		return constructionMessages{}, nil
	})
	for _, mutate := range []func(*backend.BackendSelection){func(s *backend.BackendSelection) { s.Secrets.Name = "absent" }, func(s *backend.BackendSelection) { s.Orchestrator.Name = "absent" }, func(s *backend.BackendSelection) { s.Orchestrator.Configuration = json.RawMessage(`{}`) }} {
		route := fixtureRoute()
		mutate(route.Backends)
		if _, err := r.Connect(t.Context(), route); err == nil {
			t.Fatal("invalid selection admitted")
		}
	}
	if calls != 0 {
		t.Fatal("messaging started before validation")
	}
}
