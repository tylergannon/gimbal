package gimbal

import "context"

type Task struct{ Kind string }

func Set[V any](context.Context, string, V)                          {}
func SetJSON[V any](context.Context, string, V)                      {}
func Check(context.Context, string, string, string, ...string) error { return nil }

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

type PromiseLoopType struct{}

func PromiseLoop(context.Context, string, string, *Session) *PromiseLoopType {
	return &PromiseLoopType{}
}
func (*PromiseLoopType) Tasks(yield func(context.Context, Task) bool) {
	yield(context.Background(), Task{})
}
func Iterate[T any](_ context.Context, _ string, items []T) func(func(context.Context, T) bool) {
	return func(yield func(context.Context, T) bool) {
		for _, item := range items {
			if !yield(context.Background(), item) {
				return
			}
		}
	}
}

// AgentOption stands in for gimbal.AgentOption in the testdata stub.
type AgentOption func()

// Session stands in for gimbal.Session: a generic Generate method whose
// prompt argument GIMBAL108 checks, matching (*gimbal.Session).Generate.
type Session struct{}

func (*Session) Generate[T any](context.Context, string, ...AgentOption) (T, error) {
	var zero T
	return zero, nil
}

// WithSupervisor stands in for gimbal.WithSupervisor: its instruction
// argument is the second one GIMBAL108 checks.
func WithSupervisor(*Session, string, ...AgentOption) AgentOption {
	return func() {}
}

// WithScopeTemplate stands in for gimbal.WithScopeTemplate: its template
// text is what GIMBAL109 checks.
func WithScopeTemplate(string) AgentOption {
	return func() {}
}
