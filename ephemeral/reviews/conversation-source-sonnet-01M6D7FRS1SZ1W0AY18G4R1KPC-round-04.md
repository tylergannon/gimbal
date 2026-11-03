# Adversarial review: conversation source stage, persistent loop round 04 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain** (one issue, verification only; no critical source defect found)

## Target

- The uncommitted working tree on top of `f2ef93b1` in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`): 59 changed files (1595 insertions, 176 deletions), plus untracked `plugins/messaging-river/named.go`, `internal/generate/testdata/fixture/environment_audit.go` and notes.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` (including its added work-order, consumer and byte-path paragraphs) and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, my rounds 01–03, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary; it did not narrow scope or predict a verdict. The cross-machine/native/browser proof (C1 live, D6–D10) has not occurred and is not claimed.

## Boundary note

Unlike round 03, I did not run `go generate` in the product tree. I checked generation in a copy of the tree under my scratchpad (`rsync` without `.git` and `ephemeral`, with `web/node_modules` symlinked) and ran one scratch test there. The product worktree is unchanged by me: `git diff --stat` still reports 59 files, 1595 insertions, 176 deletions, and the only file I wrote is this artifact.

## Changes since round 03

- **Conversation consumer** (round 03 finding 1):
  - `environments.Service.NamespaceRoute` selects a configured River namespace for a Route machine or for a local directory, without connecting. It rejects `Route.Environment` or an orchestrator, and rejects an environment configured in more than one namespace.
  - `Manager.Create` runs `WorkflowPrepare` with that route, then `createNamespace`, which requires an existing shared directory and persists the namespace route (`conversationRecord.NamespaceRoute`).
  - `Submit` starts the workflow with that route, and `host.Project.Start` accepts a non-absolute native working directory for a named route.
  - `command.go` and `service.go` no longer apply `filepath.Abs` on the named path.
  - Placement is projected onto the conversation from the session's recorded `WorkerID`.
- **Cleanup** (round 03 finding 2): `transport.CleanupScope` sends one stable cleanup request per retained owner and awaits each completion. `effects.Cleanup` calls it on the named path.
- **Placement projection** (round 03 finding 5): `taskPlacement` lifecycle record, schema regenerated, observation store and rows carry `Environment`/`WorkerID`, `Registry` replaces the old task summary with the actual or `mixed` placement, and `RunTranscript.svelte` shows `environment · worker`.
- **Tests added**: named `Submit` rejection and uncertainty (`internal/effects`), cleanup to every retained owner (Postgres), a configured-local conversation reload test, and two opt-in native tests (below).
- The watcher duplication nitpick from round 03 is fixed: there is now a single watcher.

## Evidence inspected

- Diffs of `plugins/messaging-river/{main,named}.go` and its tests, `backend/messaging.go`, `internal/effects`, `internal/taskdaemon`, `internal/conversation`, `internal/host`, `internal/environments`, `internal/observation`, the root lifecycle files, `jsonschema*`, and the web files.
- Checks I ran personally in the product worktree (the DSN is the private disposable-Postgres file):
  - `go build ./...`: only the known `main`-less c-shared plugin link errors. `go vet` over the River package, `./internal/...`, root and `backend/...`: clean.
  - `go test -race -count=1 . ./backend/... ./workflow/... ./plugins/messaging-river ./internal/... ./cmd/...` with `GIMBAL_RIVER_TEST_DSN_FILE` set: every package `ok`, run twice, no failures. The River package runs all 18 Postgres-backed tests.
  - `GIMBAL_MESSAGING_NATIVE_TEST=1 ... go test -race -v -run 'TestNativeNamedSessionBytesContinuityAndCleanup|TestNativeHostConfiguredLocalNamespaceConversation' ./plugins/messaging-river` (real Codex, model `gpt-6-luna`): both **pass**.
    - The first sends memory-only task bytes, retains them across a second turn, and reads a ~160 KB complete context value on the named session, with cleanup.
    - The second launches a configured-local directory through the host and manager over River, gets two replies in one conversation, asserts `Placement.Machine.ID == "conversation-worker"` and the session row's `Environment`/`WorkerID`, then finishes the conversation with workflow cleanup.
    - Both ran on one machine and cannot show absence of access to A's files. That belongs to the later live proof.
  - `web/node_modules/.bin/vp run check`: 0 errors and 0 warnings across 5494 files. `vp test --run`: 234 of 234 pass. `vp fmt --check`: clean.
  - Generation, in the scratch copy only: `go generate ./...` followed by `vp fmt` on the three generated TypeScript files reproduces `observation/types.ts`, `workflow/types.ts` and `workflow-design/generated/index.ts` byte-for-byte as they are in the worktree. Go generated files and `jsonschema` match. Generation is deterministic given the owning formatter.
  - Scratch test in the copy (not committed): with `ReconnectSeconds=1`, I terminated a running worker's authority backend and had another session take the identity lock. `serve` returned `river worker authority lost`, and a request published meanwhile was never admitted (handled count 0). The behavior in finding 1 below works; it is just not tested in the repo.
