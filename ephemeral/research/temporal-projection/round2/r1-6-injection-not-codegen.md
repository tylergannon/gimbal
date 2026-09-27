# Reframing after Tyler's challenge: injection, not code generation

Date: 2026-09-27. Tyler: if all the behavior lives in the runtime, the
projection is dependency injection, not codegen. Agreed, with caveats.

## What injection already covers

- `ModelBinding.Adapter` is injected per role at `Run` (harness.go:45-50).
  A Temporal-backed `HarnessAdapter` implements `CreateSession` (allocate a
  per-run state root on the worker), `RunTurn` (Standalone Activity on the
  run's queue, events streamed back), `Steer` (HTTP to the worker's control
  listener), `Fork`, `Close`. No workflow source is touched or generated.
- The worker needs the Gimbal runtime and the local adapters, not the
  workflow. One generic worker image serves every workflow.
- Supervision stays on the orchestrator, unchanged: the transcript ring
  (supervise.go:126,166-167) is fed by the same event stream the page
  needs; objections go out through `adapter.Steer`. This supersedes
  r1-3's Option A (supervisors on the worker); both work, orchestrator-side
  needs zero change to supervise.go and supervise_jev.go.
- Scope lifetimes, Interview, PromiseLoop, Group cancellation: unchanged
  orchestrator code.

## What is not an interface swap

1. **The commands seam does not exist.** `RunCommand`, `Check`, `Service`
   call `os/exec` directly (command.go:241, service.go:72). One new
   interface beside `HarnessAdapter`; Tyler names it. Its remote
   implementation ships stdout/stderr streams back into the run directory
   (the claim-check pattern already used for large streams).
2. **Mid-turn event transport.** Heartbeats are throttled to
   min(0.8×HeartbeatTimeout, 60 s) (r1-1 F5); the page and supervisors need
   per-record delivery. The worker pushes events to the orchestrator (or the
   instance hosting it) over HTTP/gRPC. New transport, small.
3. **Per-run state roots and checkpoints.** A worker-side decorator around
   each local adapter: redirect `CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
   `XDG_DATA_HOME`, Pi's history dir to the run's volume; checkpoint at
   turn end; restore at first activity. Per-harness work (r1-2b).
4. **Path agreement.** Bodies compute absolute paths and bake them into
   prompts and scope values; the pod mounts the workdir at the same path.
   Convention, enforced by the pod template.
5. **Lint, not rewrite.** GIMBAL110-113 keep bodies from reading a
   filesystem that is no longer local. Only matters when the orchestrator
   is not on the pod (stage 1+). It is `go/analysis`, already how
   gimballint works.

## What "template" means now

A consumer supplies: an adapter pair (remote for the orchestrator, local
plus checkpoint decorator for the worker), a worker image, a pod template,
queue naming. No source transformation. The existing generator keeps
emitting what it emits today (graph, CLI, SKGO forms).

## Residual codegen worth considering, none required

A manifest of roles and adapters a workflow needs, derived from the
registered graph, for pre-warming daemons or sizing a pod. Nice to have.

## Steer is not a signal (Tyler's question)

Signals, Updates, and Queries target a Workflow Execution. Temporal has no
API that delivers into a running activity; the only inbound event is
cancellation, at the next heartbeat (round1/temporal-go-sdk F5). Under
Standalone Activities there is no workflow execution, so a signal has no
target. Even with a workflow (Finalist 2, or the executor fallback), its
handler can only schedule an activity on the session host. So in every
shape the message ends as an activity that runs in the pod holding the
turn. The Temporal-native steer is therefore `client.ExecuteActivity`
("steer", session, message) on `run-<id>`; cancel is the same with the
turn id, instant, not heartbeat-gated. This replaces r1-3's direct-HTTP
Option A: same fidelity, no worker address registry, no NetworkPolicy
ingress from the instance, one action per steer. Requires the worker's
`MaxConcurrentActivityExecutionSize` above one (default 1000, r1-1 F1).
