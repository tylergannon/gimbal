# Gimble Feature Inventory: Comprehensive Product Capability & Validation Baseline

**Repository Target**: `/Users/tyler/.codex/worktrees/c187/gimble`
**Inspected Revision**: Commit `3db61eb99397fa6e01898b0746529b5fefbd3e14` (matches `origin/main` on 2026-09-19)
**Document Path**: `ephemeral/research/daily-validation/features.md`
**Semantic Index**: [`features-sources/INDEX.md`](features-sources/INDEX.md)
**Validation Baseline**: Daily automated agent validation and regression suites.

Start with the [organized inventory](feature-inventory.md) for the readable product view and additional browser checks. This document was edited after the workflow for retry terminology and the required Go version. Source notes are research material, not live validation.

---

## 1. Overview and Architectural Spine

Gimble is an unadorned Go library and local runtime for authoring, running, observing, and steering multi-agent workflows. In Gimble, "the workflow is Go": high-level workflow wrappers and domain-specific configuration languages are rejected. Workflows use standard Go idioms (loops, branching, error propagation, concurrent groups, and defer/teardown), while Gimble provides hierarchical execution scopes, typed model generation, background service lifecycles, streaming event journals, and an embedded web dashboard.

This inventory documents the feature baseline established by inspecting repository code and tests at commit `3db61eb99397fa6e01898b0746529b5fefbd3e14`, reconciling stale documentation (`docs/web-app.md`, `README.md`) with current production source code.

### 1.1 The Five Feature Groups
1. **CLI and Built-in Workflows**: Subcommands in `cmd/gimble`, decentralized Unix socket discovery, one-shot prompt execution (`run-prompt`), and registered production workflows (`review`, `implement`, `research-document`, `pyramid-summary`).
2. **Browser Runs, Graphs, Details, and Control**: Embedded SvelteKit/Go web app (`skgo`), dual-path state management (SSR snapshot hydration + SSE delta streaming), SVG spine workflow map, transcript inspector, and live controls.
3. **Conversations and Harness Capabilities**: Durable human-agent chat environments with isolated Git worktrees, restart recovery, agent workflow launching, and architectural models across Claude, Codex, and Agy.
4. **Go Workflow Runtime, Context, Loops, and Services**: Engine core in `package gimble` providing hierarchical scopes, single-write context immutability, artifact spilling (>4k / >15k tokens), Go 1.23 finite iteration (`Iterate`), adaptive planner loops (`PromiseLoop`), background services (`Service`), shell commands (`RunCommand`, `Check`), structured generation (`Generate`), non-gating supervision (`WithSupervisor`), and blocking interviews (`Interview`).
5. **Persistence, Observability, Tooling, and Build Interfaces**: Single-writer monotonic `run.jsonl` vs multi-writer `project.jsonl` (with UTC timestamps for post-hoc sorting of concurrent appends), 19 sealed `LifecycleEvent` variants, 8 atomic observation tables, multi-tier post-run replay, hierarchical cost rollup, static AST/SSA linter (`internal/gimblelint`), and code generation pipelines.

### 1.2 Core Reconciliations & Cross-Checks
- **Workflow Launching Surfaces**: The web root `/` lacks a workflow start form (F2 is unimplemented). Workflows launch via CLI (`gimble run <wf>`) or programmatically by conversational chat agents in isolated Git worktrees (`CONV-004`).
- **Graph Visualization vs Pre-Run Browsing**: Standalone pre-run browsing (F13) is absent. However, live and completed runs consume the compiled static `workflow.Graph` (`workflow_gen.go`), rendering unreached steps as unobserved/dashed nodes along the central SVG spine.
- **Non-Gating Supervision (`WithSupervisor`)**: Background supervisors run concurrently on an interval ticker (`WithInterval`, default 3m). Workers never block waiting for review, and supervisor look failures do not abort the worker. Objections are steered into the worker turn as instructions.
- **Validation Standard**: Inspected test files represent proposed, reproducible procedures for daily validation, not proof of live runs during this research pass.

---

## 2. Group 1: CLI and Built-in Workflows

`cmd/gimble` combines process orchestration with administrative tooling. Local runtime discovery is decentralized: each runtime binds a Unix domain socket (`.gimble/<hex>.sock` or `/tmp/gimble-<hex>.sock`) and writes `.gimble/control/<hex>.json`. Clients connect via HTTP-over-UDS with a 2-second dial timeout, bypassing dead process files without checking file age. `run-prompt` inverts provider roles based on caller environment (`CODEX_THREAD_ID` invokes Claude; `CLAUDE_CODE_SESSION_ID` invokes Codex). Built-in workflows embed role bindings from `cmd/gimble/defaults.json`, overridable via CLI flags.

### 2.1 Feature Table: CLI and Built-in Workflows

