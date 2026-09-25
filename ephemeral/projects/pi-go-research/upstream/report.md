# Upstream Pi Behavioral Map

This document establishes a source-grounded map of the upstream TypeScript Pi implementation (`earendil-works/pi` @ `d6af72e1`). It isolates the exact semantics necessary for equivalent headless coding behavior in a native Go harness, bypassing UI and extension overhead.

## 1. Monorepo Architecture, Deltas & Headless Boundary

The upstream monorepo distributes logic across 12 packages. Direct participation in the headless coding path is strictly isolated to three packages:
- `@earendil-works/pi-ai`: Provider wire formats, stream parsing, and usage accounting.
- `@earendil-works/pi-agent-core`: Turn lifecycle, tool dispatch, and event streams.
- `@earendil-works/pi-coding-agent`: Session orchestration, compaction, and core coding tools.

### Version Deltas (v0.87.1 vs d6af72e1)
The pinned commit `d6af72e1` is 48 commits ahead of the `v0.87.1` release, introducing material differences for any RPC adapter:
- **Prompt Disposition Delta (`e473b5c`)**: RPC prompt and steer commands return `{ success: true, data: { disposition: "started" | "queued" | "handled" } }`, diverging from the documented `v0.87.1` bare `{ success: true }`.
- **Lazy Session Creation Delta (`ff72fab`)**: Session files (`.jsonl`) are now saved immediately upon receiving the first user message, rather than waiting for the first assistant response, enabling aborted initial turns to be resumed.

### Core vs. Optional Scope
Dropping the TypeScript extension runner (`jiti-loader`), UI (`pi-tui`), and server daemons (`pi-protocol`, `pi-server`) sacrifices:
- Loading third-party `.js`/`.ts` plugins, custom UI dialogs, and request header mutation hooks (`pi.on("input")`).
However, this omission incurs **zero penalty** to autonomous coding fidelity. All core capabilities (multi-turn loop, tools, compaction, and steering) are fully preserved.

## 2. Provider Protocols, Assembly, and Accounting

### HTTP Formatting
Outbound requests for OpenAI-compatible endpoints (`packages/ai/src/api/openai-completions.ts:L797-L870`) are modified:
- `stream_options`: Injected as `{ include_usage: true }` unless explicitly unsupported.
- `store`: Set to `false` when supported to opt-out of data retention.
- `max_completion_tokens`: Used by default over `max_tokens`.
- **Reasoning flags**: Dynamically nested according to vendor (e.g., `reasoning_effort` for OpenAI vs nested `{ thinking: ... }` for Zai) (`packages/ai/src/api/openai-completions.ts:L875-L975`).

### Streaming Assembly & Recovery
SSE chunks are parsed into `text_delta`, `thinking_delta`, and `toolcall_delta` (`openai-completions.ts:L553-L726`).
- **Retries**: Pre-flight HTTP 429/500s are retried with exponential backoff (`packages/ai/src/utils/provider-retry.ts:L1-L126`).
- **In-flight Errors**: If the stream fails after starting, there is **no byte-offset resume**. The chunk reader catches the error and terminates the stream (`stopReason = "error"`). `AgentSession` evaluates retryability and restarts the entire turn.

### Usage Accounting
Token consumption is tallied natively (`models.ts:L1187-L1207`). `cacheReadTokens` and `cacheWriteTokens` are extracted from `prompt_tokens_details`. Cost is tiered per million tokens, where 1-hour extended cache writes (`cacheWrite1h`) are billed at double the standard input rate.

## 3. Configuration Discovery, Models, and Authentication

### Settings Hierarchy & Dynamic Values
- **Discovery**: Global settings (`~/.pi/agent/settings.json`) are deep-merged with project settings (`<cwd>/.pi/settings.json`).
- **Project Trust**: If `projectTrusted === false`, project settings are strictly ignored to prevent malicious configuration injection (`config.ts`).
- **Concurrency**: `FileSettingsStorage` uses `proper-lockfile` with synchronous busy-wait retry loops.
- **Dynamic Values**: Values prefixed with `!cmd` are executed synchronously with a 10s timeout, caching stdout in a process-wide `commandResultCache`. Values prefixed with `$ENV` are interpolated against ambient variables.

### Model Discovery & The Selection Cascade
`models.json` is validated via TypeBox, freezing composed providers. Model selection follows a strict 5-step cascade in `findInitialModel`:
1. **Explicit Override**: CLI/RPC requested model (resolves ambiguities by selecting the authenticated one).
2. **Scoped Cycle**: First scoped model if `--models` is passed.
3. **Settings Default**: `defaultProvider`/`defaultModel` from settings (bypassed if unauthenticated).
4. **Provider Defaults**: Iterates 38+ hardcoded provider defaults until finding an authenticated provider.
5. **Fallback**: Selects the very first authenticated model in the catalog.

