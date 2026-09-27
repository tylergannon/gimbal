# Round 2, R1.1: activities with host pinning, without a bespoke Temporal workflow

Research date: 2026-09-27. All primary sources fetched live today (raw GitHub
source, raw Markdown from docs.temporal.io via its documented `.md` suffix,
Temporal server source on GitHub, a blog post's own JSON-LD metadata) — not
summarized by an intermediate model, directly answering round 1's flagged
method risk. Claims resting on a summarized fetch or search synthesis are
marked inferred, with the reason.

## Verdict

**Risk R1.1 is eliminated, and Finalist 1's shape gets simpler than round 1
assumed.** Standalone Activities (GA in `go.temporal.io/sdk` v1.49.0,
2026-09-14; needs Temporal Server v1.32.0+; GA on Temporal Cloud in all
regions today) let a plain Go client call `client.ExecuteActivity` on a
caller-chosen task queue and block on `ActivityHandle.Get`, with full
heartbeat/cancel/retry support — so a Gimbal session's whole sequence of
turns, commands, and services can be pinned to one worker pod by giving them
all the same session-scoped task-queue name, using **no Temporal workflow at
all**. A generic Update-driven executor workflow and Sessions both work too
and are documented for exactly this "same host" purpose, but both carry a
workflow's history/limits bookkeeping (10 in-flight / 2,000 total Updates per
execution, "don't continue-as-new while an Update is open") that pure
Standalone Activities avoid outright, since a Standalone Activity Execution
has **no Workflow Event History and no deterministic replay** at all.

## Finding 1: Standalone Activities

**Verified directly against sdk-go v1.49.0 source
(`internal/internal_activity_client.go`, `client/client.go`) and against
`docs.temporal.io/standalone-activity` and the Go quickstart, both fetched as
raw Markdown today via the `.md` suffix these docs pages advertise — not as
page summaries.**

A Standalone Activity is "a top-level Activity Execution started directly by
a Client, without using a Workflow"; the Activity Definition and Worker
registration are identical to a workflow-driven Activity, only the start
path differs (`client.ExecuteActivity` vs `workflow.ExecuteActivity`), and the
same function runs either way unchanged. `client.ExecuteActivity(ctx,
options, activity, args...)` returns an `ActivityHandle`.
`StartActivityOptions` (source) has `ID`, `TaskQueue string`,
`ScheduleToCloseTimeout`, `StartToCloseTimeout`, `HeartbeatTimeout`,
`RetryPolicy`, `ActivityIDConflictPolicy`; `ID` and `TaskQueue` are required,
plus at least one timeout.

**Await the result**: `ActivityHandle.Get(ctx, &result)` (source) is a
repeated long-poll against `PollActivityResult` until an outcome exists —
"analogous to calling `Get()` on a `WorkflowRun`." Answers the brief
directly: a client can start one and block for the result in ordinary Go, no
workflow needed.

**Heartbeats, cancellation, retries**: `HeartbeatTimeout` plus
`activity.RecordHeartbeat` in the body — the same `activity` package API a
workflow Activity uses (same Activity Definition). `ActivityHandle.Cancel`
requests cooperative cancellation ("Temporal can accept a Cancel request
without the Worker honoring it," gated on the next heartbeat, exactly as for
a workflow Activity); `.Terminate` closes unconditionally. `RetryPolicy`
governs retries; documented default is "at-least-once: retry with
exponential backoff until it succeeds or Schedule-To-Close elapses," same
shape as a workflow Activity's default. `MaximumAttempts: 1` gives
at-most-once. A retry restarts the function from the top — unchanged from
round 1's Finding 3, now confirmed to apply identically here.

**Host-specific task-queue routing**: `TaskQueue` is a plain caller-chosen
string on every call. This is the load-bearing fact for R1.1: a Gimbal
session's every turn/command/service can be issued as its own
`client.ExecuteActivity` call sharing one `TaskQueue` (e.g. derived from the
Gimbal session id); as long as one worker polls that name, every one of that
session's activities lands on that pod. This is the samples-go
worker-specific-task-queue pattern (Finding 3), needing no workflow to
orchestrate it and no Sessions API (Sessions is workflow-scoped —
`workflow.CreateSession` takes a `workflow.Context`, reconfirmed today by
reading `internal/session.go`, Finding 4).

