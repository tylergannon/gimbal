package taskdaemon

import (
	"errors"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

func (d *Daemon) result(v backend.Target) backend.Result {
	p := d.poll(effects.Poll{Run: v.Run, ID: v.ID})
	result := backend.Result{Done: p.Done, Result: p.Result, Error: p.Error, Expired: p.Error == "task payload expired"}
	d.mu.Lock()
	task := d.tasks[key(v.Run, v.ID)]
	d.mu.Unlock()
	if task != nil {
		task.mu.Lock()
		result.InvalidOutput = errors.Is(task.err, gimbal.ErrInvalidOutput)
		task.mu.Unlock()
	}
	return result
}
