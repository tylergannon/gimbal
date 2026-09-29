# Hand-written instrumented workflow

- User timebox: 24 minutes, then check in even if incomplete. Implementer owns coding; independent validator reviews only.
- Implementation is a specimen of future compiler output, not a compiler. Keep named Temporal activities, one Gimbal run/event owner, shared workspace, inline context, and existing supervisor code.
- User explicitly excludes effective supervision feedback as a gate; concurrent work owns Jev migration. Do not repair unrelated supervision behavior here.
- Live Pi attempt exposed a provider catalog shape mismatch (capabilities array versus expected map). Preserve that finding for the separate integration work.
- TCP proxy needed to translate same-origin requests to its internal backend origin. Guard all routes, including custom control endpoints, before proxying.
- Cancellation can race a successful activity response before its next heartbeat. A canceled workflow must check its own context after Generate even if that activity returned success.
- Container cleanup ends its live UI listener; completed records remain on the host. The view mode reuses the existing Gimbal UI against a completed workspace without adding an event sink service.

## Rebase onto d224afd6

- Main now removes timed supervision and requires TYPESAFE_API_KEY. Remove obsolete WithInterval calls from both source and instrumented activity; pass the Jev key through Docker's runtime environment and update the runtime-only test credential fixture.
- Main's Pi catalog parser accepts capabilities arrays. A live native Pi turn using diffusion/deepseek-4.1-flash returned PI_CATALOG_OK; the earlier catalog blocker is resolved. A fresh Temporal specimen run completed with the Jev-only runtime and normal cleanup. This does not establish a successful supervisory review/feedback cycle.


## Loop and parallel extension

decision: The next compiler-output specimen uses two fixed iterations, each with two parallel repairs in one container and shared workspace. Separate assigned files are a task constraint; no worktrees, reducer, reconciliation, or rollback are introduced.
decision: Temporal owns the iteration loop and activity futures. Activity code retains Gimbal root/iteration scopes and Group ownership, with group joining and closure before the next iteration. Keep plain Go source beside the handwritten target; no transformer or generic action interpreter yet.
correction: A successful mock cannot prove native supervision. Normal coding runs use real Jev evaluation; the opt-in test controls only the Jev HTTP answer and runs real Claude and Pi harnesses through the actual supervision path. Label the latter controlled routing, not detection quality.
friction: Temporal's test environment resolves canceled activity futures before mocked activity goroutines necessarily exit. Use the real hosted cancellation test and real Temporal/container cancellation to establish lifetime cleanup.
correction: Independent review found cancellation could race iteration teardown against Group.Wait and EndIteration pointer clearing. Iteration teardown now waits for the owner's explicit signal after active operations join; EndIteration participates in that join.
