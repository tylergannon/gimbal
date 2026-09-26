# Native Go Coding Harness Evaluation

## Executive Summary

This report evaluates native Go coding harnesses, agent loop implementations, and provider streaming libraries to determine a viable path for embedding a native Go coding harness in Gimbal, eliminating the Node/Bun Pi subprocess. The evaluation included `amit-timalsina/pi-agent-go` (and `pi-llm-go`), alternative ports like `sky-valley/pi`, `dimetron/pi-go`, `yosukeno/pi-go`, `sonnes/pi-go`, standalone CLI agents (`charmbracelet/crush`), and orchestration frameworks (`google/adk-go`, `cloudwego/eino`).

The central conclusion is that **no existing solution is a drop-in replacement that meets Gimbal's `HarnessAdapter` contract**. `pi-agent-go` is structurally sound but lacks critical features (tools, disk persistence, and instruction discovery). Standalone agents like `crush` are blocked by internal package visibility and licensing. Full framework ports like `sky-valley/pi` carry substantial overhead.

The recommended pathway is **Selective Reuse (Hybrid Strategy)**: adopting proven lower-level components (like `pi-llm-go`'s streaming and `yosukeno`'s filesystem tools) to assemble a purpose-built `HarnessAdapter` owned directly by Gimbal, while satisfying required Apache 2.0 and MIT licensing obligations.

---

## Is Pi Agent Go Good Enough?

**Is it good enough?** **Yes.**
`pi-agent-go` uses zero reflection (`agent.Raw` avoids `jsonschema` entirely), zero global state, and ensures reliable goroutine cleanup via `errgroup` (e.g., `waitDone` synchronization). Tests under `go test -race` pass cleanly. `pi-llm-go` provides excellent streaming iterators and context cancellation.

**Is it up to date enough?** **Partially.**
It tracks the core concepts from early upstream Pi (v0.67 to v0.72) like `TransformContext` hooks but completely omits Pi 0.87.1+ features like tree branching, compaction, and mid-turn RPC schemas.

**Is it complete enough?** **No.**
It requires significant wrapping to function as a coding harness. It lacks:
1. Built-in coding tools (`bash`, `read`, `write`, `edit`).
2. Disk session persistence (`Snapshot` only provides in-memory clones with shared backing arrays).
3. Working directory scoping.
4. Correct idle steering (unanswered `Steer()` drops messages instead of returning `landed=false`).
5. Instruction and skill discovery logic (explicitly deferred to the caller).

---

## Candidate Matrix, Maintenance, and Rejections

| Candidate | Maintenance & Release Status | Rejection / Adoption Reason |
|---|---|---|
| **`pi-agent-go`** / `pi-llm-go` | Moderate maintenance. Clean test suite. | **Selective Reuse**. Clean concurrency, no reflection. Retain `pi-llm-go` for streaming; wrap `pi-agent-go` heavily or port the loop. |
| **`sky-valley/pi`** | **Daily upstream synchronization**, tracking Pi commits. | **Reference**. Perfect line-for-line port of upstream Pi, but carries large overhead (embedded catalog). Useful as a benchmark. |
| **`charmbracelet/crush`** | Active. | **Rejected**. Blocked by FSL-1.1-MIT commercial non-compete clause. Core logic sealed inside `internal/`. Process-global singletons limit concurrency. |
| **`dimetron/pi-go`** | Active release cycle using **GoReleaser** and **Sigstore** provenance. | **Rejected**. Over-abstracted over Google ADK Go. Suffers from `os.Root` path traversal blocks. Logic is sealed in `internal/`. |
| **`google/adk-go`** / `eino` | Active enterprise frameworks. | **Rejected**. Introduces generic orchestration, complex DAGs, and heavy abstractions violating Gimbal's "ordinary Go" rules. |
| **`sonnes/pi-go`** | Intermittent bursts. | **Selective Reuse**. Good tool patterns. Uses Go 1.23 `iter.Seq2`. Re-architected as a framework rather than an agent, making it unfit as a direct drop-in. |
| **`yosukeno/pi-go`** | **Abandoned**. Last commit precisely on 2026-08-14. | **Selective Reuse**. Abandoned prototype, but contains clean, zero-dependency implementations of `edit` and `bash` tools that can be selectively salvaged. |

---

## Deep Dive Findings & Evidence

### 1. Provider Streaming and Concurrency (`pi-llm-go`)
- **Fragmented Arguments**: `pi-llm-go` successfully concatenates streamed tool arguments using a string builder (`internal/sse/sse.go:45-102`).
- **Context Cancellation**: Canceling a stream halts the underlying HTTP request correctly, unwinding the `iter.Seq2` safely. However, **Defect**: if an endpoint abruptly terminates without emitting `data: [DONE]`, `finalize()` is skipped and the tool call is silently dropped (`stream.go:144-178`).
- **Usage Metrics**: Successfully partitions `CacheReadTokens` and reasoning tokens (`usage.go:7-79`), making it compatible with Diffusion Router metrics.

### 2. Endpoint Routing and Configurability
- **`fantasy` Routing Mechanics**: `charmbracelet/fantasy` natively handles custom base URLs, custom headers, and model aliases for Diffusion Router via functional options (`WithBaseURL`, `WithHeaders`) in its `openaicompat` package.
- **`pi-llm-go` Gap**: `pi-llm-go` currently lacks this level of dynamic endpoint configurability, custom header injection, and catalog aliasing required to natively route through Diffusion Router, representing a necessary implementation gap.

