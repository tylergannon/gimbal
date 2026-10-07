# Issue 430 diagnosis

decision: Diagnose against fetched origin/main at 4377acbc7581bc8a20551eb0dde7f7276ba1fc61. This task is diagnosis and a half-page plan, not implementation.

friction: Codex readTurn returns on lost transport, RunTurn clears active ownership, and implement exits before independent validation. Reconnection resubscribes sessions but discards returned turn history; it does not reconcile the interrupted operation. Lost turn/start acknowledgement is also ambiguous and must not cause blind replay.

decision: Installed codex-cli 0.160.0's generated protocol exposes thread status, turn status and items, and rejoins a running thread by threadId. These shapes support a reconciliation design but do not establish live disconnect/restart behavior. Verify active, completed, and interrupted cases with an isolated daemon before choosing continuation behavior.

decision: Keep bounded automatic recovery inside the Codex adapter so the existing Generate and workflow stack remain intact. If reconciliation cannot safely continue, retain same-run task state and an explicit operator resume path. Limit continuation work to the required existing workflow; do not introduce arbitrary Go workflow replay.

friction: Close deletes ownership before archive, and the current dead-connection test requires that deletion. Retained cancellation and retryable cleanup need coordinated changes with issue 422, including replacement of that test's incorrect expectation. Shared-daemon restart and unrelated-thread control remain outside this repair.

decision: The existing Claude unknown-tool retry sends a new follow-up; broadening it to Codex EOF would permit duplicate execution. Recover native state first. Prove preservation of tool effects, no concurrent replacement, one task validation, and no replay of accepted outcomes across active-turn loss, lost completion, durable-history restart, persistent outage, and cancellation.
