# Claude Agent Lifecycle, Streaming Contracts, and Completion Semantics

Update: the [Generate-lifetime experiment](generate-lifetime.md) demonstrated
that a new process can resume the same conversation with a different schema or
no schema. Prefer retaining the process through one logical Generate, rather
than the whole Session, when live work need not span calls. Earlier session-wide
recommendations below are superseded by this narrower option.


## 1. Executive Summary & Problem Framing

Gimbal integrates LLM agents as Go workflows via `claude-agent-sdk-go` (pinned at `v1.1.1-0.20260912021749-9a4ffeca77cc`). In Incident 317 ([`issue-317.md`](issue-317.md)), Claude ran background commands, yielded a waiting turn, and was torn down by Gimbal's per-turn adapter lifecycle ([`claude/claude.go`](../../../claude/claude.go)). Resuming via `--resume` injected an orphan notification before answering the debrief prompt, emitting an empty result that aborted the run. Incident 317 native transcript proves an orphan notification queued before debrief and records show an empty result; the exact raw sequence (`result_index: 0` empty before `result_index: 1`) was observed in controlled reproduction probes (`sdk-text/`, `sdk-structured/`), not from a retained raw stream of Incident 317.

This report synthesizes Anthropic specifications, SDK implementations, 13 audited historical GitHub issues, and independent live probes on CLI `2.1.270` with `claude-haiku-4-5-20251001` ([`sol-semantics.md`](sol-semantics.md), [`terra-lifecycle.md`](terra-lifecycle.md)).

### Core Observations & Boundaries

1. **Protocol Symmetry of Waiting and Completion**: At the wire level, the observed intermediate waiting turns can yield `ResultMessage` with `subtype: "success"`, `stop_reason: "end_turn"` or `"tool_use"`, and `terminal_reason: "completed"`. Wire protocol envelopes alone cannot differentiate waiting from assignment completion ([`topic-001/clips/waiting-turn-vs-task-completion.md`](/private/tmp/gimbal-317-literature/corpus/topic-001/clips/waiting-turn-vs-task-completion.md)).
2. **Structured Conformance Trap**: Structured output is enforced via client-side synthetic tool `StructuredOutput`. Because validation verifies only schema conformance, intermediate waiting yields emit schema-valid payloads (e.g. `{"phase": "waiting"}`), passing validation cleanly ([`topic-003/clips/waiting-turn-conformance-trap.md`](/private/tmp/gimbal-317-literature/corpus/topic-003/clips/waiting-turn-conformance-trap.md)).
3. **Fixed Schema Sufficiency & Runtime Immutability on CLI 2.1.270**: Waiting and continuation of a single assignment do **not** require schema changes; a fixed schema suffices across intermediate waits and wakeups. Runtime schema changes arise only if attempting to preserve one subprocess across successive, differently typed `Generate[T]` calls. While native `initialize` accepts `jsonSchema`, live probes on CLI 2.1.270 show runtime `reinitialize(B)` and `reinitialize(jsonSchema: null)` acknowledge success but fail to update or clear the active schema across subsequent turns or late notifications ([`terra-lifecycle.md`](terra-lifecycle.md)). This is an observed version-specific constraint, not a universal protocol impossibility; schema switching is not a prerequisite of persistence or completion.
4. **Destructive Per-Turn Teardown**: In Go SDK v1.1.1, `SubprocessTransport.Close()` signals `stdin` EOF, awaits exit up to 5s, then kills the top-level PID if it has not exited (`transport.go:583-636`, `subprocess.go:129-135`). Resuming via `--resume` restores conversation history from disk, but in controlled probes running tasks terminated and emitted orphan results (`result_index: 0`) before processing new prompts (`result_index: 1`).
5. **No Existing Product Fix**: Gimbal is not repaired. The adapter still terminates on the first `ResultMessage`. Session-scoped hosting and explicit completion contracts are proposed policies requiring live validation.

---

## 2. Topic 001: Streaming Contracts, Background Tasks, and Process Lifecycles

