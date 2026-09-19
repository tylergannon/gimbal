# Actionable constraints and unresolved questions

Goal: turn the process-tree evidence into constraints the issue-276 maintainer can use to settle the contract. Primary evidence is downloaded in [`SOURCES.md`](sources/SOURCES.md): Linux `setpgid(2)`, `setsid(2)`, and `kill(2)`; Apple `setpgid(2)`, `setsid(2)`, `kill(2)`, and `sigaction(2)`; and Apple launchd guidance. The longer extracts are [`unix-termination.md`](clips/unix-termination.md) and [`portability-boundary.md`](clips/portability-boundary.md).

## Exactly four actionable constraints

1. **Separate start from lifetime observation.** A successful start is only permission for workflow work to continue. The owner must observe the direct child to classify normal exit, unexpected exit, signal termination, and output completion; a start error must be visible synchronously.
2. **Make the owned boundary explicit and finite.** Shutdown must target a dedicated process-group/session boundary where the platform permits it, send `SIGTERM`, wait a finite bound, escalate to `SIGKILL`, and report failure when the final state cannot be observed. Killing only the shell/direct PID is not sufficient descendant cleanup.
3. **Make required-child failure scope-fatal and race-ordered.** An exit before intentional shutdown fails the owning scope; normal completion and cancellation initiate cleanup and wait for it. Record which terminal event won, and preserve the actual process status/output in the result.
4. **Keep readiness in ordinary workflow checks and keep evidence observable.** No readiness protocol, health-check framework, restart loop, dependency graph, launchd registration, persistent daemon, or Docker-specific manager belongs in this first contract. Workflow code may perform the readiness check it needs; the service lifecycle reports command output/status for that check and for failures.

## Portability boundary and unresolved point

Linux and macOS both document `setsid`, process groups, negative-PID group signaling, `SIGTERM`, `SIGKILL`, and permission/nonexistence errors. Linux additionally documents the `setsid` process-group-leader precondition and `kill`'s group-targeting rules; Apple documents corresponding BSD calls, but the downloaded sources do not establish one portable Go recipe for creating the boundary around a zsh-launched workload and proving that the complete descendant set is empty after nested tools change groups or sessions.

**Unresolved:** the exact macOS/Linux implementation-independent guarantee for descendant discovery and post-escalation emptiness remains open. The evidence supports group/session targeting and bounded escalation, but not a universal claim that all descendants are controllable or that a direct `Wait` proves the tree is gone. The initial contract should therefore make cleanup failure/escape observable and leave the platform-specific setup and verification as an implementation-dependent constraint to settle with real tests. Do not use launchd to paper over this: Apple's launchd material describes independently managed jobs, relaunch behavior, configuration, and daemon-specific ownership, which conflicts with scope lifetime.

## Definition-of-done checks for the final report

- This assigned synthesis contributes one recommended contract, not a fourth competing approach. The only comparison material here is the structured-scope failure/cancellation pattern and the Unix process-group cleanup boundary; the final document must retain no more than three total approaches.
- The evidence is primarily downloaded primary documentation, with local source files and longer local clips linked above.
- All assigned questions are answered above; the sole explicit unresolved point is the portable Go setup/verification guarantee for descendant cleanup on macOS versus Linux. No additional research is warranted unless that implementation-dependent boundary is chosen for settlement.
