# Adversarial review: conversation source stage, persistent loop round 08 (session 01M6D7FRS1SZ1W0AY18G4R1KPC)

Reviewer: Sonnet 5.5, independent session.
Outcome: **only nitpicks remain**

## Target

- Branch `codex/distributed-execution-slice` in `/Users/tyler/.codex/worktrees/1430/gimbal`. I began on an uncommitted tree on top of `fb7009b2` (39 files) and finished on `1006a81d`. During the review the parent committed that tree as `88b9314e` ("feat: complete named conversation setup and worker recovery"), then `cc919362` (worklog only) and `1006a81d` (errcheck-style `defer func() { _ = x.Close() }()` edits in three test files). I re-ran the full matrix after the commits, and the last three packages again after `1006a81d`. The working tree is clean.
- Measured against `ephemeral/plans/conversation-implementation-continuation.md`, including its added paragraphs (route overlay and schema, per-stream sequence, nil-selected-factory rejection, non-shared preparation and attachments, removal of replaced paths, the older native test, and the live UI setup finding), and its sources: `distributed-execution-slice.md` (R1–R10, D1–D5b, C1), `conversation-placement-prerequisite.md`, `conversation-declarations-task.md`, `river-named-worker-task.md`, my rounds 01–07, and the repository `AGENTS.md`.
- The caller gave the target, artifact path and read-only boundary and did not narrow scope or predict a verdict. The later C1 live and D6–D10 cross-machine, Compose and browser proof has not occurred and is not claimed. The worklog also records that a host-level dependency is still open for that proof; it is outside this source stage.

## Boundary note

I did not run `go generate` in the product tree; generation ran in a scratchpad copy. The only file I wrote in the product worktree is this artifact.

## Changes since round 07

- **Live UI setup finding closed.** `Manager.SetupEnvironment` now requires a messaging namespace (an unlisted name is rejected before any connection; `remote_test.go`). It resolves the logical name first, then inspects through the selected boundary. `Service.Inspect` returns the worker's directory and harness facts. Requests and cleanup use the logical environment name where the bus is named, and a configured-local directory is inspected by the worker using the worker's `gimbal` executable, not the sender's path. Setup advertises steer and not fork.
  - `TestNativeHostConfiguredLocalNamespaceConversation` now runs `SetupEnvironment` for `local-notes` and the remote-named `task-remote` against the real River worker, checks the worker's harnesses and directory, checks via the database that both reached the named worker, and then starts and finishes conversations on both.
- **Nil-selected-factory rejection.** A messaging connection, execution connection, secrets store or orchestrator that returns nil without error is rejected, and already-created dependencies are closed (`registry.go`, `resource_registration.go`, `environment_messaging.go`). `TestSelectedFactoriesCannotReturnNil` and `TestEnvironmentRouterRejectsNilSelectedBus` cover them.
- **Older shared-queue native test fixed.** It used a service command that never wrote its readiness file; it now writes `$ENV_FILE`. This explains the failure I saw at the baseline in round 05. It also uses the unversioned `luna` alias and labels the run truthfully.
- **Stream identity stated** in `backend/messaging.go`: `Source` names an independently ordered stream and `Sequence` is its durable ordinal. No global native order is implied.
- Earlier: route overlay and plugin schema, `RebindFrom`, checkout preparation with the branch projected from the workflow, and removal of `createRemote`, `remoteState` and the retained transport record.

## Evidence

Checks I ran personally in the product worktree (the DSN is the private disposable-Postgres file):

- `go build ./...`: only the known `main`-less c-shared plugin link errors. `go vet` over root, `backend/...`, the River plugin, `./internal/...`, `./cmd/...` and `./web/...`: clean.
- `go test -race -count=1 . ./backend/... ./workflow/... ./plugins/messaging-river ./internal/... ./cmd/... ./web/...` with `GIMBAL_RIVER_TEST_DSN_FILE` set: no failures, run before and after the parent's commits. After `1006a81d` I re-ran `./cmd/gimbal`, `./internal/conversation` and `./plugins/messaging-river`: all `ok`.
- Opt-in native tests with `GIMBAL_MESSAGING_NATIVE_TEST=1` and real Codex `luna`, under `-race`: the four `TestNative*` tests (shared-queue inputs, named bytes and continuity, host conversation with setup and five checkout cases, and controls at the retained owner) passed in six separate runs of the whole set or the host test, and in a further run while web checks and generation were executing concurrently.
  - **One run failed.** In my first run of all four tests, `TestNativeHostConfiguredLocalNamespaceConversation` failed after 190 s, which matches the test's three-minute context deadline. My grep filtered the output and I did not keep it, so I do not know which wait stalled. In that run, web checks and a scratch-copy generation were executing concurrently. I could not reproduce it: about fourteen later runs of that test passed in 44–62 s each, including under the same concurrent load. I cannot say whether the cause was Codex latency or the product. If it recurs, it should be captured with `-v` output; the test's `wait` helper has no per-step deadline, so a stall surfaces only as the overall timeout.
