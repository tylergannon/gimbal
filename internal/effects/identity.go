package effects

import (
	"context"
	"encoding/json"
	"github.com/tylergannon/gimbal/backend"
)

type executionIdentityKey struct{}

func WithExecutionIdentity(ctx context.Context, id backend.ExecutionIdentity) context.Context {
	return context.WithValue(ctx, executionIdentityKey{}, id)
}
func ExecutionIdentityFromContext(ctx context.Context) backend.ExecutionIdentity {
	id, _ := ctx.Value(executionIdentityKey{}).(backend.ExecutionIdentity)
	return id
}

// Standalone producers must call this at their operation boundary. Workflow
// calls always carry a recorded identity and never allocate here.
func OperationIdentity(ctx context.Context, kind string) backend.ExecutionIdentity {
	if id := ExecutionIdentityFromContext(ctx); id.RequestID != "" {
		return id
	}
	if Identity(ctx).Run != "" {
		return backend.ExecutionIdentity{}
	}
	return backend.NewExecutionIdentity(kind)
}

type routeKey struct{}

func WithRoute(ctx context.Context, r backend.Route) context.Context {
	return context.WithValue(ctx, routeKey{}, r)
}
func Route(ctx context.Context) backend.Route {
	if r, ok := ctx.Value(routeKey{}).(backend.Route); ok && r.Name != "" {
		return r
	}
	if ep := Endpoint(ctx); ep != "" {
		if ep != "backend" {
			raw, _ := json.Marshal(map[string]string{"endpoint": ep})
			return backend.Route{Name: "http", Configuration: raw, Environment: Environment(ctx)}
		}
		return backend.Route{Name: ep, Environment: Environment(ctx)}
	}
	return backend.Route{Name: "local"}
}
