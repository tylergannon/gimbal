# Behavioral contract matrix

Goal: give the issue-276 document one narrow, observable contract for a required foreground service. The evidence is local in [`go-exec-source.txt`](sources/go-exec-source.txt), [`go-os-source.txt`](sources/go-os-source.txt), the rendered Go docs ([`go-os-exec.md`](sources/go-os-exec.md), [`go-os-process.md`](sources/go-os-process.md)), and the structured-concurrency references ([`python-asyncio-taskgroups.md`](sources/python-asyncio-taskgroups.md), [`swift-taskgroup.md`](sources/swift-taskgroup.md)). Exact lifecycle excerpts are in [`go-lifecycle.md`](clips/go-lifecycle.md) and [`structured-scope.md`](clips/structured-scope.md).

## Recommended contract

| Event | Owning scope's observable outcome |
| --- | --- |
| Synchronous startup failure | Starting the shell/process fails before the service is considered running. The owning scope observes a startup failure and does not proceed as if the required service existed. `Cmd.Start` is the synchronous boundary; Go documents that context cancellation does not call `Cancel` when `Start` fails. |
| Service runs | Workflow work may continue. A single lifecycle observation reports the eventual process status and output; the scope does not infer liveness from successful start alone. |
| Unexpected exit | Any service exit before the owner begins intentional shutdown, including a zero exit, is a required-child failure. Fail the owning scope and preserve the process exit/signal status and captured output. This follows structured scope practice: a non-cancellation child failure cancels siblings and is surfaced after child cleanup. |
| Normal scope completion | Completion initiates service shutdown, then waits for bounded cleanup and final status observation. The scope must not intentionally outlive its service or report completion while the owned workload remains within the cleanup boundary. A clean intentional exit is not a runtime failure; cleanup failure is observable. |
| Scope cancellation | Cancellation initiates the same shutdown path. The scope returns cancellation as its primary outcome after bounded cleanup; cleanup/termination errors remain observable. If an unexpected exit was observed before shutdown began, report the service failure; an exit caused by intentional shutdown is not reclassified as an unexpected runtime failure. |
| SIGTERM | Shutdown sends `SIGTERM` to the owned workload boundary, not only the shell PID. A cooperative service may exit normally and must still be waited for. |
| Bounded escalation | If the workload has not terminated by the finite shutdown bound, escalate to `SIGKILL` for the same owned boundary, then observe the final direct-child status. The contract reports escalation/cleanup failure; it never silently treats an unobserved descendant as gone. Go's `Process.Kill` and `WaitDelay` alone are insufficient because they target/bound the direct process and pipes, not its descendants. |
| Descendants | The guarantee covers descendants that remain in the deliberately owned process-group/session boundary. A nested tool that escapes that boundary or daemonizes is outside Gimbal's resource management, but escape or inability to verify cleanup must be visible as incomplete cleanup, not hidden. |
| Output and status | Standard output/error remain observable during the run and in the final lifecycle result. Final status distinguishes startup error, unexpected exit, intentional shutdown, cancellation, signal, escalation, and cleanup failure. Go's `Wait` waits for configured I/O copying, so output observation and final process observation are one ordered lifecycle. |

## Ordering rule

Record the first terminal cause (startup failure, unexpected exit, normal close, or cancellation). Once normal close/cancellation begins, classify the service exit as shutdown work, not as a new required-child failure, while still preserving its actual status. The scope's terminal result is emitted only after `Wait` and bounded cleanup have completed. This is the smallest rule that prevents a required child from disappearing silently and avoids a restart policy.

## Questions addressed and limit

- The matrix answers both assigned questions: it gives one contract for every requested event and names the owning scope's observable outcome for each.
- The Go evidence establishes the synchronous/asynchronous split: `Start` reports launch failure; `Wait` reports exit, I/O, context, and resource-release outcomes. Structured-concurrency evidence supplies the fail-owner/cancel-siblings and wait-for-children ordering.
- This index deliberately adds no API name/signature, implementation plan, readiness framework, restart behavior, dependency graph, or external-resource manager.
- Portability remains unresolved at the process-group setup/verification seam; see [`portability-boundary.md`](../topic-010/clips/portability-boundary.md). The contract therefore requires bounded, observable cleanup without claiming that every tool that escapes its group can be controlled.