**Server version, Temporal Cloud**: "Standalone Activities require Temporal
CLI v1.9.1 or higher and Temporal Server v1.32.0 or higher"
(docs.temporal.io/standalone-activity, verbatim, today). "Standalone
Activities are Generally Available in Temporal Cloud, in all regions...
match [Workflows'] SLO/SLA" (same page). Cloud always runs a current server,
so the version floor only matters self-hosted.

**Release-stage precision** (corrects round 1's date ambiguity): Standalone
Activities were announced **Public Preview** at "Replay 2026," a blog post
whose own JSON-LD gives `datePublished: 2026-05-06` (read from the raw HTML).
CHANGELOG.md then records under `[1.49.0] - 2026-09-14`: "Standalone
Activities are now generally available (GA)" (operator commands and Nexus
use remain experimental). Today's docs page agrees: GA for start/get/
heartbeat/cancel/terminate/retry; only Pause/Unpause/Reset/UpdateOptions and
batch List-Filter ops remain Public Preview — none of which Gimbal needs.

**History cost**: "there's no Workflow Event History and no deterministic
replay" for a Standalone Activity Execution (docs, "Standalone Activity
versus Workflow Activity") — the entire history-size/continue-as-new
question that dominates a workflow-shaped design does not exist here.

**Restart behavior**: not documented as a Standalone-Activity-specific
concept, but mechanically identical to Finding 3's task-queue routing: if the
one worker on a session's dedicated queue dies mid-Activity, the retry is
scheduled onto the **same** queue and sits there until a poller reattaches or
`MaximumAttempts`/`ScheduleToCloseTimeout` exhausts, surfacing as an error
from `ActivityHandle.Get` the orchestrator can react to.

**Deduplication as free idempotency**: Standalone Activity IDs live in their
own ID space, with an **ID Conflict Policy** (`Fail`, the default, or
`UseExisting` — "returns a handle to the running one") and a separate **ID
Reuse Policy** for closed executions. Gimbal's node ids are already
deterministic and stable across two executions (round 1 synthesis fact 2,
`lap.3/check.2`); using one as the Activity ID with `UseExisting` gives
orchestrator-restart idempotency for the execution layer for free — a
synergy with R1.4 round 1 did not connect.

**Concurrency headroom**: a worker's `MaxConcurrentActivityExecutionSize`
defaults to 1,000 (source, `internal/worker.go`), with no Sessions-style
admission cap in play here at all — two session-scoped activities (a running
turn plus a "steer" activity) on the same queue can run concurrently, bounded
only by this generous default.

## Finding 2: A generic executor workflow driven by Updates

**Verified directly from `docs.temporal.io/handling-messages` (raw Markdown,
today) and from `temporalio/temporal`'s own
`common/dynamicconfig/constants.go` (raw source, today) — an upgrade over
round 1's summarized docs with no hard numbers for Updates.**

**Can an Update handler await an hour-long Activity?** Yes: "Sometimes, you
need your message handler to wait for long-running operations such as
executing an Activity... the handler will yield control back to the loop."
`SetUpdateHandler`'s own doc comment (source) agrees: "update code is free to
invoke and wait on the results of activities. Update handler code is free to
mutate workflow state," and the handler "can take a `workflow.Context` as its
first parameter" — full workflow API access, including `CreateSession` and
`Await`.

**The catch**: "You should generally finish running all handlers before the
Workflow run completes or continues as new... If you don't need to ensure
your handlers complete, [set] Handler Unfinished Policy as Abandon to turn
off the warnings. However, ... clients waiting for Updates will get Not
Found errors if they're waiting for Updates that never complete before the
Workflow run completes." For a Gimbal-session executor, continue-as-new can
never fire while a turn's Update is in flight without either waiting first
(defeating prompt history rollover) or accepting the orchestrator's
`UpdateWorkflow` call will need its own resubmit-on-Not-Found logic.

**Concurrency and lifetime limits**, from Temporal server source
(`common/dynamicconfig/constants.go`, read today):

- `WorkflowExecutionMaxInFlightUpdates` (`history.maxInFlightUpdates`),
  **default 10** — admitted-but-not-completed Updates per workflow
  execution at once.
- `WorkflowExecutionMaxTotalUpdates` (`history.maxTotalUpdates`), **default
  2,000** cumulative Updates per execution, with
  `...SuggestContinueAsNewThreshold` **default 0.9** (≈1,800) hinting
  continue-as-new.
- `WorkflowExecutionMaxInFlightUpdatePayloads`, **default 20 MiB** total
  in-flight Update payload size.
- `NumPendingActivitiesLimit`, **default 2,000** incomplete Activities per
  execution (docs.temporal.io/workflow-execution/limits, re-confirmed
  today).

These are namespace dynamic-config, so raisable self-hosted (Cloud's own
ceiling unverified — see Unknowns). At face value 10 in-flight is
comfortably above one Gimbal session's real concurrency need, and 2,000 (hint
at 1,800) is a concrete, previously-missing continue-as-new trigger for this
design — a real answer to round 1's "needs new static analysis" risk, at
least on the Update-count axis.

