package effects

import (
	"context"
	"encoding/json"
	"github.com/tylergannon/gimbal/backend"
)

type observationKey struct{}
type observationOptions struct {
	detach    bool
	cursor    int
	task      func(int, json.RawMessage)
	message   func(backend.MessageEvent)
	placement func(backend.MessageEvent)
}

// ObservePlacement records admitted placement from transport facts.
func ObservePlacement(ctx context.Context, apply func(backend.MessageEvent)) context.Context {
	o, _ := ctx.Value(observationKey{}).(observationOptions)
	o.placement = apply
	return context.WithValue(ctx, observationKey{}, o)
}

// ObserveDetached makes cancellation release a reader, never native work. A
// durable conversation uses separate, identified interrupt requests for Stop.
func ObserveDetached(ctx context.Context) context.Context {
	o, _ := ctx.Value(observationKey{}).(observationOptions)
	o.detach = true
	return context.WithValue(ctx, observationKey{}, o)
}
func detached(ctx context.Context) bool {
	o, _ := ctx.Value(observationKey{}).(observationOptions)
	return o.detach
}

// ObserveTask commits each event with its next cursor in the durable owner.
func ObserveTask(ctx context.Context, cursor int, apply func(int, json.RawMessage)) context.Context {
	o, _ := ctx.Value(observationKey{}).(observationOptions)
	o.cursor, o.task = cursor, apply
	return context.WithValue(ctx, observationKey{}, o)
}

// ObserveMessages preserves transport event IDs for durable projection dedupe.
func ObserveMessages(ctx context.Context, apply func(backend.MessageEvent)) context.Context {
	o, _ := ctx.Value(observationKey{}).(observationOptions)
	o.message = apply
	return context.WithValue(ctx, observationKey{}, o)
}
