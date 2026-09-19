# Issue 276 prior-art semantic index

Author entry point for the compact report requested by
[`issue-276.md`](../issue-276.md). The corpus is downloaded local primary
material plus topic syntheses; follow the route matching the question, then
open the linked clip when the compressed synthesis would hide a race, error
case, or portability boundary.

## Route map

| Question | Start here | Longer evidence when needed |
| --- | --- | --- |
| Go start, wait, cancellation, and output | [`topic-001/INDEX.md`](topic-001/INDEX.md) | [`topic-001/clips/go-lifecycle.md`](topic-001/clips/go-lifecycle.md) and [`topic-009/clips/go-lifecycle.md`](topic-009/clips/go-lifecycle.md) |
| What `zsh -c` owns and reports | [`topic-002/INDEX.md`](topic-002/INDEX.md) | [`topic-002/clips/zsh-ownership.md`](topic-002/clips/zsh-ownership.md) |
| SIGTERM, process groups, escalation, descendants | [`topic-003/INDEX.md`](topic-003/INDEX.md) | [`topic-003/clips/termination-evidence.md`](topic-003/clips/termination-evidence.md) |
| macOS/Linux portability boundary | [`topic-004/INDEX.md`](topic-004/INDEX.md) | [`topic-004/clips/portability-evidence.md`](topic-004/clips/portability-evidence.md) and [`topic-010/clips/portability-boundary.md`](topic-010/clips/portability-boundary.md) |
| Required-child failure and sibling cancellation | [`topic-005/INDEX.md`](topic-005/INDEX.md) | [`topic-005/clips/failure-and-supervision.md`](topic-005/clips/failure-and-supervision.md) |
| Normal completion, cancellation, and wait ordering | [`topic-006/INDEX.md`](topic-006/INDEX.md) | [`topic-006/clips/completion-and-cancellation-order.md`](topic-006/clips/completion-and-cancellation-order.md) |
| Bounded foreground-service comparisons | [`topic-007/INDEX.md`](topic-007/INDEX.md) and [`topic-008/INDEX.md`](topic-008/INDEX.md) | Downloaded Playwright and `start-server-and-test` sources linked there |
| Draft contract matrix | [`topic-009/INDEX.md`](topic-009/INDEX.md) | [`topic-009/clips/structured-scope.md`](topic-009/clips/structured-scope.md) |
| Exactly four constraints and one unresolved point | [`topic-010/INDEX.md`](topic-010/INDEX.md) | [`topic-010/clips/unix-termination.md`](topic-010/clips/unix-termination.md) |

## Evidence spine for the document

### 1. Launch is not lifetime or readiness

Go's `Cmd.Start` synchronously reports preparation, lookup, I/O setup, and
spawn failure; success only establishes a process. `Wait` is the runtime
authority: it observes exit, waits for configured I/O copying, reports the
exit result, and releases resources. `CommandContext`'s default cancellation
kills only the tracked process, while `WaitDelay` bounds that process or an
open pipe, not its descendants. A live service therefore needs an independent
status/output observation. See the local official implementation and tests
listed by [`topic-001/INDEX.md`](topic-001/INDEX.md).

`zsh -c` interprets shell code. A normal foreground command is waited for, but
pipelines, asynchronous lists, `exec`, disowning, and delegated tools change
what the shell's PID and status mean. Shell exit is not proof that descendants
are gone. See the official zsh/POSIX sources through
[`topic-002/clips/zsh-ownership.md`](topic-002/clips/zsh-ownership.md).

### 2. The defensible Unix ownership boundary is a deliberately created group

Process groups are signalable collections; ordinary descendants inherit the
group. Linux can target a group with `kill(-pgid, signal)` and macOS exposes
`killpg`. Establish the boundary before exec/reorganization; `setpgid` and
`setsid` have documented timing and process-group-leader failure modes. A
session may contain multiple groups, so “session” alone is not a universal
kill target.

Shutdown evidence supports: send `SIGTERM` to the owned group, wait for a
finite bound, escalate to `SIGKILL` for that same group, then wait/reap the
direct child and preserve its status. Signal-call success is not proof that
every member exited; `ESRCH`, permission failures, or an expired final wait
are cleanup uncertainty/failure. A descendant that calls `setpgid`/`setsid`,
daemonizes, or is managed externally can escape. The full error and signal
semantics are in [`topic-003/clips/termination-evidence.md`](topic-003/clips/termination-evidence.md).

### 3. Structured scopes favor fail-fast required-child semantics