### 2.1 1:1 Subprocess Architecture and Hosting Contracts
- **Canonical Specs**: [`hosting.md`](https://code.claude.com/docs/en/agent-sdk/hosting.md); [`streaming-vs-single-mode.md`](https://code.claude.com/docs/en/agent-sdk/streaming-vs-single-mode.md) (Local: [`topic-001/sources/agent-sdk-streaming-and-hosting.md`](/private/tmp/gimbal-317-literature/corpus/topic-001/sources/agent-sdk-streaming-and-hosting.md)).
- **Subprocess Model**: Each session runs one `claude` CLI subprocess over stdio (`stdin`/`stdout`/`stderr`). Concurrency scales via $N$ subprocesses; the SDK retains per-session workers. Remote Control has a multi-session server but also spawns workers; see [hosting follow-up](server-hosting.md).
- **Streaming vs Single-Message**: Single-message mode (`claude -p`) exits on turn completion. Persistent streaming input (`ClaudeSDKClient`, `streamInput()`, Go `Stream`) maintains the subprocess across turns, enabling queued inputs, cancellation, and background task notifications.

### 2.2 Stdio Pipe Lifetimes and Teardown Sequence
- **Canonical Specs**: [`headless.md`](https://code.claude.com/docs/en/headless.md) (Local: [`topic-001/sources/claude-code-headless.md`](/private/tmp/gimbal-317-literature/corpus/topic-001/sources/claude-code-headless.md)); Go SDK `transport.go:583-636`, `subprocess.go:129-135` (Local clip: [`topic-005/clips/clip-subprocess-close-lifecycle.md`](/private/tmp/gimbal-317-literature/corpus/topic-005/clips/clip-subprocess-close-lifecycle.md)).
- Closing `stdin` (`EndInput()`) sends EOF. In Go SDK v1.1.1, `SubprocessTransport.Close()` signals `stdin` EOF, awaits exit up to 5s, then calls `runner.Kill()` if the top-level process has not exited.

### 2.3 Background Process Invariants and Termination
- **Canonical Specs**: [`tools-reference.md`](https://code.claude.com/docs/en/tools-reference.md); [`env-vars.md`](https://code.claude.com/docs/en/env-vars.md) (Local: [`topic-001/sources/env-vars-and-tools-reference.md`](/private/tmp/gimbal-317-literature/corpus/topic-001/sources/env-vars-and-tools-reference.md)).
- The live probes establish background-shell survival across generation boundaries while the native process remains alive. They do not establish identical lifetimes for foreground subagents, MCP tasks, or every automatic-backgrounding path.
- Killing the top-level CLI does not prove universal death or survival of detached descendants. The [environment reference](https://code.claude.com/docs/en/env-vars) separately documents supervised background-session handoff through `CLAUDE_CODE_DISABLE_BG_EXIT_HANDOFF`. That specialized handoff is not ordinary SDK close/resume and was not tested here; it is a possible separate hosting investigation.

### 2.4 Environment Controls (`CLAUDE_CODE_DISABLE_BACKGROUND_TASKS`)
- The [environment reference](https://code.claude.com/docs/en/env-vars) documents `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` disabling native background tasks, including Bash/subagent `run_in_background`, auto-backgrounding, and Ctrl+B. This restricts capabilities; it is not a semantic completion signal or a universal command-timeout rule.

---

## 3. Topic 002: SDK Message Hierarchy, Result Origins, and Prompt Correlation

### 3.1 Exported Type System and Message Hierarchy
- **Canonical Specs**: Go SDK `messages.go:155-238, 847-950, 1340-1385` (Local: [`topic-002/sources/source-1-go-sdk-types-and-transports.md`](/private/tmp/gimbal-317-literature/corpus/topic-002/sources/source-1-go-sdk-types-and-transports.md)); TypeScript SDK reference ([`topic-002/sources/source-2-typescript-agent-sdk-reference.md`](/private/tmp/gimbal-317-literature/corpus/topic-002/sources/source-2-typescript-agent-sdk-reference.md)).
- `ResultMessage`: Encapsulates generation completion (`Subtype`, `Result`, `StructuredOutput`, `StopReason`, `TerminalReason`, `ResultIndex`, `Origin`).
- `MessageOrigin`: Provenance (`messages.go:206-224`) via `Kind` (`human`, `task-notification`, `auto-continuation`, etc.). TypeScript mirrors this in `SDKMessageOrigin`.
- `TaskNotificationMessage`: Emitted when tasks reach terminal states (`completed`, `failed`, `stopped`). Contains `task_id`, `tool_use_id`, `status`, `output_file`, and `summary`.
- `BackgroundTasksChangedMessage`: Level signal carrying active `BackgroundTask` records (`messages.go:1344-1358`).

### 3.2 Wire Transport Correlation Gap
- **Canonical Specs**: Go SDK `protocol.go:825-840, 1445-1460`; `cli-protocol.md:78-120` (Local clip: [`topic-002/clips/clip-2-wire-transport-correlation-gap.md`](/private/tmp/gimbal-317-literature/corpus/topic-002/clips/clip-2-wire-transport-correlation-gap.md)).
- Control messages correlate via `request_id`. Conversational messages carry **no prompt correlation ID on the wire** (specifying `session_id`, `uuid`, `origin`, but no prompt ID).
- Wire frames arrive in order over stdio, but the protocol provides **no guaranteed FIFO prompt-to-result mapping under concurrent prompt queues or notification races**. A task completion injects a synthetic notification producing a `ResultMessage` with `origin.kind = "task-notification"`. An adapter consuming the first `ResultMessage` misattributes this notification to the awaiting prompt.

### 3.3 Lifecycle Hooks and Stream Observers
- **Canonical Specs**: Go SDK `options.go:120-135, 1492-1585`; [`hooks.md`](https://code.claude.com/docs/en/agent-sdk/hooks.md) (Local: [`topic-002/sources/source-4-agent-sdk-hooks-specification.md`](/private/tmp/gimbal-317-literature/corpus/topic-002/sources/source-4-agent-sdk-hooks-specification.md)).
- `WithRawMessageObserver` intercepts raw JSON stdout; `WithStderr` intercepts stderr. 20+ hooks (`PreToolUse`, `PostToolUse`, `TaskCreated`, `TaskCompleted`, `SubagentStart`, `SubagentStop`, `SessionStart`, `SessionEnd`, `Stop`, `StopFailure`) monitor tool, task, and session events.

### 3.4 Multi-Turn Session Model vs Reconnectable Transports
- The transport layer provides no auto-reconnect (`transport.go:462`). If the process terminates, the connection closes (`ErrTransportClosed`). Resumption (`--resume`) spawns a fresh subprocess. The intended multi-turn architecture maintains one continuous subprocess via `client.Stream(ctx)` (`client.go:656-754`).

---

## 4. Topic 003: Structured Results, Schema Transitions, and Completion Signals

### 4.1 Synthetic Tool Mechanism for Structured Outputs
- **Canonical Specs**: [`structured-outputs`](https://code.claude.com/docs/en/agent-sdk/structured-outputs) (Local: [`topic-003/sources/claude-docs-structured-outputs.md`](/private/tmp/gimbal-317-literature/corpus/topic-003/sources/claude-docs-structured-outputs.md); clip: [`topic-003/clips/synthetic-tool-mechanism.md`](/private/tmp/gimbal-317-literature/corpus/topic-003/clips/synthetic-tool-mechanism.md)).
- The observed CLI exposes the synthetic tool `StructuredOutput`. These captures do not establish every underlying decoding mechanism. The model is prompted to invoke this tool to conclude its turn. The CLI validates the payload and re-prompts on failure. In CLI 2.1.270 probes, the model adhered strictly to the schema, wrapping foreign data into valid fields rather than triggering retry exhaustion.

### 4.2 The Waiting Turn Conformance Trap
- In probe `schema-late-notification` on CLI 2.1.270 ([`terra-lifecycle.md`](terra-lifecycle.md); [`topic-003/clips/waiting-turn-conformance-trap.md`](/private/tmp/gimbal-317-literature/corpus/topic-003/clips/waiting-turn-conformance-trap.md)), Claude ran background `sleep 9` and yielded an intermediate `ResultMessage`:
  - `subtype: "success"`, `terminal_reason: "completed"`, `stop_reason: "tool_use"`, `structured_output: {"kind": "waiting", "value": "INITIAL"}`
- Schema validity proves syntax, not task completion. Adapters unblocking on the first valid JSON payload exit prematurely.

### 4.3 Schema Immutability Boundary on CLI 2.1.270
- **Canonical Specs & Probes**: Go SDK `protocol.go:132-155`; [`terra-lifecycle.md`](terra-lifecycle.md) (`/private/tmp/gimbal-317-terra/control-schema/raw.jsonl`).
- Waiting and continuation of a single assignment do **not** require schema mutation; a fixed schema handles the entire assignment lifecycle (intermediate waits and late completion). Runtime schema immutability on CLI 2.1.270 affects only multi-turn workflows attempting to issue successive, differently typed `Generate[T]` calls over the same persistent process.
- Initial handshake `initialize(jsonSchema=A)` produced `{a: "FIRST"}`. Runtime `reinitialize(jsonSchema=B)` acknowledged success, but response remained constrained to Schema A: `{a: "{\"b\": \"SECOND\"}"}`. Runtime `reinitialize(jsonSchema: null)` acknowledged success, but requests remained constrained to Schema A: `{a: "CLEAR_THIRD"}`.
- *Finding*: There is **no verified runtime schema update or clear path on CLI 2.1.270**. A persistent process retains its launch schema.

### 4.4 Evaluation of Candidate Completion Rules

| Rule | Counterexample / Failure Mode | Evidence Source |
| :--- | :--- | :--- |
| **First `ResultMessage`** | Fails: Waiting turns emit `subtype: "success"`. | Incident 317; Sol probe. |
| **`terminal_reason == "completed"`** | Fails: Carried by waiting yields and completions; `background_requested`/`tool_deferred` (`messages.go:560-565`) identify specific yield paths, not all conversational waits. | Sol probe; `messages.go`. |
| **Valid `structured_output`** | Fails: Intermediate waiting yields emit schema-valid JSON. | Terra probe (`raw.jsonl:51`). |
| **No Active Tasks** | Fails: Persistent services hang indefinitely. | Sol probe (`natural-raw.jsonl`). |
| **All Tasks Terminal** | Fails: Hangs on services; model may omit registering dependencies. | Sol probe ([`sol-semantics.md`](sol-semantics.md)). |
| **`origin.kind == "task-notification"`** | Attribution only: Wakeup result may fail or continue waiting. | Sol probe (`raw.jsonl:85-158`). |
| **Monotonic `result_index`** | Tracks generation sequence within process; lacks task semantics. | Go SDK `messages.go:175`. |
| **`TaskOutput(block: true)`** | Effective for known finite tasks; fails on services; Current [tools reference](https://code.claude.com/docs/en/tools-reference) deprecates TaskOutput in favor of reading its output file; blocking worked in the probe. | probe `sdk-wait/raw-1.jsonl`. |
| **Model Declaration (`state: "completed"`)** | Mechanically parsable model claim, not verified external truth. Model may hallucinate completion. | Sol probe ([`sol-semantics.md`](sol-semantics.md)). |
| **Host Completion Tool** | Model declaration coupled with host validation; provides handshake, not semantic truth. | [`matrix.md`](/private/tmp/gimbal-317-literature/corpus/topic-003/clips/candidate-completion-mechanisms-matrix.md). |

---

## 5. Topic 004: Lifetimes, Resumption, and Data-Loss Boundaries

### 5.1 Five Architectural Lifetime Boundaries
- **Canonical Specs**: [`headless.md`](https://code.claude.com/docs/en/headless.md); Go SDK `transport.go`, `subprocess.go` (Local clip: [`topic-004/clips/clip-01-lifetime-boundaries-and-exit-teardown.md`](/private/tmp/gimbal-317-literature/corpus/topic-004/clips/clip-01-lifetime-boundaries-and-exit-teardown.md)).
1. *Model Token Generation*: Bounded by turn inference; idle during background waits.
2. *Native Event Stream*: Bounded by subprocess stdout; streams events continuously across turns and wakeups while connected.
3. *SDK Iteration*: Bounded by `Stream.Messages()` consumer. Exiting the loop stops host consumption; it does not itself close the native process.
4. *OS Stdio Pipes & Process*: Owned by transport. The pinned SDK allows up to 5s for process exit after stdin EOF. Top-level kill aborts session.
5. *Background Process Tree*: OS child processes. Main conversation tasks run asynchronously; top-level kill does not prove universal death of detached external jobs.

### 5.2 Resumed Session Wire Sequence: The Synthetic Orphan Double-Result
- **Canonical Captures**: Probe runs [`sdk-text/raw-2.jsonl`](/private/tmp/gimbal-317-investigation/sdk-text/raw-2.jsonl) and [`sdk-structured/raw-2.jsonl`](/private/tmp/gimbal-317-investigation/sdk-structured/raw-2.jsonl) (Local clip: [`topic-004/clips/clip-02-resumed-orphan-task-wire-sequence.md`](/private/tmp/gimbal-317-literature/corpus/topic-004/clips/clip-02-resumed-orphan-task-wire-sequence.md)).
- When resuming a session with pending background tasks via `--resume` in CLI 2.1.270 probes:
  1. CLI injects `system/task_notification`: `status: "stopped"`, summary: `"Background shell command didn't finish before the previous session ended"`.
  2. CLI emits synthetic `result_index: 0`: `subtype: "success"`, `origin: {"kind": "task-notification"}`, `num_turns: 0`. In text mode `result` is `""`; in structured mode `structured_output` is **omitted entirely** (not wire null).
  3. CLI processes user prompt, emitting `result_index: 1`: `subtype: "success"`, `num_turns: > 0`, containing prompt answer and valid structured output.

### 5.3 Data-Loss Boundaries: Live Streaming vs Transcript Replay

| Dimension | Live Stream (Active Process) | Resumption Replay (`--resume`) |
| :--- | :--- | :--- |
| **In-Flight Tasks** | Commands continue; exit events fire naturally. | Probes show tasks terminated; injected as `stopped` orphans. |
| **Event Stream** | Continuous across turns; subject to pipe drop. | Lossy: Only transcript messages survive; stream tokens lost. |
| **Prompt Attribution** | Notifications carry `origin.kind`; FIFO prompt correlation under races is unproven. | Double-Result: Emits empty `result_index: 0` before prompt. |
| **Output Files** | Intact: Live stdout/stderr on disk remains accessible. | Truncated: Flushed output kept; unwritten buffers lost. |
| **Host Overhead** | Requires managing persistent subprocess and reader. | Low idle footprint; high resumption replay latency. |

---

## 6. Topic 005: Empirical Precedents and Upstream Issues

### 6.1 Incident 317: Original Incident vs Controlled Probes
- **Original Incident Evidence**: Native transcript `/Users/tyler/.claude/projects/-private-tmp-scrabbler-drag-drop-eval/deee5fb6-61a3-4c41-b827-d5033fa663ae.jsonl` proves Opus yielded a waiting turn with `stop_reason: "end_turn"` (lines 326–334 in [`topic-005/clips/clip-transcript-waiting-turn-and-orphan.md`](/private/tmp/gimbal-317-literature/corpus/topic-005/clips/clip-transcript-waiting-turn-and-orphan.md)) and normalized records confirm Gimbal received an empty result on debrief.
- **Controlled Probes**: Side-by-side two-result captures (`result_index: 0` empty success followed by `result_index: 1`) were produced by controlled verification probes in `sdk-text/` and `sdk-structured/`, not from a retained wire tap of Incident 317.

### 6.2 Audited Upstream GitHub Issues (13 Closed Historical Reports)
All 13 issues from `anthropics/claude-code` are **closed historical reports** tied to past versions ([`topic-005/sources/source-01-upstream-issues.md`](/private/tmp/gimbal-317-literature/corpus/topic-005/sources/source-01-upstream-issues.md); [`sol-audit.md`](/private/tmp/gimbal-317-literature/verification/sol-audit.md)):
- **[#60142](https://github.com/anthropics/claude-code/issues/60142)** (CLI 2.1.92, Python SDK 0.1.56): Subagent notification arrived before subagent `ResultMessage`; inversion absent across later rounds; concerns subagent notifications, not host prompt queue interleaving.
- **[#60001](https://github.com/anthropics/claude-code/issues/60001)** (CLI 2.1.91): Missing notifications when $\ge 3$ agents dispatched; registry GC was unconfirmed hypothesis.
- **[#38805](https://github.com/anthropics/claude-code/issues/38805) & [#39028](https://github.com/anthropics/claude-code/issues/39028)** (CLI 2.1.83): Empty `result: ""` in stream-json; fixed in v2.1.84 by skipping trailing progress/attachments.
- **[#38651](https://github.com/anthropics/claude-code/issues/38651)** (CLI 2.1.83): `Stop` hooks causing empty results; resolved in v2.1.84.
- **[#40432](https://github.com/anthropics/claude-code/issues/40432)** (CLI 2.1.85): Interrupted stream returning empty success; fixed in v2.1.105 via retry-once.
- **[#45717](https://github.com/anthropics/claude-code/issues/45717)** (CLI 2.1.97): Bash timeout propagating `SIGTERM` (143) to parent; process-group signal explanation was commenter discussion.
- **Child Leaks ([#19433](https://github.com/anthropics/claude-code/issues/19433), [#18405](https://github.com/anthropics/claude-code/issues/18405), [#27959](https://github.com/anthropics/claude-code/issues/27959), [#51264](https://github.com/anthropics/claude-code/issues/51264))**: Heterogeneous reports; surviving `SIGTERM` was specific to `bun` runtimes (#51264); top-level CLI kill does not prove universal death of detached external jobs.
- **Pipe Stalls ([#25670](https://github.com/anthropics/claude-code/issues/25670), [#17248](https://github.com/anthropics/claude-code/issues/17248))**: Early stdout buffering in v2.1.3/v2.1.42; closed duplicates.

### 6.3 Critique of Public Integration Patterns
- **A. Env Disabling (`CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1`)**: Disables native background execution; it does not prove semantic completion or eliminate every reason to wait ([`topic-005/sources/source-04-sdk-evolution-and-patterns.md`](/private/tmp/gimbal-317-literature/corpus/topic-005/sources/source-04-sdk-evolution-and-patterns.md)).
- **B. Synchronous Blocking (`TaskOutput(block: true)`)**: Requires model compliance; fails on persistent services; current [tools reference](https://code.claude.com/docs/en/tools-reference) lists TaskOutput as deprecated in favor of Read on its output file. Blocking remains live-observed on the tested CLI.
- **C. Ralph Wiggum Loops**: Session-grain stop interceptor; cannot handle turn-level waits inside active sessions.
- **D. Session-Owned Stream with Demultiplexing**: Proposed approach. Session owns subprocess and continuous reader, routing wakeup results to waiting logical tasks.

---

## 7. Architectural Synthesis & Proposed Policies for Gimbal

### 7.1 Why the Current Gimbal Adapter Fails
1. **Per-Turn Teardown**: `RunTurn` creates and closes clients per turn ([`claude/claude.go`](../../../claude/claude.go)), closing the transport and killing the top-level process if it remains after the five-second grace; observed pending tasks stopped.
2. **First-Result Selection**: Consumes first `ResultMessage`, misattributing waiting yields or orphan results.
3. **Turn-Scoped Event Dispatching**: Event projection ([`session.go`](../../../session.go)) is active only during a caller turn; late events between turns have no listener.

### 7.2 Proposed Direction Requiring Live Validation
These points constitute a proposed hosting policy requiring live prototyping and testing in Gimbal, rather than an established provider contract:
1. **Session-Scoped Subprocess**: Elevate client/transport ownership from `RunTurn` to `Session`, keeping one subprocess open across the session lifecycle.
2. **Continuous Reader Goroutine**: Run one reader on `Stream.Messages()` that demultiplexes events:
   - Updates task status from `background_tasks_changed` and `task_notification`.
   - Routes `origin.kind == "task-notification"` results to the logical turn/`Generate` awaiting task completion, continuing that waiting turn until an associated terminal declaration arrives. Unsolicited orphan notifications are never handed to unrelated prompt callers.
   - Routes ordinary prompt results to active prompt callers.
3. **Explicit Completion Contract**: Enforce completion via explicit model declaration (e.g. `state: "waiting" | "completed"`) or host completion tool (`complete_task`), both requiring host validation.
4. **Service vs Dependency Declaration**: Register persistent background services so the reader does not mistake running services for uncompleted dependencies.

---

## 8. Source and investigation locations

The linked corpus files are synthesized extracts and clips, not complete original
sources or byte-for-byte wire captures. Exact native captures remain in
/private/tmp/gimbal-317-investigation/, /private/tmp/gimbal-317-sol/, and
/private/tmp/gimbal-317-terra/. The pinned SDK source is in the local Go module
cache at github.com/tylergannon/claude-agent-sdk-go@v1.1.1-0.20260912021749-9a4ffeca77cc.
The [combined decision brief](findings.md), [Sol report](sol-semantics.md), and
[Terra report](terra-lifecycle.md) distinguish observations from proposals.
