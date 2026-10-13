package taskdaemon

import (
	"context"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/envfile"
)

// Acquisition is owned before entering an injected dependency so cancellation
// and scope cleanup can join it without holding the daemon lock.
func (d *Daemon) acquire(ctx context.Context, owner backend.Owner, requires backend.Requirements) (map[string]string, func(), error) {
	d.mu.Lock()
	if err := d.admit(owner); err != nil {
		d.mu.Unlock()
		return nil, func() {}, err
	}
	id := key(owner.Run, owner.Task)
	if d.admitting[id] != nil {
		d.mu.Unlock()
		return nil, func() {}, fmt.Errorf("admission already in progress")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	pending := &task{owner: owner, admissionCtx: ctx, cancel: cancel, done: make(chan struct{})}
	if d.admitting == nil {
		d.admitting = map[string]*task{}
	}
	d.admitting[id] = pending
	d.mu.Unlock()
	finish := func() { cancel(); d.mu.Lock(); delete(d.admitting, id); d.mu.Unlock(); close(pending.done) }
	if len(requires) == 0 {
		return nil, finish, nil
	}
	if d.Secrets == nil {
		return nil, finish, fmt.Errorf("requirements need a secrets backend")
	}
	values, err := d.Secrets.Acquire(ctx, backend.Scope{Run: owner.Run, Scope: owner.Scope}, requires)
	if err == nil {
		err = ctx.Err()
	}
	return values, finish, err
}

func (d *Daemon) publishProducer(ctx context.Context, req backend.Process, path string) error {
	values, err := envfile.Read(path, req.Yields)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	if d.Secrets == nil {
		return fmt.Errorf("producer requires a secrets backend")
	}
	write := backend.StoreWrite{Store: req.Store}
	for name, value := range values {
		write.Values = append(write.Values, backend.NamedValue{Name: name, Value: value})
	}
	return d.Secrets.Publish(ctx, backend.Scope{Run: req.Run, Scope: req.Scope}, write)
}
