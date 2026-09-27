# R1.3/R2.4: Steer, Kill, and WithSupervisor across the activity/worker boundary

Date: 2026-09-27. Scope per SYNTHESIS.md: risk R1.3 (shared with R2.4) for
Finalist 1 (runtime projection) and Finalist 2 (source projection) alike,
since both put the worker's own turn on a remote host and both keep
`WithSupervisor`'s in-process code as the only implementation that exists.
Gimbal claims below are source-only, cited file:line, read directly this
session. Temporal claims are carried forward from round 1's already-verified
findings (temporal-go-sdk.md, agents-on-temporal-prior-art.md), not
re-fetched; this note's job is applying them to a concrete design, not
re-sourcing them.

## Verdict

**Option A — the whole `Generate` call, supervisors included, runs inside the
activity on the worker; Steer/Kill/Interview reach the worker directly over
HTTP/gRPC, not through Temporal — is the only option that keeps today's
semantics, and it is what Finalist 1 already implies.** Round 1 already
showed B (Temporal Update relay) and C (queue-and-poll) cannot reproduce
`Session.Steer`'s "lands inside the still-open harness connection, this turn"
contract, because Temporal only ever routes a message to the right *host*,
never into the *process* (temporal-go-sdk.md Finding 5; prior-art.md Findings
6-9). Gimbal's existing in-process code is itself the missing half Temporal
doesn't supply. This note confirms A is buildable at acceptable cost, and
sizes what B/C would give up instead of re-litigating whether they could
match fidelity — they can't, by construction.

**R1.3 is reduced, not eliminated.** Under A, Steer/Kill/Interview keep their
current in-process semantics for one turn, but Kill's network path (instance
to worker) adds a failure mode today's single process doesn't have
(NetworkPolicy misconfiguration, unreachable worker, Session-host death), and
Interview needs the worker or a shared component to expose the equivalent of
`interviewWaiter` across that boundary — new code, not a reuse of
`run.go:307-345`. The residual risk lives entirely in that new surface, not
in whether mid-turn interaction survives the move.

## Gimbal facts (source, file:line)

### What runs inside one `Generate` call today

`Session.Generate[T]` (`session.go:87-95`) renders scope context onto the
prompt (`session.go:101-126`), then `dispatchRecorded` (`session.go:168-174`)
calls `generate[T]` (`session.go:185-223`) without supervisors. That loop
retries an invalid result up to `generateAttempts=3` and a Claude protocol
string up to `protocolAttempts=2` (`session.go:182-183,185-222`), each retry
re-asking with a prompt built from the *previous* attempt's own error text
(`session.go:191-196`) — not idempotent, must run once, in one place, to
completion. `(*Session).turn` (`session.go:228-391`) takes `s.mu`, sets
`s.running=true`, computes `turnID` (`session.go:229-239`); lazily calls
`adapter.CreateSession` on the first turn only (`session.go:248-256`);
registers a per-turn cancellable context in `run.turns` keyed by `turnID`
(`session.go:317-319`, `run.go:291-301`); calls
`adapter.RunTurn(turnCtx, native, prompt, schema, wrapped)`
(`session.go:321`), where `wrapped` stamps and writes every native event to
the session log and observation store (`session.go:274-296`,
`event_persistence.go:154-187`); clears `s.running/s.turnID/s.activeEmit` on
return (`session.go:240-246`). Live in-process state a call into this session
depends on: `s.running`, `s.native`, `s.turnID`, `s.activeEmit`
(`session.go:25-37`).

With supervisors, `dispatchRecorded` calls `supervise[T]`
(`session.go:169-170`, `supervise.go:108-123`), which writes a
`SuperviseAttached` record per supervisor (`supervise.go:110-117`) then
dispatches to `superviseTimed` or, with `TYPESAFE_API_KEY` set,
`superviseWithJev` (`supervise.go:119-122`). Both spin one goroutine per
supervisor (`supervise.go:136-165`, `supervise_jev.go:232-293`) that: shares
a bounded `transcript` fed by the worker's own `onEvent` callback
(`supervise.go:126,166-167`, `supervise_jev.go:296-299`); on a timer tick
(`WithInterval`, default 3 min, `supervise.go:79-81,138`) or a Jev-screened
`session.reasoning.ended` item over threshold (`supervise_jev.go:80-120,
157-176`), renders new transcript entries via `t.look` (`supervise.go:
482-505`); dispatches that look as its own turn via `dispatch[review]`
(`supervise.go:152`, `supervise_jev.go:194,266`); and, on objections, calls
`s.Steer` on the worker session directly, in-process, from that goroutine
(`supervise.go:161`, `supervise_jev.go:208,281`). `Generate` returns only
after every supervisor goroutine is cancelled and joined (`supervise.go:
130-135`, `supervise_jev.go:300-306`).

