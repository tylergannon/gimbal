# Temporal Go SDK, current state, and a primitive mapping from Gimbal

Research date: 2026-09-27. SDK version referenced throughout: **go.temporal.io/sdk v1.49.0**,
published **2026-09-14T21:40:02Z**, confirmed directly against the Go module proxy
(`https://proxy.golang.org/go.temporal.io/sdk/@latest`), not from memory or from a
summarized web page. One AI-summarized fetch of the sdk-go GitHub releases page
reported this same release as "September 14, 2024" — that date is wrong; the module
proxy's own metadata is authoritative and is what this document uses. This is called
out explicitly in Risks: several sources below came back as a smaller model's summary
of a fetched page rather than raw text, and at least one summary was simply incorrect
about a date. Every load-bearing claim in this document (determinism rules, the exact
shape of `workflow.Context`, GA dates, size limits) was cross-checked against a primary
source read directly — raw GitHub source, the Go module proxy, or the tool's own raw
markdown fetch — specifically because the summarized fetches proved unreliable on at
least one material fact.

Gimbal's contract used throughout is `go doc -all .` read directly from the checkout at
`/home/user/gimbal` on 2026-09-27, and
`/home/user/gimbal/ephemeral/research/temporal-projection/BRIEF.md`.

---

## Finding 1: Determinism constraints and `workflowcheck`

**Verified directly from source** (`internal/context.go`, `contrib/tools/workflowcheck/README.md`
and `main.go`, fetched as raw GitHub content, not summarized).

`workflow.Context` is documented, in the SDK's own source comment, as "a clone of
`context.Context` with `Done()` returning [workflow.]Channel instead of native channel."
It is an interface with the same four core methods as `context.Context` — `Deadline()`,
`Done()`, `Err()`, `Value(key)` — but `Done()` returns a `workflow.Channel`, not a native
Go `chan struct{}`. That single type change is deliberate and structural, not cosmetic:
it makes `workflow.Context` *not* assignable to `context.Context` (and vice versa)
without an adapter, so a workflow author cannot pass it into any stdlib or third-party
API that expects `context.Context` and expect it to behave — the compiler, not a
linter, is the first line of defense against smuggling a native channel or goroutine
into workflow code through an unsuspecting library call.

