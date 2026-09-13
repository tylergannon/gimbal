# Sprint Intent: The run store

Unnumbered by Tyler's instruction. No token cache or semantic index is used
for this sprint; see `docs/SEMANTIC-INDEX.md` for what exists.

## Seed

Design and build the deliberate, file-based data store for a run: the store
that replaces `observation.json`. It must support the run page as it is
today and as it is planned (#173: usage, time, and cost by scope, one tree
from run to scope to turn to model call). It does not build the tree.

Tyler, 2026-09-13, on what is wrong today: `observation.json` "is a name
without a meaning, a file without a name, a data store that feels too
improvised to really be able to work with it. Nobody has spent any time
thinking about 'data store'. It just happened." And on what he wants: "We
DEFINITELY want to be storing data in a way that the roll-ups are easy to
access." Any project directory is self-contained; nothing centralized.
Not sqlite for now: "it's a question of what's strictly easier to build
and maintain, for the moment."

## Decisions already taken (constraints, not questions)

These were settled with Tyler before this sprint. Drafts build on them and
do not reopen them.

1. **Seven tables of facts, no stored aggregates.** Times are Unix ms.

   ```
   run          id, name, status, error, started, ended
   scope        run, key, name, status, error, task, began, ended
   scope_value  run, scope, key, value
   decision     run, scope, seq, body
   session      run, id, name, adapter, model, scope, parent, created
   turn         run, id, session, scope, prompt, output_type, result,
                error, interrupted, started, ended, duration
   turn_usage   run, turn, model, input, cache_read, cache_write,
                output, reasoning, stated_cost
   model_call   run, turn, message, model, input, cache_read, cache_write,
                output, reasoning, started, ended
   ```

   `scope.key` is the slash path (`sprint.1/task.2`); ancestry is a prefix
   match; the root is `""`. `session.scope` is where the session was
   created; `turn.scope` is where the turn ran. They differ, and that is
   the placement correction #157 asked for. `turn_usage` is one row per
   model per turn, written from steps as they end and replaced wholesale
   from the harness's report at `turn_ended` (including an empty report).
   Every roll-up reads `turn_usage` only, live or finished. `model_call`
   is one row per `session.step.ended` (or `session.step.failed` carrying
   both cost and tokens), the drill below a turn, never summed. Session
   totals are a sum of `turn_usage` by session; the `session.usage.updated`
   running total is not stored.

2. **One JSON file per table under `runs/<id>/`**, named for the table:
   `run.json`, `scopes.json` (with each scope's values and decisions
   inside its row), `sessions.json`, `turns.json`, `turn_usage.json`,
   `model_calls.json`. Each is a JSON array of rows, rewritten whole and
   atomically (write temp, rename) whenever one of its rows changes. The
   `turn_usage` replacement at turn end is just the next rewrite. Files
   are greppable with `jq`. The run directory is self-contained.

3. **Live and finished are one path.** The in-memory store is the seven
   tables as Go maps. A live run's page reads the maps and streams frames.
   Opening a finished run loads the files into the same maps. The sum rule
   (tokens and stated cost per scope by prefix containment, per model)
   runs in Go over the maps in both cases, and the server publishes
   recomputed per-scope totals as a frame whenever a `turn_usage` row
   changes. The browser never implements the sum rule.

4. **The files are derived from the log and rebuildable.** A run directory
   with `run.jsonl` and `sessions/*/*.jsonl` but no table files is rebuilt
   by replaying the log through the same store on open. `observation.json`,
   `checkpoint.go`, `loadCheckpoint`, and the write at `Store.Close` go
   away. This is what #169 becomes.

5. **Transcript content stays in the log.** No message table. Message
   text, deltas, tool calls, and thinking are the session log's, and
   `model_call.message` points at the assistant message id.

6. **Scale assumption.** Tens of scopes, at most a few hundred turns per
   run. Nothing needs a query engine. If that changes, the seven tables
   become a sqlite schema behind a sqlc DAL and the loader is the
   migration; nothing in the page changes because it reads maps.

## Context

- Restarted repo, 2026-09-10. Rule: as simple as possible. `AGENTS.md`:
  no wrappers, no new exported names without being asked, ports under
  `internal/sessionstate/` and `web/src/lib/sessionstate/` untouched, the
  page's Go imports `gimble` and never the reverse.
