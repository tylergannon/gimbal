# Designing `gimbal`: A Go Orchestration Library over Coding-Agent Harnesses

## TL;DR
- **Drive harnesses over ACP; do not build your own abstraction over the Anthropic/OpenAI model SDKs, and do not expose ACP raw.** No existing Go library does what gimbal wants (a multi-harness façade); the closest usable primitive is `coder/acp-go-sdk` (v0.13.5, Apache-2.0), a complete typed ACP client you should sit on top of. ACP's `_meta` fields, underscore-prefixed custom methods, and `Unstable*` method surface mean the "abstraction hides vendor features" fear is **largely unjustified for control/pass-through**, but **justified for first-class ergonomics** of vendor features.
- **On your API-shape questions: no Future object, no per-turn `Task` interface.** Model a turn as `iter.Seq2[Event, error]` (Go 1.23 range-over-func) returned from `Session.Prompt`, keep the session itself as the steer/interrupt handle, and use package-level generic functions for structured output (interface methods can never declare type parameters — not even in Go 1.27). Design B (a `Task` handle) is over-engineered: every real harness models steer/interrupt/compact as session/thread-scoped operations, not turn-scoped ones.
- **Structured output, capability discovery, and the outer loop are where to invest.** Use optional interfaces + type assertion (the `http.Flusher` pattern) for vendor-specific features; own the outer coding→review→coding loop in plain Go; reach for Temporal only if you need durability.

## Key Findings

1. **No Go multi-harness orchestration library exists.** The ecosystem splits into (a) LLM app frameworks (Eino, LangChainGo, Genkit Go) that orchestrate *models*, not *coding-agent harnesses*; (b) vendor model SDKs (anthropic-sdk-go, openai-go, google genai); (c) a proliferating set of unofficial single-vendor Claude Code subprocess drivers; and (d) ACP client/server libraries. Nobody has unified Claude Code + Codex + Gemini behind one idiomatic Go API. Genuine gap.

2. **ACP is the de-facto wire standard and the right foundation.** Gemini CLI (`--acp`), Claude Code (adapter), Codex (adapter), and Antigravity (community adapters) all converge on newline-delimited JSON-RPC 2.0 over stdio. `coder/acp-go-sdk` models the entire current schema including the `Unstable*` extension surface.

3. **The Future/promise debate is settled against you in idiomatic Go.** Temporal uses `workflow.Future` only because it must run deterministic, replayable, single-threaded coroutines where real goroutines/channels are illegal. In-process you have goroutines, channels, `context`, `errgroup`, and `iter.Seq`. Don't import a promise abstraction.

4. **"Steer," "follow-up," and "interrupt" are three distinct operations — but only Codex exposes true mid-turn steering, and only at its app-server/TUI layer, not headless exec.** Claude Code's streaming input mode queues messages and supports `interrupt()` + re-`query()` but does not inject into a running turn. This directly determines whether a `Steer` method can honestly be mandatory (it can't).

5. **Explicit compaction is not a first-class SDK call in either Claude or Codex.** Claude exposes `PreCompact`/`PostCompact` hooks + a `/compact` CLI command; Codex exposes `thread/compact/start` at the app-server layer but not in the high-level SDK or `codex exec`. A mandatory `Compact(ctx)` method would be a leak — it belongs behind an optional interface.

## Details

### PART 1 — Landscape survey

**Go agent/LLM frameworks (orchestrate models, not harnesses):**

- **Eino** (`github.com/cloudwego/eino`, ByteDance/CloudWeGo, Apache-2.0, very active — docs updated as recently as Jan 23, 2026). The most serious Go agent framework; its ADK deliberately mirrors Google ADK. Core interface:
  ```go
  type Agent interface {
      Name(ctx context.Context) string
      Description(ctx context.Context) string
      Run(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent]
  }
  ```
  Note the event-streaming shape: `Run` returns an `*AsyncIterator[*AgentEvent]` (a pre-`iter.Seq` custom iterator type). Collaboration primitives: Sequential/Parallel/Loop/Supervisor/Plan-Execute; a `Runner` with callbacks, interrupts, checkpoints. Closest existing precedent for what gimbal's *event stream* should look like, though Eino targets in-process model ReAct loops, not external harness subprocesses.
- **LangChainGo** — LangChain port; chains/agents/tools over models. General-purpose, not harness-oriented.
- **Genkit Go** (Firebase) — model orchestration, flows, tracing.
- **swarmgo / go-agent / agno-go** — small/experimental; not authoritative.

**Vendor model SDKs (API-shape precedent, not harness driving):**

