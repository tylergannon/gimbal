# Resource-lifetime validation, round 01

Reviewer: Sonnet 5.5 (independent). Read-only except this artifact. No live tests, provider turns or service operations were run.

## Target

Worktree `/Users/tyler/.codex/worktrees/run-inventory-tests/gimbal`, branch `codex/codex-resource-lifetime`, HEAD 97034284 plus uncommitted changes:

- `codex/daemon_live_test.go` (modified): `liveLuna` alias resolution, socket-based `managedDaemonPID`, new `TestSharedSessionsReleaseLive` and `TestTerminalConnectionFailureLive`.
- `internal/host/resource_lifetime_live_test.go` (new): `TestNativeGroupSessionsLive`.
- `ephemeral/plans/resource-lifetime-acceptance.md`: status line changed to approved.

Reviewed against `ephemeral/plans/resource-lifetime-acceptance.md` (definition of done, claims 1-5), the repository AGENTS.md, and the code in `codex/codex.go`, `codex/rpc.go`, `internal/agentexecutor/agent.go`, `internal/host/host.go`, `internal/binding/binding.go`.

## Caller constraints

The launch prompt's read-only boundary, artifact path and no-live-run rule are valid operating constraints and were followed. It did not narrow scope. The review covers the whole target.

## Evidence inspected

- Full `git diff` and the new file.
- `releaseSession`, `use()`, `connection.finishRead`, `read()`, `daemonStatus`, `current()`.
- `agentexecutor.init` and `Submit`. The host test binary re-executes itself as `--gimbal-agent-worker` because `host` imports `agentexecutor`, so `TestNativeGroupSessionsLive` does use real worker processes.
- No run evidence exists yet: no run transcript, resource inventory or baseline/delta record appears in the repository, the chat context given to me, or `ephemeral/reviews/`. Claims 1, 2, 3 and 5 depend on the CLI observation sequence, which this round cannot assess. They are unproven in the available record, not failed.

## Findings

### 1. Issue: the private-daemon vehicle and concurrent cold-start case are absent (incomplete requirement)

DoD claim 2 requires "one two-worker concurrent cold start against a verified private daemon", and the plan says sequential-only observations cannot satisfy the full claim. Claim 4's first-choice vehicle is a private daemon with an in-test guard: GIMBAL_LIVE plus an explicit opt-in naming the test-owned home, and a check that fails closed if the resolved socket equals the default home's socket or ownership is unestablished.

The diff contains none of this: no CODEX_HOME isolation, no opt-in variable, no isolation guard, no cold-start test. The only terminal-failure test uses the second-choice vehicle, a `CloseNow` on the adapter's own connection to the shared daemon. The plan permits that only if the private daemon is determined unavailable. Nothing in the target records that determination (no check of whether CODEX_HOME isolates the daemon endpoint, no auth-configuration finding). Until either the private vehicle exists or its unavailability is documented, claim 4's real-failure part and the cold-start part of claim 2 are unmet.

### 2. Issue: `TestNativeGroupSessionsLive` passes even if resources leak (incomplete requirement, claims 2 and 5)

`internal/host/resource_lifetime_live_test.go:15-52` asserts only that both workers created their `crossed-N` files. That proves the two tool commands overlapped. That is a good concurrency proof, and it is the one thing the test establishes. It does not observe worker exit and reap, tool-child exit, the daemon's loaded-thread set or client count, or any process, FD or socket delta before and after `owner.Close()`. Claim 2 requires counting indirect resources, and claim 5 requires that workers and tool children "exit and are reaped after their owner lifetimes". A repaired build and an unrepaired build would both pass this test, so by itself it supports no stabilization statement. Whatever external inventory accompanies it must carry that weight, and the report must say so.

Secondary: each worker's shell loop runs up to 30 s waiting for its peer. If the model declines to run the command, or runs it in a sandbox that cannot write to `project.Path()`, the failure surfaces as a missing `crossed-N` stat error with no indication of which cause applies.

### 3. Issue: `TestTerminalConnectionFailureLive` cannot distinguish "no blind replay" from "replay impossible"

Claim 4 requires that the real failure "does not blindly replay an accepted assignment". The test installs `ad.dial` returning an error for every redial (`codex/daemon_live_test.go:805`) before the turn starts. After `CloseNow`, no replay can occur by construction, so `effect == "EFFECT\n"` at line 850 holds regardless of whether the recovery code would have replayed the accepted turn. The assertion is vacuous for the replay claim. The test does show an honest error, failed release with a reported error, and zero sessions and routes, which are valid. The replay half needs either a redial that succeeds (as `TestActiveTurnReconnectPreservesToolEffectLive` does) or a different statement in the report. The plan already warns that a recovered success alone does not satisfy the terminal-failure claim, so neither existing test covers "terminal failure and no replay" together.

Related robustness issue: `<-conn.readerDone` (lines 765 and 839) is unbounded. If a release path fails to terminate the reader, which is precisely the defect being tested, the test hangs until the global `go test` timeout instead of failing with a diagnostic. Use a select with `ctx.Done()`.

### 4. Nitpick: stale hard-coded model strings after the alias change

AGENTS.md says to use unversioned aliases. `liveLuna` now resolves `luna`, but line 639 (skip message), line 705 (log text), and the comments at lines 30 and 221 still say `gpt-5.6-luna`. The log line asserts a model name the run may not have used. `liveLuna` also logs on every call, so the active-reconnect test logs the resolution four times.

### 5. Nitpick: `managedDaemonPID` depends on `lsof` ownership semantics

`lsof -t -- <socket path>` is used to identify the daemon. It fails closed on zero or multiple owners, which fits the plan. Whether macOS `lsof` reports only the listener for a unix socket path, and not connected clients, is unverified in the target. The live test that uses it will tell you, but a failure there would look like a daemon problem. It is also unavailable on Windows. It is acceptable for opt-in live tests, but it should be validated once with a second connected client present.

## Other observations (not findings)

- `git status` briefly showed unrelated modifications to `web/src/lib/observation/types.ts` and `web/src/lib/workflow-design/generated/index.ts` (formatting-only rewrites of generated files) midway through this review. They were gone on a later check. Something else touched the worktree concurrently. Confirm nothing from that gets committed with this work.
- `TestSharedSessionsReleaseLive` is a sound adapter-level check: two sessions on one connection, release of one leaves the connection and the other usable, last release clears the connection, reader and routes. It does not include a separate client sharing the daemon, which the plan covers through other cases.
- The plan's status edit is documentation of approval, not behavior. It is consistent with the caller statement.

## Outcome

material findings remain
