# Sprint: The run store

Unnumbered, by Tyler's instruction. Planned 2026-09-13 with the DF
sprint-plan skill. The intent's six decisions are fixed and are restated here only where the
code needs them. This sprint is what #169 becomes; #173 (the tree) and
#172 (prices) read what it builds and are not built here.

## Pyramid Index

- L0: Replace `observation.json` with the designed store: seven tables of
  facts as Go maps in `internal/observation`, six JSON array files under
  `runs/<id>/` rewritten atomically as rows change, totals computed in Go
  and pushed as a frame, one fold for the live run and for opening a
  finished one, and the current run page showing what it shows today from
  that store.
- L1:
  - The fold moves into the store: `Store.Lifecycle` decodes the exact
    record JSON; `run.go` stops translating variants. `turn_started` and
    `turn_ended` become rows; a turn is charged to the scope it ran in.
  - Usage: `turn_usage` accumulates from steps and is replaced wholesale
    by the report at `turn_ended`; one guard (no step usage on an ended
    turn) makes replay order-free. `model_call` rows are facts, never
    summed. Totals per scope (prefix containment) and per session, all
    models and by model, are recomputed in Go when `turn_usage` changes.
  - Files: six arrays, initialized empty at `Open`, rewritten under the
    store lock once per accepted record, nothing on text deltas; a write
    error reaches `recordFailure`.
  - Opening a finished run loads the six files into the maps; the
    session logs are read only for transcripts; the logs are replayed
    through the fold, and the six files written, only when one of the
    six is missing. The opened store is cached in the registry.
    `checkpoint.go` and `loadCheckpoint` go.
  - Frames: `snapshot`, `row`, `totals`, `event`. The browser folds
    nothing but native transcript events; `foldLifecycle` and the usage
    fold are deleted; the `scopeUsage` remote query is deleted.
  - Proof: gates plus one cheap-tier live run with a root-created session
    used in a child scope and a `Group` of two, seen live and after a
    restart, with `jq` over `turns.json` and `turn_usage.json` against
    the page, and the saved issue-149 run opened from a copy of its logs.
- L2: § Architecture (rows, fold, sum rule, files, opening, frames and
  page); § Implementation Plan (three phases); § Definition of Done;
  § Open Questions.

## Overview

Today `internal/observation` keeps run info, sessions with running usage
totals, scopes, and one transcript projection per turn, and writes all of
it as `observation.json` at `Close`. A finished run is that file. Turns
have no record of their own. Scope usage is summed by the session's
creating scope, which is the wrong key (#157).

After this sprint the store is the seven tables the intent names, as Go
maps behind `Store.mu`, and six files beside the logs. `Store.Lifecycle`
and `Store.Event` fold the live run and rebuild a finished one whose
files are missing; a finished run with its files is loaded. Per-scope
and per-session totals are computed in Go from `turn_usage` and published
whenever it changes. The page renders the same header, scope list, session
cards, and transcripts it renders today, from `turns`, `sessions`,
`scopes`, `totals`, and `transcripts`.

## Use Cases

1. **jq over a run.** After a run, `turns.json` and `turn_usage.json`
   answer "tokens per model for scope X" in one `jq` call. No server.
2. **The page today, from the store.** Live: header with the run's total,
   scope list, session cards with session totals, transcripts, updating
   from frames. Finished: the same, from the six files loaded once.
3. **A run finished before this sprint.** The saved issue-149 directory
   has logs, no table files, and an old `observation.json`. Opening a
   copy of it takes the rebuild path: the logs are replayed, the six
   files written, and the page renders; the old file is ignored. Opening
   the copy again loads the files and replays nothing.
4. **#173 reads the maps.** Scope times, turn rows with prompt, output
   type, result, error, interrupted, times, duration, per-model usage per
   turn, model calls, per-scope totals: all in `RunSnapshot` after this
   sprint. #173 adds no store code.

## Architecture

### Rows and maps

`internal/observation/rows.go`. One struct per table; the same struct is
the file row, the map value, and the snapshot value on the wire. Times
are Unix ms (`int64`); JSON names are the intent's column names.

```go
// Tokens is the five counts every harness reports, flat. A count the
// provider did not report is 0.
type Tokens struct {
	Input      float64 `json:"input"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Output     float64 `json:"output"`
	Reasoning  float64 `json:"reasoning"`
}

// Usage is the tokens and the cost the harness stated, 0 when it stated none.
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

