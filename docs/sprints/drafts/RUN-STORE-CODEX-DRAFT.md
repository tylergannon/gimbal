# Sprint Draft: The run store

Unnumbered. Based on [RUN-STORE-INTENT.md](RUN-STORE-INTENT.md).
Its settled decisions govern this draft; Sprint 001 supplies the document
structure. This sprint builds the store and keeps the current page working.

## Pyramid Index

- L0: Replace the final observation checkpoint with explicit run facts,
  written through to six JSON files, with turn-based totals computed in Go
  and the current page working live and after restart.
- L1: Persist the agreed rows; charge usage to turn placement; load saved
  facts and reconstruct transcripts; replace browser lifecycle folding
  with server-produced state; validate with existing tests and one live run.
- L2: Architecture defines files, folding, reopening, and frames;
  Implementation Plan assigns four phases; Definition of Done states
  acceptance; Open Questions gives the four recommendations.

## Overview

Replace `internal/observation`'s close-time checkpoint with the intent's
fact tables and its creation-scope accounting with turn placement.
`turn_usage` becomes the sole accounting source. The snapshot supplies
header and session totals, replacing the separate `scopeUsage` query.
The flat scope list, cards, and transcripts remain. #173 gets its required
facts, but no tree, bars, prices, database, message table, or root exports
are added. Adapters, logs, and both reducer ports stay unchanged.

## Use Cases

1. Watch a run: scopes and transcripts update, and root and session totals
   reflect step usage and the final turn report.
2. Create a session at the root and generate in a child: the turn records
   the child, and its usage counts toward that child and its ancestors.
3. Restart the server: the same page renders saved facts and log-backed
   transcripts without starting an agent.
4. Open the saved issue-149 run with only its logs: the store reconstructs
   the tables and the page renders all six turns.
5. Inspect a run with `jq`: join `turns.json` and `turn_usage.json` to sum
   tokens by model for a scope, without reading transcript projections.

## Architecture

### Rows and files

Keep ownership in `internal/observation`. `snapshot.go` declares the
concrete rows and read shape; `store.go` folds records; `tables.go` owns
the six explicit JSON reads and atomic writes; `replay.go` reads logs.
There is no storage interface or sibling DAL package.

Use every field in the intent's schema, with these physical groupings:

| File | Contents and map identity within one run |
| --- | --- |
| `run.json` | One-element array containing the run row, keyed by id |
| `scopes.json` | Scopes keyed by slash path, including values keyed by value key and decisions with log sequence and body |
| `sessions.json` | Sessions keyed by id, including creation scope and parent |
| `turns.json` | Turns keyed by id, including execution scope and session |
| `turn_usage.json` | Usage keyed by turn and model |
| `model_calls.json` | One row per qualifying terminal step, grouped by turn; message points into the transcript |

All six files are arrays, including empty tables. Retain the intent's run
and relationship fields and deterministic row order. Convert recorded
timestamps and duration to milliseconds. Preserve task, value, decision
body, and result JSON, decoding source strings where the log requires it.

Each mutation holds the store lock through folding and rewriting the
affected files: temp file in the same directory, close, rename. Initialize
all six files when opening a new run. Rewrite each affected file once per
accepted record; do no disk work for text deltas or other events that
change only a transcript projection. No timers or background flush queue.
Atomicity is per file, not a transaction across six files; live page reads
use the maps under the same lock.

### One fold and one accounting rule

Move lifecycle decoding into observation using a small internal record
shape. Live ingestion and replay pass exact log JSON to the same fold,
replacing `run.observeLifecycle`'s relationship switch without introducing
an observation-to-gimble import cycle or changing public log types.

- Lifecycle records set run and scope status/times, values, decisions,
  session metadata, and turn start/end facts. Preserve cancellation as
  terminal and the existing distinction between an ended scope and a
  successful run. Use `planner_decision`, the actual recorded kind.
