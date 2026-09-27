# Durable-execution alternatives to Temporal

Date: 2026-09-27. Research sub-agent, round 1. Scope: challenge BRIEF.md's
premise that Temporal is the right projection target for Gimbal workflows.
No code below; this is a survey and an argument.

## How to read this note

Claims are tagged: **(verified, url, date)** — read from an official
doc, repo, or pkg.go.dev page this session; **(inferred)** — my own
reasoning, or something I could not confirm, stated with a confidence
level; **(secondary, url)** — a blog or aggregator, not the vendor's own
docs, lower confidence. Several WebFetch calls this session returned a
paraphrase rather than raw quotation; where that happened I say so.

## What Gimbal actually needs projected

From `go doc -all .` (read directly): a workflow is
`func(ctx context.Context, env gimbal.Env, params P) error`. Orchestration
is `Scope`, `Group` (`g.Go(name, fn)` is the *only* way a workflow starts
a goroutine, errgroup-shaped), `Iterate`, and `PromiseLoop` (a planner
picks tasks adaptively at runtime — not fixed at compile time). The long
steps are `Session.Generate` (one blocking agent turn, minutes to an
hour), `RunCommand`, `Service` (a long-lived foreground process), and
`Check`. Every node is named at its call site with a scope key plus an
ordinal (`lap.3/check.2`), and the run log already durably records
`TurnStarted`/`TurnEnded`, `CommandStarted`/`CommandEnded`,
`SessionCreated`/`SessionClosed`, `Steer`, and interview exchanges.
`internal/generate` already statically extracts a `workflow.Graph` via
`go/packages`/`go/types`, and refuses (as diagnostics) constructs it
cannot read: `select`, bare `go`, `defer`, type switches.

Comparison criteria: (a) does orchestration code stay ordinary Go with
`context.Context`, (b) goroutines/errgroup allowed or SDK primitives
required, (c) hour-long steps with progress/heartbeat, (d) pin a series
of steps to one host holding process state (a session's workdir), (e)
signal/human-in-the-loop into a running step, (f) versioning of in-flight
runs when code changes, (g) self-host on Kubernetes and cost, (h)
maturity/adoption in 2026, (i) observability/UI, (j) payload size limits.

## Findings per engine

**Temporal.** Go SDK latest is v1.46.0, July 2026 (secondary,
daily.dev digest; not independently opened). The model splits sharply:
the **Workflow** function uses `workflow.Context`, not `context.Context`;
concurrency must go through `workflow.Go`/`Selector`, never bare `go`;
time/randomness go through `workflow.Now`/`SideEffect` — because the
workflow is replayed from Event History and must reproduce the same SDK
commands. **Activities** take ordinary `context.Context`, allow
goroutines, aren't replayed, and support `Heartbeat` plus effectively
unbounded `StartToCloseTimeout` — a strong match for Gimbal's turns/
commands/services. Versioning: `workflow.GetVersion`/`Patched` "applies a
code change to new Workflow Executions while avoiding disruptive changes
to in-progress" ones (verified, docs.temporal.io/develop/go/versioning,
2026-09-27); the newer Build-ID-based Worker Versioning is being removed
from Temporal Server in **March 2026** per that same page — its own
versioning story is still moving. Payload limits: **2 MB/payload
default** (warns at 256 KB), **4 MB gRPC cap** per Workflow Task
(verified via search of docs.temporal.io/troubleshooting/blob-size-limit-error);
External Storage for larger blobs hit Public Preview 2026-05-14
(secondary). Self-host: Helm charts, Cassandra at scale/Postgres smaller
(secondary); Cloud ~$25/M Actions plus storage, with self-host reported
cost-competitive only above roughly 30–50M Actions/month (secondary,
automationatlas.io, 2026). GA, largest ecosystem/adoption/observability
(Temporal Web) of everything surveyed.

