# Prior art: long-running AI coding-agent loops on Temporal (and similar)

Research date: 2026-09-27. Scope per BRIEF.md: can Gimbal project a workflow
onto Temporal, with agent turns run by workers in a per-project EKS
namespace. This note surveys what exists today. All web research; no code
run. Every claim below is marked **[verified: URL, date]** when read directly
from a primary source, or **[inferred]** when it is my synthesis across
sources rather than something a source states outright.

## Findings

1. **Temporal's official AI surface is Python-first and still Public
   Preview.** The OpenAI Agents SDK integration
   (`temporalio.contrib.openai_agents`) went to Public Preview in the Python
   SDK; there is no equivalent for the Go SDK
   **[verified: docs.temporal.io/develop/python/integrations/openai-agents,
   read 2026-09-27; temporal.io/change-log/open-ai-agents-sdk-integration-pp]**.
   It wraps **only the model call** as a Temporal Activity ("model calls are
   executed as Activities, so they retry durably and are not repeated during
   Workflow replay"); `@function_tool`/inline tools run as plain deterministic
   workflow code, `activity_as_tool()` wraps I/O tools as Activities, and MCP
   operations each become their own Activity **[verified: same URL]**.

2. **Long tool calls are handled by two knobs, not a special protocol.**
   `ModelActivityParameters` exposes `heartbeat_timeout` and
   `cancellation_type` for the model-call Activity, and streaming is
   explicitly incompatible with local Activities ("Local Activities support
   neither heartbeats nor the Workflow stream signal channel")
   **[verified: docs.temporal.io/develop/python/integrations/openai-agents]**.
   For a long-running agent loop overall, the documented recommendation is
   Continue-as-New once conversation history approaches Workflow Event
   History limits **[verified: same URL]**; those limits are a hard 50,000
   events / 50MB per Workflow Execution, after which the run terminates in
   error **[verified: temporal.io/blog/very-long-running-workflows, dated
   2024-01-25 — this is the oldest source used here and may be superseded by
   newer server versions; treat the exact numbers as indicative, not
   current-version-verified]**.

3. **Temporal's own Go-language AI sample is a Google ADK integration, not a
   generic coding agent.** `temporalio/samples-go` has a first-party `googleadk`
   contrib sample with sub-scenarios for a multi-agent system, human-in-the-loop
   tool approval, a continue-as-new chat loop, and model-usage metrics
   **[verified via search result of github.com/temporalio/samples-go, listing
   read 2026-09-27]**. The Cookbook otherwise documents Python recipes: a
   durable agentic loop with **Claude tool calling** (Anthropic, not just
   OpenAI) and a non-looping single-turn agent
   **[verified: docs.temporal.io/ai-cookbook, listing read 2026-09-27]**. I did
   not find an official Temporal sample that runs a full coding-agent loop
   (shell/file tools, git) in Go.

4. **A Temporal engineer has already built almost exactly what Gimbal wants,
   in Go, as a personal proof of concept.** `mfateev/temporal-coding-harness-poc`
   (author: Maxim Fateev, a Temporal co-founder) is "a durable agentic coding
   assistant built on Temporal, originally based on OpenAI Codex," in Go, with
   the loop described as "LLM call → tool execution → repeat," six tools
   (`shell, read_file, write_file, apply_patch, list_dir, grep_files`), a
   pluggable LLM provider (OpenAI and Anthropic Claude models), and OS-level
   shell sandboxing (macOS Seatbelt / Linux bubblewrap) with three approval
   modes **[verified: github.com/mfateev/temporal-coding-harness-poc README,
   read 2026-09-27]**. The README does not explain how the workdir/sandbox is
   pinned to one worker across the many tool-call Activities in a turn, nor
   whether git-commit side effects are made idempotent — **this is the exact
   open question the brief raises, and even this closest precedent leaves it
   unanswered in public docs** **[verified absence: same README, read
   2026-09-27]**. It is explicitly a "-poc" repo: not a Temporal product, not
   battle-tested, no stated production users.

5. **Temporal's newer, more complete answer is "Temporal Agent Harness"
   (`temporal-community/temporal-agent-harness`), announced August 20, 2026,
   pre-1.0 and "with documented rough edges."** It is Python-only (no Go)
   **[verified: temporal.io/blog/temporal-agent-harness-durable-agent-infrastructure,
   dated 2026-08-20; github.com/temporal-community/temporal-agent-harness,
   read 2026-09-27]**. Its unit of work is a "turn" — "the interval the agent
   is non-idle," which can outlive one message (`turn_id` vs `message_id`).
   Model calls run through per-provider AI SDK plugins compiled to Activities;
   tools split three ways: `@agent.activity_tool_defn` (a retried, observable
   Activity), `@agent.tool_defn` (inline workflow code), and
   `@agent.callback_tool_defn` (executed on a *client's own machine*, with the
   workflow paused and the call published as an event until the client
   answers) **[verified: same repo README]**. It ships a coding-agent example
   with the identical six tools as Fateev's PoC, executed via a
   "coding-shim" + OpenCode TUI, which is itself evidence that Temporal
   internally treats "coding agent" as the flagship demo for this harness
   **[verified: same README]**.

6. **The Harness's steering mechanism is the most concrete "message into a
   running turn" pattern found anywhere in this research, and it is not a
   Signal-to-an-Activity — it's a workflow-side queue drained by the model
   loop.** A handler configured `MidTurn.ACCEPT` can "join" an already-running
   turn (sharing its `turn_id`) rather than starting a new one; the joining
   handler appends text to a steering list which "the model loop" drains on
   its next iteration, i.e., a running turn is not interrupted mid-Activity —
   its *next* loop iteration picks up the new instruction
   **[verified: same README, quoting `ctx.joined_turn` /
   `self._steering.append(msg.text)`]**. This is fully consistent with what
   Temporal's core messaging docs say generally: Signals/Updates/Queries
   operate on Workflow state, and Temporal's documentation nowhere describes
   messaging a *running Activity* directly
   **[verified absence: docs.temporal.io/handling-messages, read 2026-09-27]**.

7. **Temporal has no first-class "send a message into a running Activity"
   primitive; the practical options are Signal-the-workflow-then-poll, or
   heartbeat-and-cancel.** The one interruption mechanism Temporal documents
   for an in-flight Activity is cancellation: "Activities must heartbeat to be
   cancellable mid-execution... when the abort signal arrives, it cancels the
   main execution scope, which interrupts any running activity (if it
   heartbeats)" **[verified via search synthesis of
   community.temporal.io threads and docs.temporal.io/activity-execution,
   read 2026-09-27 — this is search-engine-summarized, not a single page I
   fetched directly; treat as lower-confidence verified]**. There is no
   "inject text into a live Activity" API; every HITL sample found (Replit,
   `durable-hitl-agents`) instead pauses the *workflow* on a Signal/Update and
   resumes the loop with the human's input as the next turn's input, which is
   architecturally "cancel/replan next iteration," not "reach into the
   process."

8. **Temporal's own official human-in-the-loop demo confirms the
   Signal + `wait_condition` pattern in both directions.**
   `temporal-community/durable-hitl-agents` shows (a) human→agent: a
   `@workflow.signal` handler updates pending state while the workflow
   `wait_condition`s for approval before a Google ADK agent re-reasons; and
   (b) agent→human: a LangGraph agent calls `interrupt(...)`, which Temporal
   preserves durably until a Signal supplies the human's answer as the next
   observation **[verified: github.com/temporal-community/durable-hitl-agents,
   read 2026-09-27]**.

9. **Replit is the best-documented production case of a coding agent on
   Temporal, and it uses Workflow Updates, not Sessions, for steering.**
   Replit Agent (launched September 2024) moved onto Temporal starting
   November 2024, migrated "in a couple of weeks." Each agent run is one
   Temporal Workflow with a unique Workflow ID ("only one active agent
   process per user session"); failure-prone/non-deterministic logic
   (implicitly including LLM calls) lives in Activities; user steering and
   consent-before-continuing go through Temporal's **Workflow Update**
   feature, which can push a message into the running workflow and pause
   before resuming **[verified:
   temporal.io/resources/case-studies/replit-uses-temporal-to-power-replit-agent-reliably-at-scale,
   read 2026-09-27]**. Replit separately uses Temporal to orchestrate
   "Previews" checkpoints (app + DB state snapshots) and deployment pipelines
   **[verified: same source]**. The case study does not disclose how the
   sandbox/container running the agent's actual code execution is placed
   relative to the Temporal worker, nor concrete idempotency mechanics for
   tool side effects — **[gap, confirmed absent in the source]**.

10. **Every non-Temporal coding-agent product surveyed uses a bespoke sandbox
    layer, not a durable-execution engine, for the actual code-running
    process** — Temporal (or Restate/DBOS-class engines) appears only in the
    *orchestration* layer of the small subset of companies that use it
    (Replit) or in demos (Fateev's PoC, Temporal's own Harness), never as the
    thing that owns the sandboxed process itself. Devin, Codex cloud, Claude
    Code cloud, Google Jules, GitHub Copilot coding agent, Sourcegraph Amp,
    and Factory Droid each describe their own container/VM lifecycle
    (ephemeral-per-task or resumable-per-session) with no public mention of
    Temporal, Cadence, Restate, or DBOS underneath **[verified across each
    company's own docs/blog, individually cited in the table below, read
    2026-09-27]**. This is an important negative finding for the brief's
    premise: **no publicly-documented coding-agent product runs its actual
    long-lived agent *session* as a Temporal workflow that also directly hosts
    the sandbox**; Replit's own description keeps the sandbox/Preview
    infrastructure and the Workflow orchestration as separate systems joined
    by Activities.

11. **Sandbox placement in industry splits into two families: ephemeral
    per-task VM (destroy after use) and resumable/snapshotted session
    (pause/restore).** Google Jules is purely the former: "every operation...
    happens inside a dedicated, sandboxed VM... once the task completes,
    whether it succeeds or fails, the environment is destroyed. No persistent
    containers, no shared volumes" **[verified via search synthesis of
    Jules architecture articles, read 2026-09-27 — no official Google
    engineering blog was directly fetched; moderate confidence]**. GitHub
    Copilot coding agent is the same family, riding GitHub Actions:
    "isolated GitHub Actions compute container... destroyed after completion"
    **[verified via search synthesis of github.blog changelog/posts, read
    2026-09-27]**. The resumable family: Devin/Cognition ("suspending a
    session snapshots its files and installed tools so the workspace can
    resume without re-cloning"), Sourcegraph Amp ("orbs," remote machines
    that "keep executing after you disconnect... pick up from any device"),
    Factory Droid (sessions resumable with `droid --resume`, forkable with
    `/fork`, synced to Factory's cloud), and Anthropic's Claude Code cloud
    (task-scoped gVisor container, but "idle sessions expire, the VM is
    reclaimed" — resumable only within its active lifetime, not truly
    snapshot/restore) **[verified: respective sources below, read
    2026-09-27]**.

12. **Cognition's "Devin Outposts" (launched July 21, 2026) is the clearest
    public statement that "agent brain in a durable/managed control plane,
    sandbox on commodity infra" is now a recognized split.** "Devin's agent
    loop stays in Devin's cloud; sessions execute on machines you operate,"
    with named backends Daytona (sub-90ms restore from snapshot), Modal
    (same infra as Cognition's model serving), Cloudflare, NVIDIA OpenShell,
    E2B (sub-second startup), and a reference Kubernetes implementation
    (`devin-outpost-k8s`) **[verified: devin.ai/blog/introducing-devin-outposts,
    dated 2026-07-21]**. This is architecturally close to what Gimbal's brief
    describes (control plane separate from the per-project execution
    namespace) — but Cognition's "brain" stays proprietary/hosted; nothing
    public says what durable-execution engine (if any) sits under it.

13. **OpenAI Codex cloud is the most directly comparable "run your own
    sandbox provider" precedent, and explicitly lists Daytona, E2B, and
    Modal as options alongside a self-hosted `codex exec-server` over
    WebSocket** — "run agents in OpenAI's hosted sandbox, on your own
    infrastructure via `codex exec-server` over WebSocket, or through partner
    integrations with Blaxel, Cloudflare, Daytona, DigitalOcean, E2B, Modal,
    Oracle, Runloop, and Vercel" **[verified via search synthesis of OpenAI
    Codex architecture articles, read 2026-09-27 — the `exec-server`
    WebSocket detail did not come from an openai.com primary page in this
    pass and should be re-verified against OpenAI's own docs before being
    relied on]**.

14. **Snapshot/restore technology that could plausibly run on plain EKS
    exists (CRIU), but no coding-agent company was found publicly using CRIU
    directly for agent sandboxes; everyone doing sub-second resume uses a
    microVM platform (Firecracker) or a vendor's proprietary snapshot format
    (Modal, Daytona), not vanilla containerd/Kubernetes checkpointing.**
    Kubernetes' `ContainerCheckpoint` feature (kubelet Checkpoint API) is
    beta since Kubernetes 1.30 and now default-on, and AWS has a blog on
    "forensic container checkpointing on EKS" **[verified: search results
    citing kubernetes.dev/blog/2026/01/21 and an AWS containers blog post,
    read 2026-09-27 — I did not fetch the AWS post directly, so specifics
    (exact restore latency, GPU support) are the search engine's summary, not
    a page I read myself]**. E2B's restore times (~150ms typical, sub-30ms in
    tuned setups) and Modal's snapshot model (filesystem snapshots default 30
    days retention; memory snapshots capture RAM+filesystem together but
    expire after 7 days and must restore onto an identical instance type)
    both rely on Firecracker microVM snapshotting, which is not what plain
    EKS worker nodes give you out of the box **[verified: modal.com/docs/guide/sandbox-snapshots,
    read 2026-09-27; Firecracker snapshot docs and E2B/Modal comparison
    articles, read 2026-09-27]**. **My inference: on plain EKS (no
    Firecracker/Kata layer, no CRIU integration built and tested), a
    "long-lived pod per session" is achievable, but sub-second
    snapshot/restore of that pod's full process+memory state is not, without
    adopting Kata Containers or a managed microVM sandbox provider
    alongside EKS** **[inferred]**.

