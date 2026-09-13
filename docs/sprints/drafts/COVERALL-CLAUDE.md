# Token usage by scope, with a timeline

Replaces #157, #155, #152, #151, #147, #13.

## Intent

Every turn's usage is charged to the scope it ran in, and every scope shows
the sum of what ran inside it, per model, cached and new, with cost. The run
page draws one tree of scopes, turns, and model calls on the run's wall
clock, from the same snapshot live and after the run. Nothing new is
computed by the adapters; the store records what the run log already says.

## What is true today

- A turn's records carry the scope it ran in, not the scope that created its
  session: `session.go:126-127` (turn_started), `session.go:137` (native
  events), `session.go:196,204,216,222` (turn_ended) all use `scope.key`
  from the calling ctx; `run.go:141-143` puts it in `Placement.Scope`.
  `session.go:45` records session_created under the creating scope. Seen in
  `ephemeral/attest/issue125/run.jsonl:6,8`: `planner.1` created at `""`,
  its turn placed in `without-plan.1`.
- `turn_ended` already carries the turn's final usage per model, its
  duration in ns, error, and interrupted (`events.go:115-121`). The runtime
  builds it at `session.go:181-185`: the harness's report when the adapter
  returned one (Claude, `claude/claude.go:160-169`), otherwise the turn's
  step usage under the session's configured model (Codex and Antigravity
  return none). Step usage is `session.step.ended` always, and
  `session.step.failed` only when it carries both cost and tokens
  (`session.go:229-252`).
- `LifecycleRecord.Time` is in the record JSON the store already receives
  (`event_persistence.go:42-49`, `store.go:32`). Native events carry
  `created` in Unix ms (`session.go:335`). Message rows in the projection
  carry `time.created` and `time.completed` (`internal/sessionstate/reduce.go:539,548`).
- The store keeps one `invocation` per turn, keyed by turn id, created on
  the turn's first native event (`store.go:224-234`), and publishes it in
  the snapshot as `Invocation{Scope, Session, Turn, Snapshot, Provenance}`
  (`snapshot.go:60-65`). The browser mirrors it in `invocations`
  (`web/src/lib/observation/index.ts:54-57,70-74`).
- `RunInfo.Usage[session]` is each session's running total, folded from
  `session.usage.updated` on both sides (`internal/observation/usage.go:52-63`,
  `index.ts:79`), read by the session card (`SessionTimeline.svelte:20-23`).
  The session total is step tokens plus report cost (`session.go:143-146,168-179`).
- `RunInfo.ScopeUsage` sums those session totals by the session's creating
  scope (`usage.go:68-76`). It is served by the `scopeUsage` live query
  (`web/src/routes/runs/[runID]/usage.remote.go`), consumed only by the run
  header line (`+page.svelte:9-12`), and tested by `usage_test.go` and
  `web/observation_live_test.go`, both of which feed only
  `session.usage.updated`.
- The snapshot has `Scopes` since #146 (`snapshot.go:71-77`) with no times.
  The page lists scopes, then invocations, flat (`RunViewer.svelte:55-79`).
- The ported reducers never create a session's `info` record, so their
  `session.usage.updated` case is a no-op (`internal/sessionstate/reduce.go:84-87`,
  `web/src/lib/sessionstate/index.ts:48`). Nothing reads reducer info.
- Codex `normalizeUsage` subtracts cache read from input and not cache write
  (`codex/events.go:565-577`); `codex/events_test.go:51` expects 75 from
  100 input, 25 cached, 5 cache write.
- The saved run `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/`
  has three sessions in three scopes, two turns each, Claude reporting cost
  under `claude-haiku-4-5-20251001` while the session was created as
  `haiku` (`run.jsonl:6,12`).

## What each absorbed issue becomes

- **#155.** Closed by decision. Gimble has no session record to seed the
  ported reducers with and nothing reads their `info`. Session totals live
  in `RunInfo.Usage`, turn totals in the invocation, scope totals are sums
  of invocations. The ports are not edited.
- **#152.** Closed by decision. A model-call row shows the cost its step
  stated (zero today for every harness). Cost is real at the turn and
  above, from the harness's turn report. No proportional attribution.
- **#151.** Done in this issue: Codex input becomes
  `max(0, inputTokens − cachedInputTokens − cacheWriteInputTokens)`. The
  existing test's expectation moves from 75 to 70 and the comment above
  `normalizeUsage` is corrected. No live effect on today's models.
