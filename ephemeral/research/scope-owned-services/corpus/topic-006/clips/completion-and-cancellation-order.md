# Completion and cancellation ordering detail

The full downloaded sources are authoritative. These are the contract-relevant
ordering rules that are easy to lose in a short synthesis.

1. A structured parent does not complete while its children remain active.
   Kotlin states that parent failure/cancellation recursively cancels children,
   and `coroutineScope` waits for the block and children. See
   [`topic006-kotlin-coroutines-basics.md`](../sources/topic006-kotlin-coroutines-basics.md),
   lines 242-267.
2. Python `TaskGroup` waits for all tasks on context exit. After a child
   failure, it cancels siblings, waits for them, then raises the grouped
   non-cancellation failures. See
   [`topic006-python-asyncio-task.rst`](../sources/topic006-python-asyncio-task.rst),
   lines 400-467.
3. Kotlin distinguishes child cancellation from failure: explicit child
   cancellation does not cancel its parent, while a non-cancellation child
   exception cancels the parent; the original exception is handled only after
   all children terminate. See
   [`topic006-kotlin-exception-handling.md`](../sources/topic006-kotlin-exception-handling.md),
   lines 130-190.
4. `supervisorScope` changes only failure propagation between siblings; it
   still waits for children and cancels them when the scope itself fails. See
   [`topic006-kotlin-exception-handling.md`](../sources/topic006-kotlin-exception-handling.md),
   lines 407-415.
