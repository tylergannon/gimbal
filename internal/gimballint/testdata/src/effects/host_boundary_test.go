package effects

import (
	"context"
	"github.com/tylergannon/gimbal"
)

// A host-side test may exercise a runtime helper from an authored callback.
// Its production helper must not become an authored admission root just because
// that harness uses it; real workflow roots still traverse the same helper.
func exerciseHostBoundary() {
	_ = gimbal.Run(context.Background(), "host", "project", nil, func(ctx context.Context) error {
		hostRuntimeHelper()
		return nil
	})
}
