# Owned native Pi: implementation plan

Status: ready for Tyler's review. This is the coordinator's plan, superseding the independent drafts. No port or delivery workflow has been implemented or launched.

## Decision and result

Build an owned semantic port of Pi's established headless coding engine inside Gimbal. Use the pinned TypeScript implementation as the behavioral reference; borrow verified Go translations package by package. Do not adopt a Go project's entire runtime on the strength of its README or test count.

The result is a normal Gimbal coding session whose provider requests, agent loop, tools, history, and compaction run in Gimbal's process. Shell commands still create children. The existing `pi/...` model route and `HarnessAdapter` remain the integration boundary. There is no new root-Gimbal API or second `pigo`/`pinative` route.

Twenty assignments cover the port, its delivery workflow, and integration. They are not twenty independent jobs that can all start immediately. We first land shared contracts, then fan out over the independent packages, then assemble and qualify the real engine. [MODULES.md](MODULES.md) names the owners and primary source/test anchors. [WORKFLOW.md](WORKFLOW.md) describes the dedicated Gimbal program.

## Reference and scope

The reference is `/Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi` at `d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31`. Follow `packages/coding-agent/src/core/sdk.ts` into `AgentSession` and the established `packages/agent/src/agent.ts` / `agent-loop.ts`. The same repository also contains a newer harness stack; it is not a second runtime to combine with this one. The upstream pin changes only through a deliberate plan update.

The complete target includes the coding loop; streaming text, reasoning, images and tools; built-in read/write/edit/bash/grep/find/ls and PowerShell behavior; settings, model/auth resolution, skills, instructions and prompt templates; durable session trees; independent forks; compaction and branch summaries; cancellation, steering, retries and settlement. Provider families are assigned explicitly below, including authentication and refresh behavior needed by those families.

The first usable milestone is Router-backed coding through `pi/diffusion/deepseek-4.1-flash`, on macOS/Linux, using the default read/bash/edit/write loadout. It is a milestone within the port, not a redefinition of the whole project. Remaining providers, OAuth flows, available search tools, and Windows/PowerShell receive their own acceptance results before broader parity is claimed. Unqualified paths remain explicitly unavailable, never silently redirected to another protocol.

Outside this headless port: terminal/web presentation, HTML export, RPC transport, execution of JavaScript/TypeScript extensions, package installation, the newer experimental harness/durable/remote-worker stack, and non-chat services such as image generation, classification and deferred inference. Reading images for a coding model remains in scope. Gimbal supplies orchestration, supervision, structured-output validation/re-ask, and run observation. Excluded extension behavior must not be represented as having executed.

Use Pi's current v3 JSONL shapes for supported session entries in Gimbal-owned storage. Qualify current-format import/export on copies; do not promise arbitrary historical migrations or faithful replay of extension-defined behavior. Reject unsupported versions or required semantics clearly. Opening existing native history is an internal engine capability; integration must identify an existing caller path before claiming Gimbal workflow restart/resume. `HarnessAdapter` has no Resume method, and this plan does not add one.

These scope choices are recommendations for review, not claims that Tyler previously selected every detail.

## Contracts before implementation fan-out

Assignment 01 owns the shared value definitions and narrow collaboration contracts. Assignment 20 lands that baseline and any approved dependencies. Compile it and test serialization, deep copies and the important cancellation boundaries before dispatch. Do not generate placeholder implementations that return successful empty results.

**Messages and streaming.** Define typed message/content variants for text, images, reasoning with replay metadata, tool calls with stable IDs and raw JSON arguments, tool results, and the history records needed by the coding runtime. Preserve system/tool-state changes when projecting history. Snapshots own their nested slices, maps and bytes. Provider requests receive explicit model capabilities, endpoint, headers, credentials, effort and transcript. A completed provider result or failure is authoritative; partial events alone cannot establish completion. Go may use normal error returns while preserving upstream terminal error/abort semantics. Streaming transport belongs below provider-specific request and event interpretation.

