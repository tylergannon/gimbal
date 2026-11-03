# Adversarial review: distributed Neon candidate, final round 02

Reviewer: Sonnet 5.5 (`claude-sonnet-5-5`), read-only except this file.
Target: `/Users/tyler/.codex/worktrees/1430/gimbal`, branch `codex/distributed-execution-slice`.
- Production source: `4851d4ef` (pushed). It differs from round 01's `dc385201` in `cmd/gimbal/messaging.go`, `internal/invocation/invocation.go`, `internal/taskdaemon/messaging.go`, `messaging_registration.go`, `plugins/messaging-river/main.go` and `registry.go`.
- Later commit `8ee5c846` changes only `package.json`, `package-lock.json` and one worklog note. I verified with `git diff --name-only` that it touches no `.go`, `.svelte` or `.ts` file.
- Specification: `AGENTS.md` and `ephemeral/plans/distributed-execution-slice.md` (R1–R10, D1–D10, C1). Round 01 is retained and was read.

Outcome: **only nitpicks remain**

## Operating constraints

The caller's prompt gave an artifact path, a read-only boundary, an authorization list and an instruction not to infer fresh proof from old runs. Those are valid operating constraints. It named no defects, predicted no verdict and declared no area safe. Nothing was ignored as narrowing. Scope came from the plan, `AGENTS.md` and the code. While I worked, the caller sent facts about the clean build, hashes, the fresh app URL, run IDs and the startup-script change. I verified each against the filesystem or processes where I could. Where I could not, I say so.

## Evidence inspected

Source:
- Full diff `dc385201..4851d4ef` for production files. I read the changed regions in context:
  - `plugins/messaging-river/main.go`: `PublishEvent`, `ListenEvents`, `serve`, `actionWorker.Work`, `Register`.
  - `plugins/messaging-river/named.go`: read in full.
  - `internal/taskdaemon/messaging.go`: `RecoverMessages`, `WaitMessages`, the `Messages` handler.
  - `cmd/gimbal/messaging.go`: the `message-worker` command.
  - `registry.go` `Connect` and `resource_registration.go` `constructDependencies`.
  - `internal/invocation/invocation.go` `PrepareContext` and `internal/runner/runner.go` `Run`.
  - `internal/effects/messaging.go` `Await`.
  - `internal/observation/registry.go` `decorate`.
- The new tests: `TestNamedRouteRejectsLegacyWithoutDeclarations`, `TestWaitMessagesPreservesAdmittedWork` and the four `TestPostgres…` repair tests in `plugins/messaging-river/main_test.go`.
- `git diff 4851d4ef..8ee5c846` for the startup script.

Commands I ran (build and unit tests only, no services):
- `go test ./internal/invocation -run 'NamedRoute|Preflight'` passed.
- `go test ./internal/taskdaemon -run WaitMessages` passed.
- `go build ./...` fails only for `plugins/*` ("function main is undeclared"). That is the expected `-buildmode=plugin` condition and is not a regression.

I did not run the Postgres-backed tests. They skip without `GIMBAL_RIVER_TEST_DSN_FILE`, and I had no authorized DSN for the owned Docker database. `.github/` references no such variable, so CI run 41727782816 (passed on `4851d4ef`, as reported by the caller) cannot have executed them. I did not see the output of the local runs of the repair tests. Their pass status is the caller's assertion, not something I observed.

Runtime evidence, read-only:
- **Binary identity.**
  - A's `bin/gimbal` is `64b36117…14698` and `bin/plugins/river.so` is `a296654a…c1e9b1`. These equal the hashes the caller sent.
  - `go version -m bin/gimbal` shows `vcs.revision=4851d4ef012fd63a236a25ad86f373d0ede6a646` with no `vcs.modified` line, meaning a clean-tree build. This closes round 01's dirty-build gap.
  - B's `gimbal` is `64b36117…` and B's `plugins/river.so` is `a296654a…`, so A and B run the same binaries.
