# Issue 284: Show services as properties of their owning scopes in the workflow graph

## Problem

Scope-owned services are part of a scope's declared shape, but representing a
`Service` call as an ordinary command step loses that ownership relationship.
The workflow graph should let a reader see which services a scope owns before
the run starts.

This follows the scope-owned service work in #276. It is graph representation
only; it should not change service launch, readiness, failure, or shutdown
behavior.

## Desired result

- A scope's graph representation has its declared services as a property of
  that scope.
- Each service retains its constant call-site name and source location.
- Services are visually and structurally distinguishable from blocking
  `RunCommand` and `Check` operations.
- Nested and iteration scopes show ownership at the scope where `Service` is
  declared.
- Generated graph serialization and the web graph render the same ownership
  relationship.

## Boundaries

- Do not model runtime process instances, PIDs, readiness, restarts, or process
  trees in the static graph.
- Do not change the scope-owned service runtime contract from #276.
- Do not add a general dependency graph or orchestration model.

## Definition of done

A generated workflow containing services at the root, in a named child scope,
and inside an iteration produces graph data that places each service on its
declaring scope. The web graph visibly presents those services as scope-owned,
while ordinary commands remain ordered operations. Static extraction and UI
tests cover the distinction.
