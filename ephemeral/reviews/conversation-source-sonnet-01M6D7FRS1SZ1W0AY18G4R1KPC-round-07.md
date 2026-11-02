# Adversarial review: conversation source stage, persistent loop round 07 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **only nitpicks remain**

## Target

- The working tree in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`) on top of commit `fb7009b2`: 28 modified files (924 insertions, 722 deletions in the diff stat) plus the untracked `cmd/gimbal/messaging_worker_lease_test.go` and `cmd/gimbal/route_overlay_test.go`.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, my rounds 01–06, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary and did not narrow scope or predict a verdict. The later C1 live and D6–D10 cross-machine, Compose and browser proof has not occurred and is not claimed.

## Boundary note

I did not run `go generate` in the product tree. Generation ran in a scratchpad copy. The only file I wrote in the product worktree is this artifact. Scratch experiments in earlier rounds stayed in my scratchpad.

## Changes since round 06

- **Round 06's finding is closed.**
  - `createNamespace` no longer invents a branch for a non-shared conversation. The old guard that rejected non-shared namespace conversations is gone. The checkout script generates the default branch name itself, only when the directory is a Git repository with a commit, and reports it back through `gimbal.Set(ctx, "conversation_checkout_branch", …)`.
  - A directory that is not a Git repository, or has no commit, runs in place. An explicit branch or base there fails with `directory has no committed Git history` before any session starts.
  - `Manager.Get` projects the checkout branch and the session's actual working directory onto the conversation.
  - `TestNativeHostConfiguredLocalNamespaceConversation` now covers five launches through the real host, manager, River and a native `luna` session: shared checkout with an attached file, default on a plain directory (runs in place, no branch), an explicit branch on a plain directory (fails before any session exists), an unborn repository (runs in place), an explicit branch and base on a committed repository (a separate worktree on that branch, checked with `git`), and the default on a committed repository (`gimbal/conversation-<id[:12]>` projected and checked with `git`). `internal/conversation/remote_test.go` adds a reload test for checkout choices and a retry test showing a retried start does not duplicate the checkout.
- **Replaced conversation path removed.** `createRemote`, `createLocalEnvironment`, `remoteState`, `activeConversation`, the retained transport record and the `remoteReaders` machinery are deleted from `internal/conversation`. `Manager.Create` now requires a messaging namespace for any non-local environment, so there is no longer a second physical-route path beside the River one (R4).

## Evidence

Checks I ran personally in the product worktree (the DSN is the private disposable-Postgres file):

- `go build ./...`: only the known `main`-less c-shared plugin link errors. `go vet` over root, `backend/...`, the River plugin, `./internal/...` and `./cmd/...`: clean.
- `go test -race -count=1 . ./backend/... ./workflow/... ./plugins/messaging-river ./internal/... ./cmd/...` with `GIMBAL_RIVER_TEST_DSN_FILE` set: no failures in any package. This includes the Postgres-backed River tests (registration, addressed delivery, compare-and-swap rebind, authority loss and exhaustion at `serve` level, replay ordering, reconnect, restart recovery, scope cleanup), the CLI state-directory lease test, and the route overlay test.
- Opt-in native tests (`GIMBAL_MESSAGING_NATIVE_TEST=1`, real Codex `luna`, `-race`): `TestNativeNamedSessionBytesContinuityAndCleanup`, `TestNativeHostConfiguredLocalNamespaceConversation` (the five launches above) and `TestNativeNamedSessionControlsStayAtRetainedOwner` pass. The older shared-queue `TestNativeMessagingWorkflowInputsContinuityAndCleanup`, which failed in round 05 and at `f2ef93b1`, passed three times this round (once without and twice with `-race`), so that failure was not reproduced.
- Web: `vp run check` reports 0 errors and 0 warnings across 5494 files. `vp test --run` passes 234 of 234 tests in 48 files. `vp fmt --check` passes.
- Generation, in a scratchpad copy: `go generate ./...` followed by `vp fmt` on the three generated TypeScript files reproduces the product tree byte for byte (the only difference reported is the dangling `docs/lint-rules.md` symlink, which is tracked and unchanged).
- Source read: `plugins/messaging-river/{main,named}.go`, `messaging_registration.go`, `backend/messaging.go`, `internal/conversation/{manager,remote,requests}.go`, `internal/host/host.go`, `internal/workflows/conversation/conversation.go`, `cmd/gimbal/workflows.go`, and the native and Postgres tests.

## Findings

No critical issue or issue remains. Nitpicks:

1. **Replaced checkout code is left as production code with no remaining caller.** `Service.Prepare`, `prepareLocal` and the helper's "prepare" operation (`internal/environments/operations.go:36`, `local.go:259`) are now exercised only by their own tests; the conversation manager no longer calls them. The same checkout policy is now written a second time as a shell script in `internal/workflows/conversation/conversation.go`. AGENTS.md says to delete what is replaced, so this is a cleanup candidate. `Service.Acquire` and `Release` are reachable only through the same operations.
2. **The checkout script is covered only by the opt-in native test.** The default `go test` run checks the host and manager choices against stubs but never runs the script against real Git. A hermetic test with temporary repositories (plain, unborn, committed, explicit branch) would make the policy repeatable without Codex.
3. **Checkout preparation is not idempotent, and its worktree is never removed.** If the node ran twice for one conversation before its result was recorded, the second run fails because the branch already exists. The worktree under `${TMPDIR:-/tmp}/gimbal-conversations/<id>` is not removed when the conversation finishes; the native test removes its worktrees itself.
4. **`CleanupScope` reports `min(environment)` per worker.** Cosmetic, since cleanup is routed by worker.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation, configuration schema | Done and tested |
| Addressed delivery by logical name; no orchestrator/endpoint | Done; Postgres-tested |
| Retained-owner addressing; turns, steer, interrupt, close after rebind | Done; real native session test passes |
| Rebinding a name to a fresh worker identity (explicit `RebindFrom`) | Done; Postgres-tested, including a wrong previous owner |
| Exclusive registration, duplicate rejection, state-dir lease | Done; Postgres and CLI lease tests pass |
| Authority loss, reacquire, exhaustion exits and refuses admission | Done and tested through `serve` |
| Startup recovery; replay by source sequence; bounded reconnect | Done and tested |
| Scope cleanup to every retained owner | Done and tested |
| Bytes and complete-context value over the boundary | Done; native tests pass on one machine |
| Placement facts, projection, run page | Done |
| Conversation consumer on the selected namespace, launch-to-finish, checkout choices | Done; native host test covers shared, plain, unborn, explicit and default Git |
| Replaced conversation physical-route and recovery paths removed | Done in `internal/conversation` (see nitpick 1 for the environments service) |
| Environment inventory guard | Boolean plus audit and fixtures |
| Race tests, frontend checks, deterministic generation, build | Pass |

## What is sound

- Registration, addressing, rebinding, loss, replay and cleanup behave under `-race`, against a real database, and with a real native session.
- The consumer exercises the selected River path through the real host and manager, and failures for an invalid explicit branch occur before any session exists.
- Generation is deterministic and the frontend gates are clean.

## Remaining defects and blockers

- Defects: none material. Nitpicks 1–4 only.
- External blocker: none. Postgres, authenticated Codex and the frontend toolchain were available.
- Not claimed: the later C1 live and D6–D10 cross-machine, Compose and browser proof. Everything above ran on one machine; no check here shows that a worker lacks access to the sender's files.

Outcome: **only nitpicks remain**.
