# Parent notifications in the shared Gimble instance

Revised 2026-09-22 after Tyler identified the active single-instance migration.
This supersedes the earlier connector-process design and its three-round
Fable consensus. That consensus applied to the old topology, not this revision.
Claude Fable 5.1 re-reviewed this revision against the migration in round 04:
only nitpicks remain, with no material findings. Review session:
`5dc86ff9-69dc-4263-853f-2b20ae8efb53`. Its implementation clarifications
are incorporated below.
No application behavior has been implemented or proven by this document.

## Authoritative context and current integration target

Tyler wants a parent ChatGPT or Claude Desktop instance to launch a workflow,
end its turn, and receive occasional updates without model polling. This
must compose into the product without provider conditionals scattered through
execution. One server must serve concurrent parents of different kinds;
provider choice is not a process startup mode.

The active task is **Investigate issue 326**, in
`/Users/tyler/.codex/worktrees/2128/gimble`. Its authoritative migration plan is
`ephemeral/research/issue-326/single-instance-plan.md` there. Read that plan
and its current code before implementation. It says one instance can host
many projects; independently configured instances remain permissible for
tests and separate uses. This is one local user's tool, not a multi-user
security system. Notification work must not invent another runtime owner.

The migration worktree was inspected at `50ad4e6` with ongoing uncommitted
implementation. The following are observed integration points, not claims
that the migration is released or fully validated:

- `web.Instance` owns listeners/control socket and maps projects to project
  `Runtime` state. Each project owns runs, observations and conversations.
- `web.Submit` and `/control/submit` implement the shared submission path;
  generated `gimble run` commands are clients, not runtime constructors.
- Instance selection, owning project and execution workdir are distinct.
  The control client supplies `X-Gimble-Project`; the web routes use
  `/projects/{projectID}/runs/{runID}`.
- Conversation agents use the same CLI, with a conversation association.
  The structured workflow reply and direct conversation-launch callbacks
  are being removed. Do not factor or revive them for notifications.
- Existing project-scoped snapshot/SSE observations remain the source for
  notification policy. `Admission` currently returns the accepted run ID.

Background research is in
`/Users/tyler/.codex/worktrees/76c1/gimble/ephemeral/research/parent-workflow-notifications.md`.
Its transport findings remain useful, but references to the old runtime
and launch topology are historical.

## Composition: a per-subscription observer with a bound delivery endpoint

The pattern remains Observer plus Strategy. Its lifetime/composition owner
is the shared instance. One internal notification component accepts a run
reference and an already-bound delivery function. It observes that project's
existing run state, coalesces useful updates, and calls the function.

```text
Claude parent -- ordinary launch client --+
                                         +--> one Gimble instance
Codex parent  -- ordinary launch client --+      |
                                                +-- project A / runs
                                                +-- project B / runs
                                                |
                                   per-run parent subscriptions
                                     /                     \
                           bound Codex endpoint     bound Claude endpoint
                                  |                        |
                             parent turn              parent turn
```

Instance, project, run, and parent are distinct identities. A subscription
contains the run's owning project and run ID plus its own parent destination
and cadence. Execution workdir is not project identity; a Gimble conversation
ID is not an external Codex thread ID. No server-global current parent or
provider exists. Two subscriptions may bind different providers even when
they watch runs in the same project.

Transport-specific registration handlers construct the delivery function
once per subscription. The common observer receives that function; it does
not switch on provider, ask which app launched the server, or read the
server's startup environment to infer the caller. Ordinary Go function
injection is sufficient. Avoid a plugin framework, general message bus,
interface hierarchy, or provider fields propagated through workflow calls.

The instance owns subscription lifetime and cancels it at shutdown. A
subscription can stop independently of its run. Registration/disconnection
changes a subscription, never the selected instance or workflow ownership.
Workflows with no subscribers incur no notification behavior.

## Launch and attach are separate operations

Use the migration's ordinary `gimble run`/submission contract unchanged for
launch semantics. Once the caller has the accepted run ID, it subscribes on
the same selected instance and owning project. Parent tooling may present
launch-and-subscribe as one interaction, but there is one workflow submission
path, not an MCP-specific launch engine. On an ambiguous launch response,
discover/report the run rather than automatically submitting it again.

Attachment starts with the current snapshot and then follows changes, so
fast completion before subscription is supported. A missing parent transport
must not undo an accepted launch. Reattachment uses the existing run ID and
explicit destination; it never launches work.

This feature adds no worktree-creation policy. The caller supplies the
execution workdir through the existing CLI, and conversation agents retain
the migration's conversation worktree behavior. Repository worktree rules
still apply to coding tasks. Parent notification code neither chooses a
branch nor owns cleanup of execution worktrees.
Parent setup instructions must make the isolated `--work-dir` requirement
explicit for coding tasks; the CLI otherwise defaults to the project checkout.