type Decision struct {
	Seq  int64           `json:"seq"`  // the lifecycle record's seq
	Body json.RawMessage `json:"body"` // the planner_decision event object
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
	Decisions []Decision                 `json:"decisions"`
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
	Message string `json:"message"` // the normalized assistant message id
	Model   string `json:"model"`
	Tokens
	Started int64 `json:"started"`
	Ended   int64 `json:"ended"`
}
```

The store's maps, all under `Store.mu`:

```go
run         RunRow
scopes      map[string]*ScopeRow          // key
sessions    map[string]*SessionRow        // id
turns       map[string]*TurnRow           // id
turnUsage   map[string]map[string]Usage   // turn -> model
modelCalls  map[string][]ModelCallRow     // turn -> calls in step order
transcripts map[string]*transcript        // turn -> projection + provenance (not a table)
calls       map[string]openCall           // turn + "\x00" + message -> model, started
```

`transcript` is today's `invocation` without the placement triple.
`Invocation`, `RunInfo`, `SessionInfo`, `ScopeInfo`, `Lifecycle`,
`ScopeChange`, `ValueChange` are deleted. `Placement` stays.
`internal/sessionstate/` is not touched.

### The fold

`Store.Lifecycle(record json.RawMessage) error` replaces today's entry.
The store decodes the record into one private struct holding every field
any kind carries (`seq`, `time`, `scope`, `session`, `turn`, `event.kind`,
`name`, `adapter`, `model`, `parent`, `error`, `task`, `key`, `value`,
`prompt`, `output_type`, `result`, `interrupted`, `duration`, `usage`) and
switches on `kind` (snake case: `planner_decision`, `turn_ended`).
`run.observeLifecycle` becomes: hand the record to the store, pass the
returned error to `recordFailure`, as `observeAgent` does today. The type
switch in `run.go` is deleted. The record's `time` (RFC 3339) becomes the
row's ms clock; `duration` (ns in the log) becomes ms.

| kind | effect |
| --- | --- |
| `run_started` | `run.Name`, `Status` running, `Started` |
| `run_ended` | if still running: completed, or failed with the error; `Ended` |
| `run_cancelled` | cancelled with the error, the terminal fact; a later `run_ended` does not overwrite it (as today); `Ended` |
| `scope_began` | `scopes[scope]`: `Key`, `Name`, running, `Task`, `Began` |
| `scope_ended` | ended, `Error`, `Ended` |
| `value_set` | `scopes[scope].Values[key]` = the value as raw JSON (the record carries JSON source text, as today) |
| `planner_decision` | append `{seq, body}` |
| `session_created` | `sessions[session]`: name, adapter, model, `Scope` = record scope, parent, `Created` |
| `turn_started` | `turns[turn]`: session, `Scope` = record scope, prompt, output type, `Started` |
| `turn_ended` | result, error, interrupted, `Ended`, `Duration`; `turnUsage[turn]` **replaced** by the report keyed by model, an empty report giving an empty map |
| anything else | nothing (`session_closed`, `steer`, `interrupt`, `supervise_attached`, `complete`) |

After the switch, still under the lock: write the changed tables (§ Files),
publish one `row` frame per changed table, and if `turnUsage` changed
recompute and publish `totals`.

`Store.Event(at, envelope, nativeRef) error` keeps its signature. As today
it decodes the event, applies it to the turn's projection (creating the
transcript on first sight) and folds provenance. New:

- `session.step.started`: `calls[turn\0message] = {model: data.model.id,
  started: created}`; the session's configured model when the event names
  none. All three adapters put the resolved model id here.
- `session.step.ended`, and `session.step.failed` only when `data.cost`
  and `data.tokens` are both present (the rule `session.go:stepUsage`
  already applies): take the open call (model and started; the session's
  model and 0 when none was seen), append a `ModelCallRow` with the five
  counts and `Ended` = `created`, delete the open call. **If the turn has
  not ended** (`turns[turn].Ended == 0`), add the counts and cost to
  `turnUsage[turn][model]`. Write `model_calls.json` and, if changed,
  `turn_usage.json`; publish `row` frames for both and then `totals`.
- `session.usage.updated`: nothing. The running total is not stored.
- A turn the store has not seen `turn_started` for is created from the
  placement (id, session, scope) so a test can feed native events alone.

The ended-turn guard is what makes replay order-free: replay feeds
`run.jsonl` in full, so every finished turn already holds its report, and
the session logs then add model calls and transcripts without touching a
finished turn's usage. Live it is a no-op: `session.go` writes every step
event through `run.sessionEvent` before it emits `turn_ended`.

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

For every scope key in `scopes` (the runtime writes `scope_began` for the
root), sum `turnUsage` over the turns whose `Scope` the key contains, per
model and across models. For every session, sum over its turns
(`turn.session`). Tens of scopes by hundreds of turns; no index. The
browser adds nothing.

### Files

`internal/observation/files.go`. Six names: `run.json`, `scopes.json`,
`sessions.json`, `turns.json`, `turn_usage.json`, `model_calls.json`. Each
is a JSON array of that table's rows, `run.json` a one-element array. Rows
are emitted in a fixed order so the same state always writes the same
bytes: scopes by key, sessions by created then id, turns by started then
id, turn usage by turn then model, model calls by turn then step order.
`turn_usage.json` and `model_calls.json` are flattened from the per-turn
maps with `run`, `turn`, and `model` filled in.

`Open` writes all six as `[]` when it has a directory, so a run with no
steps still has `model_calls.json`. A fold writes each table it changed,
under the store lock, before it publishes: encode, `os.CreateTemp` in the
same directory, write, close, rename (the operation `checkpoint.go` has
today, moved here). Nothing is written for a text delta or any other
event that changes only a projection. No timer, no goroutine, no dirty
set. A write error is returned from `Lifecycle` or `Event` and reaches
`recordFailure`, so the run's recording verdict says so. `Close` sets
`closed` and finishes subscribers; it writes nothing. No `observation.json`.

### Opening a finished run

The six files are the data model. Gimble reads them; the log is the
transcript store and the rebuild source. `internal/observation/replay.go`:

```go
// open reads a finished run directory into a fresh store. The six table
// files are loaded into the maps; the session logs are read for the
// transcript projections only. The lifecycle log is replayed through the
// fold, and the six files written, only when one of the six is missing.
func open(registry *Registry, id, dir string) (*Store, error)
```

It makes a store with writes suppressed. If all six files exist, each is
decoded (one `json.Unmarshal` per file) into the row slices and indexed
into `run`, `scopes`, `sessions`, `turns`, `turnUsage`, `modelCalls`;
`Lifecycle` is not called and `run.jsonl` is not read. Totals come from
the loaded `turn_usage` rows as they do live. Otherwise (a run recorded
before this sprint, or a directory with a file missing) it reads
`run.jsonl` line by line (`bufio.Scanner`, one `json.Unmarshal` per line,
no following: an unfinished log is served as far as it goes with
`run.status` as the last record left it), calls `Lifecycle` on each line,
and after the session logs are read writes all six files. A missing
`run.jsonl` on the rebuild path is `ErrNoRun`.

On both paths, for every id in `sessions` in created order, it reads
`sessions/<id>.jsonl` (ids carry their scope path, so the file nests as
`sessions/agy.1/agy.1.jsonl` or `sessions/lap.1/task.2/coder.1.jsonl`; a
session that never wrote a log has no file, which is not an error),
decodes each line's `scope`, `session`, `turn`, `event`, and
`native_ref`, and calls `Event`. Over loaded facts `Event` adds
transcripts and nothing else: the ended-turn guard leaves `turn_usage`
untouched, and a model call is replaced in place by message, so the row
is the one already loaded. A line that does not decode is an error naming
the file and line. The store is marked closed. `internal/runlog` is not
used and not changed.

`Registry` keeps one map. `finish` no longer deletes a store when its run
ends; `Live(id)` returns a store only while it is open; `Snapshot(id)`
returns the snapshot of any store in the map, open or closed, else calls
`open` under the registry lock (so two requests open once) and keeps the
result. A run that finished in this process is never reopened. Nothing is
evicted; a project has tens of runs.

### Frames and the page

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
	TurnUsage   map[string]map[string]Usage `json:"turn_usage"`  // turn -> model
	ModelCalls  map[string][]ModelCallRow   `json:"model_calls"` // turn -> calls
	Totals      Totals                      `json:"totals"`
	Transcripts map[string]Transcript       `json:"transcripts"` // turn
}
```

