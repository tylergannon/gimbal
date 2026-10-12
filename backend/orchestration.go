package backend

import "context"

// Requirements grant whole stores while identifying names needed for admission.
type StoreRequirement struct {
	Store string
	Names []string
}
type Requirements []StoreRequirement

// NamedValue is a string value published under a name.
type NamedValue struct{ Name, Value string }

// StoreWrite publishes a complete batch to one store. Duplicate names fail.
type StoreWrite struct {
	Store  string
	Values []NamedValue
}

type Environment struct{ ID string }

// OrchestratorBackend establishes a ready messaging daemon and its assignment.
// Release follows native cleanup, includes descendants, and preserves unrelated use.
type OrchestratorBackend interface {
	AcquireEnvironment(ctx context.Context, owner Owner, name string) (Environment, error)
	Release(ctx context.Context, scope Scope) error
}

// SecretsBackend resolves nearest definitions before store access. Values never
// travel in workflow requests or observations. Failed publication is not success.
type SecretsBackend interface {
	Publish(ctx context.Context, scope Scope, write StoreWrite) error
	Acquire(ctx context.Context, scope Scope, requirements Requirements) (map[string]string, error)
	Release(ctx context.Context, scope Scope) error
}

// Dependencies are explicitly supplied to native messaging implementations.
// Ready is called only once the worker is listening for requests.
type Dependencies struct {
	Orchestrator OrchestratorBackend
	Secrets      SecretsBackend
	Ready        func() error
	// WrapMessaging injects concrete discovery without widening backend contracts.
	WrapMessaging func(context.Context, Messaging) (Messaging, error)
}

// ResourceFact describes readiness without a connection address or secret.
type ResourceFact struct {
	Scope
	Name, Kind, Consumer string
}
