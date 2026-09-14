# Sprint 001: Token usage by scope

Rewritten 2026-09-13 after PR #156 (the OpenCode accounting port, closing
#148 and #149) and PR #146 (scopes in the snapshot) landed on main. The
first draft's adapter phases are done; this is what remains.

## Pyramid Index

- L0: Charge every turn's usage to the scope it ran in, give scopes and
  turns their times, and render the run as one tree of scopes, turns, and
  messages with per-model token and cost cells and wall-clock bars, from
  the same code live and after the run.
- L1:
  - Placement: scope usage sums turns by where they ran, not by where their
    session was created. Today the planner and validator in the sprint
    workflow bill everything to the root and the laps show nothing.
  - Snapshot: `ScopeInfo` gains began and ended; a `turns` map carries each
    turn's prompt, output type, started, ended, duration, error,
    interrupted, and per-model usage, live from its steps and replaced by
    the harness's turn report when it states one.
  - Page: one collapsible tree from the root. Name, bar on the run's wall
    clock, six cells (input, cache read, cache write, output, reasoning,
    cost). Expanding picks the level and shows the per-model table, a
    task's text, a turn's prompt, a link from a message to its transcript
    row. No tabs, no extras.
  - One definition, checked twice: the Go `ScopeUsage` query and the
    browser's tree both sum the snapshot's turns by prefix, and one shared
    fixture holds them to the same answer.
  - Proof: fake-adapter tests for the fold and the sums; one live run on
    the cheap tier with a session created in a parent and used in a child
    scope, a concurrent group, and a scope two deep; a reconcile script
    against the run log; live and finished screenshots; a restart on a
    directory holding only `observation.json`.
- L2:
  - Architecture § "Placement" is the correction; § "The snapshot shape"
    the fold; § "Sums" the one definition; § "The page" the layout.
  - Implementation Plan phases 1 to 4.
  - Definition of Done maps each criterion to what is seen working.

## Overview

Since #156, every model call's usage reaches the assistant row as five
zero-filled fields and a cost; each session keeps a running total
(`session.usage.updated`); the turn-ended record in the run log carries the
turn's usage per model; and a Go live query answers usage for any scope by
summing the running totals of the sessions created in it. Since #146, the
snapshot carries each scope's name, status, error, task, values, and
planner decisions.

Three things are still missing for "where did the tokens go, per scope":

1. **The scope sum is by the wrong key.** `RunInfo.ScopeUsage` sums each
   session under the scope that created it. `API.md` § Keys says a turn's
   events carry the key of the scope where the turn ran, and that is how
   a coder made in the loop's scope shows a turn in each lap. In the sprint
   workflow the planner and validator are created at the root and generate
   inside the loop and its task scopes; the saved issue-125 run shows
   planner turns placed in scopes other than the planner's creation scope.
   Today those turns bill to the root and the laps show nothing for them.
2. **No times.** Scopes have no began or ended and there is no turn record
   in the snapshot, so nothing can be drawn on a clock and a finished run
   cannot say what a turn's prompt was.
3. **No tree.** The page lists scopes, then invocations, flat.

Cost is in: it is real data now, stated by Claude and zero elsewhere. Price
tables stay out. `TurnEnded` and the projection port are unchanged.

## Use Cases

1. **Where did the budget go.** Tyler opens a finished sprint run and sees,
   for the whole run, tokens and cost per model. He expands `sprint.1`
   and sees each `task.N` lap's share, including the planner's decision
   turns and the validator's assessment, then the coder's turns inside one
   lap with their prompts, then the individual model calls of the turn
   that blew up.
2. **Live watch.** A run is going. The tree grows as scopes begin, bars
   extend as frames arrive, cells rise as steps end. No reload.
3. **Compare candidates.** Two `attempt` scopes under a group overlap on
   the axis; the cells say which was cheaper.
4. **Is the cache working.** The per-model line under a scope shows cache
   read against input for the same session across two turns.
5. **A program reads the checkpoint.** `observation.json` alone has
   scopes with times, turns with prompts and usage, and the Go query
   answers any scope from it.

## Architecture

### What is already true

- `run.jsonl` has `scope_began` (name, task), `scope_ended` (error),
  `turn_started` (prompt, output type), `turn_ended` (result, error,
  usage per model with cost, duration, interrupted), each with `time`,
  scope key, session id, turn id.