| Feature ID | Description | Surface | Status & Limits | Concrete Observable End-to-End Check | Sources & Evidence |
|---|---|---|---|---|---|
| **CLI-001** | **Root Server Runtime**: Embedded HTTP server & UDS control socket over project. | CLI | **Implemented**. `--port` (8080), `--uds`, `--no-web`. Loopback only. Blocks until interrupt. | `gimble --port 8888 --no-web`: verifies `.gimble/control/*.json` created; SIGINT unlinks files. | `cmd/gimble/main.go`, `web/control.go` · [T001](features-sources/topic-001/INDEX.md) |
| **CLI-002** | **Static Linter CLI**: Executes AST/SSA workflow rules against Go source. | CLI | **Implemented**. `gimble lint [pkgs]` or `go vet -vettool=./bin/gimble`. Early routing in `main.go`. | `gimble lint ./internal/workflows/review`: exit 0; invalid code reports GIMBLE101–109. | `cmd/gimble/main.go`, `internal/gimblelint` · [T001](features-sources/topic-001/INDEX.md), [T016](features-sources/topic-016/INDEX.md) |
| **CLI-003** | **Active Runtime Discovery**: Lists active Gimble instances in directory. | CLI | **Implemented**. `gimble runs [--work-dir <path>]`. Ignores dead processes via 2s dial timeout (not file age). | `gimble runs`: returns JSON array of live runs; clean directory returns `{"runs":[]}`. | `cmd/gimble/runs.go` · [T001](features-sources/topic-001/INDEX.md) |
| **CLI-004** | **Observation Watcher**: Streams real-time run observation as NDJSON over UDS. | CLI | **Implemented**. `gimble watch [--work-dir <path>] <runID>`. Snapshot line 1, then live deltas. | `gimble watch <id>`: outputs snapshot JSON on line 1, then live SSE deltas until run completes. | `cmd/gimble/runs.go` · [T001](features-sources/topic-001/INDEX.md) |
| **CLI-005** | **Turn & Loop Control**: Injects guidance into active turn or queues for PromiseLoop planner. | CLI | **Implemented**. `gimble steer (--session <id> \| --loop <scope>) <runID> <msg>`. | `gimble steer --session lap.1/coder.1 <id> "m"`: returns `{"landed":true\|false}`. Loop: `{"queued":true}`. | `cmd/gimble/steer.go` · [T001](features-sources/topic-001/INDEX.md) |
| **CLI-006** | **BPE Token Counter**: Computes exact BPE tokens using OpenAI's `o200k_base`. | CLI | **Implemented**. `gimble count-tokens <file>`. Outputs single integer and newline. | `gimble count-tokens cmd/gimble/defaults.json`: exit 0, outputs integer `194` (stable fixture). | `cmd/gimble/count_tokens.go` · [T001](features-sources/topic-001/INDEX.md) |
| **CLI-007** | **One-Shot Prompt Harness**: Runs single prompt turn across harnesses with schema retries. | CLI | **Implemented**. `--model`, `--model-version`, `--effort`, `--output-schema`, `--workdir`, `--timeout`. | `gimble run-prompt --model gpt-5.6-luna "test"`: outputs text; `--output-schema` validates JSON (at most 3 attempts). | `cmd/gimble/run_prompt.go`, `modelalias.go` · [T002](features-sources/topic-002/INDEX.md) |
| **CLI-008** | **Code Review Workflow**: Read-only single-turn review returning structured findings. | CLI, Library | **Implemented**. `gimble run review --goal "<str>"`. Role: `code-review` (`gpt-5.6-luna`). | `gimble run review --goal "audit"`: non-empty goal required, no files changed, `Result` in log. | `internal/workflows/review`, `workflows.go` · [T003](features-sources/topic-003/INDEX.md) |
| **CLI-009** | **Bounded Implementation**: Goal-seeking implementation loop pairing planner with critique/QA. | CLI, Library | **Implemented**. `gimble run implement --promise "<str>" --definition-of-done-file <path> --max-tasks <n>`. | Run without DoD file: fails immediately. With DoD: verifies `PromiseLoop`, coding, `Check`, QA. | `internal/workflows/implementation` · [T003](features-sources/topic-003/INDEX.md) |
| **CLI-010** | **Research Document Workflow**: Multi-agent research planning, indexing, curation, and authoring. | CLI, Library | **Implemented**. `gimble run research-document --goal "<str>" --research-dir <p> --output <p> --token-budget <n>`. | Run with budget 0: fails synchronously. Valid args: verifies 5 parallel indexers, curation, drafting. | `internal/workflows/researchdocument` · [T003](features-sources/topic-003/INDEX.md) |
| **CLI-011** | **Pyramid Summary Workflow**: Multi-document summary compression down to 100-token abstracts. | CLI, Library | **Implemented**. `gimble run pyramid-summary --goal "<str>" --semantic-index <p> --largest-document <p> --output-dir <p>`. | Run with doc > budget: fails preflight. Valid args: verifies halving schedule, review, repair waves. | `internal/workflows/pyramidsummary` · [T003](features-sources/topic-003/INDEX.md) |

---

## 3. Group 2: Browser Runs, Graphs, Details, and Control

The web interface is a SvelteKit app served by Go via `skgo`. It renders live and historical runs, mapping hierarchical scopes into paper sheets along a central SVG spine.

- **Initial SSR Hydration**: `page.server.go` reads `observation.Registry.Snapshot(runID)`. The JSON snapshot is passed through SvelteKit's `transport` hook (`hooks.RunSnapshot`) and decoded via `JSON.parse` into `RunObservation`, ensuring instant first paint without client fetch waterfalls.
- **Live SSE Streaming**: If `run.status === "running"`, an `EventSource` connects to `/api/runs/{runID}/events?stream={stream}&position={position}`. The server flushes missed frames, then streams `delta` events (`row`, `totals`, `event`). Buffer exhaustion triggers a full `snapshot` frame. Disconnections initiate 250ms exponential backoff.

### 3.1 Feature Table: Browser Runs, Graphs, Details, and Control

