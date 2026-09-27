# Round 1 synthesis: directions considered, two finalists, ranked risks

Date: 2026-09-27. Written by the lead (Claude Fable 5.1) after reading all
six round-1 notes in this directory. Judgment, not new evidence; every
claim cites the note it rests on.

## Directions considered

| # | Direction | What it is | Verdict |
|---|---|---|---|
| A | Source projection | Static-rewrite the workflow function (dst) into a generated sibling package that is a Temporal workflow: `context.Context` becomes `workflow.Context`, `Group` becomes `workflow.Go`+`Selector`, turns/commands/services become activities. | Finalist 2. The literal request; highest code volume and the most semantic narrowing. |
| B | Runtime projection | Keep the workflow source and `gimbal.Run` orchestrating in ordinary Go. Project the *runtime*: sessions, commands, and services execute on remote workers as Temporal activities, pinned to a host; the orchestrator submits work to Temporal instead of `os/exec`. | Finalist 1. Smallest change to what workflow authors write; Temporal does what it is good at (routing, retries, worker fleets) and is not asked to replay ordinary Go. |
| C | Whole run in one pod | Temporal (or plain Kubernetes) starts one pod per run that executes the unmodified binary in-process, streaming events back to the shared web instance. | Not a finalist on its own; it is step zero of B. Zero rewrite, zero determinism constraints, no mid-run durability. |
| D | Gimbal-native durable replay | Memoize completed turns/commands from run.jsonl keyed by the deterministic node ids and re-run the body on restart; build a worker pool. | Not a finalist on its own; it is the durability add-on for B, and the honest answer to "do we need Temporal's replay at all". |
| E | Graph interpreter | Execute `workflow.Graph` on a generic Temporal workflow. | Rejected: the graph deliberately carries no conditions or data flow (audit §5.2). |
| F | Another engine (Restate, Hatchet, DBOS) | Same projection, different substrate. | Rejected for now: Restate and Hatchet fit "ordinary Go" better but none has Sessions-grade host pinning plus mature versioning (alternatives note); the *shape* of B is engine-agnostic anyway, so the choice can be revisited without redoing the design. |

## The load-bearing facts

1. Temporal's workflow half fights Gimbal's design; its activity half fits
   it. `workflow.Context` is not a `context.Context`, bare goroutines are
   forbidden, and every replay-based engine (Temporal, DBOS, Cadence, Dapr)
   shares this (temporal-go-sdk.md F1; alternatives.md). Activities take a
   plain `context.Context`, run for an hour with heartbeats, and are
   exactly the shape of `Generate`, `RunCommand`, `Check`, `Service`.
2. Node identity is already deterministic (audit §1.2): ordinals are
   assigned synchronously at the call site, so `lap.3/check.2` is stable
   across two executions with the same activity results. This is the
   precondition for both Temporal replay and Gimbal-native memoization.
3. `HarnessAdapter` is the only seam; commands and services are hardwired
   to `os/exec` (audit §1.1). Any remote execution needs one new seam for
   commands and services, which under AGENTS.md is Tyler's call by name.
4. Sessions are host-pinned by construction (codex app-server, opencode
   server, pi in-process); Claude and agy spawn per turn and resume from
   the harness's own transcript store (audit §3). Nothing in Temporal makes
   a workdir or a daemon durable; Sessions only keeps routing sticky while
   the host lives, and a dead host fails the session (temporal-go-sdk.md F4;
   prior-art.md L5).