**DBOS Transact for Go** (dbos-inc/dbos-transact-golang). MIT license,
841–843 stars, last updated 2026-09-24 (verified, GitHub, fetched
2026-09-27). Workflows take **`dbos.Context`**, not `context.Context`,
with the package's own `WithCancel`/`WithTimeout`/`WithValue` (verified,
pkg.go.dev). Plain goroutines are discouraged; `dbos.Go()` "runs a step
inside a Go routine and returns a channel," for deterministic concurrent
recovery (verified, same page, quoted). Steps are explicit:
`RunAsStep[R any](ctx, fn Step[R], opts...)` (verified, same page).
Database: **SQLite for dev, Postgres or CockroachDB for prod** — not
Postgres-only. Versioning: `Patch()`-guarded branches, "existing
workflows that already passed this patch point continue with old code"
(verified, quoted). **DBOS Conductor** is a real, optional, self-hostable-
or-hosted control plane for cross-executor recovery, dashboards, and
management: "if your connection to Conductor is interrupted, your
applications will continue operating normally" (verified,
docs.dbos.dev/production/conductor). Self-hosting Conductor in
production needs a paid license (free key = one executor/app). Payload/
state size limits: undocumented; Postgres TOAST (~1 GB) is the implicit
ceiling (inferred). Go SDK reads as DBOS's youngest language SDK, no
explicit GA statement found (inferred).

**Restate.** Server is **BSL-licensed** (source-available, not fully
permissive), self-hostable as a single binary/Docker/Kubernetes, plus
Restate Cloud (50k free durable actions/month) and BYOC (secondary,
restate.dev/cloud and blog posts, 2026). ~4.5k GitHub stars (verified,
fetched 2026-09-27). Handlers take a special `restate.Context`/
`ObjectContext`/`WorkflowContext`, not `context.Context` (verified,
docs.restate.dev/develop/go/services). But the determinism boundary is
much narrower than Temporal's: **only `restate.Run`-wrapped operations
are journaled and need determinism — the surrounding handler can use
ordinary goroutines and `context.Context` freely** (verified,
docs.restate.dev/concepts/durable_building_blocks/, quoted: "you can use
standard goroutines, context.Context, and non-deterministic code
freely"). Caveat (verified via search of
docs.restate.dev/develop/go/concurrent-tasks): multiple *journaled*
operations must be combined with `restate.Wait`/`Select`, not raw
`select`/channels — Restate needs to log their completion order itself.
Durable primitives: steps (memoized), durable RPC, durable sleep
(consumes no resources while waiting, resumes exactly on schedule), and
signals/awakeables for external input mid-handler — a good fit for
`Interview`. No documented worker-affinity/host-pinning mechanism found;
Restate's RPC/stateless-invoker model argues against it being a natural
fit for pinning steps to a session's workdir host (inferred). Payload/
journal size limits and in-flight versioning semantics: not found this
session.

**Inngest.** `inngestgo`'s `step` package published 2026-03-11, repo
updated 2026-09-25 (verified via search). Non-deterministic/side-effecting
work goes in `step.run()`, checkpointed so retries skip completed work;
step IDs are counters so a step can loop without changing ID (verified
via search of inngest.com/docs/learn/inngest-steps). Whether code
*between* steps may freely use goroutines/`context.Context` was **not
confirmed** this session (inferred similar to DBOS/Restate's pattern,
flagged low-moderate confidence). Self-hosting: OSS core, official Helm
chart (k8s 1.20+), SQLite/in-memory Redis by default, Postgres/Redis for
prod; "Inngest's support team does not guarantee direct support for
self-hosted instances" (verified, inngest.com/docs/self-hosting).
Payload: **4 MB per step's output, 4 MB cumulative per function run**
(verified via search of inngest.com/docs/usage-limits/inngest). License
for the Go SDK/core not confirmed this session.