| Feature ID | Description | Surface | Status & Limits | Concrete Observable End-to-End Check | Sources & Evidence |
|---|---|---|---|---|---|
| **WEB-001** | **Runs Listing & Attention (F1)**: Runs dashboard with status filters, search, and interview attention. | Browser | **Implemented**. Route `/` (`RunsList.svelte`). Filters (`All`, `Active`, etc.), search bar, 2s periodic poll. | Load `/`: assert `section.runs-list`, filters `div.filters`, and cards `button.run-card`. Pending shows `section.attention`. | `web/src/lib/run/RunsList.svelte` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-002** | **Run Graph & Spine (F3)**: Tree rendering scopes as alternating sheets along central SVG spine. | Browser | **Implemented**. `/runs/[runID]` (`Map.svelte`). Requires compiled `workflow.Graph`. Zoom/pan controls. | Load run: inspect `div.map`, nested `section.sheet[data-scope]`, and nodes `[data-selection-key]`. Zoom scales map. | `web/src/lib/run/Map.svelte`, `layout.ts` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-003** | **Scope Detail Inspector (F4)**: Sidebar for selected scope showing prompt, values, and spilled files. | Browser | **Implemented**. `DetailPane.svelte`. Renders scope key, task prompt, values written, spilled previews. | Click scope sheet: assert `aside.detail` opens with scope header, prompt, and values in `div.value code`. | `web/src/lib/run/DetailPane.svelte` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-004** | **Session Card Attribution (F5)**: Session metadata inspector (ID, adapter, model, role, code line). | Browser | **Implemented**. Inside `DetailPane`. Presenting sessions as attributes of their declaring scope is an intentional design choice. | Select turn node: assert `DetailPane` displays session ID, adapter (`claude`, `codex`, `agy`), role, model, code line. | `web/src/lib/run/DetailPane.svelte` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-005** | **Turn Inspector & Watchers (F6)**: Deep inspector for prompt text, supervisor watchers, and model calls. | Browser | **Implemented**. `DetailPane.svelte`. Disclosures for prompt, watcher status, last 4 calls, token totals. | Select active turn: inspect disclosures `"Prompt sent"`, `"Watchers"`, `"Usage"`. Verify token counts and code reference. | `web/src/lib/run/DetailPane.svelte` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-006** | **Transcript Timeline (F7)**: Native session transcript with reasoning blocks and tool executions. | Browser | **Implemented**. `SessionTimeline.svelte`. Reasoning disclosure, tool blocks, token footers (no dollars). | Inspect `section.native-session`: assert `article.assistant` has reasoning disclosure, tool blocks, and token footers. | `web/src/lib/run/SessionTimeline.svelte` · [T004](features-sources/topic-004/INDEX.md) |
| **WEB-007** | **Interactive Turn Steer**: Form in detail pane allowing operators to steer active turns. | Browser, Library | **Implemented**. Remote `steer` in `steer.remote.go`. Protected by 30s timeout via `WithoutCancel`. | On active turn, type in `footer.detail-foot textarea` and submit. Active shows `"Sent"`; ended shows error banner. | `web/src/routes/steer.remote.go` · [T005](features-sources/topic-005/INDEX.md) |
| **WEB-008** | **Loop Steer & Wrap-Up**: Guidance injection and wrap-up signaling for running PromiseLoop scopes. | Browser, Library | **Implemented**. Remote `steerLoop` in `steer.remote.go`. Accepts text or `gimble.WrapUp` button. | On running loop, click "Wrap up" or type guidance and submit: assert banner `"Message is waiting..."`; log records steer. | `web/src/routes/steer.remote.go` · [T005](features-sources/topic-005/INDEX.md) |
| **WEB-009** | **Interview Answering**: Human response interface for workflows blocking on `gimble.Interview`. | Browser, Library | **Implemented**. Remote `answerInterview` in `interview.remote.go`. Empty answer ends interview. | Launch interview: node shows blue waiting pip. Submit answer in `textarea#answer`: question unblocks. | `web/src/routes/interview.remote.go` · [T005](features-sources/topic-005/INDEX.md) |
| **WEB-010** | **Turn Stop & Run Cancel**: Operator actions to abort individual running turn or cancel entire workflow. | Browser, Library | **Implemented**. Remote `stopTurn` and `cancelRun` in `control.remote.go`. Scope cancel barred. | Click "Stop turn": turn ends with `Killed` while siblings run. Click "Cancel run": root scope context cancels with cause. | `web/src/routes/control.remote.go` · [T005](features-sources/topic-005/INDEX.md) |

---

## 4. Group 3: Conversations and Harness Capabilities

Gimble provides durable human-agent chat environments under `/conversations` managed by `internal/conversation/Manager`, alongside model harness adapters (`claude`, `codex`, `agy`).

- **Claude (`claude`)**: Subprocess via `claude-agent-sdk-go`. Uses `--session-id` or `--resume`. Settings isolated (`setting-sources: "project,local"`). Enforces `CLAUDE_CODE_AUTO_COMPACT_WINDOW="256000"` to prevent compaction thrashing. Non-resumable across restarts.
- **Codex (`codex`)**: Persistent JSON-RPC over WebSocket to shared `codex app-server` daemon. Implements `persistentSessionAdapter` (`ResumeSession`/`DetachSession`). Survives restarts. Guards against `codex-cli 0.153.4` ghost thread leaks by checking rollout paths.
- **Agy (`agy`)**: Headless print mode CLI (`agy -p ... --output-format stream-json`). Forking unsupported. Steer sends SIGINT and resumes turn with steer prompt.

### 4.1 Feature Table: Conversations and Harness Capabilities

