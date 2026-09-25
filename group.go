package gimbal

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

type group struct {
	scope *scope
	ctx   context.Context
	wg    sync.WaitGroup
	once  sync.Once
	err   error
}

// Group opens an errgroup-shaped concurrent scope named name. Call its Go
// method for each child, then return its Wait method's result:
//
//	group := gimbal.Group(ctx, "candidates")
//	group.Go("candidate", first)
//	group.Go("candidate", second)
//	return group.Wait()
//
// Each child receives its own named scope. The first error cancels the group,
// interrupting the other children's turns and services; Wait joins every
// child, ends the group scope, and returns that first error. A child killed by
// an operator (its error's cause is a Killed) is gone, not an abort: its
// siblings run on, and Wait still returns its error.
// The caller must Wait on every exit path. To stop children early, cancel
// their parent context before waiting; cancellation alone does not join them.
func Group(ctx context.Context, name string) *group {
	parent, err := current(ctx)
	if err != nil {
		return &group{err: err}
	}
	g := &group{scope: parent.child(name)}
	g.ctx, g.scope.cancel = context.WithCancelCause(context.WithValue(ctx, scopeKey{}, g.scope))
	g.scope.ctx = g.ctx
	g.scope.run.addScope(g.scope)
	g.scope.run.event(g.scope.key, "", "", ScopeBegan{Name: name})
	return g
}

// panicError is a child's panic, recovered by Go so that the group ends
// like any other failed group and Run can record the run before the panic
// continues. Its text carries the panic value and the child's stack.
type panicError struct {
	value any
	stack []byte
}

func (e *panicError) Error() string {
	return fmt.Sprintf("gimbal: panic: %v\n\n%s", e.value, e.stack)
}

// Go runs fn in a goroutine, in a child scope of the group named name. A
// panic in fn is the child's error: it cancels the group's other children,
// Wait returns it, and a Run that gets it back from its body records the
// run's end and then panics with it.
func (g *group) Go(name string, fn func(ctx context.Context) error) {
	if g.scope == nil {
		return
	}
	child := g.scope.child(name)
	g.wg.Go(func() {
		err := child.do(g.ctx, func(ctx context.Context) (err error) {
			defer func() {
				if v := recover(); v != nil {
					err = &panicError{value: v, stack: debug.Stack()}
				}
			}()
			return fn(ctx)
		})
		if err == nil {
			return
		}
		g.once.Do(func() { g.err = err })
		if _, ok := errors.AsType[Killed](err); !ok {
			g.scope.cancel(nil)
		}
	})
}

// Wait joins the group's goroutines, ends its scope, and returns the first
// error one of them returned. Call Wait once, after submitting all children.
// Child results may be read after Wait returns.
func (g *group) Wait() error {
	if g.scope == nil {
		return g.err
	}
	g.wg.Wait()
	g.err = g.scope.finish(g.err)
	return g.err
}
