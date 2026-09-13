# Sprint Plan Critique: The Run Store

Review of `RUN-STORE-CLAUDE-DRAFT.md` and `RUN-STORE-CODEX-DRAFT.md` against `RUN-STORE-INTENT.md` and the existing Gimble codebase (`internal/observation`, `run.go`, `session.go`, `event_persistence.go`, `internal/runlog`, and `web/src/lib/observation`).

---

## Executive Summary & Verdict

Both drafts grasp the core mandate from `RUN-STORE-INTENT.md`: replace `observation.json` and `checkpoint.go` with six fact tables persisted as atomic JSON files, attribute usage by turn execution scope rather than session creation scope, compute prefix-containment totals in Go, and preserve the current run page's live and finished rendering without building the tree (#173), prices (#172), a database, or a message table.

However, **Claude's draft is distinctly superior in architectural soundness, concurrency design, and code economy.**

- **Claude chooses unified replay:** It realizes that replaying the log through the exact same `Store.Lifecycle` and `Store.Event` methods eliminates the need to write and maintain six table decoders and a separate transcript loader. Furthermore, Claude's two-phase replay (`run.jsonl` first, then `sessions/**/*.jsonl` with an `Ended == 0` guard) makes replay completely order-free and immune to concurrency interleaving.
- **Claude solves disk writes safely:** It decouples disk I/O from the central store lock using a dirty bitset and an encode-under-lock / write-unlocked flusher loop. Writes coalesce naturally under load without background timers, goroutines, or mutex blocking.
- **Codex over-engineers read paths while under-engineering concurrency:** Codex introduces three separate read/deserialization mechanisms (a table loader for 6 files, a session-log transcript hydrator, and an elaborate log rebuilder). In that rebuilder, it attempts to interleave native events between lifecycle turn boundaries—a fragile approach when concurrent goroutines run turns in a `Group`. Crucially, Codex proposes holding the store mutex during synchronous disk writes and renames on every step, introducing severe lock contention on the event hot path.

**Recommendation:** Proceed with **Claude's draft as the foundation**, incorporating two key insights from Codex: (1) prevent unnecessary disk rewrites when opening an already completed run whose table files already exist, and (2) retarget `web/observation_live_test.go` to the observation SSE endpoint rather than deleting it.

---

## 1. Architectural Soundness

### Claude Draft
- **Replay & Unification (Open Question 1):** Claude answers that finished runs should be read by replaying `run.jsonl` and `sessions/**/*.jsonl` through `Store.Lifecycle` and `Store.Event`. This fulfills Decision 3 ("Live and finished are one path") in the deepest possible sense: the in-memory state is built by the exact same reduction logic whether live or finished.
- **Order-Free Accounting via Ended Guard:** Gimble emits `TurnEnded` to `run.jsonl` with the final harness `ModelUsage` report. Claude replays `run.jsonl` first, populating `turnUsage[turn]` and setting `turns[turn].Ended > 0`. When it subsequently processes session logs, `Store.Event` checks `if turns[turn].Ended == 0` before updating provisional usage. This guarantees that finished turns retain provider-reported usage without requiring timestamp synchronization or cross-log interleaving.
- **Flusher Architecture (Open Question 3):** Claude's `flush()` is an exemplary concurrent Go pattern:
  ```go
  s.mu.Lock()
  // check dirty, set flushing
  for s.dirty != 0 {
      pending := s.encodeDirtyLocked() // clears dirty bits
      s.mu.Unlock()
      for name, raw := range pending {
          writeAtomic(path, raw)
      }
      s.mu.Lock()
  }
  s.flushing = false
  s.mu.Unlock()
  ```
  `s.mu` is never held across file I/O or directory renames. If steps arrive while a flush is in flight, they simply set their dirty bits; the running loop picks them up on the next iteration. Burst writes coalesce to the disk's throughput without timer queues or dropped updates.
- **Wire Frames (Open Question 2):** Uses four frames: `snapshot`, `row`, `totals`, and `event`. Replacing `lifecycle` with `row` (`state[table][key] = row`) shrinks `web/src/lib/observation/index.ts` by deleting `foldLifecycle`, `scopeAt`, and `parseValue`. Emitting `totals` separately directly honors Intent Decision 3 ("the server publishes recomputed per-scope totals as a frame whenever a turn_usage row changes").
- **Flaw:** Claude's `open(dir)` unconditionally marks all six tables dirty and calls `flush()`, rewriting the six files even when opening an already completed run where the files are already present and valid.