- **#147.** Done in this issue: the tree shows a task scope's task once,
  from `ScopeInfo.Task`, and the scope's values omit the key `task` when
  the scope has a task. The runtime still records the value, because
  `ScopeText` and the planner's previous-task record read it (`loop.go:124,128`).
- **#13.** Closed as superseded by #156. The quota-window half has no
  consumer and is not built.

## The change

In order. Each step compiles and its tests pass before the next.

### 1. Codex cache write (`codex/events.go`, `codex/events_test.go`)

One subtraction, one expected number, one comment.

### 2. Times and the turn record in the store (Go)

`internal/observation/snapshot.go`:

```go
type ScopeInfo struct {
	Name      string                     `json:"name"`
	Status    string                     `json:"status"`
	Error     string                     `json:"error,omitempty"`
	Task      json.RawMessage            `json:"task,omitempty"`
	Values    map[string]json.RawMessage `json:"values,omitempty"`
	Decisions []json.RawMessage          `json:"decisions,omitempty"`
	Began     int64                      `json:"began,omitempty"` // Unix ms, the unit message rows use
	Ended     int64                      `json:"ended,omitempty"`
}

// Invocation is one turn: its placement, what was asked, when it ran, what
// it spent per model, its transcript projection, and native provenance.
type Invocation struct {
	Scope       string                     `json:"scope"`
	Session     string                     `json:"session"`
	Turn        string                     `json:"turn"`
	Prompt      string                     `json:"prompt,omitempty"`
	OutputType  string                     `json:"outputType,omitempty"`
	Started     int64                      `json:"started,omitempty"`
	Ended       int64                      `json:"ended,omitempty"`
	Duration    int64                      `json:"duration,omitempty"` // ms
	Error       string                     `json:"error,omitempty"`
	Interrupted bool                       `json:"interrupted,omitempty"`
	Usage       map[string]Usage           `json:"usage"` // by model
	Snapshot    sessionstate.Snapshot      `json:"snapshot"`
	Provenance  map[string]json.RawMessage `json:"provenance"`
}
```

No new type and no new map: the invocation already is the turn record,
keyed by turn id, with the placement the tree needs. `RunSnapshot` is
unchanged in shape.

`internal/observation/store.go`:

- `Lifecycle` gains `Time time.Time` and `Turn *TurnChange` beside
  `Scope *ScopeChange`, where `TurnChange{Prompt, OutputType string; Ended
  bool; Usage map[string]Usage; Duration int64; Error string; Interrupted
  bool}`. `run.go:observeLifecycle` fills them from `TurnStarted` and
  `TurnEnded` (mapping `[]ModelUsage` to the map, ns to ms) and decodes
  `Time` from the record it already has, so the fold and the log agree.
- Scope fold: `scope_began` sets `Began`, `scope_ended` sets `Ended`.
- Turn fold: `turn_started` calls `invocationLocked(at)` (this now happens
  before the first native event) and sets prompt, output type, started.
  `turn_ended` sets ended, duration, error, interrupted, and **replaces**
  `Usage` with the record's map, including an empty one. The record is the
  runtime's final answer (`session.go:181-185`); the store does not decide
  again.
- Event fold: `session.step.ended`, and `session.step.failed` with both
  cost and tokens, add to `inv.Usage[s.run.Sessions[at.Session].Model]`.
  This is the live figure for a turn that has not ended and the same rule
  `session.go:143-146` uses. When the report arrives it may rename the key
  (Claude: `haiku` becomes `claude-haiku-4-5-20251001`) and add cost; that
  is the replacement, and totals at every level are allowed to change at
  turn end.
- `snapshotLocked` copies the new fields and the usage map;
  `loadCheckpoint` defaults `Usage` on each invocation. A checkpoint from
  before this change has no times and no prompts; those rows draw no bar.

### 3. Delete the Go scope query

`RunInfo.ScopeUsage`, `Store.ScopeUsage`, `inScope`, `usage_test.go`,
`web/src/routes/runs/[runID]/usage.remote.go`, `ScopeRef`, the generated
`usage.remote.ts` and `types.ts` entries, `web/observation_live_test.go`,
and the header line in `+page.svelte`. Its only consumer is that line, and
the tree computes the same number in the browser. `RunInfo.Usage` and
`foldUsageLocked` stay: they are the session totals. If deleting the remote
removes the generated `web/src/lib/skgo/observation/types.ts`, the three
`Usage` types move into `web/src/lib/observation/index.ts`.

### 4. Mirror and sums (browser, `web/src/lib/observation/index.ts`)

- `ScopeInfo` gains `began?`, `ended?`. `Invocation` gains the same fields
  as Go, `usage: Record<string, Usage>`.
