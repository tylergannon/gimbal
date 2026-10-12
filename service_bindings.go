package gimbal

import (
	"context"
	"fmt"
	"os"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/envfile"
)

// DefaultStore selects the unnamed store shared by declarations in a scope.
const DefaultStore = ""

// WithRequirements declares whole-store access and names required before commands
// and agent turns issued with ctx start. Repeated calls accumulate requirements.
// Store and names must be constants. Agents obtain values through the Gimbal callback.
func WithRequirements(ctx context.Context, store string, names ...string) context.Context {
	return effects.WithRequirements(ctx, backend.Requirements{{Store: store, Names: names}})
}

// WithYields declares a command or service's output names and destination store.
// The producer writes NAME=value lines to ENV_FILE. Services atomically rename
// a closed file into place when ready, including when names is empty. Finite
// commands publish after successful exit. Use DefaultStore for the unnamed store.
// WithYields must appear inline as the context argument to Service, RunCommand,
// or Check; assigning or passing it elsewhere is rejected by the workflow linter.
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
	write := backend.StoreWrite{Store: store}
	for name, value := range values {
		write.Values = append(write.Values, backend.NamedValue{Name: name, Value: value})
	}
	return effects.Connection(ctx).Secrets.Publish(ctx, backend.Scope{Run: effectOwner(s, "").Run, Scope: s.key}, write)
}
