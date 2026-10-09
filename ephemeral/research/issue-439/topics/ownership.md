# Ownership and restart

[Back to TL;DR](../proposal.md)

**The server durably owns every run it launches. A restart must not make it forget a run or require anyone to search the process list.** Independent execution lets a runner survive the server; it does not release the server's responsibility to find and stop it.

## Record ownership before launch

Use the server's persistent instance identity and a small durable record per run, under its instance directory at `projects/<project>/runs/<run>/owner.json`. Keep the existing exclusive project admission lock so two active servers cannot control the same project independently.

Before creating the runner process, durably save the assigned run ID, project, workflow/build identity, run-directory path, launch identity, and `launching` state. Persist endpoint/process identity as it becomes known. The record also retains last-known execution state, cleanup state, cancellation intent, and the observation cursor. These are distinct facts: a workflow can have returned while its agent cleanup remains pending.

If reservation storage fails, do not spawn. If the server crashes between spawn and acknowledgment, the record still exists. Reconcile that launch; never blindly start another copy because acknowledgment was lost. The runner publishes `runner.json` in its assigned directory before executing the workflow, identifying its run, launch, build, and endpoint. Missing startup evidence leaves a launch unresolved and visible.

A PID is useful metadata, not identity or the source of the run list. Descriptor files help reconnect to already-owned runs; their presence alone does not establish ownership or liveness.

## Reconnect after restart

The replacement server opens the same registry and enumerates every owned run whose cessation is not established, including unresolved launches and cleanup-pending runs. It contacts each recorded endpoint and checks run and launch identity before attaching observations or sending controls.

An unavailable endpoint changes connection status to `unreachable`; it does not delete the row or invent a terminal result. A stale endpoint that answers for another identity is rejected. Resume observations from the saved cursor and reconcile the runner's current execution and cleanup state. Completed records remain history.

Runners use their own OS session/process group, with stdout and stderr in their reserved run directory. Server shutdown leaves them running. The durable registry remains responsible for reconnecting and controlling them. There is no automatic timeout that kills a healthy runner solely because the server is offline.

## Cancel one or all

A single-run cancellation selects its registry entry by run ID. Persist the cancellation intent before acknowledging acceptance, then address the verified runner endpoint. The runner performs whole-run cancellation and owns stopping its agent sessions and completing cleanup.

**Cancel all means every run owned by this server instance at acceptance, across its projects, whose cessation is not established.** Include `launching`, running, unreachable, and cleanup-pending entries. Serialize this selection with launch admission so a concurrent launch reservation cannot be missed; save the selected runs' cancellation intents before acknowledging the request. New launches admitted afterward are outside that request.

A crash partway through saving a cancel-all request can leave some intents saved, but cannot produce a successful acknowledgment. Retrying cancel-all safely selects all remaining runs again. Saved intents survive restarts; deliver them as endpoints become available, including runners that were still starting. Repeated cancellation uses the existing cancellation/cleanup retry behavior.

Report per-run progress: requested, cancellation accepted, cleanup pending, cessation confirmed, or unreachable/pending. Acceptance is never reported as “everything is dead.” Unreachable entries remain owned and actionable. Operators use the server's run list and controls, including after restart; PID hunting and manual SIGTERM are not the normal control path. Force-killing an unresponsive process is separate from cooperative cancellation and must not signal a reused PID or claim remote cleanup succeeded.

## Which runs belong here

All supported hosted launches, including CLI launches, go through the central launcher. Direct library `Run` calls remain owned by their calling process; importing their journals for history does not claim the server launched or controls them. The server's cancel-all targets its durable ownership records.

See [events and controls](interchange.md#control-results) for replies and [delivery](delivery-and-review.md#what-must-be-demonstrated) for the restart-and-cancel demonstration.
