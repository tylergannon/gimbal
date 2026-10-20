package effects

import (
	"context"
	"effecthelper"
	"encoding/json"
	"fmt"
	"github.com/tylergannon/gimbal"
	"maps"
	"os"
	"slices"
	"time"
)

func files(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "file", "input")
	_, _ = os.ReadFile("mutable")
}
func helper() { _ = time.Now() } // want "GIMBAL111-LIBRARY-EFFECTS"
func helperEffect(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "helper", "input")
	helper()
}
func cleanup(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "defer", "input")
	defer fmt.Sprint("cleanup")
}
func recovery(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "recover", "input")
	_ = recover()
}
func cancellation(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "cancel", "input")
	_ = ctx.Err()
}
func timeout(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "timeout", "input")
	_, _ = context.WithTimeout(ctx, time.Second)
}
func channel(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "channel", "input")
	_ = make(chan bool)
}
func mapOrder(ctx context.Context, values map[string]int) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "map", "input")
	for key := range values {
		_ = key
	}
}
func iteratorOrder(ctx context.Context, values map[string]int) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "iterator", "input")
	for key := range maps.Keys(values) {
		_ = key
	}
}
func pure(ctx context.Context, values map[string]int) {
	gimbal.Set(ctx, "pure", "input")
	for _, key := range slices.Sorted(maps.Keys(values)) {
		_ = values[key]
	}
	_, _ = json.Marshal(values)
	_ = fmt.Sprintf("%d", values["one"])
}

// Host setup is not an authored workflow.
func host() { _, _ = os.ReadFile("configuration"); _ = time.Now() }

func dependencyEffect(ctx context.Context) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "dependency", "input")
	effecthelper.Effect()
}
func dependencyPure(ctx context.Context) {
	gimbal.Set(ctx, "dependency-pure", effecthelper.Pure(" input "))
}

func aliasMapOrder(ctx context.Context, values map[string]int) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "alias-map", "input")
	keys := maps.Keys[map[string]int]
	for key := range keys(values) {
		_ = key
	}
}

func hostRuntimeHelper() { defer func() {}(); _, _ = os.ReadFile("host-config") }

func nativeRuntime(ctx context.Context, adapter gimbal.HarnessAdapter) {
	defer func() {}()
	_, _ = adapter.Fork(ctx, "native")
}

func authoredNative(ctx context.Context, adapter gimbal.HarnessAdapter) { // want "GIMBAL111-LIBRARY-EFFECTS"
	gimbal.Set(ctx, "native", "input")
	_, _ = adapter.Fork(ctx, "native")
}
