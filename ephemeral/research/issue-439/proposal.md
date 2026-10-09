# Small workflow runners and one observing server

Design proposal for [Gimbal issue 439](https://github.com/tylergannon/gimbal/issues/439) · October 9, 2026

## Execution and data flow

**Compile project workflows into a small executable. Run each invocation in its own process. Let the central server subscribe to that process over HTTP, with events streamed as they happen and controls sent as ordinary requests.** Use Unix sockets locally. The same application protocol can later use a network connection. Tens of runs need neither a broker nor a scheduling service.

The server initiates the connection; the runner pushes events down the open response. This is a subscription, not repeated polling. It gives us one direction for connection establishment, straightforward request/reply controls, and ordinary Go `net/http` handlers. Runner-initiated uploads would also work, but would make every runner manage a destination and reconnect policy while still requiring a return path for controls. Choose subscriptions while the server can reach the runners.

```text
Browser / CLI → central server → runner: controls and artifact reads
                              → runner: open event subscription
                              ← runner: ordered events, continuously
```

The runner owns the workflow stack, typed `Generate[T]` results, agent sessions, cancellation, cleanup, and recording. The server owns project discovery, launching, the run index, observation reduction, browser snapshots, and browser SSE. Neither browser connections nor IPC request contexts own the workflow lifetime. Cancelling a run is an explicit command.

**Keep the web application out of the runner's import graph.** A generated project entry point links its workflows, runtime, selected adapters, recorder, and small HTTP handler. It does not import `web`, skgo, embedded frontend assets, the built-in workflow catalog, or the source analyzer. Ordinary HTTP does not require loading the web application. Build-time analysis emits command descriptions and design data as sidecar files; typed schemas stay with executable workflow code.

The shared CLI constructs commands from that description, preserving flags, help, validation, and role selection; the executable validates inputs again before execution. The server renders project-specific design metadata through its common frontend. Identify a workflow by project plus name, and bind each run to its executable build and a saved copy of its design. Updating project code builds another executable without rebuilding Gimbal. Local compilation is the first delivery option; prebuilt bundles use the same boundary.

Removing web assets alone does not solve memory growth. Today `run.go` creates an observation `Store` even without a server; it retains completed turns and transcripts. **Move that projection to the central server.** A runner records lifecycle and agent events, with placement and native references, in one ordered, append-only journal. Reuse their existing payloads; add a run identity and monotonic position rather than sending Go objects or UI table mutations. The journal becomes the observation replay source; existing human-readable logs may remain, but are not a second IPC authority.

A single writer establishes event order and publishes only complete records. Readers stream from disk with bounded buffers; a slow server must not accumulate an unbounded runner queue or hold up agent execution. Runtime state and active adapter state still consume memory. The claim is narrower: no web application and no retained historical UI projection in each runner. Central projections should be released when idle and rebuilt from disk, rather than retaining every run forever.

---

## Discovery, recovery, and controls

The launcher reserves a run ID and directory, then starts the executable with validated inputs, model bindings, and that identity. The runner publishes a descriptor containing its endpoint and build identity once ready. The server verifies that identity over the connection before declaring it live. Local discovery scans descriptors at startup and periodically for independently launched runs; a descriptor or PID alone is never evidence of liveness. Project admission stays central; runners own only their run directories and do not acquire the current project-wide `owner.lock` or concurrently mutate the central index.

`GET events` streams journal records after a supplied position, then waits for new records without a replay/live gap. The server persists its received prefix before advancing its resume cursor, reduces those records, and serves the existing snapshot-plus-SSE browser contract. Reconnect resumes after that cursor; repeated positions are ignored and gaps are exposed. Runner sequence numbers and browser projection cursors are separate. A fresh server can replay from the beginning; snapshots and compaction can wait until replay cost justifies them.

A broken connection marks the run **unreachable**, not completed. Heartbeats distinguish an idle workflow from a dead connection. Terminal events record the workflow result, while cleanup status determines whether controls must remain available. Keep the runner reachable while owned cleanup is pending. After local process exit, the server can finish importing its retained journal from disk. Loss of observation connectivity does not cancel execution; recording failures are visible degradation, not invented workflow failure.

The IPC surface follows controls we already have:

| Exchange | Meaning |
| --- | --- |
| Identity and status | Verify run/build/protocol identity; report execution and cleanup state. |
| Event subscription | Lifecycle, transcript events, usage, pending interviews, and control outcomes. |
| Session steer; turn interrupt | Target current session/turn IDs; preserve landed, dropped, and error distinctions. |
| Loop steer; interview answer | Queue planner input or answer the exact pending question ID. |
| Whole-run cancel | Request cancellation and retry retained cleanup; acceptance is not proof of cessation. |
| Artifact read | Fetch run-owned values and saved design data by run-relative identity. |

Requests carry the intended run and target identity. Do not automatically retry a steer or answer after an ambiguous timeout: report an unknown outcome and reconcile against recorded state. No durable command queue or exactly-once protocol is needed initially. Starting another process belongs to the launcher, not this per-run control API; an uncertain launch must be reconciled by its reserved run ID before another launch is attempted.

For containers and other machines, keep paths local to the runner and transfer observation data and referenced artifacts through the protocol. A remote process must retain its journal and artifacts on persistent storage until imported; an ephemeral container cannot simply disappear after its final event. Remote launch, archive transfer, and deletion policy are later work. Reachable HTTPS or a tunnel can carry the same requests; use authenticated transport before exposing it beyond local sockets. Outbound-only networks may eventually need a runner-initiated tunnel. That changes connection establishment, not event or control semantics.

The first implementation should prove project command discovery, launch, live observation, interview/steer/cancel, completed-run replay, and two projects with the same workflow name. Check the runner's dependency graph and measure its idle and transcript-heavy memory separately from agent subprocesses. Demonstrate that a slow or disconnected observer does not stall the workflow or grow its IPC queue. Keep remote deployment, mixed-version compatibility, and seamless upgrades outside that first gate; reject incompatible protocol versions clearly. The substantive change is separating the recorder from the projection and hosted ownership—not introducing a new workflow language or distributed platform.
