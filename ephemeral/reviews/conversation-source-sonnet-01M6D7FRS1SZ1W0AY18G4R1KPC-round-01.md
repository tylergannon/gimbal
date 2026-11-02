# Adversarial review: conversation source stage, persistent loop round 01 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain**

## Target

- The uncommitted working tree on top of `f2ef93b1` in `/Users/tyler/.codex/worktrees/1430/gimbal`
  (branch `codex/distributed-execution-slice`): 29 modified files, 4 untracked notes.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` and its sources:
  `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`,
  `conversation-declarations-task.md`, `river-named-worker-task.md`, the earlier reviews
  `conversation-declarations-sonnet-round-01.md` and `conversation-source-sonnet-round-01.md`, and the repository `AGENTS.md`.
- Only this artifact was written. The caller gave the target, the artifact path and the read-only boundary. It did not narrow scope or predict a verdict, so nothing was ignored.
  The cross-machine/native/browser proof (C1 live, D6–D10) is out of scope and is not claimed to have occurred.

## Evidence inspected

- `git diff` of the whole tree. The only production changes are:
  - `plugins/messaging-river/main.go`: strict parse, `ReconnectSeconds`/`WorkerID`/`Environments` fields, pure `ValidateEnvironment`.
  - `cmd/gimbal/messaging.go`: executor state-directory lock.
  - `internal/invocation/invocation.go`: reject `EnvironmentIncomplete` diagnostics.
  - `internal/generate/{expr,graph,source}.go` and `workflow/graph.go`: the new `EnvironmentIncomplete` boolean.
  - Regenerated `workflow_gen.go` files, `web/src/lib/workflow/types.ts`, and `cmd/examples/interactive/main.go`.
- Greps over production code for the required path:
  - `plugins/messaging-river/main.go`: the only advisory lock is the schema-migration lock at line 136. `serve` (line 289) and `Connect` (line 346) return
    `river named-worker delivery is not yet implemented` whenever `Environments` is non-empty.
  - `backend/messaging.go`, `messaging_registration.go`, `internal/effects`, `internal/taskdaemon`: no diff. `MessageRequest` has no logical-destination or retained-owner field.
    Accepted/completed facts carry no worker identity or logical environment.
  - `internal/workflows/conversation`, `internal/host`: no use of `WithEnvironment`. `internal/host/host.go:162,171` still launches the conversation with
    `TaskRoute: route`. `internal/conversation/remote.go` still resolves `service.Resolve(...)` to a physical `Target`.
    `internal/host/routes.go:93-94` still derives the display label from `TaskRoute.Environment`.
- Notes `202611012209-conversation-source.md`, `202611012216-conversation-continuation.md` and `20261101-river-named-worker.md`. They themselves say delivery is
  unimplemented and that the inventory work is incomplete.
- Checks I ran personally in the product worktree:
  - `go build ./...`: only the known `main`-less c-shared plugin link errors.
  - `GIMBAL_RIVER_TEST_DSN_FILE=<private dsn file> go test -count=1 ./plugins/messaging-river ./internal/invocation ./internal/generate ./cmd/gimbal`: all `ok`.
    The River package exercises the old shared-queue path and the pure parse test.
  - `web/node_modules/.bin/vp` now exists, so the frozen-lockfile install has happened.
  - I did not run `go generate`, `-race` or the full frontend/application build. The defects below come from source and cannot be changed by those runs.
    Passing those checks would not demonstrate the unimplemented outcome.

## Findings

### 1. Critical: the named-worker River path is still not implemented, and configuring it is a hard startup error

Requirement:
- The continuation: "Complete the actual source outcome rather than returning another inventory: named River workers, stable native owner addressing,
  exclusive registration and state ownership, loss/reconnect/restart behavior, same-boundary input/output bytes", and "Finish the path".
- `river-named-worker-task.md` items 1–6.
- R3, R4, R6, R8, D3 and D5a.

Evidence:
- `plugins/messaging-river/main.go:289-291` and `:346-348` return `errors.New("river named-worker delivery is not yet implemented")` for any config that names an environment.
  A worker, or a workflow process, configured with the only configuration that can name an environment cannot start.
- `ReconnectSeconds` and `WorkerID` are parsed and defaulted but never used.
- There is no registration table or per-worker authority lock, no logical-name to worker resolution, no retained session-owner binding, no authority-loss pause or reacquire/exit,
  no startup reconcile and republish of retained completions, and no byte delivery through the selected boundary.
- None of the shared contracts changed: `backend/messaging.go`, `messaging_registration.go`, `internal/effects` and `internal/taskdaemon` have no diff.

Impact: the source stage's central deliverable does not exist. Nothing downstream, including the live C1/D6–D10 proof the parent plans, can run on this tree.
This is unchanged from the previous review round.

### 2. Critical: the conversation consumer was not migrated

Requirement: continuation ("the conversation launch/preparation/continued-session consumer"; "update every conversation launcher to its new schema; preserve the configured-local conversation behavior with meaningful successful host launch-to-finish coverage; keep both bus configuration and logical input"),
C1, R4 ("Remove replaced conversation-specific physical-route and recovery paths"), and `conversation-placement-prerequisite.md`.

Evidence:
- The registered Conversation workflow does not call `gimbal.WithEnvironment`. The host still passes a physical `TaskRoute` (`internal/host/host.go:162,171`).
  `Manager.createRemote` still resolves a physical `Target` (`internal/conversation/remote.go:71`).
- The replaced path (`UI -> Manager -> physical Target.Route -> invocation.Request.TaskRoute.Environment -> environmentMessaging`) is intact. The required path is absent.
- The only conversation-related edits are `cmd/examples/interactive/main.go` and `cmd/gimbal/daemon_native_test.go`. The daemon test was changed to stop being remote, as the earlier review noted.
- No test starts a host launch to finish against a configured-local namespace environment. No test sends continued turns/controls to a retained owner.

Impact: the conversation does not use the namespace path. An invalid explicit selection is still decided by the old physical routing. The required end-to-end behavior is untested.

### 3. Issue: the environment-inventory guarantee is still best-effort, and `invocation.go` rejects on a marker only some unread shapes set

Requirement: the continuation ("a real inventory guarantee, not another hard-coded list of message spellings"; "do not let an unfollowed helper/function-value hide environment selection from preflight").

Evidence:
- The marker is now a generated boolean (`EnvironmentIncomplete`), which is better than matching prose.
  `internal/generate/expr.go` sets it by hand at six call sites. Other `e.diag` sites for unread shapes remain unmarked.
  The worklog admits "unread nested expressions still need a precise analysis".
- `PrepareContext` (`internal/invocation/invocation.go:80-85`) rejects every workflow whose graph has such a diagnostic, even one that cannot reach a `WithEnvironment` call.
  That is conservative but not a statement about completeness. No test shows a workflow with an unmarked unread shape that hides a selection.

Impact: the preflight guarantee (R2) depends on which call sites remembered to use `unreadEnvironment`. A new unread shape silently weakens it.

### 4. Issue: the executor state-directory lease has no test, and the unrelated `ValidateEnvironment` path is only a name lookup

Requirement: D5a ("A second daemon cannot open the same native state directory even after database authority is released"), and the instruction that checks "must exercise the real selected implementations".

Evidence:
- `cmd/gimbal/messaging.go` takes `executor.Lock(filepath.Join(state, "taskdaemon"))` and returns "executor state directory already owned".
  That string appears only in source. No test starts a second daemon over one state directory or checks release on exit.
- The added River test covers strict parsing only.

Impact: a mandatory D5a observation is unverified.

### 5. Issue: acceptance evidence is incomplete

Requirement: the continuation requires actual database-backed registration/delivery/replay/reconnect tests, native-daemon integration tests, affected race tests, generation, frontend checks and the application build.

Evidence:
- No database-backed test of registration, delivery, replay or reconnect exists for the named path. It cannot exist while finding 1 stands.
- No clean `go generate ./...` comparison, frontend check, `-race` run or `just build` result is recorded in the tree or notes.
  The worklog still states generated TypeScript is unformatted because of the missing formatter. The continuation says `vp fmt` owns this, and `vp` is now installed, so the record is stale.

Impact: the required acceptance bundle cannot be satisfied.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload parsing, pure namespace validation, defaults | Done and tested (pure only) |
| Named delivery to a logical environment over one connection | **Not implemented** (startup error) |
| Retained-owner binding and native owner addressing | **Not implemented** |
| Exclusive registration, authority loss/reacquire/exit | **Not implemented** |
| Startup reconcile/republish, no blind replay | **Not implemented** |
| Replay/live delivery and bounded reconnect | **Not implemented** |
| Same-boundary input/output bytes | **Not implemented** |
| Worker identity and logical environment in accepted/completed facts | **Not implemented** |
| State-directory lease in message-worker | Implemented, untested |
| Conversation consumer on the namespace path | **Not implemented** |
| Configured-local conversation launch-to-finish coverage | **Not met** |
| Boolean incomplete-inventory marker | Partial (six sites marked, no completeness argument) |
| Generation, frontend checks, race tests, application build | Not demonstrated |

## What is sound

- `parse` rejects unknown fields and trailing JSON, and validates names, worker identities and the reconnect default. The pure preflight callback resolves a name from configuration alone.
- The boolean `EnvironmentIncomplete` replaces message-text matching and is covered for the function-value and cross-package shapes. Generated graph literals carry it.
- The shared-queue River tests pass against the disposable Postgres, so the old path is intact.
- Failing closed on a named configuration is correct while delivery does not exist. It is an interim guard, not the deliverable.

## Remaining defects and blockers

- Defects: findings 1 and 2 are the unimplemented source outcome. Findings 3–5 are verification and robustness gaps.
- External blocker: none. The disposable Postgres DSN file is present and the River tests ran against it. `web/node_modules/.bin/vp` is installed.
- The tree has changed little since the previous review round, and the implementation work itself has not been done.

Outcome: **material findings remain**. Status is not ready.
