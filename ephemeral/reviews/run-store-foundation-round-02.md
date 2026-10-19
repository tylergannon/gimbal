# Adversarial review: run store foundation, round 02

Date: 2026-10-18. Reviewer: Claude (adversarial-review skill, performed directly, read-only apart from this file).

## Review target

- Worktree `/Users/tyler/.codex/worktrees/run-store-implementation-plan/gimbal`, branch `codex/run-store-foundation` at 00e901f3, clean tree, seven commits over `main` (c5c9b142): a5c4bfb2, 78e02672, 478d46b4, df87da7e, e3f87d2a (ownership and independent readback), 924ea3bf (routes retained through session cleanup), 00e901f3 (transient housekeeping delivery IDs). 148 files changed, +5190/-721.
- Authoritative sources: `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md`, `ephemeral/run-store-session.md`, `AGENTS.md`, `docs/definition-of-done.md`, `justfile` (`vet` and `lint-go` recipes), `.github/workflows/release.yml`, and the round-01 review `ephemeral/reviews/run-store-foundation-round-01.md`.
- Proof under review: `/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/review-evidence.md` (claims proof at 00e901f3: live app on 127.0.0.1:18089, native run 01M5976D32KP481AWM4N64X0JP on Codex `luna`, remote run 01M5976W7GT86CACRFFNQ4Y49K on a Linux/aarch64 Docker executor, UI-cancelled run 01M59771ZKNQ8KTNZ7YHPHQ8RN, byte-identical readback before and after spool removal, full suite, vet, workflow lint, frontend checks, build).
- Caller constraints (read-only, one output file) are valid operating constraints; no narrowing of scope or verdict was present.

## Evidence inspected

- Read in full: `runstore/store.go`, `runstore/directory/directory.go`, `run_recording.go`, `internal/runner/task_cleanup.go`, `internal/runner/recording_test.go`, `internal/taskdaemon/recording_test.go`, `runstore/directory/directory_test.go`, `run_recording_test.go`, `run_store_substitution_test.go`, and the worklog `ephemeral/worklog/run-store-foundation.md`. Read the branch diff of `run.go`, `scope.go`, `session.go`, `command.go`, `service.go`, `group.go`, `interview.go`, `supervise.go`, `remote_execution.go`, `event_persistence.go`, `registry.go`, `internal/observation/*`, `internal/runstate/*`, `internal/host/{host,routes}.go`, `internal/runner/runner.go`, `internal/environments/remote_test.go`, `plugins/messaging-http/main_test.go`.
- Ran at 00e901f3: `go vet ./...` (clean); `go test -count=1 ./...` (77 packages ok, exit 0; log `/private/tmp/claude-501/-Users-tyler--codex-worktrees-run-store-implementation-plan-gimbal/12ee73ad-8bd1-4068-ac79-03305c66cc8a/scratchpad/fulltests.log`); `golangci-lint run ./...` with golangci-lint 2.15.0 (16 issues, exit 1, see finding 1; log `.../scratchpad/lint.log`).
- Grep checks: every `r.event`/`r.eventResult` caller passes a scope key, session ID or turn ID that was registered by `nextNode`/`adopt`/`ensureNode` first, so `nodeByKey` cannot hand the store an empty node for an engine-produced event. No remaining `executor.NewCommandID` outside the standalone `internal/executor/files.go`. `backend.NewExecutionIdentity` remains only in standalone/control producers, in the legacy-format cleanup branch, in the deliberate transient-delivery line `task_cleanup.go:216`, and as fallbacks when node creation already failed. The route configurations that intents now persist carry no secrets in tree: messaging-http stores `URL/Listen/EventsDir`, river stores a DSN file path, HTTP stores the endpoint only.
- Seam assessment, unchanged from round 01 and re-confirmed on the committed tree: the engine writes authoritative state only through `run.records` (the `runstore.Run` interface); `run_store_substitution_test.go` drives the engine with an in-memory Store through retained cleanup; the observation projection is fed after facts; `Project.Run` rejects a context-only store override (`internal/host/host.go:302`). I found no application path for new-format runs that bypasses the store handle.

## Round-01 findings, status

1. `go vet` struct tags: resolved. `internal/observation/owned.go` and `terminated.go` tags are correct; `go vet ./...` passes.
2. Orphan cleanup identity never recorded: resolved. `cleanupStopped` derives the identity from the stored action node (`task_cleanup.go:193`), commits intent and binding in one `Committed` append before any dispatch (`:203-206`), and `TestOrphanCleanupRecordsTheIdentityAdmittedByMessaging` asserts the first admission the bus saw equals the stored binding's `RequestID`, while later sweeps use fresh transient IDs per the design's bulk-cleanup paragraph.
3. Missing repeatable checks: resolved for the named items. Old-daemon rejection now covers a workflow turn (`internal/taskdaemon/recording_test.go:106`), the helper receipt settlement and the release-failure variant (`internal/environments/remote_test.go:217`), and messaging 400/404/405 (`plugins/messaging-http/main_test.go:119`). Output independence is a child-process readback after removing `executor-spool` with a 1 MiB output (`run_recording_test.go`). Reducer readback compares a full snapshot after a failed command and an invalid-answer correction (`run_recording_test.go`).
4. Explicit stop without `runner.lock`: resolved (`task_cleanup.go:128-130`, asserted by `TestOrphanWithoutRunnerLockDoesNotInferDeath`).
5. Uncommitted tree: resolved; the tree is clean at 00e901f3 and the proof names that revision.

