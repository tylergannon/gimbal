package gimbal

import (
	"context"
	"iter"
)

// Iterate yields each item with a fresh child scope named scope. Sessions
// created with the yielded ctx close and services it starts stop before the
// next item is yielded. Items retain their order; cancellation or a required
// service failure stops iteration.
func Iterate[T any](ctx context.Context, scope string, items []T) iter.Seq2[context.Context, T] {
	return func(yield func(context.Context, T) bool) {
		parent, err := current(ctx)
		if err != nil {
			panic(err)
		}
		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			more := true
			err := parent.child(scope).do(ctx, func(itemCtx context.Context) error {
				more = yield(itemCtx, item)
				return nil
			})
			if err != nil {
				// The iterator has no separate error channel. Its callback
				// cannot return an error, so a child error here is an owned
				// service failure and must fail the enclosing scope rather than
				// disappear between items.
				parent.failService(err)
				return
			}
			if !more {
				return
			}
		}
	}
}
