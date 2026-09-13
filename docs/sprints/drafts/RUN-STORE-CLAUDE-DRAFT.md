# Sprint: The run store (Claude draft)

Drafted 2026-09-13 from `RUN-STORE-INTENT.md`. Unnumbered. The intent's
six decisions are taken as given; this draft says how they become code,
answers the four open questions, and keeps the current run page showing
what it shows today. It builds no tree (#173), no prices (#172), no
message table, no database.

## Pyramid Index

- L0: One store, seven tables of facts as Go maps, one fold fed by log
  records on the live path and on replay, six JSON files under
  `runs/<id>/` rewritten atomically as rows change, per-scope totals
  computed in Go and pushed as a frame. `observation.json` is deleted.
  The page shows what it shows today, from the new store, live and
  finished.
- L1:
  - The four answers. (1) Replay the whole log on open; no loader, no
    on-demand projection. (2) `RunSnapshot` is the six tables as maps plus
    `totals` plus `transcripts`; frames are `snapshot`, `row`, `totals`,
    `event`; the browser folds nothing but native events. (3) A dirty set
    and one flusher; a flush in progress absorbs the mutations that land
    during it, so hot files coalesce under load with no timer and no
    second path; replay flushes once at the end. (4) `internal/observation`
    grows; no sibling package.
  - The fold moves into the store. `Store.Lifecycle` takes the record
    JSON and decodes what it needs; `run.observeLifecycle` becomes one
    line and the `Lifecycle`/`ScopeChange`/`ValueChange` structs go away.
    One decoder serves the live path and the replay.
  - Placement: a turn row carries the scope it ran in; `turn_usage` is
    keyed by turn; the sum rule is prefix containment over turns. A
    session created at the root and used in a child bills the child.
  - Page: `index.ts` loses `foldLifecycle` and the usage fold; a `row`
    frame sets `state[table][key]`; the header line and the session card
    read `totals`. The `scopeUsage` remote query and its test are deleted.
  - Proof: one cheap-tier run with a parent-created session used in a
    child and a `Group` of two, seen live and after a server restart; `jq`
    over `turns.json` and `turn_usage.json` against the page's numbers; the
    saved issue-149 run opened from its logs alone.
- L2:
  - Architecture § "Answers" settles the four questions; § "Rows and
    maps" is the schema; § "The fold" the rules; § "Files" the writer;
    § "Opening a finished run" the replay; § "Frames and the page" the
    wire.
  - Implementation Plan phases 1 to 4, each compiling and green before
    the next.
  - Definition of Done maps the intent's six criteria to what is seen.

## Overview

Today `internal/observation` keeps run info, sessions with running usage
totals, scopes, and one projection per turn, and writes all of it as
`observation.json` at `Close`. A finished run is that file. Turns have no
record of their own: no prompt, no times, no per-model usage, no scope of
their own beyond the invocation's placement. Scope usage is summed by the
session's creating scope, which is the wrong key.

After this sprint the store is the seven tables the intent names, as Go
maps behind the store's one mutex, and the six files beside the logs. The
same `Store.Lifecycle` and `Store.Event` fold the live run and replay a
finished one. Per-scope and per-session totals are computed in Go from
`turn_usage` and published whenever it changes. The page renders the same
header, scope list, session cards, and transcripts it renders today, from
`turns`, `sessions`, `scopes`, `totals`, and `transcripts`.

## Use Cases

1. **jq over a run.** After a run, `runs/<id>/turn_usage.json` and
   `turns.json` answer "tokens per model for scope X" in one `jq` call.
   No server, no browser.
2. **The page today, from the store.** Live: header with the run's total,
   scope list, session cards with session totals, transcripts, all
   updating from frames. Finished: the same, from the log replayed.
3. **A run finished before this sprint.** The saved issue-149 directory
   has logs and an old `observation.json`. Opening it replays the logs,
   writes the six files, and renders; the old file is ignored.
4. **#173 reads the maps.** The tree needs scope times, turn rows with
   prompt, output type, result, error, interrupted, times, duration,
   per-model usage per turn, model calls, and per-scope totals. All of it
   is in `RunSnapshot` after this sprint; #173 adds no store code.

## Architecture

### Answers to the intent's four questions

**1. Finished-run transcripts: replay the whole log on open.** Opening a
finished run constructs a store and feeds it `run.jsonl` then every
`sessions/**/*.jsonl`, line by line, through `Store.Lifecycle` and
`Store.Event`. That fills the seven maps and every turn's projection in
one pass and, at the end, writes the six files. There is no loader for
the table files and no on-demand projection builder. Less code because:
the replay is the live path with a file reader in front of it (about
sixty lines); a loader is six decoders plus map indexing plus a second
`Snapshot` shape where transcripts are lazy, and the on-demand builder
still has to read the session log, so the log is read either way.
Decision 3's "loads the files into the same maps" is satisfied by the
replay filling the same maps from the same input the live path uses; the
files are output on both paths and input on neither. The registry caches
the opened store, so a page view (SSR load, JSON endpoint, SSE connect)
replays once, not three times.

**2. The snapshot shape.** `RunSnapshot` becomes the six tables as maps,
`totals` (per scope and per session, all models and by model), and
`transcripts` (one projection plus provenance per turn). Frames:
`snapshot` (the whole thing, first on every connection), `row` (one
table's value at one key, set semantics), `totals` (the whole totals
object, set semantics), `event` (a native event for the transcript,
unchanged). The `lifecycle` frame is gone. `index.ts` keeps only the
transcript reducer call and three assignments; `foldLifecycle`,
`LifecycleRecord`, the usage fold, `scopeAt`, and `parseValue` are
deleted. See § Frames and the page.

**3. Write cadence.** `model_calls.json` and `turn_usage.json` change on
every `session.step.ended`; the others change per turn or per scope.
One mechanism: each fold marks the tables it changed in a dirty set and
calls `flush`. `flush` returns at once if a flush is already running;
otherwise it loops, encoding and writing every dirty table under the
lock-then-unlock pattern until the set is empty. Mutations that land
while a write is in flight only mark the set, and the running flush
picks them up on its next loop. Idle, every change is on disk before the
call returns; under load, writes coalesce to the disk's pace. No timer,
no goroutine, no debounce. Replay sets a `replaying` flag that makes
`flush` a no-op, then clears it and flushes once. `Close` drains: it
waits for a running flush and flushes what is left. Rewrite cost is
O(rows) per write; at a few hundred turns and a few thousand model calls
that is hundreds of kilobytes a few times a second at peak, measured on
the proof run.

**4. Where the tables live: `internal/observation`.** The fold, the maps,
the sum rule, the writer, and the subscribers all run under one mutex
and touch the same maps. A sibling package would export row types, a
writer, and a lock discipline that only `observation` calls, and a
reader would open two packages to follow one write. After this sprint
the package is `rows.go` (the schema), `store.go` (the fold),
`files.go` (the writer), `replay.go` (opening a finished run),
`usage.go` (the sum rule), `subscribe.go`, `http.go`, `registry.go`.
`checkpoint.go` is deleted.

### Rows and maps

`internal/observation/rows.go`. One struct per table; the same struct is
the file row, the map value in the store, and the snapshot value on the
wire. Times are Unix ms; JSON names are the intent's column names.

```go
// Tokens is the five counts every harness reports, flat: the columns of
// turn_usage and model_call. A count the provider did not report is 0.
type Tokens struct {
	Input      float64 `json:"input"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Output     float64 `json:"output"`
	Reasoning  float64 `json:"reasoning"`
}