**Tools and loop.** Tools declare literal JSON schemas, validate arguments, execute with a context, emit updates, and return content/details/error/usage/termination. Model-visible tool failure is different from cancellation or an infrastructure failure. Preserve upstream preparation order and sequential/parallel scheduling: completion events may interleave while persisted result messages retain call order. Truncated tool arguments do not execute. Avoid reflection-based schema derivation. Tests must cover steering and follow-up queues at the actual model-call boundaries.

**History and compaction.** History owns append, projection and branch storage. Compaction consumes an immutable snapshot and a provider function and returns a proposed cut and summary; the session owner validates that the snapshot is still current before installing it. Neither module imports the other. Preserve tool-call/result boundaries and reject incomplete summaries. A cancelled compaction changes no accepted history. One writer owns a session file; reopening a busy file must not silently establish a second writer.

**Lifecycle.** A session owns the active turn, queues, append ordering and settlement. `Steer` reports true when consumed for the running turn's next model call, not merely queued. A turn that ends first returns false to pending steers. Context cancellation reaches HTTP, tools, retries and compaction; `Close` cancels, joins and releases resources idempotently. Never hold a session lock while calling providers or user callbacks. `RunTurn` returns after retry/compaction/follow-up settlement, not on the first low-level `agent_end` event.

For active-turn `Fork`, recommend a deep snapshot of the latest complete conversation prefix, excluding an unfinished assistant/tool batch. The parent continues; the child gets independent history, queues and cancellation state, and the same working directory. The contract owner must test and document that boundary before dispatch. Do not port Pi's rewind/replace-current-session command as Gimbal's independent fork.

**Embedding and configuration.** Pass working directory, home/config roots, environment snapshot, trust decision and storage root explicitly. Never call process-wide `Chdir` or `Setenv` for a session. Dynamic-config caches belong to their configuration context. Configured credential ownership must not fall through to ambient credentials after failure. File mutation coordination must cover sessions operating on the same canonical file, including symlink aliases: an adapter-owned shared service supplies the lock; a per-session lock is insufficient.

**Gimbal adapter.** Only assignment 18 imports root Gimbal from the engine tree. Use the existing event projection vocabulary and native references; serialize callback delivery as required. Return a JSON string for plain text, raw candidate JSON for schema output, and let existing `Generate` validate/re-ask. Report all model usage, including compaction/recovery, without double-counting step and turn totals. Normalize into Gimbal's five disjoint token buckets using provider-specific semantics; unavailable pricing is not a fabricated cost. Observation defects remain observable without reversing an otherwise successful execution; actual provider failure, cancellation or missing result is still failure.

## Package ownership and dependency waves

Every assignment owns production files, colocated tests and testdata only in its listed directories. No worker changes shared contracts, another owner's package, module dependencies, registration or generated files. A necessary contract change returns to 01/20, lands serially, and rebases affected workers. Helpers stay with their coherent source module; this is not a package per TypeScript file.

The allowed dependency direction is: shared model values → wire/auth/files/history/agent → providers/config/tools/resources/compaction → session → adapter → Gimbal binding. Arrows here mean assembly order; imports point toward dependencies. More precisely, providers use model/wire and resolved credentials, not config; config composes auth and model metadata without importing providers; agent depends on model contracts, not concrete tools/providers; compaction receives a provider function, not a provider registry. Session constructs the registry and tool set. This avoids history↔compaction, config↔provider and agent↔session cycles.

1. **Foundation:** 01 and 20 settle contracts, dependency choices and licensing placement. 19 can build the delivery workflow against today's Gimbal independently. Workflow registration/generation is owned by 20.
2. **Independent foundations:** 02 wire, 07 auth, 09 files, 14 history and 16 loop run concurrently. Contract-defined fakes let each test without another worker's mutable branch.
3. **Main fan-out:** after foundations integrate, 03–06 provider families, 08 configuration, 10–12 tools, and 15 compaction can run concurrently: nine independent assignments. 13 resources follows 08, producing another useful overlap rather than waiting for every provider. Provider subfamilies may be split into additional owners if the broad 06 assignment becomes the bottleneck; do not add workers by splitting files that share mutable implementation.
4. **Assembly:** 17 session begins with the qualified Router path and default tools, then 18 adapter and 20 cutover prove the first live milestone. Wider provider/OS completion continues under the same owned packages; Router progress need not wait for a credential unavailable for another provider. No unfinished production stub gets bound as working support.
5. **Qualification:** independent review of package behavior plus integrated tests and real runs. Finish remaining named coverage or clearly report the release as the narrower Router milestone.

