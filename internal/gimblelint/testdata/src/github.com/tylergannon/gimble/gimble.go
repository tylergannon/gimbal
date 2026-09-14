package gimble

import "context"

type Task struct{ Kind string }

func Set[V any](context.Context, string, V)     {}
func SetJSON[V any](context.Context, string, V) {}

func Run(ctx context.Context, _ string, body func(context.Context) error) error {
	return body(ctx)
}

func Scope(ctx context.Context, _ string, body func(context.Context) error) error {
	return body(ctx)
}

type GroupType struct{}

func Group(context.Context, string) *GroupType { return &GroupType{} }
func (*GroupType) Go(_ string, body func(context.Context) error) {
	_ = body(context.Background())
}
func (*GroupType) Wait() error { return nil }

type LoopType struct{}

func Loop(context.Context, string, string, any) *LoopType { return &LoopType{} }
func (*LoopType) Tasks(yield func(context.Context, Task) bool) {
	yield(context.Background(), Task{})
}
