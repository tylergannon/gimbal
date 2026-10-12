package effects

import (
	"context"

	"github.com/tylergannon/gimbal/backend"
)

type servicesKey struct{}

func WithServices(ctx context.Context, names []string) context.Context {
	values := append(append([]string(nil), Services(ctx)...), names...)
	return context.WithValue(ctx, servicesKey{}, values)
}
func Services(ctx context.Context) []string {
	values, _ := ctx.Value(servicesKey{}).([]string)
	return append([]string(nil), values...)
}

type resourceObserverKey struct{}

func WithResourceObserver(ctx context.Context, observe func(backend.ServiceUse)) context.Context {
	return context.WithValue(ctx, resourceObserverKey{}, observe)
}
func resourceObserver(ctx context.Context) func(backend.ServiceUse) {
	observe, _ := ctx.Value(resourceObserverKey{}).(func(backend.ServiceUse))
	return observe
}
