# R1.5 / R2.3: the numbers, verified against primary sources today

Round 2, 2026-09-27. Every dollar figure, limit, and GA date below was re-fetched directly from `docs.temporal.io` and `temporal.io` this session (raw HTML pulled with `curl`, stripped to text locally — not a WebFetch AI summary, since round 1 flagged that path as unreliable). URLs and read times are in Sources.

## Verdict

**Neither Action cost nor Event History size is a real risk at Gimbal's stated volumes, under either finalist.** A run with 20 hour-scale turns (30 min average, 30s heartbeat cadence), 40 commands, 5 steers, and 1 interview costs about **$0.06–0.07** in Temporal Cloud Actions and uses **400–650 history events** — 0.8–1.3% of the 51,200-event ceiling. At 300 runs/month the account's Action bill is **$19–22/month** before any flat plan charge, smaller than the cheapest paid support plan's own $500/month minimum. Continue-as-new for a pure `PromiseLoop` of turns at this cadence would not be forced until somewhere **between ~1,000 turns (soft warning) and 5,000–8,500 turns (hard limit)** per run — one to two orders of magnitude past any run Gimbal is likely to author. The one number genuinely unsized is **Active Storage**, which depends on payload bytes per run, not event count (Section 3).

Round 1's two numeric disagreements are both resolved with citations below: Action pricing is **$50/million today** (the $25 figure was a stale secondary source); and the "Build-ID versioning removed March 2026" and "Worker Deployment Versioning GA'd March 2026" claims are **both true, about two different features**, which happened to land in the same month (Section 5).

Net effect on the SYNTHESIS.md's provisional recommendation (Finalist 1, runtime projection): this round doesn't change that call. Cost and history size were never a reason to prefer Finalist 2's bespoke-per-workflow generation, and they still aren't — the ~5% Action/history overhead from Finalist 1's generic executor dispatching every turn through an Update (Sections 3–4) is real but small, and is more than offset by Finalist 1's simplicity. The one genuine tradeoff this round surfaces is versioning blast radius (Section 5): Finalist 1's single generic executor couples a deploy's drain-and-retire cost to the busiest hour across every Gimbal workflow type, where Finalist 2's bespoke-per-workflow generation would isolate it to the one workflow being changed. That's a real point in Finalist 2's favor, but on its own it's not enough to overturn round 1's recommendation.

## 1. Temporal Cloud pricing today

Fetched from `docs.temporal.io/cloud/pricing` and `docs.temporal.io/cloud/actions`, 2026-09-27.

**Actions.** $0.00005/Action ($50/million) at every volume on the Developer plan; Business/Enterprise/Mission Critical get volume discounts past the plan's included allocation:

| Actions in the month | Price per million |
|---|---|
| First 5M | $50 |
| Next 5M (to 10M) | $45 |
| Next 10M (to 20M) | $40 |
| Next 30M (to 50M) | $35 |
| Next 50M (to 100M) | $30 |
| Next 100M (to 200M) | $25 |
| Over 200M | negotiated |

