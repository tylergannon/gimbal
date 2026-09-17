package gimble

import (
	"context"
	"iter"
)

// Iterate yields each item with a fresh child scope named scope. Sessions
// created with the yielded ctx close before the next item is yielded. Items
// retain their order; cancellation stops iteration.
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
			_ = parent.child(scope).do(ctx, func(itemCtx context.Context) error {
				more = yield(itemCtx, item)
				return nil
			})
			if !more {
				return
			}
		}
	}
}
