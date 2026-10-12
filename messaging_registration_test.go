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

type constructionMessages struct{}

func (constructionMessages) PublishRequest(context.Context, backend.MessageRequest) error { return nil }
func (constructionMessages) PublishEvent(context.Context, backend.MessageEvent) error     { return nil }
func (constructionMessages) ListenEvents(context.Context, backend.MessageSelection, time.Time, func(backend.MessageEvent) error) error {
	return nil
}

func TestMessagingConstructsDependencyBeforeFactory(t *testing.T) {
	registry := &Registry{}
	calls := 0
	var supplied backend.Dependencies
	definition := MessagingDefinition{Name: "fixture", ValidateConfiguration: func(json.RawMessage) error { return nil },
		ConstructBackends: func(_ context.Context, raw json.RawMessage) (backend.Dependencies, error) {
			calls++
			if string(raw) != `{"placement":"fixture"}` {
				return backend.Dependencies{}, errors.New("construction data lost")
			}
			return backend.Dependencies{Orchestrator: &constructionFixture{}}, nil
		},
		Connect: func(_ context.Context, _ json.RawMessage, dependency backend.Dependencies) (backend.Messaging, error) {
			supplied = dependency
			return constructionMessages{}, nil
		},
		Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error {
			return nil
		},
	}
	if err := registry.RegisterMessaging(definition); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		connection, err := registry.Connect(context.Background(), backend.Route{Name: "fixture", Backends: json.RawMessage(`{"placement":"fixture"}`)})
		if err != nil || supplied.Orchestrator == nil || connection.Orchestrator != supplied.Orchestrator {
			t.Fatalf("constructed dependency not supplied: %v", err)
		}
	}
	if calls != 2 {
		t.Fatal("each connection must construct its local dependency")
	}
	if _, err := registry.Connect(context.Background(), backend.Route{Name: "fixture"}); err != nil || supplied.Orchestrator != nil {
		t.Fatal("absent dependency must preserve ordinary deployments", err)
	}
}

func TestFailedMessagingConnectClosesDependencyWithoutReleasingUse(t *testing.T) {
	fixture := &constructionFixture{}
	registry := &Registry{}
	err := registry.RegisterMessaging(MessagingDefinition{Name: "failure", ValidateConfiguration: func(json.RawMessage) error { return nil }, ConstructBackends: func(context.Context, json.RawMessage) (backend.Dependencies, error) {
		return backend.Dependencies{Orchestrator: fixture}, nil
	}, Connect: func(context.Context, json.RawMessage, backend.Dependencies) (backend.Messaging, error) {
		return nil, errors.New("unavailable")
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error {
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Connect(context.Background(), backend.Route{Name: "failure", Backends: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("missing connection failure")
	}
	if fixture.closes != 1 || fixture.releases != 0 {
		t.Fatalf("connection and scope ownership confused: %+v", fixture)
	}
}