// Usage is the tokens and the cost the harness stated, zero when it stated none.
type Usage struct {
	Tokens
	StatedCost float64 `json:"stated_cost"`
}

type RunRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Error   string `json:"error"`
	Started int64  `json:"started"`
	Ended   int64  `json:"ended"`
}

type ScopeRow struct {
	Run       string                     `json:"run"`
	Key       string                     `json:"key"` // slash path; "" is the root
	Name      string                     `json:"name"`
	Status    string                     `json:"status"`
	Error     string                     `json:"error"`
	Task      json.RawMessage            `json:"task,omitempty"`
	Began     int64                      `json:"began"`
	Ended     int64                      `json:"ended"`
	Values    map[string]json.RawMessage `json:"values"`
	Decisions []json.RawMessage          `json:"decisions"`
}

type SessionRow struct {
	Run     string `json:"run"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Adapter string `json:"adapter"`
	Model   string `json:"model"`
	Scope   string `json:"scope"` // where the session was created
	Parent  string `json:"parent"`
	Created int64  `json:"created"`
}

type TurnRow struct {
	Run         string `json:"run"`
	ID          string `json:"id"`
	Session     string `json:"session"`
	Scope       string `json:"scope"` // where the turn ran
	Prompt      string `json:"prompt"`
	OutputType  string `json:"output_type"`
	Result      string `json:"result"` // the recorded JSON text
	Error       string `json:"error"`
	Interrupted bool   `json:"interrupted"`
	Started     int64  `json:"started"`
	Ended       int64  `json:"ended"`
	Duration    int64  `json:"duration"` // ms
}

type TurnUsageRow struct {
	Run   string `json:"run"`
	Turn  string `json:"turn"`
	Model string `json:"model"`
	Usage
}

type ModelCallRow struct {
	Run     string `json:"run"`
	Turn    string `json:"turn"`
	Message string `json:"message"` // the assistant message id
	Model   string `json:"model"`
	Tokens
	Started int64 `json:"started"`
	Ended   int64 `json:"ended"`
}
```

The store's maps, all under `Store.mu`:

```go
run         RunRow
scopes      map[string]*ScopeRow           // key
sessions    map[string]*SessionRow         // id
turns       map[string]*TurnRow            // id
turnUsage   map[string]map[string]Usage    // turn → model
modelCalls  map[string][]ModelCallRow      // turn → calls in step order
transcripts map[string]*transcript         // turn → projection + provenance (not a table)
calls       map[string]openCall            // turn + "\x00" + message → model, started (step.started seen, step.ended not yet)
dirty       tables                         // bitset over the six files
flushing    bool
replaying   bool
```

`transcript` is today's `invocation` without the placement triple:
`projection *sessionstate.Projection; provenance map[string]json.RawMessage`.
`Invocation` and `RunInfo`, `SessionInfo`, `ScopeInfo` are deleted.
`internal/sessionstate/` is not touched.

### The fold

`Store.Lifecycle(record json.RawMessage)` replaces the current
`Lifecycle` entry. The store decodes the record into one private struct
with every field any kind carries (`time`, `scope`, `session`, `turn`,
`event.kind`, `name`, `adapter`, `model`, `parent`, `error`, `task`,
`key`, `value`, `prompt`, `output_type`, `result`, `interrupted`,
`duration`, `usage []{model, cost, tokens{...}}`) and switches on `kind`.
`run.observeLifecycle` becomes `r.store.Lifecycle(record)`; the switch in
`run.go` is deleted. The record's `time` is the store's clock:
`time.UnixMilli()`.

| kind | effect |
| --- | --- |
| `run_started` | `run.Name`, `Status` running, `Started` |
| `run_ended` | if still running: completed, or failed with the error; `Ended` |
| `run_cancelled` | cancelled with the error (the terminal fact, as today); `Ended` |
| `scope_began` | `scopes[scope]`: `Key`, `Name`, running, `Task`, `Began` |
| `scope_ended` | ended, `Error`, `Ended` |
| `value_set` | `scopes[scope].Values[key]` = the value's JSON text as raw JSON (as today) |
| `planner_decision` | append the event object, raw |
| `session_created` | `sessions[session]`: name, adapter, model, `Scope` = record scope, parent, `Created` |
| `turn_started` | `turns[turn]`: session, `Scope` = record scope, prompt, output type, `Started` |
| `turn_ended` | result, error, interrupted, `Ended`, `Duration` = ns / 1e6; `turnUsage[turn]` **replaced** by the report keyed by model, an empty report giving an empty map |
| anything else | nothing (`session_closed`, `steer`, `interrupt`, `supervise_attached`, `complete`) |

After the switch: publish one `row` frame per table changed; if
`turnUsage` changed, recompute and publish `totals`; mark dirty; `flush`.

`Store.Event(at, envelope, nativeRef)` keeps its signature. As today it
decodes the event, applies it to the turn's projection (creating the
transcript on first sight) and folds provenance. New:

- `session.step.started`: `calls[turn\0message] = {model: data.model.id,
  started: created}`; the session's configured model when the event
  names none.
- `session.step.ended`, and `session.step.failed` only when `data.cost`
  and `data.tokens` are both present: take the open call (model and
  started; the session's model and 0 when none was seen), append a
  `ModelCallRow` with the five counts and `Ended` = `created`, delete the
  open call. **If the turn has not ended** (`turns[turn].Ended == 0`),
  add the counts and cost to `turnUsage[turn][model]`. Publish `row`
  frames for `model_call` and `turn_usage`, then `totals`; mark dirty;
  `flush`.
- `session.usage.updated`: nothing. The running total is not stored.
- A turn row the store has not seen `turn_started` for is created from
  the placement (id, session, scope) so a test can feed native events
  alone; live and on replay `turn_started` always comes first.

The "turn has not ended" guard is what makes replay order-free: the
replay feeds `run.jsonl` in full, so every finished turn already holds
its report, and the session logs then add model calls and transcripts
without touching a finished turn's usage. Live it is a no-op, since
`session.go` writes every step event before its `turn_ended` record.

Model keys come from the step (`claude-haiku-4-5-20251001`), not the
session's configured name (`haiku`), so the report's replacement at turn
end usually keeps the same key. When a harness reports under another
name the key changes at turn end; the totals are still right.

### The sum rule

`internal/observation/usage.go`. `contains(scope, turnScope)` is
`scope == ""`, equality, or `turnScope` starts with `scope + "/"`;
`attempt.1` does not contain `attempt.10`. `totalsLocked()` builds:

```go
type Total struct {
	All     Usage            `json:"all"`
	ByModel map[string]Usage `json:"by_model"`
}

type Totals struct {
	Scopes   map[string]Total `json:"scopes"`   // every scope key, "" included
	Sessions map[string]Total `json:"sessions"` // every session id
}
```

For every scope key in `scopes` (the root `""` is always present; the
runtime writes `scope_began` for it), sum `turnUsage` over the turns
whose `Scope` the key contains, per model and across models. For every
session, sum over its turns. Tens of scopes by hundreds of turns; no
index. The browser adds nothing: it prints `all` or a `by_model` line.

### Files

`internal/observation/files.go`. Six names: `run.json`, `scopes.json`,
`sessions.json`, `turns.json`, `turn_usage.json`, `model_calls.json`.
Each is a JSON array of that table's rows, `run.json` a one-element
array. Rows are emitted in a fixed order so the same state always writes
the same bytes: scopes by key, sessions by created then id, turns by
started then id, turn usage by turn then model, model calls in fold
order. `turn_usage.json` and `model_calls.json` are flattened from the
per-turn maps with the `run`, `turn`, and `model` columns filled in.

`flush()`:

```go
func (s *Store) flush() error {
	s.mu.Lock()
	if s.replaying || s.flushing || s.dirty == 0 || s.dir == "" {
		s.mu.Unlock()
		return nil
	}
	s.flushing = true
	var first error
	for s.dirty != 0 {
		pending := s.encodeDirtyLocked() // table name → bytes, and clears dirty
		s.mu.Unlock()
		for name, raw := range pending {
			if err := writeAtomic(filepath.Join(s.dir, name), raw); err != nil && first == nil {
				first = err
			}
		}
		s.mu.Lock()
	}
	s.flushing = false
	s.mu.Unlock()
	return first
}
```

`writeAtomic` is today's write-temp-then-rename. A write error is
returned to the caller that triggered the flush; `Lifecycle` and `Event`
report it the way `Event` reports a malformed event today, so it lands
in the run's recording verdict through `recordFailure`. `Close` sets
`closed`, finishes subscribers, then waits for `flushing` to clear
(a `sync.Cond` on `mu`, or a short loop; implementer's choice) and calls
`flush` once more, so after `Close` returns every table is on disk and
nothing is left dirty. No `observation.json`.

### Opening a finished run

`internal/observation/replay.go`:

```go
// open replays a run directory's logs through a fresh store and writes
// its table files. It is how every finished run is read, whether the
// directory has table files or not: the log is the input, the files are
// the output.
func open(dir string) (*Store, error)
```

It makes a store with `replaying` set, reads `run.jsonl` line by line
(`bufio.Reader.ReadBytes('\n')`, as `runlog` does; no following, an
unfinished log is served as far as it goes) and calls `Lifecycle` on each
line; then walks `sessions/` in sorted path order, and for every
`*.jsonl` decodes each line's `scope`, `session`, `turn`, `event`, and
`native_ref` and calls `Event`. Session ids nest with scope depth, so the
files are `sessions/<scope path>/<name.N>.jsonl` at any depth
(`sessions/agy.1/agy.1.jsonl`, `sessions/lap.1/task.2/coder.1.jsonl`),
not only two levels down. A missing `run.jsonl` is `ErrNoRun`; a line
that does not decode is an error naming the file and line. Then it
clears `replaying`, marks all six tables dirty, flushes once, and marks
the store closed. `internal/runlog` is not used and not changed.

`Registry` keeps `runs` (live) and gains `finished map[string]*Store`.
`Snapshot(id)`: a live store's snapshot, else a finished store's, else
`open(runDir(id))` cached into `finished`. `finish(id)` moves a store
from `runs` to `finished` instead of deleting it, so a run that ended in
this process is never replayed. `Live(id)` is unchanged. Nothing is
evicted; a project has tens of runs (see Risks).

### Frames and the page

`RunSnapshot`:

```go
type Transcript struct {
	Snapshot   sessionstate.Snapshot      `json:"snapshot"`
	Provenance map[string]json.RawMessage `json:"provenance"`
}

type RunSnapshot struct {
	Run         RunRow                      `json:"run"`
	Scopes      map[string]ScopeRow         `json:"scopes"`
	Sessions    map[string]SessionRow       `json:"sessions"`
	Turns       map[string]TurnRow          `json:"turns"`
	TurnUsage   map[string]map[string]Usage `json:"turn_usage"`  // turn → model
	ModelCalls  map[string][]ModelCallRow   `json:"model_calls"` // turn → calls
	Totals      Totals                      `json:"totals"`
	Transcripts map[string]Transcript       `json:"transcripts"` // turn
}
```

Frames, in the order the store reduced them, one snapshot first on every
connection as today:

| name | data | browser |
| --- | --- | --- |
| `snapshot` | `RunSnapshot` | replace everything |
| `row` | `{"table": "run"\|"scope"\|"session"\|"turn"\|"turn_usage"\|"model_call", "key": "...", "row": <the snapshot's value at that key>}` | `run = row` or `state[table][key] = row` |
| `totals` | `Totals` | `totals = data` |
| `event` | unchanged: `{scope, session, turn, event, nativeRef}` | apply to the turn's projection; provenance |

For `turn_usage` the row is the turn's whole model map; for `model_call`
the turn's whole call list. Both are small per turn and set semantics
keep the browser to one assignment.

`web/src/lib/observation/index.ts` after the change: the row types and
`RunSnapshot` mirrored by hand (as today; nothing is generated for this
package once the remote query is gone, and the `Usage` type moves here
from the generated `lib/skgo/observation/types.ts`, which disappears);
`RunObservation` with `replace`, `apply` for the four frames,
`state(turn)`, `messageRevision`, `snapshot()`, and the connection
generation guard as today; `usageOf(messageRow)` reading the message's
nested `tokens`/`cost` into the flat `Usage`; `usageText(Usage)`. The
transcript reducer under `web/src/lib/sessionstate/` is untouched.

The page shows what it shows today:

- `+page.svelte`: renders `RunViewer` only; the `scopeUsage` query and
  the header line move out.
- `RunViewer.svelte`: run header (name, status, error, connection) plus
  the run's total line from `totals.scopes[""].all`; the scope list from
  `scopes` rows (name, key, status, error, task, decisions, values, as
  now); one section per turn from `turns` sorted by `started` then id,
  with the session row's name, adapter, model, and the transcript from
  `transcripts`.
- `SessionTimeline.svelte`: the session total from
  `totals.sessions[turn.session].all`.
- `MessageRow.svelte`: unchanged call, flat result.

Deleted: `usage.remote.go`, `usage.remote.ts`, `types.ts` (generated),
`web/observation_live_test.go`, the `Usage` polytype output under
`web/src/lib/skgo/observation/`. `go generate ./...` rewrites
`skgo_remotes_gen.go`, `web/skgo.remotes.json`, and `internal/skgo/links`.
`web/server.go`, `page.server.go`, `hooks.go`, `hooks.ts` are unchanged.

## Implementation Plan

Each phase compiles and passes its tests before the next. Checks, in
order: `just build`, `go test -count=1 ./...`, `go vet ./...`,
`cd web && pnpm test`, `cd web && pnpm run check`.

### Phase 1: Rows, fold, totals, frames (Go)

Files: `internal/observation/rows.go` (new), `snapshot.go`, `store.go`,
`usage.go`, `identity.go`, `subscribe.go` (frame names), `doc.go`,
`store_test.go`, `usage_test.go`; `run.go`; `gimble_test.go`.

1. `rows.go`: the six row structs, `Tokens`, `Usage`, `Usage.add`.
   `snapshot.go`: `Transcript`, `Total`, `Totals`, `RunSnapshot`; delete
   `RunInfo`, `SessionInfo`, `ScopeInfo`, `Invocation`, `Placement` stays.
2. `store.go`: the maps; `Lifecycle(record json.RawMessage)` with the
   private record struct and the table above; `Event` with the step
   folds, the open-call map, and the ended-turn guard; `snapshotLocked`
   copying every map (raw JSON cloned as today) and filling `Totals`;
   `row` and `totals` frames; `FrameLifecycle` deleted, `FrameRow` and
   `FrameTotals` added.
3. `usage.go`: flat `Usage`, `contains`, `totalsLocked`. Delete
   `RunInfo.ScopeUsage`, `Store.ScopeUsage`, `foldUsageLocked`,
   `inScope`.
4. `run.go`: `observeLifecycle` hands the record to the store; delete the
   type switch. `event_persistence.go` already returns the record bytes.
5. Tests. `store_test.go` fixtures become JSON record lines (they already
   half are). Keep the subscription-boundary tests with `row`/`event`
   frames. New: a session created at `""` with `turn_started` in
   `lap.1/task.2` and two step ends is charged to `lap.1/task.2`,
   `lap.1`, and `""` and not to `lap.10`; a `turn_ended` report replaces
   the live per-model map, including an empty report and a report under
   another model name; a `step.failed` with tokens but no cost adds
   nothing; a `step.started` then `step.ended` yields one model call with
   the step's model and both times; a subscriber sees a `totals` frame
   after a step end with the scope's new number; a subscriber joining
   after `turn_started` has the prompt in its first snapshot.
   `usage_test.go` is folded into these. `gimble_test.go`'s
   `TestCancelledRunStaysCancelled` feeds the fixture lines to
   `store.Lifecycle` directly.

At the end of this phase `Close` still writes `observation.json` from
the new snapshot; Phase 2 removes it.

### Phase 2: Files, replay, registry (Go)

Files: `internal/observation/files.go` (new), `replay.go` (new),
`registry.go`, `store.go` (`Close`, dirty set, `flush`), delete
`checkpoint.go`; `internal/observation/testdata/issue-149/` (the saved
run's `run.jsonl` and `sessions/`, without its `observation.json`);
`store_test.go`, `registry_test.go` (new); `gimble_test.go`.

1. `files.go`: names, row ordering, `encodeDirtyLocked`, `writeAtomic`,
   `flush`. Dirty marks in `Lifecycle` and `Event`.
2. `store.go`: `Close` drains and calls `registry.finish`; no
   checkpoint. Delete `checkpoint.go`, `loadCheckpoint`, `checkpointName`.
3. `replay.go`: `open(dir)`. `registry.go`: `finished`, the three-way
   `Snapshot`, `finish` moving instead of deleting.
4. Tests. After `Close` the directory holds the six files and no
   `observation.json`, and `turn_usage.json` decodes to the rows the
   snapshot holds; a store whose flush is in flight absorbs a second
   mutation into the same flush (the file after `Close` has both rows);
   `open` on `testdata/issue-149` yields three scopes, three sessions,
   six turns, six model calls, a transcript per turn, and root totals of
   exactly `claude-haiku-4-5-20251001` 20 / 66341 / 8269 / 130 / 220,
   stated cost 0.0249421; `gpt-5.6-luna` 15210 / 30208 / 0 / 112 / 0;
   `gemini-3.8-flash-low` 30006 / 0 / 0 / 101 / 0 (input, cache read,
   cache write, output, reasoning; summed by `jq` from the six
   `turn_ended` records on 2026-09-13), and writes the six files into
   the copy it opened (use `t.TempDir()`); the registry serves a run that
   finished in-process without replaying (the store pointer is the same)
   and a directory it never saw by replaying once for two `Snapshot`
   calls. `gimble_test.go`: after a fake-adapter run the six files exist,
   no `observation.json`, and every `turn_started` in `run.jsonl` is a
   row in `turns.json` with its scope.

### Phase 3: The page

Files: `web/src/lib/observation/index.ts`, `index.test.ts`,
`RunViewer.svelte`, `SessionTimeline.svelte`, `MessageRow.svelte`;
`web/src/routes/runs/[runID]/+page.svelte`; delete `usage.remote.go`,
`usage.remote.ts`, `types.ts`, `web/observation_live_test.go`,
`web/src/lib/skgo/observation/types.ts`; `web/observation_ssr_test.go`
fixtures; regenerated `skgo_remotes_gen.go`, `web/skgo.remotes.json`,
`internal/skgo/links/**`.

1. `index.ts` as in § Frames and the page. `index.test.ts`: a `row`
   frame sets a scope, a turn, a turn's usage; a `totals` frame replaces
   totals; a replacement snapshot resets everything; a message revision
   survives a following `row` frame; `usageOf` zero-fills and reads the
   nested message shape into the flat one. Delete the lifecycle-fold
   tests and the cancelled-run replay (that fold is Go's now; the Go
   fixture stays).
2. `RunViewer`, `SessionTimeline`, `MessageRow`, `+page.svelte` as above.
   `svelte-autofixer` on each until clean.
3. Delete the remote query and its test; `go generate ./...`; confirm the
   generated `types.ts` is gone (delete it by hand if the generator
   leaves it) and `just build` passes. `observation_ssr_test.go` feeds a
   `session_created` and a `turn_started` record as JSON lines and still
   finds `run-1`, `turn-1`, `hello from Gimble`, `test-model` in the
   document.

### Phase 4: Proof

Files: `ephemeral/attest/run-store/main.go`, `result.md`, two
screenshots.

The program is set up as `ephemeral/attest/issue-149/main.go` is:
`web.NewRuntime` on a loopback port, the project directory at
`ephemeral/attest/run-store/.gimble` (so the binary can serve it later),
scratch workdirs, signal cancellation, the server held open after the
run, the run URL printed. Cheap tier only. The body, inline:

```go
runtime.Run(ctx, "run-store-proof", func(ctx context.Context) error {
	// created at the root, used in a child scope: the placement case
	shared := gimble.NewSession(ctx, "shared", codex.New(), "gpt-5.6-luna", a)
	if _, err := shared.Generate[gimble.Text](ctx, "Define a token budget in one sentence, without tools."); err != nil { return err }
	if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
		_, err := shared.Generate[gimble.Text](ctx, "Name one useful budget measurement, without tools.")
		return err
	}); err != nil { return err }
	// a Group of two; the preamble makes Claude's cache columns nonzero
	g := gimble.Group(ctx, "compare")
	g.Go("attempt", func(ctx context.Context) error {
		s := gimble.NewSession(ctx, "writer", claude.New(), "claude-haiku-4-5-20251001", b)
		if _, err := s.Generate[gimble.Text](ctx, preamble+"Explain this in 100 words without tools."); err != nil { return err }
		_, err := s.Generate[gimble.Text](ctx, preamble+"Give one example in 100 words without tools.")
		return err
	})
	g.Go("attempt", func(ctx context.Context) error {
		s := gimble.NewSession(ctx, "writer", agy.New(), "gemini-3.8-flash-low", c)
		_, err := s.Generate[gimble.Text](ctx, "Explain token caching in 150 words without tools.")
		return err
	})
	return g.Wait()
})
```

1. Run it with the page open. Screenshot while `compare.1` is going:
   the header total and a session card total have risen since the page
   loaded. After it ends: `ls .gimble/runs/<id>/` shows `run.jsonl`,
   `sessions/`, and the six table files, no `observation.json`.
2. By hand, one `jq` call per scope, e.g. for `research.1`:

   ```sh
   jq -n --arg s research.1 \
     '[input[] | select(.scope == $s or (.scope | startswith($s + "/"))) | .id] as $t
      | [input[] | select(.turn as $x | $t | index($x))]
      | group_by(.model) | map({model: .[0].model, input: map(.input) | add,
        cache_read: map(.cache_read) | add, cache_write: map(.cache_write) | add,
        output: map(.output) | add, reasoning: map(.reasoning) | add,
        stated_cost: map(.stated_cost) | add})' turns.json turn_usage.json
   ```

   For `""` (the root) the filter is every turn. Compare the root to the
   page's header line and each session's rows to its card. `research.1`
   must hold `shared.1/turn.2` and not `turn.1`. Write every number and
   the `turn_ended` line it matches into `result.md`.
3. Stop the program. `cd ephemeral/attest/run-store && go run ../../../cmd
   --port 8099`, open `/runs/<id>`: the same header, scopes, cards, and
   transcripts. Screenshot.
4. Copy the saved issue-149 run's `run.jsonl` and `sessions/` (not its
   `observation.json`) to `.gimble/runs/20260912-205306.issue-149/`, open
   it: it renders, the six files appear beside the logs, and the header
   equals the sum in Phase 2's test.
5. `result.md`: configured and resolved model ids, run id, port, the
   numbers from 2 and 4 beside what the page showed.

## Files Summary

| File | Change |
| --- | --- |
| `internal/observation/rows.go` | new: six row structs, `Tokens`, `Usage` |
| `internal/observation/snapshot.go` | `RunSnapshot` over the tables, `Totals`, `Transcript`; old info structs deleted |
| `internal/observation/store.go` | maps; `Lifecycle(record)` decodes the record; step folds; `row`/`totals` frames; `Close` drains |
| `internal/observation/usage.go` | flat usage, `contains`, `totalsLocked`; session-total fold and `ScopeUsage` deleted |
| `internal/observation/files.go` | new: table names, ordering, `flush`, `writeAtomic` |
| `internal/observation/replay.go` | new: `open(dir)` |
| `internal/observation/registry.go` | `finished` map; `Snapshot` live, finished, or replayed |
| `internal/observation/checkpoint.go` | deleted |
| `internal/observation/doc.go`, `identity.go`, `subscribe.go` | wording; frame names; `transcript` |
| `internal/observation/store_test.go`, `registry_test.go` | as in Phases 1 and 2; `usage_test.go` folded in |
| `internal/observation/testdata/issue-149/` | the saved run's logs |
| `run.go` | `observeLifecycle` is one line |
| `gimble_test.go` | fixture replay through `Lifecycle`; files after a run |
| `web/src/lib/observation/index.ts`, `index.test.ts` | rows, four frames, no lifecycle fold, flat `Usage` |
| `web/src/lib/observation/RunViewer.svelte` | total line in the header; turns and transcripts from the tables |
| `web/src/lib/observation/SessionTimeline.svelte` | session total from `totals` |
| `web/src/lib/observation/MessageRow.svelte` | flat `usageOf` |
| `web/src/routes/runs/[runID]/+page.svelte` | renders `RunViewer` only |
| `web/src/routes/runs/[runID]/usage.remote.go`, `usage.remote.ts`, `types.ts` | deleted |
| `web/observation_live_test.go` | deleted |
| `web/observation_ssr_test.go` | JSON record fixtures |
| `web/src/lib/skgo/observation/types.ts`, `skgo_remotes_gen.go`, `web/skgo.remotes.json`, `internal/skgo/links/**` | regenerated |
| `ephemeral/attest/run-store/` | proof program, result, screenshots |

Not changed: `events.go`, `event_persistence.go`, `session.go`,
`scope.go`, `usage.go` (root), `jsonschema/`, the adapters,
`internal/runlog/`, `internal/sessionstate/`, `web/src/lib/sessionstate/`,
`web/server.go`, `page.server.go`, `hooks.go`, `hooks.ts`. No new
exported name in package `gimble`.

## Definition of Done

Gated on the intent's six success criteria per `docs/definition-of-done.md`.

1. **Six files, no checkpoint.** After the proof run, `runs/<id>/` lists
   the six table files and no `observation.json`; the `jq` call in Phase
   4 step 2 answers per-model tokens for `research.1` and the root, and
   the validator's hand sums match the page.
2. **The page unchanged.** Live and after restart, the run header shows
   the root's total, the scope list shows every scope with task,
   decisions, and values, each session card shows its total, and the
   transcripts render. Seen in the two screenshots.
3. **Placement.** `shared.1/turn.2` sits under `research.1` in
   `turns.json` and its tokens count in `research.1`'s `jq` answer; the
   root's answer counts it once. `store_test.go`'s placement case.
4. **From logs alone.** The copied issue-149 directory renders, gets its
   six files, and its root totals equal the literal sums in the Phase 2
   test.
5. **Totals as a frame.** The live screenshot shows a total that rose
   without reload; `store_test.go` sees a `totals` frame after a step
   end. `index.ts` contains no sum over turns.
6. **Checks.** `just build`, `go test -count=1 ./...`, `go vet ./...`,
   `cd web && pnpm test`, `cd web && pnpm run check` pass.

Exit at 90 to 95 percent: a presentation quirk is filed and the branch
merged.

## Risks & Mitigations

- **Rewrite cost per step.** Two files rewritten on every step end,
  O(rows) each. The in-flight flush absorbs bursts; the proof run
  reports how many renames `model_calls.json` saw against how many step
  ends. If a real sprint run shows the disk falling behind, the fallback
  is to mark those two tables dirty on step end but flush them only on
  the next turn boundary: same flusher, one condition.
- **Finished stores held in memory.** The registry never evicts. A
  project with tens of runs holds tens of stores with transcripts; a run
  is opened only when viewed. Eviction is a later issue if it is ever
  needed.
- **Replay of a large run on first view.** The reduction is what the
  live run already did once; the issue-149 run is 157 KB of logs. The
  proof records how long the copied run took to open.
- **A step that lands after `turn_ended`.** Impossible live: the step's
  event is written before the turn's record. On replay the guard makes
  the order irrelevant. Tested.
- **Model key changes at turn end.** Steps name the resolved model; a
  harness that reports under another name moves the turn's usage to that
  key at turn end. Totals are unchanged; accepted.
- **`step.failed` without usage.** No model call row, no usage, by
  decision 1. A turn that failed before any step has no calls and its
  report's usage, which may be empty.
- **Codex resumed turns without steps.** Usage arrives with the report at
  turn end, as today; accepted.
- **Stale generated files.** Deleting the remote query must remove
  `web/src/lib/skgo/observation/types.ts`; Phase 3 checks it and `just
  build` fails if `index.ts` still imports it.
- **Write error on the event path.** A failed rename is returned from
  `Event` and reaches `recordFailure` like a malformed event; the run's
  recording verdict says so. No retry.
- **An unfinished log in a run directory.** Replay serves what is there
  with the run still `running`. Another process's live run is not
  followed; out of scope.

## Dependencies

- Main at or after #156 and #146 (both in `ad6f2f6`). PR #158 (Codex
  cache write) is independent; merge order does not matter.
- The three harness CLIs logged in; the cheap tier:
  `claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`.
- `jq` on the validator's machine.
- `just build` before `go test ./...` in a fresh worktree.
- #173 consumes this snapshot; #172 supplies prices later. Neither is
  built here.

## Open Questions

1. **`invocations` renamed to `transcripts`.** The turn row now carries
   the placement, so the old `Invocation` is a projection plus
   provenance. The rename is recommended and cheap to reverse.
2. **`model_call` frame granularity.** The whole per-turn list is
   republished on each step end. A turn with hundreds of calls would
   make that frame grow; publishing one call keyed by message is the
   alternative. Chosen the list for one assignment in the browser.
3. **`internal/runlog`.** The server no longer reads it; `gimble_test.go`
   still does. Left as is; delete when nothing needs it.
4. **`session_closed`.** No column in `session`. If the tree wants a
   closed time, it is one column and one fold line.
5. **Eviction of finished stores.** None now; see Risks.