- **Routes.**
  - `neon-worker-a.json` and `neon-worker-b.json` select `river` with `DSNEnv=DATABASE_URL_UNPOOLED`, one shared queue (`gimbal_neon_01a19a10`), `Environments {workflow-local: worker-a, task-remote: worker-b}` and the worker's own `WorkerID`.
  - The mixed run's retained `task-route` has the same shape.
  - None carries `Environment`, `Backends` or a task endpoint.
  - I read no credential value.
- **Observed connections.** I took snapshots at 16:10:54–57 during the mixed run, again at about 16:12, and a read-only lookup of the app afterwards.
  - Worker A (pid 15197): only `:5432` connections to the Neon address `2600:1f16:555:9621:c1fd:73cf:753e:5e0`. Its local source address is `2606:4700:cf1:1000::1`, which looks like a tunnel or VPN interface. That is the machine's own route to Neon and not a peer.
  - App (pid 16659, listening on `127.0.0.1:57430`): one `:5432` connection to the same Neon address and, at 16:10:54, one `:443` connection to `2606:4700:3108::ac42:2bca`. That connection was gone 20 seconds later.
  - Worker B (pid 22516, then pid 24812 after the controlled restart; both run the `64b36117` binary): `:5432` connections only to `3.14.138.241`, `77.112.129.102` and `3.142.41.151`. These are Neon's resolved addresses. At about 16:12 pid 24812 also held one `:443` connection to `172.66.43.202`.
  - `dig` resolves `api.doppler.com` to `172.66.43.202` and `2606:4700:3108::ac42:2bca`. `cmd/gimbal/secrets.go` `loadCommandSecrets` fetches `https://api.doppler.com/...` for any command not on its exclusion list when `DOPPLER_TOKEN` is present. `message-worker` is not on that list, and the app is started the same way. I therefore attribute those `:443` connections to the existing Doppler secrets bootstrap. This is an inference from the address match and the code path, not from payload inspection. I observed no connection between A and B and none to a peer task address.
- **Fresh mixed run `01M6HQCZJ7QGS6XD2Y09PZNR3M`.** From the retained run directory:
  - `commands.json`: `local-command.1` has `environment=workflow-local`, `worker_id=worker-a`, exit 0, and stdout is host `Esmeralda.lan` followed by two UUIDs.
  - `sessions.json`: `remote.1/agentic-dialogues.1` has `environment=task-remote`, `worker_id=worker-b`, model `gpt-6-luna`, effort `high`, workdir `/private/tmp/gimbal-dist-01a19a10.sK9NHy/work`.
  - `turns.json`: two turns, no error, none unresolved. Turn 1's result is five lines: `Amritanandamayi.local`, A's first UUID, `buried-D797782F-…` (the second UUID from A, which was held as the large-value context), `file-89035A78-…` and `child-value`. That matches the task file and the shadowed value.
  - `run.json` status is `completed`.
- **Fresh D7 run `01M6HQES4FG7PQG3DAE6HR2ZC6`.** `commands.json` shows `write-marker.1`, `verify-marker.1` and `remove-marker.1` on `task-remote · worker-b`. The marker write printed `…\n1\n`. Request and event IDs and the test's pass/fail output are the caller's report. I did not see the test log.
- **Screenshot `/tmp/gimbal-distributed-owner-b/neon-mixed-4851d4ef.jpg`** (1280×720, viewed). It is the mixed run after reload.
  - The header chip reads `mixed`.
  - The command row reads `local-command  workflow-local · worker-a  exit 0`.
  - The session's detail chip reads `task-remote · worker-b`, next to `gpt-6-luna` and `high`.
  - The Invocation card says `where mixed · /private/tmp/…/work`.
  - Text is legible at that size, and both placements are visible.
  - `observation.json` holds an empty `task_environment`. The `mixed` label comes from `Registry.decorate` (`internal/observation/registry.go:211`), which derives it from retained per-task rows on read.