**Net: nothing about "one Generate call including supervisors" is
request/response.** The worker's own turn is a clean activity boundary; the
supervisor path is a live, concurrent side channel — one goroutine watching a
shared in-memory `transcript`, calling `Steer` on the same `*Session` the
worker's own `turn` goroutine is blocked inside — requiring both halves to be
the same process holding the same adapter connection
(`gimbal-internals-audit.md §2`, "the single biggest mismatch found in this
audit").

### What Steer needs

`Session.Steer` (`session.go:552-596`) reads `s.native/s.running/s.turnID/
s.activeEmit` under `s.mu` (`session.go:553-555`), returns immediately
(`landed=false, err=nil`) with no turn running (`session.go:562-566`), else
writes a `session.inbox.enqueued` record via the session's own emit closure
(`session.go:568-575`) and calls `s.adapter.Steer(ctx, native, message)`
(`session.go:576`) — the `HarnessAdapter` method reaching the live harness
connection (codex's shared daemon, opencode's shared server, pi's in-process
object; audit §3). Steer's own doc comment says "The message lands at the
worker's next model call" — Gimbal already treats Steer as queued for the
next model call, the shape Temporal's ecosystem converges on
(prior-art.md Finding 6), except Gimbal delivers it through a live adapter
connection in the same process, not by waiting on a workflow-level decision.

`run.turns` (`run.go:76,291-301,427-442`) is what `CancelTurn` uses:
`(*run).CancelTurn(id, cause)` looks up the turn's own
`context.CancelCauseFunc` and calls it directly (`run.go:427-441`) — no
polling, no heartbeat. `turnCtx` (`session.go:317`) is cancelled and
`adapter.RunTurn` observes it through its own `ctx.Done()` handling,
synchronously.

### What the live web page's controls call

The page's controls are SKGO remotes resolving `live.RunsFrom(ctx)`
(`internal/live/live.go:57-77`) to a `live.Controller`, then calling one
method directly:

- `steer.remote.go:39-64` calls `run.Steer(deliver, session, message)` =
  `(*run).Steer` (`run.go:366-384`): resolves the session's scope from
  `r.scopes` and calls that `*Session`'s own `Steer` in-process (`run.go:383`).
  Line 57 detaches delivery from the browser's request ctx
  (`context.WithoutCancel`, 30s timeout) since "the browser cannot cancel the
  delivery."
- `control.remote.go:31-42` (`stopTurn`) calls `run.CancelTurn` (`run.go:
  427-441`).
- `control.remote.go:44-55` (`cancelRun`) calls `run.CancelScope("", cause)`
  (`run.go:409-422`).
- `interview.remote.go:27-43` (`answerInterview`) calls
  `run.AnswerInterview` (`run.go:350-360`), sending on a buffered channel a
  waiter from `registerInterviewQuestion` (`run.go:311-327`) read by
  `waitInterviewAnswer` (`run.go:329-345`).

Every call is a direct, synchronous, in-process method on the `*run`/`*scope`/
`*Session` tree — no HTTP hop, no queue (`gimbal-internals-audit.md §7`, Fact
15: "submit" and "execute" are the same process). `web/control.go:47-124`
already answers Steer/SteerLoop over a Unix-domain socket for the CLI's
cross-process path — evidence Gimbal already has one HTTP hop in its control
path today (CLI to instance), just not instance-to-worker.

### What a supervisor reads

A `transcript` — a byte-bounded (256 KiB, `supervise.go:98`) in-memory ring
of `transcriptEntry` (`supervise.go:286-343,353-420`) — is populated by the
same `onEvent` callback the worker's turn emits through
(`supervise.go:167`, `supervise_jev.go:296-299`). `t.since` (`supervise.go:
440-478`) advances a supervisor's cursor and returns only new entries,
reporting an explicit `[gap]` marker plus the durable per-session JSONL path
(`filepath.Join(runDir(ctx), "sessions", s.id+".jsonl")`, `supervise.go:109`)
when retention or the per-look budget dropped anything (`supervise.go:
482-505`). **A supervisor's "since last look" view is in-process, in-memory,
fed by the same stream the worker emits, with the durable log as its only
fallback** — nothing reads from a remote store or a file the worker didn't
already write locally.

## Three options, evaluated

### Option A — whole call runs inside the activity; Steer/Kill/Interview travel instance -> worker directly

**Shape.** The activity runs Gimbal's existing in-process code unmodified —
`dispatchRecorded`, `supervise*`, the supervisor goroutines, `Session.Steer`
— exactly as today when everything runs on one machine. Only *how the
message reaches that process* changes: the worker registers its address (pod
IP:port, or a Session-scoped id) with the instance at turn start; the
instance's remote `live.Controller` does an HTTP/gRPC call there instead of
an in-process method call. `CancelTurn` becomes "cancel this turn's
`CancelCauseFunc` on the worker," same path. Temporal is not involved in
delivering Steer/Kill/Interview at all — it only decides which
activity/worker owns the turn (R1.1's problem).

**Network/RBAC on EKS.** Needs a `NetworkPolicy` ingress rule from the
instance's pod/Service to worker pods on the control port, per-project
namespace. Small and well-understood, but new: today nothing needs
instance-to-worker reachability, because there is no worker distinct from
the instance. Also needs a small registry (worker address learned at
turn-start, invalidated at turn-end or worker death) — the "turn handle" the
audit already names as the missing piece.

**Behind a Session.** Sessions (Finding 4) pin *subsequent activities* to the
accepting worker; they don't expose that worker's address outside SDK-internal
routing. So the address registry is orthogonal to Sessions: Sessions decides
which worker runs the next activity, this design separately needs which
address answers Steer for a still-running turn. Session death
(`ErrSessionFailed`) needs to invalidate the registry entry too — new code,
though it can share whatever health-check the instance needs for progress
display anyway.

**Latency.** One HTTP/gRPC hop inside the cluster, no Temporal round trip:
comparable to today's Unix-socket control latency, low single-digit
milliseconds for a healthy pod, plus the unchanged in-process
`Session.Steer` cost (the dominant cost either way). This does not inherit
Temporal's Signal-then-poll-drained-by-workflow-task latency, because
Temporal is bypassed for this path entirely.

A is the only option whose Steer/supervision/kill latency is "today's cost
plus one small local hop," not a redesign of any primitive.

### Option B — Steer through Temporal: Update -> second activity on the same Session host -> local delivery

**Shape.** The instance's Steer call becomes a `workflow.Update`/Signal
against the run's workflow. Its handler stashes the message and schedules a
*second* activity against the same session's `workflow.Context`, whose only
job is delivering the message to the worker process holding the live turn —
still needing A's exact worker-local side channel underneath, because
Temporal supplies "route to the right host," never "inject into the running
process" (temporal-go-sdk.md Finding 5, quoting the community forum directly:
"There is no functionality to send signal to an activity"; prior-art.md
Finding 7 corroborates independently).

**Can two activities run concurrently on one Session host?** Finding 4
documents Sessions routing every activity on a session-scoped context to one
worker process, and `MaxConcurrentSessionExecutionSize` capping concurrent
*sessions* per worker, not activities within one session. So a second,
short "deliver steer" activity alongside the long turn's activity is
plausible per the documented model, but **this exact configuration was not
verified against a primary source in round 1** — Sessions' single-host
routing and `ErrSessionFailed` are confirmed; concurrent-activity-count
behavior within one live session is not. Carried forward as an open unknown.

**Update round-trip latency.** Round 1's sources describe Updates as "a
trackable synchronous request" purpose-built for Interview, but neither note
quotes a concrete latency figure. What is documented: an Update only takes
effect the next time the workflow's coroutine runs a scheduling decision —
not instant the way an in-process channel send is — and the second activity
still needs its own scheduling round trip. Temporal Agent Harness's own
design (prior-art.md Finding 6) explicitly avoids sub-activity-boundary
delivery, appending to a queue "the model loop" drains on its *next*
iteration — the ecosystem's own admission this path isn't sub-second-reliable
enough for mid-token delivery. Estimate (not independently verified this
round): low seconds to tens of seconds for message-visible-to-workflow, plus
the second activity's dispatch and worker-local delivery on top — materially
slower than A's single local hop, with two more independently-failing moving
parts.

**Verdict on B vs A:** B reproduces Steer's *routing* half through Temporal
instead of a Gimbal-maintained registry, at the cost of Update/second-activity
latency and an unverified concurrency assumption, while still needing A's
entire worker-local delivery mechanism underneath. It buys "one control-plane
system, one audit trail," but is strictly slower and no more correct than A.

### Option C — queued, delivered only between turns / at the next model call, via a worker-local file/socket the activity polls

**Shape.** The activity polls a worker-local file/socket at model-call (or
tool-call) boundaries for queued steer/kill/objection messages and applies
them there — identical in shape to Temporal Agent Harness's own pattern
(prior-art.md Finding 6: "its *next* loop iteration picks up the new
instruction") and to Replit's production design (Finding 9: Update "can push
a message into the running workflow and pause before resuming" — steering as
pause-and-resume, not mid-flight injection, even in the one confirmed
production case).

**What Gimbal loses.** Steer's own doc comment already says delivery is "at
the worker's next model call" (`session.go:544-547`), so C's between-turn
framing is a smaller narrowing than it first looks for the exported contract
— today's adapter-level delivery (codex/opencode's live daemon connection)
can still land mid-turn before the next full turn, but C additionally forces
`WithSupervisor`'s mid-turn look-and-object loop (ticking every 3 minutes
*during* a turn, `supervise.go:79-81,138`) down to "reviewable only at turn
boundaries," which for hour-long turns (BRIEF.md) could mean a supervisor's
first chance to object arrives only at the very end of the turn it was meant
to watch — the entire value of "watches ... mid-turn" (`supervise.go:35-46`)
is what C gives up. Kill under C is bounded by the same poll cadence, worse
than A's synchronous cancellation and worse than B's Update path, adding a
worker-chosen poll interval on top of transport latency. Interview is
unaffected in kind (it already blocks indefinitely), only in the small added
poll latency on delivery.

