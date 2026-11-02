# Adversarial review: distributed-execution first-slice task definition, round 01

Reviewer session: `b0717a8e-4ad5-4238-9167-addc6015bc52` (Claude Fable 5.1), November 1, 2026.
Read-only review; the only file written is this one.

## Target

`ephemeral/plans/distributed-execution-slice.md` (untracked draft, status DRAFT, not approved
for implementation), reviewed against its authoritative sources at the inspected baseline
`a3ee508b8e7a1582971ce03170e5695fadedf118`, which is also the worktree HEAD.

Caller narrowing: none ignored. The launch prompt described the draft's content and set a
read-only/artifact-path constraint; it did not limit the defects, files or verdict. Scope below
is derived from the plan, Tyler's recorded decisions, repository instructions and current source.

## Evidence inspected

Authoritative inputs (all local):

- `/Users/tyler/src/gimbal-view/ephemeral/distributed-execution/interface-migration-proposal.md`,
  `current-architecture.md`, `backend-condition-and-configuration.md`, `transition-plan.md`,
  `target-architecture.md` (first sections).
- `/Users/tyler/src/gimbal-view/ephemeral/issues/architecture-seam-merge-gate.md`.
- `/Users/tyler/src/gimbal-view/ephemeral/worklog/20261031-distributed-architecture.md` (Tyler's
  recorded decisions) and `/Users/tyler/.codex/worktrees/oct31-work-recap/gimbal-view/ephemeral/worklog/20261101-work-recap.md`.
- Repository `AGENTS.md`, `/Users/tyler/.codex/AGENTS.md`, `docs/definition-of-done.md`,
  `skills/gimbal/SKILL.md`, and `ephemeral/worklog/distributed-execution-slice.md` (the draft's own
  worklog, including its correction about native recovery).

Source read in full or in the relevant parts: every file in the plan's cell map (`env.go`,
`registry.go`, `messaging_registration.go`, `resource_registration.go`, `workflow/graph.go`,
`workflow/description.go`, `internal/generate/declarations.go`, `environment_messaging.go`,
`plugins/messaging-river/main.go`, `internal/effects/{messaging,identity,services}.go`,
`internal/taskdaemon/{messaging,daemon}.go`, `session.go`, `task_files.go`, `command.go`), plus the
surrounding code the requirements depend on: `backend/{backend,messaging,orchestration}.go`,
`backend/localorchestrator/orchestrator.go`, `internal/effects/{client,connection,task_files}.go`,
`internal/taskdaemon/{bytes,retention,persistence,local,service}.go`, `internal/invocation/{invocation,backends}.go`,
`internal/runner/runner.go`, `internal/host/routes.go`, `cmd/gimbal/{messaging,daemon,workflows}.go`,
`cmd/gimbal-worker/{worker,environment}.go`, `remote_execution.go`, `artifact.go`, `scope.go`,
`plugins/messaging-filesystem/main.go`, and the test inventories of the River plugin, taskdaemon
messaging, environment routing and effects.

Checks run:

- `wc -l` over each cell's listed files: Declaration 914, Transport 509, Native dispatch 1,053,
  Context 1,572. All four baselines in the plan are correct.
- `git diff --stat 2f67be83..a3ee508b`: tests, e2e, changelog and ephemeral notes only. The plan's
  claim that nothing since the proposal baseline touches distributed runtime is correct.
- `go build . ./cmd/... ./internal/... ./backend/...` succeeds. (`go build ./...` fails only
  because plugin `main` packages need `-buildmode=plugin`.)
- `go test ./internal/taskdaemon -run 'TestRetainedNativeSuccessSurvivesNormalWorkflowDeliveryLoss|TestMessagingSessionSurvivesTwoCompletedTurns'`
  and `go test . -run 'TestEnvironmentRouter|TestParentCleanup'` pass at baseline.

