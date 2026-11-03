# Adversarial review: conversation source stage, persistent loop round 03 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain**

## Target

- The uncommitted working tree on top of `f2ef93b1` in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`): 34 changed files, plus untracked `plugins/messaging-river/named.go`, `internal/generate/testdata/fixture/environment_audit.go` and notes.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` (including its new work-order paragraph and the parent's correction about `conversation.go` and `TaskRoute`) and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary and did not narrow scope or predict a verdict. The cross-machine/native/browser proof (C1 live, D6–D10) has not occurred and is not claimed.

## Boundary disclosure

I ran `go generate ./...` to test generation determinism. In this worktree it rewrites `web/src/lib/workflow/types.ts` (unformatted, quoted keys), `web/src/lib/observation/types.ts` and `web/src/lib/workflow-design/generated/index.ts`, which is outside the read-only boundary. I restored all three immediately.
- The last two were checked out to `HEAD`, since neither was modified before.
- `workflow/types.ts` was set to `HEAD` plus the four-line `environment_incomplete` field the tree carried before (confirmed by `git diff --stat`: 34 files, 900 insertions, 129 deletions, identical to before my run, `web/src/lib/workflow/types.ts | 4 +`).
- The implementer should confirm that file's four lines (comment block plus `environment_incomplete: boolean;`) match their generator's formatted output.
- Generated output is therefore not shown to be deterministic against the checked-in files. That needs the owning formatter step (`vp fmt`), which I did not run.

## Changes since round 02

This pass is the first to implement the named River path:
- `plugins/messaging-river/named.go` (new) and `main.go`:
  - Per-worker queues derived from namespace and worker identity, so a rebind cannot redirect queued work.
  - Session-level advisory-lock registration (`pg_try_advisory_lock`), with a duplicate refused as "already registered".
  - Publish-time vacancy check using `pg_try_advisory_xact_lock`.
  - Retained owner table `gimbal_message_owners`, with turns and controls resolved to the original worker and environment after rebind.
  - Acceptance table `gimbal_message_acceptances`, committed on the lock-owning connection, with a digest conflict check.
  - Authority watch, reacquire within `ReconnectSeconds`, and exit on exhaustion.
  - `ListenEvents` rewritten as full replay with a bounded reconnect budget.
- `backend/messaging.go`: `Environment`/`WorkerID` on requests and events, and `NamedMessaging`.
- `internal/effects/messaging.go`: `Submit` takes the named branch, rejects an orchestrator or task endpoint, stamps the captured environment, and returns `DeliveryUncertain` for unconfirmed publication.
- `internal/taskdaemon/messaging.go`: `RecoverMessages`, and events carry `Environment`/`WorkerID`. `cmd/gimbal/messaging.go` takes the state-dir lock and runs recovery before ready.

## Evidence inspected

- Full source of `named.go` and the `main.go`, `backend/messaging.go`, `internal/effects/messaging.go`, `internal/taskdaemon/messaging.go` and `cmd/gimbal/messaging.go` diffs.
- The new Postgres tests:
  - `TestPostgresNamedWorkersReceiveOnlyAddressedRequests`: two workers, duplicate rejected, rebind keeps the retained owner.
  - `TestPostgresNamedNativeCommand`: real `taskdaemon` through the effects `MessageCall`.
  - `TestPostgresWorkerAuthorityReacquiresAfterConnectionLoss`
  - `TestPostgresListenerReconnectsAndReplays` and `...ReconnectBudgetExhaustion`
  - `TestPostgresNamedNativeCompletionRecoversAfterRestart`
  - `TestMessagingStartupReconcilesWithoutRedelivery`
- Source of `internal/conversation/{manager,remote,requests}.go`, `internal/host/host.go` and `internal/effects/client.go`, to trace the conversation path, cleanup and byte transport.
- Checks I ran personally in the product worktree:
  - `GIMBAL_RIVER_TEST_DSN_FILE=<private dsn file> go test -race -count=1 -v ./plugins/messaging-river`: all 15 pass against the disposable Postgres. The native-workflow test is skipped, as before.
  - `go test -race -count=1` over root, `backend/...`, `workflow/...`, `internal/{effects,taskdaemon,invocation,generate/...,conversation,host,environments}` and `cmd/...`: all `ok`.
  - `go vet` of the River, backend, effects and gimbal-cmd packages: clean.
  - `go build ./cmd/gimbal`: builds. I did not run `just build`, which would rewrite `web/build`.
  - `web/node_modules/.bin/vp run check` (svelte-check): 0 errors, 0 warnings across 5494 files.
  - `go generate ./...`: see the boundary disclosure above.

## Findings

### 1. Critical: the conversation consumer is still not on the namespace path, and the remote launch path now conflicts with preflight

Requirement: the continuation ("the conversation launch/preparation/continued-session consumer"; "preserve configured local environments through a real selected namespace path"; "meaningful successful host launch-to-finish coverage"; "update every conversation launcher"), C1, R4 ("Remove replaced conversation-specific physical-route and recovery paths"), and the prerequisite's required path (`UI selection -> manager -> ... -> selected River request/retained owner`).

Evidence:
- `internal/host`, `internal/conversation`, `internal/runner` and `internal/environments` have no diff. `Manager.New` still sets `route = target.Route` from `service.Resolve` for any non-local environment (`internal/conversation/manager.go:254-270`, `remote.go:71`), and `WorkflowStart` still receives that physical route (`requests.go:76-99`, `host.go:171`).
- That route carries a physical `Route.Environment`. For a workflow that declares environments, `PrepareContext` rejects exactly that ("workflow environment declarations cannot use a physical task destination"). The registered Conversation workflow always declares one (`conversation.go:20`). By reading, a remote conversation launch therefore errors in preflight, and nothing supplies a Route that selects the River namespace.
- I did not reproduce this by running a remote launch. `cmd/gimbal/daemon_native_test.go` was changed so it is no longer remote, so no test covers it.
- The parent's correction is right that `Name/Configuration` alone is only bus selection, and that `conversation.go` already calls `WithEnvironment`. The gap is that no host path supplies a River route, and none of the conversation code (the manager and its remote and request paths) addresses a retained owner through the namespace.
- No test starts a conversation launch-to-finish against a configured namespace environment. No test sends successive turns or controls from the conversation path to a retained owner.

Impact: the transport exists but its stated consumer does not use it. C1's source half (UI selection through the namespace to a retained native session) cannot be demonstrated, and a remote conversation likely cannot launch at all.

### 2. Issue: scope cleanup is explicitly rejected on the named path

Requirement: `river-named-worker-task.md` item 2 ("turns/controls/cleanup reach the original worker after name rebinding"), R6/R7, and D3.

Evidence: `publishNamed` returns `ContractRejected: named scope cleanup across retained owners is not implemented` for `Kind == "cleanup"` (`named.go`). `effects.Cleanup` (`client.go:75-95`) issues exactly that request through `MessageCall` at every scope end when a messaging connection is present, so any scope that ends under a named configuration surfaces a cleanup error. Native dependents are then stopped only if the workflow closed each session itself.

Impact: a required control path is a stub with a hard error, and ordinary scope teardown on the new path fails.

### 3. Issue: no input/output bytes cross the selected boundary

Requirement: the continuation ("same-boundary input/output bytes"), `river-named-worker-task.md` item 6 ("No direct ReadBytes/WriteBytes HTTP shortcut"), R5 and D5.

Evidence: `grep ReadBytes|WriteBytes` shows byte transport only in `backend/http.go` and `internal/taskdaemon/local.go`. `backend/messaging.go` has no byte or artifact field, and the River schema has no byte table. Large complete-value context artifacts therefore have no path to a worker that cannot read the sender's files.

Impact: the explicit-bytes requirement, which R5 makes mandatory for the large-context demonstration, is unimplemented.

### 4. Issue: several mandatory behaviors are untested or tested only against stubs

Requirement: "actual database-backed registration/delivery/replay/reconnect and native-daemon integration tests ... exercise the real selected implementations"; D3, D5a.

Evidence:
- No test for the state-directory lease (`executor state directory already owned` appears only in source).
- No test for `effects.Submit`'s named branch: rejection of an orchestrator or task endpoint, `DeliveryUncertain` on an unconfirmed publish, and the empty-environment rejection.
- No `serve`-level test that exhausting the reacquire budget stops consumption and returns the actionable error, or that lost authority stops a worker admitting while its data pool is healthy. The reacquire test calls `confirm`/`reacquire` directly.
- The rebind and retained-turn test uses a stub handler. Turns, steer and close on a retained session are never driven through the real `taskdaemon` on the named path. Only a process command is.
- No test for a distinct worker identity being refused for an already-bound name.

Impact: D3, D5a and R3's loss behavior are asserted by implementation reading, not demonstrated.

### 5. Issue: admitted placement is not projected, and generation determinism is unestablished

Requirement: the prerequisite (item 4: "surface actual admitted placement in existing observation/UI models"), R10, D5b, and "generation ... inspect actual deterministic output".

Evidence:
- Accepted and completed events now carry `Environment` and `WorkerID`, but `internal/observation`, `events.go` and the run-page components have no diff, so those facts are not recorded as lifecycle placement or displayed.
- `go generate ./...` produces unformatted TypeScript in this worktree; I did not run the formatter step, so I cannot confirm the checked-in generated files match deterministic output.

Impact: the placement facts have no consumer. The generation-clean claim is not demonstrated.

### Nitpick

`serve` starts a new `owner.watch` goroutine after each reacquire. When the loss was signalled by `admit` rather than the watcher, the old watcher is still running, so duplicates accumulate. `confirm` and `watch` also treat a 2s ping timeout as authority loss.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload and pure namespace validation | Done and tested |
| Addressed delivery of new work to a logical name over one connection | **Implemented**, tested against Postgres |
| Retained-owner addressing after rebind | **Implemented**, tested with a stub handler |
| Exclusive registration, duplicate rejection | **Implemented** and tested |
| Authority loss, reacquire, exit on exhaustion | Implemented; reacquire tested directly, exhaustion and admission-halt untested at `serve` level |
| Startup recovery, republish with original event ID | **Implemented** and tested; named-path restart recovery tested against Postgres |
| Replay, live delivery, bounded reconnect | **Implemented** and tested |
| Native process command over the named path | Implemented and tested; sessions, turns and close not driven through the real daemon |
| Scope cleanup to retained owners | **Not implemented** (hard reject) |
| Input/output bytes through the boundary | **Not implemented** |
| Placement in lifecycle facts and the observation projection | Event fields only; **no projection** |
| State-directory lease | Implemented, untested |
| Conversation consumer on the namespace path | **Not implemented** |
| Configured-local launch-to-finish coverage | **Not met** |
| Environment inventory guard | Boolean plus audit and fixtures (carried over) |
| Race tests, frontend check, build | Run; pass. Generation determinism not shown |

## What is sound

- The addressed-delivery design is coherent. Queue names derive from worker identity, owner binding commits in the publish transaction with the River insert, and acceptance is committed on the lock-owning connection with a digest fence.
- Race tests pass for every affected package, including the new Postgres ones.
- `RecoverMessages` reuses the existing disposition store and preserves the original event ID.
- The pure namespace callback rejects an unconfigured name with no connection or credential read.

## Remaining defects and blockers

- Defects: finding 1 is the unmigrated consumer, which is the stage's stated purpose. Findings 2–3 are required transport capabilities that are stubs or absent. Findings 4–5 are verification and projection gaps.
- External blocker: none. The disposable Postgres works and the frontend dependencies are installed.
- The named-worker transport has advanced substantially since round 02. The source-stage outcome is still not met.

Outcome: **material findings remain**. Status is not ready.
