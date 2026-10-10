# Delivery and review

[Back to TL;DR](../proposal.md)

**The first implementation must demonstrate small local runners that remain known and controllable through a server restart.** That local slice is not the full target: mixed Linux/macOS tasks and remote computer-use sessions are required backend capabilities. They need the separate live evidence described in [execution placement](execution-placement.md#what-remains-to-demonstrate). Mixed-version upgrades remain later work.

## What must be demonstrated

- Discover project workflows, launch them, and show their live observations and original designs. Two projects may share a workflow name.
- Start multiple runs, restart the server, recover the same owned run IDs, cancel one by ID, then cancel all remaining owned runs. Observe agent cleanup; do not infer cessation from command acceptance or a missing connection.
- Validate the public [pi adapter](../../../../pi/adapter.go) first; its existing `HarnessAdapter` route makes it a concrete candidate, but its controlled spawn paths still need migration and real tests. Exercise long-running tool work and descendants creating their own groups; wedge the local runner and prove independent cessation. Pi is in-process, so its harness and runner wedge are the same case. Separately wedge a harness subprocess when admitting agy or another subprocess profile; the pi result does not cover it. Add agy/Claude profiles after their real containment checks. Crash at the registration/start gate and verify that no unregistered workload executes.
- On first hosted launch with no shared daemon, require an explicit absent-provider failure and no lazy start. Provision it through the existing setup/lifecycle path, retry, then cancel one run and confirm shared infrastructure survives. Admit shared Codex/OpenCode sessions after actual native interrupt-and-observe checks establish tool cleanup. Make a provider unavailable and show its named session, reservation, and stop intent stay pending until recovery; never kill the shared daemon. No listed profile is claimed already proven for the new backend.
- Include an unresolved launch and an unreachable or cleanup-pending run in cancel-all. Restart during cancellation. Show saved stop intent and unresolved entries surviving until their outcomes can be established.
- Demonstrate interview answers, session/loop steering, turn interruption, completed-run replay, and importing direct library runs as recorded history.
- Verify the runner dependency graph excludes the web app. Measure idle and transcript-heavy runner memory separately from agent subprocesses. A slow/disconnected observer must not stall execution or create an unbounded IPC queue.

These are observations to make during implementation, not results already achieved. Automated checks that should repeat belong beside the code they test; no separate proof programs or committed run dumps are proposed.

## Current sources

The original source audit baseline is `37f8e3a9`; the [saved issue](../issue.json) describes the in-process boundary. The proposal now also includes the merged desktop worker at `1c3eb84c94c5cce42a7f78018ca51e285e789dc1`, whose [implemented scope](../computer-use/session-control/implemented-worker.md) is narrower than a Gimbal backend. Relevant contracts and implementation include:

- [Project admission and hosted starts](../../../../internal/host/host.go)
- [Run and recording](../../../../run.go)
- [Current controls](../../../../internal/live/live.go)
- [Current socket handling](../../../../web/control.go)
- [Durable observations](../../../../internal/observation/durable.go)
- [Context objects](../../../../contextdata/context.go)
- [Definition of done](../../../../docs/definition-of-done.md)

## Implemented evidence and remaining delivery

The user separately authorized and received the local Codex desktop worker in [PR #440](https://github.com/tylergannon/gimbal/pull/440). Fresh native sessions with prior app provisioning, external initiation with an idle owner, active messaging, and bounded archive-and-stop are demonstrated. The local worker received implementation review; that acceptance does not ratify this whole proposal or prove remote execution. Its native evidence is Chrome-specific; before using another app, establish the same fresh-session path against that app.

For changes to the desktop worker, run `npm --prefix plugins/codex-desktop test` as an explicit separate delivery gate, together with the applicable live acceptance check. The repository's current `just test` omits this Node package; a passing repository test command alone is not worker validation. This proposal records the separate gate without changing unrelated build configuration.

Continue with the small local-runner split and durable server ownership above. The compiled/backend seam must preserve task placement and run identities now, without making the central server implement a scheduler. Before the desktop integration can claim complete run ownership, it must demonstrate provider allocation-before-turn with durable native identity before dispatch, or recoverable idempotent native creation. The tested ordinary MCP call starts a turn before returning its ID; this lost-reply ownership gap remains an explicit provider integration dependency.

The subsequent mixed-platform slice must connect the actual backend to the desktop adapter, deliver artifacts, and establish remote ownership, exclusive desktop use, observation, and cancellation. This keeps required remote behavior visible without making unbuilt infrastructure a prerequisite for project workflow discovery.

## Review status

Claude Fable 5.1 [round 09](../../../reviews/20261009-issue-439-round-09.md) returned **only nitpicks remain** for an earlier design. A subsequent [application architecture review](../application-review.md) identified foreign-owner discovery, force-stop/child ownership, and overlapping durable recordings. The current ownership and backend pages resolve those as explicit design decisions, with focused validation claims.

User requirements then added distributed execution, mixed Linux/macOS tasks, and unattended native provider sessions. The local desktop implementation now supplies bounded runtime evidence for that provider path. **This integrated proposal revision is pending fresh Claude Fable review.** Earlier consensus and the worker's implementation acceptance do not establish consensus for the revised application architecture.