- On `session.step.started`, remember the message's model and start time
  from the event/projection. A qualifying terminal step writes its
  `model_call` row and increments that turn/model's `turn_usage` row.
  Use the step model id, falling back to the configured session model
  only when absent. Failed steps qualify only with both cost and tokens,
  as `session.go:stepUsage` already specifies. Model-call times use the
  stamped step events; an absent start remains unset rather than invented.
- At every `turn_ended`, delete all usage rows for that turn and insert
  exactly the recorded report, even when empty. `session.go` already
  selects harness usage or its step fallback before writing that record;
  preserve this behavior. Terminal replacement may lower totals or change
  the model keys. It never removes model-call facts.
- `session.usage.updated` still reaches the unchanged transcript reducer
  and log, but never becomes an accounting row or a page total.

`usage.go` sums only `turn_usage`, joining through `turns` for placement
and session identity. A scope contains itself and paths beginning with
its key plus `/`; `""` contains all turns. Thus `task.1` excludes
`task.10`. Compute per-model totals and a combined total for each scope
and session; the combined total preserves today's header/card line.
These are derived response fields, never persisted. Model calls are
drill-down facts and never a second input to sums.

### Opening a finished run and rebuilding

Load the six files into the same maps. For transcripts, scan each session
log once, dispatch by recorded turn id, and apply the shared
projection/provenance helper. Today's page requests every turn: hydrate
them within the snapshot request, with no new endpoint. Hydration never
runs the accounting fold over loaded facts.

If table files are missing or cannot be decoded, reconstruct the whole set
from the logs through the same lifecycle and native-event folds used live.
Read native records in session sequence, grouped by turn; while walking
the lifecycle log in sequence, feed each turn's native records after its
start and before its end report. This preserves final report replacement
without pretending independent log sequence numbers form one global
sequence. Cross-turn interleaving is unnecessary for a finished snapshot.
The rebuild already has projections, so do not hydrate them again.

Use a finite read for this operation. `internal/runlog.Read` currently
tails until `complete`; preserve that contract and share its line-reading
logic with a finite internal-package reader. A saved log without
`complete` must return an explicit incomplete-log error rather than leave
an HTTP request waiting indefinitely or claim completion. Read nested
session paths as written today (`sessions/<session-id>.jsonl`, where the
id itself contains slashes). Never read the old checkpoint.

Keep the store registered through final table writes and log finalization,
then close subscribers and unregister it. `Close` writes no snapshot.
Table failures enter the existing recording-error path and `complete`
record. Serialize lifecycle append plus fold so concurrent producers
publish in log order.

### Snapshot and frames

`RunSnapshot` contains `run`, `scopes`, `sessions`, `turns`, `turn_usage`,
`model_calls`, derived `scope_totals` and `session_totals`, and
`invocations` holding transcript snapshots/provenance for every observed
turn, live or finished. The root's total is `scope_totals[""]`.

Use three frame names:

| Frame | Payload and browser action |
| --- | --- |
| `snapshot` | Complete facts, totals, and projections; replace everything on connect/reconnect |
| `state` | Complete fact maps and totals, excluding transcript projections; replace those fields |
| `event` | Existing placed native event and sidecar; apply the unchanged transcript reducer |

Publish `state` when lifecycle facts change and whenever `turn_usage`
changes, including an empty terminal replacement. Recompute totals on
usage changes in Go. Publish the native event and resulting state in the
same locked mutation, preserving the snapshot-and-suffix cut and existing
bounded subscriber behavior. At this scale, a complete fact replacement
is simpler than table-specific patch types; transcripts stay incremental.

Delete browser lifecycle and session-total folds. Preserve connection
guards, native reduction, provenance, and message revisions across state
frames. `RunViewer` reads the root total; `SessionTimeline` reads session
totals; `MessageRow` keeps displaying native message data.

## Implementation Plan

### Phase 1: Facts, accounting, and write-through

