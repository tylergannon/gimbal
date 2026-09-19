# Failure and supervision detail

These excerpts preserve the lifecycle distinctions used in `INDEX.md`; the
full downloaded sources remain authoritative.

## Fail-fast groups

- Go `errgroup.WithContext` cancels its derived context on the first non-nil
  child error; `Wait` waits for every function and returns the first error.
  See [`topic005-go-errgroup.go.txt`](../sources/topic005-go-errgroup.go.txt), lines
  43-75.
- Python `asyncio.TaskGroup` cancels the remaining tasks on the first
  non-cancellation exception, waits for them, and raises the resulting
  exception group after all tasks finish. See
  [`topic005-python-asyncio-task.rst`](../sources/topic005-python-asyncio-task.rst),
  lines 400-467.

## Supervision is deliberately different

Kotlin's `supervisorScope` lets a child fail without affecting sibling
children, but still waits for all children and cancels them if the scope
itself fails. This is appropriate only when child failure is acceptable or is
handled by the child. See
[`topic005-kotlin-exception-handling.md`](../sources/topic005-kotlin-exception-handling.md),
lines 339-415 and 461-464.
