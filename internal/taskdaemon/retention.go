package taskdaemon

import (
	"os"
	"time"
)

const retention = 5 * time.Minute

// Payloads can retire only after native ownership has been confirmed released.
func (d *Daemon) retireLocked(t *task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.released || (!t.acknowledged && time.Since(t.completed) < retention) {
		return
	}
	for _, p := range t.captures {
		_ = os.Remove(p)
	}
	t.expired = true
	t.captures = nil
	t.events = nil
	t.result = nil
}
func (d *Daemon) expireLocked(now time.Time) {
	for id, t := range d.tasks {
		d.retireLocked(t)
		t.mu.Lock()
		remove := t.expired && now.Sub(t.completed) >= 2*retention
		t.mu.Unlock()
		if remove {
			delete(d.tasks, id)
		}
	}
	for scope, closed := range d.closed {
		if now.Sub(closed) < retention {
			continue
		}
		pending := false
		for _, t := range d.tasks {
			if t.owner.Run == scope.Run && within(t.owner.Scope, scope.Scope) && !t.released {
				pending = true
			}
		}
		for _, s := range d.sessions {
			if s.owner.Run == scope.Run && within(s.owner.Scope, scope.Scope) {
				pending = true
			}
		}
		if !pending {
			delete(d.closed, scope)
		}
	}
}
