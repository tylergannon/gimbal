# Structured-scope excerpt

Sources: [`python-asyncio-taskgroups.md`](../sources/python-asyncio-taskgroups.md) and [`swift-taskgroup.md`](../sources/swift-taskgroup.md).

Python `asyncio.TaskGroup` waits for all children when its context exits. The first non-cancellation child failure cancels the remaining children, prevents new children, and is raised after all children finish. Cancellation is propagated and cleanup must not swallow the cancellation signal.

Swift `TaskGroup` likewise propagates cancellation from the parent task to child tasks and describes child tasks as structured under the parent. This supports the narrow rule that a required foreground service is a child of its owning scope: unexpected service exit fails that scope; scope cancellation or normal close initiates service shutdown and the scope does not claim completion until cleanup has been observed.

These sources support lifetime and ordering semantics, not a restart policy, readiness framework, or public API shape.
