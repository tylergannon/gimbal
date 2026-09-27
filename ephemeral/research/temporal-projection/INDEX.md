# Index: projecting Gimbal workflows into Temporal

A pyramid over 13 notes (about 6,300 lines). Level 0 is one paragraph.
Level 1 routes a question to a file and section. Level 2 lists each
note's sections and its sharpest facts. Read `EXECUTIVE-SUMMARY.md` first,
`QUESTIONS-FOR-TYLER.md` second.

## Level 0

Project the runtime, not the source: keep workflow files unchanged, run
each `Generate`, `RunCommand`, `Check`, `Service` as a Temporal Standalone
Activity (GA 2026-09-14) on a per-run task queue that pins a worker pod
holding the workdir and harness daemons; steer, kill, and interview reach
the worker directly; the page keeps getting per-event records. A dst
source rewrite into a Temporal workflow is feasible (tested) but buys
orchestration replay that cannot protect the state that matters
(sessions), at the cost of the determinism tax. Cost and history are not
risks. Session loss on pod death is a constraint under every engine.
Infra collapses to GitHub App install, a repo file, and Terraform.

## Level 1: question to section

| Question | Where |
|---|---|
| Why not rewrite the workflow into a Temporal workflow? | EXECUTIVE-SUMMARY "Answer"; round1/SYNTHESIS "load-bearing facts" 1, 10; round1/temporal-go-sdk F1, F2 |
| What exactly is a Standalone Activity, is it GA, on Cloud? | round2/r1-1 Finding 1 |
| How do I pin all of a run's work to one pod? | round2/r1-1 Findings 1, 3 (caller-chosen task queue); Finding 4 for why Sessions is worse (hostname-bound, `ErrSessionFailed`) |
| Can a client await an hour-long activity, cancel it, retry it? | round2/r1-1 Finding 1; round1/temporal-go-sdk F3 |
| What happens when a worker pod dies mid-turn, per harness? | round2/r1-2 per-adapter findings and placement table; "what a retry must do" |
| Can session state be checkpointed to a bucket and restored on another pod? | round2/r1-2b (yes; verified for Claude Code; per-harness table; turn-boundary granularity) |
| Where does each harness keep its conversation on disk? | round2/r1-2 (Claude `~/.claude/projects/<cwd-key>/`, Codex `~/.codex/sessions/...` plus SQLite, agy `~/.gemini/`, OpenCode `opencode.db`, Pi `os.MkdirTemp`) |
| How does Steer reach a running remote turn? | round2/r1-3 Option A; round1/temporal-go-sdk F5 ("no signal to an activity") |
| Does WithSupervisor survive? | round2/r1-3 "Gimbal facts" and Option A vs C |
| How does Interview map? | round2/r1-3 "Interview under each option"; round1/temporal-go-sdk F5 (Update) |
| How fast is kill / Group cancellation? | round2/r1-3 "Cancellation propagation" |
| Can Gimbal resume a run after the orchestrator restarts, without Temporal? | round2/r1-4 verdict, §1 table, §2 hazards, §6 comparison |
| Is the native session id recorded? | round2/r1-4 §3 (no; buried in event Data) |
| Is the graph recorded with a run for version checks? | round2/r1-4 §5 (no; hash into RunStarted proposed) |
| What does it cost on Temporal Cloud? | round2/r1-5 §1, §3 ($0.07/run, $20/month at 300 runs) |
| Do heartbeats grow history? | round2/r1-5 §2 (no); round2/r1-1 Finding 2 |
| When is continue-as-new forced? | round2/r1-5 §4 (thousands of turns) |
| Versioning story, Pinned vs Auto-Upgrade, drain time | round2/r1-5 §5 |
| Self-host vs Cloud | round2/r1-5 §6; round1/infra F23 |
| Does dst handle Go 1.27 generic methods? | round2/r2-1 (yes; issue #74 latent; Go 1.27 legalized generic methods) |
| Which Go rewrite tools exist and which are typed? | round1/go-rewrite-tooling comparison table |
| Full rewrite-rule catalogue for a source projection | round2/r2-2 §1 |
| Which stock workflow lines are forbidden under determinism? | round2/r2-2 §2 (11 lines, all validateproduct.go) |
| Proposed lint rules | round2/r2-2 §3 (GIMBAL110 no fs I/O, 111 no exec, 112 no clock/host identity, 113 no defer) |
| Which workflow bodies do OS side effects today? | round1/gimbal-internals-audit §4 |
| What does one Generate call do internally? | round1/gimbal-internals-audit §2; round2/r1-3 "Gimbal facts" |
| Where is the seam for remote commands? | round1/gimbal-internals-audit §1.1 (none), §7 (`Project.Run` in internal/host/host.go) |
| Are node ids deterministic? | round1/gimbal-internals-audit §1.2 (yes, call-site ordinals) |
| What must a remote worker ship back for the page? | round1/gimbal-internals-audit §6; round2/r1-1 Finding 5 (HTTP push, not heartbeats or Workflow Streams) |
| GitHub App tokens, permissions, webhooks, rate limits | round1/eks-secrets-github-infra F1-F7 |
| Secrets on EKS | round1/eks-secrets-github-infra F8-F10 |
| Namespace per project, sandboxing, node policy | round1/eks-secrets-github-infra F11-F15 |
| Images and repo cache | round1/eks-secrets-github-infra F16-F18 |
| Worker autoscaling, shutdown, Cloud connectivity | round1/eks-secrets-github-infra F19-F23 |
| Minimum configuration surface | round1/eks-secrets-github-infra table |
| How do Codex cloud, Claude Code cloud, Cursor, Devin, Jules configure environments? | round1/eks-secrets-github-infra F7; round1/agents-on-temporal-prior-art F10-F13 |
| Who runs coding agents on Temporal? | round1/agents-on-temporal-prior-art F4 (Fateev PoC), F5-F6 (Agent Harness), F9 (Replit) |
| Alternatives to Temporal | round1/durable-execution-alternatives table and verdict (Restate, Hatchet strongest) |
| Home-grown durable replay prior art | round1/durable-execution-alternatives "Home-grown replay"; round2/r1-4 sources |
| Directions considered and rejected | round1/SYNTHESIS table |

## Contradictions found and resolved

| Claim A | Claim B | Resolution |
|---|---|---|
| Temporal Cloud $25 per million actions (alternatives note, secondary) | $50 per million (SDK note) | $50, pricing page fetched raw. round2/r1-5 §1 |
| Legacy Build-ID versioning removed March 2026 | Worker Deployment Versioning GA 2026-03-30 | Both true, two features. round2/r1-5 §5 |
| Heartbeats add history events (SDK note F3) | Heartbeats do not | Do not; Action billed, history "N/A". round2/r1-1 F2, r1-5 §2 |
| Group children writing shared arrays is a problem (audit §4) | Safe under workflow.Go | Safe: cooperative scheduler. round2/r2-2 §4a |
| Latest Go SDK v1.46.0 (alternatives note) | v1.49.0 | v1.49.0, module proxy. round1/temporal-go-sdk |
| MaxConcurrentSessionExecutionSize default contested | 1000 | 1000, from sdk source. round2/r1-1 F4 |

## Level 2: the notes

### BRIEF.md
The question, Gimbal in ten lines, sourcing rules for every note.

### round1/SYNTHESIS.md
Six directions (A source projection, B runtime projection, C whole run in
a pod, D native replay, E graph interpreter, F other engines); ten
load-bearing facts; two finalists with ranked risks R1.1-R1.5, R2.1-R2.4;
provisional recommendation (Finalist 1). Written before round 2; round 2
confirmed it and simplified Finalist 1 further (no workflow needed).

### round1/temporal-go-sdk.md (572 lines)
F1 determinism and workflowcheck (`workflow.Context` is a clone of
`context.Context` with a `workflow.Channel` Done). F2 Group has no
errgroup equivalent. F3 activities: timeouts, retry from the top,
heartbeat-gated cancel, 2 MB payload, 51,200 events. F4 Sessions API. F5
no signal to an activity; Interview is an Update. F6 versioning. F7 Nexus
not needed. F8 claim-check. F9 Cloud pricing and shared-namespace
guidance. Mapping table of every Gimbal primitive with fit. Eight risks.

### round1/go-rewrite-tooling.md (445 lines)
dst v0.28.0 (2026-09-10) status and generics history; 17-tool comparison
(typed, comments, maintained); subset enforcement with go/analysis Facts
as in workflowcheck; sibling-package generation with overlay
type-checking; why Gimbal's closed input set makes a typed rewrite
tractable.

### round1/durable-execution-alternatives.md (441 lines)
Temporal, DBOS Go, Restate, Inngest, Hatchet, Cadence, Dapr, LittleHorse,
Windmill, Conductor, home-grown. Criteria a-j table. Verdict: Temporal
defensible, not uniquely right; Restate closest in spirit, Hatchet best
worker affinity; no engine solves session host pinning cleanly.

### round1/eks-secrets-github-infra.md (512 lines)
25 findings: GitHub App tokens (1 hour), permissions, install-time repo
picker, webhooks, rate limits, prior art (Codex, Claude Code, Cursor,
Jules, Devin); Pod Identity, ESO; namespace isolation, vCluster, Kata;
Karpenter, spot policy; images, EFS clone plus worktrees; KEDA Temporal
scaler (bug #6703 with API keys), worker shutdown, Cloud connectivity,
Helm status; web instance placement and event transport. Minimum
configuration table. Five questions.

### round1/gimbal-internals-audit.md (475 lines)
§1 runtime inventory (only seam is HarnessAdapter; ids deterministic; Seq
counter races across Group children). §2 Generate internals; supervision
is not request/response. §3 session lifecycle per adapter. §4 side effects
in the five stock workflows. §5 the static reader and gimballint rules
101-109. §6 observation contract (run.jsonl, per-session jsonl, command
logs, eight tables, SSE). §7 hosted path; `Project.Run` is the switch
point. Twenty constraining facts; seven questions.

### round1/agents-on-temporal-prior-art.md (509 lines)
Temporal's Python-first agent story; Fateev's Go coding-harness PoC;
Temporal Agent Harness (2026-08-20, steering queue drained next
iteration); Replit on Temporal with Updates; industry keeps control plane
and sandbox separate; Firecracker snapshots vs plain EKS. Fifteen lessons;
pushback that Gimbal's premise is novel.

### round2/r1-1-activities-without-bespoke-workflow.md (434 lines)
Standalone Activities in detail (GA dates, server floor, ID conflict
policy as free idempotency, 1000 concurrent activities per worker);
Update-driven executor fallback with server limits (10 in-flight, 2000
total Updates); worker-specific task queues; Sessions from source; why
heartbeats and Workflow Streams (Public Preview) are not the live-page
transport.

### round2/r1-2-session-durability.md (420 lines)
Per-harness storage paths and versions (Claude ~v2.1.x docs, Codex HEAD
985cf47a, agy 1.2.11, OpenCode 1.18.32, Pi native port); cross-host resume
feasibility; mid-turn failure shapes; placement table (a stateless, b pod
per session, c pod per turn); what a retry must do; what run.jsonl does not
capture (workdir state).

### round2/r1-2b-session-checkpointing.md
Addendum after Tyler's challenge: Claude Code transcript moved between
config dirs and resumed by id, cwd-independent, fork works; per-harness
checkpoint table; design changes (per-run state roots, checkpoint at turn
boundaries, EFS vs bucket).

### round2/r1-3-steer-supervision-remote.md (426 lines)
What one Generate runs, what Steer needs, what the page's controls call;
Options A (worker-direct), B (Update relay), C (queue until next turn)
with a comparison table; Interview and cancellation under each.

### round2/r1-4-memoized-resume.md (379 lines)
Which records reconstruct which node; seven determinism hazards checked
against stock workflows (one real); native id gap; in-flight node at
crash; graph hash for versioning; pod-per-run first.

### round2/r1-5-temporal-numbers.md (196 lines)
Pricing and Action taxonomy; history semantics; two scenarios costed;
continue-as-new thresholds; versioning lifecycle; self-host sketch.

### round2/r2-1-dst-generic-methods.md (251 lines)
The experiment: fixture, byte-for-byte round trip, typed three-rule edit,
issue #74 reproduced but latent, workarounds (Ident.Path for imports,
stale gofmt), effort estimate (low hundreds of lines).

### round2/r2-2-rewrite-rules-and-subset-cost.md (524 lines)
Rule per primitive and construct; statement census (about 410 statements,
11 forbidden); GIMBAL110-113; the two hardest cases; generic vs
consumer-template split.