C is a legitimate fallback if A's registry work is judged too costly, but it
redefines `WithSupervisor` from "mid-turn watcher" to "between-turn
reviewer" — a semantic change AGENTS.md's "build what was asked"/"no
wrappers" posture would need Tyler to approve explicitly, not something round
2 should assume.

## Comparison table

| | A: direct worker HTTP/gRPC | B: Temporal Update -> second activity | C: worker-local poll, between-turn delivery |
|---|---|---|---|
| Fidelity — Steer | Full: same in-process call, remote caller | Partial: Temporal routing + Update round trip on top of A's delivery | Reduced: deferred to next turn/tool boundary |
| Fidelity — WithSupervisor | Full: goroutines/transcript/dispatch unmodified | Same as A once delivered, inherits B's latency | Lost: only fires at turn boundaries |
| Fidelity — Interview | Full, if worker exposes the waiter equivalent | Full — Update is purpose-built for this (Finding 5) | Full — Interview already blocks |
| Fidelity — CancelTurn/Kill | Near-identical: one hop + in-process cancel | Heartbeat/Update-round-trip-gated + second-activity dispatch | Poll-interval-gated on top of transport delay |
| New code surface | Worker control listener; instance-side address registry; NetworkPolicy; worker/Session-death health check | Update/Signal handler; second-activity scheduling; A's listener underneath | Worker poll loop; instance-side queue writer; redefined supervisor cadence |
| Temporal dependence | None for steering/kill (only R1.1's scheduling) | Full: every Steer/Kill/Interview is a Temporal Update/Signal | None for delivery mechanics |
| Failure modes | Unreachable worker; stale registry entry after restart; Session death leaves a stale entry | Update rejected/queued; unresponsive workflow; unverified concurrent-activity assumption; still inherits A's failure modes | Poll loop crash silently drops a steer; objections arrive too late to matter |
| Cost to build | Moderate: one HTTP surface, one registry, one NetworkPolicy rule | Moderate-high: A's surface plus workflow-side plumbing plus an unresolved concurrency question | Low-moderate: no cross-pod network needed if using a shared volume, but the semantic narrowing is the real cost |

## Interview under each option

`Interview` (call sites read in `run.go:307-345` and the answering remote;
its own body not read this round) blocks on `waitInterviewAnswer`, selecting
on the buffered `waiter.answer` channel or `ctx.Done()` (`run.go:329-345`);
the person answers via `answerInterview` -> `run.AnswerInterview`
(`run.go:350-360`) — an in-process channel send today, because run and
waiter share a process.

- **Under A:** blocking happens on the worker, since that's where the
  workflow-body code calling `Interview` runs (extending Finalist 1's
  "the whole turn/supervisor tree executes on the worker" to other blocking
  calls in the same activity). The worker needs its own waiter equivalent,
  and `answerInterview` reaches it via the same address-registry Steer uses —
  symmetric with Steer/Kill, no new mechanism.
- **Under B:** Interview is the one primitive Temporal's own primitives fit
  best without a worker-local side channel — `workflow.Update` (Finding 5) is
  the documented shape for exactly this, and the blocking can happen in the
  *workflow* (via `workflow.Await` on the update having landed) instead of
  requiring an activity-level listener, **if** `Interview` is a workflow-level
  construct rather than activity-adjacent code. Replit's consent-pause and
  `durable-hitl-agents`'s Signal+`wait_condition` pattern (prior-art.md
  Findings 8-9, 12) both confirm this shape works well in production. **Load-
  bearing correction:** whether Interview blocks on the worker (A's framing)
  or on the orchestrator depends entirely on whether the *workflow body*
  itself runs on Temporal's workflow side (Finalist 2, R2.1-R2.4's subject)
  or stays ordinary Go on an orchestrator process (Finalist 1's actual
  shape). Under Finalist 1, Interview is workflow-body code running on the
  orchestrator, which already holds `run.turns`/`r.interviews` today, unchanged
  by any of A/B/C — Interview is not a `Generate` call and is not inside the
  activity boundary R1.3 is about. This note does not resolve which shape
  applies; that is R1.1's and R2.1-R2.3's territory.
