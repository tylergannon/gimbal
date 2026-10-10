# Project workflows and their owning server

## TL;DR

**Keep execution in small, independent project runners; keep ownership and the website in one central server.** The server durably records every launch before it can start, survives a restart without forgetting its runs, and can cancel one or all by identity. One kernel-held lock protects each server state directory. Backends own the execution units on every participating machine.

**The server connects; the backend streams.** Use HTTP over Unix sockets locally: one resumable event subscription per run, with ordinary request/reply controls. The backend owns one authoritative logical history; the server builds the UI from it. At tens of concurrent runs, neither a broker nor polling every control socket is needed. A tunnel or authenticated HTTPS can carry the same boundary remotely; shared directories are not required.

**A workflow can mix Linux tasks and macOS desktop tasks.** The workflow selects a role; ordinary Go role bindings select a backend adapter configured for the required worker. Placement and provider integration belong to the backend. For Codex desktop work, the recommended arrangement is a desktop-launched Gimbal MCP worker: Gimbal submits the task, and Codex creates the native session. The owning agent can remain idle. Build the web application into the server executable and omit it from project runners; a web plugin would add complexity without making this boundary better.

## What is now established

[PR #440](https://github.com/tylergannon/gimbal/pull/440) delivered a local experimental Codex desktop worker. **Two distinct fresh sessions used native Chrome capture and input without an observed per-run approval**, against the real persistently approved Chrome bundle. External submission, observation, active messaging, and archive-and-stop were also exercised. This resolves the narrow question of whether desktop-owned native work can be initiated unattended after provisioning. [Evidence and exact limits](computer-use/session-control/implemented-worker.md).

The worker uses a private desktop bridge and requires its owner task to remain loaded. It is not yet a Gimbal `HarnessAdapter`. Crucially, its native creation starts work before returning an ID: a lost reply can leave an unknown task. The full backend requires provider-enforced allocation/identity persistence before dispatch (or recoverable idempotent creation); that route remains unresolved. Remote ingress, automatic restart recovery, packaged marketplace installation, and mixed-platform workflow execution also remain unproved. Chrome's saved permission does not authorize arbitrary apps or actions. The backend must reject missing prerequisites and turn unexpected approval waits into a visible failure and owned cleanup.

## Simple decisions for the main risks

| Risk | Smallest architectural response | Claim to establish |
| --- | --- | --- |
| Two servers or a crashed server's lock file | Hold an OS lock for the lifetime of one canonical state directory; leave its file in place. | Second startup refuses; crash releases the lock. |
| Forgotten or duplicated runs after restart | Persist identity before work starts; native providers must support allocation-before-turn or recoverable idempotent creation. | The same admitted runs remain listed and cancellable; the current desktop worker has an unresolved creation-reply gap. |
| Lost events, replay races, or redundant recordings | One backend-owned history with durable positions and resumable delivery; derive UI views. | Reconnect yields the same projection as uninterrupted observation. |
| Distributed cancellation leaves work alive | Track jobs/native sessions; gate local commands on durable unit registration and reject placements without verified descendant control. | Real-harness tests establish whole-run stop, including unresponsive harnesses; uncertainty stays visible. |
| Runners retain the entire website and transcript UI | Separate builds and move historical projection into the server. | Runner dependencies exclude web/skgo; history does not grow its projection heap. |
| Native sessions prompt or depend on private interfaces | Provision worker/apps once; isolate the provider adapter and check its actual path after updates. | Fresh Chrome tasks passed; test each additional app before claiming it. Missing access fails explicitly. |
| Two tasks compete or a desktop owner disappears | Reserve the desktop; rebind the owner, or explicitly hand its ledger to a replacement under an exclusive worker lock. | Retain the same native tasks and stop intents; no second controller or blind replay. |

## Drill down

- [Ownership and restart](topics/ownership.md) — directory lock, launch recovery, cancel one/all, and foreign-owned history.
- [Events and controls](topics/interchange.md) — why subscribe, replay and recording, command outcomes, and the directory-tail alternative.
- [Lean runners](topics/lean-runners.md) — executable boundaries and memory.
- [Workflow discovery](topics/workflow-discovery.md) — project commands, builds, and the original design associated with each run.
- [Execution placement](topics/execution-placement.md) — current desktop arrangement, provisioning, mixed Linux/macOS execution, and remaining obligations.
- [Backend contract](topics/backend-contract.md) — distributed ownership, history, cancellation, and focused validation.
- [Storage and remote execution](topics/storage-and-remote.md) — artifact transfer, retention, and transport reachability.
- [Delivery and review](topics/delivery-and-review.md) — the next bounded implementation and current review status.

This is the editable proposal for [issue 439](https://github.com/tylergannon/gimbal/issues/439), including the subsequent [requirements](requirements.md). These are proposed application boundaries; the local desktop worker is the one implemented slice described here. Fresh Claude Fable review of this revision is pending.
