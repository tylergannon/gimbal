# Sprint 001 Critique: Claude Draft vs. Codex Draft

**Reviewer:** Gemini  
**Target:** `SPRINT-001-CLAUDE-DRAFT.md` and `SPRINT-001-CODEX-DRAFT.md`  
**Context:** `docs/sprints/drafts/SPRINT-001-INTENT.md`, `AGENTS.md`, `docs/definition-of-done.md`, and repository state at worktree `token-usage-by-scope-50a907`.  

---

## 1. Executive Summary & Comparative Verdict

Both Claude and Codex deliver high-caliber drafts that correctly recognize the core premise of Sprint 001: Gimble already captures model-level token usage in OpenCode projection rows, places every message by scope key, and emits lifecycle events to `run.jsonl`. Both drafts correctly reject premature cost calculation / price tables, avoid altering the sealed `LifecycleEvent` union (`TurnEnded.Tokens` remains `[]JSONText`), leave the `internal/sessionstate` projection port untouched, and insist that live streaming and post-run checkpoint viewing share a single browser rendering path.

However, they diverge sharply in design philosophy, edge-case rigor, and execution scope:

- **Claude's Draft** excels at concrete technical specification. It provides explicit TypeScript types and Go structs, formalizes usage as an additive commutative monoid (`Cell: { tokens, unknown }`), includes a per-turn revision cache to prevent UI performance degradation on streaming deltas, and structures a lean proof with a standalone reconciliation script. However, it suffers from a crowded single-component UI architecture (`UsageRow`), overlooks Antigravity's tool-only assistant row projection trap, fails to handle the root scope naming bug (`"."`), and under-specifies boundary prefix containment.
- **Codex's Draft** excels at systems edge cases and operational rigor. It catches the Antigravity tool-row accounting distortion, enforces segment-safe scope key containment (`attempt.1/` vs `attempt.10`), fixes the root scope display name, separates interval duration from token weight on the timeline, and specifies a restart proof from `observation.json` alone. However, its prose is overly descriptive with zero concrete type signatures, its negative-value rule is overly rigid, and its Phase 5 plan balloons into full Playwright-BDD feature engineering that threatens single-sprint feasibility.

**Verdict:** The optimal sprint plan is an **architectural synthesis**: adopt **Claude's additive monoid (`Cell`), concrete Go/TS type definitions, per-turn revision caching, and lean proof script**, combined with **Codex's edge-case rigor (tool-row filtering, segment-safe prefixing, root name override, timeline weight/duration separation, and checkpoint-only restart test)**.

---

## 2. Evaluation of Claude's Draft (`SPRINT-001-CLAUDE-DRAFT.md`)

### Architectural Soundness
- **Strengths:**
  - **Single Browser Rollup (`usage.ts`):** Properly places rollup computation in the browser from canonical message rows, keeping Go as a lean producer and eliminating drift between live and replayed runs.
  - **Monoid Formulation:** Models each field cell as `{ tokens: number; unknown: number }`. Making `Usage` an additive commutative monoid ensures parents equal the sum of their children by construction without reconciliation hacks.
  - **Per-Turn Revision Caching:** Recognizing that recomputing usage over hundreds of turns on every text streaming delta would cause UI frame drops, Claude introduces a per-turn revision cache that keeps per-frame rollup complexity at $O(\text{turns})$.
  - **Durable Snapshot Structures:** Cleanly extends `RunSnapshot` with `Scopes map[string]ScopeInfo` and `Turns map[string]TurnInfo`, properly capturing began/ended timestamps, turn prompts, durations, and task JSON.
- **Weaknesses & Gaps:**
  - **Root Scope Name Bug:** In `scope.go:74`, the root scope's name is derived as `path.Base("")`, which is `"."`. Claude folds this directly into `ScopeInfo.Name`, causing the UI root row to be labeled `"."` instead of the human-readable run name.
  - **Tool-Only Row Blindspot:** In real Antigravity runs, tool executions emit assistant rows without token usage. Claude treats all rows lacking accounting as having "all fields unknown" (`unknown += 1`), which distorts the denominator of model calls.
  - **UI Crowding:** Cramming scope/turn disclosure, labels, wall-clock bar, five token columns, model names, and per-model `<details>` into a single recursive `UsageRow.svelte` creates an excessively wide, unreadable row.