Frames, in the order the store reduced them, one `snapshot` first on
every connection as today; the subscription cut, bounds, and terminal
drain in `subscribe.go` are unchanged:

| name | data | browser |
| --- | --- | --- |
| `snapshot` | `RunSnapshot` | replace everything |
| `row` | `{"table": "run"\|"scopes"\|"sessions"\|"turns"\|"turn_usage"\|"model_calls", "key": "...", "row": <the snapshot's value at that key>}` | `run = row`, or `state[table][key] = row` |
| `totals` | `Totals` | `totals = data` |
| `event` | unchanged: `{scope, session, turn, event, nativeRef}` | apply to the turn's projection; provenance |

Table names equal the snapshot's keys. For `turn_usage` the row is the
turn's whole model map; for `model_calls` the turn's whole call list. The
`lifecycle` frame is gone: `FrameLifecycle` is deleted, `FrameRow` and
`FrameTotals` added.

`web/src/lib/observation/index.ts` after the change: the row types and
`RunSnapshot` mirrored by hand, including a flat `Usage` declared here
(the generated `lib/skgo/observation/types.ts` goes away with the remote);
`RunObservation` with `replace`, `apply` for the four frames,
`state(turn)`, `messageRevision`, `snapshot()`, and the connection
generation guard as today; `usageOf(messageRow)` reading a message's
nested `tokens`/`cost` into the flat `Usage`; `usageText`. Deleted:
`foldLifecycle`, `LifecycleRecord`, `scopeAt`, `parseValue`, the
`session.usage.updated` line. The transcript reducer under
`web/src/lib/sessionstate/` is untouched.

