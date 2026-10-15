# Can Gimbal initiate desktop Codex sessions through a plugin?

[Proposal](../../proposal.md) · [Authorization research](../findings.md)

## TL;DR

**That is the right integration to investigate first: Gimbal submits work, and the provider desktop creates and operates its session. A polling controller agent is not a requirement.** A plugin could be the adapter, but merely installing a plugin does not supply the reverse session-control interface.

**The first live check found no usable public app-server attachment point in this running desktop.** Its two Codex children used private stdio connections; neither exposed a TCP listener or named public app-server socket. The documented socket path existed but refused a connect-only attempt. No RPC was sent. This result applies to desktop `26.1007.21159`, build `20052`, as inspected on October 9; it does not prove attachment impossible in every configuration. [Live observations and exact scope](live-attachment.md).

**The later implementation resolved the fresh-session native question for the tested app.** PR #440's ordinary desktop-launched MCP worker created two fresh native Chrome tasks with no observed per-run approval, using legitimate persistent app permission. Its owner remained idle between external requests. The earlier [live spike](live-spike.md) encountered temporary consent; its remaining acceptance check is now satisfied for this bounded path. [Implemented worker, evidence, and remaining limits](implemented-worker.md).

## What each route actually establishes

| Route | Established | Missing |
| --- | --- | --- |
| Attach to public app-server | Explicit Unix/WebSocket clients can start/resume threads, submit/steer turns, receive events, and interrupt turns. | A reachable listener belonging to this desktop, plus its native tool setup on externally created threads. Current listener check failed. |
| Start another app-server / SDK | Programmatic session control in a process the caller starts. | Desktop ownership and working native Computer Use; sharing history storage does not establish either. |
| Desktop-launched custom MCP adapter | Ordinary project MCP received genuine context, stayed available while its owner was idle, and accepted external create/read requests through the private desktop bridge. | Packaged marketplace startup/reload, automatic restart recovery, remote ingress, and full Gimbal adapter semantics. Fresh native no-prompt operation is now demonstrated for persistently approved Chrome. |
| Supported remote desktop clients | Official remote-access documentation describes use of host Computer Use and plugins. | A documented arbitrary Gimbal client API for that same route. |
| Deep link / `codex app` | UI handoff to the desktop; traced routing includes composer prefill. | Demonstrated unattended submission and a session lifecycle API. |