- `Store.Lifecycle` folds run status, sessions, and (since #146) scope
  name, status, error, task, values, decisions. `Store.Event` folds
  `session.usage.updated` into `RunInfo.Usage[session]`.
- `session.go` sums a turn's step usage under the session's model and
  replaces it with the harness's per-model report at turn end when there is
  one (`ephemeral/research/issue-149/FINDINGS.md` has the per-harness
  rules).
- `Placement.Scope` on an invocation and on every native event is the key
  of the scope the turn ran in.
- The browser reducer mirrors the store, applies frames incrementally, and
  replaces itself on a new snapshot.

### Placement

A turn's usage belongs to the scope it ran in. Session running totals stay
as they are for the session card. Scope usage, in Go and in the browser,
becomes the sum over turns whose placement scope the asked-for scope
contains: equality, or prefix followed by `/`, with the root containing
everything. `attempt.1` does not contain `attempt.10`.

### The snapshot shape

`internal/observation/snapshot.go`:

```go
type ScopeInfo struct {
	Name      string                     `json:"name"`
	Status    string                     `json:"status"`
	Error     string                     `json:"error,omitempty"`
	Task      json.RawMessage            `json:"task,omitempty"`
	Values    map[string]json.RawMessage `json:"values,omitempty"`
	Decisions []json.RawMessage          `json:"decisions,omitempty"`
	Began     int64                      `json:"began"`           // Unix ms, the unit message rows use
	Ended     int64                      `json:"ended,omitempty"` // zero while open
}

// TurnInfo is one turn, from turn_started on. Usage is per model: the
// turn's steps summed under the session's model while it runs, replaced by
// the harness's report at turn end when it stated one, as session.go does.
type TurnInfo struct {
	Scope       string           `json:"scope"`
	Session     string           `json:"session"`
	Prompt      string           `json:"prompt"`
	OutputType  string           `json:"outputType"`
	Started     int64            `json:"started"`
	Ended       int64            `json:"ended,omitempty"`
	Duration    int64            `json:"duration,omitempty"` // ms
	Error       string           `json:"error,omitempty"`
	Interrupted bool             `json:"interrupted,omitempty"`
	Usage       map[string]Usage `json:"usage"`               // by model
}

type RunSnapshot struct {
	Run         RunInfo               `json:"run"`
	Scopes      map[string]ScopeInfo  `json:"scopes"`
	Turns       map[string]TurnInfo   `json:"turns"`
	Invocations map[string]Invocation `json:"invocations"`
}
```

`observation.Lifecycle` gains the record `Time` and one optional entry per
folded variant (`TurnStarted{Prompt, OutputType}`, `TurnEnded{Usage,
Duration, Error, Interrupted}`); scope began and ended use the time the
entry already carries the status change with. `writeLifecycle` returns the
record time beside its bytes and `run.observeLifecycle` forwards it, so
the fold and the log agree to the millisecond. `Store.Event` folds each
`session.step.ended` and `session.step.failed` usage into
`Turns[at.Turn].Usage[session model]`; `TurnEnded` replaces the map with
the report when the report is non-empty. `snapshotLocked` copies the map;
`loadCheckpoint` defaults it. A checkpoint written before this sprint has
no `turns` and no times; the page shows what it has, no shim.

Browser, `web/src/lib/observation/index.ts`: `RunSnapshot` gains `turns`
and the two scope times; `foldLifecycle` gains the cases, parsing the
record's RFC 3339 `time` with `Date.parse` and the duration from
nanoseconds to milliseconds; `apply` folds step usage into the turn the
same way; `replace` refills. Same rules as Go, held to it by the fixture
below.

### Sums

One definition, in words: a turn's usage is its `Usage` map; a scope's is
the sum of the turns it contains, per model; the run is the root. Two
implementations exist because main already has one:

- Go, `internal/observation/usage.go`: `ScopeUsage(scope)` is rewritten
  over `Turns` by placement. The `scopeUsage` live query and its tests are
  unchanged in shape. It serves the page header and any program.
- Browser, `web/src/lib/observation/usage.ts`: `rollup(observation)`
  returns usage by scope key, by turn id, and by message (from the row's
  own `tokens` and `cost`), the tree's data. A per-turn revision, bumped
  on that turn's frames and reset by `replace`, caches turn usage so a text
  delta recomputes one turn.

A shared fixture, a real `run.jsonl` plus session logs from the proof run
under `internal/observation/testdata/`, is replayed by a Go test and a TS
test that assert the same per-scope numbers. Drift fails a test.

### The page

One tree, one level-picking gesture: expand. The scope list and the
invocation list on today's run page are replaced by the tree; the
transcript sections stay below as the drill target.

```text
+------------------------------------------------------------------------------------------------+
| Run usage-proof                     completed        live                                      |
| claude-haiku-4-5   in 1,204  cr 9,876  cw 2,010  out 640  reason 0   $0.0249                  |
| gpt-5.6-luna       in 3,110  cr 0      cw 0      out 220  reason 180 $0                       |
| gemini-3.8-flash   in 14,980 cr 0      cw 0      out 124  reason 0   $0                       |
+------------------------------------------------------------------------------------------------+
| name                      | 0s ---------------- 84s |  input | cache r | cache w | out | reason | cost |
|---------------------------|-------------------------|--------|---------|---------|-----|--------|------|
| [-] usage-proof           | [=====================] | 19,294 |   9,876 |   2,010 | 984 |    180 | .025 |
|   turn 1 (planner)        | [=]                     |        |         |         |     |        |      |
|   [-] research.1          | [=====]                 |        |         |         |     |        |      |
|       turn 2 (planner)    | [==]                    |        |         |         |     |        |      |
|       turn 1 (researcher) | [==]                    |        |         |         |     |        |      |
|   [-] bakeoff.1           |        [========]       |        |         |         |     |        |      |
|     [+] attempt.1         |        [======]         |        |         |         |     |        |      |
|     [+] attempt.2         |        [========]       |        |         |         |     |        |      |
|   [+] review.1            |                 [=====] |        |         |         |     |        |      |
+------------------------------------------------------------------------------------------------+
```

- `RunViewer.svelte` keeps the observation, the SSE connection, and the
  revision counter, and renders `UsageView` where the scope and invocation
  lists are today.
- `UsageView.svelte`: `$derived` rollup on `revision`; axis from root
  `began` to root `ended`, or while running the latest timestamp the
  snapshot holds so every frame extends it; per-model summary lines for
  the run in the header (the existing `scopeUsage` query keeps feeding the
  run line above the viewer).
- `UsageRow.svelte`, recursive over three kinds. Name: scope name with
  ordinal (run name for the root); the turn's session name and turn
  number; the message's model and step ordinal. Bar: the row's interval
  (scope began/ended, turn started/ended, message `time.created` to
  `time.completed`), open intervals to the axis end and marked open. Six
  cells from the summed usage. Expanding a scope shows its child scopes
  and the turns placed directly in it, in began/started order, and a task
  scope's task name and description. Expanding a turn shows its prompt
  (first line, the rest in a `<details>`) and its assistant rows. A
  message row links to its `data-message-id` anchor in the transcript.
  Under any expanded row a `<details>` "by model" gives one line per
  model with the six cells. Expand state is a `Set<string>` keyed by row
  id. Wide content scrolls in its own container.
- Bars say when and how long; cells say how much. Plain Svelte and CSS.
- `svelte-autofixer` on every component before it is called done.

## Implementation Plan

### Phase 1: Placement and the turn record (Go)

Files: `internal/observation/snapshot.go`, `store.go`, `usage.go`,
`usage_test.go`, `checkpoint.go`, `store_test.go`; `event_persistence.go`;
`run.go`; `gimble_test.go`.

1. `TurnInfo`, `Turns` on `RunSnapshot`, `Began`/`Ended` on `ScopeInfo`.
2. `writeLifecycle` returns the record time; `run.event` passes it;
   `Lifecycle` gains `Time` and the turn entries; `observeLifecycle` fills
   them, mapping `[]ModelUsage` to the map.
3. `Store.Lifecycle`: began/ended on scopes; insert the turn at
   `turn_started`; set ended, duration in ms, error, interrupted, and the
   report at `turn_ended`. `Store.Event`: step usage into the turn under
   the session's model. Snapshot copies; checkpoint defaults.
4. `ScopeUsage` over `Turns` by placement, segment-safe `inScope`
   (`attempt.1` vs `attempt.10` test).
5. `store_test.go`: a session created at the root with a turn placed in
   `lap.1/task.2` bills `task.2`, `lap.1`, and the root, and nothing to
   any sibling; a turn's live step sums are replaced by its report; times
   and prompts survive `Close` and `loadCheckpoint`; a late subscriber's
   first snapshot already has them.
6. `gimble_test.go`: after the fake attest run, `observation.json` scope
   keys equal the `scope_began` keys in `run.jsonl`, every `turn_started`
   is in `turns` with its prompt, every ended is at or after its began,
   and every turn's usage equals its `turn_ended` record.

### Phase 2: Mirror and sums (browser)

Files: `web/src/lib/observation/index.ts`, `usage.ts` (new),
`index.test.ts`, `usage.test.ts` (new; add to the explicit `test` list in
`web/package.json`); `internal/observation/testdata/<proof run>/` (from
Phase 4, replayed by both sides).

1. Types, fold cases, step usage into turns, `replace`, per-turn revision.
2. `usage.ts`: `contains`, `rollup` with the turn cache, `total` across
   models.
3. Tests: fold produces the same maps as the Go checkpoint for the proof
   run; parent = direct turns + children on a synthetic three-level tree
   with a turn at each level; `attempt.1` vs `attempt.10`; two models in
   one scope stay separate; a replacement snapshot resets the rollup to
   exactly the new value; the shared fixture gives the same per-scope
   numbers as the Go test.

### Phase 3: The page

Files: `web/src/lib/observation/RunViewer.svelte`, `UsageView.svelte`
(new), `UsageRow.svelte` (new).

1. `UsageView` and `UsageRow` as in § The page.
2. `RunViewer` renders `UsageView` in place of the scope and invocation
   lists; transcript sections stay below.
3. `svelte-autofixer`, then `cd web && pnpm run check && pnpm test`, then
   `just build`.

### Phase 4: Proof

Files: `ephemeral/attest/usage-by-scope/main.go`, `result.md`,
screenshots; `web/scripts/reconcile-usage.ts`;
`internal/observation/testdata/<proof run>/`.

The workflow, inline per `AGENTS.md`, extends the issue-149 program's
shape with the three things it lacked:

```go
runtime.Run(ctx, "usage-proof", func(ctx context.Context) error {
	// created at the root, used in a child scope: the placement case
	planner := gimble.NewSession(ctx, "planner", codex.New(), "gpt-5.6-luna", workdirA)
	if _, err := planner.Generate[gimble.Text](ctx, "In one line, what is a token budget?"); err != nil { return err }
	if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
		if _, err := planner.Generate[gimble.Text](ctx, "Name one thing to research about token budgets."); err != nil { return err }
		s := gimble.NewSession(ctx, "researcher", claude.New(), "claude-haiku-4-5-20251001", workdirB)
		if _, err := s.Generate[gimble.Text](ctx, preamble+"Summarize this in one line."); err != nil { return err }
		_, err := s.Generate[gimble.Text](ctx, preamble+"Name its three most important identifiers.")
		return err
	}); err != nil { return err }
	// concurrent siblings
	g := gimble.Group(ctx, "bakeoff")
	for i, dir := range []string{workdirC, workdirD} {
		g.Go("attempt", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "candidate", codex.New(), "gpt-5.6-luna", dir)
			_, err := s.Generate[gimble.Text](ctx, fmt.Sprintf("Write haiku %d about token budgets.", i+1))
			return err
		})
	}
	if err := g.Wait(); err != nil { return err }
	// two deep
	return gimble.Scope(ctx, "review", func(ctx context.Context) error {
		return gimble.Scope(ctx, "verdict", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "judge", agy.New(), "gemini-3.8-flash-low", workdirE)
			_, err := s.Generate[gimble.Text](ctx, "Pick the better of these two haiku: ...")
			return err
		})
	})
})
```

1. Run it with the page on. Capture the tree live with the two `attempt`
   bars overlapping and cells rising; after it ends, reload and capture
   the finished view. The planner's second turn must appear under
   `research.1`, not the root.
2. `bun web/scripts/reconcile-usage.ts <run dir>` reads `observation.json`,
   `run.jsonl`, and `sessions/*.jsonl` and exits non-zero unless every
   turn's usage equals its `turn_ended` record, every scope equals its
   direct turns plus its children, the root equals the Go `scopeUsage`
   answer for `""`, and per-model sums equal the step events grouped by
   the session's model or the harness's report. Paste the output into
   `result.md`.
3. Copy only `observation.json` into a fresh project directory, start the
   server on it, open the run: same tree, same numbers.
4. Copy the run's logs into `internal/observation/testdata/` as the shared
   fixture for Phases 1 and 2.
5. `result.md`: models and resolved ids, run id, every number the
   screenshots show beside the script's output.

Checks, in order: `just build`, `go test -count=1 ./...`, `go vet ./...`,
`cd web && pnpm test`, `cd web && pnpm run check`.

## Files Summary

| File | Change |
| --- | --- |
| `internal/observation/snapshot.go` | `TurnInfo`, `Turns`; `Began`, `Ended` on `ScopeInfo` |
| `internal/observation/store.go` | record time and turn entries on `Lifecycle`; fold; step usage into turns; copies |
| `internal/observation/usage.go`, `usage_test.go` | `ScopeUsage` over turns by placement; segment-safe `inScope` |
| `internal/observation/checkpoint.go` | default `Turns` |
| `internal/observation/store_test.go` | placement, report replacement, times, late subscriber |
| `internal/observation/testdata/<proof run>/` | shared fixture |
| `event_persistence.go` | `writeLifecycle` returns the record time |
| `run.go` | `observeLifecycle` fills turn entries and times |
| `gimble_test.go` | checkpoint agrees with `run.jsonl` |
| `web/src/lib/observation/index.ts` | `turns`, times, fold cases, step usage, per-turn revision |
| `web/src/lib/observation/usage.ts`, `usage.test.ts` | new: `contains`, `rollup`, `total`; tests |
| `web/src/lib/observation/index.test.ts` | fold cases, shared fixture |
| `web/src/lib/observation/RunViewer.svelte` | tree replaces the scope and invocation lists |
| `web/src/lib/observation/UsageView.svelte`, `UsageRow.svelte` | new |
| `web/package.json` | test list includes `usage.test.ts` |
| `web/scripts/reconcile-usage.ts` | new: proof script |
| `ephemeral/attest/usage-by-scope/` | workflow, result, screenshots |

Not changed: adapters, `events.go`, `jsonschema/`, `usage.go` (root),
`session.go`, `internal/sessionstate/`, `web/src/lib/sessionstate/`,
`usage.remote.go`, `hooks.go`, `hooks.ts`, `page.server.go`.

## Definition of Done

Gated on the intent's success criteria per `docs/definition-of-done.md`.

1. **Placement.** On the proof run's page, the planner's second turn sits
   under `research.1` and its usage counts there, not at the root; the
   Go `scopeUsage` for `research.1` agrees. Seen in the screenshot and in
   the reconcile output.
2. **Totals at every level.** Root and every scope row show six cells and
   a by-model table; children plus direct turns equal the parent at every
   node. Reconcile script (b) and the three-level unit test.
3. **Drill.** Expanding `bakeoff.1` shows two `attempt` rows, then a turn
   with its prompt, then its model-call rows; expanding a task scope
   shows its task. Seen by eye on the proof run.
4. **Timeline.** The two `attempt` bars overlap; `research.1` and
   `review.1` do not; the root spans the run; open bars marked open in the
   live screenshot.
5. **Checkpoint alone.** A server on a directory holding only
   `observation.json` renders the same tree and numbers.
6. **One definition.** The shared fixture gives the same per-scope numbers
   in the Go test and the TS test.
7. **Tests and build.** `go test ./...`, `cd web && pnpm test`, `pnpm run
   check` with zero errors, `just build`.

Exit at 90 to 95 percent: a presentation quirk is filed and the branch
merged.

## Risks & Mitigations

- **Live step usage keyed by the session's model when the harness's report
  names another model.** The report replaces the map at turn end, as
  `session.go` does; between steps and report the by-model line can move.
  Accepted; the totals do not change.
- **Two sum implementations drift.** The shared fixture is a real run and
  both tests replay it.
- **Rollup cost per frame.** Per-turn revision cache; measured on the
  proof run.
- **Reconnect double counting.** The rollup is derived from current turns
  and dropped whole on a replacement snapshot; tested.
- **Root named ".", `attempt.1` containing `attempt.10`.** Named in the
  fold and the `contains` test.
- **Old checkpoints.** No `turns`, no times: empty rows, no shim.
- **Codex resumed turns.** #156 passes `experimentalRawEvents` on resume;
  if a resumed turn still shows no steps, its usage arrives with the
  report at turn end. Acceptable.

## Dependencies

- Main at or after #156 and #146.
- The three harness CLIs logged in; the cheap tier:
  `claude-haiku-4-5-20251001`, `gpt-5.6-luna`, `gemini-3.8-flash-low`.
- `just build` before `go test ./...` in a fresh worktree.
- Open follow-ups that stay separate: #152 (cost on message rows), #155
  (reducer session info seeding), #151 (Codex cache write placement).

## Open Questions

1. **Delete the Go query instead?** The page will not need it once the
   tree exists. Kept here because a program reading a run wants one
   answer without a browser; delete it if that reader never appears.
2. **Session grouping.** A second axis, usage by session regardless of
   scope, is one `<details>` away and not in this sprint.
3. **Extras declined for now:** segmented bars, a duration column, tokens
   per second, a ticking clock.
