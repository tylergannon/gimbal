package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tylergannon/gimbal"
)

// demoAdapter is a deterministic provider substitute, not an AI model. It lets
// the public compiler/runtime/Temporal/UI path run without provider credentials.
type demoAdapter struct {
	delay time.Duration
	next  atomic.Int64
}

func (a *demoAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return fmt.Sprint(a.next.Add(1)), nil
}
func (a *demoAdapter) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	select {
	case <-ctx.Done():
		return gimbal.TurnResult{}, ctx.Err()
	case <-time.After(a.delay):
	}
	raw, _ := json.Marshal("Deterministic example response: delivery ready for its check.")
	return gimbal.TurnResult{Output: raw}, nil
}
func (a *demoAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (a *demoAdapter) Fork(context.Context, string) (string, error) {
	return fmt.Sprint(a.next.Add(1)), nil
}
func (a *demoAdapter) Close(context.Context, string) error { return nil }
