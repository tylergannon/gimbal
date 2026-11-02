# Resource-lifetime validation, round 03

Reviewer: Sonnet 5.5 (independent). Read-only except this artifact. Runtime output is in `/tmp/r3-sonnet/`, outside the repository.

## Target

Source revision **52e292b4d26edaf339225e4038c48ffe3feaaac5** (branch `codex/codex-resource-lifetime`, clean tree). Runtime code is identical to be889c1c: the commits after it change only tests, notes and reviews (`git diff be889c1c HEAD -- codex internal` touches only `*_test.go` files). Reviewed against the updated acceptance plan, AGENTS.md, the round-01 and round-02 artifacts, and the caller's context file `/private/tmp/gimbal-resource-665-be889c1c/observations.md`.

## Caller framing

The caller said the private cold-start experiment was replaced by the startup lock. That matches plan claim 6 and the worklog, and I treated claim 6 as already specified. The review stayed broad.

## Evidence inspected

- Full diff since be889c1c: the `TestTerminalConnectionFailureLive` rewrite, `TestSeparateClientsSurviveReleaseLive`, `TestNativeConversationDetachResumeLive`, and the relative-path fix to `TestNativeGroupSessionsLive`.
- `codex/recovery.go`, to understand the terminal-failure behavior.
- The caller's own-run records under `/private/tmp/gimbal-resource-665-be889c1c/project/.gimbal/runs`. I checked `run.json` for one normal run (01M6D2PKTK…, completed, with a real `cat fixture; printf GIMBAL665_NORMAL_1` shell command in its session JSONL), one cancelled run (01M6D2TC7R…, "cancelled through the control socket"), and one provider-error run (01M6D36WX9…, failed: "gpt-gimbal-665-invalid … not supported").
- Shared-service ownership, checked read-only. Daemon PID 48413 (listener of the selected socket, and parent of code-mode host 48733) and central PID 97934 matched the context. I did not stop or restart either.

## Independent live validation

I built the test binaries from 52e292b4 myself and ran them against the shared daemon, using test-owned threads and unversioned `luna`, which resolved to `gpt-6-luna`. I sampled the daemon (PID 48413) and the central (PID 97934) after each test.

| Test | Result | Note |
|---|---|---|
| `TestSharedSessionsReleaseLive` | PASS 4.70s | |
| `TestSeparateClientsSurviveReleaseLive` | PASS 5.78s | |
| `TestTerminalConnectionFailureLive` | PASS 4.30s | Turn ended with accepted `DeliveryUncertain` ("codex has not confirmed the stop; the native turn may still be running"). Release error "thread/archive … unconfirmed; local scope released". |
| `TestDetachedConversationResumesTheSameNativeThread` | PASS 3.66s | Thread resumed with native context. Daemon socket unchanged. |
| `TestCloseArchivesThreadsWithoutTouchingTheDaemon` | PASS 3.87s | `thread/loaded/list` empty after Close. Both archived threads unloaded. Daemon PID 48413 before and after. |
| `TestNativeGroupSessionsLive` (host) | PASS 6.82s | Two native workers ran concurrently and the peer-barrier assertions held. |

