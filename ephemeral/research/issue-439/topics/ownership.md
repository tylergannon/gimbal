# Ownership and restart

[Back to TL;DR](../proposal.md)

**The server durably owns every run it launches. A restart must not make it forget a run or require anyone to search the process list.** Independent execution lets a runner survive the server; it does not release the server's responsibility to find and stop it.

## One directory, one active server

A server instance is identified by its canonical state/run directory. Default to one stable per-user location, independent of the current working directory. An explicit different directory creates a different instance. Acquire a nonblocking exclusive OS lock on `server.lock` before reading or changing instance state; refuse startup if it is held. Keep the descriptor open for the server lifetime, close-on-exec, and never pass it to runners.

Leave the lock file in place. The kernel releases the lock when the server exits or crashes; file existence is not evidence of a live owner. Do not unlink the file to recover: a replacement inode could allow a second lock holder. A replacement server takes the released lock and reads the same `instance-id` and ownership records. This is one active server per directory, not one server per machine. The directory uses a filesystem with the required lock semantics; placing compute remotely does not require sharing this directory among machines or adding distributed server failover.

## Record ownership before launch

Add an `instance-id` file under `--instance-dir`, created atomically once and reused on every restart. This is the durable owner identity. Today's random per-process control ID remains endpoint metadata; it does not determine ownership. Use a small durable record per run at `projects/<project>/runs/<run>/owner.json` under that directory. Also keep the existing exclusive project admission lock so two active servers cannot control the same project independently.

Before creating the runner process, durably save the assigned run ID, project, workflow/build identity, run-directory path (`<project>/.gimbal/runs/<run>/`), launch identity, and `launching` state. Persist endpoint/process identity as it becomes known. The record also retains last-known execution state, cleanup state, cancellation intent, and the backend execution identity. Recover the observation cursor from the durable journal prefix rather than duplicating it in this record. These are distinct facts: a workflow can have returned while its agent cleanup remains pending.

If reservation storage fails, do not spawn. If the server crashes between spawn and acknowledgment, the record still exists. Reconcile that launch; never blindly start another copy because acknowledgment was lost. The runner publishes `runner.json` in its assigned directory before executing the workflow, identifying its owner instance, run, launch, build, and endpoint. A returned spawn error records `failed-to-start`. The crash window before a startup acknowledgment is handled below.

A PID is useful metadata, not identity or the source of the run list. Descriptor files help reconnect to already-owned runs; their presence alone does not establish ownership or liveness.

### Resolve an interrupted launch

For local launch, acquire an exclusive `process.lock` in the reserved run directory before spawning and pass that locked file descriptor to the child. The launcher closes its copy after spawn; the runner immediately marks the inherited descriptor close-on-exec and retains its copy until exit. Do not explicitly unlock the shared descriptor, or pass it to agent subprocesses. This small OS-held lifetime marker covers the crash window before a PID or endpoint can be saved. It uses the same local-filesystem locking assumption as project admission.

After recovering exclusive project admission, reconcile a descriptor-less launch by attempting that lock: if it is available, neither launcher nor runner remains, so record `failed-to-start`; if held, keep the launch visible as starting/unresponsive and preserve stop intent. A timeout alone does not prove death. Publishing `runner.json` durably must precede all workflow/agent work. A descriptor with no remaining lock means the runner exited; replay its journal and retain unresolved agent cleanup separately. This rule resolves a launch that never executed without pretending a crashed runner cleaned up its agents.

## Reconnect after restart

The replacement server opens the same registry and enumerates every owned run whose cessation is not established, including unresolved launches and cleanup-pending runs. It contacts each recorded endpoint and checks the persistent owner ID together with run and launch identity before attaching observations or sending controls. The identity/status reply carries all three; a new server process reads the same owner ID from disk.

An unavailable endpoint changes connection status to `unreachable`; it does not delete the row or invent a terminal result. A stale endpoint that answers for another identity is rejected. Resume observations from the recovered journal position and reconcile the runner's current execution and cleanup state. Completed records remain history.

Local runners use a run-owned OS execution boundary, with stdout and stderr in their reserved run directory. Distributed backends own the equivalent boundary at every execution site, as specified in the [backend contract](backend-contract.md). A process group is a local implementation, not the definition of a distributed run. Server shutdown leaves them running. The durable registry remains responsible for reconnecting and controlling them. There is no automatic timeout that kills a healthy runner solely because the server is offline.

## Cancel one or all

A single-run cancellation selects its registry entry by run ID. Persist the cancellation intent before acknowledging acceptance, then address the verified runner endpoint. The runner performs whole-run cancellation and owns stopping its agent sessions and completing cleanup.

Expose cancel-all as an instance-level request on the central control endpoint. The CLI resolves that endpoint through discovery under the selected `--instance-dir`, rather than choosing a project-scoped handler.

**Cancel all means every run owned by this server instance at acceptance, across its projects, whose cessation is not established.** Include `launching`, running, unreachable, and cleanup-pending entries. Serialize this selection with launch admission so a concurrent launch reservation cannot be missed; save the selected runs' cancellation intents before acknowledging the request. New launches admitted afterward are outside that request.

A crash partway through saving a cancel-all request can leave some intents saved, but cannot produce a successful acknowledgment. Retrying cancel-all safely selects all remaining runs again. Saved intents survive restarts; deliver them as endpoints become available, including runners that were still starting. Repeated cancellation uses the existing cancellation/cleanup retry behavior.

Report per-run progress: requested, cancellation accepted, cleanup pending, cessation confirmed, or unreachable/pending. Acceptance is never reported as “everything is dead.” Unreachable entries remain owned and actionable. Operators use the server's run list and controls, including after restart; PID hunting and manual SIGTERM are not the normal control path. Force-stop addresses the backend's entire recorded execution, not just its coordinator process. The backend must provide a way to terminate owned execution units when their workflow process is unresponsive, verify identities, stop further scheduling/retries, and report any units whose cessation cannot be confirmed. See the [backend contract](backend-contract.md); no unreachable machine is declared stopped merely because a request was accepted.

## Which runs belong here

All supported hosted launches, including CLI launches, go through the central launcher. Direct library `Run` calls remain owned by their calling process; importing their journals for history does not claim the server launched or controls them. At project admission and explicit history refresh, scan `<project>/.gimbal/runs/`, skip IDs already in the ownership registry, and inspect descriptors before importing direct-run journals. A hosted descriptor naming another owner is foreign-owned, never a direct run; report its owner and leave its files untouched. Direct-run history import does not create ownership records. The server's cancel-all targets its durable ownership records.

See [events and controls](interchange.md#control-results) for replies and [delivery](delivery-and-review.md#what-must-be-demonstrated) for the restart-and-cancel demonstration.