### Completeness
- Claude provides full TypeScript signatures, Go struct definitions, and an exact file modification table (21 files).
- Explicitly accounts for optional task JSON serialization quirks (`polytype.Optional[Task]` marshaling as `""`).
- Leaves no ambiguity about data structures or module boundaries.

### Phasing & Ordering
- **Phasing Structure:** 6 phases (0 through 5).
- **Gated Probe:** Phase 0 runs live probes against cheap models (`gpt-5.6-luna`, `claude-haiku-4-5-20251001`, `gemini-3.8-flash-low`) before modifying adapter normalization. This prevents building on unverified provider semantics.
- Order of execution (Probe $\to$ Go fold $\to$ Browser fold/monoid $\to$ Adapters $\to$ UI $\to$ Proof) is logical and minimizes rework.

### Risk Coverage
- Strong coverage of live provider deviations, Claude/Codex unavailable-by-construction traps, frame computation bottlenecks, and backward compatibility with pre-sprint checkpoints.
- **Gap:** Fails to anticipate string prefix collision in scope containment (e.g., matching `attempt.10` when querying `attempt.1`).

### Feasibility
- **High.** Avoids proof harness bloat. Relies on an ordinary Go script (`main.go`) and a focused Bun script (`reconcile-usage.ts`), ensuring the work comfortably fits within one focused sprint.

### Definition of Done
- 7 observable, artifact-backed criteria.
- Requires both live and checkpoint screenshots, reconciliation script output, and zero-error builds.
- Includes a pragmatic 90–95% exit condition for minor visual styling quirks.

---

## 3. Evaluation of Codex's Draft (`SPRINT-001-CODEX-DRAFT.md`)

### Architectural Soundness
- **Strengths:**
  - **Antigravity Tool-Row Handling:** Explicitly distinguishes tool execution rows from actual model-call rows using provenance. Tool execution rows are recognized as non-billable and excluded from model-call availability denominators.
  - **Segment-Safe Scope Containment:** Correctly specifies that scope membership requires exact match or prefix followed by `/` (`key === scope || key.startsWith(scope + '/')`), preventing `attempt.10` from falsely falling under `attempt.1`.
  - **Root Scope Identity:** Explicitly overrides `ScopeBegan`'s `"."` with the actual run name for UI presentation.
  - **Visual Timeline Encodings:** Decouples interval duration (bar width) from token weight (numeric indicators). This avoids the visual illusion that longer elapsed wall-clock duration implies higher token spend.
  - **Additive Availability Tracking:** Tracks `{ sum, contributingCalls, observedCalls }`, supporting clean partial-coverage reporting.
- **Weaknesses & Gaps:**
  - **Overly Rigid Negative-Value Policy:** Codex mandates that negative derived values must not be clamped via `max(0, ...)` and must instead be marked completely unavailable. In practice, provider token reporting occasionally exhibits minor skew or rounding; discarding the entire field because of a -1 jitter is counterproductive.
  - **Missing Type Specifications:** Avoids declaring concrete Go and TypeScript interfaces, leaving struct field naming and serialization contracts undefined.
  - **Split UI Architecture:** Splitting the view into separate `UsageTree` and `UsageTimeline` components requires synchronized selection state across distinct widgets, increasing implementation surface.

### Completeness
- Conceptually exhaustive across failure modes, cancellation precedence, subscriber isolation, and multi-model scope accounting.
- **Gap:** Lacks concrete code snippets, type definitions, and explicit data structures, leaving substantial design decisions to the implementer during coding.

### Phasing & Ordering
- **Phasing Structure:** 5 phases (1 through 5).
- Combines adapter research and normalization into Phase 1, and bundles Go and browser persistence in Phase 2.
- **Gap:** Phase 5 is overloaded, bundling Playwright-BDD test generation, e2e step implementations, live workflows, deterministic fake-adapter modes, and checkpoint-only assertions into a single phase.

### Risk Coverage
- Best-in-class coverage of subtle data-integrity risks:
  - Tool-only projection distortion.
  - Scope key ordinal collisions.
  - Reconnect/SSE re-read double counting.
  - Drift between Go and TypeScript fold logic.
  - Misleading timeline visual weights.

### Feasibility
- **Moderate.** The architecture is sound, but Phase 5 threatens sprint delivery. Demanding full Gherkin feature definitions (`e2e/features/token-usage.feature`), Playwright step bindings, dual-mode runner flags, and dual UI components creates excessive overhead for a single sprint.