### 3. Schema Handling and Validation
- **Gimbal Outer Retry Loop**: Because upstream Pi core RPC and `pi-agent-go` lack native `response_format` or top-level JSON Schema enforcement, schema turns must be managed externally. Gimbal's runtime already supplies this via an outer validation loop (`session.go:187-223`) which applies `out.ValidateJSON` against the model's text response, re-prompting up to 3 times on failure.
- **`fantasy` Fallback**: `charmbracelet/fantasy` experiments with an `ObjectModeText` fallback mechanism (`object.go:26`) that injects schema requirements into the prompt and repairs malformed JSON using `jsonrepair`, though it relies on runtime reflection for tool schema generation (`tool.go:108`), which conflicts with Gimbal's zero-reflection rule.

### 4. File Editing & Bash Tools
- **Upstream Pi vs Crush**: Upstream Pi enforces strict, disjoint replacements using NFKC normalization (`edit.ts:21-51`). `crush` employs sequential string replacement with whitespace fallback (`edit.go:195-222`), which is more forgiving to models but risks semantic shifts in whitespace-sensitive languages.
- **Child Processes**: Both `sky-valley` and `crush` handle bash backgrounding effectively, but `crush` relies on a global `BackgroundShellManager` singleton (`background.go:62-87`), creating concurrency hazards for multi-agent workloads.

### 5. Instruction and Skill Discovery
- **`pi-agent-go`**: Intentionally excludes instruction and skill discovery, leaving it entirely to the harness layer.
- **`sky-valley/pi`**: Achieves exact semantic parity with upstream Pi, recursively discovering `AGENTS.md` and injecting `<available_skills>` for progressive disclosure. A test failure observed here (`~/.pi/skills`) was diagnosed as an unisolated host `HOME` variable flaw in the test harness (`resources_test.go`), not a logic defect.
- **`dimetron/pi-go`**: Implements active unicode security scanning (`audit.ScanFile`) to prevent prompt injection and uses a two-level active-skill prompt injection upon `/skill` slash command activation.
- **`sonnes/pi-go`**: Takes a functional `io/fs.FS` approach, requiring a dedicated `skill` tool for execution instead of progressive disclosure.

### 6. Session Persistence & Branching
- `sky-valley/pi` faithfully implements upstream Pi's Version 3 JSONL persistence tree (`coding/session_tree.go:18-50`).
- `pi-agent-go`'s `RunSnapshot.Messages` uses `[]llm.Block` interfaces, failing standard `encoding/json` deserialization (`restore.go:36-75`). Shallow copying causes branched snapshots to mutate the same memory addresses (`&snapOrig.Messages[0].Content[0]`).

### 7. Test Suite Isolation and Credential Failures
- **Clean Harnesses**: `pi-agent-go`, `pi-llm-go`, and `sonnes/pi-go` pass clean no-key tests under `go test -race` in isolated scratch environments.
- **Credential & Setup Brittleness**: `dimetron/pi-go` unit tests fail in no-key scratch environments due to unmocked live API calls to Anthropic in `internal/acp/server`. Similarly, `yosukeno/pi-go` unit tests fail outright due to hardcoded dependencies requiring an existing `~/.pi-go/providers.json` configuration file on disk.

---

## Ownership Strategy Recommendation

**Recommended Path: Selective Reuse (Hybrid)**
Gimbal should own its `HarnessAdapter` directly. Adopting a complex monolith (`sky-valley`) introduces maintenance drag, while raw dependency (`pi-agent-go`) leaves massive functional gaps. 

**Licensing and Attribution Obligations**:
- `pi-llm-go` is licensed under **MIT**, requiring standard copyright preservation.
- `yosukeno/pi-go` is licensed under **Apache 2.0**. Reusing its zero-dependency tool implementations requires strictly adhering to **Apache Section 4 modification notice obligations** and proper copyright attribution.

**Implementation Plan:**
1. **Streaming**: Embed or rely on `pi-llm-go` for stable OpenAI-compatible SSE parsing and token accounting.
2. **Agent Loop**: Fork or semantic port of `pi-agent-go`'s loop to fix `[DONE]` truncation drops, solve shallow-copy persistence bugs, and enforce Gimbal's steering protocol.
3. **Tools & Discovery**: Port the zero-dependency tool implementations from `yosukeno/pi-go` (respecting Apache 2.0 notice requirements) and implement Gimbal-native instruction and skill discovery aligned with `sky-valley`'s progressive disclosure.

### Remaining Qualification Work for Gimbal

Before the Node/Bun subprocess can be safely removed, the following test gates must be satisfied by the constructed harness:
1. **Streaming Validation**: A mock SSE server proving resilient recovery from truncated tool JSON and omitted `[DONE]` events without dropping agent actions.
2. **Tool Sandboxing**: A filesystem test suite proving parallel tools lock correctly, and that bash subshells are terminated cleanly on context cancellation without leaking process groups.
3. **Contract Adherence**: Integration tests proving `HarnessAdapter.Fork` provides fully detached memory and that `Steer` accurately rejects payloads during text-only generation.
4. **Live Router Qualification**: An E2E test executing a multi-turn file edit sequence over `diffusion/deepseek-4.1-flash` resolving successfully.

### Unresolved Evidence Gaps
1. **Prefix Cache Invalidation**: It remains unverified whether dynamic modification of a `SKILL.md` file during an active multi-turn session invalidates LLM prompt prefix caches across providers.
2. **Skill Delivery Alignment**: It is not yet settled which skill delivery mechanism best aligns with Gimbal's architecture: upstream Pi/sky-valley progressive disclosure (`<available_skills>` + read tool), dimetron's two-level active-skill prompt injection, or sonnes's dedicated skill tool invocation.
