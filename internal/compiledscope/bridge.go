// Package compiledscope is a private seam for the handwritten compiler target.
// It exposes existing resource entry/cleanup, not workflow execution. Gimbal
// installs these functions at init to avoid an import cycle. Bindings is opaque
// here solely because ModelBinding belongs to the importing root package.
package compiledscope

import "context"

var OpenRun func(ctx context.Context, name string, bindings any) (context.Context, func(error) error, error)
var OpenScope func(ctx context.Context, name string) (context.Context, func(error) error, error)

// Context writes/rendering execute only in activities. Resource ownership stays
// on ctx; the immutable input overrides live ancestor values for consumption.
var WriteContext func(context.Context, Store, Snapshot, ...Entry) (Snapshot, error)
var BindContext func(context.Context, Store, Snapshot) (context.Context, error)