5. Steer into a running turn has no Temporal-native construct ("there is no
   functionality to send signal to an activity", temporal-go-sdk.md F5);
   the ecosystem's best pattern queues text for the next loop iteration
   (prior-art.md F6). `WithSupervisor` is therefore only implementable
   *inside* the activity that runs the turn, on the worker, where Gimbal's
   in-process supervisor code already works.
6. The workflow bodies already contain host-specific side effects:
   `os.Executable`, `exec.LookPath`, `os.MkdirTemp`, `time.Now`, raw
   `exec.CommandContext`, `WalkDir` over agent output (audit §4). Direction
   A must forbid or rewrite all of it; direction B tolerates it as long as
   the orchestrator and the workers share a filesystem view or the
   workflow is nudged toward `Check`/`RunCommand`.
7. Hard limits: 2 MB per payload, 51,200 events / 50 MB history, hence
   continue-as-new for long PromiseLoops under A (temporal-go-sdk.md F3, F8).
   Gimbal's bounded-excerpt-plus-file pattern already is the claim-check
   pattern.
8. Industry keeps the durable control plane and the sandbox as two systems
   joined per task or per session (Replit, Devin Outposts, Codex, Claude
   Code cloud; prior-art.md F10, F12). No one hosts a multi-hour steerable
   agent session inside a Temporal activity in public. B follows the
   industry split; A is novel.
9. Infra is not the risk. GitHub App install is the repo-selection UI,
   webhooks report changes, tokens last one hour and a credential helper
   refreshes them, ESO plus Pod Identity distributes secrets, KEDA scales
   workers by task-queue backlog, Temporal Cloud avoids running the
   service (infra.md). Every config row collapses to app install, a repo
   file, and platform Terraform; the only plausible UI is egress policy.
10. The pushback on the premise: durable *replay of orchestration* buys
    less for Gimbal than for a payments workflow, because the expensive
    state (a session's conversation and workdir) lives on the worker and
    is not made durable by any engine. What Gimbal needs from a substrate
    is host-pinned remote execution, scheduling, and resume-after-
    orchestrator-restart. Temporal supplies the first two out of the box
    under either direction; the third under A comes with the whole
    determinism tax, under B it is a Gimbal-native memoized resume built
    on fact 2.

## Finalist 1: runtime projection (B, with C as step zero and D as the add-on)

Shape: the workflow source and the orchestrator are unchanged Go. A run is
submitted to a per-project task queue. Each `Generate` (with its
supervisors), `RunCommand`, `Check`, and `Service` executes as a Temporal
activity on a worker pod that owns the workdir and the harness daemon,
pinned by a Temporal Session (or a worker-specific task queue). Events
stream from the worker to the shared web instance as today's per-event
records, so the page is unchanged. Steer and Interview reach the worker
through the instance. Codegen emits the worker binary's registration,
the per-project task-queue and Kubernetes manifests, and nothing else.

Ranked risks, each getting a round-2 cycle:

- R1.1 Can Temporal execute activities with host pinning without a
  hand-written Temporal workflow per Gimbal workflow? Candidates:
  Standalone Activities (noted GA in SDK v1.49.0), or one generic
  "executor" workflow per run driven by Updates. If neither works, B
  degenerates to C plus a home-grown scheduler.
- R1.2 Session durability: what survives a worker pod death for each
  harness, and can Claude/agy sessions resume on another pod if their
  transcript store is on shared storage. Decides whether "retry the turn"
  is the only recovery.
- R1.3 Steer and supervision across the process boundary: the worker
  hosts the in-process supervisor; steer arrives worker-direct or via a
  second same-session activity. Latency and correctness.
- R1.4 Orchestrator restart: memoized resume from run.jsonl in Gimbal
  itself (deterministic ids, PromiseLoop decisions as recorded results),
  versus running the orchestrator itself as a pod per run.
- R1.5 Cost and cadence: heartbeat interval versus Temporal Cloud actions;
  pricing figures disagreed between notes ($25 vs $50 per million).

## Finalist 2: source projection (A)

Shape: `gimbalgen` grows a second output: a sibling package per workflow
that is a Temporal workflow function, produced by a dst rewrite of the
entry function and its inlined same-package helpers, with a consumer
template deciding the target dialect. Activities are the same worker as in
Finalist 1. The lint grows a determinism subset (no `os`, `time`, `exec`,
`defer` in bodies; everything through the primitives).

Ranked risks, each getting a round-2 cycle:

- R2.1 dst round-trips and edits Go 1.27 generic methods (`Generate[T]`).
  Unverified; a hands-on test settles it.
- R2.2 Expressiveness cost: the full rewrite-rule catalogue against the five
  stock workflows, and which of them would pass a determinism subset today.
- R2.3 History growth, continue-as-new insertion, and Pinned versioning
  with hour-long turns: the numbers.
- R2.4 Supervision and steer under a generated workflow (shared with R1.3).

## Provisional recommendation

Finalist 1, with Finalist 2 held as a later option once the activity layer
exists (it is a strict superset: A's activities are B's activities). The
static analysis and codegen Tyler asked for still exist in Finalist 1, but
they generate linkage (worker registration, task queues, manifests, the
lint subset) rather than rewriting the workflow. Round 2 tests this.
