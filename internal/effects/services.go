package effects

import (
	"context"

	"github.com/tylergannon/gimbal/backend"
)

type requirementsKey struct{}
type yieldsKey struct{}
type environmentKey struct{}
type publication struct {
	Store string
	Names []string
}

func WithRequirements(ctx context.Context, requirements backend.Requirements) context.Context {
	values := append(Requirements(ctx), requirements...)
	return context.WithValue(ctx, requirementsKey{}, values)
}
func Requirements(ctx context.Context) backend.Requirements {
	values, _ := ctx.Value(requirementsKey{}).(backend.Requirements)
	result := make(backend.Requirements, len(values))
	for i, v := range values {
		result[i] = backend.StoreRequirement{Store: v.Store, Names: append([]string(nil), v.Names...)}
	}
	return result
}
func WithYields(ctx context.Context, store string, names []string) context.Context {
	return context.WithValue(ctx, yieldsKey{}, publication{store, append([]string(nil), names...)})
}
func Yields(ctx context.Context) (string, []string, bool) {
	p, ok := ctx.Value(yieldsKey{}).(publication)
	return p.Store, append([]string(nil), p.Names...), ok
}
func WithEnvironment(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, environmentKey{}, name)
}
func EnvironmentSelected(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(environmentKey{}).(string)
	return name, ok
}
func Environment(ctx context.Context) string {
	name, _ := ctx.Value(environmentKey{}).(string)
	return name
}

type resourceObserverKey struct{}

func WithResourceObserver(ctx context.Context, observe func(backend.ResourceFact)) context.Context {
	return context.WithValue(ctx, resourceObserverKey{}, observe)
}
func resourceObserver(ctx context.Context) func(backend.ResourceFact) {
	observe, _ := ctx.Value(resourceObserverKey{}).(func(backend.ResourceFact))
	return observe
}