## Status of round 01's findings

1. **Live worker never republishes a failed publish: closed in source.**
   - `PublishEvent` (`plugins/messaging-river/main.go:242`) now retries the same event ID and sequence with `ON CONFLICT DO NOTHING` until the plugin's `ReconnectSeconds` budget ends. The default is 30 seconds. It stops at once for a caller cancellation or a permanent error.
   - On exhaustion it returns an error naming the request and event, and signals `PublicationFailure`.
   - `cmd/gimbal/messaging.go:104-126` takes that signal from the same `connection.Messages` the daemon publishes through, cancels the worker context, prints the error, waits for admitted native work to retain its outcome, and exits non-zero. Restart reconciliation republishes using the original identities.
   - `Registry.Connect` returns the plugin's `*transport` unwrapped when no routing is configured, so the `PublicationFailure()` type assertion at `cmd/gimbal/messaging.go:107` succeeds for a named route.
   - The behaviour is covered by `TestPostgresPublicationRetriesOriginalEvent`, `TestPostgresLiveWorkerRepublishesCompletion` and `TestPostgresPublicationTerminalErrors`. I did not see them run (see the evidence notes above). They are plausible by reading: a trigger fails the first completed-event insert with SQLSTATE 08006 and the test asserts one native execution and one stored event.
2. **Legacy routing rejected only when declarations exist: closed in source.**
   - `NamedEnvironments` is a new pure plugin callback.
   - It is checked in `Registry.Connect` (`registry.go:356`), before `constructDependencies`, so no orchestrator or secrets capacity is built first.
   - It is also checked in `invocation.PrepareContext`, before the declaration branch, and in `message-worker` before the state lock.
   - `serve` rejects `Orchestrator` or `Routing` for a named worker.
   - `TestNamedRouteRejectsLegacyWithoutDeclarations` registers a workflow with no declarations and asserts that `Environment`, an orchestrator selection and a task endpoint are each rejected in `Prepare`, and that `Connect` rejects the first two before the plugin's `Connect` is called. I ran this test and it passes.
   - The runner calls `Prepare` before it opens a connection (`internal/runner/runner.go:37`).
3. **`ListenEvents` re-downloads every event body every 50 ms: closed in source.** Each poll now selects only ids and fetches `event` payloads for ids not yet `seen`. `TestPostgresListenerDoesNotRefetchSeenPayload` corrupts an already-delivered row and asserts that the listener still delivers the next event. I did not see it run.
4. **Restart-test contract undocumented: closed.** The doc comment on `TestDistributedRestartOwnedParticipant` now states the environment variables and both callback phases.
5. **Steering affordance in a mixed run: unchanged.** It was an out-of-scope presentation limit in round 01. The fresh screenshot is taken after the run ended, so it does not show the steer box. It stays out of D10's bounded scope.

## Findings (most severe first)

### 1. Nitpick — Any PostgreSQL error outside class 08 and `57P01–03` is treated as permanent, and that is now terminal for a worker

`permanentDatabaseError` (`plugins/messaging-river/main.go:237`) classifies every `*pgconn.PgError` as permanent unless its code starts with `08` or is `57P01`, `57P02` or `57P03`. Classes that routinely signal a transient condition on a hosted, autoscaling Postgres therefore end retries at once:
- `53xxx` (for example `53300 too_many_connections`)
- `57014 query_canceled`
- `40001` and `40P01`

Before the repair this classification only ended a listener. Now the same predicate makes `PublishEvent` return immediately and signal the worker to stop consuming (`t.fatal`, line 278).

Failure scenario: Neon rejects one insert with `53300` while a Codex turn is mid-flight. The worker stops consuming and enters the wait described in finding 2, although a retry 100 ms later would have succeeded. No result is lost, because the completion is retained and restart republishes it. The cost is availability and an unnecessary operator restart, so this is a nitpick rather than a defect.