Nine ready assignments is potential parallelism, not a promise about available agent capacity. The workflow's configurable concurrency limit accounts for worker and reviewer sessions. This interactive task has four collaboration slots; a future detached Gimbal run needs its own verified capacity and budget. Integration is serial regardless of worker count.

## Reuse policy

Inspect sky-valley/pi first as a translation donor, at `e6b56e7223cfab707b21a0a19e0a9a2417ac960a`. Its provider, loop, edit and session translations may save substantial work. Retain only behavior verified against our TypeScript pin; remove unwanted client/protocol/telemetry dependencies through ordinary owned code, not compatibility scaffolding. Its existing differential fixtures may target a different upstream commit.

Pi Agent Go and pi-llm-go remain secondary donors. Their custom endpoint/header and reflection-free tool paths exist; the earlier research's contrary rejection reasons were wrong. Pi Agent Go's shallow message snapshot requires correction for independent forks. The earlier sky-valley test sample passed ai/providers/agent but failed two coding tests; it did not qualify the whole project or establish the cause of every failure. Preserve upstream and donor licenses/attribution for translated code and fixtures. Do not import repository-wide machinery simply to reuse one function.

## Acceptance: behavior, then integration

Each owner translates relevant upstream cases beside the Go package, with source/pin references. Direct differential comparisons use the pinned TypeScript runtime and identical scripted provider events, files and tool outcomes. Compare requests, results, history and meaningful event order; normalize only incidental IDs/timestamps while retaining their relationships. A donor's outputs are comparison candidates, never the oracle. Run references in a disposable checkout; repeatable comparison drivers belong in package tests, not committed proof programs or run dumps.

The decisive cases are fragmented and truncated streams; reasoning replay; tool argument errors and result order; Unicode/fuzzy edits and CRLF; image reads; symlink mutation ordering; shell output arriving after process exit versus stale callbacks after operation completion; cancellation of process trees; configuration isolation; compacted history reopen; and deep independent forks. Use isolated home/config roots, race tests and deterministic failures. Do not turn a known failure into a skipped success.

For the Router milestone, use the built application's normal binding to run a real coding task: read a seeded file, edit it, execute an assertion, return schema-valid output, then continue in a second turn. Observe actual files and command results. Also demonstrate consumed steering, cancellation, independent parent/child follow-ups, compaction with retained context, fresh-process native history reopen, and simultaneous sessions with different directories/configuration. Observe tools/text on the existing run page. Confirm no Pi Node/Bun agent child is present; ordinary tool children are expected.

Run this cheaply with `pi/diffusion/deepseek-4.1-flash` and state the model in the report. Validate actual Router request/response compatibility and usage; credentials or unsupported fields can block this milestone and cannot be replaced with a mock-only success claim. Other provider families need their own fixture coverage and available live credentials; list any unqualified family explicitly. Windows behavior requires Windows execution before it is called supported.

20 alone rebases onto the actual result of #386, replaces its Pi binding/implementation when native acceptance passes, and removes what is replaced without a shim. Other harness regressions remain part of integration checks. The workflow authoring check includes graph generation/lint and a cheap live fan-out demonstration before it dispatches the real port. Report observed behavior in chat/PR; no committed screenshots, session logs or proof programs.

## Immediate next step

Review this package map and scope, then implement 01 shared contracts and 19 the small dedicated workflow in parallel, with 20 owning their integration. Those are the prerequisites to safely launching the package workers. No further broad research round is needed.
