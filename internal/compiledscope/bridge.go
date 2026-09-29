// Package compiledscope is a private seam for experimental compiler targets.
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

// Loop support exposes single-dispatch mechanics only. The compiler output
// owns repetition, task selection, scheduling and feedback handoff.
var OpenLoop func(context.Context, string) (context.Context, func(error) error, error)
var OpenTask func(context.Context, string, []byte) (context.Context, func(error) error, error)

// Plan takes the authored loop name and goal; the session supplies its workdir.
var Plan func(context.Context, any, string, string, []byte, string) ([]byte, error)
var RecordPlan func(context.Context, string, []byte) error
var TaskFeedback func(context.Context) (string, error)
var EndLoop func(context.Context) error

var CancelRun func(context.Context) error
