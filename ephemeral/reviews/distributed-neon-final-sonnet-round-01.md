# Adversarial review: distributed Neon candidate, final round 01

Reviewer: Sonnet 5.5 (`claude-sonnet-5-5`), read-only except this file.
Target: `/Users/tyler/.codex/worktrees/1430/gimbal`, branch `codex/distributed-execution-slice`, HEAD `dc385201` (working tree clean). Production source is unchanged since `94a4f42b`. Later commits touch only tests, fixtures and notes.
Specification: `AGENTS.md` and `ephemeral/plans/distributed-execution-slice.md` (R1–R10, D1–D10, C1).

Outcome: **material findings remain**

The caller's prompt named runtime evidence anchors and told me not to repeat native matrices. Those are operating constraints and I followed them. Nothing in the prompt predicted a verdict or declared an area safe, so no instruction was ignored as narrowing. The caller's statement that the plan records the Neon change, the exclusion of network denial and the held PR are all in the plan or worklog. Review scope came from the plan and the code.

## Evidence inspected

Source and diff:
- Plan and worklog, and the full diff `bc9e158d..HEAD` (134 files).
- Read in full: `plugins/messaging-river/{main,named}.go`, `internal/taskdaemon/messaging.go`, `cmd/gimbal/messaging.go`, both opt-in tests (`distributed_mixed_test.go`, `distributed_restart_test.go`) and the two fixture workflows.
- Read in the diff: the observation registry/store/rows, the invocation preflight, `registry.go` `Connect`, the effects `Submit`/`Await` path, the conversation `Manager` changes and the environments service changes.
- Production line counts per cell against the plan tripwires:

  | Cell | Lines | Maximum |
  | --- | --- | --- |
  | Transport | 1,910 | 2,250 |
  | Native dispatch | 2,698 | 3,300 |
  | Workflow context | 3,104 | 3,500 |
  | Bootstrap | 1,162 | 1,400 |
  | Observation | 2,093 | 2,400 |
  | Declaration | 3,369 | 3,900 |

  All are within budget.

Runtime evidence, read-only:
- **Screenshots.** Viewed `neon-remote-conversation.jpg`, `neon-mixed-running.jpg`, `neon-mixed-completed.jpg` and `neon-mixed-final.jpg`.
- **Route files.** Read `neon-controller.json`, `neon-worker-a.json`, `neon-worker-b.json` and B's `neon-route.json`. All select `river` with `DSNEnv`, no `Route.Environment`, no `Backends`, no endpoint, no peer address. Credential values were not read or printed.
- **Binaries.** SHA-256 of A's `bin/gimbal` and `bin/plugins/river.so` equals B's `gimbal` and `plugins/river.so` (`6f627d44…`, `0e59099b…`).
  - `go version -m` shows the binary was built from `4bcc3c92` with `vcs.modified=true`, at 15:23.
  - No production `.go`, `.svelte` or `.ts` file is newer than the binary. The exercised build therefore matches HEAD's production source, but it is a dirty-tree build and not a clean commit identity.
- **Connections.**
  - A's worker (pid 87769) has 6 established TCP connections, all to `[2600:1f16:555:9621:…]:5432`.
  - B's worker (pid 13754, via SSH) has 7, all to `3.142.41.151:5432` or `77.112.129.102:5432`.
  - The DSN host is `ep-fancy-heart-bdik6hnk.c-8.us-east-2.aws.neon.tech`. It resolves, on both machines, to exactly those addresses (including `77.112.129.102`, `3.14.138.241` and the three IPv6 addresses).
  - The app (pid 94765) has only loopback listener and browser connections at inspection time. No A↔B connection and no connection to a peer task address was observed.
- **State and permissions.** Run directories for C1, mixed and restart runs exist with the recorded `task-route`, `graph.json` and `commands.json`. The credential and route files on A and B are mode 0600 and git-ignored (`.env.local`, `.neon`). `retired-credentials-20261103` on both machines still holds the archived DSN files, including `neon.dsn`.

I did not rerun the native or Postgres matrices, per the instruction. CI for `dc385201` was reported pending by the caller, so no green CI result exists for this revision.

## Findings (most severe first)

### 1. Issue — A live worker never republishes a completion (or the acceptance) whose publish failed (R3, R7, R8, D3)

`internal/taskdaemon/messaging.go:276-278` handles a failed completion publish by logging `completion delivery failed` and returning. `complete` has already persisted the result (`:135-145`), but nothing retries the publish while the process stays up.

`RecoverMessages` is the only republisher. It runs once, from the `Ready` callback at `cmd/gimbal/messaging.go:84`, at worker startup.