- **anthropic-sdk-go** (`github.com/anthropics/anthropic-sdk-go`, official). The single most important Go streaming precedent for you:
  ```go
  func (r *MessageService) NewStreaming(ctx context.Context, body MessageNewParams, opts ...option.RequestOption) *ssestream.Stream[MessageStreamEventUnion]
  ```
  Usage: `for stream.Next() { event := stream.Current(); message.Accumulate(event) }` then check `stream.Err()`. This is the **pre-`iter.Seq` "stateful cursor" pattern** (`Next()`/`Current()`/`Err()`), same shape as `sql.Rows` and `bufio.Scanner`. Known weakness to improve on: mid-stream errors surface as unstructured `fmt.Errorf("received error while streaming: …")` (issue #200). `Message.Accumulate(event)` folds streaming deltas into a complete `Message` — the accumulation pattern gimbal should copy.
- **openai-go** — same house style (stateful streaming cursors, params structs, union types).
- **google genai Go SDK**, **Ollama Go bindings** — model-level.

**MCP Go SDKs:**
- **`github.com/modelcontextprotocol/go-sdk`** — the **official** MCP Go SDK, maintained in collaboration with Google. It has since shipped stable: v1.0.0 ("going forward we won't make breaking API changes"), reaching v1.2.0 by Dec 22, 2025 and v1.4.1+ in 2026. Packages: `mcp`, `jsonrpc`, `auth`, `oauthex`; uses `_meta` keys (e.g. `io.modelcontextprotocol/protocolVersion`). Use this if gimbal needs to *host* MCP servers.
- **`github.com/mark3labs/mcp-go`** (Ed Zynda) — popular unofficial predecessor that influenced the official design; still viable.

**Unofficial Claude Code Go drivers (the "Go Claude Agent SDK") — crowded and immature:**
- `victorarias/claude-agent-sdk-go` — Python-SDK port; author states "not been fully tested in production. Use at your own risk."
- `severity1/claude-agent-sdk-go` — two APIs, `Query()` for automation and a `Client` for interaction; `WithClient(ctx, func(client))` context-manager idiom.
- `Roasbeef/claude-agent-sdk-go` — pure-Go, drives the CLI as subprocess over line-delimited JSON on stdin/stdout; unusually explicit about tracking upstream control-protocol churn (list_models, get_workspace_diff, get_plan, set_cwd control requests).
- `schlunsen/claude-agent-sdk-go` — `Query()` one-shot + `Client` bidirectional; hooks, MCP, zero-dependency core; `ListSessions`/`ListSubagents` helpers.
- `M1n9X/claude-agent-sdk-go`, `character-ai/claude-agent-sdk-go` — claim near-full Python parity (204 features, all 12 hook events).
None is official or dominant. **Do not adopt any as a dependency;** study `Roasbeef`'s and `severity1`'s shapes as references. Note the recurring split — a package-level `Query()` for one-shot and a stateful `Client` for interactive — which maps directly onto your Design A/B question.

**ACP libraries in Go:**
- **`github.com/coder/acp-go-sdk`** (v0.13.5, Apache-2.0, published Jun 2, 2026, imported-by 18) — the strongest option. Fully typed, generated from the official schema. Per its release notes it "Catches the SDK up to ACP schema 0.10.8 (from 0.6.3), adds first-class ACP extension methods… `ExtensionMethodHandler` plus `CallExtension`/`NotifyExtension`, and `Unstable*` types for in-development methods." The `ClientSideConnection` (what gimbal *is*) exposes `Initialize`, `NewSession`, `LoadSession`, `ResumeSession`, `ListSessions`, `Prompt`, `Cancel`, `SetSessionMode`, `SetSessionConfigOption`, `Authenticate`, `Logout`, `CloseSession`, plus a large `Unstable*` surface (`UnstableForkSession`, `UnstableListProviders`/`UnstableSetProvider`, `UnstableConnectMcp`, NES methods) and `CallExtension`/`NotifyExtension`. Ships `example/claude-code`, `example/gemini`, `example/client`. Constructors: `NewClientSideConnection(client, stdin, stdout)`, helper builders `TextBlock`/`ImageBlock`/`ResourceBlock`, `Ptr[T]`.
- `ironpark/acp-go`, `a3tai/openclaw-go/acp` — other unofficial Go implementations; Coder's is the most complete.

**Orchestration primitives worth stealing:**
- **Temporal Go SDK** — `workflow.NewFuture(ctx) (Future, Settable)`, `Selector` (deterministic replacement for `select` that waits on Futures *and* Channels), `workflow.Await(ctx, cond)`, `workflow.Go` (coroutine). The idiom `future, settable := workflow.NewFuture(ctx); workflow.Go(ctx, func(ctx){ … settable.Set(v, err) })` is exactly the promise pattern — but it exists *only because* Temporal forbids real goroutines inside workflows for determinism/replay. **This model does not transfer to an in-process library.**
- **errgroup** (`golang.org/x/sync/errgroup`) — right tool for the outer loop's fan-out/first-error/context-cancellation.

### PART 2 — ACP evaluation

**Current protocol version: wire version 1** (stable). Schema artifacts are versioned separately from the wire protocol; compatibility must be judged from the negotiated `protocolVersion` plus exchanged capabilities, **not** from SDK/crate/schema version. Transport is newline-delimited JSON-RPC 2.0 over stdio; a remote HTTP/WebSocket design exists only as an active RFD.

**Method set** (from the schema and coder/acp-go-sdk): `initialize`, `authenticate`, `session/new`, `session/load`, `session/prompt`, `session/update` (agent→client streaming notification), `session/cancel` (notification, not request), `session/request_permission`, `session/set_mode`, `fs/read_text_file`, `fs/write_text_file`, and terminal methods (`terminal/create`, `terminal/output`, `terminal/release`, `terminal/wait_for_exit`, `terminal/kill`). Newer/unstable: session resume, fork, list, delete; provider list/set; MCP connect/disconnect; elicitation; next-edit-suggestion (NES). Slash commands surface via `session/update` `available_commands_update`; modes via `SessionModeState`/`set_mode`.

**Streaming + stop reasons.** A prompt turn begins with a `session/prompt` request and ends when the agent returns a `StopReason` on that request's response. In between, the agent streams `session/update` notifications: `agent_message_chunk`, `agent_thought_chunk`, `tool_call`/`tool_call_update`, `plan`, `user_message_chunk` (on replay), usage updates. Stop reasons are a closed enum: `end_turn`, `cancelled`, `max_tokens`, `max_turn_requests`, `refusal`. Cancellation is cooperative: client sends `session/cancel` (notification); agent aborts model/tool calls, flushes pending updates, then responds to the original `session/prompt` with `cancelled`. Clients must keep accepting tool-call updates after cancelling.

**Extensibility — the crux of your concern.** ACP has an explicit, first-class extension story, so the "abstracts away vendor features / takes control away" fear is **largely unjustified for pass-through**:
- **`_meta` field on every type** (`{ [key: string]: unknown }`) — on requests, responses, notifications, content blocks, tool calls, plan entries, and capability objects. Reserved keys `traceparent`/`tracestate`/`baggage` for W3C trace context.
- **Underscore-prefixed custom methods** — any method starting with `_` is reserved for custom extensions and won't collide with future protocol versions. coder's SDK exposes this via `HandleExtensionMethod(ctx, method, params)` inbound and `CallExtension`/`NotifyExtension` outbound.
- **Capability negotiation via `_meta` in capability objects** — e.g. `agentCapabilities._meta["zed.dev"] = {workspace:true, fileNotifications:true}` lets both sides advertise/detect vendor extensions during `initialize`.

**BUT the concern is justified in a narrower, real sense:** ACP is an editor↔agent protocol. Its data model is the *lowest common denominator of an IDE chat panel* (messages, thoughts, tool calls, diffs, permissions, plans). Vendor-specific *first-class* concepts — Claude Code's subagents/skills/hooks/checkpointing, Codex's `--output-schema` and guardian review sub-agent — are **not** modeled as first-class ACP methods. You *can* pass them through `_meta` and `_`-methods, but you get no type safety, no ergonomics, and you depend on each adapter (claude-code-acp, codex acp) actually surfacing them. Some are simply lost at the adapter boundary today: the Claude Code ACP adapter reports `end_turn` with no output when the subscription session limit is hit (claude-agent-acp issue #146), i.e. errors that don't map to an ACP stop reason get swallowed. **Net:** ACP preserves *control* (permissions, cancellation, file access) well; it is lossy on *vendor feature surface* unless the adapter and both endpoints cooperate through `_meta`.

**Who speaks ACP today:** Gemini CLI (`--acp`, reference implementation; `--experimental-acp` still used by many integration guides — support both), Claude Code (beta adapter, `claude --acp`/`claude-code-acp`), Codex (adapter/`codex acp`), Goose, Antigravity (community adapters; native `agy --acp` is an open upstream request, not shipped — and Google's FAQ calls third-party ACP access a ToS violation). JetBrains and Zed both ship ACP clients.

### PART 3 — API shape / idiomatic Go design

**(a) Future vs channel vs Wait()-able session → use `iter.Seq2[Event, error]`.**

Community consensus is that futures/promises are un-idiomatic; you have goroutines + channels + `context`. Go 1.23 (released August 13, 2024) made range-over-func stable and added the `iter` package defining `type Seq[V any] func(yield func(V) bool)` and `type Seq2[K,V any] func(yield func(K,V) bool)`. The `iter.Seq2[T, error]` shape — yielding `(value, error)` pairs, caller `break`s to stop, cleanup via `defer` inside the iterator — is now the canonical way to return an event stream, mirroring the streaming-DB-rows pattern. So:

```go
func (s *Session) Prompt(ctx context.Context, text string) iter.Seq2[Event, error]
```
```go
for ev, err := range sess.Prompt(ctx, "refactor X") {
    if err != nil { break }
    switch e := ev.(type) {
    case MessageChunk: ...
    case ToolCall:     ...
    case TurnEnded:    _ = e.StopReason
    }
}
```

This beats the alternatives: a Future forces `.Get()` and loses streaming; a raw `<-chan Event` leaks close/cancellation semantics onto the caller and can't carry a terminal error cleanly; a `Wait()`-able session forces buffering or out-of-band events. It matches Anthropic's own Go SDK philosophy (stream of typed union events) while upgrading from the older `Next()/Current()/Err()` cursor. **Caveat:** `iter.Seq2` is pull-driven by the caller's loop; a harness turn is push-driven (the subprocess emits whenever it wants). Run a goroutine reading the ACP `session/update` stream into an internal channel and yield from that within the iterator (bridge with `iter.Pull` only if you must drive two streams in lockstep). Keep stateful accumulation (final assistant message, `StopReason`) inside the `Session` so the caller reads `sess.LastResult()` after the loop — exactly as `Message.Accumulate` does. Put the stop reason on both the `TurnEnded` event and the session.

**(b) Structured output → package-level generic function, not a method.** Go methods could not declare their own type parameters before Go 1.27 (the `method must have no type parameters` error), and even Go 1.27's generic-methods feature **still forbids type parameters on *interface* methods** (the compiler can't enumerate one method-set entry per possible instantiation). Since gimbal's session is an interface, you **cannot** write `func (s Session) PromptJSON[T any](...) (T, error)` now or later. The correct workaround is the package-level generic function taking the session as its first argument — exactly how `math/rand/v2` had to expose a package-level generic draw, and how iterator libs expose `Map(s, f)` instead of `s.Map(f)`:

```go
func PromptJSON[T any](ctx context.Context, s Session, prompt string, schema *jsonschema.Schema) (T, error)
```

This mirrors the harnesses (Codex `codex exec --output-schema <file>`; Claude Agent SDK's `output_format`/`outputFormat: {type:"json_schema", schema:…}` validated against the final result, returned on `message.structured_output`) and both idiomatic Go patterns: caller-provided pointer (`json.Unmarshal`/`sql.Rows.Scan` "output parameter" idiom) *or* generic return. Prefer the generic return; also offer a `Scan`-style `PromptInto(ctx, s, prompt, &dst)`. Do **not** add a stringly-typed second method returning JSON-as-string.

**(c) Design A vs Design B → Design A, refined; reject the `Task` handle.**

Interface guidance is unambiguous: keep interfaces small, "accept interfaces, return structs" (Jack Lindamood; Dave Cheney, *SOLID Go Design*/*Practical Go*), interfaces belong to the consumer, avoid interface pollution (rakyll; Ardan Labs; Go Code Review Comments). Design B returns a `Task` interface from `RunPrompt` — a returned interface, violating "return structs" — and reifies a "turn" as a first-class object.

**Does a "turn"/"task" object exist in any real harness API? Essentially no, in the way Design B implies.** Claude's `ClaudeSDKClient` models everything *session-scoped*: `query()`, `interrupt()`, `receive_response()`, `set_model()`, `set_permission_mode()`, `rewind_files()` — `interrupt()` is a client method, not a handle returned by `query()`. (The TypeScript `query()` *does* return a `Query` object carrying `interrupt()`/`setModel()`/`setPermissionMode()`, so there's *one* precedent for a per-call handle — but even there it's the whole query stream, not a lightweight `Task`.) Codex's steer/interrupt/compact (`turn/steer`, `interruptTurn`, `thread/compact/start`) are thread/session-scoped app-server methods. So steering and interrupting are properties of the *session*; the streaming *turn* is naturally the `iter.Seq2` you already return — not a separate handle.

Recommended shape (Design A, corrected to return an iterator and gate optional ops behind capability interfaces):
```go
// Core, mandatory surface — small, consumer-defined.
type Session interface {
    Prompt(ctx context.Context, text string) iter.Seq2[Event, error]
    Interrupt(ctx context.Context) error
    Close() error
}
// Optional capabilities, discovered by type assertion (the io.ReaderFrom / http.Flusher pattern).
type Steerer     interface { Steer(ctx context.Context, text string) error }
type Compactor   interface { Compact(ctx context.Context) error }
type ModelSetter interface { SetModel(ctx context.Context, model string) error }
type Forker      interface { Fork(ctx context.Context) (Session, error) }
```
`FollowUp` is **not** a distinct primitive — a follow-up is just the next `Prompt` on the same session (Claude/Codex both model it as another turn on the persisted session). Fold it away.

**Semantics you must get right (and where your proposed API leaks):**
- **Interrupt** = cancel the in-flight turn. Universally supported (ACP `session/cancel`; Claude `interrupt()`; Codex `interruptTurn`). Belongs on the core interface. Claude caveat: after `interrupt()` you must drain the interrupted turn's messages (including its `ResultMessage` with `subtype="error_during_execution"`) before the next turn's output appears — so your iterator must surface an `Interrupted`/`TurnEnded{Cancelled}` event, not silently swap streams.
- **Steer** = inject a message into a *running* turn without cancelling. **The leaky one.** Only Codex documents true mid-turn steering (`turn/steer`: "append user input to the currently in-flight turn without creating a new turn"), and only at the app-server/TUI layer — *not* in headless `codex exec` or the high-level SDK — and it fails on non-steerable turns (`review`, `compact`). Claude Code's streaming input mode *queues* messages and supports interrupt-then-resend, but does not inject into a running turn; ACP has no steer method. **Therefore `Steer` cannot be a mandatory method** — put it behind the `Steerer` optional interface, implement it for Codex via the `_`-method/`_meta` extension, and elsewhere fall back honestly to interrupt+resubmit or post-turn queue (documented).
- **Compact** = explicitly compress context. Not first-class anywhere: Claude has `/compact` (CLI) + `PreCompact`/`PostCompact` hooks; Codex has app-server `thread/compact/start` but not exec/SDK. `Compact` belongs behind an optional `Compactor` interface.

Decisive argument against both your candidates as written: **A puts `Steer`/`Compact` on the mandatory interface (dishonest — most backends can't do them); B invents a `Task` handle nothing in the ecosystem needs.** Correct design = A's flat session shape, minus `FollowUp`, minus mandatory `Steer`/`Compact`, plus optional capability interfaces.

**(d) Capability matrix and the optional-interface pattern.**

| Capability | Claude Code | Codex | Gemini CLI | Antigravity | ACP method |
|---|---|---|---|---|---|
| Session create/resume | yes (`--continue`, id) | yes (`codex exec resume`, `--last`, id) | yes (`--resume`, `/resume`) | via adapter (SQLite conv DB) | `session/new`, `session/load`, resume (unstable) |
| Streaming events | yes (stream-json) | yes (`--json` JSONL) | yes (ACP updates) | yes (adapter) | `session/update` |
| Tool/permission callbacks | yes (permission callback) | yes (guardian/approvals) | yes (approval mode) | yes (hooks can *ask*) | `session/request_permission` |
| Interrupt/cancel | yes (`interrupt()`) | yes (`interruptTurn`) | yes (`cancel`) | yes | `session/cancel` |
| Mid-turn steering | no (queue + interrupt) | yes (app-server `turn/steer`; not exec) | no | no | none (use `_`-ext) |
| Compaction (explicit) | `/compact` + hooks | app-server `thread/compact/start` | no explicit | auto | none first-class |
| Model selection | `set_model()`/`setModel` | model flag/config | `-m`, unstable `setSessionModel` | `--model` | `set_mode`/unstable model |
| MCP servers | yes | yes | yes | yes (subagent MCP isolation broken) | MCP config + unstable connect |
| Hooks | 12 events (PreToolUse…PostCompact) | TUI hook activity | limited | yes (policy hooks) | none |
| Subagents | yes (SDK) | yes (multi-agent V2) | no | yes (SDK, caveats) | none |
| Structured output | `output_format` json_schema | `--output-schema` | no | via SDK | none |
| Cost/token reporting | yes (result msg) | yes | yes | yes | `usage`/`Cost` updates |

The Go-idiomatic rendering of "optional capabilities" is the **optional interface + comma-ok type assertion**, exactly as the stdlib does with `io.ReaderFrom`/`io.WriterTo` (used by `io.Copy` for a fast path) and `http.Flusher`/`http.Hijacker`/`http.Pusher` (upgraded from `http.ResponseWriter`):
```go
if sc, ok := sess.(Steerer); ok { _ = sc.Steer(ctx, "actually, use x/foo") }
```
**Heed the known anti-pattern** (Merovius, "The trouble with optional interfaces"; Doxsey, "Fixing interface erasure in Go"): if gimbal ever *wraps* a `Session` (middleware, tracing, retry), the wrapper silently drops the wrapped value's optional interfaces — the same bug that makes a wrapped `http.ResponseWriter` lose `Flush`/`Hijack`. Mitigations: keep the mandatory `Session` tiny; make optional methods safe to be absent (return typed `ErrUnsupported`); and because you *will* wrap (tracing + outer loop), **also expose an explicit `func Supports(s Session, cap Capability) bool` / `Capabilities()`** so discovery doesn't depend solely on interface transparency surviving wrapping.

### PART 4 — Higher-level orchestration patterns

**The outer loop (coding → review → coding) is yours to own, in plain Go.** This is the central lesson of Dex Horthy / HumanLayer's **12-Factor Agents** (humanlayer/12-factor-agents, ~23.5k GitHub stars, last pushed Sept 2025). Factor 8, "Own Your Control Flow": *"The model can choose the next action, but the application should own the loop, stop conditions, retries, approval gates, and budget ceilings. This is the difference between a product and a runaway process."* (Also Factor 10 small focused agents; Factor 5 unify execution + business state; Factor 12 stateless reducer.) Anthropic's own guidance — Erik Schluntz and Barry Zhang, "Building Effective Agents" (Dec 19, 2024) — names the **evaluator-optimizer** workflow: *"one LLM call generates a response while another provides evaluation and feedback in a loop,"* plus **orchestrator-workers**. Your outer loop *is* evaluator-optimizer with a coding harness as optimizer and a review harness as evaluator. LangGraph models this as an explicit graph with a loop edge + termination condition; you don't need a graph library for a single loop.

Concrete state machine to implement in Go (no framework):
```
state: {iteration, maxIter, budget(tokens/$), lastDiff, lastReview}
loop:
  1. code := coder.Prompt(ctx, task+lastReview)   // consume iter.Seq2; capture diff, StopReason, cost
  2. if stop == refusal/max_tokens → break with typed error
  3. review := reviewer.Prompt(ctx, code.diff)    // separate Session, possibly different harness
  4. if review.approved || iteration>=maxIter || overBudget → break
  5. lastReview = review.text; iteration++
```
Termination is explicit: `maxIter`, cost/token budget, an approval predicate, or a no-progress heuristic (identical diff twice). Use `context` for cancellation and `errgroup` if you fan out multiple reviewers. Persist `{iteration, session ids, cost}` so you can resume — Horthy's "unify execution and business state."

**Sprints/milestones/plans/tasks.** You likely won't model this; what exists: Claude Code's `TodoWrite`/todo lists (surfaced in some Go drivers as `EnableTodos`/`TodoStore` + `AgentEventTodosUpdated`); ACP's first-class `plan`/`PlanEntry` updates (priority + status, replace-semantics); HumanLayer CodeLayer, Devin, OpenHands, Roo/Cline task lists; spec-driven approaches (GitHub spec-kit, Amazon Kiro). If you want it later, ACP's `plan` update stream is the harness-neutral surface to expose.

**Durable execution in Go.** Temporal's `workflow.Future`/`Selector`/`Await` model **does not transfer** to an in-process, non-durable library — it exists to make single-threaded, replayable, deterministic workflows possible, the opposite of your goroutine-rich design. If you later need durability (survive process restart mid-loop), the options with Go SDKs are Temporal (each harness turn is non-deterministic so it *must* be an Activity, not workflow code), Restate, or Inngest. **Do not build durability into v1;** keep sessions resumable via the harnesses' own resume (Codex `resume`, Claude `--continue`, ACP `session/load`) and persist outer-loop state yourself.

**Primitives across the major SDKs, with Go-idiomatic rendering:**
- sessions/threads → `Session` interface ✔
- runs/turns → the `iter.Seq2` stream ✔ (no separate object)
- handoffs (OpenAI Agents SDK) → a function you write in the outer loop; not a library primitive
- guardrails (OpenAI) / permission callbacks (Claude) → a `PermissionFunc` on session construction ✔
- hooks (Claude 12 events) → optional `Hooker` interface / functional options; vendor-specific, gate behind capability
- middleware (Eino) → Go `func(Session) Session` decorators — but beware interface erasure (above)
- tools / MCP servers → construction-time config; use official `modelcontextprotocol/go-sdk`
- subagents → vendor-specific; optional interface or `_meta`
- memory/compaction → optional `Compactor`
- checkpoints/forking/rewind (Claude `rewind_files`, ACP `UnstableForkSession`) → optional `Forker`/`Rewinder`
- permission modes (Claude `plan`/`acceptEdits`; ACP modes) → `SetMode`/optional
- output schemas → package-level `PromptJSON[T]` ✔
- tracing/telemetry → `_meta` `traceparent` pass-through + `slog`
- cost accounting → `Usage`/`Cost` on `TurnEnded` ✔
- human-in-the-loop → permission callback + outer loop; HumanLayer Factor 7 ("contact humans with tool calls")

### PART 5 — Synthesis: recommended API and build-vs-adopt verdict

**Verdict: build gimbal as a thin, opinionated multi-harness façade on top of `coder/acp-go-sdk`, driving harnesses over ACP where they speak it and via direct subprocess adapters where they don't (Claude Code today, Antigravity).** Do not adopt any existing Go library wholesale — none spans harnesses. Do not expose ACP types raw (they're an IDE-panel data model and will leak editor concepts into your API). Do not build on the unofficial Claude-Code Go SDKs (unmaintained, single-vendor). Prefer ACP over hand-rolling N subprocess protocols, because ACP already normalizes sessions/streaming/permissions/cancellation/files and gives you `_meta` + `_`-methods for vendor pass-through — the exact escape hatch that neutralizes your "abstraction hides features" concern *for control*. Accept that vendor *feature ergonomics* (subagents, skills, hooks, structured output) must be surfaced through optional interfaces and extension methods, harness-by-harness.

Recommended package surface:
```go
package gimbal

// Construction: accept interfaces / functional options; return concrete structs.
type Harness interface { // implemented by claudeHarness, codexHarness, geminiHarness…
    Open(ctx context.Context, opts ...SessionOption) (Session, error)
}
func Claude(opts ...Option) Harness
func Codex(opts ...Option)  Harness
func Gemini(opts ...Option) Harness
func OverACP(cmd string, args ...string) Harness // generic: any `x --acp` binary

// Core session — deliberately tiny.
type Session interface {
    Prompt(ctx context.Context, text string) iter.Seq2[Event, error]
    Interrupt(ctx context.Context) error
    Info() SessionInfo // id, model, cwd, capabilities snapshot
    Close() error
}

// Events: a closed set of concrete structs behind a sealed interface.
type Event interface{ isEvent() }
type MessageChunk     struct{ Text string }
type ThoughtChunk     struct{ Text string }
type ToolCall         struct{ ID, Title string; Kind ToolKind; Status ToolStatus; Input json.RawMessage }
type ToolCallUpdate   struct{ ID string; Status ToolStatus; Output json.RawMessage; Diff *Diff }
type PlanUpdate       struct{ Entries []PlanEntry }
type PermissionRequest struct{ ToolCall ToolCall; Options []PermissionOption } // answered via callback
type TurnEnded        struct{ StopReason StopReason; Usage Usage; Cost *Cost }

// Optional capabilities — discovered by assertion AND by explicit predicate.
type Steerer     interface{ Steer(ctx context.Context, text string) error }
type Compactor   interface{ Compact(ctx context.Context) error }
type ModelSetter interface{ SetModel(ctx context.Context, model string) error }
type ModeSetter  interface{ SetMode(ctx context.Context, mode string) error }
type Forker      interface{ Fork(ctx context.Context) (Session, error) }
type Rewinder    interface{ Rewind(ctx context.Context, toMessageID string) error }
type Extender    interface{ // raw ACP escape hatch — never lose a vendor feature
    CallExtension(ctx context.Context, method string, params any) (json.RawMessage, error)
}
func Supports(s Session, c Capability) bool

// Structured output — package-level generic (interface methods can never take type params).
func PromptJSON[T any](ctx context.Context, s Session, prompt string, schema *jsonschema.Schema) (T, error)
func PromptInto(ctx context.Context, s Session, prompt string, dst any) error // Scan-style

// Errors: typed sentinels + wrapping, for errors.Is/As.
var ErrUnsupported = errors.New("gimbal: capability not supported by harness")
type StopError struct{ Reason StopReason } // Reason ∈ {Refusal, MaxTokens, MaxTurnRequests}
func (e *StopError) Error() string
type PermissionDenied struct{ Tool string }
```

Rationale mapped to evidence:
- **Session lifecycle:** `Harness.Open` returns a concrete `*session` behind `Session` ("accept interfaces, return structs"). Resume via `SessionOption` (`WithResume(id)`) → Codex `resume`, Claude `--continue`, ACP `session/load`.
- **Prompt/streaming:** `iter.Seq2[Event, error]` (§3a); accumulate final result on the session; `TurnEnded` carries `StopReason`/`Usage`/`Cost`.
- **Structured output:** package-level generic (§3b).
- **Steer/interrupt/compact:** `Interrupt` core; `Steer`/`Compact` optional (§3c) — honest about backend support.
- **Capability discovery:** optional interfaces + `Supports()` to survive wrapping (§3d).
- **Error handling:** typed `StopError`/`PermissionDenied`/`ErrUnsupported` with `errors.Is`/`As`; do **not** expose harness-specific error structs publicly (Cheney: assert behavior, not concrete error types).
- **Context:** first arg everywhere; cancellation maps to ACP `session/cancel`; honor `ctx.Done()` in the reader goroutine.
- **Outer loop, written by a user of gimbal:**
  ```go
  coder := gimbal.Claude().Open(ctx, gimbal.WithCwd(repo))
  reviewer := gimbal.Codex().Open(ctx)
  for i := 0; i < maxIter; i++ {
      var diff string
      for ev, err := range coder.Prompt(ctx, task+lastReview) {
          if err != nil { return err }
          if e, ok := ev.(gimbal.ToolCallUpdate); ok && e.Diff != nil { diff += e.Diff.String() }
          if e, ok := ev.(gimbal.TurnEnded); ok && e.StopReason == gimbal.StopRefusal {
              return &gimbal.StopError{Reason: e.StopReason}
          }
      }
      verdict, err := gimbal.PromptJSON[Review](ctx, reviewer, reviewPrompt(diff), reviewSchema)
      if err != nil { return err }
      if verdict.Approved { break }
      lastReview = verdict.Notes
  }
  ```

## Recommendations

1. **Adopt `coder/acp-go-sdk` as the transport layer now; wrap it, don't expose it.** Only complete typed ACP client in Go; ships Claude/Gemini examples. Pin the version — it's pre-1.0 (v0.13.5) and the `Unstable*` surface will churn.
2. **Ship v1 with the tiny core `Session` interface + `iter.Seq2` streaming + package-level `PromptJSON[T]` + optional capability interfaces.** Implement three harnesses: Gemini (native `--acp`), Claude Code (`claude --acp` adapter), Codex (adapter). Treat Antigravity as experimental (adapters exist; Google's ToS is hostile).
3. **Make `Steer` and `Compact` optional and honest.** Implement `Steer` for Codex via the `turn/steer` extension; elsewhere return `ErrUnsupported` (or document interrupt+resubmit fallback). Do not promise mid-turn steering you can't deliver.
4. **Add `Supports()`/`Capabilities()` explicitly**, not just type assertions — you *will* wrap sessions (tracing, outer loop) and interface erasure will otherwise silently drop capabilities.
5. **Own the outer loop in plain Go** (errgroup + context + explicit termination). No Temporal/LangGraph in v1. Revisit durability only to survive restarts mid-loop — threshold: loops running longer than a CI job or costing more than you're willing to redo.
6. **Preserve vendor features via the `Extender` escape hatch** (`CallExtension`/`_meta`). This is what makes "ACP hides features" moot for anything you're willing to wire through.

**Thresholds that would change these:** if ACP adds first-class steering/compaction/subagent methods (watch `agentclientprotocol/agent-client-protocol` and coder's `Unstable*` surface), promote them from optional interfaces to core. If an official Anthropic or OpenAI **Go** agent SDK ships, re-evaluate build-vs-adopt for that vendor's path. Go 1.27's generic methods do **not** relax the interface-method restriction, so `PromptJSON` stays a package function permanently.

## Caveats

- **The Claude-Code Go SDK space is immature and unofficial** — every option is a solo-maintainer port of the Python SDK, several with explicit "not production tested" warnings. Design references only.
- **ACP is v1 but its extension surface is in flux.** coder/acp-go-sdk's `Unstable*` methods (fork, providers, NES, elicitation) can change without wire-version bumps; gate features on negotiated capabilities, not package/schema versions.
- **Mid-turn steering is thinly supported and thinly documented.** The strongest evidence (Codex `turn/steer`, non-steerable `review`/`compact` turns, exec-mode not supporting it) comes from app-server docs and issue threads, not a stable public spec; I could not locate a more authoritative source than app-server docs/issues. Verify against the Codex version you target.
- **Antigravity via third-party tools may violate Google's ToS** (per Google's FAQ, surfaced in multiple adapter READMEs). Don't make it a supported path without legal review; note MCP-tool isolation for Antigravity subagents is reportedly broken.
- **The `--acp` vs `--experimental-acp` flag for Gemini is genuinely ambiguous** across sources (repo docs now say `--acp`; many integration guides still use `--experimental-acp`); detect/support both.
- **Explicit compaction as a portable API is essentially fictional today** — hooks (Claude), an app-server method (Codex), or automatic. Anything gimbal exposes as `Compact()` is a backend-specific shim.
- A few harness-internal details (exact Antigravity ACP method names, Codex app-server method stability) I could verify only at issue-tracker / adapter-README level; confirm against the edge/latest binaries you run.