- `foldLifecycle`: `scope_began`/`scope_ended` set the times from
  `Date.parse(record.time)`; `turn_started` creates the invocation if
  absent (with an empty projection, as `apply` does today) and sets prompt,
  output type, started; `turn_ended` sets ended, duration (`event.duration`
  ns to ms), error, interrupted, and replaces `usage` from the record's
  `usage` array keyed by `model`. `foldLifecycle` therefore takes the
  invocations map as well as `run` and `scopes`.
- `apply` for an event: `session.step.ended`, and `session.step.failed`
  with both cost and tokens, add to `invocation.usage[run.sessions[session].model]`.
- `replace` copies the fields.
- One exported function, `usageByScope(observation)`, returns for every
  scope key the per-model sum of the invocations whose placement scope the
  key contains: equal, or key plus `/` is a prefix; the root `""` contains
  everything. Computed on demand from the current invocations; no cache.
  The three-level and `attempt.1` versus `attempt.10` cases are its test.

### 5. The page (`web/src/lib/observation/`)

`RunViewer.svelte` keeps the observation, the SSE connection, the revision,
and the run header, and renders one tree in place of the scope list and the
invocation list. Two new components, `UsageTree.svelte` (the header row and
the axis) and `UsageRow.svelte` (recursive).

Rows, three kinds:

- **Scope.** Name from `ScopeInfo.Name` (`.` for the root, shown as the run
  name). Bar from `began` to `ended`. Cells from `usageByScope`. Expanded:
  the task's name and description if the scope has a task, error if any,
  decisions, values (omitting `task` when a task exists), then child scopes
  and the invocations placed directly in it, in began or started order.
- **Turn.** Name is the session's name and the turn ordinal from the turn
  id. Bar from `started` to `ended`. Cells from `invocation.usage` summed
  across models. Expanded: the prompt (first line, rest in `<details>`),
  then the turn's `SessionTimeline`, which is today's transcript with its
  message rows and their own usage lines. The drill ends in the transcript
  itself, so there are no anchors and no second copy of the transcript.
- **By model.** Under every expanded scope or turn row, a `<details>` with
  one line per model and the six cells.

Six cells: input, cache read, cache write, output, reasoning, cost. The axis
runs from the root's `began` to its `ended`, or to `Date.now()` read inside
the `$derived` on `revision` while the run is going, so every frame extends
it; an open interval draws to the axis end and is marked open. Expand state
is a `SvelteSet<string>` keyed by row kind and full key. Wide content
scrolls in its own container. `svelte-autofixer` on both components and on
`RunViewer.svelte`.

Deleted: the flat scope sections and the flat invocation sections in
`RunViewer.svelte`.

### 6. Checks

`just build`, `go test -count=1 ./...`, `go vet ./...`, `cd web && pnpm
test`, `cd web && pnpm run check`.

## What the validator must observe on the proof run

The proof program is `ephemeral/attest/usage-by-scope/main.go`, set up
exactly as `ephemeral/attest/issue-149/main.go:38-62` (fresh `logs/`,
`web.NewRuntime` on a loopback port, server held open). Its workflow:

```go
runtime.Run(ctx, "usage-proof", func(ctx context.Context) error {
	// created at the root, used in child scopes: the placement case
	planner := gimble.NewSession(ctx, "planner", codex.New(), "gpt-5.6-luna", ws)
	if _, err := planner.Generate[gimble.Text](ctx, "In one line, what is a token budget? No tools."); err != nil { return err }
	if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
		if _, err := planner.Generate[gimble.Text](ctx, "In one line, name one thing to research about token budgets. No tools."); err != nil { return err }
		r := gimble.NewSession(ctx, "researcher", claude.New(), "claude-haiku-4-5-20251001", ws)
		if _, err := r.Generate[gimble.Text](ctx, "In one line, what is a prompt cache? No tools."); err != nil { return err }
		_, err := r.Generate[gimble.Text](ctx, "In one line, when does it help? No tools.")
		return err
	}); err != nil { return err }
	// concurrent siblings
	g := gimble.Group(ctx, "bakeoff")
	for i := range 2 {
		g.Go("attempt", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "candidate", codex.New(), "gpt-5.6-luna", ws)
			_, err := s.Generate[gimble.Text](ctx, fmt.Sprintf("Write haiku %d about token budgets. No tools.", i+1))
			return err
		})
	}
	if err := g.Wait(); err != nil { return err }
	// two deep
	if err := gimble.Scope(ctx, "review", func(ctx context.Context) error {
		return gimble.Scope(ctx, "verdict", func(ctx context.Context) error {
			j := gimble.NewSession(ctx, "judge", agy.New(), "gemini-3.8-flash-low", ws)
			_, err := j.Generate[gimble.Text](ctx, "In one line, what makes a haiku good? No tools.")
			return err
		})
	}); err != nil { return err }
	// one real Loop dispatch, so a task scope with a task exists
	loop := gimble.Loop(ctx, "sprint", "Write one haiku about token budgets into haiku.txt", planner)
	for ctx, task := range loop.Tasks {
		w := gimble.NewSession(ctx, "worker", claude.New(), "claude-haiku-4-5-20251001", ws)
		if _, err := w.Generate[gimble.Text](ctx, task.Description+"\n\n"+gimble.ScopeText(ctx)); err != nil { return err }
		break
	}
	return loop.Err()
})
```