## Findings

### 1. Issue: `just lint-go` fails, and the branch adds thirteen of the sixteen reports

- `golangci-lint run ./...` (the `lint-go` recipe that `.github/workflows/release.yml:169` runs during release qualification) reports 15 `errcheck` and 1 `staticcheck` issues. Three predate the branch and are on `main` (`backend/messagefiles/store_test.go:46`, `internal/pi/agent/agent_test.go:221`, `internal/pi/session/session_test.go:40`). The other thirteen were introduced here, four of them in non-test code written for this work: `internal/runner/runner.go:266` and `internal/runner/task_cleanup.go:163` (`defer connection.Close()`), `task_cleanup.go:136` (`defer lock.Close()`), `runstore/directory/directory.go:721` (`defer os.Remove(f.Name())`), plus `directory.go:438` (QF1001, negated conjunction). The remaining eight are unchecked `Close` calls in `run_recording_test.go` and `runstore/directory/directory_test.go`.
- Reproduction: `just lint-go` at 00e901f3 exits 1 (log path above).
- Impact: `just vet` (go vet, builds, `gimbal lint`) passes, which is what the evidence's "workflow lint" refers to, so the merge-time gate is green. The release qualification is not: it already failed on `main` for the three pre-existing test issues, and this branch makes the first-release fix larger and puts unchecked error returns into the new store and cleanup owner. The production cases are all deferred best-effort closes and can be `defer func() { _ = x.Close() }()` as the same files already do elsewhere (`task_cleanup.go:29,40`).

### 2. Nitpick: a node created after its scope ended is latched as a run-wide recording failure

- `run_recording.go:44-62` (`nextNode`) and `scope.go:136-150` (`adopt`) call `recordFailure("create node", "scope ... ended")` when `s.ended` is observed before or after `ensureNode`. `recordFailure` latches `recordErr`, which surfaces as `Complete.RecordingError` and in the error `Run` returns, alongside genuine store write failures.
- Every public entry point already checks `current(ctx)` (`scope.go:104-115`) and returns "scope ended" to the caller, so the only way to reach this branch is the window between that check and the ordinal allocation, or use of a scope value after `Wait`. I could not reach it from a correctly written workflow, and the node is still created in the store on the post-check path (the ID is returned), so no fact is lost.
- Why it matters anyway: the design distinguishes "the run's recording failed" from "the workflow misused a scope"; this path reports the second as the first. A caller error returned to the workflow, or a distinct fact kind, would keep `RecordingError` meaning what the UI says it means.

### 3. Nitpick: acceptance items still without a repeatable check

These are session-listed scenarios that the proof asserts by reading or by a single live run, not by a test; none contradicts the code I read, which is why they are nitpicks and not issues:

- Pausing `Store.Create` between lock-file creation and `flock`, and pausing runner startup between `runner.lock` creation and `flock`, with the sweeper and `Snapshot` racing. `TestIncompleteStoreDoesNotAcquireLivenessOrDispatch` covers the "state dir exists, nothing else" shape only.
- Host restart performs zero `Open` calls over a mix of successful and failed-body released runs. `countedStore` asserts zero opens for one settled run.
- `Project.Run` rejecting a context-only store override (`host.go:302`): no test exercises it; `grep Override --include='*_test.go'` finds nothing relevant.
- Segment roll: the 16 MiB roll in `runstore/directory` is never crossed by any test; the largest written payload is the 2 MiB blob and the 1 MiB output. The roll and the "truncate only the last segment" rule are therefore unproved by tests.
- Parallel completion inversion: Group children allocate their nodes before their goroutines start, which makes ordinals deterministic, but no test completes children out of order and compares the readback.

### 4. Nitpick: session creation is bound as a `turn_` operation

- `actionIdentity` (`run_recording.go:83-95`) only special-cases `cleanup` and `process`, so the session-create action (`session.go:299`) and the fork action carry `ExecutorOperation: "turn_<session-node>"` and `ExecutorSession: "session_<session-node>"`. The daemon validates the prefix against `ActionID` (`backend/identity.go:45`) and accepts it, so nothing fails; the executor just records a create as a turn-shaped operation. Harmless now, misleading when someone reads bindings back by operation prefix.

## What I did not do

- I did not rerun the live application proof (the two Codex `luna` runs, the Docker remote executor, the UI cancel). I checked the evidence's claims against the code paths and the test suite instead. The suite claims in the evidence (full Go suite, vet, build, frontend tests) match what I observed for the Go side; I did not run the frontend checks.

## Outcome

material findings remain: finding 1 only (the release-qualification lint gate). Every round-01 finding is resolved; findings 2 to 4 are nitpicks and unproved-but-consistent acceptance scenarios.
