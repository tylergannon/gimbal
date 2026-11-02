# Resource-lifetime validation, round 02

Reviewer: Sonnet 5.5 (independent). Read-only except this artifact. No live provider or service operations. I ran only local non-live checks: `go build ./codex`, `GOOS=windows go vet ./codex`, and `go test ./codex -run 'StartupLock|StartupRecheck' -count=3`. All passed.

## Target

Branch `codex/codex-resource-lifetime`, clean tree at HEAD be889c1c ("fix: serialize Codex daemon startup across workers"), on top of 97034284. The current work is:

- `codex/startup_lock.go`, `startup_lock_unix.go`, `startup_lock_windows.go`, and the `connect` change in `codex/rpc.go`.
- `codex/startup_lock_test.go` and `codex/startup_recheck_unix_test.go`.
- `codex/daemon_live_test.go` (`liveLuna`, socket-based `managedDaemonPID`, `TestSharedSessionsReleaseLive`, `TestTerminalConnectionFailureLive`).
- `internal/host/resource_lifetime_live_test.go` (`TestNativeGroupSessionsLive`).
- The updated plan `ephemeral/plans/resource-lifetime-acceptance.md` (new claim 6, private cold-start experiment retired) and the worklog.

Reviewed against that plan, AGENTS.md, and the round-01 artifact.

## Caller framing

The statement that the lock closes only the concurrent-start vulnerability and does not prove a leak fixed matches plan claim 6 and the worklog. It narrows nothing, so the review stayed broad. I judged the lock against claim 6 as written, and I did not treat it as evidence for claims 1-5.

## Evidence inspected

- Full diff of be889c1c, the lock implementations, both lock tests, and `connect`.
- Repository search: `startDaemonLocked` is the only `daemon start` call site in non-test code, so every Gimbal start goes through the lock.
- Round-01 findings re-checked against the current code.
- Run evidence: none exists in the repository, the worklog, or `ephemeral/reviews/`. Repository rules forbid committing run output. Any observation record must therefore be in the chat or PR body, and none was available to me. I did not query GitHub.

## Claim 6 (startup lock): satisfied as specified

- The lock is an OS `flock` (Windows `LockFileEx`) on a persistent file beside the selected socket. It is opened with Go's default close-on-exec, so the spawned `codex daemon start` does not inherit it. Process death releases it. A cancelled waiter returns `ctx.Err()` and closes its file. `lockStartup` polls every 10 ms and honours the context.
- `startDaemonLocked` rechecks status under the lock and starts only if still stopped. It then polls readiness under the same lock. It aborts if the resolved endpoint changes, which prevents a waiter from starting a different daemon.
- `TestStartupLockSerializesProcessesAndReleasesAfterExit` uses real subprocesses and shows exclusion, release on kill, and cancelled-waiter exit. `TestStartupRechecksUnderProcessLock` races two real processes through a fake CLI and asserts exactly one `start`.
- These tests drive `startDaemonLocked`, not `connect`. The single call in `connect` is trivial, so this is a nitpick (see below).

## Round-01 findings, current status

1. Private-daemon vehicle and cold-start case: closed by Tyler's explicit authorization and the plan change.
2. Group test cannot detect leaks: unchanged in the code. The plan assigns the resource evidence to the external inventory (see finding 1).
3. Replay assertion vacuous: the plan now says to report what each test establishes (forced redial rejection shows failure and release; successful real redial with an exactly-once effect shows recovery without replay), so the claim is scoped correctly. The unbounded `<-conn.readerDone` waits are fixed with `ctx.Done()` selects.
4. Stale `gpt-5.6-luna` strings: fixed.
5. `lsof` ownership semantics: unchanged (nitpick below).

## Findings

### 1. Issue: no available evidence for the integration claims (incomplete requirement)

The plan's Done condition is "all integration claims on the actual candidate and independent validation of their evidence". Claims 1, 2, 3, 5 and the live parts of 4 are about native behavior: final results, worker and tool-child exit and reap, process, descendant, socket and FD baselines and deltas, loaded-thread counts, and identity-preserving resume. The committed code adds harnesses only.

- `TestNativeGroupSessionsLive` (`internal/host/resource_lifetime_live_test.go:15-52`) asserts only that the two tool commands overlapped. A leaking build and a repaired build both pass it. Its resource significance depends entirely on an external before-and-after inventory.
- No observation record from the CLI sequence (`gimbal run review ...`, cancel, error, warmup and three repetitions, human resume, second shared client) was available to me. Until it is produced and validated, those claims are unproven. They are not shown to fail.
- The startup lock does not substitute. The plan and Tyler both say it closes one race and proves no leak fixed.

Required before "done": a report naming the source SHA, resolved luna model, exact cases and repetitions, per-boundary resource baselines and deltas with owner explanations, and the cancellation observation (or a stated unobtained result after three attempts). The live tests must be run on this SHA and the results reported.

## Nitpicks

- `startupLockPath` runs `MkdirAll(dir, 0700)` and is also called for pure comparison (the recheck and every readiness poll). It creates the vendor's socket directory as a side effect of a path computation, and it leaves a persistent lock file there. Both are intended, but the mkdir belongs only in `lockStartup`.
- No test exercises `connect` taking the lock path (for example two real processes calling `connect(ctx, true)` against the fake CLI). A future refactor could bypass `startDaemonLocked` without a test failing.
- `managedDaemonPID` (`codex/daemon_live_test.go`) depends on `lsof -t` returning only the listener for a unix socket path. This has not been validated with a second connected client present. It fails closed, so a wrong assumption shows up as a confusing test failure rather than a wrong result. It is unavailable on Windows.
- `TestStartupLockSerializesProcessesAndReleasesAfterExit` relies on a fixed 200 ms negative wait. That is acceptable for a negative assertion, but it cannot prove exclusion on its own. The in-process 100 ms `lockStartup` check adds a deterministic second proof, so this is fine.

## Outcome

material findings remain
