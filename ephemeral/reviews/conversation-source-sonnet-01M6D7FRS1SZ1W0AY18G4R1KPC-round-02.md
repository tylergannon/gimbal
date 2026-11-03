# Adversarial review: conversation source stage, persistent loop round 02 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain**

## Target

- The uncommitted working tree on top of `f2ef93b1` in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`): 32 modified files plus untracked notes and one new fixture.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, the earlier reviews (`conversation-declarations-sonnet-round-01.md`, `conversation-source-sonnet-round-01.md`, and my round 01), and the repository `AGENTS.md`.
- Only this artifact was written. The caller gave the target, artifact path and read-only boundary; nothing narrowed scope or predicted a verdict. The cross-machine/native/browser proof (C1 live, D6–D10) has not occurred and is not claimed.

## Changes since round 01

- `internal/taskdaemon/messaging.go`: new `Daemon.RecoverMessages`, which replays retained admissions through the existing handler at startup. It is called from `cmd/gimbal/messaging.go:84`. A test, `TestMessagingStartupReconcilesWithoutRedelivery`, shows a retained completion is republished once, with its original event ID, without queue redelivery.
- `internal/generate/read.go`: new `auditEnvironments`, which walks reachable local bodies even when the shape walker skips their syntax. New fixture `internal/generate/testdata/fixture/environment_audit.go` covers a selector hidden behind an `else if` condition, and a function-value call.
- Everything else is unchanged from round 01.

## Evidence inspected

- `git diff` and `git status` of the whole tree. `git diff --stat` for `backend/`, `messaging_registration.go`, `internal/effects`, `internal/host`, `internal/conversation` and `internal/runner` is empty.
- `plugins/messaging-river/main.go`:
  - Lines 290 and 347 still return `river named-worker delivery is not yet implemented` whenever `Environments` is non-empty.
  - The only advisory lock is the schema-migration lock. `WorkerID` and `ReconnectSeconds` are parsed but never used.
- `internal/host/host.go:160-175`: the conversation input now carries `"environment"`, but `WorkflowStart` and `WorkflowPrepare` still pass the physical `TaskRoute: route`. `internal/conversation/remote.go` still resolves a physical `Target`.
- `internal/workflows/conversation/conversation.go:20` calls `gimbal.WithEnvironment(ctx, params.Environment)`. That landed in the declaration checkpoint and is unchanged.
- Checks I ran personally in the product worktree:
  - `GIMBAL_RIVER_TEST_DSN_FILE=<private dsn file> go test -count=1 ./plugins/messaging-river ./internal/taskdaemon ./internal/invocation ./internal/generate ./cmd/gimbal`: all `ok`.
  - The River tests run against the disposable Postgres, but they cover the old shared-queue path and pure parsing only.
  - I did not repeat `go build ./...`, `go generate`, `-race` or the frontend/application build. Round 01 established that the defects are in the source, and these runs cannot change that. Passing them would not demonstrate the missing outcome.

## Findings

### 1. Critical: the named-worker River delivery path is still not implemented, and configuring it is a hard startup error

Requirement: the continuation ("Complete the actual source outcome rather than returning another inventory: named River workers, stable native owner addressing, exclusive registration and state ownership, loss/reconnect/restart behavior, same-boundary input/output bytes"; "Finish the path"; "requires actual database-backed registration/delivery/replay/reconnect and native-daemon integration tests"), `river-named-worker-task.md` items 1–6, R3, R4, R6, R8, D3 and D5a.

Evidence:
- `plugins/messaging-river/main.go:290` and `:347` fail startup for any configuration that names an environment. A worker or workflow process using the only configuration that can address a logical name cannot start, so the `RecoverMessages` call added in `cmd/gimbal/messaging.go` can never run on the named path.
- There is no registration table or per-worker authority lock. There is no logical-name to worker resolution and no retained session-owner binding. Authority loss does not pause admission and nothing reacquires authority within a budget.
- `backend/messaging.go` still has no logical-destination or retained-owner field on `MessageRequest`. Accepted and completed facts carry no worker identity or logical environment. Effects cannot submit a captured environment. No bytes travel through the selected boundary.

Impact: this is the central source-stage deliverable and it does not exist. The new `RecoverMessages` is a real step toward R7 but is only reachable through a transport that fails closed. Nothing downstream, including the parent's live C1/D6–D10 proof, can run on this tree. This is unchanged since round 01.

### 2. Critical: the conversation consumer is not migrated onto a namespace path

Requirement: the continuation ("the conversation launch/preparation/continued-session consumer"; "update every conversation launcher to its new schema"; "preserve the configured-local conversation behavior with meaningful successful host launch-to-finish coverage"), C1, R4 ("Remove replaced conversation-specific physical-route and recovery paths").

Evidence:
- The host passes the logical `environment` in the workflow input, which is the declared-input half. It still also launches with a physical `TaskRoute` (`internal/host/host.go:162,171`).
- `Manager.createRemote` still resolves a physical `Target` and `Manager.createLocalEnvironment` still serves the configured-local case. The replaced chain (`Manager -> Target.Route -> TaskRoute.Environment -> environmentMessaging`) is intact.
- `internal/host`, `internal/conversation`, `internal/effects` and `internal/runner` have no diff. Continued turns and controls therefore cannot reach a retained owner through a selected namespace.
- No test starts a host launch to finish for a configured-local environment through a selected namespace. No test sends successive turns or controls to a retained owner.

Impact: the conversation does not use the retained-owner path, an invalid explicit selection is still decided by the old routing, and the required end-to-end behavior is untested.

### 3. Issue: the environment-inventory audit improves but still lacks a completeness argument

Requirement: the continuation ("a real inventory guarantee, not another hard-coded list of message spellings"; "do not let an unfollowed helper/function-value hide environment selection from preflight").

Evidence:
- `auditEnvironments` adds a second walk and new fixture cases, and the boolean `EnvironmentIncomplete` replaces message matching. This is real progress.
- The audit and the shape walker remain two parallel traversals that must agree. The audit does not follow interface or method-value dispatch or calls into non-Gimbal-importing packages, and records an indeterminate call only when `takesContext` is true.
- The audit also records an environment declaration with a zero-valued selector for any `WithEnvironment` the shape walker skipped. No test shows what preflight does with such an entry beyond the two fixtures.

Impact: R2 holds for the fixtures, but the guarantee is still a best-effort second pass, not a stated, tested invariant. It is an issue, not critical, because the earlier reproduced bypasses are now covered.

### 4. Issue: the executor state-directory lease and recovery wiring have no end-to-end test

Requirement: D5a ("A second daemon cannot open the same native state directory even after database authority is released") and "exercise the real selected implementations".

Evidence:
- `cmd/gimbal/messaging.go` takes `executor.Lock(...)` and returns `executor state directory already owned`. That string appears only in source, with no test that starts a second daemon over one state directory.
- `RecoverMessages` is tested only against a direct file store. Nothing exercises it through a database-backed transport, which cannot exist until finding 1 is fixed.

Impact: a mandatory D5a observation is unverified.

### 5. Issue: acceptance evidence is incomplete

Requirement: the continuation requires database-backed registration/delivery/replay/reconnect tests, native-daemon integration tests, affected race tests, generation, frontend checks and the application build.

Evidence:
- No database-backed test of registration, delivery, replay or reconnect exists for the named path. No `-race`, `go generate ./...` comparison, frontend check or `just build` result is recorded.
- The worklog still says generated TypeScript is unformatted because of a missing formatter. `vp` is installed in `web/node_modules/.bin`, so that note is stale.

Impact: the required acceptance bundle cannot be satisfied.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation | Done and tested (pure only) |
| Named delivery over one connection | **Not implemented** (startup error) |
| Retained-owner binding and native owner addressing | **Not implemented** |
| Exclusive registration, authority loss/reacquire/exit | **Not implemented** |
| Startup reconcile and republish of retained completions | Implemented in the daemon (`RecoverMessages`) and tested against the file store; unreachable on the named path |
| Replay/live delivery and bounded reconnect | **Not implemented** |
| Same-boundary input/output bytes | **Not implemented** |
| Worker identity and environment in accepted/completed facts | **Not implemented** |
| State-directory lease in message-worker | Implemented, untested |
| Conversation consumer on the namespace path | **Not implemented** (logical input passed; physical TaskRoute remains) |
| Configured-local launch-to-finish coverage | **Not met** |
| Incomplete-inventory guarantee | Partial (boolean plus audit and fixtures; no completeness argument) |
| Generation, frontend, race, application build | Not demonstrated |

## What is sound

- `parse` rejects unknown fields and trailing JSON; the pure namespace callback resolves names from configuration alone.
- `RecoverMessages` reuses the existing handler and disposition store, so the original event ID is preserved and it is idempotent in the tested case.
- The shared-queue River tests pass against the disposable Postgres, so the old path is intact.
- Failing closed on a named configuration is correct while delivery does not exist. It is an interim guard, not the deliverable.

## Remaining defects and blockers

- Defects: findings 1 and 2 are the unimplemented source outcome. Findings 3–5 are verification and robustness gaps.
- External blocker: none. The disposable Postgres DSN file works and the frontend dependencies are installed.
- Only the recovery helper and the audit hardening advanced since round 01. The transport, the retained-owner contract and the conversation migration are still unimplemented.

Outcome: **material findings remain**. Status is not ready.