The page shows what it shows today:

- `+page.svelte`: renders `RunViewer` only; the `scopeUsage` query and its
  header line move into `RunViewer`.
- `RunViewer.svelte`: registers `snapshot`, `row`, `totals`, `event` on
  the EventSource; run header (name, status, error, connection) plus the
  run's total line from `totals.scopes[""].all`; the scope list from
  `scopes` rows (name, key, status, error, task, decisions read as
  `decision.body.task`, values); one section per turn from `turns` sorted
  by `started` then id, with `sessions[turn.session]`'s name, adapter,
  model, and the transcript from `transcripts`.
- `SessionTimeline.svelte`: the session total from
  `totals.sessions[turn.session].all`, keyed by placement session as the
  card is today.
- `MessageRow.svelte`: unchanged call, flat `usageOf` result.

Deleted by hand (the skgo generator prunes only its link tree):
`web/src/routes/runs/[runID]/usage.remote.go`, `usage.remote.ts`,
`[runID]/types.ts`, `web/src/lib/skgo/observation/types.ts`. Then
`go generate ./...` rewrites `skgo_remotes_gen.go`, `web/skgo.remotes.json`,
and `internal/skgo/links`. `web/server.go`, `page.server.go`, `hooks.go`,
`hooks.ts` are unchanged.

## Implementation Plan

Phases follow compilation. Checks, in order: `just build`,
`go test -count=1 ./...`, `go vet ./...`, `cd web && pnpm test`,
`cd web && pnpm run check`. Phase 1's gate is the Go part of that list
(the browser still imports the deleted remote until Phase 2); Phase 2's
gate is the whole list.

### Phase 1: The store (Go)

Files: `internal/observation/rows.go` (new), `files.go` (new),
`replay.go` (new), `snapshot.go`, `store.go`, `usage.go`, `registry.go`,
`subscribe.go` (frame names), `identity.go`, `doc.go`; delete
`checkpoint.go`; `store_test.go`, `registry_test.go` (new), delete
`usage_test.go` (folded into `store_test.go`);
`internal/observation/testdata/issue-149/` (the saved run's `run.jsonl`
and `sessions/`, not its `observation.json`); `run.go`; `gimble_test.go`;
`web/src/routes/runs/[runID]/usage.remote.go` and the three generated
files (deleted), `skgo_remotes_gen.go`, `web/skgo.remotes.json`,
`internal/skgo/links/**` (regenerated); `web/observation_live_test.go`,
`web/observation_ssr_test.go`.

1. `rows.go`, `snapshot.go`: the structs above; delete the old info
   structs. `usage.go`: flat `Usage`, `contains`, `totalsLocked`; delete
   `RunInfo.ScopeUsage`, `Store.ScopeUsage`, `foldUsageLocked`, `inScope`.
2. `store.go`: the maps; `Lifecycle(record) error` with the private record
   struct and the table above; `Event` with the step folds, the open-call
   map, and the ended-turn guard; `snapshotLocked` copying every map (raw
   JSON cloned as today) and filling `Totals`; `row` and `totals` frames;
   `Close` writes nothing.
