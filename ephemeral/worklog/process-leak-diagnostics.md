# Process-leak diagnostics

decision: Tyler approved the temporary opt-in process observer and explicitly assigned Terra implementation and Sonnet validation. Build from released main in an isolated managed worktree; the earlier merged checkout remains referenced by Desktop tool processes and is preserved.

constraint: The startup-lock exclusion claim, observed bounded stabilization, and historical process-leak causation are separate. Diagnostics must record uncertainty for shared-daemon ancestry and must not become a cleanup ledger, automatic killer or workflow completion gate.

constraint: Root checkout has unrelated user edits and could not fast-forward safely. Do not stash, reset or edit them. Runtime proof output remains outside the repository; repeatable checks belong in package tests.

implementation correction: Logical owner completion and OS reap are distinct diagnostic events. End observations from existing result publication/service stop boundaries so a still-alive worker can be reported; ending only after Wait can detect descendants but can never identify the root worker surviving completion. Anchor birth identity before the wait goroutine may reap, and fail unknown identity as an observation gap.

friction: A new worktree has no generated frontend manifest. Prepare it with just build before broad Go tests; failures from absent assets do not justify product-code changes.