- Production line counts against the parent cell maximums: the Transport files total 1,785 lines (maximum 2,250). Native dispatch totals 2,679 (maximum 3,300). Conversation, host and environment additions total about 190 lines (the prerequisite's tripwire is 700). No breach.

## Findings

### 1. Issue: mandatory D3/D5a observations and several controls have no committed test

Requirement: the continuation ("They must exercise the real selected implementations"; "actual database-backed registration/delivery/replay/reconnect and native-daemon integration tests"; "turns/controls"), D3 ("Lost authority stops new admissions despite a healthy data pool ... exhausted recovery stops consumption") and D5a ("A second daemon cannot open the same native state directory even after database authority is released").

Evidence:
- The state-directory lease added in `cmd/gimbal/messaging.go` (`executor state directory already owned`) has no test anywhere: `grep` finds the string only in source, and no `cmd/gimbal` test starts a second daemon over one state directory or checks release on exit.
- `TestPostgresWorkerAuthorityReacquiresAfterConnectionLoss` calls `confirm`/`reacquire` directly. No committed test drives `serve` through loss, so neither "admission is refused while authority is lost although the pool is healthy" nor "budget exhaustion stops consumption and exits with the actionable error" is demonstrated by the repository. My scratch test shows both behave correctly; it is not in the tree.
- Native-daemon coverage on the named path covers process commands, prompt turns, finish and cleanup. Steer, interrupt and identified-close at the retained owner are exercised only with stub handlers (the rebind test) or not at all, and the two native tests are opt-in (`GIMBAL_MESSAGING_NATIVE_TEST=1`).

Impact: a required claim ("lost authority stops admissions", "second daemon cannot open the state directory") is true by reading and by my scratch run but is not repeatable from the repository, so the stage's "required checks genuinely demonstrate it" bar is not yet met. The fix is tests only. No source defect depends on it.

### Nitpicks

- `CleanupScope` groups retained owners by worker with `min(environment)`. If one worker ever served two environment names under one scope, the cleanup request carries only the smallest name. Cleanup is routed by worker, so this is cosmetic.
- `Manager.Get` writes and saves the item while holding a read lock when it projects the worker placement. This copies the pre-existing transcript-projection pattern, and the race tests pass.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation | Done and tested |
| Addressed delivery to a logical name over one connection, no orchestrator/endpoint | Done; Postgres-tested; `Submit` rejection tested |
| Retained-owner addressing after rebind | Done; Postgres-tested (stub handler) and exercised by the native conversation test |
| Exclusive registration, duplicate rejection | Done and tested |
| Authority loss, reacquire, exit on exhaustion | Implemented; reacquire tested directly; exhaustion and admission-halt verified by my scratch test only |
| Startup recovery and republish with original event ID | Done and tested (daemon and Postgres restart) |
| Replay, live delivery, bounded reconnect | Done and tested |
| Scope cleanup to every retained owner | Done and tested |
| Input/output bytes and complete-context value over the boundary | Done through the existing `TaskFile`/`MessageResult` byte fields; native test passes on one machine |
| Placement in lifecycle facts, projection, run page | Done; store test and native assertion |
| State-directory lease in message-worker | Implemented, **untested** |
| Conversation consumer on the namespace path, configured-local launch-to-finish | Done; native host test passes; reload test passes |
| Conversation controls beyond prompt/finish through the daemon | Not covered |
| Environment inventory guard | Boolean plus audit and fixtures |
| Race tests, frontend checks, deterministic generation, build | Run and pass |

## What is sound

- The named path is coherent: queues derive from worker identity, owner binding commits in the publish transaction with the River insert, acceptance is fenced on the lock-owning connection, and an in-flight reconnect keeps the queued work.
- The consumer migration removes the physical route for namespace environments and keeps the old path for non-namespace ones (existing conversation and environment suites pass). A namespace route that also names a physical destination is rejected.
- Generation, formatting, frontend checks and the full race matrix all pass, and a real native `luna` session and a real host conversation ran end to end over the Postgres River transport.

## Remaining defects and blockers

- Defects: finding 1 only, a test-coverage gap for D3, D5a and controls, with correct underlying behavior.
- External blocker: none. The disposable Postgres, authenticated Codex and the frontend toolchain were all available.
- The later C1/D6–D10 cross-machine, Compose and browser proof has not occurred and is not claimed.

Outcome: **material findings remain** (finding 1). Status is not ready until the lease, serve-level loss/exhaustion, and retained-owner controls have committed tests.