Go `errgroup` and Python `TaskGroup` cancel sibling work on the first real
child failure, wait for cleanup, then report failure. Kotlin supervision is a
useful contrast: it deliberately lets a child fail without cancelling
siblings, so it fits optional work, not a required service. A required service
that exits before intentional shutdown should therefore fail its owning
scope; it must not disappear silently. Normal completion and explicit
cancellation both initiate shutdown and do not become observable until owned
cleanup and final status observation finish. See
[`topic-005/INDEX.md`](topic-005/INDEX.md) and
[`topic-006/clips/completion-and-cancellation-order.md`](topic-006/clips/completion-and-cancellation-order.md).

### 4. Only two bounded workflow tools are needed for comparison

Playwright Test `webServer` and `start-server-and-test` both start a command,
keep it for bounded test work, expose output, and clean up on success/failure.
Playwright provides the stronger comparison: early exit is a startup error and
its graceful path signals a process group, waits, then escalates to a forced
group kill. `start-server-and-test` adds useful inherited output and
unconditional cleanup, but its shown implementation removes unexpected-exit
observation after readiness and has weaker explicit escalation evidence.

Transfer only ownership across the whole operation, failure visibility,
observable output/status, and bounded cleanup. Reject reuse of existing
servers, multi-server ordering, URL/regex readiness, restart behavior,
launchd registration, and Docker/Overmind-specific resource management. Keep
readiness in ordinary workflow checks. See
[`topic-007/INDEX.md`](topic-007/INDEX.md) and
[`topic-008/INDEX.md`](topic-008/INDEX.md).

## Contract synthesis to lift into the report

The topic evidence converges on this single initial contract; it is not a new
API proposal:

- A synchronous start error is visible before the scope proceeds as though the
  required service exists. Successful start permits work but is not readiness.
- Any service exit before intentional shutdown, including exit status zero,
  fails the owning scope and cancels/unwinds its work. Preserve the service's
  exit/signal result and output.
- Normal scope close and scope cancellation use the same bounded shutdown:
  `SIGTERM`, finite wait, then `SIGKILL` for the owned boundary if needed.
  Wait for the direct child and cleanup observation before publishing scope
  completion or cancellation. An exit caused by intentional shutdown is not a
  new unexpected-exit failure; preserve cleanup failure separately.
- Output remains observable during the run and with final status. “Shutdown
  initiated” is not final status.
- The cleanup guarantee is limited to the deliberately owned group/session.
  Escape or inability to prove final cleanup is observable incomplete cleanup,
  not silently successful.

### Four actionable constraints

1. Separate synchronous start from a single, ordered lifetime/status
   observation.
2. Make a finite process-group/session boundary the shutdown target; use
   SIGTERM, bounded escalation, and visible cleanup failure.
3. Treat unexpected required-child exit as scope-fatal, with explicit ordering
   between the first cause, cancellation, shutdown, wait, and final status.
4. Keep readiness in workflow code and keep command output/status visible; add
   no restart, health-check, dependency, persistent-daemon, launchd, or
   external-tool manager.

## Explicit unresolved portability point

Linux and macOS share the process-group model and basic signals, but their
documented group-targeting/error conventions differ (Linux negative-PID
`kill`; macOS `killpg`). Linux also documents subreapers and cgroup-v2
tree-kill facilities that the reviewed current macOS SDK sources do not match.
The unresolved point is whether one implementation-independent Go setup and
verification rule can prove that every descendant is gone when a supplied
shell/tool changes process group/session or daemonizes. Do not strengthen the
contract beyond the owned boundary until real macOS/Linux evidence settles it;
do not use launchd to hide the gap. See
[`topic-004/INDEX.md`](topic-004/INDEX.md).

## Retrieval probes

These probes are the minimum author routes: `start vs wait` → topic 001;
`zsh shell status vs descendants` → 002; `SIGTERM group escalation` → 003;
`macOS Linux portability` → 004; `required child unexpected exit` → 005;
`scope cancellation waits` → 006; `foreground service comparison` → 007/008;
`final contract` → 009/010. Each route reaches a topic index and its longer
clip where needed; no separate benchmark is warranted for this small corpus.

## Housekeeping

Mode: scratch build. Built 2026-09-18 from the ten topic indexes under this
directory; 79 local corpus files are available including this entrypoint (78
prior corpus files), with downloaded primary sources, topic syntheses, and
clips. The index has one entrypoint and nine
task-first routes (the final route is the synthesis pair 009/010); all routes
are local relative links. Known debt is only the explicit macOS/Linux
descendant-cleanup portability point above. Stop here: the document goal does
not require source mirroring, retrieval perfection, or further research.
