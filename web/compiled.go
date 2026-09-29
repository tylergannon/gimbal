package web

import (
	"context"
	"errors"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
	"github.com/tylergannon/gimbal/internal/host"
)

type contextStoreOption struct {
	project  string
	store    *contextdata.Store
	cacheDir string
}

// WithContextStore configures the immutable context store for a project,
// including artifacts served from retained runs after an instance restart.
// cacheDir must be absolute and is local to the web host; worker materialization
// is configured separately by OpenCompiledRun. Retain the store's objects for
// as long as a retained run references them. Duplicate project configuration
// and nil stores are rejected by NewInstance.
func WithContextStore(project string, store *contextdata.Store, cacheDir string) Option {
	return func(c *config) error {
		c.contextStores = append(c.contextStores, contextStoreOption{project, store, cacheDir})
		return nil
	}
}

// CompiledControls connects console cancellation to the consumer's backend run.
// CancelRun is required and must honor its context deadline. A nil result means
// the backend accepted the request; errors and timeouts are recorded as
// unconfirmed delivery. Local work is cancelled after the bounded delivery
// attempt even when the backend is unavailable. The caller still owns draining
// that work and finishing the run.
type CompiledControls struct {
	CancelRun func(context.Context, gimbal.Killed) error
	// DeliveryTimeout defaults to five seconds. Negative durations are invalid.
	DeliveryTimeout time.Duration
}

// OpenCompiledRun opens one compiled run in an admitted project and joins the
// instance's observations, controls and shutdown. The project must have a store
// configured with WithContextStore, and the serving binary must register the
// authored graph under name. initial is loaded from that configured store.
// localDir is the absolute, agent-visible materialization directory.
//
// The caller must react to root cancellation, stop admitting work, join active
// operations, close child scopes, and call finish once. Instance shutdown waits
// for finish. Do not also call gimbal.OpenRun for this execution.
func (i *Instance) OpenCompiledRun(ctx context.Context, project, name string,
	models map[gimbal.WorkflowRole]gimbal.ModelBinding, initial contextdata.Snapshot,
	localDir string, controls CompiledControls,
) (context.Context, func(error) error, error) {
	if i == nil || i.Owner == nil {
		return nil, nil, errors.New("gimbal: compiled run requires an instance")
	}
	p, err := i.Owner.Project(project)
	if err != nil {
		return nil, nil, err
	}
	return p.OpenCompiledRun(ctx, name, models, initial, localDir, host.CompiledControls{
		CancelRun: controls.CancelRun, DeliveryTimeout: controls.DeliveryTimeout,
	})
}
