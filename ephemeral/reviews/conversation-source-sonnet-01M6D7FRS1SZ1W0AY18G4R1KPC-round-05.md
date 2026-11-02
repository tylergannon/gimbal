# Adversarial review: conversation source stage, persistent loop round 05 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain** (one issue with a reproduction; no critical defect found)

## Target

- The working tree in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`) on top of commit `fb7009b2` ("feat: route named native work and conversations through River"), plus 18 uncommitted modified files and the untracked `cmd/gimbal/messaging_worker_lease_test.go`.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, my rounds 01–04, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary and did not narrow scope or predict a verdict. The cross-machine/native/browser proof (C1 live, D6–D10) has not occurred and is not claimed.

## Boundary note

I did not run `go generate` in the product tree. The scratch test and the baseline run below were done in copies under my scratchpad (a `git archive` of `f2ef93b1` and an `rsync` of the current tree). The product worktree is unchanged by me (`git diff --stat`: 18 files, 424 insertions, 43 deletions, as before). The only file I wrote there is this artifact.

## Changes since round 04

- **Round 04's finding closed.**
  - `cmd/gimbal/messaging_worker_lease_test.go` builds the real CLI and River plugin, starts a worker, terminates its database authority so another session holds the identity lock, and shows a second `message-worker` over the same state directory is rejected with `executor state directory already owned`. It then shows a third worker starts after the first exits.
  - `TestPostgresWorkerStopsAdmissionWhenAuthorityRecoveryExhausts` drives `serve` through loss and exhaustion.
  - `TestNativeNamedSessionControlsStayAtRetainedOwner` runs steer, interrupt and identified-close at the original worker after a rebind, with a real native session.
  - The conversation native test now also attaches a file (`FileID`), whose bytes are sent as a `TaskFile` on the named path instead of a physical path.
- **Event ordering**: `MessageEvent` gained `Source` and `Sequence`. The daemon stamps them and persists the sequence in the disposition. Both the River and filesystem replay order by source then sequence, not by clock (`TestPostgresReplayOrdersBySourceSequenceNotClock`, `TestReplayOrdersOneSourceBySequence`).
- **Interactive input files** (`interactive.go`, `runstate`) carry task files through durable input, with conflict detection.
- **Worker registration vs bindings**: a new `gimbal_message_bindings` table records which worker each logical name is bound to. `registerWorker` refuses a worker whose configured name is already bound to another worker, and the publisher updates the binding when it records a new owner.

## Evidence inspected

- Diffs of `plugins/messaging-river/{main,named}.go` and tests, `backend/messaging.go`, `backend/messagefiles`, `internal/taskdaemon`, `internal/conversation/requests.go`, `interactive.go`, `internal/runstate`, `environment_messaging.go`, and the lease test.
- Checks I ran personally in the product worktree (the DSN is the private disposable-Postgres file):
  - `go build ./...`: only the known `main`-less c-shared plugin link errors. `go vet` over root, `backend/...`, the River plugin, `./internal/...` and `./cmd/...`: clean.
  - `go test -race -count=1 . ./backend/... ./workflow/... ./plugins/messaging-river ./internal/... ./cmd/...` with `GIMBAL_RIVER_TEST_DSN_FILE` set: every package `ok`, including the new lease test.
  - With `GIMBAL_MESSAGING_NATIVE_TEST=1` and real Codex (`gpt-6-luna`), the whole River package: all 18 Postgres tests pass. The three named native tests also pass: `TestNativeNamedSessionBytesContinuityAndCleanup`, `TestNativeHostConfiguredLocalNamespaceConversation` (now with an attached file) and `TestNativeNamedSessionControlsStayAtRetainedOwner`.
  - One native test fails: `TestNativeMessagingWorkflowInputsContinuityAndCleanup`, the older shared-queue test, with `service exited before readiness`. I reproduced the same failure on a pristine `git archive` of `f2ef93b1` (`native_test.go:64`), so it predates this work and is not a regression. It is hidden in ordinary runs because it is opt-in.
  - I did not rerun the web checks or generation. Round 04 showed generation deterministic with `vp fmt`, and this round changed no web or generated files.
  - Scratch test in a copy: see finding 1.

## Findings

### 1. Issue: moving a logical name to a fresh worker identity cannot be done, and the error names an operation that does not exist

Requirement: R3 ("A distinct worker identity cannot claim an already-bound logical name without an explicit configuration rebind; rebinding preserves old session owners"), D3 ("Rebind a logical name: new work selects the new owner and existing sessions still address the original owner"), and the continuation's "stable native owner addressing ... loss/reconnect/restart behavior".

Evidence (`plugins/messaging-river/named.go`):
- `registerWorker` inserts and then locks the `gimbal_message_bindings` row for each name the worker's configuration maps to itself. If another worker is bound, it fails with `river environment "east" is already bound to worker "w1"; explicitly rebind it first`.
- The only code that updates a binding is `publishNamed`, which upserts it inside the publishing transaction when it records a new owner row. That transaction then checks that the target worker holds its authority lock. If the new worker is not registered, the publish is rejected and the transaction, including the binding update, rolls back.
- So a fresh worker `w2` cannot register for `east` because `east` is bound to `w1`, and a publisher cannot rebind `east` to `w2` because `w2` is not registered.
- Reproduced (scratch test in a copy of the current tree, 0.6s):
  1. Start `w1` with `{east: w1}` and publish an `identified-session` on `east`.
  2. Switch the configuration to `east -> w2` and start `w2` with `{east: w2}`: `w2 failed to start: river environment "east" is already bound to worker "w1"; explicitly rebind it first`.
  3. Publish to `east` with `w2` down: `river worker "w2" is not registered`.
- `TestPostgresNamedWorkersReceiveOnlyAddressedRequests` passes its rebind step only because the new owner (`worker-b`) was already registered for another name, so the order the stage needs is never exercised.
- Only an obscure order works: start `w2` with a configuration that does not yet list `east`, publish with `east -> w2` to rewrite the binding, then restart `w2` with the final configuration.

Impact: the ordinary case of moving a name to a new machine or worker identity fails at both ends, and the message tells the operator to do something the system offers no way to do. It is not fail-open, since a duplicate or squatting worker is correctly refused, but D3's rebind claim holds only for a pre-registered target. Existing session owners are preserved. The fix is either an explicit rebind step (for example, a binding change that does not require the target to be registered first) or letting the registering worker take over a name that the current configuration maps to it, plus a test with a fresh worker.

### Nitpicks

- `TestNativeMessagingWorkflowInputsContinuityAndCleanup` (shared-queue) fails under `GIMBAL_MESSAGING_NATIVE_TEST=1` both before and after this work. It is not caused by the named path, but a green opt-in suite would let that flag mean something.
- `CleanupScope` groups retained owners by worker with `min(environment)` (cosmetic; cleanup is routed by worker). `Manager.Get` writes while holding a read lock, matching a pre-existing pattern.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation | Done and tested |
| Addressed delivery by logical name; no orchestrator/endpoint | Done; Postgres-tested; `Submit` rejection tested |
| Retained-owner addressing; turns, steer, interrupt, close after rebind | Done; real native session test passes |
| Exclusive registration, duplicate rejection, state-dir lease | Done; Postgres test and CLI lease test pass |
| Authority loss, reacquire, exhaustion exits and refuses admission | Done and tested through `serve` |
| Startup recovery and republish with original event ID and sequence | Done and tested |
| Replay by source sequence, live delivery, bounded reconnect | Done and tested |
| Scope cleanup to every retained owner | Done and tested |
| Bytes and complete-context value over the boundary | Done (existing byte fields); native tests pass on one machine |
| Placement facts, projection, run page | Done |
| Conversation consumer, configured-local launch-to-finish, attached file | Done; native host test passes |
| Rebinding a name to a fresh worker identity | **Not workable** (finding 1) |
| Environment inventory guard | Boolean plus audit and fixtures |
| Race tests, frontend checks, generation | Pass (web and generation last verified in round 04; no web or generated files changed since) |

## What is sound

- All of round 04's gap is closed with real tests, including a CLI-level lease test against the database.
- Retained owners, acceptance fencing, loss handling and cleanup all behave as designed under `-race` and with a real native session.
- Event replay is deterministic by source and sequence across both stores.

## Remaining defects and blockers

- Defects: finding 1, a rebind deadlock for a fresh worker identity, with a short reproduction.
- External blocker: none. Postgres and authenticated Codex were available.
- The later C1/D6–D10 cross-machine, Compose and browser proof has not occurred and is not claimed.

Outcome: **material findings remain** (finding 1). Status is not ready.
