package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tylergannon/gimbal"
)

type specimenAdapter struct {
	closeErr             error
	mu                   sync.Mutex
	next                 int
	prompts, ids, closed []string
	turn                 func(context.Context, string, string) (any, error)
}

func (a *specimenAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next++
	return fmt.Sprint(a.next), nil
}
func (a *specimenAdapter) RunTurn(ctx context.Context, id, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.mu.Lock()
	a.prompts = append(a.prompts, prompt)
	a.ids = append(a.ids, id)
	a.mu.Unlock()
	value, err := a.turn(ctx, id, prompt)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	raw, err := json.Marshal(value)
	return gimbal.TurnResult{Output: raw}, err
}
func (a *specimenAdapter) Steer(context.Context, string, string) (bool, error) { return true, nil }
func (a *specimenAdapter) Fork(ctx context.Context, id string) (string, error) {
	return a.CreateSession(ctx, "", "", "")
}
func (a *specimenAdapter) Close(_ context.Context, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = append(a.closed, id)
	return a.closeErr
}
