# Let a parent assistant sleep while Gimble works

Research date: 2026-09-22. Gimble inspected at `73e7cbb`.
Status: research and proposed experiments, not an implemented integration.

## Desired behavior

A parent conversation starts a Gimble workflow, receives its run ID, and ends
its turn. Gimble continues independently. Progress, a question, failure, or
completion later causes the original parent conversation to receive context
and, when appropriate, take another turn. No model repeatedly waits on a
status command. A small ordinary process may remain connected to an event
stream; keeping a process alive does not require ongoing model inference.

Three separate capabilities must be demonstrated: delivering an event,
putting it into the correct conversation, and scheduling another model turn.
A progress display or desktop notification alone establishes only the first.

## What Gimble already has

The existing runtime supplies most of the producer side:

- `web/runtime.go:306`: `launchConversationWorkflow` starts a configured
  review or implementation workflow using the runtime context. It returns
  an actual run ID and a completion channel before the workflow finishes.
  This is currently an internal conversation path, not a general external
  launch endpoint.
- `internal/conversation/manager.go:419`: `watchRun` waits on that channel
  without model calls and saves completion/error state. It does not start
  a follow-up parent turn.
- `internal/observation/http.go:22`: snapshot and SSE endpoints expose runs
  over the runtime's existing transports. The stream includes terminal state.
- `internal/observation/subscribe.go:57`: reconnect can resume using stream
  identity and position, or receive a replacement snapshot when the suffix
  is no longer available.
- `web/control.go:35` and `cmd/gimble/runs.go:58`: local control and `gimble
  watch` already give another process a way to observe a run. The control
  handler has no generic launch operation today.

Runtime-owned execution survives the initiating request ending, while the
runtime remains alive. It does not imply execution survives process death or
computer shutdown. Durable observations are not durable workflow execution.

## Client findings

### ChatGPT

