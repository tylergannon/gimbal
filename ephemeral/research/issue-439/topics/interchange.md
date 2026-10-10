# Events and controls

[Back to TL;DR](../proposal.md)

**The server opens a stream; the run's backend sends events as they happen.** A local runner implements that endpoint directly and may itself drive a runtime execution backend with remote environments; a compiled distributed consumer can present the same logical run boundary. Neither choice requires compiling ordinary workflow control just to use remote workers. Use HTTP over Unix sockets locally, newline-delimited JSON for events, and ordinary request/reply HTTP for controls. Tens of runs do not justify a broker.

## Why this direction

A subscription avoids repeated polling while keeping connection establishment and reconnect policy in the central server. Runner-initiated uploads would make each runner manage a destination and would still need a return path for controls. Choose subscriptions while runners are reachable; a later outbound tunnel can preserve these semantics.

```text
Browser / CLI → server → runner: controls and artifact reads
                      → runner: open event subscription
                      ← runner: ordered events
```

The runner owns workflow execution, typed results, active sessions, recording, and cleanup. The server reduces observations and serves browser snapshots plus SSE. Browser connections and IPC request contexts do not own the workflow lifetime.

## Why not just tail a directory?

That is a good local alternative: the required journal already exists, server downtime is harmless, and direct reads remove an IPC hop. Referencing that same journal also avoids a second byte copy. At tens of runs, polling offsets or watching files is practical. Readers still need complete-record boundaries, restart cursors, and reconciliation when notifications are missed; files alone provide neither controls nor liveness.

The tradeoff is location. A remote machine's directory is not readable by the central server without a shared filesystem or a transfer service. Keep the journal as the runner's durable source and expose it through the resumable subscription. This spends one stream per run to give local and eventual remote execution the same observation boundary. Direct file reads remain useful for importing finished local runs; no separate live transport optimization is needed now.

## Recording and replay

Each logical run has one authoritative Gimbal event history with lifecycle and agent payloads, placement and native references, and a resumable position. The local backend stores it as `events.jsonl`, replacing overlapping `run.jsonl` and session recording authorities; human/session views are derived from it. A distributed backend provides the same history through its run endpoint, using storage appropriate to that backend. Workers on separate machines do not append to a shared JSONL file. The [backend contract](backend-contract.md) states the collection, retry, and ordering requirements.

One logical append authority assigns positions to complete records. The local file implementation publishes after successful sync; bounded batches may share one sync. A distributed implementation publishes after its durable commit. Subscribers see only committed records. `GET events` reads after a supplied position and transitions to waiting for new records without a replay/live gap. Readers use bounded buffers; a slow or disconnected observer cannot hold up agent execution or grow an unbounded queue. Heartbeats expose connection loss when the workflow itself is idle.

The server reduces committed events. It can reference the authoritative journal when it has access to the same storage; otherwise it persists a replica of the received prefix before advancing its cursor. This is a storage choice behind the same stream contract, not a distinction in workflow semantics. Reconnect ignores repeated positions and exposes gaps. Runner journal positions and browser projection cursors are distinct. A fresh projection can replay its journal; compaction can wait until replay cost justifies it. After local runner exit, import any missing suffix directly from its retained journal.

Loss of connectivity means unreachable, not completed. Failure to write the authoritative journal retains the public `Run` recording-error verdict. Failure of the server's UI projection is reported separately from execution.

## Control results

| Request | Meaning |
| --- | --- |
| Identity/status | Verify persistent owner, run, launch, build, and protocol identity; report execution and cleanup state. |
| Session steer / turn interrupt | Address current IDs and retain landed, dropped, error, and unavailable distinctions. Expose a control only when the selected provider can honor its meaning. |
| Loop steer / interview answer | Queue planner input or answer the exact pending question ID. |
| Run cancellation | Stop the run and retry owned cleanup; return acceptance separately from cessation. |
| Artifact read | Retrieve objects referenced by that run. |

The IPC handler accepts a command ID, calls the existing controller, and records a `control_result` journal envelope containing the ID and returned outcome/error. Public lifecycle payloads and adapter signatures need not acquire this transport detail.

After an ambiguous timeout, match the command ID to its recorded result; absence means unknown. Do not automatically replay steers or answers. Cancellation is different: its [durable stop intent](ownership.md#cancel-one-or-all) remains pending until cessation is established and can safely drive the existing cancellation/cleanup retries. This needs a persistent stop flag, not a general command queue.

The experimental Codex desktop worker does not yet implement this event/control contract: its `read` returns a bounded recent view, `message` does not confirm a landed steer, and archive-and-stop cancels a task rather than interrupting a turn while preserving its session. The backend integration must preserve those distinctions; do not advertise unsupported controls or substitute recent history for the authoritative journal. [Current worker boundary](../computer-use/session-control/implemented-worker.md#remaining-architecture-obligations).
