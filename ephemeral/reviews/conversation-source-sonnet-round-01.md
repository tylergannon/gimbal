# Adversarial review: conversation source stage (named River workers + conversation consumer)

Reviewer: Sonnet 5.5, independent session, round 01.
Outcome: **material findings remain**

## Target

- The uncommitted working tree on top of `9fe8e769` in `/Users/tyler/.codex/worktrees/1430/gimbal`
  (branch `codex/distributed-execution-slice`): 16 modified files plus the untracked plan and worklog notes.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` (the authoritative task) and
  its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`,
  `conversation-declarations-task.md`, `river-named-worker-task.md`, the previous review
  `conversation-declarations-sonnet-round-01.md`, and the repository `AGENTS.md`.
- No project file other than this artifact was written.

## Operating-constraint note

The launch prompt gave the target, the artifact path and a read-only boundary. It did not narrow the
review or predict a verdict, so nothing was ignored. The later cross-machine/native/browser proof
(C1 live, D6–D10) is out of scope and is not claimed to have occurred.

## Evidence inspected

- `git diff` of the whole working tree: `cmd/gimbal/messaging.go`, `internal/invocation/invocation{,_test}.go`,
  `plugins/messaging-river/main{,_test}.go`, `cmd/examples/interactive/main.go`,
  `cmd/gimbal/daemon_native_test.go`, and nine regenerated `internal/workflows/*/workflow_gen.go`.
- Surrounding code: `PrepareContext` (`internal/invocation/invocation.go:70-150`),
  `Manager.createLocalEnvironment` (`internal/conversation/remote.go:236`),
  `internal/host/routes_test.go:160-176`, `backend/messaging.go`, `messaging_registration.go`,
  `plugins/messaging-river/main.go` (all 357 lines).
- Source greps: `not yet implemented`, `ValidateEnvironment`, `pg_advisory`, `WorkerID`,
  `executor state directory already owned`.
- Both worklog notes (`20261101-river-named-worker.md`, `202611012209-conversation-source.md`). They
  themselves record that delivery is unimplemented and that the inventory work is incomplete.
- Checks I ran personally in the product worktree:
  - `go build ./...`: only the known `main`-less c-shared plugin link errors.
  - `GIMBAL_RIVER_TEST_DSN_FILE=<private dsn file> go test -count=1 ./plugins/messaging-river ./internal/invocation ./cmd/gimbal`:
    all `ok`. In `-v` the River package runs 9 tests (existing shared-queue queue/retained-event/slow-listener/schema tests
    plus the new strict-parse test) against the disposable Postgres; `TestNativeMessagingWorkflowInputsContinuityAndCleanup`
    is `SKIP` (needs `GIMBAL_MESSAGING_NATIVE_TEST=1`).
  - `web/node_modules/.bin/prettier` does not exist, so a clean `go generate ./...` comparison and the frontend checks
    cannot be produced in this worktree (the worklog records the same blocker).
- I did not run the full `-race` matrix or the application build. The core defect below is established by reading
  source, so there was no passing outcome those runs could change.

## Findings

### 1. Critical: the named-worker River delivery path was not implemented; configuring it is a hard error

