// Package compiledscope is the private integration seam used by the compiler
// experiment. Runtime semantics stay in Gimbal; this leaf package owns only
// context data, transport types and delegation to the owning runtime.
package compiledscope

import (
	"context"
	"fmt"
)

// OpenRun is the single bootstrap hook required by the root/leaf import cycle.
// Every subsequent operation is dispatched to the runtime attached to its scope.
var OpenRun func(context.Context, string, any) (context.Context, func(error) error, error)

type Runtime interface {
	OpenScope(context.Context, string) (context.Context, func(error) error, error)
	OpenLoop(context.Context, string) (context.Context, func(error) error, error)
	OpenTask(context.Context, string, []byte, Snapshot) (context.Context, Snapshot, func(error) error, error)
	InitializeContext(context.Context, Store, string, Snapshot) error
	WriteContext(context.Context, Snapshot, ...Entry) (Snapshot, error)
	BindContext(context.Context, Snapshot) (context.Context, error)
	CheckContext(context.Context, Snapshot, string, string, string, ...string) (Snapshot, CheckResult, error)
	GenerateResponse(context.Context, any, Snapshot, string, Output) ([]byte, error)
	Plan(context.Context, any, Snapshot, string, []byte, string) ([]byte, error)
	RecordPlan(context.Context, string, []byte) error
	TaskFeedback(context.Context) (string, error)
	EndLoop(context.Context) error
	CancelRun(context.Context) error
}
type runtimeKey struct{}

func WithRuntime(ctx context.Context, r Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, r)
}
func runtime(ctx context.Context) (Runtime, error) {
	r, _ := ctx.Value(runtimeKey{}).(Runtime)
	if r == nil {
		return nil, fmt.Errorf("gimbal: no compiled runtime on context")
	}
	return r, nil
}
func OpenScope(ctx context.Context, name string) (context.Context, func(error) error, error) {
	r, e := runtime(ctx)
	if e != nil {
		return nil, nil, e
	}
	return r.OpenScope(ctx, name)
}
func OpenLoop(ctx context.Context, name string) (context.Context, func(error) error, error) {
	r, e := runtime(ctx)
	if e != nil {
		return nil, nil, e
	}
	return r.OpenLoop(ctx, name)
}
func OpenTask(ctx context.Context, name string, task []byte, base Snapshot) (context.Context, Snapshot, func(error) error, error) {
	r, e := runtime(ctx)
	if e != nil {
		return nil, base, nil, e
	}
	return r.OpenTask(ctx, name, task, base)
}
func InitializeContext(ctx context.Context, store Store, localDir string, initial Snapshot) error {
	r, e := runtime(ctx)
	if e != nil {
		return e
	}
	return r.InitializeContext(ctx, store, localDir, initial)
}
func WriteContext(ctx context.Context, base Snapshot, writes ...Entry) (Snapshot, error) {
	r, e := runtime(ctx)
	if e != nil {
		return base, e
	}
	return r.WriteContext(ctx, base, writes...)
}
func BindContext(ctx context.Context, ref Snapshot) (context.Context, error) {
	r, e := runtime(ctx)
	if e != nil {
		return nil, e
	}
	return r.BindContext(ctx, ref)
}
func CheckContext(ctx context.Context, base Snapshot, key, dir, command string, args ...string) (Snapshot, CheckResult, error) {
	r, e := runtime(ctx)
	if e != nil {
		return base, CheckResult{}, e
	}
	return r.CheckContext(ctx, base, key, dir, command, args...)
}
func Plan(ctx context.Context, session any, input Snapshot, goal string, tasks []byte, previous string) ([]byte, error) {
	r, e := runtime(ctx)
	if e != nil {
		return nil, e
	}
	return r.Plan(ctx, session, input, goal, tasks, previous)
}
func RecordPlan(ctx context.Context, goal string, plan []byte) error {
	r, e := runtime(ctx)
	if e != nil {
		return e
	}
	return r.RecordPlan(ctx, goal, plan)
}
func TaskFeedback(ctx context.Context) (string, error) {
	r, e := runtime(ctx)
	if e != nil {
		return "", e
	}
	return r.TaskFeedback(ctx)
}
func EndLoop(ctx context.Context) error {
	r, e := runtime(ctx)
	if e != nil {
		return e
	}
	return r.EndLoop(ctx)
}
func CancelRun(ctx context.Context) error {
	r, e := runtime(ctx)
	if e != nil {
		return e
	}
	return r.CancelRun(ctx)
}

// CheckResult is the canonical result captured from command observations.
type CheckResult struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Workdir  string   `json:"workdir"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	Error    string   `json:"error,omitempty"`
}