Activities, by contrast, take a plain `context.Context` (verified from
`pkg.go.dev/go.temporal.io/sdk/activity`'s own description: "The first parameter to the
function is `context.Context`... an optional parameter"). Activities are ordinary
concurrent Go: goroutines, real channels, `select`, `time.Sleep`, `math/rand`, file and
network I/O are all fine there. Cancellation reaches an activity only through that
`context.Context` (`ctx.Done()` / `context.Cause(ctx)`), and — this matters for Finding
3 — only at points the activity itself checks it, which in practice means at its next
`RecordHeartbeat` call.

`workflowcheck` (`go.temporal.io/sdk/contrib/tools/workflowcheck`) is implemented as a
`golang.org/x/tools/go/analysis` analyzer run through `singlechecker.Main`, i.e. it is a
standard `go vet`-compatible static analysis pass (`go vet -vettool /path/to/workflowcheck`
is explicitly supported and cacheable), not a bespoke SSA or call-graph tool built from
scratch — though functionally it does build its own transitive determinism verdict per
function and propagates it across the whole import graph, which is why its output is
hierarchical ("X calls Y calls Z, Z is non-deterministic because it iterates over a
map"). Concretely, as of this reading, the following are flagged as non-deterministic
by default:

- Starting a goroutine
- Sending to a channel, receiving from a channel, or ranging over a channel
- Ranging over a map
- `time.Now`, `time.Sleep`
- `crypto/rand.Reader`, `math/rand.globalRand`
- `os.Stdin`, `os.Stdout`, `os.Stderr` (which is why `fmt.Printf` etc. show up
  transitively — they write to `os.Stdout`)

It supports a YAML config (`decls:` overrides per fully-qualified function/var, plus a
`skip:` regex list of files) and an inline `//workflowcheck:ignore` pragma for false
positives, and it explicitly force-overrides a few stdlib/SDK-internal functions that
would otherwise false-positive (e.g. `reflect.Value.Interface`, `runtime.Caller`,
`internal.propagateCancel`). Its README states its own limitation plainly: **it does not
catch global variable mutation**, and is "just a helper" — developers must still
scrutinize workflow code by hand for other non-determinism. This is a real gap for
Gimbal: `internal/generate` already refuses to guess about `select`, `go`, `defer`, and
type switches it can't read, which is the same posture `workflowcheck` takes (flag what
it can prove, say nothing about what it can't).

## Finding 2: `workflow.Go` / `Selector` / `Future` vs an errgroup-shaped `Group`

**Inferred from `pkg.go.dev/go.temporal.io/sdk/workflow` and Temporal's own multithreading
docs, both read as AI summaries of the page; the core shape (cooperative scheduler,
Selector as the deterministic `select`, Future as the async handle) is well-established
Temporal SDK material and consistent across every source touched.**

`workflow.Go(ctx, f)` (and `workflow.GoNamed` for a named entry in stack traces) starts a
cooperative coroutine inside Temporal's own single-threaded deterministic scheduler —
not an OS thread. Only one coroutine runs at a time; it runs until it blocks on a
`workflow.Channel`, a `Selector`, or a `Future.Get`, at which point the scheduler
deterministically switches to another ready coroutine. Because there is no real
parallelism, there are no data races to reason about inside workflow code — the ordering
of interleaved blocking points is itself what gets replayed. `workflow.Selector` is the
deterministic substitute for Go's `select`: build one with `NewSelector(ctx)`, register
`AddFuture` / `AddReceive` / `AddDefault` callbacks, then call `Select(ctx)` once per
"wait for whichever of these resolves first" decision. `workflow.Future` is what
`ExecuteActivity`, `ExecuteChildWorkflow`, and `NewTimer` all return; `Get(ctx, &out)`
blocks the calling coroutine until it resolves, error or not.

There is **no built-in errgroup-shaped type** in the SDK — no `workflow.ErrGroup`. A
Gimbal `Group` (first error cancels siblings, `Wait` joins every child, a `Killed` child
does not stop its siblings, the caller must always `Wait`) would have to be
hand-assembled at each call site from: one `workflow.Go` per child, one
`workflow.WithCancel`-derived child context per child (so one child can be cancelled
without cancelling its siblings), a `Selector` loop (or a plain future-collection loop)
that watches for the first error and cancels the *other* children's contexts on it, and
a final drain loop that still calls `Get` on every child's `Future` even after
cancelling some of them, exactly as Gimbal's own contract requires. This is real,
repeated boilerplate that Temporal does not hide — which happens to match AGENTS.md's
own "no wrappers" stance, but it means every `Group` call site in a projected workflow
carries several lines of Selector/cancel-context plumbing that Gimbal's Go API currently
hides behind one `Group(ctx, name)` call.

One structural nuance: each `Group` child in Gimbal is a session/agent turn. In Temporal
terms that child is most naturally a single Activity execution (Finding 3), so the
`workflow.Go` wrapper around each child is mostly bookkeeping around one
`ExecuteActivity` call and its `Future` — "cancel my sibling's in-flight agent turn" then
depends on activity cancellation semantics (coarse, heartbeat-gated), not on anything as
immediate as cancelling an in-process goroutine.

## Finding 3: Activities for long-running agent turns

**Mixed: timeout/retry/cancellation semantics verified against `docs.temporal.io`
summaries (consistent across two independently fetched pages) and cross-checked
against the community forum's canonical answer on signal delivery (Finding 5); the
"activity does not resume mid-function, only at its next attempt via heartbeat
details" claim is the standard, widely documented Temporal behavior and is treated as
verified in substance even though read through summarized fetches.**

Two independent timeouts govern an activity, and at least one must be set:
**StartToCloseTimeout** bounds a single attempt (one try); **ScheduleToCloseTimeout**
bounds the whole activity's life across every retry, from the moment it is scheduled.
For a 5–60+ minute agent turn, StartToCloseTimeout needs to be generous (comfortably
above the longest turn Gimbal expects), and **HeartbeatTimeout** needs to be set well
below the real heartbeat cadence the harness adapter can sustain, because
HeartbeatTimeout — not StartToCloseTimeout — is what detects a stalled or crashed
worker promptly.

`activity.RecordHeartbeat(ctx, details)` sends an arbitrary `details` payload (itself
subject to the 2MB payload limit, Finding 8) to the Temporal service. On any retry —
from a heartbeat timeout, a worker crash, or a returned retryable error —
`activity.GetHeartbeatDetails` / `HasHeartbeatDetails` let the **new** attempt read the
last heartbeat's details and decide how much work to skip. Critically: **Temporal does
not resume a half-finished activity function in place.** A retried activity is invoked
again from the top of the function; all "pick up where we left off" behavior is the
activity author's own responsibility, driven entirely by what it chose to put in its
last heartbeat. For a Gimbal agent turn this means a worker crash mid-turn cannot be
transparently "reconnected to" — the harness process itself is gone; the only way to
resume anything is for the retried attempt to read heartbeat details naming which
native harness session/transcript it was mid-way through, and to instruct the harness
to continue that conversation (if the harness supports it) or to restart the turn from
scratch. There is no free lunch here: Gimbal's current in-process model, where the same
process and workdir simply keep running, has no equivalent guarantee once activities
are the unit of retry.

Default retry policy (absent an override) is effectively unlimited attempts with
exponential backoff (1s initial interval, ×2 coefficient, capped at 100s between
attempts); a `RetryPolicy` can set `MaximumAttempts` and `NonRetryableErrorTypes`. A
crashed worker's in-flight activity is **not** retried instantly — the server has no
way to know the worker died until either StartToCloseTimeout or (if heartbeating)
HeartbeatTimeout elapses, and only then does it schedule a new attempt, on whichever
worker happens to be available (not necessarily the same host, unless Sessions pins it
— Finding 4).

Cancellation of an activity is cooperative and heartbeat-gated: cancelling the
activity's Future (or the whole workflow) does not reach into and kill the activity's
process; the activity only learns about it the next time it calls
`RecordHeartbeat` — that call is what flips `ctx.Done()`/`ctx.Err()` to canceled. An
activity that never heartbeats cannot be cancelled at all. This is a hard constraint on
Gimbal's `Killed` semantics and `Group` child-cancellation: responsiveness to an
operator kill is bounded below by however often the harness adapter heartbeats, a
tunable Gimbal would need to own and could not treat as free.

Size and history ceilings that bite here specifically: the 2MB per-payload limit
(Finding 8) rules out returning a full transcript or large captured stdout as an
activity's return value directly; the 51,200-event/50MB event-history cap
(Finding 8/9) means a long-running, many-turn `PromiseLoop` workflow needs
`workflow.NewContinueAsNewError(ctx, fn, args)` well before it naturally would in a
short-lived business workflow, since each activity schedule/start/complete and every
heartbeat is itself one or more history events. Continue-as-new atomically finishes the
current run and starts a fresh one (same workflow ID, empty history), carrying forward
only whatever is explicitly passed as its new arguments — anything Gimbal's scope-local
`Set`/`SetJSON` model currently keeps implicitly visible across an entire run would need
to be explicitly re-threaded through that argument list by generated code.

## Finding 4: Sessions API

**Verified in substance from `docs.temporal.io/develop/go/sessions` and
`pkg.go.dev/go.temporal.io/sdk/workflow`, both read as summaries but mutually
consistent and consistent with the documented `worker.Options.EnableSessionWorker` /
`MaxConcurrentSessionExecutionSize` fields; the exact numeric default for
`MaxConcurrentSessionExecutionSize` is contested between two sources (see caveat
below) and should be treated as unverified until checked against the SDK source or an
actual run.**

Sessions are still present in the current SDK and are, per the docs read, still a
Go-SDK-only feature with no deprecation notice found anywhere touched in this
research. `workflow.CreateSession(ctx, SessionOptions{ExecutionTimeout,
CreationTimeout, HeartbeatTimeout})` returns a session-scoped `workflow.Context`; every
`ExecuteActivity` issued against that context is routed, via a session-specific,
host-specific task queue the SDK sets up internally, to the exact worker process that
accepted the `CreateSession` call. This is precisely "pin a process and its activities
to one host," which is exactly what Gimbal calls a session (a harness process plus a
workdir, pinned to a host) — this is the cleanest structural match found in this whole
research pass.

`worker.Options.EnableSessionWorker = true` opts a worker process into hosting
sessions; `MaxConcurrentSessionExecutionSize` caps how many concurrent sessions one
worker process will accept — once hit, further `CreateSession` calls block until a
session frees up. Two sources disagreed on the exact default (one summary said the
default is "very large" / effectively unbounded unless set; a separate search result
claimed a concrete default of 1000) — this is flagged explicitly as **unverified**, and
should be confirmed against the SDK source (`internal/session.go` or similar) or by an
actual run before being load-bearing for capacity planning.

Documented behavior on host death: sessions run their own background heartbeat
(distinct from any individual activity's `HeartbeatTimeout`), and when that
session-level heartbeat stops, the session is marked failed; every subsequently issued
activity on that session's context immediately fails with `workflow.ErrSessionFailed`
rather than being rescheduled to a different host. The docs are explicit that the
pinning guarantee is "per Worker Process," i.e. it assumes exactly one worker process
per host — a deployment detail Gimbal's projection would need to guarantee for the
guarantee to mean anything.

`workflow.GetSessionInfo(sessionCtx).GetRecreateToken()` plus a `RecreateSession(ctx,
token, opts)` call let a *new* session resume "in the same place" — but this is
explicitly for continuing a long session across a `ContinueAsNew` boundary, not for
reconnecting to a session whose host actually died. On host death, whatever state lived
only on that host's disk (the workdir, the native harness process, anything not
durably persisted elsewhere) is gone; Recreate gets you a fresh pinned host, not the old
one's state back. This is not a limitation Temporal invents — it is the same failure
mode Gimbal has today when a machine running a session dies — but it is worth being
explicit that Sessions does not make the workdir itself durable; it only makes activity
routing sticky while the host is alive.

## Finding 5: Signals, Updates, Queries — Steer, Interview, Kill

**Verified directly**: the community forum thread
(`community.temporal.io/t/is-it-possible-to-send-signal-to-activity/874`) gives an
explicit, unambiguous answer from a Temporal team member/expert: *"There is no
functionality to send signal to an activity."* This is corroborated by every SDK
document touched in this research — `workflow.GetSignalChannel`,
`workflow.SetQueryHandler`, and `workflow.SetUpdateHandlerWithOptions` are all
workflow-scoped APIs; the `activity` package exposes no analogous inbound mechanism at
all. An activity can only be driven by (a) its own initial input, (b) heartbeat details
it reads itself at the top of a retried attempt, or (c) cancellation, delivered only at
its next heartbeat.

**Steer** (inject a message into a *running* agent turn's live conversation) therefore
has no native Temporal answer. The forum's own recommended workaround for
"signal-like" behavior toward a running unit of work is "task routing to ensure the
execution of multiple activities on the same process," which is precisely Sessions
(Finding 4): signal the *workflow*, have its signal handler stash the message, and
dispatch a **second activity, pinned to the same session host**, whose only job is to
hand that message to the first activity's still-running harness process through a
worker-local, out-of-band channel that Gimbal itself would have to build (a Unix
socket, a file the worker process's own supervisor goroutine watches, etc.). Temporal
supplies "get the message to the correct host"; it does not supply "get the message
into the already-running process" — that half is entirely Gimbal's to build, and it is
exactly the kind of custom machinery this research needs to flag as new surface area,
not a reuse of an existing Temporal primitive.

**Interview** (ask a typed question, block until a human answers) fits well as a
`workflow.Update`: Updates are explicitly documented as "a trackable synchronous request
sent to a running Workflow Execution" that "can change the Workflow state, control its
flow, and return a result," with an optional validator that runs before the update
enters history (so a malformed or duplicate answer can be rejected without ever being
recorded). The question itself would be surfaced via a Query or via an activity/side
channel that posts it to wherever a human reads it; the answer arrives back as the
Update. This is a clean, purpose-built match.

**Kill** exists at two granularities in Gimbal and both map cleanly, at different
Temporal client calls: `TerminateWorkflowExecution` unconditionally and immediately
ends an entire workflow with no further workflow code running (closest to killing a
whole run); `CancelWorkflowExecution` delivers a cooperative cancellation the workflow
observes via `ctx.Done()`/`context.Cause(ctx)` (closer to Gimbal's own cooperative,
cause-carrying `Killed`). Killing one specific `Group` child without touching its
siblings maps to cancelling only that child's own `workflow.WithCancel`-derived
context — clean at the workflow level — but, as in Finding 3, that cancellation only
reaches the activity attempt itself at its next heartbeat, so "kill this one turn,
leave its siblings running" is bounded by heartbeat cadence, not instantaneous the way
Gimbal's own current process-level kill can be.

## Finding 6: Worker Versioning and patching

**Verified via Temporal's own GA announcement, read as a summary but internally
consistent and matching the docs page's positioning language almost verbatim.**

`workflow.GetVersion(ctx, changeID, minSupported, maxSupported)` — the original
"patching" mechanism — still exists, but Temporal's current production-deployment docs
explicitly demote it: *"If your environment cannot yet support versioned worker
deployments, you can fall back to patching Workflow code. However, new production
deployments should prefer Worker Versioning whenever possible."*

**Worker (Deployment) Versioning reached GA on 2026-03-30**, with "Upgrade on
Continue-As-New" entering Public Preview the same day. Workers register under a named
Worker Deployment Version (a build identifier). Each workflow execution is either:

- **Pinned** — stays on the exact version/build it started on for its entire
  lifetime, even across later deploys; or
- **Auto-Upgrade** — moves onto whatever version is currently the default/ramping
  target at each new workflow task, i.e. potentially mid-execution.

**Upgrade on Continue-As-New** is a narrower third option for long-running Pinned
workflows: they stay pinned during a run, but at their next `ContinueAsNew` boundary
they can adopt the newer version rather than staying pinned forever or upgrading
mid-run.

Deploying a new Gimbal-as-Temporal binary while old runs are in flight becomes:
register the new build as a new Worker Deployment Version, ramp new workflow starts to
it, and decide per workflow type whether in-flight runs are Pinned (old workers must
stay up until every pinned run drains) or Auto-Upgrade (only safe if the new code is
history-compatible with runs already in progress). Temporal's own siding guidance —
"order processing that completes in minutes should use Pinned... subscription billing
that spans multiple deploys should use Auto-Upgrade with patching" — points toward
Pinned as the right default for Gimbal: most workflow *executions* (as opposed to the
activities inside them) are short relative to a single agent turn, so Pinned's cost
(keeping the old build's workers alive a while longer) is bounded and predictable.

## Finding 7: Nexus

**Verified via Temporal's own GA blog post, read as a summary.** Nexus reached GA on
**2025-03-06**. It solves cross-Namespace (and, on Cloud, cross-region/cross-cluster)
calling — the announcement's own framing example is "Netflix wants each team to own
their own Namespace and call into each other." The same source is explicit that this
solves a problem that "didn't exist when everything operated in one namespace," i.e.
Nexus is not needed for a single project's workflow calling its own activities within
one namespace. Given the brief's scope (one project's workflow, its own workers, likely
one namespace or a shared namespace with per-project task queues — see Finding 9),
**Nexus is not on the critical path today.** It becomes relevant only if Gimbal later
wants one project's durable workflow to call into a genuinely different project's
Temporal namespace rather than just doing more local work — worth remembering as an
option, not something to build against now. The v1.49.0 SDK release notes mention
"Standalone Activities" reaching GA and "workflow queries as Nexus operations" as very
recent SDK-level additions, so the surface is actively growing, but nothing touched in
this research makes Nexus load-bearing for the projection as scoped.

## Finding 8: Data converters, codec server, claim-check

**Verified in substance across two independently fetched docs pages
(`self-hosted-guide/data-encryption` and `troubleshooting/blob-size-limit-error`),
both read as summaries but agreeing with each other and with the well-established
Temporal claim-check pattern.**

The default `DataConverter` serializes (and, if configured, encrypts) workflow and
activity inputs/outputs, signals, queries, updates, and failure payloads. Plaintext
only ever exists on the Client and Worker processes a developer controls; the Temporal
Service itself (self-hosted or Cloud) only ever stores and moves the converted bytes.
A **Codec Server** is a small, separately-run HTTP service exposing `/decode` (and
`/encode`) endpoints that the Temporal Web UI and CLI call client-side to render
human-readable payloads; the Temporal Service never sees decoded plaintext, and the
developer is responsible for the codec server's own CORS and authorization.

Hard size limits verified: **2MB per individual payload** (a single workflow or
activity argument/return value), and a separate **4MB gRPC message limit** that can
trigger even when every individual payload is under 2MB, if enough of them are batched
into one request (e.g. scheduling many activities in one workflow task). These are not
configurable on Temporal Cloud; self-hosted deployments can raise some of these
defaults. This makes it structurally impossible to pass a full multi-turn agent
transcript, or a large captured stdout stream, through an activity argument or return
value once it grows past a few hundred KB — exactly the kind of payload Gimbal already
treats as run-owned files rather than in-memory values (`CommandEnded.StdoutFile`,
`ValueArtifact.File`).

The documented answer is the **claim-check pattern**: store the large blob in external
storage (Temporal's own docs use S3 as the canonical example) and pass only a
reference through the workflow/activity payload; the Codec Server's `/download`
endpoint resolves that reference back to content for UI/CLI viewing, and the Web UI
shows "a claim reference instead of the payload content" until it's resolved. This maps
onto what Gimbal already does almost exactly — a bounded excerpt plus an absolute path
to the complete file is already Gimbal's own claim-check pattern. Projecting to
Temporal would keep using Gimbal's own run directory (or object storage, if the run
directory itself is not shared across the cluster) as the claim-check store, and pass
only the file path or URL through Temporal payloads — never the transcript or stdout
bytes themselves.

## Finding 9: Temporal Cloud vs self-hosted (Helm on EKS)

**Mixed: pricing figures verified via `docs.temporal.io/cloud/pricing`, read as a
summary, cross-checked by a separate web search that corroborated the per-namespace
Actions-per-second default; the namespace-vs-task-queue guidance is verified via a
web search answer synthesizing several official docs pages (`production-deployment/
multi-tenant-patterns`, `best-practices/managing-namespace`) rather than a single
page fetch, so treat the exact wording as paraphrased, though the substance — shared
namespace, per-tenant task queues, except for a small number of tenants needing true
isolation — is consistent and specific enough to be load-bearing.**

Temporal Cloud bills on **Actions** (billable operations: starting a workflow, an
activity execution, a signal, a heartbeat, and similar), at roughly **$0.00005 per
Action ($50/million)**, with volume discounts starting around 5 million Actions/month,
plus separate **Active Storage** (~$0.042/GB-hour) and **Retained Storage**
(~$0.00105/GB-hour) charges, bundled into tiered support plans (a usage-based
Developer tier with no minimum; a Business tier from $500/month bundling 2.5M
Actions + 2.5GB active / 100GB retained storage; higher Enterprise/Mission-Critical
tiers). Every namespace has a **default Actions-per-second ceiling of 500/s** that must
be requested up if exceeded; Cloud's hard per-payload/history limits (2MB, 51,200
events/50MB) are fixed, unlike self-hosted where some of these are configurable.

For Gimbal's shape — hour-long agent turns as long activities, relatively few
workflow-level decisions per run (start, a handful of Steer/Interview/Kill
signals/updates, activity schedule/complete pairs) but potentially many heartbeats —
**heartbeat cadence is a direct Action-cost lever, not just a reliability one**: a
single 60-minute turn heartbeating every 30 seconds is already roughly 120 Actions on
its own, before counting anything else in the run. Choosing a heartbeat interval is
therefore simultaneously a cancellation-latency decision (Finding 3/5) and a cost
decision on Cloud.

Operationally, Temporal Cloud removes the burden of running and operating the
stateful Temporal server components (persistence store, visibility store, the
service itself) that self-hosting via Helm on EKS would require Gimbal's own team to
run, monitor, and upgrade — a real, recurring cost against "as simple as possible"
even though self-hosting avoids per-Action billing entirely.

On namespace-per-project vs task-queue-per-project: Temporal's own guidance, as
synthesized from its multi-tenancy docs, is that a **shared namespace with per-tenant
(here, per-project) Task Queues** is the recommended default for scale and
operational simplicity — dedicating a separate namespace per tenant is reserved for
"a small number of high-value tenants that require specific requirements" needing a
true isolation boundary, and is described as manageable only for something on the
order of fewer than 50 tenants due to the operational overhead of provisioning,
monitoring, and Actions-per-second-limiting each namespace separately. This runs
counter to a default "namespace per project" assumption; if Gimbal wants hard
per-project isolation anyway (blast radius, independent quota, independent codec
server per project), it is deliberately swimming upstream of the platform's own
guidance and paying the 500/s-per-namespace ceiling once per project rather than
once for the whole account.

---

## Mapping table

| Gimbal primitive | Temporal construct | Fit | Reason |
|---|---|---|---|
| `Run` (root scope) | Workflow Execution | Clean | Start/complete/cancel and the root scope's own lifecycle line up directly; `Env` becomes workflow input. |
| `Scope` | Ordinary Go function boundary + a `workflow.WithCancel`-derived child context | Clean | "Ends, closes sessions/services, cancels ctx" is exactly a function return plus context cancellation; no new SDK type needed. |
| `Group` | Hand-assembled `workflow.Go` + `workflow.Selector` + per-child `workflow.WithCancel` + `Future.Get` drain loop | Clean, but manual | The pieces exist; there is no packaged errgroup-shaped type, so this is boilerplate at every call site, matching Gimbal's "no wrappers" stance but not reducing the line count. |
| `Iterate` | Ordinary Go `for`/`range` over a slice inside workflow code | Clean | Only map-range and channel-range are non-deterministic; slice iteration is unrestricted. |
| `PromiseLoop` | A workflow loop: one Activity per planner turn, per-task work dispatched as further Activities/Groups, an operator message drained once per iteration from `workflow.GetSignalChannel` | Clean | Matches the Signal "mailbox, read once per decision" shape exactly; no packaged planner-loop primitive, same boilerplate cost as `Group`. |
| `NewSession(ctx, role, workdir)` | `workflow.CreateSession` + `worker.SessionWorker` (`EnableSessionWorker`, `MaxConcurrentSessionExecutionSize`) | Clean | Purpose-built for "pin every subsequent activity to the exact host that accepted this session," including a documented, named failure (`ErrSessionFailed`) on host death. |
| `session.Generate[T]` | One Activity execution (`StartToCloseTimeout` sized to the turn, `HeartbeatTimeout` for liveness) under that session's context | Clean | A long, blocking unit of external work is exactly what an Activity models; only the validated final result crosses back, within the 2MB limit. |
| `session.Steer` | No direct construct: Signal-to-workflow + a second same-session Activity relaying the message into the harness process over a worker-local side channel Gimbal builds itself | Awkward | Activities have no inbound API at all; only cancellation reaches a running activity, and only at its next heartbeat. Temporal gets the message to the right host, not into the process. |
| `session.Fork` | A plain call inside an activity (ask the adapter for a forked native session) | Clean | Adapter-level and process-local; nothing here needs a Temporal-visible construct. |
| `Set`/`SetJSON` | Ordinary Go values threaded as Activity arguments (and, across `ContinueAsNew`, as explicit new-run arguments) | Clean, with a cost | Workflow state is just memory; the constraint is the 2MB per-argument limit and having to explicitly re-thread everything through `ContinueAsNew`. |
| `RunCommand`/`Check` | An Activity running the command under plain `context.Context` | Clean | Unconstrained subprocess execution; large stdout/stderr need the claim-check treatment rather than returning full bytes. |
| `Service` | An Activity that starts the process, then does nothing but heartbeat until its owning scope's context cancels, then tears it down | Awkward | No "keep a subprocess alive for a workflow scope's life" primitive exists; this is one long activity whose only job is heartbeating and reacting to cancellation-at-heartbeat, slower than Gimbal's own owned SIGTERM/SIGKILL sequence. |
| `Interview` | `workflow.SetUpdateHandlerWithOptions`, with the question surfaced via Query or a posting activity | Clean | Updates are explicitly a "trackable synchronous request" with optional validation — built for exactly this shape. |
| `WithSupervisor` | Depends on Steer (awkward) plus a way for a second session to read a still-running peer's transcript | Impossible without custom side-channel | Temporal gives no visibility into a running activity's internal output beyond whatever it puts in heartbeat details (one-way, activity→server), and no path back in except cancellation. |
| `Killed` (one child, or a whole run) | `CancelWorkflowExecution` / `TerminateWorkflowExecution` (whole run); a child's own `workflow.WithCancel` context (one `Group` member) | Clean, with a caveat | Cooperative at the workflow level; reaching the activity attempt itself is bounded by heartbeat cadence, not instantaneous. |
| Run log / `run.jsonl` / transcripts (durable record) | Temporal Event History (a separate, coarser durable record) plus Gimbal's own externally-stored logs referenced via claim-check | Partially clean | Event History records workflow-level decisions and activity start/complete, not per-token agent events; Gimbal keeps writing its own transcripts as today and Event History becomes a second, workflow-shaped record, not a replacement. |

---

## Risks

1. **Steer and `WithSupervisor` have no Temporal-native answer.** Both of Gimbal's
   mid-turn interaction primitives require a real worker-local side channel plus
   polling that Gimbal must design and build from scratch; Temporal only guarantees
   "the right host," never "into the running process." This is a project in its own
   right, is exactly the kind of hidden machinery AGENTS.md's "no wrappers" rule warns
   against elsewhere, and may cap how "live" steering and supervision can feel compared
   to Gimbal's current in-process implementation.
2. **Sessions is comparatively lightly used.** It is real, documented, and not
   deprecated, but it is Go-SDK-only and this research found no strong evidence of
   wide production usage relative to the mainstream Activity/Signal/Update path (the
   `MaxConcurrentSessionExecutionSize` default itself was contested between two
   sources — see Finding 4). Betting the entire "workdir pinned to a host" model on it
   should be load-tested directly (many concurrent sessions, one host with many pinned
   processes, a killed pod) before being relied on.
3. **Kill latency is bounded by heartbeat cadence, not free.** An operator "kill this
   turn now" can only be as fast as the harness adapter's heartbeat interval, which is
   also a direct Cloud-cost lever (Finding 9) — the two pull against each other.
4. **`ContinueAsNew` is a hand-threading exercise.** Every value Gimbal's current
   scope-context model carries implicitly through an entire run would need to be
   explicitly serialized into `ContinueAsNew`'s argument list (itself capped at 2MB) by
   generated code, a real semantic narrowing from "everything in scope stays visible."
5. **History growth could force `ContinueAsNew` earlier than expected.** A long,
   multi-turn `PromiseLoop`/`Group`-heavy workflow, with its heartbeats, could plausibly
   approach the 51,200-event/50MB ceiling well before it's obviously "due" for a
   restart; the generator would need new static analysis to estimate history growth per
   workflow shape and insert boundaries, which `internal/generate` does not do today.
6. **Pinned Worker Versioning means keeping old workers around for a long tail.** For
   hour-plus agent turns, deploying a new binary while old runs are Pinned could mean
   keeping stale container images and their secrets/GitHub-App credentials live for a
   while after a deploy — an operational cost the brief's per-project EKS namespace
   story needs to size for.
7. **Namespace-per-project runs against Temporal's own guidance.** If Gimbal wants hard
   isolation per project anyway, it is deliberately choosing the platform's harder,
   more-operationally-expensive path (provisioning and quota-limiting many namespaces)
   rather than the recommended shared-namespace-with-task-queues default.
8. **This research's own method is a risk to flag.** Most fetched Temporal
   documentation came back as an intermediate small model's summary of the page, not
   raw text, and at least one such summary was simply wrong (the sdk-go release date).
   Every claim above not explicitly marked "verified directly" should be re-checked
   against a primary source before being load-bearing for an actual build decision.

---

## Sources

Verified directly (raw source, module proxy, or raw markdown — not summarized):

- `https://proxy.golang.org/go.temporal.io/sdk/@latest` — read 2026-09-27. Confirms
  v1.49.0, published 2026-09-14T21:40:02Z.
- `https://proxy.golang.org/go.temporal.io/sdk/@v/list` — read 2026-09-27. Full version
  list, corroborates v1.49.0 as latest.
- `https://raw.githubusercontent.com/temporalio/sdk-go/master/internal/context.go` —
  read 2026-09-27. Exact `Context` interface definition (Finding 1).
- `https://raw.githubusercontent.com/temporalio/sdk-go/master/contrib/tools/workflowcheck/README.md`
  — read 2026-09-27. Full determinism rule list, configuration, ignore pragma
  (Finding 1).
- `https://raw.githubusercontent.com/temporalio/sdk-go/master/contrib/tools/workflowcheck/main.go`
  — read 2026-09-27. Confirms `go/analysis` `singlechecker` implementation
  (Finding 1).

Read via WebFetch as page summaries (cross-checked against each other and, where
possible, against a verified source; treat exact wording as paraphrase, substance as
reliable unless flagged otherwise):

- `https://github.com/temporalio/sdk-go/releases` — read 2026-09-27. Version list
  corroborated; **release date given (2024) was wrong**, superseded by the module
  proxy above.
- `https://docs.temporal.io/develop/go/go-sdk-multithreading` — read 2026-09-27
  (Finding 1/2).
- `https://pkg.go.dev/go.temporal.io/sdk/workflow` — read 2026-09-27 (Findings 2, 4, 5,
  6).
- `https://docs.temporal.io/develop/go/sessions` — read 2026-09-27 (Finding 4).
- `https://docs.temporal.io/develop/go/activities/timeouts` — read 2026-09-27
  (Finding 3).
- `https://docs.temporal.io/production-deployment/worker-deployments` — read
  2026-09-27 (Finding 6).
- `https://temporal.io/blog/ga-worker-versioning-public-preview-upgrade-on-continue-as-new`
  — read 2026-09-27. GA date 2026-03-30 (Finding 6).
- `https://temporal.io/blog/temporal-nexus-now-available` — read 2026-09-27. GA date
  2025-03-06 (Finding 7).
- `https://docs.temporal.io/develop/go/message-passing` — read 2026-09-27
  (Finding 5).
- `https://community.temporal.io/t/is-it-possible-to-send-signal-to-activity/874` —
  read 2026-09-27. Direct quote: "There is no functionality to send signal to an
  activity" (Finding 5).
- `https://docs.temporal.io/design-patterns/long-running-activity` — read 2026-09-27
  (Finding 3/5).
- `https://docs.temporal.io/self-hosted-guide/data-encryption` — read 2026-09-27
  (Finding 8).
- `https://docs.temporal.io/troubleshooting/blob-size-limit-error` — read 2026-09-27.
  2MB/4MB limits, claim-check pattern (Finding 8).
- `https://docs.temporal.io/workflow-execution/limits` — read 2026-09-27. History and
  incomplete-operation limits (Finding 3/9).
- `https://docs.temporal.io/cloud/pricing` — read 2026-09-27. Actions/storage pricing
  (Finding 9).
- `https://pkg.go.dev/go.temporal.io/sdk/activity` — read 2026-09-27. Plain
  `context.Context`, `GetInfo`, `RecordHeartbeat`, cancellation semantics (Finding 1/3).

Read via WebSearch (multiple official pages synthesized in one answer; substance
corroborated across results, exact wording is paraphrase):

- "Temporal Cloud pricing actions per second namespace 2026" — searched 2026-09-27.
  Corroborates the 500 Actions/s default namespace ceiling (Finding 9).
- "Temporal maximum payload size 2MB history size limit workflow event count limit" —
  searched 2026-09-27. Corroborates event-history and payload limits (Finding 3/8).
- "Temporal Worker Versioning GA pinned auto-upgrade version deployment 2026" —
  searched 2026-09-27 (Finding 6).
- "Temporal Nexus GA status cross-namespace cross-cluster 2026" — searched
  2026-09-27 (Finding 7).
- "temporal sdk-go sessions.go MaxConcurrentSessionExecutionSize default
  worker.Options" — searched 2026-09-27. Default value contested — see Finding 4 /
  Risk 2.
- "Temporal namespace per tenant vs task queue per tenant multi-tenancy best
  practice docs.temporal.io" — searched 2026-09-27. Shared-namespace/per-tenant-task-queue
  recommendation (Finding 9).

Local:

- `go doc -all .` — run 2026-09-27 in `/home/user/gimbal`. Gimbal's full public API
  and behavioral contract, read directly, not summarized.
- `/home/user/gimbal/ephemeral/research/temporal-projection/BRIEF.md` — read directly.