### Codex Draft
- **Triple Read Mechanism:** Codex creates:
  1. `tables.go` loading 6 JSON table arrays into memory.
  2. Transcript hydration scanning session logs on snapshot request.
  3. A log rebuilder when table files are missing or corrupt.
  This creates three different paths to populate store state. If the table writer and table loader diverge in subtle schema details, bugs will arise that do not occur on the live path.
- **Fragile Interleaving in Rebuild:** Codex states: *"while walking the lifecycle log in sequence, feed each turn's native records after its start and before its end report."* In a concurrent run with `gimble.Group(ctx, "compare")`, turns from multiple sessions interleave across `run.jsonl`. Implementing a streaming interleaver that matches turn IDs, pauses the lifecycle walk, buffers session records, and feeds them before each respective `turn_ended` requires complex queueing machinery.
- **Severe Lock Contention on Disk Writes:** Codex specifies: *"Each mutation holds the store lock through folding and rewriting the affected files: temp file in the same directory, close, rename."* Holding `s.mu` across `os.WriteFile` and `os.Rename` for `model_calls.json` and `turn_usage.json` on every step violates the established principle in `checkpoint.go` ("No disk work happens on the event path or while the reduction lock is held"). Under concurrent sessions, all goroutines will stall waiting for disk I/O.
- **Inefficient `state` Frame:** Codex sends the complete fact maps for the entire run (`run`, `scopes`, `sessions`, `turns`, `turn_usage`, `model_calls`, and totals) on *every* step ended and *every* lifecycle event. In a run with 50 turns and 200 model calls, serializing and sending a 100KB JSON payload several times per second over SSE is needlessly heavy for both the server and the browser.

---

## 2. Completeness & Code Reality Check

Checking claims against the actual codebase:

### `internal/observation` (`store.go`, `usage.go`, `snapshot.go`, `checkpoint.go`)
- **Session-level vs Turn-level Accounting:** Today, `usage.go:ScopeUsage` sums `r.run.Usage` filtered by `inScope(scope, r.Sessions[session].Scope)`. Because `r.Sessions[session].Scope` is where the session was created, child turns are misattributed to the root (#157). Both drafts correctly identify this and pivot to `turn_usage` joined with `turns[turn].Scope`.
- **Struct Definitions:** Claude provides full, concrete Go struct definitions (`Tokens`, `Usage`, `RunRow`, `ScopeRow`, `SessionRow`, `TurnRow`, `TurnUsageRow`, `ModelCallRow`) with exact JSON tags matching the Intent. Codex only provides a high-level summary table and omits struct details.
- **Tracking In-Flight Calls:** Between `session.step.started` and `session.step.ended`, model identity and start times must be held. Claude defines an explicit private `calls map[string]openCall` under `s.mu`. Codex describes this in prose but omits data structures.

### `run.go` and Lifecycle Observation
- In `run.go:observeLifecycle`, the current code runs a type switch over `LifecycleEvent` variants (`RunStarted`, `RunEnded`, `RunCancelled`, `SessionCreated`, `ScopeBegan`, `ScopeEnded`, `ValueSet`, `PlannerDecision`). Crucially, **`TurnStarted` and `TurnEnded` are currently ignored by `observeLifecycle`**.
- Both drafts correctly recognize that `observeLifecycle` must pass the raw event record to `Store.Lifecycle` so `TurnStarted` (prompt, output type) and `TurnEnded` (result, error, usage report, duration, interrupted) are folded into `turns` and `turn_usage`. Claude completely eliminates the switch in `run.go`, delegating the single JSON decode to `store.go`.

### `session.go` and `event_persistence.go`
- `session.go:stepUsage` extracts usage from `session.step.ended` and qualifying `session.step.failed` events (only when both cost and tokens are present). Both drafts respect this logic.
- Session log paths: In `event_persistence.go`, `sessionEvent` writes to `filepath.Join(r.dir, "sessions", session+".jsonl")`. Because child sessions have IDs like `agy.1/agy.1`, the resulting paths are nested (`sessions/agy.1/agy.1.jsonl`).
  - Claude notes that walking `sessions/` must handle arbitrary depth.
  - Codex notes that session IDs contain slashes and maps them directly. Both accurately reflect the code.

### `internal/runlog`
- `internal/runlog/reader.go` implements `Read[T]`, which tails the log with `time.After(10 * time.Millisecond)` until an envelope with `Kind == "complete"` is seen.
- Codex proposes modifying `reader.go` to add a finite reader. However, `RUN-STORE-INTENT.md` explicitly lists changes to adapters, log formats, or reducer ports as out of scope.
- Claude avoids touching `internal/runlog` entirely, using a standard, self-contained `bufio.Reader` loop in `replay.go`. This is less code and zero risk to existing consumers of `runlog.Read`.

### `web/src/lib/observation` and Svelte Components
- Both drafts correctly identify that `web/src/routes/runs/[runID]/usage.remote.go` (`skgo.LiveQuery(scopeUsage)`) should be deleted, with totals flowing directly from `RunSnapshot.totals`.
- In `RunViewer.svelte`, the root total replaces the remote query text. In `SessionTimeline.svelte`, session totals read from `totals.sessions[sessionID].all`.
- Claude specifies the exact TypeScript refactoring in `index.ts`, deleting `foldLifecycle`, `LifecycleRecord`, `scopeAt`, and `parseValue`.

---

## 3. Phasing & Ordering

| Aspect | Claude Draft | Codex Draft | Assessment |
| :--- | :--- | :--- | :--- |
| **Phase 1** | Go schema, fold, totals, frames. Temporarily keeps writing `observation.json` at `Close` so existing tests stay green. | Go facts, accounting, write-through. Eliminates checkpoint writes immediately. | **Claude is safer.** Keeping the temporary checkpoint write in Phase 1 allows iterative testing of the fold before overhauling the storage lifecycle. |
| **Phase 2** | `files.go`, `replay.go`, `registry.go`. Deletes `checkpoint.go`. Tests on issue-149 testdata. | `tables.go`, `replay.go`, registry, and edits `internal/runlog`. Deletes `checkpoint.go`. | **Claude is cleaner.** Codex modifies `runlog/reader.go`, creating cross-package churn. |
| **Phase 3** | Svelte/TS page update, deletes remote query, regenerates skgo bindings, verifies SSR. | Svelte/TS page update, deletes remote query, retargets `observation_live_test.go`. | **Codex has a better test idea.** Retargeting `observation_live_test.go` to the SSE endpoint provides better regression coverage than deleting it. |
| **Phase 4** | Live proof run on cheap tier with inline code, `jq` checks, restart check, screenshots. | Live proof run described in prose, manual `jq` validation. | **Claude is more concrete.** Claude provides the exact proof script and `jq` command. |

---

## 4. Risk Coverage

| Risk Area | Claude Handling | Codex Handling | Assessment |
| :--- | :--- | :--- | :--- |
| **Write amplification / hot disk path** | Detailed analysis. Measured via proof run. Flusher coalesces in-flight mutations. Fallback to turn-boundary flushing ready. | "Start synchronous and revisit only if the live run shows a problem." | **Claude wins decisively.** Codex's plan to hold `s.mu` during disk writes is a major bottleneck. |
| **Concurrency during replay** | Order-free: `run.jsonl` processed first, then session logs. Guarded by `turns[turn].Ended == 0`. | Interleaving algorithm feeding native events between turn start/end. | **Claude is robust; Codex is brittle.** Codex's interleaving risks failures on concurrent groups. |
| **Model naming discrepancies** | Acknowledges that step models (`claude-haiku-4-5...`) may differ from session or harness report keys; handled cleanly at turn end. | Notes report replacement may change keys, but lacks detailed handling of open calls. | Claude is more thorough. |
| **Memory usage of finished runs** | Acknowledged in `registry.finished`; justified by project scale (tens of runs). | Mentioned briefly. | Realistic for the stated scale. |
| **Stale generated artifacts** | Explicitly calls out manual cleanup of `web/src/lib/skgo/observation/types.ts` if `go generate` orphans it. | Notes regeneration; warns not to hand-edit generated files. | Claude anticipates generator edge cases. |

---

## 5. Feasibility & "Less Code" Assessment

The prompt's primary constraint is: **Prefer the answer that is less code; a criticism that requires more machinery than the intent allows is out of bounds.**

- **Loader vs. Replay Code:**
  - Writing six JSON array decoders, handling schema migrations, error handling for each file, and combining loaded state with a transcript scanner requires ~250–350 lines of Go.
  - In contrast, Claude's `replay.go` is ~60 lines: open files, scan lines, pass to `Store.Lifecycle` and `Store.Event`.
- **Rebuild Complexity:**
  - Codex's rebuild requires an interleaving scheduler to match lifecycle events with native event queues.
  - Claude's two-pass replay requires no scheduling or buffering.
- **External Package Footprint:**
  - Codex edits `internal/runlog/reader.go` and adds tests in `reader_test.go`.
  - Claude leaves `internal/runlog` untouched.
- **Wire Frame Handlers:**
  - Claude's `row` frame requires a single dynamic assignment in TypeScript (`state[table][key] = row`).
  - Codex's `state` frame requires replacing every map on every frame, which complicates local cache preservation and message revision tracking.

Claude's plan results in substantially less code and far fewer moving parts.

---

## 6. Definition of Done Evaluation

Both drafts align with the Intent's six success criteria:
1. Six table files present, no `observation.json`, `jq` validation passes.
2. Run page renders unchanged, header shows root totals, session cards show totals, transcripts render.
3. Turn placement attributes child-executed turns to child scopes.
4. Saved issue-149 run opens from logs alone, creates files, and renders.
5. Totals broadcast as frames; browser implements no sum rule.
6. Build and tests pass; one live proof run on cheap tier (`claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`).

**Strengths of Claude's DoD:**
- Gives the exact `jq` command for token aggregation across `turns.json` and `turn_usage.json`.
- Supplies the exact expected token numbers for the issue-149 test case (e.g., Haiku: 20 input, 66341 cache read, 8269 cache write, 130 output, 220 reasoning).
- Explicitly details the two screenshots required.

**Strengths of Codex's DoD:**
- Adds an explicit item 7 confirming no out-of-scope machinery was introduced.

---

## 7. Strongest Ideas Worth Keeping

### From Claude:
1. **Replay as the Sole Deserializer:** Treating the log as the single source of truth and the table files as derived outputs avoids duplicating ingestion logic.
2. **Order-Free Ended Guard (`Ended == 0`):** Processing `run.jsonl` before session logs allows independent, uncoordinated file reads during replay without overwriting final reports.
3. **Unlocked Coalescing Flusher:** Moving disk I/O outside `s.mu` while using dirty bits to coalesce bursts is essential for performance and prevents deadlocks.
4. **Targeted Wire Frames (`row` + `totals`):** Streaming updates by table row and broadcasting recomputed totals matches the Intent's reactive design without SSE bloat.
5. **Concrete Proof Scenario:** The inline proof script exercising root-created sessions in child scopes alongside concurrent `Group` execution directly tests the hard placement and concurrency requirements.

### From Codex:
1. **Retargeting `web/observation_live_test.go`:** Rather than deleting this test when `usage.remote.go` is removed, retargeting it to verify that `/api/runs/{runID}/events` streams frames to HTTP clients maintains vital end-to-end integration coverage.
2. **Idempotent Finished Open:** Avoiding disk writes when opening an existing, intact finished run preserves file timestamps and avoids unnecessary I/O.

---

## 8. Synthesis: Refinements for the Final Sprint Plan

To finalize the sprint plan, adopt Claude's draft with the following two surgical adjustments:

1. **Avoid Rewriting Intact Finished Runs:**
   In `internal/observation/replay.go`, before marking all tables dirty and calling `flush()`, check if the six table files already exist in `dir`. If all six files are present and non-empty, skip the final flush. If any file is missing, flush all six. This preserves Claude's "rebuild by open" feature while keeping read operations read-only for normal runs.

2. **Retarget rather than Delete `web/observation_live_test.go`:**
   In Phase 3, adapt `web/observation_live_test.go` to connect to `GET /api/runs/{runID}/events` using the existing `httptest.Server` and `NewHandler`. Assert that it receives the opening `snapshot` frame, followed by `totals` and `row` frames when events are folded, and closes cleanly on `store.Close()`.
