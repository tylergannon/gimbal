package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
)

// This capability has no local orchestrator, daemon process, or receipt files.
type alternateRouting struct {
	mu           sync.Mutex
	destinations []backend.EnvironmentDestination
	active       []backend.EnvironmentDestination
}

func (r *alternateRouting) Destinations(context.Context, backend.MessageRequest) ([]backend.EnvironmentDestination, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]backend.EnvironmentDestination(nil), r.destinations...), nil
}
func (r *alternateRouting) ActiveEnvironments(context.Context) ([]backend.EnvironmentDestination, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]backend.EnvironmentDestination(nil), r.active...), nil
}

type routingMessages struct {
	*messagefiles.Store
	requests chan backend.MessageRequest
}

func (m *routingMessages) PublishRequest(ctx context.Context, request backend.MessageRequest) error {
	select {
	case m.requests <- request:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func newRoutingMessages(t *testing.T) *routingMessages {
	t.Helper()
	store, err := messagefiles.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &routingMessages{Store: store, requests: make(chan backend.MessageRequest, 5)}
}
func newRoutingFixture(t *testing.T, routing backend.EnvironmentRouting) (backend.Messaging, *routingMessages, map[string]*routingMessages) {
	t.Helper()
	base := newRoutingMessages(t)
	buses := map[string]*routingMessages{"one": newRoutingMessages(t), "two": newRoutingMessages(t)}
	registry := &Registry{}
	err := registry.RegisterMessaging(MessagingDefinition{ExecutionIdentity: true, Name: "alternate", ValidateConfiguration: func(raw json.RawMessage) error {
		var name string
		return json.Unmarshal(raw, &name)
	}, Connect: func(_ context.Context, raw json.RawMessage, deps backend.Dependencies) (backend.Messaging, error) {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return nil, err
		}
		if deps.Routing != routing {
			return nil, errors.New("routing dependency lost")
		}
		return buses[name], nil
	}, Worker: func(context.Context, json.RawMessage, backend.Dependencies, backend.MessageHandler) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	bus, err := registry.connectEnvironments(t.Context(), base, routing, backend.Dependencies{Routing: routing})
	if err != nil {
		t.Fatal(err)
	}
	return bus, base, buses
}
func routingDestination(id string) backend.EnvironmentDestination {
	raw, _ := json.Marshal(id)
	return backend.EnvironmentDestination{ID: id, Route: backend.Route{Name: "alternate", Configuration: raw}}
}
func awaitRoutingRequest(t *testing.T, ctx context.Context, bus *routingMessages) backend.MessageRequest {
	t.Helper()
	select {
	case request := <-bus.requests:
		return request
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		return backend.MessageRequest{}
	}
}
func publishRoutingCompletion(t *testing.T, ctx context.Context, bus *routingMessages, request backend.MessageRequest, result *backend.MessageResult) {
	t.Helper()
	err := bus.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: request.ID, Created: time.Now().UTC(), Owner: request.Owner, Kind: "completed", Result: result})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentRouterUsesAlternateCapability(t *testing.T) {
	routing := &alternateRouting{destinations: []backend.EnvironmentDestination{routingDestination("two")}}
	router, _, buses := newRoutingFixture(t, routing)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	request := backend.MessageRequest{ID: "process", Kind: "process", Run: "run", Task: "task"}
	if err := router.PublishRequest(ctx, request); err != nil {
		t.Fatal(err)
	}
	if got := awaitRoutingRequest(t, ctx, buses["two"]); got.ID != request.ID {
		t.Fatal(got)
	}
	if len(buses["one"].requests) != 0 {
		t.Fatal("request reached unassigned environment")
	}
	// Discovery starts listening to a destination that became active later.
	publishRoutingCompletion(t, ctx, buses["two"], request, &backend.MessageResult{})
	observed := make(chan error, 1)
	done := errors.New("observed")
	go func() {
		observed <- router.ListenEvents(ctx, backend.MessageSelection{RequestID: request.ID}, time.Time{}, func(event backend.MessageEvent) error {
			if event.RequestID == request.ID {
				return done
			}
			return nil
		})
	}()
	routing.mu.Lock()
	routing.active = []backend.EnvironmentDestination{routingDestination("two")}
	routing.mu.Unlock()
	if err := <-observed; !errors.Is(err, done) {
		t.Fatalf("discovered destination event: %v", err)
	}
}

func TestEnvironmentRouterAggregatesAlternateCleanup(t *testing.T) {
	for _, unresolved := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "unresolved"}[unresolved], func(t *testing.T) {
			routing := &alternateRouting{destinations: []backend.EnvironmentDestination{routingDestination("one"), routingDestination("two")}}
			router, base, buses := newRoutingFixture(t, routing)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			request := backend.MessageRequest{ID: "cleanup", Kind: "cleanup", Run: "run", ScopeTarget: &backend.Scope{Run: "run"}}
			if err := router.PublishRequest(ctx, request); err != nil {
				t.Fatal(err)
			}
			one := awaitRoutingRequest(t, ctx, buses["one"])
			two := awaitRoutingRequest(t, ctx, buses["two"])
			if one.ID == two.ID || one.ID == request.ID || two.ID == request.ID {
				t.Fatal("cleanup request identities collided")
			}
			publishRoutingCompletion(t, ctx, buses["one"], one, &backend.MessageResult{})
			publishRoutingCompletion(t, ctx, buses["two"], two, &backend.MessageResult{Unresolved: unresolved})
			done := errors.New("complete")
			var result *backend.MessageResult
			err := base.ListenEvents(ctx, backend.MessageSelection{RequestID: request.ID}, time.Time{}, func(event backend.MessageEvent) error {
				if event.Kind == "completed" {
					result = event.Result
					return done
				}
				return nil
			})
			if !errors.Is(err, done) || result == nil || result.Unresolved != unresolved {
				t.Fatalf("aggregate result %+v: %v", result, err)
			}
		})
	}
}

func TestParentCleanupAfterLastEnvironmentRelease(t *testing.T) {
	router, base, _ := newRoutingFixture(t, &alternateRouting{})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	request := backend.MessageRequest{ID: "cleanup", Kind: "cleanup", Run: "run", ScopeTarget: &backend.Scope{Run: "run"}}
	if err := router.PublishRequest(ctx, request); err != nil {
		t.Fatal(err)
	}
	done := errors.New("complete")
	err := base.ListenEvents(ctx, backend.MessageSelection{RequestID: request.ID}, time.Time{}, func(event backend.MessageEvent) error {
		if event.Kind != "completed" || event.Result == nil || event.Result.Unresolved {
			t.Fatalf("parent cleanup did not confirm no remaining ownership: %+v", event)
		}
		return done
	})
	if !errors.Is(err, done) {
		t.Fatal(err)
	}
}