Verified and **not** findings: the complete-value artifact transfer the plan relies on for R5/D5
exists today (`scope.spillValueLocked` writes under `artifacts/values/...` via `run.artifactName`;
`remoteTaskFiles` in `task_files.go` collects `Complete value:` references under `artifacts/` and
ships bytes in `Turn.Files`; `taskdaemon.turnEvents` rewrites the marker to the native path).
The plan's rendering of Tyler's decisions (network denial removed permanently, attestation
instead, one worker per name, ordinary context, no sync/cleanup/scheduler, approval gate,
#664 hold, single chat) matches the worklogs. The exclusions list is consistent with the
recorded decisions.

## Findings

### F1. Critical — the orchestration seam is unaddressed, and current code refuses named environments without an orchestrator that provisions local subprocesses

**Requirement misunderstood/omitted.** R1–R4 and D1/D2 describe logical placement resolved by
"plugin configuration" and transport-owned namespace, but the plan never says what becomes of
`backend.OrchestratorBackend.AcquireEnvironment/Release`, `backend.EnvironmentRouting`, the
`local` orchestrator, or `environment_messaging.go` (which it lists as a Transport file to be
*extended*, not deleted). The design input it cites resolves this explicitly (proposal §2:
"Replace this application-facing route contract with logical-name and retained-owner addressing
inside the selected transport"; "Change `OrchestratorDefinition.Construct` to receive that
selected transport's narrow provisioning/registration capability"). The plan dropped it.

**Why it blocks the slice as written.** Every current path to a named environment requires an
orchestrator with routing, and the only orchestrator spawns a worker *on the caller's machine*:

- `registry.go:365-367`: `Connect` rejects `route.Environment != ""` unless
  `dependencies.Routing != nil`; `resource_registration.go:97-101` only obtains `Routing` from an
  orchestrator that implements `EnvironmentRouting`.
- `internal/runner/runner.go:49-52`: `named task environments require orchestration and messaging`.
- `internal/effects/messaging.go:30-37`: `Submit` calls `Orchestrator.AcquireEnvironment` for
  every `identified-session`/`identified-process`, and errors `named environment %q requires an
  orchestrator backend` when there is none.
- `backend/localorchestrator/orchestrator.go` `AcquireEnvironment`: `exec.Command(definition.Command...)`
  with `--ready-file`/`--secrets-config-stdin`, liveness via `syscall.Kill(pid, 0)`, receipts in a
  local directory. None of this can represent a worker that a human started on computer B.
- `environment_messaging.go`: constructs one plugin connection per `EnvironmentDestination.Route`
  and polls `routing.ActiveEnvironments` every 20 ms — the per-peer connection construction R4
  forbids, in the file the plan keeps.

So on computer A, with B's `message-worker` started independently, the specimen in D6 cannot be
admitted today: either `Connect` fails for lack of routing or `Submit` tries to acquire an
environment nobody provisions. The plan's own "any change outside this map is a spec-defect
intake before coding" clause would fire on the first design step, and the Transport baseline
(509 lines, 192 of which are `environment_messaging.go`) is computed over code the design input
says to replace. `backend/orchestration.go` and `backend/localorchestrator/` are in no cell.

**Needed before approval.** State the orchestration disposition for this slice: whether
`AcquireEnvironment`/`Routing` are removed from the named-environment path in favour of
plugin-owned logical addressing (and `environment_messaging.go`/`local` orchestrator deleted per
"No wrappers / delete what is replaced"), or what minimal orchestrator stands in for a remotely
started worker. Add the owning files to the map and recompute the Transport baseline.

### F2. Critical — D7 requires a retained native result to reach A after a worker restart, but no described mechanism publishes it, and the current code has none

**Requirement incomplete.** R7/D7: "Worker restart after native completion but before return to
A recovers that result under its original identity, with one native execution." The plan's
"Central semantics" `on_worker_restart` block has load → reconcile → report-unresolved, and never
"publish the retained outcome for accepted requests whose completion was not delivered". No cell
contract names that responsibility (Transport "cannot decide native success"; Native dispatch
"reconcile original native outcomes" — reconcile with whom?).

**Why the current code cannot satisfy D7 as described.**

- `internal/taskdaemon/messaging.go`: the only completion publisher is the goroutine
  `go func() { <-t.done; … complete(result, nativeErr) }` created *inside* a handler invocation.
- `internal/taskdaemon/persistence.go` `restore()`: loads `d.messages` dispositions and `d.tasks`
  (closing `t.done` for tasks with `Completed` set, re-waiting executors otherwise) but publishes
  nothing. `d.messages` is read only by the handler (`messaging.go:32,56`).
- `plugins/messaging-river/main.go`: `PublishRequest` inserts with `MaxAttempts: 1`;
  `actionWorker.Work` returns `nil` once the handler returns, which completes the job. After the
  worker restarts there is no redelivery, so the handler never runs again for that request.
- A is blocked in `effects.Await` → `ListenEvents{RequestID}` waiting for `completed`; it ends only
  at ctx deadline as `DeliveryUncertain`.

The only existing path that would re-present the request to the handler is A re-submitting the
same identity through same-run re-entry (`gimbal.Run`/`loadReplay` on the workflow runner), which
R7 explicitly excludes ("does not promise … replacing a lost workflow runner"). The draft's own
worklog (line 9) flags River "reconnect/redelivery/publication behavior requires inspection"; this
review resolves it: there is no publication path. `disposition.EventID` already gives a stable
completion event id, so daemon-side republication on restart is implementable inside the mapped
Native-dispatch files, but the plan must say so (and whether A must also tolerate an
`accepted`/`completed` arriving after reconnect) or an implementer can satisfy "reconcile" locally
while A hangs, and a reviewer reading the semantics block would pass it.

### F3. Issue — the files that define the plan's own seams are outside every cell, and R8 provably requires changing one of them

The plan says "The registered catalog, dispatch envelopes, native admission state and retained
record/byte references are the seams", yet `MessageRequest`, `MessageEvent`, `MessageSelection`
and `Messaging.ListenEvents(…, since time.Time, …)` live in `backend/messaging.go`; `Route.Environment`
in `backend/backend.go`; `EnvironmentRouting`/`Dependencies` in `backend/orchestration.go`. None is
in the cell map, and the deferred-tripwire note anticipates only "bootstrap, registry generation
or native state persistence" as likely out-of-map changes.

R8 makes at least one such change unavoidable: "Record identity/deduplication cannot rely only on
synchronized wall clocks." Today `effects.Await` (`internal/effects/messaging.go:67`) passes A's
`r.Created` as `since`; River `ListenEvents` filters `created >= $2` where `created` is stamped by
the worker's `time.Now().UTC()` (`taskdaemon/messaging.go` publish closures;
`plugins/messaging-river/main.go` `PublishEvent`/`ListenEvents`). On two real computers with B's
clock behind A's by more than the publish latency, A never sees `accepted`/`completed` and the
turn ends `DeliveryUncertain`. The single-machine tests cannot expose this. Fixing it changes the
listener cursor contract in `backend/messaging.go`. Likewise R3's "duplicate active registration is
rejected" needs a registration concept absent from `MessagingDefinition.Worker`/`backend.Dependencies`
(River `serve` has no worker identity at all), and R6's "distinct facts for request storage,
native-owner acceptance and native completion" are the event `Kind` vocabulary in the same file.

Impact: the budgets exclude the files most likely to change, and the "spec-defect intake before
coding" clause will fire on known work. Either add the contract files to the Transport/Native
cells with recomputed baselines, or state that envelope/listener contract changes are in scope
for Transport without intake.

### F4. Issue — the remote session's working directory is unspecified, and the messaging participant has no way to set one

D6 needs "a real session on B". Where on B? `taskdaemon.create` calls
`a.CreateSession(ctx, req.Model, req.Effort, d.workdir(req.Workdir))`; `service.go:3-8` returns the
*requested* path unless `d.Workdir` is set. `cmd/gimbal/messaging.go` `message-worker` exposes
`--executor-state-dir`, `--ready-file`, `--secrets-config-stdin`, `--task-route`, `--plugin-dir` and
never sets `daemon.Workdir` (only the HTTP `gimbal-worker` does). The requested path is whatever
the workflow source passed to `NewSession(ctx, role, workdir)` on A, and `runCommand` additionally
bakes A-side `filepath.Abs(workdir)` into replay intent (`command.go`, flagged in proposal §4).

So D6 passes only if the specimen hard-codes an absolute path valid on B in authored workflow
code — the kind of machine binding R1 and the target architecture keep out of workflow source — or
if the implementer adds an unlisted worker option. The proposal's review synthesis recorded
"native directory semantics" as a required clarification; the plan omits it. Decide and record:
owner-side working directory (per-worker configuration) versus author-supplied path, and what the
intent record stores.

### F5. Issue — "maintained example code" for the specimen conflicts with the repository's proof rule

Stage 2: "Build a real registered workflow as maintained application/example code where
appropriate; do not commit a proof-only program." `AGENTS.md` ("Proof is what you saw"): "There
are no proof programs. Nothing written to perform or record a run is committed." The D6 specimen
(local command produces a nonce → remote session derives a value → follow-up turn) has no product
purpose beyond demonstrating placement; committing it under `cmd/examples/` or
`internal/workflows/` as "example code" is the thing the rule forbids, while not committing it
leaves D1/D2 ("exercise registered preparation/admission") and the Stage 4 validator without the
registered definition to re-run. The plan should pick one: a registered workflow with genuine
project use that happens to exercise placement, or a test beside the code that stands up a real
registry, two `message-worker` participants and the River plugin — and say which counts for D1–D6.

## Lesser notes (not counted as findings)

- R3's rebinding semantics ("existing sessions retain their owner after rebinding") has no D claim,
  while R9 says attestation "cannot certify unexercised product paths". Either add an observation
  or mark rebinding as unclaimed for this slice.
- `env.go` (`Env{WorkDir}`, a runtime input) sits in the Declaration cell; if it is meant to carry
  the root environment, that contradicts R2's "without running the workflow body". Clarify or drop it.
- R4's "local command on A through the same contract" implies a `message-worker` participant on A
  as well as B; the live-checkpoint "owned processes" line should name it.

## Outcome

`material findings remain`