Files: observation `snapshot.go`, `store.go`, `usage.go`, new `tables.go`;
`event_persistence.go`, `run.go`; existing store, usage, and root tests.

1. Declare the agreed rows/maps and six file encodings; initialize empty
   tables and add atomic writes with propagated errors.
2. Fold exact lifecycle JSON, including times and turn reports. Wire the
   root writer to this fold, keeping append/fold order consistent.
3. Fold terminal step facts and provisional usage; replace usage wholesale
   at turn end. Derive scope/session totals exclusively from those rows.
4. Replace session-accounting tests with placement, sibling-prefix,
   multi-model, empty-report, failed-step, and no-double-counting cases.
   Check files during a run, timestamp units, and write-error propagation.

### Phase 2: Saved runs and log reconstruction

Files: new observation `replay.go`, `tables_test.go`, `replay_test.go`;
`registry.go`, `store.go`, `doc.go`; runlog `reader.go`, `reader_test.go`;
`run.go`; delete `checkpoint.go`.

1. Load table arrays into the store maps and hydrate transcripts with the
   shared projection helper, reading each session log once.
2. Rebuild missing tables through the normal folds with per-turn native
   events preceding terminal reports. Keep finite reads separate from
   the existing tailing contract.
3. Remove checkpoint reading/writing and place close/unregister after log
   finalization. Remove stale checkpoint assertions and documentation.
4. Test table load versus rebuild, nested session paths, empty usage
   replacement after replay, finite failure for incomplete logs, and
   transcript/provenance retention. Use the existing issue-149 logs in a
   temporary copy; do not add a fixture framework or mutate the saved run.

### Phase 3: Keep the page working through one stream

Files: observation `subscribe.go`, `http.go`, `store_test.go`;
web observation `index.ts`, `index.test.ts`, `RunViewer.svelte`,
`SessionTimeline.svelte`; run route `+page.svelte`; web live/SSR tests.

1. Send the snapshot/state/event shapes and preserve subscription ordering,
   overflow handling, and terminal-frame draining.
2. Replace browser lifecycle folding with fact-map replacement, retaining
   incremental transcript rendering. Feed header and card totals from Go.
3. Delete `runs/[runID]/usage.remote.go`; regenerate skgo bindings and
   remove obsolete generated usage-query artifacts through the generator.
   Keep needed display types local if removing the remote removes their
   generated TypeScript source. Do not hand-edit generated files.
4. Retarget `web/observation_live_test.go` from the removed remote to the
   observation SSE endpoint through `NewHandler`. Extend the existing SSR
   test to a reopened run with totals and transcripts. TS tests cover
   state replacement, empty totals, reconnects, and retained message
   revisions; they implement no accounting oracle.

### Phase 4: Build and see it working

Run the checks below sequentially, then the intent's one cheap-tier live
run. Use an ordinary inline workflow with a parent-created session used
in a child and a joined `Group` of two. Observe the existing page live,
after completion, and after server restart. Open a temporary logs-only
copy of issue-149. The validator checks the relevant table rows and sums
by hand with `jq`. No reconcile script, fixture harness, or new proof
machinery is part of this sprint.

## Files Summary

| Area | Changes |
| --- | --- |
| `internal/observation/{snapshot,store,usage}.go` | Rows/maps, shared folds, derived totals, snapshots/frames |
| `internal/observation/{tables,replay}.go` | New: explicit file I/O, finite reconstruction, projection hydration |
| `internal/observation/{registry,subscribe,http,doc}.go` | Saved-run loading, lifecycle of the store, stream contract, docs |
| `internal/observation/checkpoint.go` | Delete |
| `internal/observation/*_test.go` | Adapt existing tests; focused table/replay tests |
| `internal/runlog/reader.go`, `reader_test.go` | Finite reading without weakening existing tailing behavior |
| `event_persistence.go`, `run.go`, `gimble_test.go` | Exact-record handoff, ordering, errors, finalization, runtime assertions |
| `web/src/lib/observation/{index.ts,index.test.ts,RunViewer.svelte,SessionTimeline.svelte}` | State replacement and server totals; current transcript rendering |
| `web/src/routes/runs/[runID]/+page.svelte`, `usage.remote.go` | Remove separate usage subscription and remote |
| `web/observation_live_test.go`, `web/observation_ssr_test.go` | Current SSE and saved-page behavior |
| skgo generated bindings/manifests/types | Regenerate after remote removal |