Resource samples from my runs (`lsof` line counts, not the caller's numeric-FD counts):

- Daemon FDs were 75 before. After each adapter test, 5 s settled: 75, 75, 75, 75, 75. Unix sockets stayed at 5.
- During the Group test: 2 workers, daemon FDs 117 then 111, and 10-11 daemon children. After the owner lifetime: 0 workers, 73 FDs, and a single daemon child (the original PID 48733).
- A further 20 s later: daemon 73 FDs, 5 Unix sockets, central 18 FDs (unchanged from the start), 0 workers, no `sleep 8` or barrier processes.
- Non-live `go test ./codex ./internal/host` passed.

Every adapter lifetime returned to the same plateau. The central and the original code-mode host were never perturbed. The in-test descendants (about 10 children during the Group test) were gone after the owner lifetime.

## Claims assessment

- **Claim 1 (normal, cancelled, failed through the candidate).** Observed. Three normal production CLI runs, three accepted cancellations, and three provider-error failures are recorded in the project records. My checks of the run records match the caller's description.
- **Claim 2 (no accumulating resources, two-session concurrent Group).** Observed for this sample. The Group test passes, and the daemon FD, socket and descendant counts returned to baseline in my samples. The caller's repetitions show native FD counts of 56/58/58/57/59 across boundaries. The variance is small and non-monotone, and the settled-handle inventory in the context file explains the baseline. A claim of strict constancy would be too strong.
- **Claim 3 (human session resumes with the same identity; separate client survives).** Observed by `TestDetachedConversationResumesTheSameNativeThread`, `TestSeparateClientsSurviveReleaseLive`, and the caller's production `TestNativeConversationDetachResumeLive`, which I did not rerun.
- **Claim 4 (terminal real failure; honest result; local release).** Observed, with the scoping below.
- **Claim 5 (workers and tool children exit and are reaped).** Observed through the zero-worker and descendant-count results.
- **Claim 6 (startup lock).** Subprocess tests pass (round 02). The lock is not leak evidence, as stated.

## Findings

### 1. Nitpick (reporting accuracy): claim 1 and 5's "identifiable tool child" is evidenced only by the Group test, not by the production CLI runs

The normal CLI runs execute millisecond-lived commands (`cat ... ; printf MARKER`). No child can be sampled alive, so the plan's "child process can be identified" is not met by those runs. The cancelled runs (for example 01M6D2TC7R…) ended with 4 session records and no tool command, so no cancelled run had a tool child to reap. The tool-child exit-and-reap evidence comes from `TestNativeGroupSessionsLive` (30-second barrier loops), my own descendant sampling (10-11 children then 1), and the caller's note about a failed earlier attempt. That is real evidence, but it belongs to the Group case. The report should say so rather than attribute child exit to the CLI normal and cancelled lifetimes.

### 2. Nitpick (reporting accuracy): the "failed" lifetime in the production CLI evidence is an input rejection, not a lost owner

The three failed CLI runs were rejected at turn start for an unsupported model. No connection, thread or tool existed to be released. The caller already calls this a negative input, but claim 1 "failed Gimbal work" and claim 4's release semantics are therefore carried by `TestTerminalConnectionFailureLive` alone, and only for one connection-loss shape on one machine. That test is reliable in my three-way sampling, but its recovery behavior should be reported accurately.

### 3. Nitpick: terminal failure ends only on external cancellation

Per `recovery.go` (lines ~100-250), a lost connection leads to bounded reconnect attempts and then a "recovery paused" state that waits for an operator `resume` or cancel. The turn does not fail by itself. The test therefore cancels after observing the rejected redial, and the resulting `DeliveryUncertain{Accepted:true}` is honest. In production this keeps a worker process alive until the turn deadline, an operator resume, or a cancel. That is existing, documented design and not a regression, but it affects what "released" means for a worker whose daemon connection is permanently lost, and the report should state it. The exactly-once-effect assertion in this test remains vacuous for replay (redial is denied), as round 01 noted. `TestActiveTurnReconnectPreservesToolEffectLive` carries that claim, and the caller reports it passing.

### 4. Nitpick: loaded-thread counts are not recorded for the production CLI runs

The plan wants loaded owned threads observed where possible. Loaded-thread emptiness is shown by `TestCloseArchivesThreadsWithoutTouchingTheDaemon`, but the caller's CLI boundaries record only FD, socket and code-mode host counts. Adding `thread/loaded/list` to the boundary samples would close that gap cheaply.

### 5. Nitpick: carried over from earlier rounds

- `startupLockPath` runs `MkdirAll` on every call, including recheck comparisons.
- No test exercises `connect` taking the lock path.
- `managedDaemonPID` relies on `lsof -t` returning a single listener. It returned exactly PID 48413 in this run, with other clients connected, which supports the assumption for macOS.

## Gaps

- Cold start under the lock was exercised only with a fake CLI. No real daemon cold start was run, per the authorized replacement.
- Strict baseline constancy is not demonstrated beyond the small variance noted above. The causal link to the historical roughly-30-process incident is not made, and no growth recurred in this sample.
- I did not rerun the production CLI sequence, `TestNativeConversationDetachResumeLive`, or `TestActiveTurnReconnectPreservesToolEffectLive`. I checked the caller's records for the first, and I rely on the caller's report for the other two.

## Outcome

only nitpicks remain