**Hatchet.** MIT license, ~8,000+ stars, 508 forks (verified, GitHub,
fetched 2026-09-27). Two task tiers: **regular tasks** are at-least-once
only — "a task can run more than once, so your task code should be
idempotent" — with no determinism requirement, ordinary `context.Context`
and goroutines fine (verified, docs.hatchet.run/v1/architecture-and-guarantees).
**Durable tasks** (`NewDurableTask`), for work that must survive a
worker crash or wait on external events without holding a slot, "should
be deterministic" (verified via search of
docs.hatchet.run/home/durable-best-practices) — a Temporal-style
constraint, scoped to only this task type, mirroring Restate's and
DBOS's confine-determinism-to-what-needs-it approach. **Worker
affinity** (labels, weighted rules, a "sticky-workers" example) is the
**best-evidenced mechanism surveyed** for pinning a run's steps to one
host (criterion d). State persists transactionally in Postgres; "Hatchet
Cloud and self-hosted Hatchet share the same architecture" (verified).
No general documented payload ceiling (caution above ~1 MB), but a
concrete sourced gotcha: `put_stream()` (closest analog to Gimbal's
`Steer`) is capped at **~8 KB by Postgres's `pg_notify`** (verified,
github.com/hatchet-dev/hatchet/issues/3087, filed 2026-02-23). Hatchet
also publishes `hatchet-dev/durable-execution-the-hard-way` ("set up a
durable execution engine from scratch using Postgres, no dependencies" —
title verified via search), directly relevant prior art for Gimbal's own
home-grown option.

**Cadence.** `go.uber.org/cadence`, Uber-originated 2017; Temporal is a
2019 fork by the same team, sharing almost the identical `workflow.Context`
/deterministic-replay model (verified via search of cadenceworkflow.io
FAQ and comparison pieces). No first-party Cadence Cloud; Uber runs it
internally at claimed 12B executions/month (secondary). Every comparison
source surveyed recommends Temporal over Cadence for new 2026 projects
(tooling, multi-region, managed cloud). Technically almost the same
target as Temporal with a materially weaker ecosystem — not worth
separate investment.

**Dapr Workflow (Go).** CNCF-graduated project. Orchestrator takes
`*workflow.WorkflowContext`, must be deterministic, routes side effects
through `ctx.CallActivity` (which does take `context.Context`) — same
split as Temporal (secondary, docs.dapr.io summary, moderate confidence:
paraphrased not directly quoted). Human-in-the-loop:
`ctx.WaitForExternalEvent(name, timeout)` checkpoints and suspends,
resuming when another service posts the event via Dapr's HTTP API
(secondary, oneuptime.com blog series, 2026-03-31). Dapr 1.18 (June
2026) added attestation/tamper-evident execution history for workflows
and agents; 1.18.2 was latest stable as of July 2026 (secondary, via
Wikipedia). Self-host is the standard Dapr sidecar on Kubernetes with
any state store — no vendor consumption fee (inferred). Dapr overall is
mature/CNCF-graduated; the Workflow building block reads as a newer,
thinner layer within it (inferred, moderate confidence).

**LittleHorse.** Dual-licensed: AGPLv3 (server/dashboard), Apache-2.0
(SDKs); 401 stars, 2,139 commits, 37 forks (verified, GitHub, fetched
2026-09-27); self-hostable via a single Docker image. My own
recollection, **not verified this session** (flagged low confidence): the
Go SDK likely expresses control flow via a `WorkflowThread` builder
(`DoIf`/`DoWhile`/`Execute`) rather than native `if`/`for` — a DSL
embedded in Go, closer to Conductor's/Windmill's declarative-graph model
than to Temporal's or Restate's "run this as a real function." This is
the single biggest open question left in this note and should be
resolved directly against LittleHorse's SDK reference before ruling it
in or out. Community is small (401 vs. Restate's 4.5k, Hatchet's 8k).