## Definition of Done

1. Every completed run contains the six named JSON arrays, with all agreed
   facts and no new `observation.json`; writes happen while the run is live.
2. Child-placed turns bill their execution scope and ancestors. Per-model
   totals use only `turn_usage`; final reports replace provisional usage,
   including an empty report. Session cards use the same source.
3. The current page shows its header, root totals, scopes, session cards,
   and transcripts live and after restart. Usage changes publish totals
   through the observation stream; the browser has no sum rule or separate
   `scopeUsage` subscription.
4. A logs-only copy of the saved issue-149 run creates all six files and
   renders its three sessions and six turns with transcripts.
5. The validator observes one live workflow using the cheap tier
   (`claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`),
   records the actual models used, checks parent/child placement and the
   group of two, and checks scope/model arithmetic directly from
   `turns.json` and `turn_usage.json`. The restarted page agrees.
6. `just build`, `go test ./...`, `go vet ./...`, `cd web && pnpm test`,
   and `cd web && pnpm run check` pass, in that order.
7. No new root exports, adapter/log/reducer changes, tree, pricing, database,
   compatibility reader, or proof framework. Validation requests remain
   tied to these requirements under `docs/definition-of-done.md`.

## Risks & Mitigations

- **Disk latency under the lock.** Only fact changes rewrite files; tokens
  streamed as text do not. With hundreds of turns, start synchronous and
  revisit only if the live run shows a problem.
- **Partial file set or stale checkpoint.** Rebuild the whole table set
  from logs when files are absent/unreadable. Never consult the checkpoint;
  no multi-file transaction protocol is introduced.
- **Replay overwrites final usage with provisional steps.** Feed a turn's
  native history before its terminal report and test empty replacement.
- **Transcript hydration doubles usage.** Share projection application,
  not accounting mutation, when the fact maps came from files.
- **Close/open race.** Keep the live store available until final records
  are written; finite saved-run reads cannot tail an unfinished log forever.
- **Generated usage types disappear with the remote.** Move the small
  display-only type to the observation client and regenerate bindings.

## Dependencies

Existing logs, observation/subscription code, reducer ports, and skgo
`NewHandler`; no new package dependency. Use the repo's toolchains and
authenticated cheap-tier harnesses. Build before web Go tests. #173 and
#172 are later consumers. No semantic-index or token-cache work.

## Open Questions

1. **Finished-run transcripts — load tables, hydrate from session logs on
   snapshot demand.** Full replay alone would be fewer lines if it replaced
   table loading altogether, but the settled design includes loading the
   files. Within that constraint, six explicit decodes plus one shared
   projection helper avoid both replaying all fact writes on each open and
   a new transcript endpoint. The current page needs every turn, so batch
   that demand into one read per session. Reserve full fact replay for
   reconstruction.
2. **Snapshot shape — facts, derived scope/session totals, all turn
   projections.** Use full `snapshot`, fact-only `state`, and native `event`
   frames. State replacement deletes browser lifecycle/accounting folds
   while preserving the native reducer and current page.
3. **Write cadence — synchronous, once per changed table per record.**
   Model-call and usage files change at step boundaries, not text deltas.
   The agreed scale does not justify coalescing or a second flush path.
4. **Go ownership — keep it in `internal/observation`.** Add concrete file
   and replay code beside the existing maps and fold. A sibling package
   would split one store across boundaries without a second consumer or
   storage implementation to justify it.
