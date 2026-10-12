package valuechecks

import (
	"context"

	"github.com/tylergannon/gimbal"
)

func declarations(ctx context.Context, dynamic string) {
	_ = gimbal.WithYields(ctx, "db", "DATABASE_URL")
	_ = gimbal.WithYields(ctx, "db", dynamic)      // want "GIMBAL111-CONSTANT-VALUES"
	_ = gimbal.WithYields(ctx, "db", "URL", "URL") // want "GIMBAL112-DUPLICATE-YIELD"
	_ = gimbal.WithRequirements(ctx, gimbal.StoreRequirement{Store: "db", Names: []string{"DATABASE_URL"}})
	_ = gimbal.WithRequirements(ctx, gimbal.StoreRequirement{Store: dynamic})           // want "GIMBAL111-CONSTANT-VALUES"
	_ = gimbal.WithRequirements(ctx, gimbal.StoreRequirement{Names: []string{dynamic}}) // want "GIMBAL111-CONSTANT-VALUES"
}