- **Under C:** the answer arrives via the same poll mechanism; since
  Interview already tolerates blocking indefinitely, the only cost is the
  poll interval's added latency, small relative to human response time.

## Cancellation propagation (Group first-error cancels siblings' turns)

`Group.Go` computes each child's scope synchronously before spawning its
goroutine (audit §1.2, `group.go:63-85`); on the first non-`Killed` child
error the shared ctx is cancelled with `cancel(nil)` (audit §1.4's `Group`
row), which every sibling's scope-derived context observes through ordinary
`context.Context` propagation — free and instantaneous today because it is
one process's context tree.

- **Under Option A's registry, reused for Group:** if the orchestrator
  (wherever `Group`'s cancellation logic runs) reaches the worker directly to
  cancel a specific sibling's `turnCtx` — the same registry/HTTP path A
  builds for Steer/Kill — cancellation is as fast as that one hop:
  Group-triggered cancellation and operator-triggered `CancelTurn` become the
  same worker-side primitive with two callers. This is the cleanest outcome
  of choosing A.
- **Under a literal Temporal-workflow `Group` (Finalist 2's translation, per
  temporal-go-sdk.md Finding 2's mapping table row for `Group`):**
  cancelling one sibling's activity-backed turn is "cooperative and
  heartbeat-gated: cancelling the activity's Future ... does not reach into
  and kill the activity's process; the activity only learns about it the
  next time it calls `RecordHeartbeat`" (Finding 3, quoted directly) — so
  Group-triggered cancellation is bounded below by the harness adapter's
  heartbeat cadence, a real latency floor Gimbal does not have today
  (today: instantaneous `CancelCauseFunc`). A genuine narrowing of `Killed`
  semantics if Finalist 2's `Group` translation is adopted, independent of
  which of A/B/C handles Steer.
- **Under Finalist 1's actual shape (workflow body stays ordinary Go on the
  orchestrator; only `Generate`/`RunCommand`/`Check`/`Service` are remote
  activities):** `Group`'s cancellation logic is unchanged Go running on the
  orchestrator (`group.go:63-85`); only the *turn* needs cross-process reach,
  which is A's registry/HTTP path. Cancellation stays free at the `Group`
  level and only pays A's one-hop cost at the turn level, never Temporal's
  heartbeat-gated cost — one more reason SYNTHESIS.md's Finalist-1-first
  recommendation is the right basis for R1.3.

## Remaining unknowns

1. Whether two Activity executions can run concurrently against one
   Session-scoped `workflow.Context` (needed for Option B) — not verified
   against SDK source or docs in round 1 or here; only single-host routing
   and `ErrSessionFailed` are confirmed. A hands-on test (one session, two
   overlapping activities against its context) would settle this.
2. No concrete Update round-trip latency figure was found in round 1;
   only the qualitative shape is confirmed. A load test against a real
   Temporal deployment would give the number this note only estimates.
3. Whether `Interview`'s own implementation (not read in full this round)
   ties its blocking location to the same scope/session machinery as
   `Generate` — this determines whether the A-vs-B choice for Interview is
   live at all, or already settled by Interview being workflow-body code
   rather than `Generate`-adjacent code.
4. The exact protocol for "worker registers its address with the instance"
   (push at turn-start, pull via a Temporal Query, or an external registry)
   is undecided; each has different latency/failure tradeoffs not explored
   here — implementation detail for whoever builds Option A, not a fork this
   note found materially different in fidelity/latency/failure-mode terms.
5. Whether `WithSupervisor`'s Jev screening path (`supervise_jev.go`,
   requiring `TYPESAFE_API_KEY` and network access to the Jev service) is
   expected to run on the worker (assumed here, co-located with the turn) or
   could run on the orchestrator reading a remote transcript stream — not
   asked by the brief; the current code's tight coupling between the
   transcript ring and the same-process `onEvent` callback (`supervise.go:
   167`, `supervise_jev.go:296-299`) suggests co-location is the only design
   the current code supports without new plumbing.

