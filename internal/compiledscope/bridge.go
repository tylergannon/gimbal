// Package compiledscope is a private seam for the handwritten compiler target.
// It exposes existing resource entry/cleanup, not workflow execution. Gimbal
// installs these functions at init to avoid an import cycle. Bindings is opaque
// here solely because ModelBinding belongs to the importing root package.
package compiledscope

import "context"

var OpenRun func(ctx context.Context, name string, bindings any) (context.Context, func(error) error, error)
var OpenScope func(ctx context.Context, name string) (context.Context, func(error) error, error)
