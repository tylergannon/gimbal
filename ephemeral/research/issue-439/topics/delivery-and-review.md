# Delivery and review

[Back to TL;DR](../proposal.md)

**The first implementation must demonstrate small local runners that remain known and controllable through a server restart.** Remote deployment and mixed-version upgrades are separate follow-up work.

## What must be demonstrated

- Discover project workflows, launch them, and show their live observations and original designs. Two projects may share a workflow name.
- Start multiple runs, restart the server, recover the same owned run IDs, cancel one by ID, then cancel all remaining owned runs. Observe agent cleanup; do not infer cessation from command acceptance or a missing connection.
- Include an unresolved launch and an unreachable or cleanup-pending run in cancel-all. Restart during cancellation. Show saved stop intent and unresolved entries surviving until their outcomes can be established.
- Demonstrate interview answers, session/loop steering, turn interruption, completed-run replay, and importing direct library runs as recorded history.
- Verify the runner dependency graph excludes the web app. Measure idle and transcript-heavy runner memory separately from agent subprocesses. A slow/disconnected observer must not stall execution or create an unbounded IPC queue.

These are observations to make during implementation, not results already achieved. Automated checks that should repeat belong beside the code they test; no separate proof programs or committed run dumps are proposed.

## Current sources

The source audit baseline is `37f8e3a9`; the [saved issue](../issue.json) describes the current in-process boundary. Relevant contracts and implementation include:

- [Project admission and hosted starts](../../../../internal/host/host.go)
- [Run and recording](../../../../run.go)
- [Current controls](../../../../internal/live/live.go)
- [Current socket handling](../../../../web/control.go)
- [Durable observations](../../../../internal/observation/durable.go)
- [Context objects](../../../../contextdata/context.go)
- [Definition of done](../../../../docs/definition-of-done.md)

## Review status

Claude Fable 5.1 reviewed the complete revised tree through [round 09](../../../reviews/20261009-issue-439-round-09.md), with the outcome **only nitpicks remain** and no material findings. The final clarifications name close-on-exec for the lifetime-lock descriptor, include that file in the ownership table, and define synced journal publication. Consensus is complete for this design; runtime behavior and memory savings remain to be demonstrated during implementation.