### Authentication Boundaries
`@earendil-works/pi-agent-core` is strictly auth-agnostic. `ModelRuntime` manages credentials:
- **`auth.json` Storage**: Created with `0o600` permissions, using `proper-lockfile` (30s stale window).
- **Precedence**: `RuntimeCredentials` (in-memory overlay) > `auth.json` > Ambient/Environment Variables.
- **Isolation Invariant**: Stored credentials strictly *own* the provider. If stored keys expire, upstream explicitly does **not** fall back to ambient keys, preventing silent cross-account billing leaks.
- **OAuth Refresh**: Proactive rotation using double-checked locking under a file lock if validity is < 5 minutes.

## 4. Agent Turn Lifecycle & Tool Scheduling

### Event Contracts and Turn Phases
State transitions follow a strict phased lifecycle (`packages/agent/src/agent-loop.ts` and `agent.ts`):
1. **Turn Start**: Emits `agent_start`, `turn_start`, and injects prompts (`message_start/end`).
2. **Generation**: Invokes `streamAssistantResponse`, emitting `message_update` deltas.
3. **Message Completion**: `message_end` commits the individual assistant message.
4. **Tool Execution**: `tool_execution_start/end` wrapper.
5. **Turn Settlement**: `turn_end` fires with assistant message and tool results. Evaluates steering or next turns.
6. **Loop Completion (`agent_end`)**: Inner loop drains.
7. **Idle (`agent_settled`)**: `AgentSession` finishes post-run recovery and compaction, entering an idle state to receive prompts (`agent-session.ts:L873-L894`).

### Tool Dispatch
Handled sequentially or in parallel (`executeToolCallsParallel`).
- **Error Feedback**: Exceptions during tool parsing or execution are **never thrown** to crash the process. They are converted into `createErrorToolResult(error.message)` (`agent-loop.ts:L764-L811`) so the LLM context receives the error and self-corrects.
- **Length Truncation**: If `stopReason === "length"`, `failToolCallsFromTruncatedMessage` marks all tool calls in the batch as failed with a warning, bypassing execution (`agent-loop.ts:L468-L500`).

## 5. Steering, Follow-ups, and Cancellation

### Queue Ordering
- **Steering Queue**: Evaluated and ingested into context *after* the current tool batch completes and *before* the next LLM inference turn (`agent-loop.ts:L294-L297`). Does not interrupt active tools.
- **Follow-up Queue**: Evaluated strictly in an outer loop only after the inner loop achieves idle state (`hasMoreToolCalls === false`).

### Mid-Turn Abort Lifecycles
`session.abort()` cascades an `AbortSignal`.
- **Process Teardown**: Detached `bash` child processes are killed via OS process groups (`syscall.Kill(-pgid, syscall.SIGKILL)` on Unix, `taskkill` on Windows) (`packages/coding-agent/src/utils/shell.ts:L214-L247`).
- **History Invariants**: Aborted generations are appended with `stopReason: "aborted"`. Orphaned tool calls synthesize `"Operation aborted"` results (`createLocalShellOperations`).
- **Model Replay**: During context replay (`transformMessages`), all `stopReason: "error"` or `"aborted"` messages are completely stripped, protecting the model from schema validation errors on broken tool blocks (`packages/ai/src/api/transform-messages.ts:L160-L235`).

## 6. Session Storage, Branching, and Compaction

### Serialization and Disk Layout
Sessions use JSONL (`${timestamp}_${uuid}.jsonl`). Line 1 is a `SessionHeader` (`version: 3`, `cwd`, `parentSession`). Subsequent lines (`SessionEntryBase`) form a DAG via `parentId` pointers (`packages/coding-agent/src/core/session-manager.ts`). The file is lazily opened (`wx`) only on the first user message.

### Tree Branching (`fork` vs `clone`)
Branching extracts a linear slice of history into a new file (`createBranchedSession`):
- `/clone`: Sets `position: "at"` and copies history up to the active `leafId`.
- `/fork`: Sets `position: "before"`. Targets a specific `userMsgId`'s `parentId`, slicing off all subsequent events, and extracts the prompt for prefilling (`agent-session-runtime.ts`).