Run it with the page open. The validator, working from the run directory
and the page only, records these in `ephemeral/attest/usage-by-scope/result.md`:

1. **Placement.** In `run.jsonl`, the `turn_started` for `planner.1/turn.2`
   has `"scope":"research.1"` and the one for `planner.1/turn.1` has
   `"scope":""`. On the page, expanding `research.1` shows the planner's
   turn 2 as a row; the root's direct rows show turn 1 and not turn 2.
2. **Turn equals its record.** For every `turn_ended` in `run.jsonl`, the
   turn row's by-model lines equal that record's `usage` array, model for
   model, six numbers each. Write the table.
3. **Scope equals its parts.** For `research.1`, `bakeoff.1`, each
   `attempt.N`, `review.1`, `review.1/verdict.1`, `sprint.1`,
   `sprint.1/task.1`, and the root: the row's cells equal the sum of the
   rows directly under it. Do the addition by hand from the table in 2 and
   write it down.
4. **Root equals the run.** The root row's per-model lines equal the sum of
   every `turn_ended` usage in `run.jsonl` grouped by model. Claude's cost
   is nonzero; Codex and Antigravity cost is zero.
5. **Timeline.** Screenshot of the finished page: the two `attempt` bars
   overlap each other; `research.1`, `bakeoff.1`, `review.1`, `sprint.1`
   do not overlap each other; the root spans them all. Compare each bar's
   ends against the `scope_began`/`scope_ended` times in `run.jsonl`.
6. **Live.** Screenshot taken while `bakeoff.1` is running: its bar and
   both `attempt` bars are open, and at least one cell is nonzero before
   the turn ends (from a step event).
7. **Drill.** Expanding `sprint.1/task.1` shows the task name and
   description once, then the worker's turn; expanding the turn shows its
   prompt and the transcript with its message rows.
8. **Checkpoint alone.** Copy only `observation.json` into
   `<fresh project>/runs/<run id>/`, start the server on `<fresh project>`,
   open the run: the same tree and the same numbers as 2 through 5.
9. **Session card.** Each session's total line still matches its last
   `session.usage.updated` in `sessions/<session>/<session>.jsonl`.

Models used, resolved ids, run id, and the port go at the top of the file.
No script is written for this; the numbers are read and added by hand.

## Unit tests (gates, not proof)

- `internal/observation/store_test.go`: a session created at `""` with a
  turn placed in `lap.1/task.2` is charged there; live step sums are
  replaced by the turn report, including an empty report and a report that
  renames the model key; scope times and turn fields survive `Close` and
  `loadCheckpoint`; a subscriber joining after `turn_started` sees the
  prompt in its first snapshot.
- `codex/events_test.go`: 70, not 75.
- `web/src/lib/observation/index.test.ts`: the same folds on the browser
  side; `usageByScope` on a three-level tree with a turn at each level;
  `attempt.1` versus `attempt.10`; two models in one scope stay separate; a
  replacement snapshot yields exactly the new value.
- The saved issue-149 run's `run.jsonl` and session logs, copied under
  `internal/observation/testdata/issue-149/`, replayed by one Go test and
  one TS test asserting the same literal per-scope numbers. Test data, not
  a harness.

## Open decisions for Tyler

1. **Name.** The fold needs one new struct in `internal/observation`,
   `TurnChange`, beside `ScopeChange`. Grant it, or say to put its fields
   flat on `Lifecycle`.
2. **The Go scope query.** This proposal deletes it. Keeping it means
   moving `ScopeUsage` from `RunInfo` to `RunSnapshot` (it needs the
   invocations), rewriting its two tests to feed turns instead of session
   totals, and leaving the header line. Say if a program outside the page
   will need it.
