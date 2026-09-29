// Package resulttypes exercises the admitted Polytype result contract.
package resulttypes

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate

type State string

const Ready State = "ready"
const Waiting State = "waiting"

func (State) enum() {}

type Outcome interface{ outcome() }
type Success struct {
	Message string `json:"message"`
}

func (Success) outcome() {}

type Rejected struct {
	Reason string `json:"reason"`
}

func (*Rejected) outcome() {}

type Detail struct {
	Outcome Outcome `json:"outcome"`
	Flags   []bool  `json:"flags"`
}
type Result struct {
	State   State  `json:"state"`
	Detail  Detail `json:"detail"`
	Receipt string `json:"receipt"`
}

// Numeric demonstrates the pinned schema/Go decoder gap; it is deliberately
// outside the compiled result contract until Polytype guarantees that boundary.
type Numeric struct {
	Count int8 `json:"count"`
}

// Collision exposes case-folding between the union discriminator and a field.
type CollisionOutcome interface{ collision() }
type Collision struct {
	Flag bool `json:"TYPE"`
}

func (Collision) collision() {}

type CollisionResult struct {
	Outcome CollisionOutcome `json:"outcome"`
}

func Results(ctx context.Context, env gimbal.Env) error {
	session := gimbal.NewSession(ctx, "coder", env.WorkDir)
	result, err := session.Generate[Result](ctx, "Return a ready result with a receipt and outcome.")
	if err != nil {
		return err
	}
	if result.State != Ready {
		return fmt.Errorf("result is not ready")
	}
	gimbal.SetJSON(ctx, "original", result)
	result.Receipt = "consumed:" + result.Receipt
	gimbal.SetJSON(ctx, "consumed", result)
	return nil
}
