# Issue 611

decision: Native adapters remain responsible for harness semantics. Independent command and agent processes own acceptance, execution, file retention, and controls; caller observation does not own their lifetimes.
decision: Operation locks establish live ownership. A missing socket cannot establish native loss. Retained terminal operations deduplicate for seven days; session continuation state is not vacuumed.

correction: Tyler requires unversioned model aliases in AGENTS.md (luna, haiku, flash), not hard-coded model versions.

correction: Tyler requires the Gemini (agy) and OpenCode wrappers to move through the independent executor too; the three harnesses named in issue 611 are not the complete migration scope.

correction: Tyler subsequently excluded OpenCode from this migration and prioritized Gemini. OpenCode keeps its existing path.

decision: Explicit session close retains each harness's existing cleanup, while turn completion detaches for continuation. Pi forks require separately owned history roots so closing a parent cannot delete a child's history.
friction: A two-second socket deadline was shorter than Pi's native steering receipt, which arrives at the next model step -> controls carry their caller deadline; change-notification waits remain short.
decision: The old task-host acknowledgement API is removed. Retiring a bridge projection never deletes executor-owned files; the executor vacuum controls their retention.
decision: Session controls tolerate the completed turn's directory being vacuumed. Ownership checks never create operation directories; completing a turn clears the session's live pointer.
decision: A native adapter reporting an unconfirmed stop does not produce a terminal cancellation. Its executor retains ownership and retries native Close, never the model assignment, until cleanup confirms cessation.
decision: The twenty-minute default belongs to executor CLI flags. Workflow submissions inherit their context deadline without an added default, and the task-host protocol forwards declared deadlines.
friction: Task-daemon package tests wrote command records into the real home directory -> daemon construction now takes its executor root, and tests use disposable roots. Diagnostics belong to operation directories and vacuum removes submission records with their operation.
decision: Session closure has its own retained receipt, independent of the close turn's retention. Repeated CLI close reads that receipt rather than describing a vacuumed operation.
correction: Tyler asked to stop spending time on the speculative cancellation edge and fix concrete problems. Further work stays on the observed close-receipt, deadline, and test-isolation defects.
friction: PR checks and vet passed but tagged qualification caught unchecked cleanup returns plus a local pending-state result overwritten during acknowledgement removal. Run the actual release lint gate before merging; preserve the pending result and cover that dispatch regression.