15. **Idempotency of tool side effects is treated by Temporal's own
    ecosystem as the developer's problem, stated as a principle, not solved
    by the platform.** Recurring guidance across Temporal's own blog and
    third-party Temporal-focused write-ups: "Write-capable Activities should
    have an idempotency or deduplication contract... a retry that recharges a
    card or double-books a driver is a bug, so anything with a side effect
    needs an idempotency key or a deduplication guard" **[verified via search
    synthesis across xgrid.co/resources and buildmvpfast.com articles about
    Temporal AI-agent patterns, read 2026-09-27 — third-party, not a Temporal
    primary source, moderate confidence]**. For a coding agent this maps
    directly onto Gimbal's git-through-`RunCommand` model: a retried
    "apply patch and commit" Activity must itself be idempotent (e.g. check
    if the commit already exists / working tree already matches) because
    Temporal retries the *whole* Activity from the start on worker death —
    "Temporal checkpoints at the Activity boundary, not inside an inference"
    **[verified via search synthesis, read 2026-09-27]**.

16. **What breaks when a worker dies mid-turn, concretely:** the in-flight
    Activity (e.g., one LLM call, or one tool invocation) is retried from
    scratch on another worker per the Activity's `RetryPolicy`; anything the
    dead worker held in local memory or on local disk *outside* what the
    Activity itself durably wrote (e.g., a half-written file, a git index
    lock, a spawned subprocess) is simply gone and must be either
    reconstructed by the retried Activity or made irrelevant by design
    **[inferred, from Temporal's stated Activity-retry model — verified:
    "Understanding how signals work internally" and retry-policy docs,
    combined with the fileprocessing-sample rationale that a *subsequent*
    Activity may land on a different host unless pinned; read 2026-09-27]**.
    If the agent's actual OS-level session (shell process, checked-out repo)
    is *not* re-derivable from the Activity's own inputs, Worker Sessions or
    a worker-specific task queue is required to guarantee the retry lands
    where the state is — but Sessions are Go-SDK-only and explicitly do not
    yet handle "worker process restarts" as a first-class case
    **[verified: docs.temporal.io/develop/go/sessions, read 2026-09-27]**.