3. `files.go`: names, row ordering, `writeAtomic` (moved from
   `checkpoint.go`), the six empty arrays at `Open`, the per-fold writes.
   `replay.go`: `open`. `registry.go`: keep after close, `Live` only while
   open, `Snapshot` three ways, open under the registry lock. Delete
   `checkpoint.go`, `loadCheckpoint`, `checkpointName`.
4. `run.go`: `observeLifecycle` hands the record to the store and records
   its error; delete the type switch. `event_persistence.go` already
   returns the record bytes.
5. Web Go: delete `usage.remote.go` and the three generated files;
   `go generate ./...`. Retarget `web/observation_live_test.go` at
   `GET /api/runs/{id}/events` through `NewHandler`: it receives the
   opening `snapshot`, then a `totals` frame after a step end, and the
   stream closes on `Close`. `web/observation_ssr_test.go` feeds a
   `session_created` and a `turn_started` record as JSON lines and still
   finds `run-1`, `turn-1`, `hello from Gimble`, `test-model` in the
   document.
6. Tests. `store_test.go` fixtures become JSON record lines. Keep the
   subscription-boundary tests with `row`/`event` frames. New: a session
   created at `""` with `turn_started` in `lap.1/task.2` and two step ends
   is charged to `lap.1/task.2`, `lap.1`, and `""` and not to `lap.10`; a
   `turn_ended` report replaces the live per-model map, including an empty
   report and a report under another model name; a `step.failed` with
   tokens but no cost adds nothing; a `step.started` then `step.ended`
   yields one model call with the step's model and both times; a
   subscriber sees a `totals` frame after a step end with the scope's new
   number; a subscriber joining after `turn_started` has the prompt in its
   first snapshot; after `Close` the directory holds the six files and no
   `observation.json`, and `turn_usage.json` decodes to the rows the
   snapshot holds; a store opened with a directory has six `[]` files
   before any record; a failed write is returned. `registry_test.go`:
   `open` on a `t.TempDir()` copy of `testdata/issue-149` yields four
   scopes (the root and three), three sessions, six turns, six model
   calls, a transcript per turn, and root totals of exactly
   `claude-haiku-4-5-20251001` 20 / 66341 / 8269 / 130 / 220 with stated
   cost 0.0249421; `gpt-5.6-luna` 15210 / 30208 / 0 / 112 / 0;
   `gemini-3.8-flash-low` 30006 / 0 / 0 / 101 / 0 (input, cache read,
   cache write, output, reasoning; summed by hand from the six
   `turn_ended` records), and writes the six files into the copy; opening
   the copy again writes nothing (mtimes unchanged) and reads the files,
   not the log: with one number altered in the copy's `turn_usage.json`,
   a fresh registry's snapshot shows the altered number, the file is
   untouched, and the transcripts still render; with the copy's
   `turn_usage.json` deleted, opening rebuilds all six from the logs; the
   registry serves a run that finished in-process without reopening (same
   store pointer) and a directory it never saw by opening once for two
   `Snapshot` calls. `gimble_test.go`: `TestCancelledRunStaysCancelled` feeds the
   fixture lines to `store.Lifecycle`; after a fake-adapter run the six
   files exist, no `observation.json`, and every `turn_started` in
   `run.jsonl` is a row in `turns.json` with its scope.

### Phase 2: The page

Files: `web/src/lib/observation/index.ts`, `index.test.ts`,
`RunViewer.svelte`, `SessionTimeline.svelte`, `MessageRow.svelte`;
`web/src/routes/runs/[runID]/+page.svelte`.

1. `index.ts` as in § Frames and the page. `index.test.ts`: a `row` frame
   sets a scope, a turn, a turn's usage; a `totals` frame replaces
   totals; a replacement snapshot resets everything; a message revision
   survives a following `row` frame; `usageOf` zero-fills and reads the
   nested message shape into the flat one. Delete the lifecycle-fold tests
   and the cancelled-run replay (that fold is Go's now; the Go fixture
   stays).
2. `RunViewer`, `SessionTimeline`, `MessageRow`, `+page.svelte` as above.
   `svelte-autofixer` on each until clean.
3. The whole check list passes.

### Phase 3: Proof

Proof is running the real thing and saying what you saw, in the PR
description. No proof program, nothing under `ephemeral/`, nothing from
the run committed. What the live run must exercise, on the cheap tier:
one session made at the root and used inside `research`, and two
concurrent attempts in `compare`.