A subscription setup response should supply the actual project-qualified
run URL from the selected instance, or an absolute run directory when no
usable web origin exists. Retain/query the origin on reattachment too;
never infer it from port 8080 or a Unix socket. The migration's actual route
and instance-selection contract are authoritative.
The instance must retain its web origin; a Unix socket without a configured
web origin also means there is no usable browser URL.

## Transport adapters without more Gimble servers

Codex: registration binds the actual invoking thread on the shared app-server.
The instance's outbound adapter keeps whatever connection lifecycle is
required and submits `turn/start` with empty input and `toolOutput`. It
borrows that thread; it does not own, archive, replace, or change its approval
policy. Its connection serves this destination, not a global Codex mode.

Claude Code: the host may require a stdio MCP/channel process. Treat that as
a thin transport client of the existing instance, analogous to the CLI,
not another Gimble server or notification-policy owner. It opens a
project/run subscription stream on the existing control surface and forwards
already-selected updates as `notifications/claude/channel`. The server's
selected-update stream is a new project-scoped control route, using
`X-Gimble-Project`; it is not the existing raw observation SSE. The server's
bound send function writes to that subscription's stream. Backpressure and
stream errors stay in that subscription. No work executes in the relay and
it does not duplicate cadence/coalescing logic. Current Channels require
legacy MCP negotiation; that belongs to the relay, not the Gimble instance.

The stream path and direct Codex path implement the same internal send
function with an intentionally limited contract: successful local transport
submission, not proof that the parent model processed it. For a streamed
relay that means the connected stream accepted the update, not that Claude
read it. No invented cross-transport acknowledgement system is required.
If the relay disconnects, stop that binding; a reattachment starts from
current state and may repeat a summary. Explain this support limit honestly.

Ordinary Claude Desktop Chat remains a separate target. Test its mounted
MCP App `sendMessage` behavior. If it works, the view consumes the instance's
selected-update stream as another transport client. It has no local copy of
notification policy. A widget's lifetime cannot be presented as durable
background delivery. If the app cannot start the required idle turn, record
that missing host capability; Claude Code is not a substitute for it.

## Observation policy and isolation

One policy uses structured run facts. Start with a configurable three-minute
progress interval, reporting changed scope/task status rather than token
activity; pending interviews and terminal state notify promptly. Interview
updates carry the existing answering URL and question ID; if no answering
surface is available say so. Suppress a question that has already been
answered. No summarizer model or token stream is needed.

Each subscription keeps bounded in-memory pending state: latest progress,
current pending questions, and terminal state. Reading observations and
sending updates are separate so a slow parent cannot block the store or
execution. Use per-send deadlines and capped retry backoff for reachable
outbound adapters; terminal state supersedes stale progress. Normal
conditionals for state changes and errors remain local to this component.

The notification component does not consume a workflow's completion channel
or reinterpret the execution result. It reports the existing observation
state. EOF, transport loss, or missing observations are not success. Missing
notifications never turn a successful workflow into a failure.

There is no durable notification ledger or exactly-once promise in the first
slice. Instance restart loses subscriptions, while project/run history keeps
its existing durability. Explicit reattachment reports current state without
restarting work. Do not make durable execution or offline wake claims from
durable observation files.

## Delivery gates and sequence

1. Wait for #326 to land before implementing notification integration, then
   rebase and verify the landed submission and project-routing interfaces.
   Independent host capability experiments can happen before it lands.
   Do not modify the migration worktree or
   build on its removed callbacks. This plan adds notification subscription
   handling on the same control surface and one instance-owned component;
   no changes to Run, Generate, loops, workflow bodies, or event producers.
2. Prove host binding and wake with disposable parents. For Codex, inspect
   per-call metadata and client process scope, verify the real invoking
   identity for parent and subagent, and check which client handles approval
   requests after event injection/disconnection. Never infer caller identity
   from the shared Gimble process environment. A model-relayed ID is explicit
   caller configuration, not host-authenticated identity; use only a proven
   automatic binding or an honestly documented explicit destination.
   First test an ordinary registration CLI's own `CODEX_THREAD_ID`, supplied
   by its invoking host, alongside per-call MCP metadata. Its scope and
   subagent behavior still need live verification.
3. Test an idle Claude Code channel and ordinary Claude Desktop App follow-up
   independently, including switched conversation and app closure. Keep
   unsupported lifecycle cases explicit. Use cheap models; report exact
   versions, behavior, and model in chat/PR, not committed run artifacts.
4. Deliver one built-in run from launch receipt through subscription, parent
   final answer, progress and terminal update. Notifications are independent
   of the workflow name; no review/implementation-only launch subsystem is
   introduced. Add each host adapter only after its capability experiment.
5. The composition proof is simultaneous subscriptions for a Codex parent and
   a Claude parent on the same instance, including different projects and
   two parents sharing a project. Demonstrate correct routing with no server
   restart or global mode change. Focus package tests on this routing,
   coalescing, fast completion, slow-subscriber isolation and unsubscribe.

Design consensus is about this composition and plan. It is not proof of host
wake behavior. Internal symbol names remain implementation choices; no new
root-package API is proposed.