## Table of systems

| System | Durable-execution engine | Where the agent session lives | Steering mechanism (public) | Checkpointing | Public source |
|---|---|---|---|---|---|
| Temporal OpenAI Agents SDK integration | Temporal (Python SDK, Public Preview) | Workflow orchestrates; model calls are Activities; tools inline or `activity_as_tool` | Workflow Update/Signal (framework-level, not integration-specific) | Event History replay; Continue-as-New recommended for long chats | docs.temporal.io/develop/python/integrations/openai-agents |
| Temporal Agent Harness | Temporal (Python, pre-1.0) | Workflow per agent; "turns" as the durability unit | `MidTurn.ACCEPT` join + steering queue drained by model loop; approval policies gate tool calls | AgentEvent stream + replay; callback tools for client-side execution | github.com/temporal-community/temporal-agent-harness; temporal.io/blog/temporal-agent-harness-durable-agent-infrastructure (2026-08-20) |
| `temporal-coding-harness-poc` (Fateev) | Temporal (Go, personal PoC) | Workflow drives LLM-call→tool-exec loop; sandbox pinning undocumented | Not documented (Ctrl+C / `--session` resume in the local REPL) | Event History (implicit); no stated idempotency contract for tool side effects | github.com/mfateev/temporal-coding-harness-poc |
| Replit Agent (3) | Temporal (Python) | One Workflow per agent run/session, ID-pinned to the user session | Temporal Workflow Update (pause for consent, then resume) | "Previews" feature uses Temporal to snapshot app+DB state | temporal.io/resources/case-studies/replit-uses-temporal... |
| Devin / Cognition (Outposts) | Not disclosed (agent loop stays in Cognition's cloud) | Long-lived, snapshot/resumable session on partner infra (Daytona, Modal, E2B, Cloudflare, K8s) | Not detailed publicly beyond session pause/resume | Daytona sub-90ms snapshot restore; per-session filesystem+tools snapshot | devin.ai/blog/introducing-devin-outposts (2026-07-21) |
| OpenAI Codex cloud | Not disclosed | OpenAI-hosted sandbox, or self-hosted via `exec-server`/partner sandboxes (Daytona, E2B, Modal, etc.) | Not detailed publicly in sources read | Partner-dependent (E2B/Modal Firecracker snapshots) | search-derived (OpenAI Codex architecture articles); recommend re-verifying against openai.com/docs directly |
| Anthropic Claude Code cloud | Not disclosed | Per-session gVisor container (Anthropic-hosted) or self-hosted sandbox with orchestration still Anthropic-side | Active steering while running ("actively steer Claude to adjust course"); cross-session `SendMessage`/`ListAgents` for independent sessions | None described beyond session lifetime; idle sessions expire and are reclaimed (not snapshot/resume) | code.claude.com/docs/en/claude-code-on-the-web; platform.claude.com self-hosted-sandboxes docs; code.claude.com/docs/en/cross-session-messaging |
| Google Jules | Not disclosed | One ephemeral GCP VM per task, destroyed on completion | Not applicable (no persistent session to steer mid-task, per sources read) | None — no persistence by design | search-derived (Jules architecture articles); no official Google engineering post fetched directly |
| GitHub Copilot coding agent | Not disclosed (built on GitHub Actions) | Ephemeral GitHub Actions container per session; cloud or local sandbox | Not detailed in sources read | None — ephemeral, destroyed after completion | search-derived (github.blog posts); recommend direct fetch of github.blog changelog for confirmation |
| Sourcegraph Amp | Not disclosed | "Orbs" — persistent remote machines threads are hand off to; resumable from any device | Thread hand-off/pickup; no documented mid-run interrupt-and-inject mechanism found | Session/thread state persists on the orb until closed | search-derived (openagents.org, tomrochette.com); no amp.dev primary doc fetched |
| Factory Droid | Not disclosed | Sessions stored locally or synced to Factory cloud; resumable/forkable | `droid --resume`, `/fork`, Sessions API for programmatic access | Cloud-synced session state | search-derived (Factory docs/blog articles); no factory.ai primary doc fetched directly |
| Ona (formerly Gitpod, acquired by OpenAI June 2026) | Not disclosed | Declarative environments (`devcontainer.json`/`automations.yml`), run in Ona cloud or customer VPC | Not detailed in sources read | Environment-level persistence for hours/days (motivation for OpenAI's acquisition) | search-derived (bex.co, digitalapplied.com articles); no ona.com primary architecture doc fetched |
| E2B | Firecracker microVMs (not Temporal) | Per-sandbox microVM, sub-second to ~150ms restore from snapshot | Not a durable-execution steering story; this is a sandbox provider, not an orchestrator | Firecracker memory+filesystem snapshot | search-derived (E2B comparison articles, Firecracker docs); read 2026-09-27 |
| Modal | Not Temporal; own snapshot system on Firecracker-class isolation | Sandboxes run up to 24h; filesystem/memory snapshots extend state beyond that | Not a durable-execution steering story | Filesystem snapshot (30-day default retention); memory snapshot (RAM+FS, 7-day expiry, same-instance-type restore only) | modal.com/docs/guide/sandbox-snapshots (read 2026-09-27) |
| Daytona | Not Temporal | Sandbox provider; sub-90ms restore from snapshot (per Devin Outposts claim) | Not detailed independently; used as a backend by Devin/Codex | Snapshot-based restore | devin.ai/blog/introducing-devin-outposts (secondhand claim about Daytona, not a Daytona primary source) |

## Lessons for Gimbal

1. Nobody public has solved "one Temporal Activity hosts a long-lived,
   steerable OS process for many agent turns" cleanly — even Temporal's own
   flagship Harness and Fateev's PoC leave sandbox-pinning and steering as
   either undocumented or as "join the turn and let the *next* loop iteration
   see your text," not true mid-Activity injection. Gimbal should expect to
   invent this, not adopt it. **[tied to Findings 4, 5, 6]**

2. If Gimbal projects a `session.Generate` call to a Temporal Activity, the
   model call itself should be its own Activity (as every source does),
   separate from the Activity (or Session) that owns the harness process —
   because Temporal's own integration treats "the model call" as the atomic,
   retriable, replay-safe unit, and lets everything else be ordinary
   (re-derivable) code around it. **[tied to Finding 1]**

3. `session.Steer` cannot be "send a message into the running Activity" in
   Temporal terms — no Temporal doc describes that. It should be built as
   Signal/Update-to-workflow, with the workflow appending to a queue an
   already-running turn drains on its next step — exactly Temporal Agent
   Harness's pattern. Gimbal's projection should generate this shape, not try
   to reach into a live process. **[tied to Findings 6, 7]**

4. `WithSupervisor`'s "watches the worker's transcript and steers it mid-turn"
   is a harder requirement than anything found in prior art: even Temporal's
   Harness only supports steering by queuing text for the *next* loop
   iteration, not truly interrupting an in-flight model/tool call. Gimbal
   should scope supervisor interruption to "between tool calls / between
   turns," matching what the ecosystem actually supports, unless it accepts
   building novel infrastructure. **[tied to Findings 6, 7]**

5. Worker Sessions (Go SDK) is the one Temporal-native primitive that matches
   "pin subsequent Activities, and thus a live workdir, to one worker" — but
   it is explicitly documented as not yet handling worker-process restarts,
   and a dead session fails every subsequent `ExecuteActivity()` call with
   `ErrSessionFailed`. Any Gimbal projection relying on Sessions must plan for
   "session died, redo the whole turn against a freshly cloned workdir," not
   "resume in place." **[tied to Finding 16, and the Go-SDK Sessions doc]**

6. Because Temporal retries a whole Activity from the start on worker death
   (not from wherever it got to), every tool side effect Gimbal projects into
   an Activity — a git commit, `RunCommand`, a file write — needs its own
   idempotency check (e.g., "is the working tree already at this state?")
   independent of Temporal's replay guarantees; Temporal's own advice is
   explicit that this is the developer's job, not something the platform
   gives you. **[tied to Finding 15]**

7. Continue-as-New is not optional plumbing for anything like `PromiseLoop` or
   `Iterate` run for many turns: Temporal's hard 50k-event/50MB Workflow
   Execution ceiling means Gimbal's generated Temporal workflow must
   periodically restart itself carrying forward whatever state a `Scope`
   holds, and drain any pending Signals first. This should be a mechanical
   part of codegen, not something workflow authors think about. **[tied to
   Finding 2, and the Continue-as-New blog]**

8. Model-call streaming and local Activities don't mix in Temporal's own
   integration ("Local Activities support neither heartbeats nor the
   Workflow stream signal channel"); if Gimbal's live web page wants to
   stream agent output token-by-token from a Temporal-hosted run, the
   generated Activity must be a normal (non-local) Activity with heartbeats,
   which has cost/latency implications the projection needs to expose to
   authors, not hide. **[tied to Finding 2]**

9. The industry pattern that actually matches Gimbal's stated target
   (durable control plane separate from a replaceable execution sandbox) is
   Devin Outposts and OpenAI Codex's partner-sandbox model — "agent loop
   stays in the vendor's cloud; sessions execute on machines you operate" —
   which validates the brief's premise of workflow-in-Temporal,
   turn-execution-in-EKS-namespace as an architecture other serious players
   have converged on, even though none of them uses Temporal for the split.
   **[tied to Findings 12, 13]**

10. No public coding-agent product does sub-second checkpoint/restore of a
    live agent sandbox on plain, unmodified EKS; everyone who gets fast
    resume uses Firecracker microVMs (E2B, Modal, Daytona) or a proprietary
    snapshot format, and Kubernetes-native CRIU checkpointing is still
    "beta, test per workload class," not a drop-in for a long-lived agent
    pod. Gimbal should plan for "long-lived pod per session, no cheap
    snapshot/restore" on EKS unless it also adopts Kata Containers or an
    external sandbox vendor. **[tied to Finding 14]**

11. Replit — the one confirmed production coding-agent-on-Temporal case
    study — keeps the sandbox/execution infrastructure and the Temporal
    Workflow as separate systems joined by Activities, and uses Workflow
    Update (not Sessions, not a side channel) for its consent/steering UX.
    This is independent validation for point 3 above and suggests Gimbal's
    projection should default to the same shape rather than inventing a
    novel messaging path. **[tied to Finding 9]**

12. Approval-before-continuing ("Interview" in Gimbal's vocabulary) has two
    clean precedents to borrow from: Replit's Workflow-Update-based consent
    pause, and `durable-hitl-agents`'s Signal + `wait_condition` pattern in
    both human→agent and agent→human directions. Both are ordinary
    Signal/Update mechanics on the *workflow*, confirming `Interview` should
    project to a workflow-level Update/Signal handler, never an
    Activity-level one. **[tied to Findings 8, 9]**

13. The one company that ships an official Go+Temporal AI sample
    (`temporalio/samples-go`'s `googleadk` package) chose Google's ADK, not
    a from-scratch coding-agent harness, and includes a
    "continue-as-new chat" scenario as a named example — meaning Temporal
    itself treats continue-as-new-for-chat-history as important enough to
    demo explicitly, reinforcing lesson 7. **[tied to Finding 3]**

14. Fateev's PoC being a personal, unofficial, "-poc"-suffixed repo (not
    endorsed as a Temporal product) is itself informative: even the person
    who helped design Temporal did not turn "coding agent in Go on Temporal"
    into an official, production-hardened offering. Gimbal should treat this
    space as genuinely unsolved rather than assume a ready-made template
    exists to crib from. **[tied to Finding 4]**

15. Approval/tool-gating in the Temporal Agent Harness ("a seam between the
    model deciding to use a capability and that capability actually
    executing," with static allow-lists, self-declared-safe tools, and an
    "auto mode" that escalates ambiguous calls to a human) is close in shape
    to what `Check` and `WithSupervisor` want to express in Gimbal, and is
    worth reading in full before designing Gimbal's own policy layer, since
    it is the most developed public design for exactly this problem.
    **[tied to Finding 5]**

## Risks

- **Freshness/volatility of the primary evidence.** Both `temporal-agent-harness`
  (announced 2026-08-20) and the OpenAI Agents SDK integration are pre-1.0 /
  Public Preview; APIs, semantics, and even the repos themselves may change
  before Gimbal could depend on them. Any design Gimbal bases on these should
  assume breakage, not stability.
- **Several company-level claims in this note are search-engine syntheses,
  not pages I fetched and read myself** (flagged inline as such): Google
  Jules architecture, GitHub Copilot coding agent's Actions-based sandbox,
  Sourcegraph Amp's orb model, Factory Droid's session model, Ona's VPC
  model, and the AWS EKS forensic-checkpointing post. Before any of these
  facts becomes load-bearing for a design decision, fetch and read the
  named primary source directly.
- **No source anywhere describes running a coding agent's actual sandbox
  process *inside* a Temporal Activity for hours** (Gimbal's stated need:
  "agent turns take minutes to an hour," and a session persists across many
  turns). Every precedent either (a) treats the sandbox as external
  infrastructure the workflow calls into per turn, discarding sandbox state
  between Activities except what the workflow explicitly carries, or (b)
  uses Worker Sessions/task-queue pinning, which is Go-only, not
  restart-safe, and not demonstrated at the scale of "many one-hour agent
  turns." Gimbal may be doing genuinely novel integration work here, not
  applying an existing pattern.
- **The brief's premise deserves a pointed pushback on one count**: describing
  Temporal as *the* natural home for "worker running agent turns in an EKS
  namespace" implicitly assumes the sandbox and the durable-execution engine
  belong in the same system. Every mature coding-agent company that has
  published architecture (Devin, Codex, Jules, Copilot, Amp, Factory) instead
  keeps them as two systems: a control plane that owns durability/state
  (whatever tech that is — undisclosed in most cases) and a *separate*,
  interchangeable sandbox-execution layer (Daytona/E2B/Modal/GitHub Actions/a
  proprietary VM fleet) that the control plane calls into per task or per
  session. If Gimbal instead tries to make the Temporal Activity/Workflow
  itself *be* the sandbox host, it is choosing a design nobody else in this
  survey has shipped, and should budget research and fallback time
  accordingly.

## Sources

All read/searched on 2026-09-27 unless a different publication date is noted.

- https://docs.temporal.io/develop/python/integrations/openai-agents — Temporal docs, OpenAI Agents SDK integration (Python).
- https://temporal.io/change-log/open-ai-agents-sdk-integration-pp — Temporal changelog, Public Preview announcement.
- https://github.com/temporal-community/openai-agents-demos — three OpenAI Agents SDK + Temporal demos.
- https://github.com/temporalio/sdk-python/tree/main/temporalio/contrib/openai_agents — source of the integration.
- https://docs.temporal.io/ai-cookbook/openai-agents-sdk-python and https://docs.temporal.io/ai/cookbook/openai-agents-sdk-python — AI Cookbook recipe (duplicate paths for the same content, both resolved).
- https://docs.temporal.io/ai-cookbook — AI Cookbook index, lists a Claude tool-calling recipe and others.
- https://docs.temporal.io/ai — "Durable AI" docs landing page.
- https://github.com/temporalio/ai-cookbook — AI Cookbook source repo.
- https://learn.temporal.io/tutorials/ai/durable-ai-agent/ — Python durable-AI-agent tutorial.
- https://github.com/temporal-community/tutorial-temporal-ai-agent — tutorial source.
- https://github.com/steveandroulakis/temporal-ai-agent and https://github.com/temporal-community/temporal-ai-agent — multi-turn conversational agent demo (Python).
- https://github.com/Origens-Dev/go-temporal-ai-sdk — third-party (not Temporal-official) Go AI-agent SDK.
- https://github.com/agenticenv/agent-sdk-go — third-party Go AI-agent SDK, Temporal-optional.
- https://github.com/temporal-community/durable-hitl-agents — official human-in-the-loop demo, both HITL directions.
- https://github.com/temporalio/samples-go — Go SDK samples repo; `googleadk` and `worker-specific-task-queues` and `workflowstreams` packages referenced.
- https://github.com/temporalio/samples-go/blob/main/worker-specific-task-queues/workflow.go — worker-specific task queue sample.
- https://docs.temporal.io/guides/worker-execution-affinity — worker execution affinity guide.
- https://docs.temporal.io/task-routing — task routing / sticky execution doc.
- https://docs.temporal.io/develop/go/sessions — Worker Sessions, Go SDK (Go-only feature, noted limitations).
- https://community.temporal.io/t/does-the-dotnet-sdk-have-the-session-api-for-complex-fileprocessing/17051/2 — forum, confirms Sessions is Go-only.
- https://docs.temporal.io/handling-messages — Signals/Updates/Queries doc; confirms no Activity-messaging primitive is documented.
- https://docs.temporal.io/develop/python/message-passing — Python message-passing doc.
- https://docs.temporal.io/design-patterns/entity-workflow — Entity Workflow pattern doc.
- https://docs.temporal.io/design-patterns — Temporal design patterns index.
- https://temporal.io/blog/very-long-running-workflows — Continue-as-New guidance, dated 2026-01-25 (note: earlier year than most other sources in this note).
- https://temporal.io/blog/of-course-you-can-build-dynamic-ai-agents-with-temporal — dated 2025-11-12; LLM-call-as-Activity, replay-for-recovery guidance.
- https://temporal.io/blog/building-ai-agents-that-overcome-the-complexity-cliff — dated 2026-03-10; names OpenAI, Abridge, Lovable, Replit, Hebbia as Temporal AI users.
- https://temporal.io/blog/temporal-agent-harness-durable-agent-infrastructure — dated 2026-08-20; Temporal Agent Harness announcement.
- https://github.com/temporal-community/temporal-agent-harness — Harness source repo/README (Python, MIT, version 0.4.0 at read time).
- https://github.com/mfateev/temporal-coding-harness-poc — Maxim Fateev's Go coding-agent-on-Temporal proof of concept.
- https://temporal.io/resources/case-studies/replit-uses-temporal-to-power-replit-agent-reliably-at-scale — Replit case study.
- https://temporal.io/replay/2025 and https://temporal.io/replay/2026 — Replay conference pages.
- https://temporal.io/blog/replay-2026-product-announcements — Replay 2026 product announcements.
- https://www.infoq.com/news/2025/09/temporal-aiagent/ — InfoQ coverage of the OpenAI Agents SDK integration announcement, dated September 2025.
- https://www.businesswire.com/news/home/20251210314521/en/... — Temporal joins Agentic AI Foundation, dated 2025-12-10.
- https://devin.ai/blog/introducing-devin-outposts — dated 2026-07-21; Devin Outposts architecture (Daytona/Modal/Cloudflare/E2B/K8s backends).
- https://modal.com/docs/guide/sandbox-snapshots — Modal filesystem vs. memory snapshot semantics and limits.
- https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md — Firecracker snapshot mechanism.
- https://www.kubernetes.dev/blog/2026/01/21/introducing-checkpoint-restore-wg/ — Kubernetes Checkpoint/Restore Working Group announcement, dated 2026-01-21.
- https://code.claude.com/docs/en/claude-code-on-the-web — Claude Code cloud sessions.
- https://platform.claude.com/docs/en/managed-agents/self-hosted-sandboxes — Claude self-hosted sandboxes docs.
- https://code.claude.com/docs/en/cross-session-messaging — Claude Code `SendMessage`/`ListAgents` cross-session messaging.
- https://anthropic.com/news/claude-code-on-the-web — Anthropic announcement.

Search-engine-synthesized (not individually fetched and read as a page in
this session; lower confidence, listed for traceability and recommended for
direct verification before load-bearing use): Google Jules architecture
articles (gocodeo.com, morphllm.com, weavai.app); GitHub Copilot coding
agent architecture posts (github.blog changelog and product posts,
itnext.io architecture writeup); Sourcegraph Amp architecture (openagents.org,
tomrochette.com); Factory Droid architecture (factory.ai, deepwiki.com,
developersdigest.tech); Ona/Gitpod-OpenAI acquisition coverage (bex.co,
digitalapplied.com); OpenAI Codex cloud/`exec-server` details (aiidelist.com,
datastudios.org, blockainews.com); AWS EKS forensic container checkpointing
blog (aws.amazon.com/blogs/containers); CRIU-on-Kubernetes production-status
claims (oneuptime.com, devzero.io, criu.org/Kubernetes); Temporal
Activity-retry/idempotency guidance (xgrid.co, buildmvpfast.com,
activewizards.com, intuitionlabs.ai).
