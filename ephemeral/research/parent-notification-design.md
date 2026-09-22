# Parent notifications: composition and delivery plan

Design reviewed with Claude Fable 5.1, 2026-09-22. Research basis:
`/Users/tyler/.codex/worktrees/76c1/gimble/ephemeral/research/parent-workflow-notifications.md`.
This is a delivery plan, not implemented behavior or a new public API.

Consensus: three rounds in reviewer session
`5dc86ff9-69dc-4263-853f-2b20ae8efb53`; round 3 reports **only nitpicks
remain**, with no material findings. Local review artifacts are
`ephemeral/reviews/parent-notifications-fable-round-01.md` through
`parent-notifications-fable-round-03.md`. The repository excludes these
review artifacts from Git. Round 3 leaves four implementation notes:
retain the web origin for reattachment (including a UDS without an origin),
validate workflow availability before creating a worktree, choose the
default progress cadence and relevant changes, and report live proof in
chat/PR rather than committing run records. These do not change the agreed
composition. Host capability experiments remain prerequisites to delivery.

## Authoritative request

Tyler wants a parent ChatGPT or Claude Desktop instance to initiate a Gimble
workflow, finish its turn, and receive occasional later updates without
remaining active polling. He asks for consensus with Claude Fable on the
plan and a software design pattern that composes this behavior into the
existing product without conditionals interleaved throughout it.
Repository AGENTS.md and docs/definition-of-done.md apply. Existing workflow
code remains ordinary Go. A protocol feature is not proof of client behavior.

## Composition: observer plus delivery adapter

Put the observer and its delivery adapter together in a small parent
connector, outside the workflow runtime. The connector subscribes to the
existing Gimble run stream, selects useful updates, and supplies them to a
bound delivery function. In pattern terms this is Observer plus Strategy,
with a ports-and-adapters boundary around the host protocol.

```text
parent --launch tool--> connector --local control--> Gimble runtime
                           ^                            |
                           | existing run SSE           | ordinary workflow
                           +----------------------------+
                           |
                    observation policy
                           |
                    bound send function
                           |
                    original parent turn
```

Both Codex and Claude use this same arrangement. Gimble's runtime never
speaks the parent's protocol. The connector binds the host-specific function
once for a subscription; the shared observer never switches on provider.
A Go function taking context and an update, returning submission error, is
enough. There is no event bus, plugin registry, or interface hierarchy.
Normal error handling and deciding which state changes matter belong in
this one observer, not throughout the application.

The connector is a local MCP server process, launched/configured by its
host. Its process context owns the lifetime of observers, not an individual
tool-call context. If the host shares that process, each observer has its
own immutable destination binding. The launch tool returns while that
process remains connected.
Claude's instance is also a configured channel and writes its notifications
directly; Codex's instance maintains a connection to the shared app-server.
This is one process shape for both adapters, not a runtime callback for one
and an external consumer for the other. No acknowledgement protocol or
per-parent notification stream is added to Gimble.

## Gimble integration and launch ownership

The existing `web.Runtime` continues to own execution. The connector uses
the service's Unix control socket, and the service must already be running
for the first delivery. Setup documents this requirement. Request completion,
MCP tool return, and connector disconnection do not cancel its workflow.
No claim is made about executing through service death or machine shutdown.
If discovery finds multiple live services for the project, require an
explicit control-socket selection instead of silently choosing an owner.
The launch receipt publishes the actual web origin and `/runs/{runID}` URL
from that runtime, or states that no web view is available. With web disabled,
details can still name the run directory, but there is no answering link;
an interview notice must say it requires the web answering surface. Do not
invent a URL from port 8080 or from the Unix control socket.

The first vertical delivery exposes review, then implementation: these have
existing runtime-owned launch entries in `cmd/gimble/workflows.go` and
`web/runtime.go:306`. Factor that existing operation into one application
entry called by the conversation manager and a new control launch handler.
It takes an explicit work directory and the existing workflow-specific
inputs, keeps the actual workflow calls and role bindings, and returns the
real run ID. This launch entry is unaware of parents and notifications.
No new exported root `gimble` API or general workflow catalog is proposed.
Other built-in CLI workflow launches are not silently covered by this slice.

