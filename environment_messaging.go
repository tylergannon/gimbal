package gimbal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

type environmentMessaging struct {
	registry     *Registry
	base         backend.Messaging
	routing      backend.EnvironmentRouting
	dependencies backend.Dependencies
	mu           sync.Mutex
	buses        map[string]backend.Messaging
}

func (r *Registry) connectEnvironments(_ context.Context, base backend.Messaging, routing backend.EnvironmentRouting, deps backend.Dependencies) (backend.Messaging, error) {
	return &environmentMessaging{registry: r, base: base, routing: routing, dependencies: deps, buses: map[string]backend.Messaging{}}, nil
}
func (m *environmentMessaging) bus(ctx context.Context, destination backend.EnvironmentDestination) (backend.Messaging, error) {
	if destination.ID == "" {
		return m.base, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if bus := m.buses[destination.ID]; bus != nil {
		return bus, nil
	}
	route := destination.Route
	definition, ok := m.registry.Messaging(route.Name)
	if !ok {
		return nil, fmt.Errorf("environment %q needs a messaging route", destination.ID)
	}
	if err := definition.ValidateConfiguration(route.Configuration); err != nil {
		return nil, err
	}
	bus, err := definition.Connect(ctx, route.Configuration, m.dependencies)
	if err != nil {
		return nil, err
	}
	m.buses[destination.ID] = bus
	return bus, nil
}
func (m *environmentMessaging) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	destinations, err := m.routing.Destinations(ctx, r)
	if err != nil {
		return err
	}
	if len(destinations) == 0 {
		if r.Kind != "cleanup" {
			return fmt.Errorf("no environment owns %s %s", r.Kind, r.Task)
		}
		// A parent scope may have no remaining assignments after its children
		// released their last uses. There is no native work left to contact.
		return m.base.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, Kind: "completed", Result: &backend.MessageResult{}})
	}
	if r.Kind != "cleanup" {
		if len(destinations) != 1 {
			return fmt.Errorf("multiple environments own %s %s", r.Kind, r.Task)
		}
		bus, err := m.bus(ctx, destinations[0])
		if err != nil {
			return err
		}
		return bus.PublishRequest(ctx, r)
	}
	// Publish one aggregate cleanup result after every relevant daemon replies.
	// Native failures and delivery uncertainty remain distinguishable.
	if err := m.base.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, Kind: "accepted"}); err != nil {
		return err
	}
	go func() {
		result := &backend.MessageResult{}
		var mu sync.Mutex
		var group sync.WaitGroup
		for _, destination := range destinations {
			group.Go(func() {
				bus, err := m.bus(ctx, destination)
				if err == nil {
					child := r
					child.ID = ulid.Make().String()
					child.Created = time.Now().UTC()
					err = bus.PublishRequest(ctx, child)
					if err == nil {
						done := errors.New("cleanup complete")
						err = bus.ListenEvents(ctx, backend.MessageSelection{RequestID: child.ID}, child.Created, func(e backend.MessageEvent) error {
							if e.Kind != "completed" || e.Result == nil {
								return nil
							}
							if e.Result.Error != "" || e.Result.Unresolved {
								return fmt.Errorf("environment %s cleanup: %s", destination.ID, e.Result.Error)
							}
							return done
						})
						if errors.Is(err, done) {
							err = nil
						}
					}
				}
				if err != nil {
					mu.Lock()
					result.Unresolved = true
					result.Error += err.Error() + "; "
					mu.Unlock()
				}
			})
		}
		group.Wait()
		_ = m.base.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, Kind: "completed", Result: result})
	}()
	return nil
}
func (m *environmentMessaging) PublishEvent(ctx context.Context, e backend.MessageEvent) error {
	return m.base.PublishEvent(ctx, e)
}
func (m *environmentMessaging) ListenEvents(ctx context.Context, selection backend.MessageSelection, since time.Time, fn func(backend.MessageEvent) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan backend.MessageEvent)
	failures := make(chan error)
	started := map[string]bool{}
	seen := map[string]bool{}
	start := func(destination backend.EnvironmentDestination) {
		started[destination.ID] = true
		go func() {
			bus, err := m.bus(ctx, destination)
			if err == nil {
				err = bus.ListenEvents(ctx, selection, since, func(e backend.MessageEvent) error {
					select {
					case events <- e:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}
			select {
			case failures <- err:
			case <-ctx.Done():
			}
		}()
	}
	start(backend.EnvironmentDestination{})
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case e := <-events:
			if !seen[e.ID] {
				seen[e.ID] = true
				if err := fn(e); err != nil {
					return err
				}
			}
		case <-ticker.C:
			active, err := m.routing.ActiveEnvironments(ctx)
			if err != nil {
				return err
			}
			for _, destination := range active {
				if !started[destination.ID] {
					start(destination)
				}
			}
		}
	}
}
func (m *environmentMessaging) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for _, bus := range m.buses {
		if closer, ok := bus.(io.Closer); ok {
			errs = append(errs, closer.Close())
		}
	}
	if closer, ok := m.base.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}