### Definition of Done
- 9 thorough criteria with strict adherence to `docs/definition-of-done.md`.
- Requires race detection (`go test -race`), checkpoint-only restart proof without source logs, and verifiable live model traces.
- Extremely rigorous, though slightly process-heavy.

---

## 4. Side-by-Side Comparison Matrix

| Evaluation Dimension | Claude Draft | Codex Draft | Analysis & Recommendation |
|---|---|---|---|
| **Rollup Location** | Browser-only (`web/src/lib/observation/usage.ts`) | Browser-only (`web/src/lib/observation/usage.ts`) | **Consensus.** Persisting raw canonical inputs in Go and computing rollups in TypeScript prevents dual-maintenance drift. |
| **Availability Model** | Monoid `Cell { tokens, unknown }` | `sum`, `contributingCalls`, `observedCalls` | **Claude is cleaner; Codex is more explicit.** Claude's monoid is mathematically elegant; adopting Codex's explicit call counts within that cell yields the best result. |
| **Tool Execution Rows** | Unhandled (counted as unknown model calls) | Filtered via provenance (excluded from denominators) | **Codex is superior.** Counting tool rows as model calls distorts availability percentages in Antigravity runs. |
| **Scope Containment** | Naive string prefix | Segment-safe prefix (`key === s \|\| key.startsWith(s + '/')`) | **Codex is superior.** Prevents ordinal matching bugs (`attempt.1` vs `attempt.10`). |
| **Root Scope Display** | Raw `ScopeBegan.Name` (`"."`) | Overridden to Run Name | **Codex is superior.** Avoids labeling the root row `"."` in the UI. |
| **Streaming Performance** | Per-turn revision cache ($O(\text{turns})$) | Invalidation on accounting frames | **Claude is superior.** Concrete caching prevents UI lag during active text generation. |
| **Timeline Design** | Integrated row bars (wide table) | Separate `UsageTree` & `UsageTimeline` | **Codex is superior.** Decoupling duration from token weight prevents visual misinterpretation; integrated wide rows cause overflow. |
| **Type Definitions** | Fully specified Go and TS types | Descriptive prose only | **Claude is superior.** Removes guesswork for the implementer. |
| **Proof Strategy** | Inline Go workflow + `reconcile-usage.ts` | Go script + Playwright-BDD (`.feature` + steps) | **Claude is superior.** Leaner, avoids heavy BDD harness scaffolding while still proving end-to-end reconciliation. |
| **Checkpoint Restart** | Noted in DoD | Explicit step: copy *only* `observation.json` | **Codex is superior.** Guarantees zero reliance on historical `run.jsonl` or session logs. |

---

## 5. Strongest Ideas Worth Keeping from Each

### From Claude's Draft:
1. **The Commutative Monoid Formulation:** Defining `Usage` as a commutative monoid under `add` with cell `{ tokens, unknown }` guarantees that parent totals equal the sum of their children without reconciliation steps.
2. **Concrete Structural Contracts:** Ready-to-implement definitions for `ScopeInfo`, `TurnInfo`, `RunSnapshot`, and `FieldUsage`.
3. **Per-Turn Revision Caching:** Caching turn usage by an incremental turn revision ensures the UI handles high-frequency streaming deltas effortlessly.
4. **Phase 0 Empirical Probe:** Formally isolating live probe runs before changing normalization code guarantees adapter fixes reflect actual provider payloads.
5. **Lightweight Automated Reconciliation:** `web/scripts/reconcile-usage.ts` provides a direct, scriptable proof asserting that page rollups match raw `turn_ended` log records.

### From Codex's Draft:
1. **Tool-Only Assistant Row Filtering:** Recognizing Antigravity tool execution rows from provenance and excluding them from model-call availability denominators.
2. **Segment-Safe Containment Logic:** Explicitly requiring boundary delimiters (`/`) when evaluating prefix hierarchy to prevent ordinal collision.
3. **Root Scope Human-Readable Label:** Replacing `"."` with `RunSnapshot.Run.Name` for the top-level tree row.
4. **Decoupled Duration and Token Weight:** Separating timeline bar width (wall-clock elapsed time) from token weight indicators to avoid misleading visualizations of concurrent work.
5. **Pure Checkpoint-Only Verification:** Demonstrating that a server booted in a directory containing solely `observation.json` renders the complete usage hierarchy and timeline without touching raw logs.