1. Run it with the page open. While `compare.1` is going, record the
   header total and a session card total, then record them again after
   the run ends: they rose without a reload. After it ends,
   `ls .gimble/runs/<id>/` shows `run.jsonl`, `sessions/`, and the six
   table files, no `observation.json`.
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

   For `""` the filter is every turn. Compare the root to the page's
   header line and each session's rows to its card. `research.1` must
   hold `shared.1/turn.2` and not `turn.1`.
3. Stop the run. Serve the same project directory again with the
   binary, open `/runs/<id>`: the same header, scopes, cards, and
   transcripts, with the same numbers.
4. Copy `internal/observation/testdata/issue-149/`'s `run.jsonl` and
   `sessions/` to `.gimble/runs/20260912-205306.issue-149/`, open it: it
   renders, the six files appear beside the logs, and the header equals
   the sums in the Phase 1 registry test.
5. The PR description says the configured and resolved model ids, the run
   id, and the numbers from 1 to 4 beside what the page showed.

## Files Summary

| File | Change |
| --- | --- |
| `internal/observation/rows.go` | new: six row structs, `Decision`, `Tokens`, `Usage` |
| `internal/observation/snapshot.go` | `RunSnapshot` over the tables, `Totals`, `Transcript`; old info structs deleted |
| `internal/observation/store.go` | maps; `Lifecycle(record) error` decodes the record; step folds; per-fold writes; `row`/`totals` frames; `Close` writes nothing |
| `internal/observation/usage.go` | flat usage, `contains`, `totalsLocked`; session-total fold and `ScopeUsage` deleted |
| `internal/observation/files.go` | new: table names, ordering, `writeAtomic`, empty files at `Open` |
| `internal/observation/replay.go` | new: `open` |
| `internal/observation/registry.go` | stores kept after close; `Snapshot` live, finished, or opened once |
| `internal/observation/checkpoint.go` | deleted |
| `internal/observation/doc.go`, `identity.go`, `subscribe.go` | wording; frame names; `transcript` |
| `internal/observation/store_test.go`, `registry_test.go` | as in Phase 1; `usage_test.go` folded in |
| `internal/observation/testdata/issue-149/` | the saved run's logs |
| `run.go` | `observeLifecycle` hands the record over and records the error |
| `gimble_test.go` | fixture replay through `Lifecycle`; files after a run |
| `web/src/routes/runs/[runID]/usage.remote.go`, `usage.remote.ts`, `types.ts`; `web/src/lib/skgo/observation/types.ts` | deleted |
| `skgo_remotes_gen.go`, `web/skgo.remotes.json`, `internal/skgo/links/**` | regenerated |
| `web/observation_live_test.go` | retargeted at the SSE endpoint |
| `web/observation_ssr_test.go` | JSON record fixtures |
| `web/src/lib/observation/index.ts`, `index.test.ts` | rows, four frames, no lifecycle fold, flat `Usage` |
| `web/src/lib/observation/RunViewer.svelte` | frame names; total line in the header; turns and transcripts from the tables |
| `web/src/lib/observation/SessionTimeline.svelte` | session total from `totals` |
| `web/src/lib/observation/MessageRow.svelte` | flat `usageOf` |
| `web/src/routes/runs/[runID]/+page.svelte` | renders `RunViewer` only |

Not changed: `events.go`, `event_persistence.go`, `session.go`,
`scope.go`, `usage.go` (root), `jsonschema/`, the adapters,
`internal/runlog/`, `internal/sessionstate/`, `web/src/lib/sessionstate/`,
`web/server.go`, `page.server.go`, `hooks.go`, `hooks.ts`. No new
exported name in package `gimble`.

## Definition of Done

Gated on the intent's six success criteria per `docs/definition-of-done.md`.
Exit at 90 to 95 percent: a presentation quirk is filed as an issue and the
branch merged.

1. **Six files, no checkpoint.** After the proof run, `runs/<id>/` lists
   the six table files and no `observation.json`, and the files were
   written while the run was live. The `jq` call in Phase 3 step 2
   answers per-model tokens for `research.1` and the root, and the
   validator's hand sums match the page.
2. **The page unchanged.** Live and after restart, the run header shows
   the root's total, the scope list shows every scope with task,
   decisions, and values, each session card shows its total, and the
   transcripts render. Seen in the browser; numbers recorded before and
   after.
