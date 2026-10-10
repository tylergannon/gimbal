# Backend ownership and event history

[Back to TL;DR](../proposal.md)

**One logical run, one accountable backend, however many execution sites.** Gimbal defines the observable meaning of ownership, controls, and events. The backend owns scheduling, transport, storage, and deployment. Use its existing scheduler and resource controls; this proposal does not require a new supervisor service or message broker.

## Place individual tasks, not only whole runs

A single workflow must support Linux and macOS tasks, including remote Claude or ChatGPT computer-use sessions. The workflow selects a `WorkflowRole`; ordinary project Go binds that role to a configured adapter, and a compiler maps the role to its queue/worker. Reject missing desktop mappings. The backend owns capability matching and session/desktop assignment; the server preserves and displays those identities and routes controls. See [execution placement](execution-placement.md) for task requirements, desktop exclusivity, session continuity, and the evidence required of backend authors.

## Own everything the run starts

The server persists the backend's durable execution identity before admitting work. The backend must recover the execution units belonging to that identity after a controller restart. Track resources before they can begin work, or use the scheduler's durable admission identity; losing a launch acknowledgment must not make work anonymous. For native tasks, allocate without a turn, persist the task binding, then dispatch behind stop intent, or require recoverable idempotent native creation. The current desktop bridge path does neither across a lost creation reply; [that provider capability is an unresolved integration gate](execution-placement.md#bind-native-identity-before-the-first-work-starts), not a claim established by the local worker.

On each participating machine/container, run-owned work has a local lifetime owner and an enforceable termination boundary. A parent process is useful, but ancestry alone does not guarantee descendants stop when it dies. Dedicated containers and scheduler jobs can supply platform containment. The local backend keeps per-unit process groups behind a durable registration/start gate only for real harness/workload profiles whose descendant behavior has been verified. Escaping profiles require independently enforceable containment or separately addressable native resources; reject unsupported hosted placement before dispatch. For local managed processes, a cooperative stop request is insufficient when the harness is unresponsive. Shared providers use verified native-session controls and retain named cleanup-pending ownership when unavailable; they do not offer the same independent physical force boundary. See [the local force-stop decision](ownership.md#local-force-stop-has-a-concrete-execution-boundary). Shared backend workers may host many runs; stopping one run must target its owned work, not kill the shared worker and unrelated runs.

Whole-run cancellation stops new admissions and retries, then cancels/drains active work across every site. Force-stop uses the execution platform for independently controllable run-owned units even when their workflow processes cannot answer. For shared provider sessions, issue their verified native stop and observe cessation; if the provider is unavailable, retain the provider/session identity, reservation, and stop intent pending recovery rather than kill the shared service. The backend reports acceptance separately from confirmed cessation and names unresolved units. A network partition remains unresolved until platform evidence establishes cessation. Local process-group kill is one implementation of this contract, never its distributed definition.

The current compiled control seam already distinguishes bounded cancellation delivery from local drain (`web/compiled.go`, `Run.CancelHostedRun`). Preserve that distinction while moving control out of the web process. Restart recovery and platform force-stop need explicit backend support beyond that existing callback.

## Present one logical history

The backend collects events from its execution sites into one authoritative Gimbal history per run. It assigns stable replay positions at durable append. These positions express recorded order; they do not pretend to be wall-clock order across machines. Events retain their execution/task/attempt identities so retries are not confused with prior attempts.

For retried event delivery, preserve a producer-incarnation identity and sequence until durable acknowledgment; the collector deduplicates retransmission. If disconnected execution continues, its unacknowledged events must survive locally or in backend storage. Storage failure must become an explicit recording failure, not silent loss. Reuse native backend delivery/storage guarantees where they satisfy these claims. Do not introduce a second distributed log merely because the backend already has a durable history.

The server consumes the same resumable stream and run controls regardless of deployment. Workers never need access to the server's state directory. UI projections and server replicas can be rebuilt from the authoritative history; retention must preserve that history until required transfer completes.

## Guidance and validation

Backend-author documentation must show how its actual execution units implement these claims. Keep automated checks beside the backend they check:

- A second server cannot acquire the same directory; a crashed server does not require lock-file deletion.
- After controller restart, every admitted execution unit remains associated with its run.
- Cancel/force-stop reaches work on two execution sites, prevents later retries, and leaves unrelated runs alive. For local managed profiles, use real long-running tools including descendants in new groups; wedge the runner/harness and prove independent cessation or reject placement. For shared providers, verify native-session stop includes tool cleanup, then make the provider unavailable and confirm the named session remains pending until recovery and cessation.
- A disconnected site remains pending, rather than producing a false “stopped” result.
- Event retransmission and reconnection produce no duplicate logical events or missing acknowledged events; replay yields the same UI state as live consumption.

Use compiler checks or linters for properties visible in generated source: required ownership identity, required control bindings, and forbidden raw spawn paths where the backend defines an approved launch path. A linter cannot establish process containment, crash recovery, or distributed cessation; those require backend-specific integration tests and an observed run. Add lint rules only when the backend exposes a concrete enforceable pattern, not a generic framework in anticipation.