| Feature ID | Description | Surface | Status & Limits | Concrete Observable End-to-End Check | Sources & Evidence |
|---|---|---|---|---|---|
| **CONV-001** | **Worktree Isolation**: Mints 32-char ID, creates branch `gimble/conversation-<id[:12]>` & worktree. | Browser, Library | **Implemented**. `/conversations` (`Manager.Create`). Worktrees at `<repo>/conversation-worktrees/<id>`. | Create conversation: assert worktree exists and branch is checked out in `git worktree list`. Turns isolated. | `internal/conversation/manager.go` · [T007](features-sources/topic-007/INDEX.md) |
| **CONV-002** | **Atomic State Persistence**: Saves metadata, messages, and runs to `.gimble/conversations/<id>.json`. | Library | **Implemented**. Atomic write via temp file rename. Tracks timestamps, provider, model, messages, runs. | Send message in chat: assert atomic update, valid JSON, message list, millisecond timestamps in `<id>.json`. | `internal/conversation/manager.go` · [T007](features-sources/topic-007/INDEX.md) |
| **CONV-003** | **Restart Recovery**: Recovers conversation state on reboot without contacting providers or Git. | Library | **Implemented**. In-flight `working` -> `idle`; unfinished runs -> `error`. Lazy reconnect for Codex; Claude/Agy non-live. | Kill server during turn; restart: status is `idle`. Codex marks `Live == true`; Claude/Agy mark `Live == false`. | `internal/conversation/manager.go` · [T007](features-sources/topic-007/INDEX.md) |
| **CONV-004** | **Agent Workflow Launching**: Chat agents trigger `review` or `implement` workflows in worktree. | Browser, Library | **Implemented**. Structured JSON `conversationReplySchema`. Monitored via `watchRun`; status saved to JSON. | Ask agent to implement feature with DoD: agent outputs structured JSON; workflow starts and links in UI. | `internal/conversation`, `web/runtime.go` · [T007](features-sources/topic-007/INDEX.md) |
| **HARN-001** | **Claude Agent SDK Adapter**: Subprocess adapter orchestrating Claude Code CLI with strict isolation. | Library | **Implemented**. Package `claude`. Enforces `CLAUDE_CODE_AUTO_COMPACT_WINDOW="256000"`. Non-resumable. | Run turn with Claude: verify CLI subprocess invocation with `--session-id`. Interrupt turn: assert clean exit. | `claude/claude.go` · [T008](features-sources/topic-008/INDEX.md) |
| **HARN-002** | **Codex Daemon Adapter**: Persistent adapter connecting via WebSocket to shared `codex app-server`. | Library | **Implemented**. Package `codex`. Supports `ResumeSession`/`DetachSession` and thread forking. | Run turn with Codex: inspect frames in `GIMBLE_CODEX_DEBUG_DIR`. Verify `thread/start`, `turn/start`, `archive`. | `codex/codex.go` · [T008](features-sources/topic-008/INDEX.md), [T009](features-sources/topic-009/INDEX.md) |
| **HARN-003** | **Google Antigravity Adapter**: Headless adapter invoking `agy` CLI in print mode (`stream-json`). | Library | **Implemented**. Package `agy`. Fork unsupported. Steer interrupts child process via SIGINT and resumes. | Run turn with Agy: verify NDJSON scanning (`init`, `step`, `result`). Calling `Fork` returns explicit error. | `agy/agy.go` · [T008](features-sources/topic-008/INDEX.md) |
| **HARN-004** | **Model Alias & Effort Resolution**: Universal resolution of aliases (`gpt`, `flash`, `fable`) and efforts. | Library | **Implemented**. Package `internal/modelalias`. Resolves aliases to native IDs and checks provider support. | Resolve `"gpt"` -> `gpt-5.6-sol:high`. Resolve `"flash"` -> `gemini-3.8-flash-medium`. Reject `xhigh` for Gemini. | `internal/modelalias` · [T008](features-sources/topic-008/INDEX.md) |
| **HARN-005** | **Codex Session Resume**: Detaches daemon thread on shutdown; resumes lazily on restart without history loss. | Library | **Implemented**. Interface `persistentSessionAdapter`. Detach uses `thread/unsubscribe`; resume uses `thread/resume`. | Start Codex chat, restart server, send follow-up: assert agent remembers context without re-injecting full history. | `codex/codex.go` · [T009](features-sources/topic-009/INDEX.md) |
| **HARN-006** | **Session Forking**: Clones conversation history into independent session via CLI flag or daemon RPC. | Library | **Partial**. Supported on Claude (`--fork-session`) and Codex (`thread/fork`). Explicitly unsupported on Agy print mode. | In Go test, call `session.Fork`. Run turn on child session; assert parent unchanged. Forking archived thread fails. | `claude/claude.go`, `codex/codex.go` · [T009](features-sources/topic-009/INDEX.md) |
| **HARN-007** | **In-Flight Turn Steer**: Injects instructions into active turns; distinguishes landed from dropped. | Library | **Implemented**. Via SDK stream (Claude), `turn/steer` (Codex), or SIGINT restart (Agy). Records `Steer` event. | Steer while model generates: lands and acknowledged. Steer after turn concludes: `landed: false` returned. | `session.go` · [T009](features-sources/topic-009/INDEX.md) |

---

## 5. Group 4: Go Workflow Runtime, Context, Loops, and Services

The engine (`package gimble`) provides pure Go runtime primitives, guaranteeing single-write immutability, hierarchical cancellation, automatic artifact spilling, process-group service supervision, and typed generation.

- **Scope Immutability & Shadowing**: A key can be written only once per scope (`Set`, `SetJSON`, `Check`). Duplicate writes panic. Modifying a value is permitted strictly via **shadowing in a child scope**.
- **Artifact Spilling**: Values >4,000 tokens (`contextEntryTokenLimit`) are written to `artifacts/values/<scope>/<key>.txt` with a 32 KiB preview. If aggregate prompt context exceeds 15,000 tokens (`contextTokenLimit`), remaining inline values spill, and binary search fits previews.
- **Service Lifetime Invariant**: A `Service` must remain running for that scope's lifetime. Premature exit (even code 0) is a fatal crash that cancels the scope context. Teardown sends `SIGTERM` to `-pid` (5s poll), escalating to `SIGKILL` (5s wait).
- **Non-Gating Supervision (`WithSupervisor`)**: Reviewers run in a background goroutine on a ticker (`WithInterval`, default 3m). Bounded lookups (`t.look`) steer objections into the worker. The worker **never blocks** waiting for supervision, and supervisor errors do not abort the worker.

### 5.1 Feature Table: Go Workflow Runtime, Context, Loops, and Services

