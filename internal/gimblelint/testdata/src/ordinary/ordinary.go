package ordinary

import "context"

type callback func(context.Context) error

func first(context.Context) error { return nil }

// This package has no Gimble workflow entrypoint. Conventional callback and
// collection dispatch in ordinary library code are outside the lint domain.
func Run(ctx context.Context, callbacks map[string]callback, name string, cb callback) error {
	if err := callbacks[name](ctx); err != nil {
		return err
	}
	return cb(ctx)
}