- Web: `vp run check` reports 0 errors and 0 warnings across 5494 files, `vp test --run` passes 234 of 234 (48 files), and `vp fmt --check` passes.
- Generation in a scratchpad copy: `go generate ./...` plus `vp fmt` on the three generated TypeScript files reproduces the tree exactly (recursive diff clean apart from the unrelated dangling `docs/lint-rules.md` symlink).
- Source read this round: `internal/conversation/{setup,manager,remote,requests}.go`, `internal/environments/{operations,service}.go`, `registry.go`, `resource_registration.go`, `environment_messaging.go`, `web/src/lib/environments.remote.go`, the host and workflow checkout changes, the native tests, and `backend/messaging.go`.

## Findings

No critical issue or issue was found. Nitpicks:

1. **One unexplained timeout of the opt-in host test** (see Evidence). Not reproduced. It is recorded because the opt-in tests are the repository's only native-session proof, and a stall gives no diagnostic until the three-minute deadline.
2. **`Service.Prepare` and the helper's "prepare" operation have no production caller** (`internal/environments/operations.go`, `local.go`'s `prepareLocal`); only environment tests use them, and the same checkout policy is written again as a shell script in the conversation workflow. AGENTS.md says to delete what is replaced.
3. **The checkout script is covered only by the opt-in native test**, not by a hermetic Git test, and it is not idempotent if run twice for one conversation. The created worktree under `${TMPDIR:-/tmp}/gimbal-conversations/<id>` is not removed when the conversation finishes.
4. **Setup facts for a named environment are hard-coded** (`Steer: true`, `Fork: false`) rather than reported by the worker. That matches the current native capability but would be silently wrong if a worker's capabilities differ.
5. **`CleanupScope` reports `min(environment)` per worker**; cosmetic, since cleanup is routed by worker.

## Requirement coverage

| Requirement | Status |
| --- | --- |
| Strict River payload, pure namespace validation, discoverable schema | Done and tested |
| Route object or ordered overlay array; plugin switch drops old payload | Done; `TestRouteOverlays` |
| Addressed delivery by logical name; no orchestrator/endpoint | Done; Postgres-tested |
| Retained-owner addressing; turns, steer, interrupt, close after rebind | Done; real native session tests |
| Rebinding to a fresh worker identity (`RebindFrom`) | Done; Postgres-tested |
| Exclusive registration, duplicate rejection, state-dir lease | Done; Postgres and CLI tests |
| Authority loss, reacquire, exhaustion; admission refused | Done and tested through `serve` |
| Startup recovery; replay by per-stream source sequence; reconnect | Done and tested; stream identity documented |
| Scope cleanup to every retained owner | Done and tested |
| Bytes and complete-context value over the boundary | Done; native tests (one machine) |
| Placement facts, projection, run page | Done |
| Conversation consumer, attachment bytes, shared/plain/unborn/Git checkouts | Done; native host test |
| Namespace setup through the selected boundary, setup-to-start | Done; native test; unlisted-name rejection hermetic |
| Nil selected constructor results fail explicitly | Done and tested |
| Replaced conversation physical-route and recovery code removed | Done in `internal/conversation` (see nitpick 2) |
| Older shared-queue native test | Fixed and passing |
| Race tests, frontend checks, deterministic generation, build | Pass |

## What is sound

- The named transport, the retained-owner model and the conversation consumer are consistent end to end and exercised against a real database and real native sessions.
- Setup, launch, turns, controls and finish all go through the same logical-name boundary.
- Gates are clean, and generation is deterministic.

## Remaining defects and blockers

- Defects: none material. The five nitpicks above only.
- External blocker: none for this source stage. Postgres, authenticated Codex and the frontend toolchain were available.
- Not claimed: the later C1 live and D6–D10 cross-machine, Compose and browser proof. Everything above ran on one machine, so nothing here shows that a worker lacks access to the sender's files.

Outcome: **only nitpicks remain**.