| Feature ID | Description | Surface | Status & Limits | Concrete Observable End-to-End Check | Sources & Evidence |
|---|---|---|---|---|---|
| **RT-001** | **Scope Tree & Ordinals**: Constructs nested scopes with stable keys (e.g. `lap.3/bakeoff.1`). | Library *(Go workflow)* | **Implemented**. `gimble.Run`, `gimble.Scope`. Cancellation cascades to children; teardown cleans resources. | Write workflow nesting `Scope("sub", ...)`: verify keys match ordinal `sub.1`; cancelling parent cancels child. | `scope.go` · [T010](features-sources/topic-010/INDEX.md) |
| **RT-002** | **Context Immutability**: Single-write invariant per key within scope; child shadowing supported. | Library *(Go workflow)* | **Implemented**. `gimble.Set`, `gimble.SetJSON`. Duplicate write panics. Child shadowing succeeds. | Call `Set(ctx, "k", 1)` twice in scope: assert panic. Call `Set(childCtx, "k", 2)` in child: succeeds cleanly. | `scope.go` · [T010](features-sources/topic-010/INDEX.md) |
| **RT-003** | **Artifact Spilling**: Spills values >4,000 tokens (entry) or >15,000 tokens (prompt) to disk. | Library *(Go workflow)* | **Implemented**. Tokenizer: `o200k_base`. Writes `artifacts/values/<scope>/<key>.txt`. `ValueSet` emits `Artifact`. | Store 5,000-token string with `Set`: assert file exists in `artifacts/values/` and prompt receives excerpt preview. | `artifact.go` · [T010](features-sources/topic-010/INDEX.md) |
| **RT-004** | **Concurrent Groups**: Parallel goroutines (`group.Go`) with first-error cancellation and panic recovery. | Library *(Go workflow)* | **Implemented**. `gimble.Group`, `group.Go`, `group.Wait`. First error cancels siblings (operator `Killed` exempted). | Run two workers in group; fail one: assert sibling cancelled, panic converted to error, `Wait` returns error. | `group.go` · [T010](features-sources/topic-010/INDEX.md) |
| **RT-005** | **Finite Iteration**: Go 1.23 iterator (`iter.Seq2`) executing each item in an isolated child scope. | Library *(Go workflow)* | **Implemented**. `gimble.Iterate[T]`. Lap sessions/services closed before next item. Service crash halts loop. | Range over `Iterate(ctx, "lap", items)`: start session in lap and assert session closes on each iteration step. | `iterate.go` · [T011](features-sources/topic-011/INDEX.md) |
| **RT-006** | **PromiseLoop & Backlog**: Goal-seeking loop with planner backlog evolution and 3 schema retries. | Library *(Go workflow)* | **Implemented**. `gimble.PromiseLoop`. Backlog saved to `<runDir>/scopes/<key>/backlog.md`. | Run `PromiseLoop`: assert planner writes `Task` backlog; run task with `Check` and assert check appears in next prompt. | `loop.go` · [T011](features-sources/topic-011/INDEX.md) |
| **RT-007** | **Scope Background Services**: Background process groups with lifetime invariant and 2-stage shutdown. | Library *(Go workflow)* | **Implemented**. `gimble.Service`. Uses `zsh -c`, `Setpgid: true`. Premature exit fails scope. SIGTERM/SIGKILL. | Start service exiting early with 0: assert scope context cancelled. Long service: SIGTERM terminates group on exit. | `service.go` · [T012](features-sources/topic-012/INDEX.md) |
| **RT-008** | **Command Execution**: Runs shell command, streaming logs to disk with 64 KiB head/tail excerpting. | Library *(Go workflow)* | **Implemented**. `gimble.RunCommand`. Logs saved to `artifacts/commands/<id>/`. Memory string capped at 64 KiB. | Run command generating 100 KiB: assert full output saved to disk logs, and memory return string has 64 KiB excerpt. | `command.go` · [T012](features-sources/topic-012/INDEX.md) |
| **RT-009** | **Empirical Test Checking**: Runs test command and records structured JSON evidence under unique scope key. | Library *(Go workflow)* | **Implemented**. `gimble.Check`. Stores `exit_code`, `stdout`, `stderr` in context. Nonzero returns `nil`. | Run `Check(ctx, "tests.1", "", "sh", "-c", "exit 1")`: returns `nil`. Context has exit 1. Re-using key panics. | `command.go` · [T012](features-sources/topic-012/INDEX.md) |
| **RT-010** | **Typed Generation**: Model generation validated against schema with at most 3 attempts. | Library *(Go workflow)* | **Implemented**. `Session.Generate[T Output]`. Prose uses `gimble.Text`. Custom templates via `WithScopeTemplate`. | Call `Generate[MyType]` with mock returning invalid JSON: verify engine makes at most 3 attempts, showing the validation error before each retry. | `session.go` · [T013](features-sources/topic-013/INDEX.md) |
| **RT-011** | **Non-Gating Supervision**: Reviewer watching worker turn on ticker interval without blocking worker. | Library *(Go workflow)* | **Implemented**. `WithSupervisor(session, inst, WithInterval(3m))`. Non-gating: worker never blocks; errors don't abort. | Attach supervisor to slow turn: supervisor runs in background on ticker, steering objections. Worker unblocked. | `supervise.go`, `session.go` · [T013](features-sources/topic-013/INDEX.md) |
| **RT-012** | **Interactive Interview**: Multi-turn dialogue blocking on live operator answers via ULID questions. | Library *(Go workflow)* | **Implemented**. `gimble.Interview`. Emits `InterviewQuestionAsked` with ULID; blocks on channel. Empty ends. | Run `Interview` in goroutine. Main thread polls question, calls `AnswerInterview(id, "ans")`: dialogue unblocks. | `interview.go` · [T013](features-sources/topic-013/INDEX.md) |

---

## 6. Group 5: Persistence, Observability, Tooling, and Build Interfaces

Gimble features a multi-tiered persistence architecture for zero data loss, instant post-hoc replay without workflow code execution, and compile-time static analysis enforcement.