**Concurrent activities on one Session, under this design**: yes — see
Finding 4.

**History growth per Activity, and heartbeats**: Temporal's canonical event
reference lists only `ActivityTaskScheduled/Started/Completed/Failed/
TimedOut/CancelRequested/Canceled` — no `ActivityTaskHeartbeated` type — and
notes `ActivityTaskStarted` "is not written to History until the terminal
Event... occurs," so one successful attempt costs roughly two history
entries, not a count that grows with the turn's duration. **Heartbeats do
not create history events; they only update the Activity's pending mutable
state.** This refines round 1's looser claim that "every heartbeat is itself
one or more history events" (temporal-go-sdk.md Finding 3): the schedule/
start/complete triad costs history, heartbeats cost Cloud Actions (round 1
Finding 9) but not history events; only an added retry attempt adds another
Started+terminal pair.

**Prior art**: no packaged "generic executor"/"activity runner" workflow
type was found published by Temporal; `samples-go/fileprocessing` hard-codes
its specific activities rather than genericizing "run activity X with args."
Nexus (round 1 Finding 7) solves cross-namespace calling, not this. The
nearest genuinely new "generic progress channel out of a running Workflow"
primitive is Workflow Streams (Finding 5).

## Finding 3: Worker-specific task queues

**Verified directly from `samples-go/worker-specific-task-queues/README.md`
and `samples-go/fileprocessing/README.md` (raw GitHub content, today).**

The pattern, in the sample's own words: "Use a unique Task Queue for each
Worker... Each Worker process creates two `worker` instances: one listens on
the `shared-task-queue`... another on a uniquely generated Task Queue." The
sample's own README states the tradeoff explicitly: "*Sessions are an
alternative to Worker-specific Task Queues.*" Cost: none beyond a second
`worker.Worker` poller per process — ordinary SDK usage, not a licensed
feature.

**Restart behavior**, verbatim from the sample: "You can try to
intentionally crash Workers... to see what happens when work gets 'stuck' in
a unique queue: currently the Workflow will `scheduleToCloseTimeout` without
a Worker, and retry when a Worker comes back online. After the 5th attempt,
it logs `Workflow failed after multiple retries.`" Identical mechanism, and
identical failure shape, to Standalone Activities routed the same way
(Finding 1) — expected, since it is the same task-queue machinery underneath
either.

**Design point** (inference, not documented by Temporal): because the queue
name is entirely caller-chosen, Gimbal can key it on the **Gimbal session
id** rather than the worker's own hostname (which is what Sessions does,
Finding 4). That decouples "the pinned worker" from "one physical process":
a rescheduled Kubernetes pod that mounts the same durable workdir and
re-registers on the same session-derived queue name resumes service
transparently — without Sessions' harder failure mode (permanent
`ErrSessionFailed` on host death, rather than just retrying against the same
name until a new poller appears).

## Finding 4: Sessions API — round-1's unknowns, settled from source

**Verified directly from `internal/session.go` in sdk-go master (raw
source, today) — replaces round 1's "contested between two summarized
sources" status.**

- **`MaxConcurrentSessionExecutionSize` default**: the source's own comment:
  "By default, **1000** is used." Round 1's contested default is settled.
- **Session `HeartbeatTimeout` default**: `defaultSessionHeartbeatTimeout =
  20s` (constant) — "if heartbeat is not received... within the timeout, the
  session will be declared as failed."
- **Actual heartbeat interval**: `min(activityEnv.heartbeatTimeout/3,
  maxSessionHeartbeatInterval)` where `maxSessionHeartbeatInterval = 10s`
  (source) — with the 20s default that's `min(6.67s, 10s) ≈ 6.67s`.
