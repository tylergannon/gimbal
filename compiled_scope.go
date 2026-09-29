package gimbal

import (
	"context"
	"fmt"
	"sync"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

// Keep this experimental seam internal: ordinary workflows still use Run and
// Scope. Both paths share the same entry, resource cleanup and event recording.
func init() {
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
		var finishMu sync.Mutex
		return root.begin(ctx), func(err error) error {
			finishMu.Lock()
			defer finishMu.Unlock()
			root.mu.Lock()
			wasEnded := root.ended
			root.mu.Unlock()
			if wasEnded {
				return fmt.Errorf("gimbal: run already ended")
			}
			r.cancelDeliveryMu.Lock()
			defer r.cancelDeliveryMu.Unlock()
			err = root.finishCompiled(err)
			root.mu.Lock()
			ended := root.ended
			root.mu.Unlock()
			if !ended {
				return err
			}
			release()
			return r.complete(ctx, name, err)
		}, nil
	}
}

type compiledRuntime struct{}

func (compiledRuntime) CancelRun(ctx context.Context) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	return s.run.CancelScope("", context.Canceled)
}

func (compiledRuntime) OpenScope(ctx context.Context, name string) (context.Context, func(error) error, error) {
	parent, err := current(ctx)
	if err != nil {
		return nil, nil, err
	}
	child := parent.child(name)
	return child.begin(ctx), child.finishCompiled, nil
}

// Explicit compiled lifetimes require the consumer to join its work and close
// children first. Local Scope instead always ends when its body returns.
func (s *scope) finishCompiled(bodyErr error) error {
	s.finishMu.Lock()
	defer s.finishMu.Unlock()
	if err := s.canFinish(); err != nil {
		return err
	}
	return s.finish(bodyErr)
}

// canFinish checks lifetime legality without changing the runtime's state.
func (s *scope) canFinish() error {
	s.run.mu.Lock()
	var openChild string
	for _, child := range s.run.scopes {
		if child.parent == s {
			openChild = child.key
			break
		}
	}
	s.run.mu.Unlock()
	if openChild != "" {
		return fmt.Errorf("gimbal: scope %q has open child %q", s.key, openChild)
	}
	s.mu.Lock()
	ended := s.ended
	s.mu.Unlock()
	if ended {
		return fmt.Errorf("gimbal: scope %q already ended", s.key)
	}
	return nil
}