- **`run.jsonl` (Single Writer)**: Dedicated per run. Sequence numbers (`Seq`) are strictly monotonic and gap-free (`w.seq++`). Closed by a sealed `Complete` record.
- **`project.jsonl` (Multi-Writer)**: Shared project log written concurrently across runs using `O_APPEND`. Emitted with `Seq = 0`. Because concurrent appends do not guarantee monotonic line order, records carry UTC timestamps to support post-hoc sorting by consumers.
- **19 Sealed `LifecycleEvent` Variants**: `RunStarted`, `RunEnded`, `RunCancelled`, `ScopeBegan`, `ScopeEnded`, `PlannerDecision`, `ValueSet`, `CommandStarted`, `CommandEnded`, `Steer` (covers landed/dropped via `landed: bool`), `InterviewQuestionAsked`, `InterviewQuestionAnswered`, `SessionCreated`, `SessionClosed`, `TurnStarted`, `TurnEnded`, `SuperviseAttached`, `Killed`, `Complete`.
- **Durable Post-Run Replay Cascade**: Reconstructs past runs without live runtime objects: Checkpoint fast-path (`observation.json`) -> 8 Tables fast-path -> `run.jsonl` line rebuild -> Session transcript reconstruction.

### 6.1 Feature Table: Persistence, Observability, Tooling, and Build Interfaces

| Feature ID | Description | Surface | Status & Limits | Concrete Observable End-to-End Check | Sources & Evidence |
|---|---|---|---|---|---|
| **PERS-001** | **Monotonic Run Journal**: Append-only JSONL log with gap-free sequence numbers for each run. | Library | **Implemented**. `.gimble/runs/<runID>/run.jsonl`. Monotonic `Seq`. Closed by `Complete` record. | Inspect `run.jsonl`: verify `seq` increments strictly by 1 on every line with no gaps, ending with `kind: "complete"`. | `event_persistence.go` · [T014](features-sources/topic-014/INDEX.md) |
| **PERS-002** | **Multi-Writer Project Journal**: Shared milestone log written concurrently via `O_APPEND` with `Seq = 0`. | Library | **Implemented**. `.gimble/project.jsonl`. Appended concurrently; UTC timestamps enable post-hoc sorting. | Run two concurrent workflows: inspect `project.jsonl` and assert records appear with `seq: 0` and sortable timestamps. | `event_persistence.go`, `events.go` · [T014](features-sources/topic-014/INDEX.md) |
| **PERS-003** | **Sealed Lifecycle Schemas**: Polytype sealed union representing 19 core lifecycle events. | Library | **Implemented**. Sealed union `LifecycleEvent` with discriminator `"kind"`. TypeScript definitions generated. | Inspect `jsonschema/LifecycleRecord.json`: verify 19 variants defined; `Steer` encapsulates landed/dropped. | `events.go`, `schema.go` · [T014](features-sources/topic-014/INDEX.md) |
| **PERS-004** | **Segregated Session Logs**: Isolates raw LLM deltas, tool calls, and provider refs from lifecycle logs. | Library | **Implemented**. Path: `.gimble/runs/<id>/sessions/<session>.jsonl`. Wrapped in `AgentRecord`. | Verify `run.jsonl` contains zero LLM text deltas. Inspect `sessions/*.jsonl`: assert raw tokens segregated. | `event_persistence.go` · [T014](features-sources/topic-014/INDEX.md) |
| **OBS-001** | **In-Memory Observation Tables**: Reduces event stream into 8 atomic JSON tables for current state. | Library | **Implemented**. Tables: `run.json`, `scopes.json`, `sessions.json`, `interviews.json`, `turns.json`, etc. | Check `.gimble/runs/<runID>/`: verify all 8 JSON files exist on disk and are updated atomically via temp file rename. | `internal/observation/store.go` · [T015](features-sources/topic-015/INDEX.md) |
| **OBS-002** | **SSE Deltas & Checkpointing**: Broadcasts table changes over SSE; writes checkpoint every 64 deltas. | Library, Browser | **Implemented**. Emits SSE frames: `row`, `totals`, `event`. Checkpoint saved to `observation.json` every 64 deltas. | Connect to `/api/runs/{id}/events`: assert contiguous position counters; verify `observation.json` saved after 64 deltas. | `internal/observation/store.go` · [T015](features-sources/topic-015/INDEX.md) |
| **OBS-003** | **Durable Post-Run Replay**: Reconstructs complete run snapshots from disk without running workflow code. | Library, Browser | **Implemented**. Cascade: Checkpoint fast-path -> 8 Tables fast-path -> `run.jsonl` rebuild -> Transcript rebuild. | Delete `observation.json` from completed run: load in UI and assert replay reconstructs state and saves new checkpoint. | `internal/observation/replay.go` · [T015](features-sources/topic-015/INDEX.md) |
| **OBS-004** | **Scope Cost Rollup**: Computes exact token usage and USD costs across scope hierarchies. | Library, Browser | **Implemented**. Scope prefix matching (`contains(scope, turnScope)`). Reasoning billed at output rate. | Run workflow with nested scopes: verify parent scope token totals include sum of children; verify catalog pricing. | `internal/observation/usage.go` · [T015](features-sources/topic-015/INDEX.md) |
| **LINT-001** | **Static Workflow Linter**: AST/SSA static analysis engine enforcing authoring invariants and safety. | CLI | **Implemented**. Rules: GIMBLE101 (dynamic workers), GIMBLE102 (constant keys), GIMBLE103 (duplicate writes), GIMBLE104–109. | Run `go test ./internal/gimblelint`: verifies `analysistest.Run` detects all 9 violations and verifies `cmd/gimble` exemption. | `internal/gimblelint/analyzer.go` · [T016](features-sources/topic-016/INDEX.md) |
| **BUILD-001** | **Polytype Schema Generator**: Generates JSON schemas, validation functions, and TypeScript types. | Tooling | **Implemented**. `go tool polytype`. Generates embedded schemas (`jsonschema_gen.go`) and frontend typings. | Run `go generate ./...`: verify `workflow/polytype_gen.go` and `web/src/lib/workflow/` TypeScript files regenerated cleanly. | `Justfile`, `workflow/graph.go` · [T016](features-sources/topic-016/INDEX.md) |
| **BUILD-002** | **Static Graph Generator**: Analyzes workflow source code and emits static compiled execution graphs. | Tooling | **Implemented**. `internal/generate/gimblegen`. Emits `workflow_gen.go` containing `workflow.Graph` for map. | Run `go generate ./internal/workflows/...`: verify `workflow_gen.go` generated with declared nodes, scopes, watchers. | `Justfile`, `internal/generate` · [T016](features-sources/topic-016/INDEX.md) |
| **BUILD-003** | **Type-Safe RPC Generator**: Scans `web/src/routes/*.remote.go` and generates Go/TypeScript RPC bindings. | Tooling | **Implemented**. `go tool skgo`. Emits `skgo_remotes_gen.go`, `skgo_bindings_gen.go`, and `skgo_devalue_gen.go`. | Run `go generate ./internal/skgo`: verify RPC remote wrappers and SvelteKit route bindings are updated. | `internal/skgo/config.go` · [T016](features-sources/topic-016/INDEX.md) |
| **BUILD-004** | **Unified Build Pipeline**: Justfile orchestrating Go compilation, pnpm dependencies, and bundling. | Tooling | **Implemented**. Recipe `just build`: `tidy -> pnpm install -> go generate -> vp fmt -> vp build -> go build`. | Run `just build`: verifies clean end-to-end compilation producing `./bin/gimble` with embedded assets and zero errors. | `Justfile` · [T016](features-sources/topic-016/INDEX.md) |