---

## 6. Weaknesses and Critical Gaps to Avoid

1. **Do NOT adopt Claude's overloaded `UsageRow` layout:** Stacking disclosure toggles, names, time bars, five token cells, and model breakdowns into one recursive table row will result in severe horizontal clipping and poor usability.
2. **Do NOT adopt Codex's rigid negative-clamping ban:** Marking derived fields unavailable instead of `max(0, ...)` when minor provider timing/token inconsistencies occur throws away useful measurement data.
3. **Do NOT build full Playwright-BDD test suites in Sprint 001:** Writing `.feature` files, Gherkin step definitions, and BDD harnesses in Phase 5 adds unnecessary friction. Standard Playwright assertions and the Go/TypeScript reconciliation script provide robust proof without framework overhead.
4. **Do NOT ignore the root scope naming quirk:** Deriving the root name via `path.Base("")` yields `"."`. The UI must explicitly use the run name for key `""`.
5. **Do NOT evaluate scope keys with raw `startsWith`:** Always verify segment boundaries to avoid cross-scope contamination between ordinals like `attempt.1` and `attempt.10`.
6. **Do NOT count tool execution messages as model calls:** Tool rows must be filtered out of availability tracking so missing token payloads do not trigger false "partial" or "unavailable" warnings on real LLM calls.

---

## 7. Synthesis: Recommended Merge Strategy

The final `SPRINT-001.md` plan should execute the following merged blueprint:

```
                  ┌─────────────────────────────────────────────────────────┐
                  │ Phase 0: Provider Empirical Probe                       │
                  │ (Live Haiku, Luna, Gemini 3.8 Flash warm-cache probes)  │
                  └────────────────────────────┬────────────────────────────┘
                                               │
                         ┌─────────────────────┴─────────────────────┐
                         ▼                                           ▼
┌──────────────────────────────────────────────────┐   ┌──────────────────────────────────────────────┐
│ Phase 1: Go Lifecycle Fold & Checkpoint          │   │ Phase 2: Adapter Normalization Corrections   │
│ - ScopeInfo & TurnInfo in internal/observation   │   │ - agy: input = max(0, input - cacheRead)     │
│ - Map root scope "" name to Run.Name             │   │ - claude: output independent of reasoning    │
│ - Stamped record timestamps; deep-copy in store  │   │ - codex: input independent of cacheWrite     │
└────────────────────────┬─────────────────────────┘   └──────────────────────┬───────────────────────┘
                         │                                                    │
                         └─────────────────────┬──────────────────────────────┘
                                               ▼
                  ┌─────────────────────────────────────────────────────────┐
                  │ Phase 3: TypeScript Monoid & Rollup (web/src/lib)       │
                  │ - Usage monoid: { tokens, unknown, calls, observed }    │
                  │ - Segment-safe containment (key === s || key.hasPrefix) │
                  │ - Filter out tool-only assistant rows from denominators │
                  │ - Per-turn revision cache for O(turns) streaming rollup │
                  └────────────────────────────┬────────────────────────────┘
                                               ▼
                  ┌─────────────────────────────────────────────────────────┐
                  │ Phase 4: UI Hierarchy & Decoupled Timeline              │
                  │ - RunViewer tabs: Transcript and Usage                  │
                  │ - UsageTree: Run, scope, and turn totals by model       │
                  │ - UsageTimeline: Wall-clock axis with distinct weight   │
                  │ - MessageRow: field-aware footer replacing accounting() │
                  └────────────────────────────┬────────────────────────────┘
                                               ▼
                  ┌─────────────────────────────────────────────────────────┐
                  │ Phase 5: Verification & Proof                           │
                  │ - Inline Go workflow: research, bakeoff, review scopes  │
                  │ - web/scripts/reconcile-usage.ts log reconciliation     │
                  │ - Checkpoint-only restart test (observation.json alone) │
                  │ - Production screenshots & raw accounting recording     │
                  └─────────────────────────────────────────────────────────┘
```

By fusing Claude's mathematical clarity and execution velocity with Codex's systems edge-case precision, the resulting sprint plan will be architecturally unassailable, fully aligned with repository standards, and reliably achievable within a single focused session.