## Sources

Gimbal source, read directly this session, 2026-09-27, at the checkout's
current HEAD (`/home/user/gimbal`):

- `session.go` (whole file) — `Generate`, `dispatchRecorded`, `generate`,
  `(*Session).turn`, `Steer`, `Fork`.
- `supervise.go` (whole file) — `WithSupervisor`, `supervise`,
  `superviseTimed`, `transcript`/`transcriptEntry`, `look`/`since`.
- `supervise_jev.go` (whole file) — `jevSupervision`, `superviseWithJev`,
  `superviseWithJevClient`.
- `run.go` (whole file) — `run` struct, `addTurn`/`removeTurn`, `Steer`,
  `SteerLoop`, `CancelScope`, `CancelTurn`, `registerInterviewQuestion`,
  `waitInterviewAnswer`, `AnswerInterview`.
- `internal/live/live.go` (whole file) — `Controller` interface, `Hook`,
  `Runs` table.
- `internal/observation/http.go` (whole file) — SSE read path, confirming
  it is unrelated to the four Controller write methods this note covers.
- `web/src/routes/steer.remote.go`, `web/src/routes/control.remote.go`,
  `web/src/routes/interview.remote.go` (whole files) — the SKGO remotes the
  page's forms/commands call.
- `web/control.go` (whole file) — the Unix-socket control HTTP surface,
  showing Gimbal already has one HTTP hop in its control path today.
- `events.go:237-239` — `Killed{Target, By, Reason}`, grepped to confirm the
  shape `killedEvent` (`run.go:446-451`) constructs.

Round-1 notes, read in full for this note, treated as already-verified prior
work per the brief's instruction to read them first rather than re-fetch
primary Temporal sources:

- `ephemeral/research/temporal-projection/round1/SYNTHESIS.md`
- `ephemeral/research/temporal-projection/round1/gimbal-internals-audit.md`
  §2 (Generate internals), §6 (observation), plus §1.2/§1.4/§7 for scope
  identity, per-primitive footprint, and the hosted-path submission boundary.
- `ephemeral/research/temporal-projection/round1/temporal-go-sdk.md`
  Findings 1-5 — source for every Temporal-side claim in Options B/C and the
  cancellation-latency analysis.
- `ephemeral/research/temporal-projection/round1/agents-on-temporal-prior-art.md`
  Findings 6-9 — source for every industry-precedent claim in Option C and
  the Interview section.
- `ephemeral/research/temporal-projection/BRIEF.md` — the research brief and
  its sourcing rules.
