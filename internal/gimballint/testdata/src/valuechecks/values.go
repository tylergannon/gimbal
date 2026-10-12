package valuechecks

import (
	"context"

	"github.com/tylergannon/gimbal"
)

func declarations(ctx context.Context, dynamic string, names []string) {
	_ = gimbal.Service(gimbal.WithYields(ctx, "db", "DATABASE_URL"), "db", "", "db")
	_, _, _, _ = gimbal.RunCommand(gimbal.WithYields(ctx, "db", "URL"), "run", "", "run")
	_ = gimbal.Check(gimbal.WithYields(gimbal.WithRequirements(ctx, "db", "URL"), gimbal.DefaultStore), "check", "", "check")
	_ = gimbal.Check(gimbal.WithYields(ctx, "db", dynamic), "bad", "", "check")            // want "GIMBAL111-CONSTANT-VALUES"
	_ = gimbal.Check(gimbal.WithYields(ctx, "db", "URL", "URL"), "duplicate", "", "check") // want "GIMBAL112-DUPLICATE-YIELD"
	ctx = gimbal.WithRequirements(ctx, gimbal.DefaultStore, "DATABASE_URL")
	ctx = gimbal.WithRequirements(ctx, "db", "URL")
	_ = gimbal.WithRequirements(ctx, dynamic)                                                                  // want "GIMBAL111-CONSTANT-VALUES"
	_ = gimbal.WithRequirements(ctx, "db", dynamic)                                                            // want "GIMBAL111-CONSTANT-VALUES"
	_ = gimbal.WithRequirements(ctx, "db", names...)                                                           // want "GIMBAL111-CONSTANT-VALUES"
	ctx = gimbal.WithYields(ctx, "db", "URL")                                                                  // want "GIMBAL113-INLINE-YIELDS"
	_ = gimbal.WithYields(ctx, "db")                                                                           // want "GIMBAL113-INLINE-YIELDS"
	_ = gimbal.WithRequirements(gimbal.WithYields(ctx, "db", "URL"), "db")                                     // want "GIMBAL113-INLINE-YIELDS"
	_ = gimbal.Check(gimbal.WithRequirements(gimbal.WithYields(ctx, "db", "URL"), "db"), "nested", "", "true") // want "GIMBAL113-INLINE-YIELDS"
}