The work directory contract is explicit. Existing conversations retain their
current worktree creation and ownership. For both external review and
implementation, the connector creates an isolated Git worktree on a new
branch from the caller's explicit base revision, using ordinary git. It
passes that absolute path to the launch entry. Review's read-only behavior
is a prompt instruction, not a sandbox guarantee; the isolation keeps its
normal working directory separate from the parent's checkout. The receipt
includes branch and worktree path. The parent/user owns that worktree; retain
it after success, failure, or disconnect, with cleanup through ordinary Git
after inspection/merge. Do not transplant uncommitted changes into it; this
slice reviews the requested committed revision. Pre-launch validation fails
before creating a workflow.

The launch entry's existing one-value `LaunchedRun.Done` must have exactly
one receiver. For conversations it remains `Manager.watchRun`. For an
external launch, a runtime-owned goroutine receives and logs its result.
The connector never consumes this channel: its notification source is the
existing observation snapshot/stream. Thus it cannot steal the result and
make another consumer misreport a closed channel. No completion fan-out
abstraction is needed.

On an ambiguous launch response the connector reports the uncertainty and
uses run discovery; it never automatically retries the launch. This avoids
restarting work without inventing durable idempotent-launch infrastructure.
After receiving the run ID, it attaches the observer and returns the receipt.
A fast finished run is still readable through the snapshot endpoint.

## Observer behavior

The implementation lives in one internal package used by the connector.
It reads the existing snapshot and resumes using stream identity/position.
No notification hooks are added to `Run`, `Generate`, loops, worker harness
adapters, workflow bodies, observation producers, or Svelte components.

Keep only current state, the last reported summary, and unsent important
updates in memory for the connector's lifetime:

- At the requested interval, send compact progress if relevant structured
  state changed. Coalesce intervening activity. No per-token updates and
  no summarizer model. An idle timer wakes code, not the parent model.
- Notify promptly for a pending interview with question ID and the existing
  run-page answering link. Recheck that it remains pending before a retry.
- Notify promptly for observed terminal status, superseding stale progress.
  Preserve the run ID and a details location; do not equate run completion
  with proof that the user's goal was achieved.

Observation remains diagnostic evidence. The connector reports what the
run store says, including unavailable/uncertain status if observations are
missing. EOF or loss of a runtime connection never means success. Delivery
must not strengthen the execution verdict or claim that every result can be
reconstructed when observation failed. A degraded observation stream cannot
cancel execution. The external launch receiver logs the actual returned
error; notification delivery errors stay in the connector.

Slow delivery cannot hold the observation store lock or block a workflow.
The reader coalesces into bounded pending state; a separate sender uses a
context deadline and capped backoff. An unreachable parent is a connector
problem, not a failed workflow. Existing run data remain available to the
user. Expose connector/subscription status in its tool response; no new
notification UI or workflow error fields are needed.

There is deliberately no crash-recovery outbox in the first delivery.
Within a running connector, retry unsent updates and reconcile snapshots
after transport reconnect. After connector restart, an explicit reattach
uses the existing run ID and the parent's confirmed destination; it reports
current state without relaunching work. Exactly-once delivery is not promised.
A lost response can make a retry duplicate a notification. A successful
Claude channel write proves less than a successful Codex RPC response;
neither proves the model processed the update. Stop removes the subscription,
not the workflow. Keep these limits in client setup/help.

## Host binding and capability experiments

### Codex desktop task

The capability experiment must establish how the host identifies the
invoking thread: inspect the connector's own environment, whether it is
spawned per thread or shared, and per-call request metadata. Exercise a
parent and subagent in the same checkout. Prefer verified host-supplied
per-call identity, or process identity only after verifying that process is
exclusive to one thread. A shared process must bind each subscription from
that invocation's identity, not from its startup environment.

