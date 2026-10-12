package gimbal

import (
	"context"
	"fmt"
	"os"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/envfile"
)

// StoreRequirement names a store and the values required before work starts.
// The complete visible store is granted, including non-required names.
type StoreRequirement = backend.StoreRequirement

// Requirements groups the stores and names needed by consuming work.
type Requirements = backend.Requirements

// WithRequirements declares store access and required names for commands and
// agent turns issued with ctx. Names and store declarations are constants at
// their call sites. Agent values are obtained through the Gimbal callback.
func WithRequirements(ctx context.Context, requirements ...StoreRequirement) context.Context {
	return effects.WithRequirements(ctx, requirements)
}

// WithYields declares a command or service's output names and destination store.
// The producer writes NAME=value lines to ENV_FILE. Services atomically rename
// a closed file into place when ready, including when names is empty. Finite
// commands publish after successful exit. An empty store selects the default.
func WithYields(ctx context.Context, store string, names ...string) context.Context {
	return effects.WithYields(ctx, store, names)
}

// localProcessEnvironment obtains values at the executor, never in an effect.
func localProcessEnvironment(ctx context.Context, s *scope, service bool) ([]string, string, func(), error) {
	secrets := effects.Connection(ctx).Secrets
	if secrets == nil {
		return nil, "", func() {}, fmt.Errorf("secrets backend is unavailable")
	}
	values, err := secrets.Acquire(ctx, backend.Scope{Run: effectOwner(s, "").Run, Scope: s.key}, effects.Requirements(ctx))
	if err != nil {
		return nil, "", func() {}, err
	}
	environment := os.Environ()
	for name, value := range values {
		environment = append(environment, name+"="+value)
	}
	_, names, produces := effects.Yields(ctx)
	if !service && !produces && len(names) == 0 {
		return environment, "", func() {}, nil
	}
	file, cleanup, err := envfile.Prepare()
	if err != nil {
		return nil, "", cleanup, err
	}
	return append(environment, "ENV_FILE="+file), file, cleanup, nil
}
func publishLocalOutputs(ctx context.Context, s *scope, file string) error {
	store, names, _ := effects.Yields(ctx)
	values, err := envfile.Read(file, names)
	if err != nil {
		return err
	}
	return effects.Connection(ctx).Secrets.Publish(ctx, backend.Scope{Run: effectOwner(s, "").Run, Scope: s.key}, store, values)
}
