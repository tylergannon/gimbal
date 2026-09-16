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

// AgentOption stands in for gimble.AgentOption in the testdata stub.
type AgentOption func()

// Session stands in for gimble.Session: a generic Generate method whose
// prompt argument GIMBLE108 checks, matching (*gimble.Session).Generate.
type Session struct{}

func (*Session) Generate[T any](context.Context, string, ...AgentOption) (T, error) {
	var zero T
	return zero, nil
}

// WithSupervisor stands in for gimble.WithSupervisor: its instruction
// argument is the second one GIMBLE108 checks.
func WithSupervisor(*Session, string, ...AgentOption) AgentOption {
	return func() {}
}