- **Worker process restart**: a session is backed by one long-running
  background Activity (`internalSessionCreationActivity`) that loops,
  heartbeating, until `ExecutionTimeout` or `CompleteSession`. A watcher
  coroutine marks `SessionStateFailed` and cancels the session's context the
  moment that Activity resolves with a non-canceled error (i.e. the worker
  died and its own Heartbeat/StartToClose timeout elapsed); every
  subsequent Activity against that session "will return `ErrSessionFailed`
  immediately without scheduling" (source doc comment). Confirms round 1's
  Finding 4 from source rather than a docs summary: **no automatic failover,
  the session deterministically fails.**
- **Two activities concurrently on one Session?** Yes — nothing in
  `session.go`'s admission machinery (`sessionTokenBucket`) gates anything
  but *session creation itself*; once a session context exists, concurrent
  `ExecuteActivity(sessionCtx, ...)` calls from sibling `workflow.Go`
  coroutines are ordinary workflow concurrency, bounded only by the target
  worker's `MaxConcurrentActivityExecutionSize` (default 1,000). Directly
  answers "steer needs a concurrent second activity on the same host":
  supported, no Sessions-specific serialization.
- **Can an Update handler create a Session?** `CreateSession` needs only a
  `workflow.Context` without an already-open session, and registers the
  session at the workflow-environment level, not the calling coroutine's.
  Combined with `SetUpdateHandler`'s documented context/await-activities
  freedom, nothing in either API forbids it — **inferred** from reading both
  together, not stated explicitly; worth a five-minute hands-on check before
  being load-bearing.
- **`RecreateSession`**: explicitly for continuing a long session across a
  workflow's own Continue-As-New (its token carries only the taskqueue
  name, per source), not for reconnecting after a host actually died —
  confirms round 1's Finding 4 from the token's own contents.
- **Task-queue naming, from source**: `getResourceSpecificTaskqueue(id) =
  id + "@" + hostname` — Sessions is, under the hood, the same
  worker-specific-task-queue trick (Finding 3), auto-generated and tied to
  the worker's own hostname, which is structurally why it cannot survive a
  pod being rescheduled under a new identity the way a Gimbal-chosen queue
  name can.

## Finding 5: Streaming progress — heartbeats, Workflow Streams, or HTTP

**Verified directly from `docs.temporal.io/encyclopedia/detecting-activity-failures`
and `docs.temporal.io/workflow-streams` (raw Markdown, today).**

**Heartbeat throttling, the exact rule**: "The throttle interval is the
smaller of the following: if `heartbeatTimeout` is provided,
`heartbeatTimeout * 0.8`; otherwise, `defaultHeartbeatThrottleInterval`[30s
default] — `maxHeartbeatThrottleInterval`[60s default]... After sending a
Heartbeat, the Worker sets a timer for the throttle interval [and] stops
sending Heartbeats, but continues receiving [them] from the Activity and
remembers the most recent one," sending it once the timer fires. With a 20s
`HeartbeatTimeout` that is `min(16s, 60s) = 16s` between deliveries at best;
unset, it is 30–60s. **Heartbeats are categorically too coarse for per-event
live-page progress**, and lowering the interval to compensate directly costs
Cloud Actions (round 1 Finding 9) and cancellation latency (round 1 Finding
3/9) at once.

**Workflow Streams — new, purpose-built, Public Preview**: announced in the
same 2026-05-06 post as Standalone Activities, shipped as an independently
versioned Go module `go.temporal.io/sdk/contrib/workflowstreams` (not in the
main SDK changelog, which excludes contrib modules by policy). It is "a
durable event channel hosted **inside a Workflow**" — publishers (the
Workflow, its Activities, or external clients) append batched events via a
Signal (default 2s batch, or immediate via `force-flush`); subscribers
long-poll via an Update, exactly-once and ordered, resuming by offset across
reconnects and Continue-As-New. Its own lead example is "updating a UI as an
AI agent works." Two things weigh against it now: it requires hosting a
Workflow (its constructor registers Signal/Update/Query handlers), which
reintroduces the very workflow Finalist 1 avoids for pinning alone; and it is
Public Preview, with "each batched publish [costing] one Signal and each
subscriber poll one Update... against the Workflow's history" — its own
tuning guidance suggests a 200ms batch for an LLM token stream, "roughly 150
publish Signals" per 30 seconds of one turn's output, against the same
Update ceilings as Finding 2.

