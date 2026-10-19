# Cancellation simplification

User-approved scope, 2026-10-19. Implementation checkpoint: `fe043bdc` on
`codex/run-store-foundation`. Work continues on `codex/simplify-cancellation`.
The checkpoint and Git history preserve the existing implementation for recovery.

## Desired behavior

Normal scope completion collects the final results of its agent turns and
commands. Results remain saved and visible. Scope exit removes contextual data
that must no longer be available. Preserve the backend-neutral recording seam
and exclusive ownership while a writer is open.

Cancellation requests cancellation of owned work. The central server submits
the request through the existing transport; transport owns delivery behavior;
executing nodes respond to cancellation; local resource owners eventually
release their resources. Do not introduce a new messaging subsystem in this task.

The core workflow engine does not maintain a cancellation ledger, cleanup
receipts, a retry-cleanup lifecycle, or terminal controllers retained to confirm
remote release. Storage failure must not prevent stop requests or local resource
release. Return/report operational failures plainly. An unreachable external
operation can require manual intervention.

Final output arriving after cancellation/shutdown may be lost. Where feasible,
unfinished output remains identifiable as incomplete rather than successful
empty output. Exact UI delivery cutoffs, immediate closure, ideal shutdown
sequencing, and complete post-shutdown history are not acceptance obligations.
Do not replace retained cleanup with a new framework proving eventual closure.
Ordinary synchronization must prevent use of closed resources and handle
sessions created concurrently with cancellation.

## Superseded requirements

These direct user instructions supersede contradictory retained-cleanup and
late-output requirements in the earlier run-store design/session, existing
tests, and `docs/definition-of-done.md` (including the sentence requiring
unverified remote cleanup to remain owned and retryable). Do not preserve or
recreate withdrawn behavior merely to satisfy those tests or review reports.

## Implementation and completion

Simplify and delete machinery. Preserve existing normal-operation transport and
adapter behavior that is unrelated to central cleanup supervision. Remove stale
retry-cleanup UI/control surfaces if necessary; do not redesign the UI.

Use focused checks for normal workflows/turns/commands, saved-result readback,
cancellation with eventual resource release, and operational scope/concurrency
correctness. Reuse existing fixtures and build assets where suitable. Observe
the relevant running path; keep private transcripts and proof output out of Git.
No additional Astra review, consensus loop, speculative completeness campaign,
or unrelated implementation. Stop when this scope works; report real limits.

Record what was removed, any remaining retained-cleanup paths and why they are
needed, checks and observed behavior, and implementation commits. No merge or
release is requested by this handoff.
