package fixture

import (
	"context"
	"github.com/tylergannon/gimbal"
)

func HiddenEnvironment(ctx context.Context, _ gimbal.Env) error {
	if false {
	} else if hiddenSelector(ctx) {
	}
	return nil
}
func hiddenSelector(ctx context.Context) bool { _ = gimbal.WithEnvironment(ctx, "hidden"); return true }

func HiddenFunctionValue(ctx context.Context, _ gimbal.Env) error {
	f := gimbal.WithEnvironment
	if false {
	} else if contextSelected(f(ctx, "hidden")) {
	}
	return nil
}
func contextSelected(context.Context) bool { return true }
