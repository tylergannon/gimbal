# Native Pi Headless Coding Engine in Gimbal: Architecture and Fan-Out Port Plan

Draft author: Gemini
Target file: [`ephemeral/projects/pi-go-research/planning/GEMINI-DRAFT.md`](file:///Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/planning/GEMINI-DRAFT.md)
Reference baseline: [`INTENT.md`](file:///Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/planning/INTENT.md)
Primary upstream reference: [`earendil-works/pi`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi) at `d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31`
Primary Go translation reference: [`sky-valley/pi`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi) at `e6b56e7223cfab707b21a0a19e0a9a2417ac960a`
Secondary Go references: [`amit-timalsina/pi-agent-go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-agent-go) at `bdf20cd` and [`amit-timalsina/pi-llm-go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-llm-go) at `bfd55c8`
Target integration API: Gimbal root contracts in [`harness.go`](file:///Users/tyler/src/gimbal-pi-research/harness.go), [`session.go`](file:///Users/tyler/src/gimbal-pi-research/session.go), [`events.go`](file:///Users/tyler/src/gimbal-pi-research/events.go), [`usage.go`](file:///Users/tyler/src/gimbal-pi-research/usage.go), and [`group.go`](file:///Users/tyler/src/gimbal-pi-research/group.go)

---

## 1. Overview and Use Cases

Gimbal coordinates agent workflows as ordinary Go, providing live visibility, supervisory steering, and typed schema validation. Current external coding harnesses (such as OpenCode and the in-progress issue [#386](file:///Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/issue-386.md) subprocess adapter) execute turns via separate CLI processes communicating over HTTP or line-delimited JSONL pipes. External child harnesses introduce substantial operational overhead: multi-hundred-megabyte Node/Bun memory footprints per session, startup latency, IPC serialization boundaries, brittle OS process-group signal routing, and indirect session lifecycles.

This plan details the architecture, package decomposition, contract boundaries, and execution workflow for an **owned, semantic port of Pi's headless coding engine directly into Gimbal's process**. The agent loop, provider client, session DAG persistence, context auto-compaction, settings discovery, and turn steering execute entirely within Gimbal's Go runtime. Tool subprocesses (such as `bash` commands) remain standard isolated child processes.

### Key Use Cases

1. **In-Process Coding Sessions**: Workflows instantiate lightweight coding sessions with zero Node/Bun process lag, running rapid read-edit-test cycles against local worktrees.
2. **High-Concurrency Workflows**: Parallel workflows using [`Group`](file:///Users/tyler/src/gimbal-pi-research/group.go#L34-L45) (e.g. multi-agent bake-offs, critique rounds, and parallel module implementation) run dozens of simultaneous sessions without process table exhaustion.
3. **Zero-IPC Branching and Forking**: Gimbal's [`Session.Fork`](file:///Users/tyler/src/gimbal-pi-research/session.go#L612-L634) operates directly on in-memory conversation trees and local JSONL append-only DAG files, eliminating process state synchronization.
4. **Native Steering and Deterministic Teardown**: Mid-turn steering via [`Session.Steer`](file:///Users/tyler/src/gimbal-pi-research/session.go#L552-L596) and turn cancellations use Go channels and synchronized mailboxes, terminating in-flight HTTP streams and process groups cleanly without IPC race conditions.
5. **Unified Telemetry and Validation**: The native engine maps turn progress, streaming deltas, tool starts/ends, and token usage directly into Gimbal's [`AgentEvent`](file:///Users/tyler/src/gimbal-pi-research/events.go#L19-L26) stream, routing structured schema generation through Gimbal's existing validation and re-ask loop.

---

## 2. Scope and Boundaries

### Explicitly In Scope (Headless Pi Engine)

- **Provider Wire Protocol**: Direct HTTP client streaming supporting OpenAI-compatible chat completions (`POST /v1/chat/completions` with SSE `stream: true`), streaming delta parsing, vendor-specific reasoning token extraction (`reasoning_content`, `<think>`), prompt cache accounting (cache read, cache write), and exponential backoff retry.
- **Agent Execution Loop**: The turn state machine implementing the transitions: `message_start` &rarr; streaming deltas &rarr; `tool_call_start` &rarr; tool execution &rarr; `tool_call_end` &rarr; `message_end` &rarr; `agent_end` &rarr; `agent_settled`. Handles batch tool scheduling, token limit truncation (`stopReason: "length"`), and turn continuation.
- **Built-in Coding Tools**:
  - `read`: File content inspection with 1-indexed line numbers, line offset slicing, and strict truncation thresholds (2,000 lines / 50 KB).
  - `write`: Atomic file creation and full overwriting with directory creation.
  - `edit`: Chunk replacement utilizing fuzzy whitespace/indentation matching, uniqueness validation, and line-structure preservation.
  - `bash`: Shell execution with process-group isolation (`syscall.Setpgid`), configurable timeouts (default 10s), detached process cleanup (`SIGKILL` to negative PID), and continuous stdout/stderr capture past exit.
  - Read-only search: `find` (glob search), `grep` (regex search), and `ls` (directory listing).
- **Session DAG Persistence & Branching**: Append-only JSONL session file management (`type: "session"`, `session_start`, `message`, `custom_message`, `compaction_summary`), tree traversal via `parentEntryId` pointers, active leaf tracking, and distinct `/clone` (active leaf branch) vs `/fork` (historical message branch) operations.
- **Context Window Management & Auto-Compaction**: Token count tracking, provider overflow error classification, atomic cut-point detection (preventing the separation of a tool call from its tool result), structured XML summary generation (`<summary>`, `<files_touched>`), and transcript history splicing.
- **Settings Hierarchy & Dynamic Values**: Deep-merging global (`~/.pi/agent/settings.json`) and repository (`<workdir>/.pi/settings.json`) settings, project trust security gating (`projectTrusted`), and resolving dynamic value expressions (`!cmd` execution and `$ENV` expansions).
- **Instructions & Skills Discovery**: Ascending directory traversal for repository instructions (`AGENTS.override.md` &gt; `AGENTS.md` &gt; `CLAUDE.md`) and discovery of workspace skills (`.agents/skills/*/SKILL.md`, `.pi/skills/*/SKILL.md`, `~/.pi/agent/skills/*/SKILL.md`) formatted into `<available_skills>` prompt blocks.
- **Gimbal HarnessAdapter Seam**: Implementation of [`HarnessAdapter`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L14-L40), projecting events into [`AgentEvent`](file:///Users/tyler/src/gimbal-pi-research/events.go#L19-L26) and mapping token/dollar costs to [`TurnResult`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L53-L63).

### Explicitly Excluded / Deliberately Deferred

- **Terminal UI (TUI)**: The interactive curses/ink terminal interface in `packages/tui` is excluded. Gimbal provides its own web page interface.
- **TypeScript Extension Engine**: The dynamic runtime extension loader (`jiti-loader`, dynamic JS execution, custom TS extension lifecycle hooks) is excluded. Coding tools in Gimbal are compiled Go code.
- **Experimental / Unreleased Storage**: Upstream unreleased packages (`packages/chord`, `packages/durable`, and "Pico5" experimental storage) are excluded. Persistence remains strictly on the proven JSONL DAG format.
- **Subprocess Daemon / RPC Layer**: The JSONL subprocess transport (`--mode rpc`) in `packages/server` is excluded; communication is direct in-process Go function calls.
- **Non-OpenAI Provider Wire Implementations**: Direct native wire protocols for Anthropic (`/v1/messages`) and Google Vertex/Gemini (`/v1beta/models/...`) are deferred. The first milestone routes through OpenAI-compatible endpoints used by Diffusion Router and open-weight models.
- **Desktop Session Synchronization**: Bidirectional active sync with external desktop Pi sessions is deferred. The on-disk JSONL format is compatible, but live syncing across runtimes is not required for workflow operation.

### Milestone vs. Parity & Namespace Isolation

- **Initial Live Milestone**: A vertical slice targeting model `diffusion/deepseek-4.1-flash` via Diffusion Router (`https://router.diffusion.io/v1`).
- **Namespace Collision Defense**: In-progress issue [#386](file:///Users/tyler/src/gimbal-pi-research/ephemeral/projects/pi-go-research/issue-386.md) reserves the `pi/` model namespace (e.g. `pi/diffusion/deepseek-4.1-flash`) for its external CLI subprocess adapter. To prevent merge collisions, the native Go engine will use the explicit temporary namespace `pigo/` (e.g., `pigo/diffusion/deepseek-4.1-flash`) during development.
- **Rebase & Cutover Ownership**: A dedicated cutover task will rebase onto `main` after issue #386 lands, run differential comparisons between the subprocess adapter and native engine, and switch the canonical `pi/` routing to the native engine while preserving other harnesses.

---

## 3. Package and Assignment Table

To prevent import cycles and enable parallel implementation, the codebase is partitioned into cohesive internal Go packages under `internal/pi/`. Each package has exclusive directory ownership.

The classification categories are:
- **Ported**: Direct idiomatic Go translation of upstream TypeScript reference logic.
- **Reused/Adapted**: Adapted from existing Go donors ([`sky-valley/pi`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi), [`amit-timalsina/pi-agent-go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-agent-go), [`amit-timalsina/pi-llm-go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-llm-go)) with bugs corrected, reflection removed, and contracts aligned.
- **Provided by Gimbal**: Handled natively by Gimbal runtime contracts ([`harness.go`](file:///Users/tyler/src/gimbal-pi-research/harness.go), [`session.go`](file:///Users/tyler/src/gimbal-pi-research/session.go), [`events.go`](file:///Users/tyler/src/gimbal-pi-research/events.go), [`command.go`](file:///Users/tyler/src/gimbal-pi-research/command.go)).
- **Deferred**: Postponed beyond the initial native milestone.

### Package & Assignment Matrix

| Package & Directory | Assigned Scope & Responsibilities | Classification | Primary Upstream Reference | Go Donor Reference | Key Test Target |
|---|---|---|---|---|---|
| `internal/pi/contract` | Core types, interfaces, zero-reflection tool schemas, message blocks, event variants, provider interfaces, deep-copy methods. | **Ported** | [`packages/agent/src/types.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/agent/src/types.ts)<br>[`packages/ai/src/types/`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/ai/src/types) | [`sky-valley/agent/types.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/agent/types.go)<br>[`pi-agent-go/event.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-agent-go/event.go) | `contract_test.go` (serialization, deep-copy immutability) |
| `internal/pi/provider` | HTTP client, OpenAI-compatible streaming (`stream: true`), SSE event parser, reasoning token extraction (`reasoning_content`, `<think>`), backoff retry. | **Reused/Adapted** | [`packages/ai/src/api/openai-completions.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/ai/src/api/openai-completions.ts)<br>[`packages/ai/src/stream.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/ai/src/stream.ts) | [`sky-valley/ai/providers/openai.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/ai/providers/openai.go)<br>[`pi-llm-go/providers/openai/openai.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-llm-go/providers/openai/openai.go) | `provider_test.go` (mock SSE fixtures, chunk boundaries, EOF without `[DONE]`) |
| `internal/pi/models` | Model definitions, pricing arithmetic (input, output, cache-read, cache-write, reasoning), context bounds, `auth.json` loading (`0o600`), in-memory overrides. | **Reused/Adapted** | [`packages/coding-agent/src/core/model-resolver.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/model-resolver.ts)<br>[`packages/coding-agent/src/core/auth/resolve.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/auth/resolve.ts) | [`sky-valley/ai/models_runtime.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/ai/models_runtime.go)<br>[`sky-valley/ai/auth_resolve.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/ai/auth_resolve.go) | `models_test.go` (cost math, auth precedence, no ambient fallback) |
| `internal/pi/tools/edit` | Fuzzy replacement engine, whitespace/indentation preservation, unique match verification, multi-line diff matching. | **Reused/Adapted** | [`packages/coding-agent/src/core/tools/edit.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/tools/edit.ts)<br>[`packages/coding-agent/src/core/tools/edit-diff.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/tools/edit-diff.ts) | [`sky-valley/coding/editmatch.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/editmatch.go) | `editmatch_test.go` (ambiguous match rejection, whitespace tolerance) |
| `internal/pi/tools/fs` | Built-in file tools: `read` (line slicing, 2k line / 50KB limits), `write` (atomic write), `find` (glob search), `grep` (regex search), `ls`. | **Reused/Adapted** | [`packages/coding-agent/src/core/tools/read.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/tools/read.ts)<br>[`packages/coding-agent/src/core/tools/write.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/tools/write.ts) | [`sky-valley/coding/tools.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/tools.go) | `fs_tools_test.go` (bounds truncation, path scoping outside root) |
| `internal/pi/tools/bash` | Shell execution tool, process-group isolation (`setpgid`), negative PID `SIGKILL` on cancellation/timeout, non-racing output capture. | **Reused/Adapted** | [`packages/coding-agent/src/core/tools/bash.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/tools/bash.ts)<br>[`packages/coding-agent/src/utils/shell.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/utils/shell.ts) | [`sky-valley/coding/execenv.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/execenv.go)<br>[`sky-valley/coding/proc_unix.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/proc_unix.go) | `bash_test.go` (late stdout flushes, subprocess tree teardown, timeout) |
| `internal/pi/session` | Append-only JSONL session DAG, session header, entry IDs, parent pointers, active branch resolution, clone vs fork mechanics, in-memory path mutex. | **Reused/Adapted** | [`packages/coding-agent/src/core/session-manager.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/session-manager.ts) | [`sky-valley/coding/session_store.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/session_store.go)<br>[`sky-valley/coding/session_tree.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/session_tree.go) | `session_test.go` (DAG traversal, concurrent writes, clone vs fork) |
| `internal/pi/resources` | Settings discovery and deep merge, `projectTrusted` security gating, dynamic value resolution (`!cmd`, `$ENV`), ancestor `AGENTS.md` and `SKILL.md` loading. | **Reused/Adapted** | [`packages/coding-agent/src/core/settings-manager.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/settings-manager.ts)<br>[`packages/coding-agent/src/core/resolve-config-value.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/resolve-config-value.ts)<br>[`packages/coding-agent/src/core/resource-loader.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/resource-loader.ts) | [`sky-valley/coding/resources.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/resources.go)<br>[`sky-valley/coding/resolve.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/resolve.go) | `resources_test.go` (isolated HOME skills, trust gating, dynamic timeout) |
| `internal/pi/agent` | Turn state machine (`agent-loop.ts`), tool scheduler, steering mailbox, abort cascade, history replay invariants, stop reason truncation. | **Reused/Adapted** | [`packages/agent/src/agent-loop.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/agent/src/agent-loop.ts)<br>[`packages/agent/src/agent.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/agent/src/agent.ts) | [`sky-valley/agent/loop.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/agent/loop.go)<br>[`pi-agent-go/agent.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-agent-go/agent.go) | `agent_loop_test.go` (event sequence, steering evaluation, abort cleanup) |
| `internal/pi/compaction` | Context overflow detection, token threshold heuristics, atomic tool call/result cut-point calculation, summary prompt generation, history splicing. | **Reused/Adapted** | [`packages/coding-agent/src/core/compaction/compaction.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/compaction/compaction.ts)<br>[`packages/ai/src/utils/overflow.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/ai/src/utils/overflow.ts) | [`sky-valley/coding/compaction.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/compaction.go) | `compaction_test.go` (cut-point pair atomicity, summary XML parsing, resume) |
| `internal/pi/engine` | Session coordinator (`AgentSession`), system prompt assembly, skills injection, file mutation queue, message conversion, event dispatch. | **Ported** | [`packages/coding-agent/src/core/agent-session.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/agent-session.ts)<br>[`packages/coding-agent/src/core/sdk.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/sdk.ts) | [`sky-valley/coding/session.go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi/coding/session.go) | `engine_test.go` (in-memory turn, skills assembly) |
| `internal/pi/adapter` | Gimbal [`HarnessAdapter`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L14-L40) implementation, event projection into [`AgentEvent`](file:///Users/tyler/src/gimbal-pi-research/events.go#L19-L26), usage aggregation into [`TurnResult`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L53-L63), schema re-ask bridge. | **Ported** | [`packages/coding-agent/src/core/agent-session.ts`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi/packages/coding-agent/src/core/agent-session.ts) | Precedent: [`opencode/adapter.go`](file:///Users/tyler/src/gimbal-pi-research/opencode/adapter.go) | `adapter_test.go` (Gimbal lifecycle, event projection, fork isolation) |
| `internal/binding` & `modelalias` | Model route binding for `pigo/diffusion/...`, role binding wiring, CLI help registration, and clean rebase ownership. | **Provided by Gimbal** | N/A | Existing Gimbal routing in [`internal/binding/binding.go`](file:///Users/tyler/src/gimbal-pi-research/internal/binding/binding.go) | `binding_test.go` (role binding, model resolution) |

---

## 4. Shared Contracts (Pre-Fan-Out Foundation)

Before launching parallel workers, the core contracts in `internal/pi/contract` must be frozen. This allows workers in Wave 1 and Wave 2 to compile and run tests against real types without circular imports.

### Contract 1: Messages and Deep-Copy Invariants

To eliminate the data-race and mutation bug identified in candidate inspection (where shallow-copying messages in `Snapshot()` causes forks to share slice backing stores):

```go
package contract

import (
	"encoding/json"
	"slices"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ContentBlockType string

const (
	ContentText       ContentBlockType = "text"
	ContentThinking   ContentBlockType = "thinking"
	ContentToolCall   ContentBlockType = "tool_call"
	ContentToolResult ContentBlockType = "tool_result"
)

type ContentBlock struct {
	Type         ContentBlockType `json:"type"`
	Text         string           `json:"text,omitempty"`
	Thinking     string           `json:"thinking,omitempty"`
	Signature    string           `json:"signature,omitempty"`
	ToolCallID   string           `json:"id,omitempty"`
	ToolName     string           `json:"name,omitempty"`
	ToolArgs     json.RawMessage  `json:"arguments,omitempty"`
	ToolResultID string           `json:"tool_call_id,omitempty"`
	Content      string           `json:"content,omitempty"`
	IsError      bool             `json:"is_error,omitempty"`
}

func (b ContentBlock) Clone() ContentBlock {
	cloned := b
	if len(b.ToolArgs) > 0 {
		cloned.ToolArgs = slices.Clone(b.ToolArgs)
	}
	return cloned
}

type Message struct {
	ID        string         `json:"id"`
	Role      Role           `json:"role"`
	Content   []ContentBlock `json:"content"`
	Timestamp int64          `json:"timestamp"`
}

// Clone guarantees complete deep-copy isolation for fork branches.
func (m Message) Clone() Message {
	cloned := m
	if len(m.Content) > 0 {
		cloned.Content = make([]ContentBlock, len(m.Content))
		for i, block := range m.Content {
			cloned.Content[i] = block.Clone()
		}
	}
	return cloned
}
```

### Contract 2: Zero-Reflection Tool Interface

Gimbal strictly forbids runtime reflection for tool definitions and dynamic call-site discovery. Tools consume and produce raw JSON:

```go
package contract

import (
	"context"
	"encoding/json"
)

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"parameters"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, toolCallID string, rawArgs json.RawMessage) (ToolResult, error)
}
```

### Contract 3: Streaming Provider Interface and Usage

```go
package contract

import (
	"context"
)

type TokenUsage struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	ReasoningTokens  int     `json:"reasoning_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

type StreamChunk struct {
	DeltaText      string
	DeltaThinking  string
	ToolCallStarts []ContentBlock
	ToolCallDeltas map[string]string // toolCallID -> argument fragment
	Usage          *TokenUsage
	StopReason     string
	Error          error
}

type Provider interface {
	Stream(ctx context.Context, model string, messages []Message, tools []ToolDefinition) (<-chan StreamChunk, error)
}
```

### Contract 4: Turn Events & Lifecycle Boundaries

```go
package contract

type EventType string

const (
	EventMessageStart EventType = "message_start"
	EventContentDelta EventType = "content_delta"
	EventToolStart    EventType = "tool_call_start"
	EventToolEnd      EventType = "tool_call_end"
	EventMessageEnd   EventType = "message_end"
	EventAgentEnd     EventType = "agent_end"
	EventAgentSettled EventType = "agent_settled"
)

type TurnEvent struct {
	Type       EventType    `json:"type"`
	MessageID  string       `json:"message_id,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	Block      ContentBlock `json:"block,omitempty"`
	Usage      *TokenUsage  `json:"usage,omitempty"`
	Error      error        `json:"-"`
}
```

---

## 5. Dependency Waves and Concurrency

A flat claim of fifteen independent parallel tasks is unrealistic due to topological coupling. Concurrency is organized into 6 staged dependency waves:

```
Wave 0: Foundation Contracts Freeze
  └─ [W0-1] internal/pi/contract
       │
       ├──────────────────────────────────────┬──────────────────────────────────┐
       ▼                                      ▼                                  ▼
Wave 1: Subsystem Implementations (Parallel Fan-Out, Concurrency = 7)
  ├─ [W1-1] provider (OpenAI SSE)       ├─ [W1-4] tools/fs (read/write/ls) ├─ [W1-6] session (JSONL DAG)
  ├─ [W1-2] models (auth/cost)          ├─ [W1-5] tools/bash (procgroup)   └─ [W1-7] resources (skills/settings)
  └─ [W1-3] tools/edit (fuzzy diff)
       │                                      │                                  │
       └──────────────────────────────────────┼──────────────────────────────────┘
                                              ▼
Wave 2: Core Loop & Intelligence (Concurrency = 2)
  ├─ [W2-1] internal/pi/agent (Turn loop, steer, abort)
  └─ [W2-2] internal/pi/compaction (Cut-point math, auto-summary)
       │
       ▼
Wave 3: Session Assembly (Sequential Integration)
  └─ [W3-1] internal/pi/engine (AgentSession coordinator)
       │
       ▼
Wave 4: Gimbal Integration & Routing (Concurrency = 2)
  ├─ [W4-1] internal/pi/adapter (HarnessAdapter, event projection)
  └─ [W4-2] internal/binding & modelalias (Wiring, namespace isolation)
       │
       ▼
Wave 5: Qualification & Acceptance (Sequential Verification)
  ├─ [W5-1] Deterministic differential test suite (no-key fixtures)
  └─ [W5-2] Live Router qualification (deepseek-4.1-flash milestone)
```

### Detailed Wave Progression

1. **Wave 0 (Contract Baseline)**: Commit `internal/pi/contract` definitions. Zero downstream code starts until tests in `contract` pass.
2. **Wave 1 (Subsystems Parallel Fan-Out - Concurrency 7)**:
   - 7 independent tasks executing in isolated worktrees:
     - `W1-1: provider`: Stream parsing, SSE reader, backoff retry.
     - `W1-2: models`: `models.json` catalog, `auth.json` loading, pricing arithmetic.
     - `W1-3: tools/edit`: Fuzzy diff engine, line preservation.
     - `W1-4: tools/fs`: `read`, `write`, `find`, `grep`, `ls` implementations.
     - `W1-5: tools/bash`: Shell execution, process-group isolation, non-blocking pipe drains.
     - `W1-6: session`: JSONL DAG writer/reader, fork/clone branch traversal.
     - `W1-7: resources`: Settings hierarchy, dynamic value resolution, skills discovery.
   - *Gate*: Each task must pass local package tests before wave integration.
3. **Wave 2 (Loop & Compaction - Concurrency 2)**:
   - `W2-1: agent`: Assembles `contract.Provider` and `contract.Tool` into the state machine loop, managing steering and abort signals.
   - `W2-2: compaction`: Implements token overflow classification, atomic cut-point detection, and transcript truncation.
   - *Gate*: Loop passes cancellation tests; compaction preserves tool-call pair atomicity.
4. **Wave 3 (Engine Coordinator - Single Integrator)**:
   - `W3-1: engine`: Combines loop, persistence, resources, tools, and compaction into `AgentSession`. Serial integration against the unified Wave 1 & 2 baseline.
5. **Wave 4 (Gimbal Integration - Concurrency 2)**:
   - `W4-1: adapter`: Implements [`HarnessAdapter`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L14-L40), projecting events to Gimbal and managing turn lifecycles.
   - `W4-2: routing`: Adds `pigo/` prefix routing in [`internal/binding`](file:///Users/tyler/src/gimbal-pi-research/internal/binding/binding.go) and [`internal/modelalias`](file:///Users/tyler/src/gimbal-pi-research/internal/modelalias/modelalias.go).
6. **Wave 5 (Qualification & Verification)**:
   - `W5-1`: Differential test suite with recorded mock fixtures.
   - `W5-2`: Live milestone execution against Diffusion Router.

---

## 6. Dedicated Gimbal Fan-Out Workflow Sketch

Per Tyler's directive, the fan-out is executed by a dedicated Gimbal workflow. The workflow is written in ordinary Go using existing primitives: [`Group`](file:///Users/tyler/src/gimbal-pi-research/group.go), [`Iterate`](file:///Users/tyler/src/gimbal-pi-research/iterate.go), [`Session.Generate`](file:///Users/tyler/src/gimbal-pi-research/session.go#L87-L95), [`RunCommand`](file:///Users/tyler/src/gimbal-pi-research/command.go#L110-L117), and [`Check`](file:///Users/tyler/src/gimbal-pi-research/command.go#L132-L141).

### Architectural Rules for the Fan-Out Workflow

1. **Isolation by Git Worktree**: Each worker executes inside a dedicated temporary git worktree outside the primary tree, preventing lock contention on `.git/index`.
2. **Strict Ownership Boundaries**: A worker owns *only* its designated directory (e.g. `internal/pi/tools/edit/`). Workers are prohibited from touching `go.mod`, `go.sum`, `internal/pi/contract/`, or generated files.
3. **Serialized Integration Baseline**: Workers branch from a known wave baseline commit. Completed worktree branches are merged sequentially into the integration baseline by the workflow coordinator.
4. **Scope Coaching**: Worker sessions are paired with an architectural critique supervisor (`RoleArchitecturalCritique`) using [`WithSupervisor`](file:///Users/tyler/src/gimbal-pi-research/supervise.go) to actively steer against gold-plating or unrequested abstractions.
5. **Bounded Revision**: If package compilation or `go test -race` checks fail, the worker receives feedback and is allowed a bounded number of fix attempts (maximum 3) before failing the task.
6. **Deterministic Failure & Cancellation**: Using [`Group`](file:///Users/tyler/src/gimbal-pi-research/group.go), if any worker fails unrecoverably, the enclosing group context is cancelled, interrupting sibling workers cleanly. Wait joins all goroutines before exiting.
7. **No Circular Bootstrapping**: The workflow runs on established stable harnesses (e.g., Claude Code, Codex, or Gemini). It never attempts to use the unbuilt Pi engine to construct itself.

### Go Implementation Sketch

```go
package pigo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal"
)

type WorkerTask struct {
	Name        string `json:"name"`
	PackagePath string `json:"package_path"` // e.g. "internal/pi/tools/edit"
	BriefFile   string `json:"brief_file"`   // path to local markdown brief
	TestCommand string `json:"test_command"` // e.g. "go test -race ./internal/pi/tools/edit/..."
}

type Wave struct {
	Name  string       `json:"name"`
	Tasks []WorkerTask `json:"tasks"`
}

type FanOutPlan struct {
	Waves []Wave `json:"waves"`
}

// RunPortWorkflow coordinates the wave-based multi-worker port implementation.
func RunPortWorkflow(ctx context.Context, env gimbal.Env, planFile string) error {
	raw, err := os.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}
	var plan FanOutPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return fmt.Errorf("decode plan file: %w", err)
	}

	gimbal.Set(ctx, "port repository", env.WorkDir)
	gimbal.Set(ctx, "total waves", fmt.Sprint(len(plan.Waves)))

	// Iterate through each wave sequentially
	for waveCtx, wave := range gimbal.Iterate(ctx, "wave", plan.Waves) {
		gimbal.Set(waveCtx, "current wave", wave.Name)

		// Concurrent fan-out within the wave using Gimbal Group
		grp := gimbal.Group(waveCtx, "wave-workers")
		for _, task := range wave.Tasks {
			t := task
			grp.Go("worker", func(workerCtx context.Context) error {
				return executeWorkerTask(workerCtx, env.WorkDir, wave.Name, t)
			})
		}

		// Wait joins all goroutines and returns the first error encountered
		if err := grp.Wait(); err != nil {
			return fmt.Errorf("wave %s failed: %w", wave.Name, err)
		}

		// Sequential integration step: merge worker branches into wave baseline
		if err := integrateWave(waveCtx, env.WorkDir, wave); err != nil {
			return fmt.Errorf("integrate wave %s: %w", wave.Name, err)
		}
	}

	return nil
}

func executeWorkerTask(ctx context.Context, repoRoot, waveName string, task WorkerTask) error {
	branchName := fmt.Sprintf("pigo/%s/%s", waveName, task.Name)
	worktreeDir := filepath.Join(repoRoot, ".worktrees", task.Name)

	// 1. Setup isolated git worktree
	if exitCode, out, stderr, err := gimbal.RunCommand(ctx, "worktree-add", repoRoot,
		"git", "worktree", "add", "-b", branchName, worktreeDir, "HEAD"); exitCode != 0 || err != nil {
		return fmt.Errorf("create worktree for %s: %s %w", task.Name, stderr+out, err)
	}
	defer func() {
		_, _, _, _ = gimbal.RunCommand(context.Background(), "worktree-cleanup", repoRoot,
			"git", "worktree", "remove", "--force", worktreeDir)
	}()

	gimbal.Set(ctx, "task name", task.Name)
	gimbal.Set(ctx, "package path", task.PackagePath)

	workerSession := gimbal.NewSession(ctx, gimbal.RoleSprintPlanning, worktreeDir)
	coachSession := gimbal.NewSession(ctx, gimbal.RoleArchitecturalCritique, worktreeDir)

	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		gimbal.Set(ctx, "attempt", fmt.Sprint(attempt))

		// 2. Generate task implementation with scope coaching
		workerPrompt := fmt.Sprintf(
			"Implement the assigned package in %s per the requirements in %s.\n"+
				"Touch ONLY files inside %s. Do NOT modify go.mod, go.sum, or contracts.\n"+
				"Write complete code and unit tests beside implementation.",
			task.PackagePath, task.BriefFile, task.PackagePath,
		)
		_, err := workerSession.Generate[gimbal.Text](ctx, workerPrompt,
			gimbal.WithSupervisor(coachSession, scopeCoachPrompt))
		if err != nil {
			return fmt.Errorf("worker turn error on attempt %d: %w", attempt, err)
		}

		// 3. Run validation check inside the worktree
		testKey := fmt.Sprintf("check.attempt.%d", attempt)
		if err := gimbal.Check(ctx, testKey, worktreeDir, "sh", "-lc", task.TestCommand); err != nil {
			if attempt == maxAttempts {
				return fmt.Errorf("worker %s tests failed after %d attempts: %w", task.Name, maxAttempts, err)
			}
			continue
		}

		// 4. Independent review validation
		validator := gimbal.NewSession(ctx, gimbal.RoleQAOrchestration, worktreeDir)
		assessment, err := validator.Generate[WorkerAssessment](ctx, validatePrompt)
		if err == nil && assessment.Passed {
			_, _, _, _ = gimbal.RunCommand(ctx, "git-add", worktreeDir, "git", "add", task.PackagePath)
			_, _, _, _ = gimbal.RunCommand(ctx, "git-commit", worktreeDir, "git", "commit", "-m", fmt.Sprintf("implement %s", task.Name))
			return nil
		}

		if attempt == maxAttempts {
			return fmt.Errorf("worker %s failed independent QA after %d attempts", task.Name, maxAttempts)
		}
	}

	return fmt.Errorf("worker %s exhausted attempts", task.Name)
}

func integrateWave(ctx context.Context, repoRoot string, wave Wave) error {
	for _, task := range wave.Tasks {
		branchName := fmt.Sprintf("pigo/%s/%s", wave.Name, task.Name)
		if exitCode, out, stderr, err := gimbal.RunCommand(ctx, "merge-worker", repoRoot,
			"git", "merge", "--no-ff", "-m", fmt.Sprintf("merge %s from wave %s", task.Name, wave.Name), branchName); exitCode != 0 || err != nil {
			return fmt.Errorf("failed merging %s: %s %w", branchName, stderr+out, err)
		}
	}
	// Full wave compilation and regression test check
	if exitCode, out, stderr, err := gimbal.RunCommand(ctx, "wave-vet", repoRoot,
		"go", "test", "-race", "./internal/pi/..."); exitCode != 0 || err != nil {
		return fmt.Errorf("wave regression detected: %s %w", stderr+out, err)
	}
	return nil
}

type WorkerAssessment struct {
	Passed   bool     `json:"passed"`
	Feedback string   `json:"feedback"`
	Gaps     []string `json:"gaps"`
}

const scopeCoachPrompt = `Steer strictly against gold-plating, unrequested abstractions, extra packages, or modifications outside the assigned directory. Keep the implementation minimal, idiomatic Go.`
const validatePrompt = `Verify that the implementation satisfies the brief, has accompanying unit tests, adheres to zero-reflection rules, and contains no committed proof dumps.`
```

---

## 7. Proof and Qualification Strategy

Following Gimbal's core tenets: **Proof is running the real thing yourself and saying what you saw.** There are no committed proof programs, no `.attest/` dump files, and no committed test logs. Repeatable checks live as standard Go unit tests beside the production code they test.

### Step 1: Unit & Component Qualification (BESIDE Code)

- **Provider SSE Parsing (`internal/pi/provider`)**: Tested using an in-memory `httptest.Server` replaying recorded SSE event streams. Validates:
  - Streaming text chunking and reasoning token extraction.
  - Proper handling of stream termination with or without `data: [DONE]`.
  - Mid-stream network error classification and exponential backoff retry.
- **Fuzzy Edit Replacer (`internal/pi/tools/edit`)**: Tested against multi-line code chunks with varying leading whitespace, tabs, and indentation shifts, proving that unedited lines remain bit-for-bit identical.
- **Bash Execution & Teardown (`internal/pi/tools/bash`)**: Tests that:
  - Commands that output late bytes past process exit are completely drained.
  - Commands running `sleep 60` are terminated within 50ms upon context cancellation via `syscall.Kill(-pid, syscall.SIGKILL)`.
- **Session DAG Persistence (`internal/pi/session`)**: Tests that concurrent goroutines writing to distinct sessions do not race, JSONL records are valid, and fork operations produce completely isolated, deep-copied message histories.
- **Auto-Compaction Cut-Point Math (`internal/pi/compaction`)**: Tests that history slicing never splits a `tool_call` from its matching `tool_result`, and verify XML summary prompt formatting.

### Step 2: Integrated Harness & Differential Qualification

- **Differential Test Suite**: Differential comparison against upstream Pi:
  - Input: identical multi-turn conversation traces.
  - Output: verify that `internal/pi/agent` produces identical event ordering (`message_start` &rarr; `content_delta` &rarr; `tool_call_start` &rarr; `tool_call_end` &rarr; `message_end` &rarr; `agent_end` &rarr; `agent_settled`).
- **Gimbal Harness Compliance**:
  - Exercises [`HarnessAdapter`](file:///Users/tyler/src/gimbal-pi-research/harness.go#L14-L40) methods: `CreateSession`, `RunTurn`, `Steer`, `Fork`, and `Close`.
  - Verifies structured output decoding and schema re-ask loops when an agent produces invalid JSON.
  - Verifies that `Steer` returns `landed=true` during active generation and `landed=false` when idle.

### Step 3: Live Milestone Demonstration

The live completion milestone executes a real workflow turn against **Diffusion Router**:
- **Model**: `pigo/diffusion/deepseek-4.1-flash`.
- **Observed Behavior**:
  1. Engine loads configuration from local files without ambient credential leakage.
  2. Session performs a multi-turn coding task: reads a local test fixture, edits a function using fuzzy matching, and runs a Go test command via `bash`.
  3. Context streams live into Gimbal's web UI.
  4. Mid-turn steer message is delivered and changes the worker's trajectory.
  5. Session is forked into an independent child session; both parent and child execute subsequent turns without mutual interference.
  6. Final output satisfies a requested JSON schema, validated by Gimbal's runtime.

### Statutory Licensing Notice

Copied or adapted translation code from [`earendil-works/pi`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi), [`sky-valley/pi`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/sky-valley--pi), or [`amit-timalsina/pi-agent-go`](file:///Users/tyler/.local/share/gimbal-pi-research/sources/amit-timalsina--pi-agent-go) is permissive MIT. In accordance with statutory obligations, all adapted source files must retain the original copyright notice in their top-of-file comments:
```go
// Copyright (c) 2025-2026 earendil-works / sky-valley / amit-timalsina
// Portions adapted for Gimbal under the MIT License.
```

---

## 8. Risk Analysis and Technical Mitigations

| Risk Identified | Root Cause in Candidates / Upstream | Architectural Mitigation in Gimbal Port |
|---|---|---|
| **Subprocess Output Truncation & Race** | `sky-valley` failed `TestBashCapturesOutputPastExit` because reading stopped immediately on process exit before stdout pipe reached EOF. | In `internal/pi/tools/bash`, output collection waits for pipe EOF (`io.ReadAll` / `bufio.Scanner`) before returning `cmd.Wait()` exit status. |
| **Orphaned Grandchild Subprocesses** | Standard `cmd.Process.Kill()` kills only the top-level shell, leaving child commands (e.g. build daemons, sleep) alive. | Set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on spawn, and cancel via `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`. |
| **Fork History Corruption** | `pi-agent-go` shallow-copied message slices in `Snapshot()`, leaving content block slices shared between parent and child. | Implement explicit recursive `Clone()` methods on all `Message` and `ContentBlock` types; verify slice pointer disjunction in unit tests. |
| **Silent Credential Leakage** | Ambient environment variable resolution falling back silently when configured credentials fail. | Strict auth resolution precedence: stored project/global credentials strictly own the provider. If expired or invalid, fail immediately with clear diagnostic rather than falling back. |
| **Prompt Cache Invalidation** | Dynamic instructions or modifying `SKILL.md` mid-session altering the system prompt prefix. | System prompts are assembled with stable, sorted prefix blocks. Dynamic turn additions append to the tail of the message sequence. |
| **Context Compaction Pair Mutilation** | Slicing message history on an arbitrary message boundary separates a `tool_call` from its `tool_result`, breaking provider APIs. | Atomic cut-point calculation algorithm in `internal/pi/compaction` ensures that any slice index containing a tool call is advanced to include all corresponding tool results. |
| **Namespace Friction with Issue #386** | Concurrent development of external RPC adapter (#386) and native port could cause Git merge conflicts in `internal/binding/`. | Isolate native model routes to `pigo/` prefix during development; schedule an explicit cutover task after #386 merges. |

---

## 9. Meaningful Open Decisions for Tyler and Coordinator

1. **Package Prefix & Module Path**:
   - *Option A (Recommended)*: Locate the native engine in `internal/pi/...` with model prefix `pigo/` during development, switching to canonical `pi/` upon cutover.
   - *Option B*: Establish a separate top-level module `pigo` imported by Gimbal. (Not recommended: adds multi-module workspace complexity).
2. **Session Persistence Compatibility**:
   - *Option A (Recommended)*: Maintain 1:1 JSONL DAG disk compatibility with upstream Pi so that session files created in Gimbal can be inspected with external Pi tools.
   - *Option B*: Store native sessions exclusively in Gimbal's run-owned artifact directories using Gimbal's existing event persistence format.
3. **Workspace Skill Discovery Boundary**:
   - *Option A (Recommended)*: Progressive disclosure matching upstream Pi: discover `<available_skills>` frontmatter in system prompts and let the agent inspect full instructions via the `read` tool.
   - *Option B*: Full prompt injection: inject all matching skill markdown files directly into the system prompt. (Risk: increases input token costs and risks prefix cache churn).
4. **Provider Expansion Strategy**:
   - *Option A (Recommended)*: Keep Wave 1 focused strictly on OpenAI-compatible completions for Diffusion Router, adding native Anthropic and Gemini provider adapters in a subsequent chapter.
   - *Option B*: Port `sky-valley`'s Anthropic and Google provider packages during Wave 1. (Risk: expands Wave 1 concurrency beyond 7 workers and delays the initial live milestone).

---
*End of Gemini Port Plan Draft.*