- What exists (main at `ad6f2f6`): `internal/observation` is the store.
  `Store.Lifecycle(Lifecycle)` folds a lifecycle record (it receives the
  exact record JSON plus decoded relationships); `Store.Event(Placement,
  envelope, nativeRef)` applies a native event to the turn's projection
  (`invocationLocked` creates one per turn id on first event) and folds
  `session.usage.updated` into `RunInfo.Usage`. `Snapshot()` detaches
  `RunSnapshot{Run, Scopes, Invocations}`; `Invocation` carries placement,
  a full `sessionstate.Snapshot` projection, and provenance. `Close()`
  writes `observation.json` and removes the store from the `Registry`;
  `Registry.Snapshot(id)` serves a live store or `loadCheckpoint`.
  `subscribe.go` publishes frames `snapshot`, `lifecycle`, `event`.
  `http.go` serves the endpoints. `usage.go` holds `Usage/Tokens/Cache`
  and `RunInfo.ScopeUsage` (sums session totals by creating scope: the
  wrong key). `internal/runlog/reader.go` has `Read[T]` that replays and
  follows a run log until a `complete` record.
- Writers: `event_persistence.go` appends `run.jsonl` (lifecycle records
  with `seq`, `time`, `scope`, `session`, `turn`, `event`) and one
  `sessions/<id>/<id>.jsonl` per session (native events with `nativeRef`).
  `run.go:observeLifecycle` and `session.go` hand records to the store.
- Lifecycle event kinds in `events.go`: `run_started`, `run_ended`,
  `scope_began` (name, task), `scope_ended` (error), `session_created`
  (name, adapter, model, parent, workdir), `value_set`, `decision`,
  `turn_started` (prompt, output type), `turn_ended` (result, error,
  usage `[]ModelUsage`, duration ns, interrupted), `complete`.
- Page: `web/src/routes/runs/[runID]/+page.svelte` subscribes to the
  `scopeUsage` remote query (`usage.remote.go`) for a header line;
  `RunViewer.svelte` renders the run header, a flat scope list, and a
  flat invocation list with `SessionTimeline.svelte` (session card with
  the session total from `run.usage`) and `MessageRow.svelte`.
  `web/src/lib/observation/index.ts` mirrors the store: `replace` on a
  snapshot frame, `foldLifecycle`, `apply` for events. Tests:
  `internal/observation/store_test.go`, `usage_test.go`,
  `web/observation_live_test.go`, `web/observation_ssr_test.go`,
  `web/src/lib/observation/index.test.ts`.
- Saved run for replay: `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/`
  (three sessions in three scopes, two turns each, logs plus an
  `observation.json` that predates #146 and has no scopes).
- Planned consumer: #173 (the tree) needs scope times, turn records with
  prompt/output type/result/error/interrupted/times/duration, per-model
  usage per turn, model calls, and per-scope totals. #172 supplies prices
  later; not this sprint.

## Success criteria

1. After a live run, `runs/<id>/` holds the six table files and no
   `observation.json`. `jq` over `turns.json` and `turn_usage.json` answers
   "tokens per model for scope X" in one filter; the intent's arithmetic
   is done by hand by the validator on the proof run.
2. The current run page works unchanged in what it shows, live and
   finished, from the new store: run header with the root's totals, scope
   list, session cards with totals, transcripts. The `scopeUsage` remote
   query and its header line are replaced by the snapshot's totals.
3. A turn is charged to the scope it ran in. A session created at the root
   and used in a child scope shows its turn under the child.
4. The saved issue-149 run opens with no table files present, gets them
   written, and renders.
5. Per-scope totals arrive as a frame when `turn_usage` changes; the
   browser holds no sum rule.
6. `just build`, `go test ./...`, `go vet ./...`, `cd web && pnpm test`,
   `cd web && pnpm run check` pass. One live proof run on the cheap tier
   (`claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`)
   with a session created in a parent and used in a child scope and a
   `Group` of two, observed live and after restart. No proof machinery:
   no reconcile scripts, no fixture harnesses.

## Out of scope

The tree and its cells (#173). Prices (#172). Any message table. Any
database. Any change to adapters, the log format, or the reducer ports.
Backwards compatibility with `observation.json`: it is deleted, not read.

## Open questions for the drafts

1. **Finished-run transcripts.** The page needs a turn's transcript
   projection for a finished run. The projection is not a table. Two
   candidates: replay the whole log through the store on open (tables and
   projections in one pass, files rewritten as a byproduct, no loader) or
   load the table files and rebuild a turn's projection from the session
   log on demand. Say which is less code and why.
2. **The snapshot shape.** What `RunSnapshot` becomes (the tables plus
   per-scope totals plus projections for live turns?), and what frames the
   browser receives, so `index.ts` shrinks rather than grows.
3. **Write cadence.** Every row change rewrites its file. Is any file hot
   enough on a live run (model_calls on every step) that the write should
   be coalesced, and if so how, without a second code path.
4. **Where the tables live in Go.** `internal/observation` grows the maps
   and the writer, or a sibling package owns rows and files while
   `observation` keeps the fold and the subscribers. Which is less to
   read.
