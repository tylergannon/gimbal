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
	controller   *localOrchestrator
	dependencies backend.Dependencies
	mu           sync.Mutex
	buses        map[string]backend.Messaging
}

func (r *Registry) connectEnvironments(ctx context.Context, base backend.Messaging, controller *localOrchestrator, deps backend.Dependencies) (backend.Messaging, error) {
	for name, environment := range controller.config.Environments {
		d, ok := r.Messaging(environment.Route.Name)
		if !ok {
			return nil, fmt.Errorf("environment %q needs a messaging route", name)
		}
		if err := d.ValidateConfiguration(environment.Route.Configuration); err != nil {
			return nil, err
		}
	}
	return &environmentMessaging{registry: r, base: base, controller: controller, dependencies: deps, buses: map[string]backend.Messaging{}}, nil
}
func (m *environmentMessaging) bus(ctx context.Context, name string) (backend.Messaging, error) {
	if name == "" {
		return m.base, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if bus := m.buses[name]; bus != nil {
		return bus, nil
	}
	route := m.controller.config.Environments[name].Route
	definition, _ := m.registry.Messaging(route.Name)
	bus, err := definition.Connect(ctx, route.Configuration, m.dependencies)
	if err != nil {
		return nil, err
	}
	m.buses[name] = bus
	return bus, nil
}
func (m *environmentMessaging) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	names, err := m.controller.assigned(r)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		if r.Kind != "cleanup" {
			return fmt.Errorf("no environment owns %s %s", r.Kind, r.Task)
		}
		// A parent scope may have no remaining assignments after its children
		// released their last uses. There is no native work left to contact.
		return m.base.PublishEvent(ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, Kind: "completed", Result: &backend.MessageResult{}})
	}
	if r.Kind != "cleanup" {
		bus, err := m.bus(ctx, names[0])
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
		for _, name := range names {
			group.Go(func() {
				bus, err := m.bus(ctx, name)
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
								return fmt.Errorf("environment %s cleanup: %s", name, e.Result.Error)
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
	failures := make(chan error, len(m.controller.config.Environments)+1)
	started := map[string]bool{}
	seen := map[string]bool{}
	start := func(name string) {
		started[name] = true
		go func() {
			bus, err := m.bus(ctx, name)
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
	start("")
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
			m.controller.mu.Lock()
			var active []string
			for name := range m.controller.config.Environments {
				use, err := m.controller.load(name)
				if err == nil && use.PID != 0 {
					active = append(active, name)
				}
			}
			m.controller.mu.Unlock()
			for _, name := range active {
				if !started[name] {
					start(name)
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
