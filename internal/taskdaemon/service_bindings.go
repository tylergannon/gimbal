package taskdaemon

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

var serviceName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (d *Daemon) serviceBindings(ctx context.Context, owner backend.Owner, kind, consumer string, names []string) ([]backend.ServiceBinding, func(), error) {
	if len(names) == 0 {
		return nil, func() {}, nil
	}
	d.mu.Lock()

	if err := d.admit(owner); err != nil {
		d.mu.Unlock()
		return nil, func() {}, err
	}
	if kind == "agent" {
		session := d.sessions[key(owner.Run, consumer)]
		if session == nil {
			d.mu.Unlock()
			return nil, func() {}, fmt.Errorf("service consumer session not found")
		}
		select {
		case <-session.ready:
			if session.native == "" {
				d.mu.Unlock()
				return nil, func() {}, fmt.Errorf("service consumer session creation failed")
			}
		default:
			d.mu.Unlock()
			return nil, func() {}, fmt.Errorf("service consumer session is being created")
		}
	}
	id := key(owner.Run, owner.Task)
	if d.admitting[id] != nil {
		d.mu.Unlock()
		return nil, func() {}, fmt.Errorf("service consumer admission already in progress")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pending := &task{owner: owner, admissionCtx: ctx, cancel: cancel, done: make(chan struct{})}
	if d.admitting == nil {
		d.admitting = map[string]*task{}
	}
	d.admitting[id] = pending
	d.mu.Unlock()
	finish := func() { cancel(); d.mu.Lock(); delete(d.admitting, id); d.mu.Unlock(); close(pending.done) }
	if d.Orchestration == nil {
		return nil, finish, fmt.Errorf("named services require an explicitly constructed orchestration dependency")
	}
	// Validate the complete declaration before acquiring any service.
	for _, name := range names {
		if !serviceName.MatchString(name) {
			return nil, finish, fmt.Errorf("invalid service name %q", name)
		}
	}
	var bindings []backend.ServiceBinding
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		binding, err := d.Orchestration.Acquire(ctx, backend.ServiceUse{Scope: backend.Scope{Run: owner.Run, Scope: owner.Scope}, Name: name, Kind: kind, Consumer: consumer})
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			return nil, finish, fmt.Errorf("service %s for %s %s: %w", name, kind, consumer, err)
		}
		parsed, err := url.Parse(binding.URL)
		if err != nil || binding.Name != name || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, finish, fmt.Errorf("service %s returned an invalid non-secret HTTP binding", name)
		}
		bindings = append(bindings, binding)
	}
	return bindings, finish, nil
}
