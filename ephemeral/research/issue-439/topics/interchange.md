# Events and controls

[Back to TL;DR](../proposal.md)

**The server opens a stream; the runner sends events as they happen.** Use HTTP over Unix sockets locally, newline-delimited JSON for events, and ordinary request/reply HTTP for controls. Tens of runs do not justify a broker.

## Why this direction

A subscription avoids repeated polling while keeping connection establishment and reconnect policy in the central server. Runner-initiated uploads would make each runner manage a destination and would still need a return path for controls. Choose subscriptions while runners are reachable; a later outbound tunnel can preserve these semantics.

```text
Browser / CLI → server → runner: controls and artifact reads
                      → runner: open event subscription
                      ← runner: ordered events
```

The runner owns workflow execution, typed results, active sessions, recording, and cleanup. The server reduces observations and serves browser snapshots plus SSE. Browser connections and IPC request contexts do not own the workflow lifetime.

## Recording and replay

Every `Run`, hosted or direct, writes `events.jsonl`: existing lifecycle and agent payloads, placement and native references, and a monotonic journal position. `run.jsonl` and session logs retain their human-readable records and existing per-log `Seq` meanings. This deliberately duplicates recording on disk to supply a single ordered replay source without retaining historical UI state inside the runner.

One writer orders complete records. `GET events` reads after a supplied position and transitions to waiting for new records without a replay/live gap. Readers use bounded buffers; a slow or disconnected observer cannot hold up agent execution or grow an unbounded queue. Heartbeats expose connection loss when the workflow itself is idle.

The server persists its received prefix before advancing its cursor, then reduces it. Reconnect ignores repeated positions and exposes gaps. Runner journal positions and browser projection cursors are distinct. A fresh projection can replay its journal; compaction can wait until replay cost justifies it. After local runner exit, import any missing suffix directly from its retained journal.

Loss of connectivity means unreachable, not completed. Failure to write the authoritative journal retains the public `Run` recording-error verdict. Failure of the server's UI projection is reported separately from execution.

## Control results

| Request | Meaning |
| --- | --- |
| Identity/status | Verify run, launch, build, and protocol identity; report execution and cleanup state. |
| Session steer / turn interrupt | Address current IDs and retain landed, dropped, and error distinctions. |
| Loop steer / interview answer | Queue planner input or answer the exact pending question ID. |
| Run cancellation | Stop the run and retry owned cleanup; return acceptance separately from cessation. |
| Artifact read | Retrieve objects referenced by that run. |

The IPC handler accepts a command ID, calls the existing controller, and records a `control_result` journal envelope containing the ID and returned outcome/error. Public lifecycle payloads and adapter signatures need not acquire this transport detail.

After an ambiguous timeout, match the command ID to its recorded result; absence means unknown. Do not automatically replay steers or answers. Cancellation is different: its [durable stop intent](ownership.md#cancel-one-or-all) remains pending until cessation is established and can safely drive the existing cancellation/cleanup retries. This needs a persistent stop flag, not a general command queue.