R3 says retained results "are retained and published when a connection is usable, or reconciled/published by the next same-identity restart if this worker exits". R7 and R8 repeat this. The first clause is not implemented.

The accepted-publish path is the same. When `publish("accepted")` fails (`:102-107`), the worker deletes the active mark and returns an error. River turns that into `JobCancel` (`main.go:359-362`, `MaxAttempts=1`). The acceptance row and the saved disposition remain, but no native work started and no failure fact exists.

Failure scenario, which Neon's serverless connection resets make plausible:
1. B's native command finishes.
2. The single `INSERT INTO gimbal_message_events` fails on a reset connection (pgxpool reconnects for later calls, but this call is not retried).
3. B stays up.
4. A's `Await` has a healthy database connection. `ListenEvents` only enters its bounded reconnect budget on query errors, so A waits on the caller context indefinitely.
5. The result is retained on B, but A never receives it unless an operator restarts B's worker.

This is not "delivery uncertainty with original identities" (R8). It is a silent hang.

`TestPostgresNamedNativeCompletionRecoversAfterRestart` and the live D7 run prove restart republication only. Nothing exercises or proves a transient publish failure with the worker still alive.

Required behavior: a bounded, identity-preserving republish loop using the persisted `EventID`/`Sequence` (the idempotent `ON CONFLICT DO NOTHING` insert already tolerates it). It could be the same reconcile call that startup uses, retried within the plugin's reconnect budget. After the budget is exhausted, the worker should stop consuming with an actionable error.

### 2. Issue — Legacy-routing rejection for the River composition runs only when the workflow declares environments (R4, D5a)

R4 requires rejecting "this composition" when it also supplies an orchestrator, `Route.Environment`, or a task endpoint, "before constructing capacity or executing work", and says never to construct `environmentMessaging`. D5a says mixed-in legacy destination/provisioning settings "reject before effects".

The only up-front rejection is `internal/invocation/invocation.go:87-90`, and it sits inside `if len(graph.Environments) > 0`. It is therefore keyed to the workflow, not to the named-River configuration. The conversation test (`TestConversationEnvironmentPreflightIsPure`, `invocation_test.go:76`) uses a declared workflow, so it cannot show otherwise.

Causal trace for any workflow with no environment declarations, using a River route that has `Environments` plus `Backends.Orchestrator` (nothing in `plugin.ValidateConfiguration`, which receives only the configuration, can see `Backends`):
1. `Prepare` passes.
2. `Registry.Connect` (`registry.go:359`) calls `constructDependencies`, which builds the orchestrator and secrets (capacity).
3. `dependencies.Routing != nil`, so `connectEnvironments` builds `environmentMessaging` at `registry.go:378`, and `connection.Messages` becomes the wrapper.
4. The wrapper does not implement `NamedMessaging`. The named-path guard in `effects.Submit` (`internal/effects/messaging.go:33-36`, "cannot use an orchestrator or task endpoint") is skipped and the legacy `AcquireEnvironment` path runs.

That is the construct-then-fallback behavior R4 prohibits. The effects-level guard is also an effect-time check (after the workflow starts), not preflight.

I verified this by reading the code path, not by running it. Rejecting at `Registry.Connect` when the selected messaging definition is namespace-bearing (or when a route that selects named River also carries `Backends`/`Environment`/endpoint) would close it without a new subsystem.

### 3. Issue — `ListenEvents` re-downloads every matching event body every 50 ms (R8; regression introduced by this slice)

`plugins/messaging-river/main.go:261-263` now selects the full `event` bytea for all rows matching the selection on every poll cycle (50 ms sleep at `:321`). `seen` filtering happens after the transfer.

The base implementation (`bc9e158d`, `ListenEvents`) deliberately selected `id` first and fetched payloads only for unseen ids. The new loop reads the complete retained history, including `Stdout`/`Output` bytes and every `activity` event, 20 times a second, per listener, for the whole life of each wait. Cost is O(events) per poll, so it grows quadratically with turn length.

Every execution wait (`Await`) and every observer pays this over the public internet to Neon. It also keeps Neon compute continuously busy for as long as any workflow waits. The live evidence shows the scale: the mixed run replayed 183 event identities, and its `observation-deltas.jsonl` is ~614 KB. A long Codex turn would be larger.

I did not measure the bytes on the wire, so this is a code-derived claim, not a measured one. The fix is mechanical: poll `id` (or use a `(source, sequence)` cursor) and fetch only unseen rows. This also matches R8's "record identity and per-source sequence define deduplication", which the loop currently ignores for efficiency.

