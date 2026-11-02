package fixture

import (
	"context"

	"github.com/tylergannon/gimbal"
)

func DeclarationShape(ctx context.Context, _ gimbal.Env) error {
	original := ctx
	ctx = gimbal.WithRequirements(ctx, "credentials", "TOKEN")
	ctx = gimbal.WithRequirements(ctx, gimbal.DefaultStore, "DATABASE_URL")
	if err := gimbal.Service(gimbal.WithYields(ctx, gimbal.DefaultStore, "WEB_URL"), "web", ".", "serve"); err != nil {
		return err
	}
	if err := gimbal.Check(gimbal.WithYields(ctx, "report", "RESULT"), "produce", ".", "check"); err != nil {
		return err
	}
	_, _, _, _ = gimbal.RunCommand(original, "unrelated", ".", "true")
	declarationHelper(ctx)
	return gimbal.Scope(ctx, "nested", func(child context.Context) error {
		child = gimbal.WithRequirements(child, "child", "EXTRA")
		group := gimbal.Group(child, "parallel")
		group.Go("read", func(task context.Context) error {
			session := gimbal.NewSession(task, "reader", ".")
			_, err := session.Generate[gimbal.Text](task, "Read the values")
			return err
		})
		return group.Wait()
	})
}

func declarationHelper(ctx context.Context) {
	_, _, _, _ = gimbal.RunCommand(ctx, "helper", ".", "true")
}

// EnvironmentShape declares constant and direct-input placements.
type EnvironmentParams struct {
	Environment string `json:"environment"`
}

func EnvironmentShape(ctx context.Context, env gimbal.Env, params EnvironmentParams) error {
	first := gimbal.WithEnvironment(ctx, "alpha")
	gimbal.RunCommand(first, "one", env.WorkDir, "true")
	second := gimbal.WithEnvironment(ctx, params.Environment)
	gimbal.RunCommand(second, "two", env.WorkDir, "true")
	computed := gimbal.WithEnvironment(ctx, params.Environment+"-extra")
	gimbal.RunCommand(computed, "three", env.WorkDir, "true")
	return nil
}

func MutatedEnvironmentShape(ctx context.Context, env gimbal.Env, params EnvironmentParams) error {
	params.Environment += "-computed"
	ctx = gimbal.WithEnvironment(ctx, params.Environment)
	_, _, _, err := gimbal.RunCommand(ctx, "work", env.WorkDir, "true")
	return err
}

func (p *EnvironmentParams) changeEnvironment() { p.Environment = "other" }
func MethodMutatedEnvironmentShape(ctx context.Context, env gimbal.Env, params EnvironmentParams) error {
	params.changeEnvironment()
	ctx = gimbal.WithEnvironment(ctx, params.Environment)
	_, _, _, err := gimbal.RunCommand(ctx, "work", env.WorkDir, "true")
	return err
}
