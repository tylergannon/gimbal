package host

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
	"github.com/tylergannon/gimbal/internal/live"
)

// CompiledControls carries the public web entry controls to the runtime.
type CompiledControls struct {
	CancelRun       func(context.Context, gimbal.Killed) error
	DeliveryTimeout time.Duration
}
type contextStoreConfig struct {
	store    *contextdata.Store
	cacheDir string
}
type contextStoresKey struct{}

func contextStores(ctx context.Context) map[string]contextStoreConfig {
	stores, _ := ctx.Value(contextStoresKey{}).(map[string]contextStoreConfig)
	return stores
}

// WithContextStore supplies host access at startup, including retained runs
// served after a restart. Worker materialization is configured at run entry.
func WithContextStore(ctx context.Context, project string, store *contextdata.Store, cacheDir string) (context.Context, error) {
	if ctx == nil || store == nil {
		return nil, fmt.Errorf("gimbal: context store requires context and store")
	}
	canonical, err := CanonicalProject(project)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cacheDir) {
		return nil, fmt.Errorf("gimbal: host context cache must be absolute")
	}
	stores := make(map[string]contextStoreConfig)
	maps.Copy(stores, contextStores(ctx))
	if _, exists := stores[canonical]; exists {
		return nil, fmt.Errorf("gimbal: duplicate context-store configuration for %s", canonical)
	}
	stores[canonical] = contextStoreConfig{store, cacheDir}
	return context.WithValue(ctx, contextStoresKey{}, stores), nil
}

type compiledController struct {
	live.Controller
	controls CompiledControls
}

func (c *compiledController) CancelScope(key string, cause error) error {
	if key != "" {
		return fmt.Errorf("gimbal: compiled runs support whole-run cancellation only")
	}
	runtime, ok := c.Controller.(interface {
		CancelHostedRun(gimbal.Killed, func(context.Context, gimbal.Killed) error, time.Duration) error
	})
	if !ok {
		return fmt.Errorf("gimbal: compiled run is missing hosted cancellation support")
	}
	killed, ok := cause.(gimbal.Killed)
	if !ok {
		killed = gimbal.Killed{Reason: cause.Error()}
	}
	return runtime.CancelHostedRun(killed, c.controls.CancelRun, c.controls.DeliveryTimeout)
}
