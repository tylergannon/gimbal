# Run store: critique of the Claude and Gemini drafts

Claude is the stronger starting point: it explains how the existing event
shapes become facts, preserves final turn accounting during replay, and
actually removes the browser's lifecycle fold. Keep Gemini's preference
for direct writes, but serialize them. Neither draft is ready unchanged.

This review treats the intent's decisions as fixed. Full-log replay is an
allowed answer to its explicit transcript question. The recommendations
below stay within the six files, existing logs, Go store, and current page.
They require no tree, prices, database, message table, proof framework, or
reconciliation tooling.

Evidence is from the current checkout, including the named Go and browser
modules and a read-only inspection of the saved issue-149 logs. This is a
plan review, not implementation validation. `go doc -all .` was attempted
but failed on sandbox access to the Go build cache; the relevant declarations
and implementations were read directly. No build or live proof was run.

## Claude draft

| Dimension | Assessment |
| --- | --- |
| Architectural soundness | Mostly sound. One lifecycle decoder, existing transcript projections, server totals, and assignment frames form a coherent design. The writer and finished-store cache are more elaborate than the demonstrated need. |
| Completeness | Substantially better than Gemini. Covers fields, placement, failed steps, final report replacement, timestamps, provenance, replay paths, snapshots, and generated bindings. Decision sequence and empty table initialization remain missing. |
| Phasing and ordering | The conceptual order is understandable, but its promise of green phases contradicts its file assignments: Phase 1 removes APIs that the page's Go query still calls until Phase 3. |
| Risk coverage | Identifies the important accounting and lifecycle risks. Its flush error handling does not support all its durability claims, and the proposed turn-boundary write fallback violates the fixed write cadence. |
| Feasibility | Feasible after simplifying persistence and joining the dependent Go/browser changes. Estimates such as a sixty-line replay are not evidence or useful acceptance constraints. |
| Definition of done | Concrete and close to the intent: named placement case, manual arithmetic, live observation, restart, saved-run replay, and all required commands. A few asserted observations are not produced by the proposed proof program. |

### Strongest ideas to keep

- **Decode lifecycle JSON once inside observation.** Today `run.go:137-172`
  translates typed variants into `observation.Lifecycle`, while that entry
  already carries the exact log record. Moving the decode into the store
  lets replay and live ingestion share the fold without importing the root
  package back into `internal/observation`.
- **Keep the ended-turn guard.** The draft's lifecycle-first replay installs
  each final report before reading session steps. Its guard prevents those
  steps from adding to the authoritative `turn_usage` map, while still
  building model calls and transcripts. It also preserves an explicitly
  empty final report. `session.go:180-185` distinguishes an absent harness
  report from an explicitly empty one before recording `turn_ended`; the
  store should consume that recorded report without another fallback.
- **Correlate step starts and ends by turn and assistant message.** This
  supplies the model and start time that step-end payloads do not carry.
  The saved logs confirm `data.model.id` on `session.step.started` and
  `data.assistantMessageID` on both start and end. Use the normalized message
  ID, matching the transcript; `session.go:319-335` performs that stamping.
- **Replace lifecycle frames with table assignments.** This removes real
  duplicate logic in `web/src/lib/observation/index.ts:99-131`, while leaving
  the reducer ports, provenance, message revisions, and connection guard
  intact. Whole per-turn usage replacement makes the empty-report case a
  straightforward browser assignment.
- **Replay nested session paths and stop at EOF.** The writer uses
  `sessions/<session-id>.jsonl` (`event_persistence.go:138`); session IDs
  contain their creating scope path (`session.go:21`). Claude correctly
  avoids a fixed-depth glob and the following behavior of `runlog.Read`.

### Weaknesses and required corrections

1. **Use direct, serialized writes before adding a flusher.** The draft
   introduces dirty bits, a flushing flag, replay suppression, a drain, and
   a condition variable or polling loop without evidence that straightforward
   writes cannot meet this run scale. Holding the existing store mutex
   through encoding and atomic replacement is the smaller starting point.
   The current checkpoint helper already supplies the temp-and-rename
   operation (`internal/observation/checkpoint.go:16-29`); its current use
   outside the lock is safe because it writes only after close, not because
   concurrent writers are safe.

   If the proposed flusher remains, its pseudocode clears dirty state before
   a write succeeds (draft lines 344-378). A failed rename can therefore
   leave an absent or stale table with no dirty bit for `Close` to drain.
   Returning the error is acceptable; claiming that every table reached disk
   after a subsequent successful `Close` is not. Also, the proposed one-line
   `r.store.Lifecycle(record)` drops a returned error: today's method returns
   nothing, and `run.go:172` has no error handling. Explicitly pass write
   failures to the existing `recordFailure`, as `observeAgent` already does
   at `run.go:189-195`. No retry subsystem is needed.

   Delete the risk-section fallback that waits until a turn boundary to
   flush step facts (draft lines 710-715). That changes the fixed cadence.

