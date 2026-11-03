# Adversarial review: conversation source stage, persistent loop round 06 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **material findings remain** (one issue with a partial reproduction; no critical defect found)

## Target

- The working tree in `/Users/tyler/.codex/worktrees/1430/gimbal` (branch `codex/distributed-execution-slice`) on top of commit `fb7009b2` ("feat: route named native work and conversations through River"): 28 modified files plus the untracked `cmd/gimbal/messaging_worker_lease_test.go` and `cmd/gimbal/route_overlay_test.go`.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md` and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, my rounds 01–05, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary and did not narrow scope or predict a verdict. The cross-machine/browser proof (C1 live, D6–D10) has not occurred and is not claimed.

## Boundary note

I did not run `go generate` in the product tree. Generation and the checkout-script experiments ran in copies under my scratchpad. The only file I wrote in the product worktree is this artifact.

## Changes since round 05

- **Round 05's finding closed.** The configuration gained `RebindFrom` (name to previous worker identity), and the plugin now advertises a `ConfigurationSchema`.
  - `bindName` is a compare-and-swap. It takes the binding row `FOR UPDATE`. It refuses a name bound to someone else unless `RebindFrom` names exactly that previous worker. It rejects a rebind with no previous binding.
  - `registerWorker` applies it, so a fresh worker identity can now register for a name it is configured for.
  - `TestPostgresNamedWorkersReceiveOnlyAddressedRequests` now covers the order I showed was impossible. A fresh `worker-c` that has never registered is refused with a wrong previous owner and accepted with the correct one. A new task then goes to `worker-c` and a turn for the old session still goes to `worker-a`. Parse tests cover the `RebindFrom` validation.
- **`--task-route @file` overlay** (`cmd/gimbal/workflows.go`, `route_overlay_test.go`): object or array, recursive merge, plugin switch drops old configuration, and unknown or duplicate fields are rejected. `TestRouteOverlays` passes.
- **Checkout preparation on the named path** (`internal/workflows/conversation/conversation.go`, `internal/host/host.go`): new `PrepareCheckout`, `CheckoutID`, `Branch`, `Base` inputs. When set, the workflow runs a git-worktree script on the selected worker through `RunCommand` and starts the session in the printed directory. The host sets them for any namespace route whose conversation is not a shared checkout. See finding 1.
- Other diffs read: `backend/messagefiles`, `interactive.go`, `internal/runstate`, `internal/taskdaemon`, `internal/conversation/{manager,remote,requests}.go`, `environment_messaging.go` and the generated files.

## Evidence

Checks I ran personally in the product worktree (the DSN is the private disposable-Postgres file):

- `go build ./...`: only the known `main`-less c-shared plugin link errors. `go vet` over root, `backend/...`, the River plugin, `./internal/...` and `./cmd/...`: clean.
- `go test -race -count=1 . ./backend/... ./workflow/... ./plugins/messaging-river ./internal/... ./cmd/...` with `GIMBAL_RIVER_TEST_DSN_FILE` set: no failures in any package (including the lease test and the 19 Postgres-backed tests).
- Opt-in native tests with `GIMBAL_MESSAGING_NATIVE_TEST=1` and real Codex (`luna`): `TestNativeNamedSessionBytesContinuityAndCleanup`, `TestNativeHostConfiguredLocalNamespaceConversation` and `TestNativeNamedSessionControlsStayAtRetainedOwner` pass under `-race`. I did not rerun the older shared-queue `TestNativeMessagingWorkflowInputsContinuityAndCleanup`, which failed identically at `f2ef93b1` in round 05.
- Web: `vp run check` completes (svelte-check) with no errors; `vp test --run` and `vp fmt --check` ("All matched files use the correct format") pass.
- Generation, in a scratchpad copy: `go generate ./...` followed by `vp fmt` on the three generated TypeScript files reproduces the worktree byte for byte. A recursive diff of the copy against the product tree reports no differences.
- Checkout script, in a scratchpad git repo and non-git directory: see finding 1.

## Findings

### 1. Issue: the new checkout preparation is untested, and a plain (non-git) configured directory fails to launch on the default option

Requirement: the continuation ("preserve the configured-local conversation behavior with meaningful successful host launch-to-finish coverage"; "update every conversation launcher") and AGENTS.md ("Unit tests for what you are building"). The old local path (`internal/environments/local.go`, `prepareLocal`) ran a non-git directory in place unless a branch or base was explicitly requested.

Evidence:
- `grep` finds no test referencing `prepare_checkout`, `PrepareCheckout` or `checkout_id`. The only namespace conversation tests (`native_test.go:241`, `internal/conversation/remote_test.go:67`) use `SharedCheckout: true`, which skips the script. The checkout script (about 20 lines of shell and git logic) and its host wiring have no coverage.
- `Manager.createNamespace` fills `Branch` with `gimbal/conversation-<id>` whenever the conversation is not a shared checkout and no branch was given (`remote.go`). The host then passes that branch to the workflow whenever the route is a namespace and `!SharedCheckout` (`host.go:167`).
- The script exits 1 with `directory has no committed Git history` when the selected directory is not a Git repository and a branch is set. I ran it in a non-git directory with a `gimbal/conversation-…` branch: `directory has no committed Git history`, exit 1. With an empty branch it prints the directory, so only the auto-filled branch breaks it.
- By reading, the combination is therefore: a configured-local environment that points at a plain directory (like the `local-notes` environment in the native test), launched with the default non-shared option, fails when the workflow runs. The old path accepted this.
- I did not run a full host launch for that combination. The script behavior is reproduced in isolation, and the branch auto-fill and host condition are from the source.

Impact: the default UI choice on a non-git configured directory is likely a launch failure, and no committed test would have caught it because every namespace conversation test uses the shared option. Fix: either leave the branch empty for the namespace non-shared case when the directory is not a Git repository, or skip the script for a non-git directory, and add a test for the non-shared path (git and non-git).

### Nitpicks

- The script is not idempotent if it runs twice for one conversation. A retry with the same branch fails with `a branch named '…' already exists`. This applies only when the node fails before its result is recorded.
- The created worktree under `${TMPDIR:-/tmp}/gimbal-conversations/<id>` is never removed on conversation finish. `Conversation.Worktree` on the namespace path still shows the selected directory, not the checkout actually used.
- `CleanupScope` groups retained owners with `min(environment)`; cosmetic, since cleanup is routed by worker.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation, configuration schema | Done and tested |
| Addressed delivery by logical name; no orchestrator/endpoint | Done; Postgres-tested |
| Retained-owner addressing; turns, steer, interrupt, close after rebind | Done; real native session test passes |
| Rebinding a name to a fresh worker identity | Done (round 05 finding closed); Postgres-tested incl. wrong previous owner |
| Exclusive registration, duplicate rejection, state-dir lease | Done; Postgres and CLI lease tests pass |
| Authority loss, reacquire, exhaustion exits and refuses admission | Done and tested through `serve` |
| Startup recovery; replay by source sequence; bounded reconnect | Done and tested |
| Scope cleanup to every retained owner | Done and tested |
| Bytes and complete-context value over the boundary | Done; native tests pass on one machine |
| Placement facts, projection, run page | Done |
| Conversation consumer, configured-local launch-to-finish (shared checkout) | Done; native host test passes |
| Conversation non-shared checkout on the namespace path | **Untested; non-git directory fails with the default branch** (finding 1) |
| Environment inventory guard | Boolean plus audit and fixtures |
| Race tests, frontend checks, deterministic generation | Pass (generation checked in a scratch copy) |

## What is sound

- The named transport is coherent and well tested: compare-and-swap rebinding, queue derivation, acceptance fencing, loss handling, restart recovery and cleanup behave under `-race` and with a real native session.
- Route overlay handling is strict and has a table test. Generation is deterministic.

## Remaining defects and blockers

- Defects: finding 1, a launch failure for a plain configured directory under the default checkout option, with an untested code path.
- External blocker: none. Postgres, authenticated Codex and the frontend toolchain were available.
- The later C1/D6–D10 cross-machine, Compose and browser proof has not occurred and is not claimed.

Outcome: **material findings remain** (finding 1). Status is not ready.
