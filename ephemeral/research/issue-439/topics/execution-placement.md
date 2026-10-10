# Mixed operating systems and remote computer use

[Back to TL;DR](../proposal.md)

**Keep placement in the backend and native session creation inside the provider's desktop.** One authored workflow can build on Linux, submit a macOS computer-use task, and check its result back on Linux. The server tracks one run throughout. The local Codex desktop worker now demonstrates the provider submission path; connecting it to a mixed-platform backend is the next integration boundary.

## Recommended arrangement

A macOS worker runs in the intended user's desktop environment. Codex launches the Gimbal MCP adapter; an external Gimbal client submits work to that adapter, which asks Codex to create and manage its own task. The owner task stays loaded but its agent need not poll or spend turns dispatching work. Linux execution uses the backend's ordinary execution units.

```text
Gimbal server → backend run
                  ├─ Linux task → ordinary process/container/job
                  └─ macOS task → Gimbal desktop adapter → Codex native session
```

[PR #440](https://github.com/tylergannon/gimbal/pull/440) supplies the local adapter with start/read/message/cancel, a request ledger, and saved-app-permission preflight. Two fresh `gpt-5.6-luna` sessions natively captured and operated the actual Chrome app without observed per-run approval. A separate active task received a message and was interrupted by archive-and-stop; its observed foreground process stopped. This is evidence for those paths, not arbitrary descendant containment or every active-turn steering state. [Implementation evidence](../computer-use/session-control/implemented-worker.md) and [worker contract](../../../../plugins/codex-desktop/README.md).

The demonstrated deployment registers the shipped executable as ordinary MCP in a trusted project. The package includes plugin metadata, but marketplace install/reload has not been exercised. It uses a private version-dependent bridge, local Unix ingress, and a loaded owner; it is not yet a `HarnessAdapter`. Keep these details inside the provider integration. The central server should never import or understand Codex's private IPC.

## Provision once; fail explicitly during unattended work

Explicit worker setup grants macOS access, persists native permission for the actual target apps, and provisions the specific MCP binding tool. These are separate permissions. Saved app approval is a prerequisite, not a guarantee of screen access, provider policy, or permission for every future action. Do not write the native consent store or approve dialogs automatically.

Declare the task's required apps before submission. The current adapter rejects a missing saved approval before creating a task. If a task nevertheless requests approval, its next `read` attempts archive-and-stop and reports `approval_required` with whether cancellation was confirmed. **The integrating backend must keep reading active tasks**; the adapter has no autonomous approval watcher. An unanswered sensitive-action request or a permission change must fail visibly and retain unresolved cleanup, rather than leave an unattended run waiting indefinitely. This is ordinary provider observation, separate from the server's event subscription.

One live acceptance scenario per advertised worker capability is sufficient: create a fresh task through the actual ingress, capture/input on its provisioned target, observe completion, and cancel a still-active owned task. Recheck this small scenario after desktop changes; no general permission framework is proposed. The earlier [authorization investigation](../computer-use/findings.md) explains why spawning a CLI via SSH or a sibling LaunchAgent was insufficient. Launchd may supervise future worker availability, but cannot substitute for desktop-launched bridge ancestry or proven provider access.

## Task requirements, backend placement

Keep three facts distinct: what a task requires, which worker is selected, and which provider/desktop session it uses. OS alone is insufficient. A backend-owned profile or queue can express macOS plus Codex, native access to the required apps, and an available desktop. Preserve that requirement through compilation; do not bury it in a prompt or silently substitute a headless worker. No new public placement API or generic scheduler is specified here. This is future backend metadata work: current `ModelBinding`, lifecycle events, and validated `NativeRef` fields do not already encode arbitrary worker placement.

Durably associate each run/task/attempt with its worker owner, request ID, and native thread before dependent work proceeds. The current worker records request-to-thread identity; the backend supplies the run association. Record worker, OS, harness, and opaque native session identity in observations. Gimbal retains ordinary workflow semantics; the backend owns matching, scheduling, and connection setup. A request for Claude computer use requires its own demonstrated provider integration. Fable reviewing this design is not evidence for that integration.

## One controlling task per desktop

Reserve one desktop for one controlling task unless the backend supplies genuinely separate surfaces. The local adapter does not yet enforce this reservation; its request serialization is not execution exclusivity. Keep related turns on the same session while they depend on desktop state. On lost connectivity, reconnect to the recorded task/attempt/session or report uncertainty. Do not blindly replay clicks or submit a second controller.

The provider application and owner task are shared worker infrastructure. Cancelling a run cancels its owned tasks; it must not kill Codex or an unrelated user's application. Release a desktop reservation only when the controlling activity is known to have ceased. The current archive-and-stop result does not establish arbitrary distributed process cleanup; the backend still owes [whole-run termination](backend-contract.md#own-everything-the-run-starts).

## Cross-machine handoff and recovery

Pass typed values and referenced artifacts, materialized on the selected worker; a Linux path has no implied meaning on macOS. Associate OS/architecture executable variants with one workflow build identity. Reuse an existing backend's transport and resource registry to reach the macOS adapter instead of exposing its private local socket unauthenticated. The required remote admission, ownership, and observation behavior does not require a shared filesystem.

A durable adapter ledger prevents blind replay of an uncertain start and records known thread identities. It does not make the owner survive unload or recover automatically after desktop restart. The backend must reconcile those identities, worker availability, and its desktop reservation before taking new work. Losing the shared owner means control is unavailable, not that all its created tasks stopped. Keep uncertain runs visible and cancellable; no automatic rerun is justified by a lost reply.

## What remains to demonstrate

The local provider path is demonstrated. The full required backend must additionally run one authored workflow across Linux and macOS with real artifact handoff, remote submission, exclusive desktop access, reconnect to the same session, and run-wide cancellation that leaves unrelated work alive. Demonstrate Claude separately before advertising both providers. Validate the complete `HarnessAdapter` behavior actually used by the workflow, including typed generation and correct steering outcomes; the worker's current `message` success is not that contract.

Compiler checks can establish that placement and ownership metadata survive lowering. Integration tests and a real run establish desktop access, session continuity, and cessation. Use backend-native primitives and those focused assertions; do not create a second scheduler, distributed log, or worker supervision framework in Gimbal.
