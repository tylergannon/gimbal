package gimbal

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

// Keep this experimental seam internal: ordinary workflows still use Run and
// Scope. Both paths share the same entry, resource cleanup and event recording.
func init() {
	compiledscope.CancelRun = func(ctx context.Context) error {
		s, err := current(ctx)
		if err != nil {
			return err
		}
		return s.run.CancelScope("", context.Canceled)
	}
	compiledscope.OpenRun = func(ctx context.Context, name string, bindings any) (context.Context, func(error) error, error) {
		models, ok := bindings.(map[WorkflowRole]ModelBinding)
		if !ok {
			return nil, nil, fmt.Errorf("gimbal: invalid compiled run bindings %T", bindings)
		}
		r, release, err := beginRun(ctx, name, models)
		if err != nil {
			return nil, nil, err
		}
		root := &scope{run: r}
		return root.begin(ctx), func(err error) error {
			err = root.finish(err)
			release()
			return r.complete(ctx, name, err)
		}, nil
	}
	compiledscope.OpenScope = func(ctx context.Context, name string) (context.Context, func(error) error, error) {
		parent, err := current(ctx)
		if err != nil {
			return nil, nil, err
		}
		child := parent.child(name)
		return child.begin(ctx), child.finish, nil
	}
}