**Windmill.** AGPLv3, 12,400+ stars by May 2026 (verified via search).
Self-host is "a server, a Postgres database, and one or more workers,"
Docker or Kubernetes, free Community Edition; Cloud from $10/month/
author (secondary). Critically, a Windmill "flow" is a **DAG of
independently-invoked scripts** (Go/Python/TS/Bash/SQL, each its own
process); "Workflows as Code" means the DAG's shape is *parsed out of*
code via WASM into a graph, not one continuous function running the
orchestration (verified, windmill.dev/docs/core_concepts/workflows_as_code).
Branches, for-each loops, error handlers, approval steps are all
DAG-level constructs, not Go control flow. 2026 added AI-agent steps
alongside code steps (secondary, August 2026). A materially different
authoring model from Gimbal's ordinary-Go-function approach, and a
bigger rewrite for `PromiseLoop`'s adaptive dispatch than any code-first
engine surveyed.

**Conductor OSS.** Netflix-originated 2016; Netflix stopped active
maintenance **2023-12-13**; Orkes is now primary steward and also sells
a hosted Conductor Cloud (secondary, docs.conductor-oss.org/orkes.io).
The Go SDK is **not** code-first control flow: "Go code translates to a
JSON definition ... registered with your Conductor server" (verified,
conductor-oss/go-sdk README, via search). Orchestration is data — a JSON
task graph the server interprets — and Go writes only the graph-builder
and stateless task **workers**
(`ExecuteTaskFunction func(t *Task) (interface{}, error)`). This is the
most structurally different model from Gimbal surveyed: no Go function
anywhere is "the workflow." `PromiseLoop`'s runtime-adaptive dispatch
would have no natural home short of Conductor's own dynamic-task/
sub-workflow primitives.

**Home-grown replay** (Gimbal memoizing its own run log). Gimbal already
has three of four prerequisites: **(1)** stable, deterministic node
identity at every call site (`lap.3/check.2`); **(2)** a durable,
append-only run log of every turn/command/session lifecycle event;
**(3)** a statically-extracted, registered graph
(`RegisterGraph`/`RegisteredGraph`) that already separates what it can
read from what it refuses. The closest public prior art is
**`agenticenv/durable-go`** (verified, GitHub, fetched 2026-09-27): a
"zero-infra, in-process ... single Go library with a filesystem
journal" that does **not** replay a whole deterministic call graph.
Instead it **memoizes individual step results** by stable step ID; on
resume the surrounding function just re-runs, and completed steps return
cached results — "no replay-determinism sandbox" (verified, quoted). Its
named limitations map onto risks Gimbal would inherit: step IDs must
never be renamed (a rename silently re-executes, not corrupts);
unbounded new-step-ID loops in one run are unsupported (whole journal
scanned per run); one writer per data directory via exclusive `flock`,
no built-in distributed coordination; steps must be idempotent (a crash
can re-run one after its side effect landed); non-deterministic values
must be generated *inside* the step, never the surrounding body; and
versioning is developer discipline — `WithStepVersion` must be bumped by
hand or "cached results are reused even if `fn` or inputs differ"
(verified, all quoted).

Gimbal is arguably better positioned than `durable-go`'s generic model on
that last, worst point: because `internal/generate` already turns the
workflow into a registered, structured `workflow.Graph` rather than a bag
of ad hoc string IDs, a resume path could mechanically diff the
currently-compiled graph against the graph recorded with a run and refuse
on mismatch, instead of silently reusing a stale result the way
`durable-go`'s opt-in version string does, or forcing hand-written
`GetVersion`/`Patched` branches the way Temporal does. That is unbuilt,
but it is a genuine, Gimbal-specific opportunity no surveyed engine
offers as an automatic check.

What is still missing, honestly: **(1)** actual crash/process-restart
recovery — `Run` today executes in the caller's process with no
described resume-from-log path; new work, not a free byproduct. **(2)**
`PromiseLoop`'s adaptively-chosen task sequence needs a replay-stable
identity across a crash mid-decision — a hard design problem shared by
every framework surveyed, not special to Temporal. **(3)** a remote
worker pool for sessions: pinning "the next step of this session" to the
one host holding its workdir, across a distributed pool, is new
scheduling infrastructure comparable in scope to Hatchet's worker
affinity or Temporal's sticky queues — building it is genuinely as much
work as adopting an engine that already solved it. **(4)** heartbeat/
progress on hour-long steps is, by contrast, largely already covered:
Gimbal already streams stdout/stderr and `AgentRecord` events live into
the run log and web page while a turn, command, or service runs.

