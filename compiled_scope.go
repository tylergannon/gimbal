package gimbal

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tylergannon/gimbal/contextdata"
)

// ContextAccess supplies immutable input storage and its agent-visible local cache.
// Store is copied at entry; LocalDir selects a separate cache for this run owner.
type ContextAccess struct {
	Store    *contextdata.Store
	LocalDir string
}

// OpenRun opens a process-local runtime for a consumer-owned scheduler.
// Initial input is verified and recorded before a live context is returned.
// The owner must join its work and finish child scopes before finishing the run,
// exactly once. The returned finish does not make scheduler retries idempotent.
func OpenRun(ctx context.Context, name string, models map[WorkflowRole]ModelBinding, initial contextdata.Snapshot, access ContextAccess) (context.Context, func(error) error, error) {
	if access.Store == nil {
		return nil, nil, fmt.Errorf("gimbal: compiled context store is required")
	}
	r, release, err := beginRun(ctx, name, models)
	if err != nil {
		return nil, nil, err
	}
	root := &scope{run: r}
	scoped := root.begin(ctx)
	var finishMu sync.Mutex
	finish := func(err error) error {
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
	}
	if err = initializeCompiledContext(scoped, *access.Store, access.LocalDir, initial); err != nil {
		return nil, nil, errors.Join(err, finish(err))
	}
	return scoped, finish, nil
}

// CancelRun cancels the root using its first effective cause and records one kill.
// It rejects ended runs. Calling finish remains the runtime owner's responsibility.
func CancelRun(ctx context.Context, cause error) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	return s.run.CancelScope("", cause)
}

// OpenScope opens a lexical child scope. Finish it after joining its work and
// closing its children, before resuming the parent.
func OpenScope(ctx context.Context, name string) (context.Context, func(error) error, error) {
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
