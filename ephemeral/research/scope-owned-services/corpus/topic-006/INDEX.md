# Scope completion and cancellation ordering

## Goal-relevant synthesis

Structured-concurrency systems make child lifetime subordinate to the owning
scope. Kotlin says a parent waits for its children before finishing; parent
failure or cancellation recursively cancels them, and `coroutineScope` waits
for both the block and all children ([source](sources/topic006-kotlin-coroutines-basics.md):242-267).
Python `TaskGroup` likewise waits on context exit; after the first real child
failure it cancels siblings, waits for them, and only then raises grouped
failures ([source](sources/topic006-python-asyncio-task.rst):400-467).

Kotlin makes the important race distinction explicit: cancelling a child does
not cancel its parent, but a non-cancellation child exception cancels the
parent; the original exception is handled only after all children terminate
([source](sources/topic006-kotlin-exception-handling.md):130-190). Its
`supervisorScope` changes sibling-failure propagation, not the wait-for-child
completion rule ([source](sources/topic006-kotlin-exception-handling.md):407-415).
The detailed ordering notes are in
[completion-and-cancellation-order.md](clips/completion-and-cancellation-order.md).

## Ordering rules to carry into the service contract

1. **Normal scope completion:** stop admitting new work, initiate service
   shutdown, wait for the service and its owned cleanup to finish, then make
   the scope's successful completion observable. A scope must not return while
   its service is still alive.
2. **Unexpected service exit:** record the exit as soon as observed, initiate
   owning-scope cancellation/failure, let sibling work unwind, wait for all
   owned cleanup, then publish the failure. This mirrors fail-fast groups and
   avoids a silent disappearance.
3. **Explicit cancellation:** cancellation propagates to the service and
   sibling work; shutdown observation precedes the final cancelled outcome.
   An independently observed service failure may remain the primary cause only
   if it won the race before cancellation was established; the sources do not
   define Gimbal's exact precedence policy.
4. **Final status:** status is final only after the child/service wait has
   completed. “Shutdown initiated” is an intermediate fact, not scope
   completion.

These rules are lifecycle semantics, not a readiness mechanism: readiness
remains ordinary workflow work as issue 276 requires.

## Evidence boundary / unresolved

The concurrency sources do not specify OS signal delivery, process-group
membership, descendant cleanup, or a bounded SIGTERM/escalation timeout. The
process-lifetime topics must settle those mechanics. They also do not settle
the exact cancellation-versus-exit precedence or Gimbal status labels. No
stronger claim is justified from this evidence.

## Downloaded primary sources

- [Kotlin coroutine basics documentation source](sources/topic006-kotlin-coroutines-basics.md)
  — official Kotlin coroutines documentation source, retrieved 2026-09-18;
  upstream:
  https://raw.githubusercontent.com/Kotlin/kotlinx.coroutines/master/docs/topics/coroutines-basics.md
- [Kotlin exception and supervision documentation source](sources/topic006-kotlin-exception-handling.md)
  — official Kotlin coroutines documentation source, retrieved 2026-09-18;
  upstream:
  https://raw.githubusercontent.com/Kotlin/kotlinx.coroutines/master/docs/topics/exception-handling.md
- [CPython asyncio task documentation source](sources/topic006-python-asyncio-task.rst)
  — official CPython documentation source, retrieved 2026-09-18; upstream:
  https://raw.githubusercontent.com/python/cpython/main/Doc/library/asyncio-task.rst