The one repeated lesson across every vendor's own docs touched this
session — Temporal's `NondeterminismError`/`GetVersion`, DBOS's
`Patch()`, Restate's `Wait`/`Select` requirement, Hatchet's plain/durable
split, `durable-go`'s `WithStepVersion` — is that **nondeterminism
detection and in-flight versioning is the one problem no framework,
hand-rolled or vendor-built, avoids.** A speculative, unfetched-in-depth
academic reference worth noting: "State Without a Landlord: An
Architecture Proposal for Peer-to-Peer Replication of Durable Workflow
State" (arxiv 2609.17645, title/abstract only from search) — directional
prior art for a distributed home-grown pool, not production software.

## Comparison table (a–j)

| Engine | a: ordinary Go + context.Context | b: goroutines allowed | c: hour-long step + progress | d: pin steps to one host | e: signal into running step | f: in-flight versioning | g: self-host on k8s + cost | h: 2026 maturity | i: observability | j: payload limits |
|---|---|---|---|---|---|---|---|---|---|---|
| **Temporal** | No — `workflow.Context`; activities get `context.Context` | No in workflow (`workflow.Go` only); yes in activities | Yes — Activity heartbeat, unbounded timeout | Sticky queues; older Session feature (status unclear) | Yes — Signals/Updates, mature | Best-documented: `GetVersion`/`Patched`; Worker Versioning leaving server Mar 2026 | Helm/Cassandra or Postgres; Cloud ~$25/M actions; break-even ~30-50M/mo (secondary) | GA, largest ecosystem | Temporal Web, most mature | 2 MB/payload, 4 MB gRPC cap; External Storage Public Preview May 2026 |
| **DBOS (Go)** | No — `dbos.Context` | Discouraged; `dbos.Go()` wrapper | Steps retry w/ backoff; no heartbeat found | Not evidenced | Not evidenced | `Patch()`-guarded branches | Just Postgres/CockroachDB/SQLite; optional paid Conductor for self-host prod | Newer, ~840 stars; Go SDK likely youngest | Optional Console/Conductor dashboards | Undocumented; Postgres TOAST ceiling (inferred) |
| **Restate** | **Yes for ordinary logic** — only `restate.Run` needs determinism | Yes outside journaled ops; journaled concurrency needs `Wait`/`Select` | Durable sleep, no resource use while waiting; heartbeat unconfirmed | Not evidenced; RPC/stateless model argues against it | Yes — signals/awakeables | Not deeply verified | BSL license; binary/cluster or Cloud/BYOC; ~4.5k stars | Newest credible option (1.0→1.5, 2024-2026) | Execution log + UI, not deeply verified | Not found |
| **Inngest** | Step-wrapped like DBOS/Restate; unwrapped-code freedom unconfirmed | `step.run()` boundary; parallel-step helpers | Steps can run long; heartbeat unconfirmed | Not evidenced; HTTP-push model argues against it | Signal/wait-for-event known from TS SDK, unconfirmed for Go | Not verified | OSS core, Helm k8s 1.20+, SQLite/Redis default; support not guaranteed self-hosted | Active (Mar/Sep 2026 updates); Go SDK newer than TS | Inngest dashboard | **4 MB/step, 4 MB cumulative/run** (verified) |
| **Hatchet** | Regular tasks yes; durable tasks must be deterministic | Regular tasks yes; durable tasks constrained (inferred) | Supported; heartbeat unconfirmed | **Best-evidenced fit** — worker-affinity labels | Durable Events named; specifics unconfirmed | Not verified | MIT, Postgres-backed, Docker/k8s, same arch as Cloud | Younger (v1 era), ~8k stars, AI-agent-positioned | Own dashboard, OTel + Prometheus | No general limit (~1MB caution); `put_stream` **~8 KB** (verified bug) |
| **Cadence** | No — same as Temporal (shared lineage) | No, same as Temporal | Yes, same model (inferred) | Sticky task lists (inferred) | Yes — Signals (shared lineage) | Same Patch-style (inferred) | Self-host only, no Cloud; Uber-scale proven internally | Maintained, but ecosystem trails Temporal; not recommended for new projects | Own Web UI, less active (inferred) | Not verified; inferred similar to Temporal |
| **Dapr Workflow** | No — `*workflow.WorkflowContext`; activities get `context.Context` | No in orchestrator; yes in activities | Inferred similar to Durable Task Framework heritage | Not evidenced | `WaitForExternalEvent(name, timeout)` — good fit | Not verified | Dapr sidecar on k8s, CNCF-graduated, any state store; no vendor fee | Dapr mature/CNCF; Workflow block newer/thinner (inferred) | Dapr tooling + state-store's own | Not verified |
| **LittleHorse** | Uncertain — likely a `WorkflowThread` DSL, not native `if`/`for` (inferred, low confidence) | Not verified | Not verified | Not verified | "User Task" marketed (unverified depth) | Not verified | AGPLv3 server + Apache-2.0 SDKs; single Docker image | Small (401 stars), early-stage | Marketed "real-time observability" (unverified) | Not verified |
| **Windmill** | **No** — flow is a DAG of isolated scripts | N/A — each node its own process | Steps can run long; unconfirmed | Not evidenced; generic worker pool | Approval steps, first-class DAG feature | Not verified | AGPLv3, free self-host; Cloud from $10/mo/author; 12,400+ stars (May 2026) | Fast-growing, furthest in model from ordinary-Go | Own UI, live DAG visualization | Not verified |
| **Conductor OSS** | **No** — orchestration is a JSON graph; only workers are Go | N/A for orchestration; workers use ordinary Go | Workers can run long, Conductor-governed (unverified depth) | Not evidenced | Built-in human/wait task types (unverified this session) | JSON definitions versioned server-side; in-flight keeps starting version (inferred) | OSS community edition; Orkes hosts Cloud; Netflix stopped maintaining 2023-12-13 | Long pedigree, momentum now in Orkes | Long-standing Conductor UI | Not verified |
| **Home-grown** | **Yes, by construction** | **Yes** — `Group.Go` already is this | **Already close** — live stdout/stderr + AgentRecord streaming today | **Not solved** — new scheduling work, same scope as Hatchet's/Temporal's | Partially — `Interview`/`Steer` exist; crash/resume boundary is new | Hardest, most universal problem; Gimbal's static graph could diff-and-refuse more legibly, but unbuilt | Cheapest possible — no cluster, just files, *if* recovery+dispatch get built | **Zero** — unbuilt; closest prior art has 0 stars, brand new | Already has the live run page; resume state is incremental | No framework ceiling — but untested at scale |