An allowlist of known-permanent classes (`28xxx` authentication, `42xxx` schema and privilege, `3Dxxx`) would match the plan's wording ("permanent authentication/configuration/schema errors") more closely.

### 2. Nitpick — After a publication failure the process lingers, holds the state lock and cannot republish, and the new command wiring has no test

`cmd/gimbal/messaging.go:117-125` cancels the worker context, then calls `daemon.WaitMessages(ctx)` (`internal/taskdaemon/messaging.go:46`), which blocks until every admitted request has retained an outcome.

While it waits:
- The worker has already released database authority, because `serve` returned, and stopped consuming.
- Its publishes are cancelled, because they use the cancelled worker context.
- It still holds the executor state-directory lock (`defer lease.Close()`, line 66).

A Codex turn that runs for minutes therefore keeps the process alive. During that time a same-identity restart, which is the republication path R3 and R7 designate, fails with "executor state directory already owned" until the operator sends SIGTERM. SIGTERM cancels the wait and the process exits.

I could not establish what SIGTERM does to the separate `--gimbal-agent-worker` child. D7's SIGKILL specimen indicates native results survive worker loss, but that is not the same code path.

The requirement text is "stops consuming and exits with an actionable error". The actionable error is printed before the wait, but the exit is deferred.

Separately, only `WaitMessages` (a unit test) and the transport's signalling are tested. The goroutine, ordering and exit behaviour in the command have no test and were not exercised in the fresh live runs. By reading, `failure` is filled before `stopWorker()`, so the post-`Worker` `select` sees it. That is source evidence only. The plan permits opt-in tests, so this is a gap and not a violation.

### 3. Nitpick — Activity events are best-effort but share the fatal path, and a lost one leaves an unrecoverable sequence hole

`publish("activity", …)` (`internal/taskdaemon/messaging.go:203`) persists the incremented `Sequence` before it publishes, and on error only logs. Neither the event nor its intent is retained for retry or restart. If the database is unavailable beyond the budget:
- That activity event is permanently absent from replay, while later events carry a higher sequence, so there is a silent gap in the per-source order.
- The same failure triggers the worker-fatal signal used for outcome events.

The plan's R8 asks retained observations to be "retrievable"; activity loss after the retry budget is outside that. Short blips are covered by the in-call retry, so this matters only for outages longer than the budget. It is a nitpick: an observation event should not stop consumption, and a gap in the source order should be detectable by a listener.

### 4. Nitpick — The unpooled-DSN requirement behind the registration lock is unenforced and undocumented in the plugin

Worker authority uses a session advisory lock held on a dedicated connection (`named.go:41`, `reacquire`). The worklog records that `DATABASE_URL_UNPOOLED` is required, and the live routes name it. Nothing in the plugin schema, in `parse`, or in the `message-worker` help says so. Pointing `DSNEnv` at Neon's pooled variable would silently weaken the exclusive-registration guarantee, because a transaction-pooling proxy does not preserve session locks. A one-sentence note in the `DSNEnv` description would close this. Detecting a pooler host is a heuristic and is not needed.

### 5. Nitpick — The new `start` script tolerates a missing env file and states no Node floor

`package.json:5` uses `--env-file-if-exists` for both `.env` and `.env.local`. A missing or misnamed `.env.local` therefore starts the app with no database variables, and the failure appears later as a DSN error from the first river call instead of at startup. `process.execve` also needs a recent Node, and `package.json` has no `engines` field. CI pins Node 24, and local Node 26.13 supports it.

The change is startup-only and removes the unused `@neon/env` dependency. It does not alter production Go or the exercised composition. It is still a tracked file changed after the proved source commit.

## Requirement-by-requirement assessment

- **R1–R2, D1–D2.** No change since round 01. The new `NamedEnvironments` callback keeps preflight pure: `parse` reads no DSN or file.
- **R3, D3.**
  - Authority loss and reacquisition are unchanged and match the plan.
  - Registration, rebind and acceptance are atomic on the lock-owning connection.
  - The new publication path honours "retained and published when a connection is usable, or reconciled by the next same-identity restart", with the process-lingering caveat in finding 2.