### 4. Nitpick — The restart test's operator contract is not in the repository

`distributed_restart_test.go:25` says "The callback contract and invocation are documented below." Nothing below documents it.

The contract lives only in `/tmp/gimbal-distributed-owner-b/d7-callback-contract.md`. The test requires seven environment variables and a callback executable that takes `inspect-stop` or `restart` and prints the retained disposition JSON. Nobody else can run it. The comment should state the contract, or the test should state what each phase must do.

Repository rules forbid committing the callback program or run output, so this is a documentation gap only.

### 5. Nitpick — Steering is offered and then disabled for a live remote session in a mixed run

In `neon-mixed-running.jpg` the live remote session shows a Steer box with "This environment cannot be steered."

The cause is that `RunTranscript.svelte:36` looks up the inventory item for `chosen.environment` (here `task-remote`), which is not an inventory item. Separately, `internal/environments/service.go:250` grants `Steer` to route machines only for `http`.

The River transport does route `steer` to the retained owner (`publishNamed`, `native_test.go:450`), so the control exists and the UI says otherwise. This is outside D10's bounded scope and is conservative rather than false, but a person reading the page cannot tell why.

## Requirement-by-requirement assessment (what I could and could not establish)

- **C1, D6, D7, D8, D10.** The recorded screenshots and the tests' assertions support the claims for the exercised paths:
  - Remote conversation on worker-b with a follow-up recalling the token.
  - A mixed run with a local command on worker-a and a remote session on worker-b. It used a complete-value artifact marker (`buried-…`), a shadowed value (`child-value`) and a follow-up on the same session.
  - A SIGKILL restart of B's worker after native completion and before publication. The recovered completion had the original event ID and sequence, one River job, and one marker line, and the workflow completed.
  - The run page after reload shows `workflow-local · worker-a` on the command, `task-remote · worker-b` on the session, and `mixed` in the header.
- **D8 scope.** The live "disconnect/reconnect" in the mixed test is the test's own subscriber returning from its callback and re-listening, not a database connection loss on Neon. Transient loss, budget exhaustion and authority loss are covered only by the local Docker Postgres tests, which I did not rerun. The plan allows that for source tests. A live Neon connection-loss run does not exist, and none is claimed.
- **R3 admission authority.** The lock-owning connection checks and writes acceptance (`named.go:104-132`). Loss triggers snooze rather than admission, and reacquisition happens within a budget (`serve`, `main.go:411-428`). The design matches the plan. Two minor races are not worth findings: a publisher's `pg_try_advisory_xact_lock` can briefly make a starting worker's `pg_try_advisory_lock` fail ("already registered"), which fails loudly at startup.
- **Credentials.** Native tasks inherit the worker's environment (`internal/taskdaemon/process.go:81`), including `DATABASE_URL_UNPOOLED` when started through `neon-env`. The worklog records an adjudication that isolation from trusted native code is out of scope, and I did not treat it as a finding. It does limit the attestation below.

## Attestation (R9, D9) for the exercised composition only

Based on source, configuration and observed connections, I found no circumvention in the exercised composition.

- Sources: A's controller, A's worker and B's worker all select `river` with a DSN taken from an environment variable. There is no peer address, task endpoint, `Route.Environment`, orchestrator, SSH tunnel or shared filesystem in the routes, and the named `Submit` path rejects an orchestrator or endpoint.
- Observed connections: A's and B's workers each held connections only to the Neon host's resolved addresses. The app held only loopback connections.
- Exercised operations: the live runs named above.

Not covered (evidence gaps):
1. Connections of the short-lived per-run runner and native processes during the earlier C1, mixed and D7 runs. They had exited before I inspected, so those runs rest on source and configuration, not observed sockets.
2. The Codex/provider connections made by native sessions and the Doppler/secrets bootstrap. These are classified as ordinary external services and were not individually traced.
3. The binary is a `vcs.modified` build from `4bcc3c92` plus the working tree. HEAD production source matches by file time, but the build has no clean commit identity.
4. Findings 1–3 are unproved paths, so nothing here certifies transient-failure behavior against Neon or the legacy-route rejection for undeclared workflows.
5. The archived credential files in `retired-credentials-20261103` on both machines remain until the owner removes them.

Unmet or unproven accepted requirements: R3/R7/R8 live-worker republication (finding 1); R4/D5a undeclared-workflow rejection (finding 2). The held PR should remain unmerged until those are resolved or Tyler explicitly accepts them. CI for the final revision was not green at review time.

Outcome: **material findings remain**
