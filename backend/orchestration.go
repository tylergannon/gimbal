package backend

import "context"

// Requirements grant whole stores while identifying names needed for admission.
type StoreRequirement struct {
	Store string
	Names []string
}
type Requirements []StoreRequirement

type Environment struct{ ID string }

// OrchestratorBackend establishes a ready messaging daemon and its assignment.
// Release follows native cleanup, includes descendants, and preserves unrelated use.
type OrchestratorBackend interface {
	AcquireEnvironment(context.Context, Owner, string) (Environment, error)
	Release(context.Context, Scope) error
}

// SecretsBackend resolves nearest definitions before store access. Values never
// travel in workflow requests or observations. Failed publication is not success.
type SecretsBackend interface {
	Publish(context.Context, Scope, string, map[string]string) error
	Acquire(context.Context, Scope, Requirements) (map[string]string, error)
	Release(context.Context, Scope) error
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