**What is an Action** (from the Actions page's own category list, which explicitly separates Action Types from raw history events, warning the two "do not always map 1:1"):

| Category | Billed Action types | Notes verified today |
|---|---|---|
| Workflow | start, reset, search-attribute upsert, options update, child-workflow start | Continue-As-New start counts once, on the new run |
| Activity | schedule (once per execution), retry (only attempts beyond the first — "subtract 1, because the first attempt is not a retry"), heartbeat recorded | A non-retried Activity bills for its **schedule**, not separately for "start"; Temporal's worked example ("two Activities... two `schedule_activity` Actions") confirms one Action per execution |
| Timer | start_timer, including SDK-implicit timers (e.g. `AwaitWithTimeout`) | |
| Signal | signal_workflow, signal_external_workflow, signal-with-start (billed once even if the workflow already existed) | |
| Query | query_workflow | Built-in `__temporal_workflow_metadata` query excluded |
| Update | accepted or rejected, update-with-start | De-duplicated by Update ID |
| Schedule | create/update/patch/delete + the started workflow | |
| Nexus | operation scheduled/canceled | |
| Export, Fairness, Capacity | billed separately, excluded from APS math | Fairness adds +0.1 Action per Action, per hour enabled |

**Heartbeat throttling is real and numeric**: "Temporal SDKs throttle Activity Heartbeats. The default throttle is 80% of the Heartbeat Timeout." A call cadence faster than 80% of `HeartbeatTimeout` does not all reach the server, and does not all get billed. To actually deliver one billed heartbeat every 30s (as Gimbal's scenario assumes), `HeartbeatTimeout` must be ~37.5s or less; a longer timeout (chosen for a more generous stall-detection window) silently lowers both the billed heartbeat count and the Action bill below Section 3's numbers, at the cost of slower kill detection (round 1 Finding 3/5).

**Storage.** Active Storage (open workflows) $0.042/GBh; Retained Storage (closed, within the retention window, max 90 days on Cloud) $0.00105/GBh — a 40x spread rewarding prompt closure. `1GB = 744 GBh/month` (1 GB held all month costs $31.25 Active or $0.78 Retained).

**Plans and minimums**, same page:

| Plan | Minimum monthly | Included Actions | Included Active/Retained Storage |
|---|---|---|---|
| Developer | $0 (10% of usage) | none | none |
| Business | greater of $500 or 10% of usage | 2.5M | 2.5 GB / 100 GB |
| Enterprise | annual, contact sales | 10M | 10 GB / 400 GB |
| Mission Critical | annual, contact sales | 10M | 10 GB / 400 GB |

**Namespace default: 500 Actions/second (APS)**, auto-scaling upward on On-Demand Capacity from the trailing 7 days of usage, never below 500; raised via Provisioned Capacity (billed per Temporal Resource Unit) or by request. Gimbal's scenario (Section 3) never sustains more than ~1–2 Actions/second even at 300 concurrent-ish runs, so 500 APS is not a binding constraint.

## 2. Event history semantics

Fetched from `docs.temporal.io/cloud/actions`, `docs.temporal.io/cloud/limits`, `docs.temporal.io/self-hosted-guide/defaults`, and Temporal's own blogs "Very Long-Running Workflows" and "Getting the most out of the Billable Action Count metric," all read 2026-09-27.

**Heartbeats do not create history events.** The Actions page lists `record_activity_heartbeat` / `record_activity_heartbeat_by_id` / `record_standalone_activity_heartbeat` with History Event Type **N/A** — a heartbeat only updates the pending activity's *mutable* last-heartbeat-details field, never appended to Event History. This is the single most load-bearing fact for R2.3: **heartbeat cadence is purely a cost and cancellation-latency lever, never a history-size lever.** A `PromiseLoop` does not get closer to continue-as-new by heartbeating more often.

**One `ExecuteActivity` call, no retries, produces three history events**: `ActivityTaskScheduled`, `ActivityTaskStarted`, `ActivityTaskCompleted`. The Workflow Task that reacts to that completion adds three more: `WorkflowTaskScheduled`/`Started`/`Completed`. Temporal's own worked example: *"A Workflow with only a single Activity execution will generate an Event History of length 11"* (2 for `WorkflowExecutionStarted`/`Completed`, 3 for the Activity, 6 for the two Workflow Tasks bracketing it), and *"A Workflow with a single Activity in a loop will generate an Event History of length 5 + 6·i"* — **six events per activity** in steady state. A Signal or Update costs one event for itself plus three for the Workflow Task handling it — four events, unless it rides in on `Signal-With-Start`. Queries generate **zero** history events.

**Hard limits, verified today**: **51,200 events or 50 MB** per Workflow Execution (error, terminates the run); warnings at **10,240 events or 10 MB**. Per-payload blob limit **2 MB** (error) with a **256 KB warning threshold**; per-gRPC-message and per-Event-History-transaction limits both **4 MB**. On **Cloud these are fixed**; **self-hosted, they are dynamic-config knobs** (`HistoryCountLimitError/Warn`, `HistorySizeLimitError/Warn`, `limit.maxIDLength`) an operator can raise — at the cost of the replay-time and failover guarantees the limits exist to protect.

## 3. Two cost/size scenarios

Scenario (a), one run: 20 turns (30 min average, heartbeat delivered every 30s → 60 heartbeats/turn), 40 commands (assumed short, no heartbeats), 5 Steers, 1 Interview. Steer is modeled per round 1's mapping (Finding 5): a Signal to the workflow plus a second, same-session relay Activity, since Temporal has no signal-to-activity primitive. Interview is one Update (accepted) plus one Query to surface the question.

The finalists genuinely differ here: R1.1's "generic executor workflow driven by Updates" (Finalist 1) puts one additional Update in front of every turn/command dispatch, where Finalist 2's generated bespoke workflow calls `ExecuteActivity` directly with no dispatch-Update.

**Actions per run:**

| Contributor | Count | Actions each | Finalist 2 (bespoke) | Finalist 1 (generic + Updates) |
|---|---|---|---|---|
| Workflow start | 1 | 1 | 1 | 1 |
| Turn (schedule + heartbeats) | 20 | 1 + 60 = 61 | 1,220 | — |
| Turn (dispatch Update + schedule + heartbeats) | 20 | 1+1+60 = 62 | — | 1,240 |
| Command (schedule) | 40 | 1 | 40 | — |
| Command (dispatch Update + schedule) | 40 | 2 | — | 80 |
| Steer (signal + relay activity) | 5 | 2 | 10 | 10 |
| Interview (update + query) | 1 | 2 | 2 | 2 |
| **Total Actions/run** | | | **1,273** | **1,333** |
| **Cloud cost/run** ($50/M + 10% support) | | | **$0.070** | **$0.073** |

The 60-Action gap is exactly the 60 dispatch-Updates (one per turn + command) Finalist 1's generic executor needs and Finalist 2's generated code does not — real, but under 5% of the total; heartbeats dominate both (~1,200 of ~1,300 Actions either way).

**History events per run** (six per Activity execution, four per Signal/Update, zero per heartbeat or Query, per Section 2):

| Contributor | Finalist 2 | Finalist 1 |
|---|---|---|
| Workflow start/complete | 2 | 2 |
| 20 turns (activity only, or Update+activity) | 120 | 200 |
| 40 commands (activity only, or Update+activity) | 240 | 400 |
| 5 steers (signal 4 + relay activity 6) | 50 | 50 |
| 1 interview (update 4) | 4 | 4 |
| **Total events/run** | **416** | **656** |
| **Share of 51,200 hard limit** | **0.8%** | **1.3%** |
| **Share of 10,240 warn threshold** | 4.1% | 6.4% |

**At 300 runs/month** (Actions scale linearly; history is per-run and doesn't accumulate across runs unless continued-as-new into the next month):

| | Finalist 2 | Finalist 1 |
|---|---|---|
| Actions/month | 381,900 | 399,900 |
| Raw Action spend | $19.10 | $20.00 |
| With 10% Developer-plan support charge | $21.01 | $22.00 |
| Vs. Business plan's included 2.5M Actions/mo | 15% of allocation | 16% of allocation |

Both sit inside the Business plan's *included* Actions allocation with room to spare — at Gimbal's volume the $500/month plan minimum, not Action usage, would be the real bill on a paid tier; Developer plan (no minimum) puts the true marginal cost at ~$20–22/month for the whole fleet.

**Storage is the genuinely unresolved number.** It is driven by payload bytes held per hour, not event count, and nothing in either round measured a real Gimbal-on-Temporal payload (workflow input, each Activity's argument/return — a bounded excerpt-plus-file-path per round 1 Finding 8). If a run's total in-flight payload footprint is, generously, 1–5 MB and stays "open" (Active Storage) for its ~12-hour wall-clock span, that is 0.001–0.005 GB × 12h × $0.042/GBh ≈ **$0.0005–0.0025/run** — noise next to Action cost, but an order-of-magnitude estimate, not a verified figure (Remaining unknowns).

## 4. Continue-as-new threshold for a PromiseLoop workflow

Using the verified six-events-per-activity, four-events-per-signal-or-update rule (Section 2), isolating a pure `PromiseLoop` of turns at the stated cadence (commands/steers/interviews would only lower these numbers further):

| | Events/turn | Turns to 10,240-event warning | Turns to 51,200-event hard limit |
|---|---|---|---|
| Finalist 2 (bespoke, direct `ExecuteActivity`) | 6 | ≈1,706 | ≈8,533 |
| Finalist 1 (generic executor, dispatch Update + activity) | 10 | ≈1,024 | ≈5,120 |

A real `PromiseLoop` also carries planner-decision Activities and any `Group` fan-out per iteration, so a working generator would need a smaller, workflow-shape-specific per-iteration cost (round 1's open item, Risk 5 in `temporal-go-sdk.md`) rather than these turn-only figures — but even cut by an order of magnitude for planner overhead, continue-as-new insertion is a hundreds-to-low-thousands-of-turns problem, not a tens-of-turns problem. Gimbal runs today are tens of turns; this is not an operational concern at current scale, though it is a real static-analysis feature the generator would eventually need for a very long-lived `PromiseLoop` (a multi-day autonomous agent, say).

## 5. Worker Deployment Versioning

Fetched from `docs.temporal.io/production-deployment/worker-deployments/worker-versioning`, its `sunset-and-gc` and `recover-pinned-workflows` subpages, `docs.temporal.io/develop/go/versioning`, and `temporal.io/blog/ga-worker-versioning-public-preview-upgrade-on-continue-as-new`, all 2026-09-27.

**Round 1's disagreement resolved: both dates are correct, about two different things.** The blog post "Announcing GA for Worker Versioning and Public Preview for Upgrade on Continue-as-New" is dated **Mar 30, 2026** — the *current* Worker Deployment Versioning system (Pinned / Auto-Upgrade / Upgrade-on-CaN) reaching GA. Separately, the patching docs page carries a live warning today: *"Support for the experimental Worker Versioning method before 2025 will be removed from Temporal Server in March 2026."* That is about the **older, pre-2025 experimental Build-ID-based versioning feature**, retired from the server the same month the new system reached GA. One round-1 note read the blog, the other the legacy-removal warning; neither was wrong, they described different features that happen to share a month.

**Status today**: Worker Deployment Versioning is GA, "the recommended default for deploying Workflow code changes in production," across all Temporal SDKs; minimum Go SDK v1.35.0 (current is v1.49.0). **Upgrade on Continue-as-New** is still Public Preview.

**Pinned vs Auto-Upgrade**: a Pinned workflow type is *"guaranteed to complete on a single Worker Deployment Version"* for its entire run; Auto-Upgrade moves to the current/ramping version at each new Workflow Task and must stay replay-safe via patching. Temporal's own decision table names the exact Gimbal shape directly:

| Workflow type | Duration | Recommended behavior | Temporal's own note |
|---|---|---|---|
| AI agent / Chatbot | Weeks | **Pinned + Upgrade on Continue-as-New** | "Long sleeps, uses CaN" |
| Order processing | Minutes | Pinned | Completes before next deploy |

A Gimbal run (tens of minutes to a few hours) sits closer to "Order processing" — **Pinned, no continue-as-new needed for versioning purposes** — unless a very long-lived `PromiseLoop` run is already forced into continue-as-new by history size (Section 4), at which point Upgrade-on-CaN lets it adopt new worker code at that same boundary for free.

**What happens to a Pinned run whose worker deployment version is retired mid-flight**: no forced eviction, no automatic reassignment. A version becomes *Draining* once it stops being Current/Ramping and *Drained* once every pinned workflow on it has closed — checked periodically, no fixed time bound; *"you can consider shutting down the running Workers"* once `DrainageStatus` reports Drained. If an operator tears down old workers **before** drainage completes, an in-flight activity on that pinned run has no poller for its next retry — it does not fail immediately, it sits retrying (heartbeat/schedule-to-close timeouts still apply) until a worker for that exact version returns, or an operator intervenes with a **Versioning Override** (move the run to a different, replay-safe version) or a **Reset-with-Move** (roll history back and re-pin), per the recover-pinned-workflows runbook. **Garbage collection is poller-driven, not time-driven**: a version is deleted only once drained *and* with no pollers for 5 minutes, and only once the deployment exceeds `matching.maxVersionsInDeployment` (100 on Cloud). So "how long old workers must stay up" is, in practice, "until every run pinned to that version finishes" — for Gimbal's hour-to-day-scale runs, on the order of a day, not weeks.

**Finalist 1 vs Finalist 2 under versioning**: Finalist 2's bespoke workflow-per-Gimbal-workflow means a code change to *one* Gimbal workflow forces a new Worker Deployment Version (and drain wait) only for runs of *that* workflow type. Finalist 1's single generic executor workflow means **every** code change to the executor — even one unrelated to a particular Gimbal workflow's logic — creates a new version that every currently-running Pinned run across every workflow type must drain out of before the old workers can retire. This is a real, previously-unquantified cost of "one generic executor": it couples the drain-and-retire cost of a deploy to the busiest hour across the whole fleet, not to the one workflow being changed.

## 6. Self-hosted alternative: a rough sketch

Fetched from `raw.githubusercontent.com/temporalio/helm-charts` (README) and `docs.temporal.io/self-hosted-guide/visibility`, both 2026-09-27. The dollar figures below are **not primary-sourced** (no AWS pricing page fetched this round) — order-of-magnitude, flagged as inferred.

**What the Helm chart needs, verified**: "This Helm chart installs only the Temporal server components. You must provide persistence (databases) for Temporal to use — the chart does not install any database sub-charts." Supported persistence: MySQL, PostgreSQL, Cassandra, or Elasticsearch, configured directly. **Elasticsearch/OpenSearch is not required**: PostgreSQL v12+ (Server v1.20+) can serve as the Advanced Visibility store on its own; ES/OpenSearch is Temporal's own recommendation only for setups spawning "more than a few Workflow Executions" at real scale, not a hard requirement. Cassandra support for Visibility was deprecated in Server v1.21 and removed in v1.24 — not a viable Visibility choice today regardless.

**Rough EKS monthly figure, inferred**, at Gimbal's scenario volume (300 runs/month, single namespace, no HA): a minimal production-shaped deployment (frontend, history, matching, worker server roles, typically separate Deployments/pods) plus a single PostgreSQL instance for both persistence and Advanced Visibility, no Elasticsearch, one EKS cluster:

| Component | Rough monthly cost (inferred) |
|---|---|
| EKS control plane | ~$73 (fixed AWS fee) |
| 2–3 worker nodes running the 4 Temporal server roles | ~$250–450 |
| RDS PostgreSQL (single-AZ, small instance) | ~$100–150 |
| Data transfer, EBS, misc | ~$30–60 |
| **Total infra** | **~$450–730/month** |

Against that: Cloud costs **~$20–25/month** in Actions plus a few dollars of storage at this same volume (Section 3), on the no-minimum Developer plan. Self-hosting is **materially more expensive in raw infrastructure dollars** at Gimbal's volume, before counting the operational burden Cloud removes entirely: patching/upgrading the Temporal Server, running and tuning Postgres, monitoring history/matching-service health, and re-deriving every dynamic-config default (Section 2) Cloud enforces for you. Self-hosting only becomes plausibly cost-competitive at volumes far above this brief's (round 1's `durable-execution-alternatives.md` cited a secondary source's 30–50M-Actions/month break-even, not re-verified here; Gimbal's ~400K Actions/month is roughly two orders of magnitude below it either way).

## Remaining unknowns

- **Actual payload bytes per Gimbal-on-Temporal activity/workflow argument.** Nothing in either round measured this; it drives Active/Retained Storage cost (Section 3) and, at extreme sizes, the 2 MB/256 KB payload thresholds (Section 2). Needs a hands-on activity stub sized against real `Generate`/`RunCommand` return shapes.
- **Exact per-iteration event cost of a real `PromiseLoop`** (planner Activity, adaptive `Group` fan-out) rather than the turn-only figure used in Section 4.
- **Whether "schedule Activity = 1 Action" holds for `session.Fork`ed or `WithSupervisor`-wrapped turns**, which round 1 modeled as extra in-process worker machinery, not extra Temporal-visible Activities — if a supervisor turn needs its own Activity execution, the per-turn cost in Section 3 needs a second line.
- **AWS list-price verification for Section 6**; the $450–730/month figure is reasoned from generally known EC2/RDS/EKS pricing shapes, not a fetched AWS pricing page — planning-grade order of magnitude only.
- **`MaxConcurrentSessionExecutionSize`'s real default** (round 1 Finding 4 flagged this contested between two sources) is unrelated to R1.5/R2.3 numerically but still unresolved and worth closing before betting on Sessions.

## Sources

Verified directly this round (raw HTML fetched with `curl`, stripped to text locally — not a WebFetch/AI summary):

- `docs.temporal.io/cloud/pricing` — read 2026-09-27. Action price ($50/M and volume tiers), storage prices, plan minimums/inclusions, APS default, use-case cost bands.
- `docs.temporal.io/cloud/actions` — read 2026-09-27. Full Action-type taxonomy, per-type History Event Type mapping, 80%-of-HeartbeatTimeout throttle default, "N/A" history mapping for heartbeats and queries.
- `docs.temporal.io/cloud/limits` — read 2026-09-27. 51,200-event/50MB hard limit, 10,240-event/10MB warning, 500 APS default, 2MB/4MB payload and gRPC limits (Cloud).
- `docs.temporal.io/self-hosted-guide/defaults` — read 2026-09-27. Same limits confirmed configurable self-hosted (`HistoryCountLimitError/Warn`, `HistorySizeLimitError/Warn`); 256KB payload warning threshold.
- `docs.temporal.io/self-hosted-guide/visibility` — read 2026-09-27. Elasticsearch/OpenSearch optional; PostgreSQL v12+ sufficient for Advanced Visibility; Cassandra removed in Server v1.24.
- `docs.temporal.io/production-deployment/worker-deployments/worker-versioning` — read 2026-09-27. GA status, Pinned/Auto-Upgrade semantics, the AI-agent/Chatbot "Pinned + Upgrade on CaN" recommendation row.
- `docs.temporal.io/production-deployment/worker-deployments/worker-versioning/sunset-and-gc` — read 2026-09-27. Draining/Drained lifecycle, poller-driven GC, `maxVersionsInDeployment`.
- `docs.temporal.io/production-deployment/worker-deployments/recover-pinned-workflows` — read 2026-09-27. Versioning Override and Reset-with-Move as recovery for a pinned run stuck on a torn-down version.
- `docs.temporal.io/develop/go/versioning` — read 2026-09-27. Live warning that the pre-2025 experimental Worker Versioning method is removed from Temporal Server in March 2026 (the other half of round 1's disagreement).
- `temporal.io/blog/ga-worker-versioning-public-preview-upgrade-on-continue-as-new` — read 2026-09-27, published **Mar 30, 2026**. GA of Worker Deployment Versioning, Public Preview of Upgrade on Continue-as-New.
- `temporal.io/blog/very-long-running-workflows` — read 2026-09-27. Six-events-per-activity, four-events-per-signal history-growth arithmetic; 50K(51,200)/50MB limit stated directly.
- `temporal.io/blog/getting-the-most-out-of-the-billable-action-count-metric` — read 2026-09-27, published Jun 25, 2026. Confirms one `schedule_activity` Action per Activity execution via Temporal's own worked example.
- `raw.githubusercontent.com/temporalio/helm-charts/main/README.md` — read 2026-09-27. Helm chart installs server components only, no bundled database; MySQL/PostgreSQL/Cassandra/Elasticsearch all supported.

Inferred, not fetched from a primary source this round:

- The $450–730/month EKS infra figure (Section 6): reasoned from generally known AWS EKS/EC2/RDS pricing shapes, not read off a live AWS pricing page.
- The 30–50M-Actions/month self-host break-even point, carried over from round 1's `durable-execution-alternatives.md` (secondary source, automationatlas.io, not independently re-verified this round).
- Everything in Sections 3 and 4's tables is arithmetic built on today's verified Action/event rules, applied to the brief's stated scenario — the rules are verified, the scenario's own assumptions (turn count, heartbeat delivery, command/steer counts) are the brief's, not measured from a real run.
