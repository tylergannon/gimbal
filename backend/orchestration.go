package backend

import (
	"context"
	"strings"
)

// ServiceUse names a service and its actual logical executor. The concrete
// dependency is constructed in that producer's process, so it knows its network
// environment without exposing deployment handles to workflow code.
type ServiceUse struct {
	Scope
	Name, Kind, Consumer string
}

// ServiceBinding is a non-secret HTTP address usable from this consumer.
// It must have no credentials, query, or fragment. Acquire's successful return
// establishes the implementation's declared readiness and consumer access,
// not process launch or continued availability.
type ServiceBinding struct{ Name, URL string }

func (b ServiceBinding) EnvironmentName() string {
	return "GIMBAL_SERVICE_" + strings.ToUpper(strings.ReplaceAll(b.Name, "-", "_"))
}

// Orchestration is the service dependency exercised by messaging, not a workflow
// scheduler. Gimbal supplies no infrastructure implementation. Acquire must fail
// if declared readiness/access cannot be established. It owns any partial use
// until Release. Repeated acquisition of the same named service, scope, kind
// and consumer is the same use, not an accumulating reference count.
// Release relinquishes only this scope and descendants in this
// run, preserves unrelated/shared use, and is idempotent. A failed release stays
// retryable through existing scope cleanup. It never releases native tasks.
// Implementations declare their availability/loss guarantees; this interface
// does not promise continuous health, failover, replay or infrastructure recovery.
// Objects are local to a process. Connection close never releases service use.
type Orchestration interface {
	Acquire(context.Context, ServiceUse) (ServiceBinding, error)
	Release(context.Context, Scope) error
}