## Is Temporal the right target?

**Honest verdict: a defensible baseline, not an obviously correct one.**
Temporal wins decisively on the hardest-to-build criteria: mature,
documented versioning, a heartbeat-and-unbounded-timeout Activity model
built exactly for Gimbal's minutes-to-an-hour turns, the deepest Go SDK,
and the best observability of anything surveyed. If the goal is "ship
something a Temporal-experienced platform team can operate with
confidence," it is the safe, well-trodden choice.

But BRIEF.md's own requirement — "the workflow source itself must stay
simple and locally runnable, as today" — is exactly what Temporal fights
hardest. Its Activity half matches `Generate`/`RunCommand`/`Service`
closely. Its Workflow half is the opposite of Gimbal's design: no
`context.Context`, no bare goroutines, deterministic-replay discipline
for exactly the layer (`Scope`, `Group`, `Iterate`, `PromiseLoop`) Gimbal
was built to keep ordinary. A Temporal projector would have to
statically rewrite precisely the constructs `internal/generate` today
only reads and refuses (`select`, bare `go`, `defer`) into Temporal's
replacements — new analyzer capability, not an extension of what exists.
And `PromiseLoop`'s planner-chosen task sequence is the hardest thing to
make replay-safe in Temporal's model, since the planner's choice is
exactly the kind of inline non-deterministic decision Temporal workflows
forbid.