The documented direct fallback is a scheduled task inside the existing
conversation. It resumes that conversation's context on a schedule and can
check a long-running operation. Local desktop tasks require the computer on
and the app running. This eliminates continuous waiting, but still makes
periodic model calls to check status.
[Scheduled tasks](https://learn.chatgpt.com/docs/automations#schedule-a-task-inside-a-chat)

Web/mobile also support event-triggered tasks for Gmail, selected Slack
channels, and GitHub PR activity on eligible plans. Those triggers are
explicitly unavailable in the desktop app, CLI, and IDE. A Gimble event sent
to an authorized watched channel is a plausible bridge, not a demonstrated
same-parent integration; the supported trigger and existing-chat combination
must be tested. No messages or automations were created during research.
[Event triggers](https://learn.chatgpt.com/docs/automations#trigger-tasks-from-app-events)

A mounted MCP Apps widget can request a conversation follow-up through
`ui/message` / `window.openai.sendFollowUpMessage`. A widget subscribed to
Gimble events is a candidate for an open conversation. It is not a documented
durable callback after navigation or app closure. Updating model context is
a different operation from asking for a follow-up.
[ChatGPT UI bridge](https://developers.openai.com/plugins/build/chatgpt-ui)

The documented anonymized `openai/session` metadata is a correlation ID,
not an address for backend message delivery. The widget session ends when
the iframe unmounts. No general external-server callback that starts a turn
in any existing ChatGPT conversation was established by this research.
[Apps SDK reference](https://developers.openai.com/apps-sdk/reference)

### Claude Code

Claude Code Channels directly target this behavior: a channel MCP server
pushes an event into the already-open session and Claude reacts. The session
process must remain open; this does not require the model to keep generating.
The feature is a research preview and must be explicitly enabled for that
session, with additional organization controls where applicable.
[Channels](https://code.claude.com/docs/en/channels)

This is evidence for Claude Code, not proof that ordinary Claude Desktop
chats support the same mechanism.

A custom local stdio server would expose the start tool and advertise
`experimental["claude/channel"]`, then emit
`notifications/claude/channel` with content and run/event identifiers.
Those identifiers provide routing data, not delivery acknowledgements:
Channels notifications have no acknowledgement contract. The adapter must
not interpret a successful write as proof that the parent processed it.
[Channel reference](https://code.claude.com/docs/en/channels-reference)

For a custom preview channel, Claude documents the
`--dangerously-load-development-channels server:gimble` flag after the server
is configured. Critically, Channels do not register when the MCP connection
negotiates 2026-07-28. The experiment must retain legacy negotiation, such
as `MCP_PROTOCOL_NEGOTIATION=legacy`. This client-specific channel path and
modern MCP Tasks are different integration choices.
[MCP channel compatibility](https://code.claude.com/docs/en/mcp#push-messages-with-channels)

Remote Control can expose the same locally running Claude Code conversation
through web/mobile while keeping its local tools. That is a useful companion
for a channel-enabled session, although the combination still needs live
proof. Ordinary Desktop Chat and Desktop Code startup do not document the
same channel setup. Remote Control does not keep a stopped process alive.
[Remote Control](https://code.claude.com/docs/en/remote-control)

### Claude Desktop

Local MCP extensions can package a binary connector that launches Gimble;
remote connectors are another supported entry point, but originate from
Anthropic's cloud and need a reachable endpoint. Neither entry point by
itself establishes an inbound wake mechanism.
[Local MCP](https://support.claude.com/en/articles/10949351-getting-started-with-local-mcp-servers-on-claude-desktop),
[remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)

MCP Apps are supported in Desktop. A live run widget can call `sendMessage`
to request a conversation message, subject to host handling. The Apps
contract permits consent requirements, rejection, and teardown of a view.
This is worth testing for the open-view case; it supplies no documented
durable wake guarantee when the view disappears. `updateModelContext`
only supplies context for a future turn.
[Apps overview](https://modelcontextprotocol.io/extensions/apps/overview),
[Apps specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)

Cowork schedules are not a substitute for this exact requirement: the
documented scheduled runs have their own sessions, rather than awakening
the original parent conversation.
[Cowork schedules](https://support.claude.com/en/articles/13854387-schedule-recurring-tasks-in-claude-cowork)

### Codex app-server

An application that controls the parent can explicitly resume its stored
thread and start a new turn when a Gimble event arrives. App-server documents
`thread/resume` and `turn/start`; the thread ID preserves conversation
identity. That establishes a custom-parent building block, not permission or
proof of attaching to the desktop app's own task and runtime. The documented
WebSocket transport is experimental.
[App-server](https://learn.chatgpt.com/docs/app-server)

Read-only inspection of the installed local client found a more concrete
lead. `codex --version` reported `0.155.0-alpha.9.2`; `codex app-server daemon
version` reported the running daemon's socket. A WebSocket-over-Unix-socket
connection, initialization, and `thread/loaded/list` succeeded. That list
included this research's original desktop parent; `thread/read` returned
the expected task title and workspace. No resume, new turn, or thread
mutation was sent. This demonstrates local addressability of the actual
parent today, not post-turn wake or desktop rendering of a callback.

This makes the first local experiment concrete: after a parent finishes,
have a separate event listener use the same running daemon and parent ID
to submit `turn/start` with `input: []` and `toolOutput`, then verify that
the desktop task displays it and handles concurrent user activity correctly.
The documented operation preserves the event as tool output and queues it
when a regular turn is already active. This is preferable to manufacturing
a human message. These mutation semantics are documented, not live-tested.
[Tool-output turns](https://learn.chatgpt.com/docs/app-server#start-a-turn)
Use the shared daemon; a separately spawned stdio app-server is not evidence
of integration with the existing desktop task.

`codex/rpc.go:145` already discovers that daemon socket. Reuse the relevant
connection knowledge, not the worker adapter's ownership lifecycle:
`codex/codex.go:413` archives owned threads on close. A callback connector
must leave the user's parent task intact. Capture the invoking parent ID,
not an assumed session-root ID, since a subagent can have a different target.

The experimental request to prove is:

```json
{
  "id": 42,
  "method": "turn/start",
  "params": {
    "threadId": "<original-parent-id>",
    "input": [],
    "toolOutput": {
      "name": "gimble_workflow_event",
      "output": "Run abc completed. Read its result at /absolute/path."
    }
  }
}
```

Codex asynchronous hooks are not an equivalent idle wake mechanism. Their
documented background output waits for a later turn when the conversation
is idle.
[Background hooks](https://learn.chatgpt.com/docs/hooks#how-background-hooks-run)

## What MCP does and does not solve

The current core revision is 2026-07-28. It uses per-request metadata and
explicit subscriptions. Do not combine it with the wire contract of the
experimental Tasks feature from 2025-11-25.
[Versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)

The published Tasks extension can return a durable task handle and deliver
full task-state updates over `subscriptions/listen` / `notifications/tasks`.
Both sides must support the extension. A subscription removes the need for
protocol polling while connected; reconnect still needs reconciliation with
stored task state. The contract does not dictate starting a new model turn
in the original conversation.
[Tasks specification](https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks)

Ordinary request progress, changed-resource notices, and sampling do not
establish that missing scheduling behavior either. Current sampling is
deprecated, and asks for a generation rather than resuming a specific chat.
[Progress](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/progress),
[resources](https://modelcontextprotocol.io/specification/2026-07-28/server/resources),
[sampling](https://modelcontextprotocol.io/specification/2026-07-28/client/sampling)

The official Go SDK supports the modern core protocol, but its published
Tier 1 assessment lists Tasks as unimplemented and the Tasks issue remains
open at research time. Native Tasks therefore adds work without yet proving
the required Desktop behavior. The official extension matrix also supplies
no Tasks support column with which to establish the target clients' support.
[SDK assessment](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/3220),
[Tasks implementation issue](https://github.com/modelcontextprotocol/go-sdk/issues/626),
[client matrix](https://modelcontextprotocol.io/extensions/client-matrix)

## Other technologies considered

A2A standardizes task streaming and webhook push, including callback
configuration and authentication. It could help a disconnected cloud
receiver, but that receiver would still need a host-specific way to resume
the original parent. It does not remove this task's decisive dependency;
Gimble already has a stream for a local adapter to consume.
[A2A async operations](https://a2a-protocol.org/latest/topics/streaming-and-async/)

An OS notification alerts a person; a webhook starts receiver code. Neither
is itself proof that the original assistant resumed. Claude Code's
`asyncRewake` hook can wake an idle session for a background hook failure,
but Channels better match recurring workflow events. Claude routines create
new cloud sessions and therefore serve a different requirement.
[Claude hooks](https://code.claude.com/docs/en/hooks),
[routines](https://code.claude.com/docs/en/routines)

## Proposed Gimble shape

Keep the Go workflows as they are. Put host integration at the boundary:

1. A tool starts a named workflow in an independently owned runtime and
   returns its run ID and result location once it has actually started.
2. An ordinary listener subscribes to the existing run stream and keeps a
   record of which parent requested this run.
3. It coalesces observations into occasional progress updates; questions,
   failure, cancellation, and completion warrant immediate delivery.
4. A client-specific mechanism delivers the update and, where supported,
   starts the next parent turn. The parent can read details on demand.

Keep delivery state separate from execution state. Disconnection must not
cancel work. Reconnection must not launch the workflow again. Bind the
destination to the initiating parent, preserve a cursor or delivered event
identity, and retain pending terminal updates until the delivery path can
accept them. These are proposed integration requirements, not claims about
the current implementation. Do not add a general message bus merely to
connect one run to one parent.

## Proof that would settle the decision

Use a tiny delayed workflow to demonstrate each candidate on the actual
client/version, with no polling tools called by the parent:

1. Parent launches, reports the run ID, and visibly finishes its turn.
2. A later progress event reaches that same conversation and produces a
   new turn where the host is supposed to support it.
3. A later terminal event produces the final report without another user
   message.
4. Switch conversations, background the app, disconnect/reconnect, and
   close/reopen the app separately. Record which conditions are supported.
5. Replay the last delivered event: it must not restart work or produce
   duplicate terminal reports. A second parent must not receive it.

Use the cheapest permitted model for any future live proof and report the
exact model. This research did not run model-backed workflows, configure
client plugins, or demonstrate end-to-end wake behavior.

## Recommendation

This synthesis combines six research lanes conducted by three parallel
agents in two waves: ChatGPT integrations, Claude Desktop, MCP async
protocols, local Codex addressability, Claude Code Channels, and alternative
event delivery. The coordinating agent inspected Gimble's runtime and
observation boundaries. Client support statements are documentation-backed
unless explicitly described as local observations or hypotheses.

Prove the existing local desktop app-server route first: it now has a
verified original-parent address and an explicit documented new-turn API.
Prove Claude Code Channels alongside it as the strongest documented
event-driven host contract. Both need only a small host adapter around
Gimble's existing launch/observation capabilities, subject to the prototype
revealing lifecycle constraints.

For ordinary Claude Desktop Chat, test the mounted MCP App route before
promising anything durable. For ChatGPT's supported product-level fallback,
use scheduled follow-ups in the same chat if periodic model checks are
acceptable. Treat cloud event-trigger bridges as a separate deployment
choice. Do not implement the full MCP Tasks extension merely in the hope
that the desktop host will begin new turns automatically.