---

## 7. Cross-Cutting Execution Matrix & Verification Environments

### 7.1 Execution Surface Classification
1. **Direct CLI / Browser Frontends**: Invokable from a terminal or web dashboard (`CLI-001`–`CLI-007`, `WEB-001`–`WEB-010`, `LINT-001`, `BUILD-001`–`BUILD-004`).
2. **Conversation-Launched Workflows**: Production workflows (`review`, `implement`) executed via CLI (`gimble run`) **or** triggered by chat agents in isolated Git worktrees (`CONV-004`).
3. **Library-Only Features (Requiring a Small Go Workflow)**: Core engine behaviors in Go code that cannot be exercised via generic CLI flags or web buttons alone (`RT-001`–`RT-012`). Exercising these requires writing a small Go test workflow (e.g. using `gimble.Run` and `t.TempDir()`).

### 7.2 Verification Environments & Prerequisites

- **Core CLI Commands (`CLI-001`–`CLI-006`)**: 100% offline local execution. Requires only a local filesystem directory with `.gimble/control`. Non-blocking; `watch` connects over UDS.
- **Harness Prompt Execution (`CLI-007`)**: Requires provider CLI binary (`claude`, `codex`, `agy`) on PATH and active credentials. Runs in ephemeral workspace.
- **Built-in Workflows (`CLI-008`–`CLI-011`)**: Requires provider credentials mapped in `cmd/gimble/defaults.json`. Requires Git checkout; DoD file or document inputs.
- **Web Observability (`WEB-001`–`WEB-006`)**: Offline for viewing past runs; provider active for live streaming. Runs over loopback TCP or UDS.
- **Live Web Control (`WEB-007`–`WEB-010`)**: Requires active workflow execution in local runtime. Involves human interaction: steer text, wrap-up button, interview responses, or cancellation.
- **Durable Conversations (`CONV-001`–`CONV-004`)**: Provider credentials per conversation. Creates linked Git worktrees under `conversation-worktrees/<id>`.
- **Harness Adapters (`HARN-001`–`HARN-007`)**: Installed CLI binaries; Codex requires running daemon. Supports steering, interrupts, and Codex session resumption.
- **Workflow Runtime (`RT-001`–`RT-012`)**: Offline for scopes, services, checks; credentials for Generate/Interview. Exercised via pure Go test binaries. Supports goroutines (`Group.Go`), background services (`zsh`), non-gating supervisors.
- **Persistence & Replay (`PERS-001`–`OBS-004`)**: Offline local log reading and JSON serialization. Concurrency-safe single-writer run logs; multi-writer project log.
- **Static Linter & Codegen (`LINT-001`–`BUILD-004`)**: Offline compiler/AST tools. Prerequisites: Go 1.27.1 (as declared by go.mod), Node 22, pnpm, Just, Vite-plus (`vp`).

---

## 8. Deliberate Exclusions, Structural Gaps, and Architectural Evolutions

### 8.1 Unimplemented Features from `docs/web-app.md`
1. **F2: Start a Workflow Form**: Unimplemented. Web root `/` contains no initiation form; workflows launch exclusively via CLI or chat agents.
2. **F8: Loop Backlog and Decisions Scrubbing**: Unimplemented. `DetailPane.svelte` displays current task and decisions, but lacks historical timeline scrubbing for past backlog iterations.
3. **F10: Visual Directed Graph Edges**: Unimplemented. Selection paths highlight via CSS borders and shadows, but arbitrary dynamic directed edge lines connecting separate graph nodes are not drawn on the map canvas.
4. **F12: Prompts Before They Run**: Unimplemented. Dry-run prompt pre-rendering is not exposed in the web UI; only executed turns record prompts.
5. **F13: The Template Static Pass (Standalone Pre-Run Browsing)**: Unimplemented as a standalone tool. However, active and completed runs consume the compiled `workflow.Graph`, rendering declared future steps as unobserved/dashed nodes along the central SVG spine.