**Recommendation**: keep Gimbal's existing out-of-band HTTP push (per-event
records streamed from worker to the shared instance, as today) over both
heartbeats (too coarse) and Workflow Streams (Public Preview, needs a hosted
workflow, adds Signal/Update cost a pure-Standalone-Activities design has no
other reason to take on). Revisit Workflow Streams once GA and only if
Gimbal separately wants a hosted workflow for other reasons (Finding 2).

## Comparison table

| Mechanism | Host pinning | Hour-long activity | Concurrent activities/host | Restart behavior | Cloud availability | History cost |
|---|---|---|---|---|---|---|
| Standalone Activities + session-keyed queue | Yes, caller-chosen `TaskQueue` | Yes | Yes, worker's `MaxConcurrentActivityExecutionSize` (1,000 default), no session cap | Retries queue on same name until a poller returns or attempts/timeout exhaust; name can be keyed on session id, survives pod reschedule | GA, all regions, SLO/SLA = Workflows | None — no Event History exists |
| Generic executor workflow + Updates | Yes, via Session or manual queue from inside the workflow | Yes, documented pattern | Yes (see Sessions row) | Worker death fails the open Session; workflow's own history persists on the server | GA (Workflows/Updates/Sessions all long-GA) | ~2 events/attempt, none for heartbeats; capped at 10 in-flight / 2,000 total Updates (1,800 hint), 2,000 pending Activities, 51,200-event/50MB ceiling |
| Worker-specific task queues, manual | Yes, same mechanism | Yes | Yes, ordinary worker concurrency | Same "stuck until poller returns or retries exhaust," verbatim from the sample | GA (plain Activities/queues) | Workflow-hosted: same as row 2; client-direct: none |
| Sessions API | Yes, tied to creating worker's **hostname** | Yes, via activities under it | Yes, no inter-activity serialization; cap is 1,000 *sessions*/worker, not activities/session | Host death → permanent `ErrSessionFailed`, no reroute; `RecreateSession` only across own Continue-As-New | GA | Same as row 2 |
| Heartbeat details as progress channel | N/A | N/A | N/A | N/A | GA | Cheap, but throttled to `min(0.8×HeartbeatTimeout, 60s)` — too coarse for live per-event UI |
| Workflow Streams | N/A | Its own lead use case | N/A | Subscribers auto-follow Continue-As-New via stable Workflow Id | Public Preview (Go contrib module) | 1 Signal/publish batch + 1 Update/subscriber poll, same ceilings as row 2 |

## Recommended mechanism

**Standalone Activities, one task-queue name per Gimbal session, no Temporal
workflow.** This satisfies the brief's literal ask more directly than either
alternative, is GA today including on Temporal Cloud, sidesteps every
workflow-shaped constraint this cycle surfaced (Update ceilings, the
don't-continue-as-new-while-open hazard, Event History size), and composes
with Gimbal's already-deterministic node ids for restart idempotency via
`ActivityIDConflictPolicy: UseExisting`. Steer (round 1 Finding 5 — Temporal
has no signal-to-activity primitive) and live per-event streaming (Finding 5
above) both stay exactly what round 1 already concluded: Gimbal-built,
worker-local mechanisms — a second same-queue Standalone Activity for steer,
the existing HTTP push for the page. This pass only replaces "no
Temporal-native answer, build your own, on top of a workflow" with "…, and
you no longer need a workflow underneath it either."

Keep the Update-driven executor workflow as a documented fallback: real,
recommended-by-Temporal for exactly this shape, and its numeric ceilings (10
in-flight / 2,000 total Updates, 2,000 pending Activities) are now known and
comfortable for one Gimbal session — but it buys workflow bookkeeping for no
capability Standalone Activities lack for this job. It earns reconsideration
only if Gimbal wants a durable, queryable, crash-consistent *plan* for a
whole run (all of a `PromiseLoop`'s adaptive decisions in one replayable
place), not just durable host-pinned execution of each step.

## Remaining unknowns

- Whether an Update handler can actually call `CreateSession` in practice —
  inferred from two doc/source passages read together, not stated
  explicitly or run live; settle with a short test workflow before it is
  load-bearing for the fallback design.
- Whether Temporal Cloud lets a namespace raise
  `WorkflowExecutionMaxInFlightUpdates`/`MaxTotalUpdates`/
  `NumPendingActivitiesLimit` above self-hosted defaults, or fixes them —
  the numbers above come from the self-hosted server's dynamic-config
  source; Cloud's effective values were not checked this round.
- Whether `ActivityIDConflictPolicy: UseExisting` plus Gimbal's deterministic
  node ids behaves as hoped across a real orchestrator-restart-and-resubmit
  — inferred from policy docs, not exercised live (R1.4's job; this round
  found the mechanism it should test).
- Behavior when a new worker starts late on a dead session/queue's stale
  name (after `ErrSessionFailed` or after retries already exhausted) — not
  documented, not tested live.
- Whether Standalone Activities' Public-Preview-only batch ops (batch
  Cancel/Terminate/Delete) matter operationally — a minor gap, not
  re-verified live.
