# Projecting Gimbal workflows into Temporal: executive summary

Date: 2026-09-27. Two research rounds, 13 notes, one hands-on experiment.
Round 1 surveyed six directions; round 2 spent one cycle on each top risk of
the two finalists. Every claim below points at a note; `INDEX.md` routes
questions to sections.

## Answer

Yes, and the projection should be of the **runtime, not the source**.
Keep workflow files as they are. Make Temporal the execution substrate for
the parts that are already activity-shaped (`Generate`, `RunCommand`,
`Check`, `Service`), using **Temporal Standalone Activities** (GA in Go SDK
v1.49.0, 2026-09-14, GA on Temporal Cloud) routed to a per-run task queue so
one worker pod owns the run's workdir and harness daemons. No Temporal
workflow function is written or generated at all. Static analysis and
codegen stay central, but they generate linkage (a determinism lint subset,
worker registration, queue naming, Kubernetes manifests), not a rewritten
workflow. This is Finalist 1.

Finalist 2, a dst rewrite of the workflow into a generated Temporal
workflow package, is **feasible** (the dst experiment passed byte-for-byte
with typed edits; four of five stock workflows pass a determinism subset)
but buys the wrong thing: durable replay of orchestration, when the state
that is expensive to lose (a session's conversation and workdir) lives on
the worker and no engine makes it durable. It costs the whole determinism
tax: `workflow.Context` everywhere, hand-expanded `Group` boilerplate at
every site, continue-as-new re-threading of scope values, heartbeat-gated
cancellation instead of instant `Killed`. Keep it as a later option: its
activities are exactly Finalist 1's activities, so nothing is thrown away.

## Reframed after review: injection, not codegen

The projection is a second `HarnessAdapter` (plus one new commands seam)
injected at `Run`, not generated code. The worker is generic and does not
carry the workflow. Supervisors stay on the orchestrator unchanged. What
remains beyond interface swaps: a mid-turn event transport, per-run state
roots with turn-boundary checkpoints, path agreement, and four lint rules.
See round2/r1-6.

## What the research settled

| Risk | Outcome | Evidence |
|---|---|---|
| Activities with host pinning without a bespoke workflow | Eliminated | Standalone Activities: client-started, awaitable, heartbeat, cancel, retry, caller-chosen task queue, no event history. round2/r1-1 |
| Session survival across pod death | Reduced (corrected after Tyler's challenge) | Session state is portable at turn boundaries: verified for Claude Code (one transcript file, resume by id, cwd-independent); designed-for in Codex source; Gimbal-owned for Pi. The blocker is adapter state roots, not harness formats. Checkpoint per turn, restore on any pod, lose at most the in-flight turn. round2/r1-2, r1-2b |
| Steer and supervision across the boundary | Reduced | Supervisors stay on the orchestrator, fed by the streamed events. Steer and cancel are second activities on the run's queue, which only the run's pod polls, so they land in the process holding the turn. Not a signal: signals target workflows, and there is none. round2/r1-3, r1-6 |
| Orchestrator restart mid-run | Deferred by evidence | Records suffice to memoize turns, commands, planner decisions, interviews; one hazard (`implementation.go` reads a file into `Iterate`); native session id not recorded. Today's guarantee is already "crash means restart", so pod-per-run first. round2/r1-4 |
| Cost and history size | Eliminated | $0.07 per 20-turn run, about $20 per month at 300 runs; 1% of the history limit; heartbeats bill but never touch history. round2/r1-5 |
| dst on Go 1.27 generic methods | Eliminated | Byte-for-byte round trip and a typed three-rule edit passed vet and ran. round2/r2-1 |
| Expressiveness cost of a source rewrite | Bounded | Rule catalogue is finite; 11 forbidden lines, all in `validateproduct.go`; lint rules GIMBAL110 to 113 proposed. round2/r2-2 |
| Versioning | Settled | Worker Deployment Versioning GA 2026-03-30; Pinned; old workers drain in about a day. With Standalone Activities there is no workflow to version. round2/r1-5 |
| Infra | Not the risk | GitHub App install is the repo picker; tokens last one hour and a credential helper refreshes them; ESO plus Pod Identity for secrets; KEDA scales workers by backlog; Temporal Cloud beats self-hosting by 20x in dollars at this volume. round1/eks-secrets-github-infra, round2/r1-5 |

## Pushbacks

1. **Durable replay of orchestration is the wrong prize.** Gimbal needs
   host-pinned remote execution, scheduling, and a shared page. Session
   durability comes from Gimbal checkpointing harness state and workdir at
   turn boundaries (r1-2b), which no engine does for it. Temporal supplies
   the first two without asking Gimbal to become deterministic.
2. **Nobody hosts a multi-hour steerable agent session inside a Temporal
   activity in public.** Replit, Devin Outposts, Codex, Claude Code cloud
   all keep the control plane and the sandbox as two systems. Finalist 1
   follows that split; Finalist 2 is novel.
3. **Steer is an activity, not a signal.** Temporal routes to a host, never
   into a running process ("there is no functionality to send signal to an
   activity"); a steer activity on the run's queue reaches the pod holding
   the turn and calls the local adapter's Steer.
4. **Namespace-per-project is the Kubernetes answer, not the Temporal
   answer.** Temporal's guidance is one namespace, task queues per tenant.
5. **Three stock workflows carry latent bugs independent of this work.**
   `validateproduct.go` launches two subprocesses outside `RunCommand`
   (invisible to log, graph, page); two workflows bake `os.Executable()`
   into scope context; `implementation.go` reads a file into an `Iterate`
   bound with no record. Fix before any projection.
6. **The commands seam does not exist.** `HarnessAdapter` is the only
   pluggable boundary; `RunCommand`, `Check`, `Service` call `os/exec`
   directly. Remote execution needs one new exported name, which under
   AGENTS.md is Tyler's to ask for.

## Build order that minimizes code

0. **Whole run in one pod.** `gimbal run` submits; a pod runs the unmodified
   binary in-process; the run directory lives on a shared volume the
   instance tails (or the pod pushes records to the instance). Zero workflow
   changes, zero determinism constraints, no mid-run durability (same as
   today).
1. **Standalone Activities.** Add the commands seam; the orchestrator issues
   each turn, command, and service as a Standalone Activity on the run's
   task queue; the worker pod hosts sessions and the in-process supervisor
   code; worker-direct control listener for steer, kill, interview; generated
   worker registration and manifests; lint GIMBAL110 to 113.
2. **Platform.** GitHub App with install webhooks, credential helper, ESO
   plus Pod Identity, per-project Kubernetes namespace from a template, KEDA
   on the task-queue backlog, Temporal Cloud.
3. **Later, on evidence.** Memoized resume from `run.jsonl` once orchestrator
   crashes are measured; Finalist 2's emission template if a consumer wants a
   literal Temporal workflow.

## Caveat on method

The Diffusion Router key is not present in this environment, so the Gimbal
research-document workflow with DeepSeek or GLM could not be run; research
was done by Sonnet sub-agents fetching primary sources, with summarized
fetches distrusted after one was caught wrong on a date. Each note marks
verified against inferred. Nothing here was load-tested against a live
Temporal deployment; the dst experiment is the only thing that ran.