Requirement: the continuation ("Complete the actual source outcome rather than returning another inventory: named River
workers, stable native owner addressing, exclusive registration and state ownership, loss/reconnect/restart
behavior, same-boundary input/output bytes ..."; "Finish the path"; "requires actual database-backed
registration/delivery/replay/reconnect and native-daemon integration tests"), `river-named-worker-task.md`
items 1–6, and R3/R4/R6/R8, D3, D5a.

Evidence:
- `plugins/messaging-river/main.go:289-291` (`serve`) and `:346-348` (`Connect`) both return
  `errors.New("river named-worker delivery is not yet implemented")` whenever `Environments` is non-empty.
  A worker or a workflow process configured with the only configuration that can name an environment cannot start.
- The remaining River code is still the single shared-queue implementation. `grep` finds one advisory lock
  (`pg_advisory_lock(745289004)`, the schema-migration lock at `main.go:136`). There is no per-worker registration table
  or authority lock, no worker-identity use of `WorkerID` (parsed and whitespace-checked only), and no use of
  `ReconnectSeconds` (parsed and defaulted only).
- `backend/messaging.go` is unmodified: `MessageRequest` carries no logical destination or retained-owner field, and
  accepted/completed facts carry no worker identity or logical environment (R6, R10). Effects still cannot submit a captured
  environment to a name.
- Nothing implements: logical-name to worker resolution; persistence of the session-owner binding before ambiguous delivery;
  turns/controls reaching the original worker after rebinding; exclusive registration (D3 duplicate-identity rejection, authority
  loss pausing admission, reacquire within budget, exit on exhaustion); startup reconciliation and republish of retained
  completions; identity-based replay with bounded reconnect for waits and watchers; explicit no-owner rejection versus
  delivery uncertainty; input/output byte delivery over the boundary.
- The only River test added is `TestNamespaceValidationIsPureAndStrict`, a pure configuration test. There are no
  database-backed registration, delivery, replay, reconnect, rebind, or native-daemon tests for the named path (the
  existing database tests exercise the shared queue only). The worklog confirms: "Namespace configurations must fail
  closed until addressed delivery and database fencing are implemented; a passing preflight alone is not proof of worker routing."

Impact: the principal source-stage outcome does not exist. Preflight now accepts names that no transport can serve, so a
valid-looking configuration passes validation and then fails at startup. Conversation selection of a remote environment cannot
reach a native worker through this tree. The green test results say nothing about D3, D5a or C1's source prerequisites.

### 2. Critical: no conversation consumer migration to the namespace/retained-owner path

Requirement: continuation ("the conversation launch/preparation/continued-session consumer"; "Keep both bus configuration and
logical input"; "preserve configured local environments through a real selected namespace path"), the prerequisite plan
work items 3–4, R4 ("Remove replaced conversation-specific physical-route and recovery paths"), C1.

Evidence:
- No change to `internal/conversation/{manager,remote,requests}.go`, `internal/host/*`, `internal/environments/*`,
  `internal/runner`, `internal/effects`, `internal/taskdaemon`, `session.go`, `command.go`, or the observation/web files in this
  pass. The only conversation-adjacent edits are two launchers sending `"environment":"local"`.
- `Manager.createLocalEnvironment` (`internal/conversation/remote.go:236`) still creates a conversation for any registered
  "This Mac" environment and stores the registered environment's name as `Environment`. `PrepareContext`
  (`invocation.go:104-108`) accepts only the literal `"local"` when the route has no name, so such a named local
  environment is rejected before `createLocalEnvironment` runs. This is the same problem the previous review raised as its
  finding 4. It is neither fixed nor recorded as a decision, and the continuation explicitly demands that configured local
  environments work "through a real selected namespace path, never by accepting arbitrary strings or silently executing an invalid
  selection locally". Because no messaging plugin accepts a non-`local` name on a real path (finding 1), no configured name works at all.
- The deleted `TestLocalWorkflowRetainsRemoteTaskDirectoryAndRoute` was not replaced. `internal/host/routes_test.go:160-176`
  only asserts that a physical `http` route with `environment:"remote"` is rejected. The continuation requires "meaningful successful host
  launch-to-finish coverage" for the configured-local conversation. A search finds no test that launches a registered `conversation` through the host and
  finishes it. `cmd/gimbal/daemon_native_test.go` (opt-in, `GIMBAL_TEST_NATIVE_INTERACTIVE=1`) was edited by removing its
  remote `TaskRoute` entirely, so it is now a local test; it is not the remote-owner test it was named for.
- The replaced conversation-specific remote preparation / recovery machinery (`remoteStates`, retained route paths) remains in
  `internal/conversation`, contrary to R4 "no shim; delete what is replaced". It is unreachable for any non-local input.

Impact: C1's source-side prerequisites (launch, preparation, successive turns and controls at an admitted owner, truthful
placement) are not implemented. The configured-local conversation requirement is unmet.

### 3. Issue: the "incomplete inventory" guard matches diagnostic text and rejects every workflow that hits one, not environment-bearing ones

Requirement: continuation ("do not let an unfollowed helper/function-value hide environment selection from preflight"; "preserve
generated graph consistency"), R2, D1.

Evidence: `internal/invocation/invocation.go:80-88` classifies a graph as incomplete by `strings.Contains` on three exact diagnostic
message strings. Those strings are produced elsewhere in `internal/generate` and nothing ties them together: changing wording
there silently disables the guard, and `TestUnreadCallsRejectBeforeWorkflow` builds the same literal strings by hand, so it cannot
detect drift. A genuine fix would give the diagnostic a typed kind in `workflow.Diagnostic` and have the generator mark it. The worklog itself says
"Other graph-analysis gaps still need assessment before claiming a complete environment inventory": no further gap
assessment (for example the non-direct-call shapes in the previous review's finding 1, or `WithEnvironment` reached through
interface/method values) is recorded in the tree.

Impact: the preflight guarantee rests on message text. It currently works for the three recorded shapes, but it is fragile, and the
completeness claim in R2 is not established. This is an issue, not critical, because the three previously reproduced bypasses are closed
and tested.

### 4. Issue: the new state-directory lease has no test and was added only to one command

Requirement: continuation / `river-named-worker-task.md` item 3 ("Add the existing executor-state directory lease to message-worker"),
D5a ("A second daemon cannot open the same native state directory even after database authority is released").

Evidence: `cmd/gimbal/messaging.go` now takes `executor.Lock(filepath.Join(state, "taskdaemon"))` and returns
"executor state directory already owned". `grep` finds that error string only in the source: no test starts a second daemon over one state
directory, or checks that the lease is held for the daemon's lifetime and released on exit. D5a explicitly demands this observation.
The ordering relative to `ValidateConfiguration` and credential read is as required, but nothing asserts "validate before connection/credential read".

Impact: a stated, mandatory behavior is unverified.

### 5. Issue: acceptance evidence does not include generation, frontend or application build checks

Requirement: continuation ("generation, frontend checks and application build" are required for acceptance).

Evidence: the nine `workflow_gen.go` files now carry `Environments: []workflow.EnvironmentDeclaration{},`, which closes the previous drift in
the direction the generator emits. I could not confirm a clean `go generate ./...` because `web/node_modules/.bin/prettier` is missing
(`ls` fails), so TypeScript generation emits unformatted output and the frontend check cannot run. The worklog records that frontend installation is a precondition.
No evidence that `just build` or the web checks passed exists in the tree or notes. I did not run them (see Evidence). Treat this as an unmet
acceptance check rather than a code defect; it is a local-toolchain gap the implementer can fix by installing the web dependencies.

Impact: the required acceptance bundle is incomplete even apart from findings 1–2.

## Requirement coverage summary

| Requirement | Status in the working tree |
| --- | --- |
| Strict River payload parsing, pure namespace validation, defaults | Done and tested (pure) |
| Named delivery over one connection to a logical environment | **Not implemented** (hard error) |
| Retained-owner binding, rebind survival, native owner addressing | **Not implemented** |
| Exclusive worker registration, authority loss/reacquire/exit | **Not implemented** |
| Startup reconcile/republish of retained completions, no blind replay | **Not implemented** |
| Replay/live delivery with bounded reconnect, no-owner vs uncertainty | **Not implemented** |
| Same-boundary input/output bytes | **Not implemented** |
| Placement identity in accepted/completed facts | **Not implemented** |
| State-directory lease in message-worker | Implemented, untested |
| Conversation launcher schema updates | Done (`interactive`, daemon test), but daemon test no longer remote |
| Configured-local conversation via a namespace path; launch-to-finish test | **Not met** |
| Unread helper / function-value / cross-package rejection | Done by message matching, fragile (finding 3) |
| Generated graph consistency | Appears consistent; clean `go generate` unverifiable here |
| Database-backed registration/delivery/replay/reconnect and native-daemon tests | **None exist for the named path** |
| Generation, frontend checks, application build | Not demonstrated |

## What is sound

- `parse` now rejects unknown fields and trailing JSON, and validates names/worker identities and the reconnect default. It is pure and
  tested, and the preflight callback resolves a name from configuration only.
- The shared-queue River tests pass against the disposable Postgres, so the pre-existing path is intact.
- Failing closed (rather than serving a partial named path) is the correct interim behavior and is honestly documented in the worklog.

## Remaining defects and blockers

Defects: findings 1 and 2 are the unimplemented source outcome itself; findings 3–5 are verification and robustness gaps.

External blocker: none identified. The disposable Postgres DSN file is present and River tests ran against it; the only environmental
gap is the missing `web/node_modules` (a local install, not an external blocker). The cross-machine/native/browser proof (C1 live, D6–D10) has not
occurred and is not claimed here.

Outcome: **material findings remain**. This is not ready.