- Every claim here should still be load-tested (many concurrent Standalone
  Activities on one queue, a killed pod, a resubmitted Activity ID) before
  being final for a build decision — the same caveat round 1 raised for
  Sessions applies unchanged.

## Sources

Verified directly today (2026-09-27), raw source/content, not summarized:

- `raw.githubusercontent.com/temporalio/sdk-go/master/CHANGELOG.md` — v1.49.0
  GA line (2026-09-14); v1.46.0–1.48.0 standalone-activity history.
- `.../sdk-go/master/client/client.go` — `Client.ExecuteActivity`,
  `StartActivityOptions`, `ActivityHandle` aliases and doc comments.
- `.../sdk-go/master/internal/internal_activity_client.go` — option fields,
  `Get/Describe/Cancel/Terminate/Pause/Unpause` bodies.
- `.../sdk-go/master/internal/session.go` — `MaxConcurrentSessionExecutionSize`
  (1000), `defaultSessionHeartbeatTimeout` (20s), `maxSessionHeartbeatInterval`
  (10s), `CreateSession`/`RecreateSession`/`CompleteSession`,
  `getResourceSpecificTaskqueue`, `ErrSessionFailed` propagation.
- `.../sdk-go/master/internal/workflow.go` — `SetUpdateHandler` doc comment.
- `.../sdk-go/master/internal/worker.go` — `MaxConcurrentActivityExecutionSize`
  default (1k).
- `raw.githubusercontent.com/temporalio/temporal/main/common/dynamicconfig/constants.go` —
  `WorkflowExecutionMaxInFlightUpdates` (10), `MaxTotalUpdates` (2000),
  `...SuggestContinueAsNewThreshold` (0.9), `MaxInFlightUpdatePayloads` (20MiB).
- `.../samples-go/main/worker-specific-task-queues/README.md` and
  `.../fileprocessing/README.md` — pattern, "Sessions are an alternative,"
  crash/retry behavior.
- `docs.temporal.io/standalone-activity.md` — GA/Public-Preview split, CLI
  v1.9.1+/Server v1.32.0+, Cloud GA/SLA, ID conflict/reuse policies, "no
  Workflow Event History."
- `docs.temporal.io/develop/go/activities/standalone-activities-quickstart.md`
  and `.../standalone-activities.md` — worked client/worker code, SDK
  v1.49.0+ prerequisite, Cloud connection instructions.
- `docs.temporal.io/handling-messages.md` — Update handler concurrency,
  long-running-Activity pattern, unfinished-handler/Not-Found hazard.
- `docs.temporal.io/workflow-execution/limits.md` — history ceiling,
  `NumPendingActivitiesLimit` (2,000).
- `docs.temporal.io/references/events.md` — Activity event type list (no
  heartbeat event type); `ActivityTaskStarted` written late.
- `docs.temporal.io/encyclopedia/detecting-activity-failures.md` — heartbeat
  throttling rule verbatim.
- `docs.temporal.io/workflow-streams.md` and
  `docs.temporal.io/develop/go/workflows/workflow-streams.md` — full design,
  contrib-module packaging, Public Preview status, cost per publish/poll.
- `docs.temporal.io/llms.txt` — doc-site index, used to find current URLs
  after several round-1 paths had moved.
- `temporal.io/blog/replay-2026-product-announcements` (raw HTML, its
  embedded JSON-LD `datePublished`) — 2026-05-06, Public Preview
  announcement of Standalone Activities and Workflow Streams.

Read via WebSearch (only to locate current doc URLs and a documentation-repo
PR confirming the GA docs update; not relied on alone for any load-bearing
claim, cross-checked against primary sources above):

- "Temporal 'Standalone Activities' minimum server version release notes
  2026" — searched today.

Local:

- `/home/user/gimbal/ephemeral/research/temporal-projection/BRIEF.md`,
  `round1/SYNTHESIS.md`, `round1/temporal-go-sdk.md` — read in full before
  this pass, per the assignment.