**Strongest case for the top two alternatives:**

**Restate** best matches Gimbal's "no wrappers, ordinary Go" ethos. It
confines determinism to the explicitly-wrapped step
(`restate.Run`/signals/timers) and leaves everything else — plain
goroutines, `context.Context` — untouched. `Generate`/`RunCommand`/
`Check`/`Service` already *are* Gimbal's steps; `Scope`/`Group`/
`Iterate`/`PromiseLoop` already *are* the ordinary surrounding Go. A
projector's job shrinks to wrapping already-identified nodes, not
rewriting the control flow — a smaller, more legible static-analysis
problem than Temporal's. Costs: BSL licensing, unverified versioning/
payload story, no evidenced worker-affinity mechanism (session pinning
still Gimbal's problem), smallest production track record of the
serious contenders.

**Hatchet** best matches operationally. Its worker-affinity labels are
the clearest documented mechanism found for pinning a run's steps to the
host holding a session's workdir — a load-bearing Gimbal requirement
almost nothing else addresses. Its MIT-licensed, Postgres-only self-host
is the cheapest and most operationally familiar serious option, and its
plain/durable task split echoes Restate's confine-determinism approach.
Costs: younger v1 architecture, unclear in-flight-versioning story, a
concrete sourced footgun (`put_stream`'s ~8 KB cap, which would bite a
`Steer`-shaped feature), and a durable-task model this research couldn't
fully pin down.

**Bottom line:** Temporal's case rests on maturity and Activity-model
fit, not on being uniquely compatible with how Gimbal is written — every
engine that scopes determinism to the step (Restate, Hatchet's durable
tasks, DBOS) is structurally closer to "ordinary Go orchestration." And
the most striking finding here is that Gimbal's own design — deterministic
call-site naming, a durable log, a registered static graph — already has
most of what a home-grown memoizing runtime needs, while every external
engine surveyed still leaves the hardest Gimbal-specific requirement
(pinning a session's steps to its workdir host, across a remote pool)
essentially unsolved out of the box. That argues for treating "prove out
home-grown memoization on the existing run log" as at least as urgent a
spike as "build a Temporal projector," before committing to either.

## Risks

- **The orchestration-layer rewrite is the crux, and it's shared, not
  Temporal-specific.** Temporal, DBOS, Cadence, and Dapr Workflow all
  require giving up `context.Context` and bare goroutines in the
  orchestrator. Restate and Hatchet's durable tasks narrow that to the
  wrapped step, but still impose real constraints. No target passes
  `Group`/`Iterate`/`PromiseLoop` through unchanged; the question is how
  much changes and how legibly `internal/generate` can express it.

- **`PromiseLoop`'s adaptive dispatch is the hardest case everywhere,
  including at home.** A planner choosing tasks at runtime is exactly
  what every replay-based system assumes is re-derivable or journaled.
  Design and test this one construct's durability story first, against
  whichever target is chosen, before building the rest.

- **Session/workdir host-pinning is under-solved industry-wide.**
  Hatchet's worker-affinity labels are clearest; Temporal's Session
  feature/sticky queues are next-best but their 2026 status wasn't
  confirmed. Nothing else surveyed — including home-grown today — has
  this built. Budget real design time for it regardless of target.

- **Licensing needs a second look before commitment.** Restate's server
  is BSL, not permissive OSS, for self-hosted production. Windmill's and
  LittleHorse's servers are AGPLv3 — worth legal review if Gimbal would
  ship or link against either rather than run them as an arm's-length
  service.

- **Several findings rest on secondary sources or search summaries, not
  a full primary-doc read**, and need re-verification before being
  load-bearing: Dapr Workflow's exact Go API shape, LittleHorse's actual
  control-flow model (code-first vs. embedded DSL — the single most
  consequential open question left here), Cadence's precise 2026 gap
  versus Temporal, and Inngest's determinism boundary outside
  `step.run()`. Each is flagged inline above.

- **In-flight versioning (f) is the least-evidenced criterion across
  every non-Temporal system.** Temporal is the only engine with mature,
  documented tooling for it — a real point in its favor that neither
  Restate nor Hatchet, this note's strongest structural alternatives,
  currently match. Don't let the structural-fit argument obscure that
  gap.

## Sources

Primary docs/repos fetched or directly quoted this session (accessed
2026-09-27 unless a page states its own date):

- https://docs.temporal.io/develop/go/versioning — verified: GetVersion/Patched, Worker Versioning removal (Mar 2026)
- https://docs.temporal.io/troubleshooting/blob-size-limit-error — verified via search: payload/gRPC size limits
- https://github.com/dbos-inc/dbos-transact-golang — verified: license, stars, last update
- https://pkg.go.dev/github.com/dbos-inc/dbos-transact-golang/dbos — verified: dbos.Context, dbos.Go(), RunAsStep, dialects, Patch()
- https://docs.dbos.dev/production/conductor — verified: Conductor optional, self-host vs hosted
- https://docs.restate.dev/develop/go/services — verified: restate.Context/ObjectContext/WorkflowContext
- https://docs.restate.dev/concepts/durable_building_blocks/ — verified: determinism scoped to restate.Run
- https://github.com/restatedev/restate — verified: star count, self-host options
- https://docs.hatchet.run/v1/architecture-and-guarantees — verified: at-least-once, idempotency, Postgres persistence
- https://github.com/hatchet-dev/hatchet — verified: MIT license, stars/forks, self-host/Cloud shared architecture
- https://github.com/hatchet-dev/hatchet/issues/3087 — verified: put_stream ~8KB limit, filed 2026-02-23
- https://github.com/agenticenv/durable-go — verified: step-memoization model, WithStepVersion, single-writer flock
- https://www.inngest.com/docs/self-hosting — verified: Helm chart, SQLite/Redis defaults, support caveat
- https://github.com/littlehorse-enterprises/littlehorse — verified: dual license, star/commit count
- https://www.windmill.dev/docs/core_concepts/workflows_as_code — verified: flow-as-DAG-of-scripts model
- https://cloudrps.com/blog/durable-execution-restate-dbos-hatchet-beyond-temporal/ — secondary, dated 2026-09-21: cross-engine synthesis

Search-summarized, not directly fetched in full (secondary/lower
confidence, tagged inline above): temporal.io changelog and sdk-go
releases (v1.46.0, July 2026); docs.temporal.io/self-hosted-guide and
/cloud/pricing; automationatlas.io Temporal cost analysis;
docs.dbos.dev/production/hosting-conductor licensing; docs.restate.dev/
develop/go/concurrent-tasks; restate.dev/cloud and Restate Cloud/1.5
blog posts; pkg.go.dev/github.com/inngest/inngestgo and inngestgo GitHub
activity; inngest.com/docs/usage-limits/inngest; docs.hatchet.run/home/
architecture and /home/durable-best-practices; cadenceworkflow.io FAQ
and Cadence-vs-Temporal comparisons; docs.dapr.io workflow pages and
Dapr 1.18/1.18.2 via Wikipedia; oneuptime.com Dapr external-events blog
series (2026-03-31); github.com/conductor-oss/go-sdk README and
docs.conductor-oss.org; automationatlas.io Windmill 2026 overview; arxiv
2609.17645 abstract ("State Without a Landlord").

Read directly from the checkout, not the web: `AGENTS.md`,
`ephemeral/research/temporal-projection/BRIEF.md`, and `go doc -all .`
output for package `gimbal`.