### Context Overflow & Compaction
Detected post-turn (`_handlePostAgentRun`) or pre-prompt.
- **Threshold**: Compacts when `tokens > contextWindow - 16384` (`compaction/compaction.ts`). Or upon single-retry provider overflow errors (`isContextOverflow`).
- **Cut Point (`findCutPoint`)**: Searches backwards accumulating tokens (default 20,000 kept). Ensures it **never** slices between a tool call and its result.
- **Splicing and Resume**: Summarizes older turns with `<previous-summary>` and `<modified-files>` tags. A `CompactionEntry` is appended pointing to the active `leafId`, capturing a frozen snapshot of the `systemMessage`. On resume (`buildContextEntries`), older events before `firstKeptEntryId` are pruned, and the frozen system context prevents configuration drift.

## 7. Coding Tools, Instructions & Environment State

### Built-in Tool Contracts
- `read`: 1-indexed offset/limit. Gracefully handles macOS screenshot artifacts (`\u202F`, smart quotes) (`path-utils.ts`). Head-truncated at 50KB/2000 lines.
- `write`: Recursively creates directories.
- `edit`: 2-stage diff engine (`edit-diff.ts`). Enforces non-overlapping matches. Uses exact match then fuzzy NFKC fallback. `applyReplacementsPreservingUnchangedLines` protects unmodified bytes from losing formatting.
- `bash`: Spawns detached processes. Tail-truncated at 50KB/2000 lines with a spillover file (`pi-bash-*.log`).

### Instructions and Skills
- `AGENTS.md`: Recursively searches up the directory tree to the filesystem root, handling nested worktrees (`resource-loader.ts`).
- **Skills**: Merges `.pi/skills/` (project) over `~/.pi/agent/skills/` (user), formatted into the system prompt under `<available_skills>`.

### Process-Global State & Go Isolation Seams
A Go implementation running concurrent sessions must replace upstream's process-global reliance:
- `process.cwd()`: Avoid `os.Chdir()`. Use explicit `cmd.Dir` per session.
- `process.env`: Avoid `os.Setenv()`. Clone and inject `PI_*` variables into `cmd.Env`.
- OS Signals: Avoid global signal hooks. Use `context.WithCancel` for abort control.
- Child PID Tracking: Replace `trackedDetachedChildPids` global `Set` with a thread-safe, session-scoped process tracker.
- File Locks: `SessionManager` has zero locks. Concurrent append writes in Go require a keyed `sync.Mutex` on canonical file paths to avoid JSONL line interleaving.

## 8. Decisive Qualification Traces & Test Coverage

### Test Suites for Required Traces
To establish parity, the Go runtime must pass the following traces, backed by authoritative tests:
1. **Standard Turn & Tool Success**: Emits ordered event streams (`agent_start` -> `tool_execution_start/end` -> `agent_settled`) (`packages/agent/test/agent-loop.test.ts`).
2. **Tool Failure Feedback**: Exception produces `isError: true` result. Turn continues with the model observing the error (`packages/agent/test/agent-loop.test.ts`).
3. **Mid-Turn Cancellation & Subprocess Teardown**: Context cancellation properly executes negative PGID `SIGKILL`, clears mutation queues, and prevents process leaks (Regression `#8935`).
4. **Queued Steering**: Mid-tool queue update (`landed = true`) is injected prior to the next model invocation without interrupting tools (`packages/agent/test/agent-loop.test.ts`).
5. **Session Fork Independence**: Forking an aborted turn branch point creates a child session clean of the aborted node (Regression `#8724`).
6. **Compaction Replay**: Token exhaustion triggers a valid cut point that doesn't split tool calls, generates `<summary>` XML, and cleanly prunes old entries on resume (`packages/agent/test/harness/compaction.test.ts`).

### Unresolved Evidence Gaps & Missing Test Coverage
Upstream test suites are strictly single-process and single-session, leaving material evidence gaps for a concurrent native Go port:
- **Multi-session Interference**: Zero tests cover concurrent session execution, leaving global cache contamination (like `commandResultCache`) and file lock races unverified by upstream CI.
- **Interactive Bash Deadlocks**: Interactive prompts (e.g., `sudo`, `read -p`) that hang the agent loop indefinitely lack any automated test coverage.
- **Synchronous File Locking**: `FileSettingsStorage` synchronous busy-wait lock implementations (`lockfile.lockSync`) are not tested under heavy process contention, risking node thread-pool starvation. An embedded Go harness must rely on native non-blocking OS locks (`flock`) instead.
- **Project Trust Sandboxing**: Gimbal's execution contexts will require explicit programmatic definitions of `projectTrusted` true/false, rather than upstream's default `ask` terminal prompt.
