# #110: A supervisor's failed look is only logged

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/110. Read beta-implementation-handoff.md and beta-bug-triage.md for final session decisions and coordination notes.

When a supervisor's look turn fails (a harness error, or a result that does not validate), the supervisor logs it and waits for its next tick. Nothing else records it. Once Sprint 2's run log exists, a failed look should appear there as a turn ended with an error.
