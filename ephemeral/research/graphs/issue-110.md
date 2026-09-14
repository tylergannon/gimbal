# A supervisor's failed look is only logged

URL: https://github.com/tylergannon/gimble/issues/110
State: open
Updated: 2026-09-13T16:37:47Z

When a supervisor's look turn fails (a harness error, or a result that does not validate), the supervisor logs it and waits for its next tick. Nothing else records it. Once Sprint 2's run log exists, a failed look should appear there as a turn ended with an error.