2. **Preserve the required decision sequence.** `Decisions []json.RawMessage`
   containing only the planner event drops `decision.seq` (draft lines
   181 and 269). The sequence lives on the outer lifecycle record:
   `event_persistence.go:41-49`; `PlannerDecision` itself has only `task`
   (`events.go:71-75`). Embed `{seq, body}` in the scope row, retaining the
   recorded sequence. Adjust `RunViewer`'s decision display to read `body`;
   it currently reads `decision.task` directly at lines 13-15. This is a
   required fact, not a request for another table file.

3. **Initialize all six files, including empty arrays.** Live folds mark
   only changed tables dirty. Only replay explicitly marks all six dirty
   (draft lines 392-403). A run without model steps would otherwise omit
   `model_calls.json`; a run without sessions has more untouched tables.
   Create each empty table as `[]` through the same writer when opening
   the store. Do not restore a final checkpoint write to fill this gap.

4. **Make the phase boundaries match compilation dependencies.** Phase 1
   deletes `RunInfo.ScopeUsage`, `Store.ScopeUsage`, `SessionInfo`, and
   `Lifecycle`; `usage.remote.go` still uses the first two, and
   `web/observation_live_test.go:92-95` and
   `web/observation_ssr_test.go:35-38` still construct the old types.
   Their removal or update is assigned to Phase 3. Move those changes and
   generated bindings into the same compiling phase as the API replacement,
   or treat the migration as one phase. Do not add temporary compatibility
   types to make the original outline appear green.

5. **Finish the wire details.** The `row` example names singular tables
   (`scope`, `session`, `turn`, `model_call`) but the snapshot uses plural
   keys. `state[table][key] = row` therefore is not the promised assignment
   as written. Use matching names. Explicitly update the EventSource
   listener list in `RunViewer.svelte:29` for `row` and `totals`; changing
   `RunObservation.apply` alone does not receive a named SSE event.
   Preserve the existing snapshot/subscription cut and terminal queue drain
   (`subscribe.go:128-145,158-170`). These mechanisms already exist.

6. **Do not make permanent finished-store retention a prerequisite.** The
   intent limits scopes and turns per run, not the number of project runs
   or transcript bytes. The draft's assumption of tens of project runs is
   additional. Replaying on a cold request is the smaller baseline; caching
   may be useful, but is not required to implement the chosen transcript
   path. Whichever policy is selected, serialize cold reconstruction of the
   same run so concurrent snapshot requests cannot write its fixed temp
   filenames simultaneously. An ordinary registry lock is enough; no cache
   framework or eviction machinery is warranted here.

7. **Correct and tighten the evidence claims.** The saved issue-149 run has
   four scopes including `""`, three sessions, six turns, and six ended
   model steps. Claude's literal per-model usage sums and 156,974-byte log
   size check out; its Phase 2 expectation of three scopes does not. Retain
   the good practice of replaying a copy in `t.TempDir()`.

   The live proof program emits no values, planner decisions, or task-bearing
   scopes, so its screenshots cannot demonstrate those parts of page parity.
   Keep the existing scope-content checks and update them for the new rows;
   use the manual browser observation for the content the proof actually
   emits. A screenshot alone also cannot establish an increase over time:
   record the observed before/after totals during that same run. Rename
   counting and a new performance-report requirement are unnecessary.

## Gemini draft

| Dimension | Assessment |
| --- | --- |
| Architectural soundness | Shares the right broad direction, but replay can corrupt final usage, concurrent writes are unordered, and lifecycle decoding is unresolved across the package boundary. |
| Completeness | Restates the required schema well, including an embedded decision with `seq` and `body`. Omits model/start correlation, tables from its concrete snapshot, important browser wiring, and a clear close/error path. |
| Phasing and ordering | Storage, ingestion, replay, page, proof is a reasonable outline. The snapshot replacement still affects existing consumers before their assigned phase, and the plan omits generated bindings and affected web tests. |
| Risk coverage | Too dependent on unsupported timing assertions. Does not cover the actual concurrent writer and replay-order risks or preserve the existing subscriber lifecycle explicitly. |
| Feasibility | The direct-write approach is economical once serialized. The draft needs concrete corrections before an implementer can execute it without making architectural decisions left unstated. |
| Definition of done | Names all the intent's outcomes and required commands, but mostly repeats them. It lacks Claude's concrete expected accounting and does not connect its browser instructions to a functioning live update path. |

### Strongest ideas to keep

- **Direct atomic writes, with no debounce worker.** This is the preferable
  initial cadence at the stated scale. Choose it for simplicity, not the
  claimed SSD timings or supposed hard ceiling on model event rates.
- **One package and full-log replay.** These avoid a new persistence boundary
  and a lazy transcript API. Neither alternative inherently requires the
  byte-range indexes, duplicated types, or wrappers the draft says it does;
  the actual argument is that the existing fold already does the work.
- **Explicit embedded `{seq, body}` decisions and slash-boundary containment.**
  Both follow the fixed schema. The containment rule already exists in
  `internal/observation/usage.go:91-92`; apply it to turn placement.
