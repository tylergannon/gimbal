# Unexpected exit of a required child

## Goal-relevant synthesis

Two fail-fast structured-concurrency designs agree on the key behavior for a
required foreground service: an unexpected non-cancellation exit is a failure
of the owning scope, sibling work is asked to stop, and the scope reports the
failure only after child cleanup has been observed. Go `errgroup` cancels the
associated context on the first non-nil child error and returns the first error
from `Wait` after all functions return ([source](sources/topic005-go-errgroup.go.txt):43-75).
Python `TaskGroup` cancels remaining tasks on the first non-`CancelledError`,
waits for them, and raises an `ExceptionGroup` after they finish
([source](sources/topic005-python-asyncio-task.rst):400-467).

The competing supervisor semantics are intentionally weaker: Kotlin's
`supervisorScope` allows one child to fail without cancelling siblings, while
the scope still waits for children and cancels them if the scope itself fails
([source](sources/topic005-kotlin-exception-handling.md):339-415). That is a
fit for optional work or independently handled failures, not a required
service whose disappearance must not be silent. Longer detail is in
[failure-and-supervision.md](clips/failure-and-supervision.md).

## Answers for issue 276

- **Fail, cancel, type, or continue?** Treat an unexpected required-service
  exit as owning-scope failure; initiate cancellation of work in that scope;
  wait for owned cleanup; then expose the service's non-success result as the
  scope's failure. A typed aggregate is optional implementation detail, not a
  contract requirement. Continuing silently or using supervisor semantics
  would violate “required.”
- **Best narrow fit.** Use fail-fast structured-scope semantics without
  restarts or external-resource supervision. A normal requested cancellation
  remains cancellation, not an unexpected-exit failure, unless the service
  exits independently before cancellation is in progress.
- **Race/observation rule.** Do not report the scope finished merely because
  shutdown was initiated: the result is observable only after the service
  wait/cleanup observation completes. Preserve the first relevant cause when
  an unexpected exit starts scope shutdown; cleanup failures must not erase
  the primary service failure.

## Evidence boundary / unresolved

These sources do not choose Gimbal's concrete status vocabulary, how to encode
an OS process exit versus a workflow error, or which cause wins when explicit
scope cancellation races with an independently observed service exit. Those
are still contract decisions; the evidence supports fail-fast ownership and
post-cleanup observation, not a public API shape.

## Downloaded primary sources

- [Go x/sync errgroup source](sources/topic005-go-errgroup.go.txt) — official Go
  module source, retrieved 2026-09-18; upstream:
  https://raw.githubusercontent.com/golang/sync/master/errgroup/errgroup.go
- [CPython asyncio task documentation source](sources/topic005-python-asyncio-task.rst)
  — official CPython documentation source, retrieved 2026-09-18; upstream:
  https://raw.githubusercontent.com/python/cpython/main/Doc/library/asyncio-task.rst
- [Kotlin coroutine exception/supervision documentation source](sources/topic005-kotlin-exception-handling.md)
  — official Kotlin coroutines documentation source, retrieved 2026-09-18;
  upstream:
  https://raw.githubusercontent.com/Kotlin/kotlinx.coroutines/master/docs/topics/exception-handling.md