Public protocol and SDK claims are traced in the [Codex lane](codex-report.md). Desktop handoff findings are in the [audited agy lane](agy-report.md); the [Claude Fable lane and parent corrections](claude-report.md) document the private bridge and its limits. [Official remote connections documentation](https://learn.chatgpt.com/docs/remote-connections) describes supported clients; it does not itself establish a custom harness contract. Current docs and pinned public source disagree about one non-loopback authentication rollout detail; neither is silently substituted for the other.

## Why the plugin idea remains plausible

The installed first-party MCP server gets `CODEX_APP_TOOLS_PIPE_PATH`, fetches the desktop’s dynamic tool catalog, and forwards tool calls with caller thread and turn metadata. It requires a caller thread ID and uses a length-framed private protocol, not public app-server JSON-RPC. The desktop dispatches calls through its hosted manager or a ready renderer. Those facts establish real machinery, rather than an imagined plugin API. [Saved MCP server, lines 24777–24917 and 24971 onward](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/codex-app-tools-server.mjs:24777); [bounded desktop excerpts](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/validated-desktop-excerpts.md).

The renderer’s `create_thread` handler (`sp`) calls the app’s task-creation function (`oso`, exported as `y2`), which routes local creation through `sso` and `HGi`. That path calls the desktop manager’s `startConversation` with `readThreadCreationInputs` from its desktop input provider. This traces first-party task creation into desktop setup; it does not prove a third-party caller is admitted or that Computer Use succeeds. [Saved exact creation-path slices](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/desktop-create-thread-trace.md).

The bridge authenticates process ancestry and signing identity. A standalone SSH child or sibling LaunchAgent does not become a desktop descendant by sharing a login session. However, OpenAI-signed Node can execute JavaScript from elsewhere: **binary signing checks alone do not establish that third-party plugin scripts are excluded.** Conversely, using the right runtime does not establish admission through every other check or support for this private interface. The traced wrapper is not a complete audit of the downstream task dispatcher. [Exact authorization scope](../findings.md).

This separates two questions: can a desktop-launched adapter receive a remote Gimbal request, and can it legitimately dispatch that request through the provider’s session manager? Plugin packaging addresses deployment; the second question needs an actual supported contract or a deliberately accepted private integration. No controller agent should be inserted merely because that contract is currently unknown.

## What to validate next

The [implemented worker](implemented-worker.md) passed the fresh-task native check twice using the ordinary MCP path. Keep its private interface isolated; do not reopen public attachment research as a prerequisite for every next step. The remaining backend work is to establish remote ingress, durable run/task/session association, exclusive desktop use, and the Gimbal adapter contract. These are narrow integration obligations, not a new server plugin or scheduler system.

The source distinction on cancellation matters: the MCP bridge’s `tools/cancel` aborts a pending tool call; it does not by itself prove cancellation of a task that call created. Public `turn/interrupt` targets an active turn and has its own completion behavior. The backend must demonstrate the latter outcome through whichever interface it actually uses. [Bridge source](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/codex-app-tools-server.mjs:24873); [public interrupt implementation](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/public/codex-rs/app-server/src/request_processors/turn_processor.rs:1619).

## What this changes in the Gimbal proposal

Keep task placement and provider session integration inside the backend. A macOS worker can receive remote requests, but it must invoke a demonstrated provider integration; launching a CLI under launchd is insufficient evidence. The server stores the backend’s run/task/session identities and controls logical ownership without needing to understand Codex’s private IPC. An attached provider desktop is shared infrastructure, so whole-run cancellation must interrupt run-owned sessions rather than kill the user’s desktop application.

The mixed Linux/macOS requirement remains firm. The provider-control experiment now supports continuing its implementation; no general scheduler, new server plugin system, or permission framework is required. Claude’s native Computer Use needs its own integration evidence; a Claude model researching Codex does not prove Claude desktop integration.

## How the research was adjudicated

Independent research ran through a Codex subagent, native Claude Fable CLI, and native agy using Gemini 3.8 Flash High. Their saved sources are retained outside the repository, with manifests and hashes; the linked reports are navigation into those sources, not independent proof of their conclusions.

The parent re-read the public transport, thread creation, tool routing, plugin schema, and interrupt implementations; inspected the installed desktop bridge and native authorizer; and performed the limited live attachment check. The Codex cache’s 38 hashes were verified. The agy cache’s 28 hashes were verified, including 18 available local/archive-origin comparisons after corrections.

The first agy report was rejected for fabricated source snippets, an off-by-one archive extraction, and unsupported impossibility claims. The resumed lane repaired its cache and narrowed its conclusions. Parent audit then removed a remaining invented packaging quote and corrected its use of an internal queue document as an external protocol citation. Both earlier reports are preserved in the external cache. Agreement among agents was not used as evidence.

Claude Fable independently traced the bridge but inferred that plugins categorically cannot receive external work or initiate desktop sessions. Those inferences are rejected: configurable environment forwarding is not first-party exclusivity; binary authorization is not script-origin authorization; and ordinary MCP code is not limited to model-triggered behavior. The report now opens with these parent corrections. Its 67 non-log source artifacts matched recorded hashes, including 28 direct-origin comparisons. The parent also traced the renderer creation path that Fable left uninspected. Those source findings supplied the candidate subsequently exercised in the [live spike](live-spike.md); its narrower runtime claims and remaining gaps are recorded there.

Source manifests: [Codex](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex/manifest.json), [Claude Fable](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/claude/manifest.tsv), [agy](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/agy/manifest.json), and [parent verification](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/manifest.json).