- **A small proof approach.** Manual arithmetic and one cheap-tier live run
  with restart fit the intent without extra validation infrastructure.

### Weaknesses and required corrections

1. **Replay must not mutate completed-turn usage from steps.** The draft
   reads all lifecycle records and then session events (lines 229-240,
   308-313), but every ended step updates `turn_usage` (lines 206-210).
   If updates add, the saved final report is double-counted; if they set,
   the last step replaces the report. Model names can also reappear after
   a final report removed them. Copy Claude's ended-turn guard, including
   the empty-report case, while still replaying projections and model calls.
   One Go sum function alone does not ensure live/replay equality when its
   input maps differ.

2. **Outside-lock writes are not ordered atomic persistence.** Two Group
   producers can both open `table.json.tmp`, overwrite each other's bytes,
   or rename an older detached snapshot over a newer table. A mutex around
   map mutations alone does not prevent this. Keep the direct approach but
   serialize reduction, encoding, and replacement using the existing store
   lock. Keep frames ordered under that same reduction lock. The claims of
   sub-millisecond writes and nonexistent contention are not measurements
   and do not address these races.

3. **Resolve replay decoding without an import cycle or following loop.**
   `LifecycleRecord` belongs to package `gimble`, which already imports
   observation (`run.go:14`). `Store.Lifecycle` currently accepts an
   observation-specific entry, not that root record
   (`internal/observation/store.go:25-40,113`). The proposed
   `runlog.Read[LifecycleRecord]` inside observation cannot simply call it.
   Use one private JSON decoder as Claude proposes.

   Also, `runlog.Read` follows EOF until `complete`, rather than returning
   the available records (`internal/runlog/reader.go:16-23,52-69`). That is
   unsuitable for a cold open of a log that never completed. A plain
   EOF-bounded reader suffices; no new recovery system is needed. Discover
   session files recursively or from their recorded IDs, not the displayed
   fixed-depth glob. Replay into a private store and expose it only after
   reconstruction succeeds; `observation.Open(registry, ...)` currently
   registers immediately (`store.go:103-105`).

4. **Model-call facts need the start event.** The plan extracts the model
   from step-end events, then elsewhere says to use the session default.
   The saved logs' ends contain tokens and cost but no model or start time.
   Remember the start's model and `created`, keyed by turn and normalized
   assistant message; preserve the end's `created`. The configured model
   is only a fallback when the start lacks one. Normalize lifecycle
   `time.Time` to Unix ms and `TurnEnded.Duration` from nanoseconds to ms;
   `events.go:119,165` confirms that copying these fields is insufficient.

5. **The browser simplification is based on nonexistent code.** There is
   no `rollup`, `contains`, or scope sum in `index.ts`. It assigns a running
   session total at line 79; the root query already sums in Go. Removing
   that assignment is useful, but keeping raw lifecycle frames means
   extending `foldLifecycle` for the new session/turn maps, not eliminating
   it. Prefer Claude's assignment frames to achieve the stated shrinkage.

   The concrete snapshot (lines 478-486) omits `turn_usage` and `model_calls`
   despite promising all tables. Add them. Register `totals` in
   `RunViewer.svelte:29` and read the header from the mutable observation,
   not the original SSR `data.snapshot`. Index session totals by
   `turn.session`: the `sessionID` inside `SessionTimeline` is a normalized
   transcript ID, while the current total lookup deliberately uses workflow
   placement (`SessionTimeline.svelte:17-22`).

6. **Retain close and recording-error propagation.** Deleting the complete
   `r.recordFailure("close observation", r.store.Close())` statement
   (draft line 317) removes the only runtime call that closes the store.
   Remove checkpoint persistence, retain finalization and registry removal,
   and propagate table-write failures through the existing recording-error
   path. `run.go:99-106` deliberately performs this before writing
   `complete`. Preserve cancellation precedence too: `run_cancelled` is
   followed by `run_ended` (`run.go:88-94`), and today's store prevents the
   latter from overwriting cancelled status (`store.go:125-136`).

7. **Complete the migration and its focused checks.** Initialize untouched
   tables as arrays; retain detached snapshots rather than exposing mutable
   row pointers; update the SSR fixture and remove or replace the obsolete
   remote-query test. Regenerate the query bindings and resolve the `Usage`
   import in `index.ts:2` when deleting the remote. These are existing
   dependencies, not optional cleanup. The stated Go 1.24+ prerequisite is
   wrong for this checkout: `go.mod` requires 1.27.1.

   In the existing tests, explicitly exercise final-report replacement with
   an empty report, exclusion of failed steps lacking cost or tokens, final
   replay totals, and receipt of the new frames. Replay the saved logs into
   a temporary copy rather than rewriting the source evidence directory.
   Observe live totals and transcripts before restart and compare the same
   numbers afterwards. These small corrections make the stated definition
   of done demonstrable without adding proof machinery.