- **R4, D5a.** The named-route rejection is now keyed to the configuration, not to the workflow. I found no remaining path that builds a `Routing` wrapper for a named route. `environment_messaging.go` is reached only when `dependencies.Routing != nil`, which needs an orchestrator.
- **R5–R8, D6–D8.** The fresh mixed and D7 run records support the claims for the exercised paths: a real local command on A, a real remote Luna session on B reading the transferred values, a follow-up turn on the same session, and a completed-result restart. The caller reports 166 replayed identities with exact byte checks and an original event recovered after the worker restart. I did not verify the byte checks or the test log myself.
- **D8 scope.** The live "disconnect and reconnect" is still the test subscriber re-listening, not a Neon connection loss. Transient loss, budget exhaustion and authority loss are covered only by the local Docker tests, as the plan allows. No live Neon loss run exists and none is claimed.
- **R10, D10.** The screenshot shows distinct per-task environments and workers on the command and session and the `mixed` header after reload. The run page legibly shows both placements at the viewed size.
- **C1.** Not re-exercised in this round. Round 01's evidence stands for `dc385201`, and no conversation-path code changed since.
- **Volume tripwires.** Not recounted. The repair adds about 100 production lines, mostly in the transport cell, which round 01 measured at 1,910 of 2,250. It is not near the limit.

## Attestation (R9, D9) for the exercised composition only

Based on source, configuration and observed connections, I found no circumvention in the exercised composition.

Exact identities:
- Source `4851d4ef012fd63a236a25ad86f373d0ede6a646`, clean build, identical binaries on A and B.
- `gimbal` is `64b36117e51ebebc7f7e93d9031659964efd5361b8e22db2ad243d24f2f14698`.
- `river.so` is `a296654a319b96033cad2cbdd8be873ddad547f95c32c812c83ca17a29c1e9b1`.
- The app was started through the `8ee5c846` script. It adds no Go, `.svelte` or `.ts` change.

Inspected paths:
- Route files for A's controller, worker A and worker B.
- The mixed run's retained route.
- `Registry.Connect`, `PrepareContext`, `serve`, `publishNamed` and `message-worker`.

Exercised operations: the fresh mixed run and the fresh D7 run named above.

Allowed external services observed:
- Neon PostgreSQL (the configured database host).
- Doppler's API, inferred from the address match and the `loadCommandSecrets` code path, for the app and B's worker.
- Provider traffic from native sessions was not individually traced.

Evidence gaps:
1. **Postgres-backed repair tests.** I did not run them, and CI skips them. Their pass status rests on the caller's report.
2. **Observation window.** The connection snapshots cover only a part of the mixed run. They missed the Go test process that hosts the workflow body on A and any per-run runner process. They also did not show B's native Codex turn sockets. Provider connections made through the Codex app-server processes on B (pids 14502 and 14793, `:443`) were seen but not traced. The transient `:443` connection from the app is unattributed except by the Doppler address match.
3. **D7 execution count.** The single native execution and the recovered event identity are the caller's report. I saw only the marker's one-line content.
4. **Old log on B.** `/private/tmp/gimbal-dist-01a19a10.sK9NHy/worker.log` still contains an older connection error to `192.168.50.130:55004` (the replaced Docker database). It is a stale log from the earlier composition. B's current worker log is `neon-restart-worker.log`, which I did not read.
5. **Retired credential files.** The archived credential files under `retired-credentials-20261103` on both machines remain until the owner removes them.
6. **Unproven paths.** Nothing here certifies Neon connection-loss behaviour, authority loss on Neon, or the conversation path at `4851d4ef`.
7. **CI.** CI for `8ee5c846` was running at the time of the caller's last report.

The held PR should stay unmerged under the plan's release hold. The remaining whole-application gate is outside this slice.

Outcome: **only nitpicks remain**