During this design session, the parent's shell `CODEX_THREAD_ID` matched
`01a0cb35-9664-7ae2-a736-103f91c26a4a`, the task previously reached through
app-server. That is evidence to investigate, not proof of MCP process scope.
A model-relayed thread ID is caller-supplied data, not host-authenticated
identity. It must not silently become the automatic binding mechanism.
Validate the bound thread against the daemon before launching. If no
unambiguous host-supplied source is available, automatic binding is blocked;
an explicitly operator-configured target can support a narrower integration,
with its loaded-thread validation and limitations stated. Never infer a
target from the selected UI tab, title, or matching workspace. The identity
experiment settles which of these supported paths can ship.

Bind that ID to the running shared daemon. Submit `turn/start` with empty
`input` and `toolOutput`. Reuse socket-discovery knowledge from
`codex/rpc.go`, not worker-session ownership: its adapter Close archives
threads, while this thread is borrowed. The connector never archives the
parent, changes its approval policy, restarts the daemon, or creates a
substitute conversation.

Before accepting this adapter, demonstrate which connected client receives
approval/permission requests for the injected turn, how the desktop renders
and answers them, and what happens on connector disconnect during that turn.
Keep the connection alive for required server requests. If the desktop does
not retain usable approval handling, the experiment must establish a
supported relay or report the route blocked; never reuse the worker adapter's
automatic refusals or bypass approvals to make the proof pass.

### Claude Code channel

The same connector exposes the launch tool and advertises the documented
experimental channel capability. Its bound sender emits
`notifications/claude/channel` to the session that owns the MCP connection.
It uses the legacy MCP negotiation currently required for Channels, plus
the custom-channel preview setup. No arbitrary parent ID is accepted for
this adapter. Confirm that the process remains alive after the parent ends
its turn, that an idle event starts another turn, and that busy-turn events
are handled. A stopped session has no active channel; it is not durable push.

### Ordinary Claude Desktop Chat

This remains a named target of the plan. Run an MCP App experiment before
claiming its delivery: launch a delayed run, finish the parent turn, then
have the mounted widget request a follow-up using `sendMessage` without a
user click. Test switched conversation, backgrounded app, and quit/reopen
separately. Establish both delivery and a new model turn in the same chat.
A UI context update alone is insufficient.

If the only working mechanism depends on a mounted view, document that
support boundary and design that adapter from its observed lifecycle; do
not pretend it is the stdio-channel adapter. If it cannot meet the idle-turn
requirement, record the blocker explicitly and present the proven Codex and
Claude Code routes as narrower capabilities, not completed Claude Desktop
support. No generic widget transport is built speculatively.

## Delivery sequence

1. Run the three host capability experiments above using disposable parents,
   first synthetic delayed events, then real run events. Record versions,
   exact parent identity source, approval routing, new-turn behavior, and
   process lifetime. Use cheap models for behavioral proof. This gate
   determines which host adapters can honestly ship.
2. Deliver review end-to-end: the small parent-neutral control launch seam,
   the connector's observer, and the verified Codex sender. Demonstrate a
   real run ID, a finished parent turn, later progress, and the final report
   with no parent polling. An ordinary run has no notification subscription.
3. Add the verified Claude channel sender and implementation launch with
   explicit worktree creation/ownership. Confirm that the observer logic and
   actual workflow code do not change between destinations. Deliver the
   Desktop Chat adapter if its experiment supports it; otherwise report its
   unresolved host limitation rather than closing that requirement.
4. Add focused package tests for fast completion, one-value result ownership,
   correct destination binding, coalescing, reconnect to current snapshot,
   slow delivery isolation, and failure/EOF not becoming success. Live proof
   covers the actual host UI, final-answer gap, and reconnect. Keep test and
   proof effort tied to these behaviors, not a speculative durability system.

Any implementation is a subsequent task. This task produces the reviewed
plan and the composition decision; runtime support claims require the
experiments above. Internal names are implementation choices.