3. **Placement.** `shared.1/turn.2` sits under `research.1` in
   `turns.json` and its tokens count in `research.1`'s `jq` answer; the
   root's answer counts it once. `store_test.go`'s placement case.
4. **From logs alone.** The copied issue-149 directory renders, gets its
   six files, and its root totals equal the literal sums in the Phase 1
   registry test.
5. **Totals as a frame.** The live totals rose without a reload;
   `store_test.go` sees a `totals` frame after a step end. `index.ts`
   contains no sum over turns and no lifecycle fold.
6. **Checks.** `just build`, `go test -count=1 ./...`, `go vet ./...`,
   `cd web && pnpm test`, `cd web && pnpm run check` pass. No new root
   export, no adapter, log, or reducer change, no tree, prices, database,
   message table, `observation.json` reader, reconcile script, or fixture
   harness.

### Execution

An Opus subagent implements the phases in order and commits as each
phase's gate passes. `agent -provider openai -model gpt-5.6-sol` runs
`/adversarial-review` on the branch after Phase 2 and after the proof;
findings are fixed and re-reviewed. The orchestrator watches both and
rejects any finding that widens or deepens scope or asks for proof
machinery. Unit tests gate a phase; the proof is the live run.

## Risks & Mitigations

- **Rewrite cost per step.** Two files rewritten on every step end,
  O(rows) each, under the lock. Files are tens of kilobytes at this
  scale; a rename is sub-millisecond; steps are seconds apart. The proof
  run is the measurement. No fallback path is planned.
- **A step that lands after `turn_ended`.** Impossible live: the step's
  event is written before the turn's record. On replay the guard makes
  the order irrelevant. Tested.
- **Model key changes at turn end.** Steps name the resolved model; a
  harness that reports under another name moves the turn's usage to that
  key at turn end. Totals are unchanged; accepted.
- **`step.failed` without usage.** No model call row, no usage. A turn
  that failed before any step has no calls and its report's usage, which
  may be empty.
- **Codex resumed turns without steps.** Usage arrives with the report at
  turn end, as today; accepted.
- **Finished stores held in memory.** The registry never evicts; a run is
  opened only when viewed; tens of runs per project. Eviction is a later
  issue if ever needed.
- **Two requests opening the same run.** Serialized under the registry
  lock; `os.CreateTemp` names never collide anyway.
- **An unfinished run directory from a crashed process.** The six files
  hold what was written up to the crash, so the load path serves them
  with the run still `running`. Another process's live run is not
  followed; out of scope.
- **Stale generated files.** The three deleted by hand are listed; `just
  build` fails if `index.ts` still imports the generated `Usage`.
- **Write error on the event path.** Returned from `Lifecycle` or `Event`
  and recorded through `recordFailure`; the run's recording verdict says
  so. No retry.

## Dependencies

- Main at `ad6f2f6` or later (#156 and #146 are in it). The sprint branch
  is cut from `claude/token-usage-coverall-12e8d5`, which is behind main;
  merge main into the build branch before Phase 1.
- The three harness CLIs logged in; the cheap tier:
  `claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`.
- `jq` on the validator's machine.
- `just build` before `go test ./...` in a fresh worktree.
- #173 consumes this snapshot; #172 supplies prices later. Neither is
  built here. #169 is closed by this sprint's merge.

## Open Questions

1. **Resolved: Gimble reads its own table files.** The plan as first
   filed replayed the whole log on every open and wrote the files only as
   output, an answer taken on Tyler's behalf. Tyler rejected it: the
   tables are the data model, and a store Gimble never reads is not one;
   replaying also re-derives old runs' numbers through whatever the fold
   is today. The three lanes had conflated reading the session logs for
   transcripts (which stays) with re-deriving facts from the lifecycle
   log (which does not). Opening now loads the six files; replay is the
   rebuild path for a directory with a file missing.
2. **`invocations` renamed to `transcripts`.** The turn row carries the
   placement, so the old `Invocation` is a projection plus provenance.
   Cheap to reverse.
3. **`model_calls` frame granularity.** The whole per-turn list is
   republished on each step end. Publishing one call keyed by message is
   the alternative if a turn with hundreds of calls ever appears.
4. **`internal/runlog`.** The server no longer reads it; `gimble_test.go`
   still does. Left as is.
5. **`session_closed`.** No column in `session`. If the tree wants a
   closed time, it is one column and one fold line.