### 8.2 Explicitly Documented Deliberate Exclusions
`docs/web-app.md` explicitly rejects several common features:
1. **Accounts, Login, and Roles**: Gimble serves strictly on loopback (TCP `127.0.0.1` or UDS) without auth or RBAC.
2. **Per-Message Dollars (#152)**: Costs are computed only at turn, session, scope, and run totals. Individual transcript rows display exact token counts but intentionally omit dollar amounts.
3. **Scope-Level Cancellation**: Operators can cancel an entire run (`CancelRun` targeting root scope `""`) or stop an individual turn (`StopTurn`). Cancelling an arbitrary intermediate scope (e.g. one branch of a concurrent `Group`) is barred to prevent corrupted workflow states.
4. **Context-Window Meters, Quota Gauges, Raw Payload Retention**: Declined in #173 to avoid coupling UI state to transient provider API quirks.
5. **Browser-Based Workflow Authoring**: Workflows are plain Go programs compiled and tested with standard Go toolchains; no web-based visual builder exists.
6. **Multi-Project Dashboards**: Follows "One runtime, one project directory, one page" without multi-tenant workspace switchers.

### 8.3 Key Architectural Evolutions
- **Retirement of HistoryLanes (`History.html`)**: Earlier designs specified a fallback lane-based timeline (`HistoryLanes.svelte`) for past runs lacking a compiled workflow graph. In commit `ca82875`, `HistoryLanes.svelte` was deleted. Gimble now strictly requires a matching generated `workflow.Graph` (`workflow_gen.go`). If missing, `SmallStates.svelte` renders a prominent corrective state instructing the user to run `go generate ./...` and `just build`.
- **Top Navigation Expansion**: `web/src/routes/+layout.svelte` mounts a persistent top navigation bar with three tabs: `Conversations`, `Runs`, and `About`. Navigating into a run workspace hides site navigation to maximize screen real estate.
- **Unified Detail Pane**: `DetailPane.svelte` combines turn metadata, transcript streaming, watcher status, command logs, context values, interview forms, and steering controls into a single 400px inspector.

---

## 9. Feature Inventory Summary & Statistics

### 9.1 Quantitative Breakdown

#### Table 1: Inventory Capabilities (Actual Inventory Rows by Group)

| Feature Group | Inventory Rows | Implemented | Partial | Key Surfaces |
|---|---|---|---|---|
| **Group 1: CLI & Built-in Workflows** | 11 | 11 | 0 | CLI, Library |
| **Group 2: Browser Runs, Graphs & Control** | 10 | 10 | 0 | Browser, SSE, Web RPC |
| **Group 3: Conversations & Harnesses** | 11 | 10 | 1 (HARN-006: Agy fork unsupported) | Browser, Library, Harness |
| **Group 4: Go Workflow Runtime** | 12 | 12 | 0 | Pure Go Library |
| **Group 5: Persistence, Observability & Tooling**| 13 | 13 | 0 | CLI, Library, Tooling |
| **Total Inventory Capabilities** | **57** | **56** | **1** | All System Surfaces |

#### Table 2: Planned / Unimplemented Features & Design Gaps (Detailed in Section 8)

| Category | Count | Features / Exclusions | Detailed Reference |
|---|---|---|---|
| **Unimplemented Design Features (docs/web-app.md)** | 5 | F2 (start form), F8 (backlog scrubber), F10 (graph edges), F12 (prompt dry-run), F13 (pre-run browser) | Section 8.1 |
| **Deliberate Design Exclusions** | 6 | No auth on loopback, no per-msg $, no arbitrary scope cancel, no quota meters, no web authoring, no multi-project | Section 8.2 |
| **Retired Architectural Components** | 1 | `HistoryLanes.svelte` deleted in favor of mandatory static graph | Section 8.3 |

### 9.2 Evidence Mapping & Research Index Directory
All primary sources (63 files) and detailed evidence clips (44 files) are cataloged under [`features-sources/INDEX.md`](features-sources/INDEX.md) across all 16 research topics:
- **CLI & Workflows (Topics 001–003)**: [`topic-001`](features-sources/topic-001/INDEX.md) (CLI subcommands, socket discovery, observable checks) · [`topic-002`](features-sources/topic-002/INDEX.md) (run-prompt, model/effort resolution, schema retries) · [`topic-003`](features-sources/topic-003/INDEX.md) (workflow CLI flags, execution steps, DoD preflight).
- **Browser Observability & Control (Topics 004–006)**: [`topic-004`](features-sources/topic-004/INDEX.md) (SSR hydration, SSE streaming, DOM assertions) · [`topic-005`](features-sources/topic-005/INDEX.md) (remote functions, steer feedback, interview answering) · [`topic-006`](features-sources/topic-006/INDEX.md) (unimplemented features, deliberate exclusions, HistoryLanes removal).
- **Conversations & Harnesses (Topics 007–009)**: [`topic-007`](features-sources/topic-007/INDEX.md) (worktree isolation, restart recovery, agent workflow launches) · [`topic-008`](features-sources/topic-008/INDEX.md) (subprocess vs daemon vs print mode, 256k compaction window) · [`topic-009`](features-sources/topic-009/INDEX.md) (Codex resume/detach, turn steer timing, ghost thread prevention).
- **Workflow Runtime & Engine (Topics 010–013)**: [`topic-010`](features-sources/topic-010/INDEX.md) (scope hierarchy, immutability, artifact spilling, groups) · [`topic-011`](features-sources/topic-011/INDEX.md) (iter.Seq2 finite iteration, PromiseLoop backlog dispatch) · [`topic-012`](features-sources/topic-012/INDEX.md) (Service process groups, lifetime invariant, RunCommand vs Check) · [`topic-013`](features-sources/topic-013/INDEX.md) (typed Generate schema retries, WithSupervisor, Interview).
- **Persistence, Observation & Tooling (Topics 014–016)**: [`topic-014`](features-sources/topic-014/INDEX.md) (run.jsonl vs project.jsonl, 19 sealed events, agent record segregation) · [`topic-015`](features-sources/topic-015/INDEX.md) (8 atomic tables, SSE deltas, replay cascade, cost rollup) · [`topic-016`](features-sources/topic-016/INDEX.md) (gimblelint GIMBLE101–109 rules, polytype/gimblegen/skgo codegen, Justfile build).
