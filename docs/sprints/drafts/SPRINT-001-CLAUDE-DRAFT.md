# Sprint 001: Token usage by scope

## Pyramid Index

- L0: Fold scopes and turns into the run observation, make the three
  adapters report the same cached/new split with per-field availability,
  and give the run page one usage view: a scope tree with per-model token
  totals on a time axis, drillable to a turn and to a single message, from
  the same code live and after the run.
- L1:
  - Observation: `RunSnapshot` gains `scopes` (key, name, parent, task,
    began, ended, error) and `turns` (prompt, output type, started, ended,
    duration, error, interrupted), folded in Go from the lifecycle records
    the run already writes and mirrored by the browser fold. The checkpoint
    carries them, so a finished run renders from `observation.json` alone.
  - Rollup: one TypeScript module defines usage as an additive value, per
    model, per field, each field a (tokens, unknown-count) pair. Scope usage
    is the sum over turns whose scope key has the scope as a prefix. Children
    sum to parents by construction. Go does not compute totals.
  - Adapters: Antigravity `input` excludes `cache_read_tokens`, the way Codex
    and Claude already exclude cached tokens. `output` availability depends
    on `output_tokens` alone, not on a reasoning split the provider may not
    report. The page reads `fieldAvailability`, never `tokensAvailable`.
  - Page: a Usage tab beside the transcript. Collapsible rows (run, scopes,
    turns, messages), each with a bar on the run's wall clock and five field
    totals; a per-model breakdown under each row. Expanding is how a person
    picks the aggregate level. Same component for live and finished runs.
  - Proof: fake-adapter tests for the fold and the rollup; a live workflow
    on the cheap tier with nested scopes, a concurrent group, and two or
    three harnesses; a screenshot; a script that reconciles the page's root
    total with the run log's `turn_ended` tokens.
  - Out: cost and a price table; changing `TurnEnded.Tokens`; the OpenCode
    projection port.
- L2:
  - Architecture § "Where the rollup lives" answers the Go-versus-browser
    question. § "The usage value" defines the monoid. § "Provider
    accounting" lists each adapter's change and what must be verified live
    first. § "The page" settles the tree-with-time-axis view.
  - Implementation Plan phases 0 through 5 carry files and tasks; Phase 0 is
    the live accounting probe that gates the adapter changes.
  - Definition of Done maps each success criterion in the intent to a check.

## Overview

Every model call already leaves a token record. All three harnesses emit
`session.step.ended` with `tokens {input, output, reasoning, cache {read,
write}}` and `session.step.started` with `model {providerID, id}`; the
projection keeps both on the assistant message row with `time.created` and
`time.completed`. Every message is placed by scope key, session id, and turn
id, and the scope tree is the prefix tree of keys. So the sum for any node in
the graph is already determined by data on disk. Nothing in this sprint
invents identity or a new event.

What is missing is structure and a view. The observation snapshot has no
scopes and no turns, so a finished run cannot say when a scope began or what
a turn's prompt was. The run page is a flat list of turns. And the adapters
disagree on what `input` means and mark whole messages "unavailable" when
one field is missing.

This sprint:

1. Folds `scope_began`, `scope_ended`, `turn_started`, `turn_ended` into
   the observation store, its checkpoint, and the browser's mirror fold.
2. Defines usage once, in the browser, as an additive value with per-field
   availability, and rolls it up the scope tree from message rows.
3. Fixes the two adapter inconsistencies after verifying them against live
   provider output.
4. Adds a Usage view to the run page: tree rows on a time axis, per-model
   totals, drill to turn and message, live and post-run from one code path.
5. Proves it with unit tests over the fake adapter and one live run on the
   cheap tier whose page total reconciles with its run log.

Cost is out. No adapter reports it and a price table is a second source of
truth that nobody has asked for yet. This sprint stops at tokens.

## Use Cases

1. **Where did the budget go.** Tyler opens a finished sprint run and sees,
   for the whole run, tokens per model split into input (new), cache read,
   cache write, output, reasoning. He expands the `loop.1` scope and sees
   each `task.N` lap's share, then the coder's turns inside one lap, then
   the individual model calls of the turn that blew up.
2. **Live watch.** A run is going. The Usage tab shows the scope tree
   growing, bars extending to "now", totals ticking up as steps end. No
   reload; the same rows a finished run will show.
3. **Compare candidates.** A bake-off group has three `attempt` scopes that
   overlap on the time axis. Their bars show they ran concurrently and
   their totals show which candidate was cheapest.
4. **Is the cache working.** A per-model row shows cache read against input
   (new) for a Claude Haiku session across a loop's laps. The split is
   comparable to a Codex session in the same run, because both adapters
   define `input` the same way.
5. **Honest gaps.** An Antigravity session cannot report cache write. Its
   rows show cache write as unavailable with the number of calls that lacked
   it, and every other field still sums. The run-level cache write shows the
   sum of what is known and how many calls are unknown, rather than a zero
   that reads as "no cache writes".
6. **A program reads the checkpoint.** A post-mortem agent given the run
   directory reads `observation.json` and finds scopes with began/ended and
   turns with prompts, without replaying `run.jsonl`.

## Architecture

### What is already true

- `run.jsonl` has every lifecycle record: `scope_began` (name, task),
  `scope_ended` (error), `turn_started` (prompt, output type), `turn_ended`
  (result, error, tokens, duration, interrupted), each with `seq`, `time`,
  scope key, session id, turn id (`events.go`, `event_persistence.go`).
- `run.observeLifecycle` (`run.go`) hands each record to
  `observation.Store.Lifecycle` with the fields the store folds today: run
  status, run name, session info. The record bytes are republished unchanged
  as a `lifecycle` frame.
- `RunSnapshot` is `run` (id, name, status, error, sessions) and
  `invocations` (per turn: scope, session, turn, projection snapshot,
  provenance keyed by normalized message id). The browser's
  `RunObservation` mirrors it and applies frames incrementally.
- Assistant message rows carry `model {providerID, id}`, `tokens`, `cost`,
  `time {created, streamed, completed}`. The provenance sidecar's
  `accounting` carries `fieldAvailability {input, output, reasoning,
  cacheRead, cacheWrite}`, `tokensAvailable`, `rawProviderAccounting`.
  Confirmed in
  `ephemeral/attest/antigravity-run-prompt/logs/runs/20260911-214914.run-prompt/`.
- `Placement.Scope` on an invocation is the key of the scope the turn ran
  in, which may be below the scope that created the session. That is the
  key the rollup uses; a session's creating scope is not.

### Where the rollup lives

The fold of scopes and turns is in Go, in the store, because the checkpoint
must carry them and the store is the one producer. The browser fold mirrors
it exactly, as it already does for sessions, so a live page and a replayed
checkpoint hold the same structure.

The rollup (summing tokens up the tree) lives in the browser only, in one
module, `web/src/lib/observation/usage.ts`. Reasons:

- The browser already holds every message row for both live and finished
  runs; "live and post-run are one code path" is met without the store
  publishing a second kind of frame.
- The store stays a thin producer that never decodes token JSON on the
  event path; it keeps publishing frames it does not interpret.
- A sum computed in Go and shipped in the snapshot would still have to be
  recomputed per frame in the browser, or the store would have to publish
  usage frames. Either is a second definition.
- The checkpoint needs no totals to be sufficient: scopes, turns, rows and
  provenance determine them.

If a Go program later needs totals (a post-mortem agent, the CEO loop), that
is a program reading `observation.json` and summing by prefix, written when
the workflow that needs it exists. This sprint does not add it.

`TurnEnded.Tokens` stays `[]JSONText`. The page does not read it; it reads
message rows, which also carry the model. Retyping the sealed union would
regenerate `jsonschema/LifecycleRecord.json` for no consumer. The proof uses
`turn_ended` tokens only as an independent check that the page's numbers
match the log.

### The snapshot shape

Go, `internal/observation/snapshot.go`:

```go
// ScopeInfo is one scope instance, from the run's scope_began and
// scope_ended records. Times are Unix milliseconds, the unit message rows
// already use. Ended is zero while the scope is open.
type ScopeInfo struct {
	Key    string          `json:"key"`
	Name   string          `json:"name"`
	Parent string          `json:"parent"`         // path.Dir(key), "" for a child of the root, "" for the root itself
	Task   json.RawMessage `json:"task,omitempty"` // the task the scope_began carried, when it did
	Began  int64           `json:"began"`
	Ended  int64           `json:"ended,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// TurnInfo is one turn, from turn_started and turn_ended.
type TurnInfo struct {
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	Session     string `json:"session"`
	Prompt      string `json:"prompt"`
	OutputType  string `json:"outputType"`
	Started     int64  `json:"started"`
	Ended       int64  `json:"ended,omitempty"`
	Duration    int64  `json:"duration,omitempty"` // ms; the record carries nanoseconds
	Error       string `json:"error,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

type RunSnapshot struct {
	Run         RunInfo               `json:"run"`
	Scopes      map[string]ScopeInfo  `json:"scopes"`
	Turns       map[string]TurnInfo   `json:"turns"`
	Invocations map[string]Invocation `json:"invocations"`
}
```

The root scope is `Scopes[""]`: the run writes `scope_began` with scope `""`
and name `"."` before anything else, and `scope_ended` after the body. The
page draws the root row from it.

`observation.Lifecycle` grows the fields the caller already knows from the
variant, in the pattern it uses for sessions: a record time, and one
optional pointer per folded variant (`ScopeBegan`, `ScopeEnded`,
`TurnStarted`, `TurnEnded`). `run.observeLifecycle` fills them in its
existing switch. `writeLifecycle` hands back the record's time beside its
bytes so the fold and the log agree to the millisecond.

Browser, `web/src/lib/observation/index.ts`: `RunSnapshot` gains `scopes`
and `turns` with the same field names; `RunObservation` holds them as maps;
`foldLifecycle` gains the four cases, converting the record's RFC 3339
`time` to milliseconds with `Date.parse`. A checkpoint written before this
sprint lacks `scopes` and `turns`; `loadCheckpoint` and the constructor
default them to empty, and the page shows an empty tree. No shim beyond
that.

### The usage value

`web/src/lib/observation/usage.ts` is the one definition:

```ts
export type Field = 'input' | 'cacheRead' | 'cacheWrite' | 'output' | 'reasoning'
export const FIELDS: Field[]
/** tokens summed over calls that reported the field; unknown counts calls that did not. */
export type Cell = { tokens: number; unknown: number }
export type FieldUsage = Record<Field, Cell>
/** keyed by `${providerID}/${modelID}`, falling back to the session's configured model. */
export type Usage = Record<string, FieldUsage>

export function messageUsage(row: JSONObject, provenance: unknown, fallbackModel: string): Usage | undefined
export function add(into: Usage, from: Usage): Usage
export function empty(): Usage
export function total(usage: Usage): FieldUsage   // across models
```

Rules:

- An assistant row contributes once it has `time.completed`. In flight, it
  contributes nothing (it is not "unknown", it is not done).
- A completed row with `tokens` and a sidecar reads each field's
  availability from `accounting.fieldAvailability`. Available: `tokens += n`.
  Not available: `unknown += 1`. `tokensAvailable` is not consulted.
- A completed row with no sidecar (a fake adapter, or a projection-only
  step) counts every field as unknown. A measured zero is a zero.
- `Usage` is a commutative monoid under `add`, so any grouping sums to the
  same value and a parent equals the sum of its children.

Rollup, in the same module:

```ts
export type Rollup = {
	scopes: Map<string, Usage>   // by scope key, prefix-inclusive; '' is the run
	turns: Map<string, Usage>    // by turn id
	messages: Map<string, Usage> // by `${turn}\0${messageID}`
}
export function rollup(observation: RunObservation): Rollup
```

A turn's usage is the sum of its assistant rows. A scope's usage is the sum
over turns whose `invocation.scope` equals the key or starts with `key +
'/'`; the root key `''` matches every turn. That is the prefix rule from
`API.md` § Keys, applied to placement, and it makes children sum to the
parent without a reconciliation step.

Cost: `RunObservation` gains a per-turn revision, bumped in `apply` on every
event frame for that turn, and `rollup` caches turn usage by that revision.
A frame therefore recomputes one turn and re-sums the tree, which is
`O(turns)` per frame. The scope sum is not cached; a run has tens of scopes
and hundreds of turns, not millions.

### Provider accounting

The target definition, the one Codex already meets: `input` is tokens the
provider charged as fresh input, excluding anything counted as cache read or
cache write; `output` excludes `reasoning`; each of the five fields is
available on its own.

- **Antigravity (`agy/events.go`).** Gemini's usage metadata counts cached
  content inside the prompt token count, and the recorded `total_tokens` is
  `input_tokens + output_tokens` in the real run, so `input_tokens` is the
  full prompt. Change `input` to `max(0, input_tokens - cache_read_tokens)`,
  available when both are present. Cache write stays unavailable: agy does
  not report one and Gemini has no explicit cache write on this path. This
  change is gated on Phase 0 confirming, on a live run with a warm cache,
  that `cache_read_tokens <= input_tokens` and that `total_tokens` stays
  `input_tokens + output_tokens`.
- **Claude (`claude/events.go`).** Today `output` is marked available only
  when `output_tokens_details.thinking_tokens` is also present, and
  Anthropic's usage block does not carry that detail, so every live Claude
  message is likely `tokensAvailable: false` with `output` unavailable.
  Change `output` availability to depend on `output_tokens` alone;
  `reasoning` stays unavailable when the split is absent, and `output` is
  then the total. Confirm in Phase 0 what a live Haiku turn actually
  carries.
- **Codex (`codex/events.go`).** `input` availability requires
  `cacheWriteInputTokens`, which the Responses usage block may not carry
  (OpenAI has no cache write). Change `input` availability to require only
  `input_tokens` and `cached_input_tokens`, subtracting cache write when
  present. Confirm in Phase 0 what a live `gpt-5.6-luna` turn carries.
- All three keep emitting `tokensAvailable` (true only when all five are
  available) and `fieldAvailability`; the page stops reading the former.
  `rawProviderAccounting` stays, so the proof can check normalization
  against the raw numbers.

The OpenCode projection (`internal/sessionstate`, `web/src/lib/sessionstate`)
is not touched. It already keeps `tokens` and `model` on the row.

### The page

One view, one component tree, one level-picking gesture: expand.

- `RunViewer.svelte` gets two tabs: **Transcript** (today's list) and
  **Usage**. The observation, the SSE connection, and the revision counter
  are shared; only the rendered branch changes.
- `UsageView.svelte` computes `rollup(observation)` in a `$derived` keyed
  on `revision`, computes the time axis (start: root `began`; end: root
  `ended`, or a `now` state ticked each second while the run is running),
  and renders `UsageRow.svelte` for the root.
- `UsageRow.svelte` is recursive. A row is one of: scope, turn, message.
  Columns: a disclosure toggle and the name (scope name with ordinal, or the
  turn's session name and turn number, or the message's model and step
  ordinal); a bar positioned on the axis by the row's interval (scope
  began/ended, turn started/ended, message `time.created`/`time.completed`,
  open intervals run to the axis end); five field cells from `total(usage)`;
  a models cell listing the models that contributed. Expanding a scope
  shows its child scopes (by `parent`) and the turns placed directly in it,
  in `began`/`started` order. Expanding a turn shows its prompt (first line,
  the rest under a `<details>`) and its assistant rows. A message row links
  to its `data-message-id` anchor on the Transcript tab.
- A field cell renders `tokens`, and when `unknown > 0` a marker such as
  `12,340 +3?` with a title "3 calls did not report this field"; when
  `tokens === 0 && unknown > 0` it renders `unavailable`. Zero with no
  unknown renders `0`.
- Under each row, a `<details>` "by model" shows the per-model table: one
  line per model key with the five cells. This is where "tokens per model"
  is answered at every level without widening every row.
- Token weight on the timeline: the bar's height or opacity is not used;
  the numbers beside the bar are the weight. A bar shows when and for how
  long; the cells show how much. Keeping the bar a plain interval keeps
  concurrency legible (overlapping sibling bars).
- `MessageRow.svelte`'s footer uses `messageUsage` to print per-field values
  with unavailable fields named, replacing the all-or-nothing `accounting`
  string.

The SSR path needs no change: `page.server.go` serializes whatever
`RunSnapshot` holds, `hooks.ts` parses it back.

## Implementation Plan

### Phase 0: Verify provider accounting on live output

Before touching normalization, capture what each harness actually reports.

Files: none in the tree; outputs under `ephemeral/attest/usage-by-scope/probe/`.

Tasks:

1. Run the first draft of the Phase 5 workflow program with the page off
   (`web.WithNoWeb()`), one session per harness, two turns each on the same
   session, on the cheap tier: Claude `claude-haiku-4-5-20251001`, Codex
   `gpt-5.6-luna`, Antigravity `gemini-3.8-flash-low`. Both turns share a
   long identical preamble so the second turn hits a warm cache. A single
   `gimble run-prompt` cannot do this: it is one turn per run, and a cache
   read needs a second turn.
2. For each run, extract `rawProviderAccounting` and the normalized `tokens`
   from `sessions/*.jsonl` and record them in
   `ephemeral/attest/usage-by-scope/probe/accounting.md` with the model
   used.
3. Decide from the numbers: does Gemini's `input_tokens` include
   `cache_read_tokens` (expected yes: `total == input + output` on the
   cached turn); does Anthropic carry `output_tokens_details`; does Codex
   carry a cache write field. The Phase 3 changes follow the answers; if a
   number contradicts the expectation above, the plan changes there, not
   silently in code.

### Phase 1: Fold scopes and turns into the observation (Go)

Files:
- `internal/observation/snapshot.go`: `ScopeInfo`, `TurnInfo`, the two
  maps on `RunSnapshot`.
- `internal/observation/store.go`: fields on `Lifecycle`; `Lifecycle` folds
  the four variants; `snapshotLocked` copies the maps.
- `internal/observation/checkpoint.go`: `loadCheckpoint` defaults the maps.
- `event_persistence.go`: `writeLifecycle` returns the record time.
- `run.go`: `observeLifecycle` fills the new `Lifecycle` fields for
  `ScopeBegan`, `ScopeEnded`, `TurnStarted`, `TurnEnded`.
- `internal/observation/store_test.go`: new test.
- `gimble_test.go`: extend `TestAttestEventFixture` or add a sibling.

Tasks:

1. Add the types. `Parent` is derived in the store from the key with
   `path.Dir`, mapping `"."` to `""`; the root's parent is `""` as well and
   the page tells it apart by `key === ''`.
2. `Store.Lifecycle`: on `ScopeBegan` insert `Scopes[key]` with name, task,
   began; on `ScopeEnded` set ended and error; on `TurnStarted` insert
   `Turns[turn]` with scope, session, prompt, output type, started; on
   `TurnEnded` set ended, duration, error, interrupted. Times come from the
   record. An `ended` for an unknown key inserts a row with what it has,
   so a snapshot taken from a checkpoint that predates a began is not a
   crash.
3. `snapshotLocked` deep-copies both maps; `Close` therefore persists them.
4. `observeLifecycle`: four new cases in the existing switch. `Task` is
   marshalled from the `polytype.Optional[Task]` only when present.
5. Test in `store_test.go`: feed began/started/ended/ended entries for a
   nested key (`lap.1/bakeoff.1/attempt.2`), assert `Snapshot()` and, after
   `Close`, `loadCheckpoint` carry keys, parents, times, prompt, duration.
6. Test in `gimble_test.go`: after the fake attest run, read
   `observation.json` and assert its scope keys equal the set of scope keys
   in `run.jsonl`'s `scope_began` records, every turn in `turn_started` is
   in `turns` with its prompt, and every `ended` is at or after its `began`.

### Phase 2: Mirror the fold and define usage (browser)

Files:
- `web/src/lib/observation/index.ts`: types, `RunObservation.scopes`,
  `RunObservation.turns`, `foldLifecycle` cases, per-turn revision.
- `web/src/lib/observation/usage.ts`: the module in § The usage value.
- `web/src/lib/observation/index.test.ts`, `usage.test.ts`.
- `web/src/lib/observation/fixtures/antigravity-run-prompt.json`: the real
  checkpoint from `ephemeral/attest/antigravity-run-prompt/...`, copied.
- `web/package.json`: add `usage.test.ts` to the `test` script.

Tasks:

1. Types and fold. `replace` clears and refills the maps from the snapshot;
   `apply` on a lifecycle frame folds into them; the record's `time` is
   parsed to ms. Snapshots without `scopes`/`turns` default to empty.
2. `turnRevision(turn)`: bumped in `apply` for event frames; reset by
   `replace`.
3. `usage.ts` as specified, plus `rollup` with the per-turn cache.
4. Tests: fold produces the same maps a Go snapshot would (fixture with a
   nested key); `add` is associative and commutative on random `Usage`
   values; a row with `fieldAvailability.cacheWrite: false` yields
   `cacheWrite.unknown === 1` and the other four fields summed; an
   in-flight row contributes nothing; on the real Antigravity checkpoint,
   `rollup().scopes.get('')` equals the sum of its three `turn_ended`
   token records with `cacheWrite.unknown === 2` (two steps had usage,
   one tool step had none); parent equals the sum of children on a
   synthetic three-level tree with a turn placed at each level.

### Phase 3: Adapter accounting

Files: `agy/events.go`, `agy/events_test.go`, `claude/events.go`,
`claude/events_test.go`, `codex/events.go`, `codex/events_test.go`.

Tasks, each conditional on Phase 0's finding for that harness:

1. agy: `input = max(0, input_tokens - cache_read_tokens)`; `input`
   available iff both present; leave cache write unavailable. Test with the
   probe's raw numbers.
2. Claude: `fieldAvailability.output = outputOK`; when reasoning is absent,
   `output` is the total. Test with a usage block lacking
   `output_tokens_details`.
3. Codex: `fieldAvailability.input = inputOK && cachedOK`; subtract cache
   write only when present. Test with a usage block lacking
   `cache_write_input_tokens`.
4. Run `go test ./agy ./claude ./codex` and the root package.

### Phase 4: The page

Files:
- `web/src/lib/observation/RunViewer.svelte`: tabs.
- `web/src/lib/observation/UsageView.svelte`: rollup, axis, root row.
- `web/src/lib/observation/UsageRow.svelte`: recursive row.
- `web/src/lib/observation/MessageRow.svelte`: per-field footer.
- `web/src/lib/observation/index.ts`: delete `accounting()`; its test
  moves to `usage.test.ts` as a `messageUsage` test.

Tasks:

1. Tabs on `RunViewer`. The tab is local `$state`, defaulting to Transcript;
   the Usage tab shows totals for the run in its label so the number is
   visible before switching.
2. `UsageView`: `$derived` rollup on `revision`; axis from root scope
   began to root ended or `now`; a 1 s interval only while status is
   `running`, cleared in the effect's teardown.
3. `UsageRow`: the three row kinds, interval bar as a positioned `<span>`
   inside a fixed-width track, five cells, models cell, `<details>` by
   model, children on expand. Expand state lives in a `Set<string>` on the
   view keyed by row id so re-renders keep it.
4. `MessageRow` footer: `input 1,234 · cache read 9,876 · cache write
   unavailable · output 120 · reasoning 0` from `messageUsage`.
5. Run the checks in the `svelte-autofixer` MCP tool on each component
   before finishing, then `cd web && pnpm run check && pnpm test`, then
   `just build`.
6. Extend `web/observation_ssr_test.go`: feed a `scope_began` and a
   `turn_started` lifecycle entry and assert the rendered document carries
   the scope key and the prompt.

### Phase 5: Proof

Files:
- `ephemeral/attest/usage-by-scope/main.go`: the live workflow.
- `ephemeral/attest/usage-by-scope/result.md`: what it showed, which
  models, where the run directory is, the screenshot.
- `web/scripts/reconcile-usage.ts`: reads `observation.json` and
  `run.jsonl` from a run directory, prints the rollup for every scope, and
  exits non-zero if the root total's per-field `tokens` differs from the sum
  of the log's `turn_ended` tokens.

The workflow, written inline in `main.go` per `AGENTS.md`:

```go
web.NewRuntime(ctx, logs, web.WithPort(8080))
runtime.Run(ctx, "usage-proof", func(ctx context.Context) error {
	// research: one Claude Haiku session, two turns, second one cache-warm
	if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
		s := gimble.NewSession(ctx, "researcher", claude.New(), "claude-haiku-4-5-20251001", workdir)
		_, err := s.Generate[gimble.Text](ctx, preamble+"Summarize this file in one line.")
		if err != nil { return err }
		_, err = s.Generate[gimble.Text](ctx, preamble+"Now name its three most important identifiers.")
		return err
	}); err != nil { return err }
	// bakeoff: two concurrent Codex attempts under one group
	g := gimble.Group(ctx, "bakeoff")
	for i := range 2 {
		g.Go("attempt", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "candidate", codex.New(), "gpt-5.6-luna", workdir)
			_, err := s.Generate[gimble.Text](ctx, "Write a haiku about token budgets.")
			return err
		})
	}
	if err := g.Wait(); err != nil { return err }
	// review: one Antigravity turn nested two deep
	return gimble.Scope(ctx, "review", func(ctx context.Context) error {
		return gimble.Scope(ctx, "verdict", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "judge", agy.New(), "gemini-3.8-flash-low", workdir)
			_, err := s.Generate[gimble.Text](ctx, "Pick the better haiku: ...")
			return err
		})
	})
})
```

Tasks:

1. Run it. While it runs, open `http://127.0.0.1:8080/runs/<id>` and
   capture a screenshot of the Usage tab with the bakeoff bars overlapping
   (live). After it ends, reload and capture the finished view.
2. Run `bun web/scripts/reconcile-usage.ts <run dir>` and paste its output
   into `result.md`. The root total must equal the log's sum per field,
   and `research`'s cache read on the second turn must be non-zero.
3. Kill the server, start it again on the same project directory, open the
   run: it renders from `observation.json` alone with the same numbers.
4. Record the models used, the run id, and any field a harness left
   unavailable, in `result.md`.

## Files Summary

| File | Change |
| --- | --- |
| `internal/observation/snapshot.go` | `ScopeInfo`, `TurnInfo`, `Scopes`, `Turns` on `RunSnapshot` |
| `internal/observation/store.go` | `Lifecycle` fields and fold for the four variants; snapshot copies |
| `internal/observation/checkpoint.go` | default the two maps on load |
| `internal/observation/store_test.go` | fold and checkpoint test |
| `event_persistence.go` | `writeLifecycle` returns the record time |
| `run.go` | `observeLifecycle` fills scope and turn entries |
| `gimble_test.go` | checkpoint scope tree equals the log's prefix tree |
| `agy/events.go`, `agy/events_test.go` | `input` excludes cache read |
| `claude/events.go`, `claude/events_test.go` | `output` availability independent of reasoning |
| `codex/events.go`, `codex/events_test.go` | `input` availability independent of cache write |
| `web/src/lib/observation/index.ts` | scopes, turns, fold cases, per-turn revision; remove `accounting()` |
| `web/src/lib/observation/usage.ts` | the usage value, `messageUsage`, `add`, `rollup` |
| `web/src/lib/observation/usage.test.ts` | monoid, availability, reconciliation, real fixture |
| `web/src/lib/observation/index.test.ts` | fold cases |
| `web/src/lib/observation/fixtures/antigravity-run-prompt.json` | real checkpoint fixture |
| `web/src/lib/observation/RunViewer.svelte` | Transcript and Usage tabs |
| `web/src/lib/observation/UsageView.svelte` | new |
| `web/src/lib/observation/UsageRow.svelte` | new |
| `web/src/lib/observation/MessageRow.svelte` | per-field footer |
| `web/observation_ssr_test.go` | SSR carries scopes and turns |
| `web/package.json` | test script includes `usage.test.ts` |
| `web/scripts/reconcile-usage.ts` | proof script |
| `ephemeral/attest/usage-by-scope/` | probe results, workflow, result, screenshots |

Not changed: `internal/sessionstate/`, `web/src/lib/sessionstate/`,
`events.go` (the union), `jsonschema/`, `web/src/hooks.go`, `hooks.ts`,
`page.server.go`.

## Definition of Done

Gated on the intent's success criteria, per `docs/definition-of-done.md`.
Each is a thing seen working, not a test count.

1. **Totals at every level.** On the proof run's page, live and after, the
   root row and every scope row show five field totals and a by-model
   table. Fields a harness did not report show as unavailable or with an
   unknown count, never as a bare zero. Checked by the screenshot and by
   `reconcile-usage.ts` printing the same numbers.
2. **Drill and reconcile.** Expanding `bakeoff.1` shows two `attempt` rows;
   expanding one shows its turn with the prompt; expanding the turn shows
   its assistant rows. The parent's numbers equal the sum of the rows
   beneath at each step. Checked by the unit test on a three-level tree and
   by eye on the proof run.
3. **Timeline.** The two `attempt` bars overlap on the axis; `research` and
   `review` do not. Open rows extend to now while the run is live. Checked
   by the live screenshot.
4. **Checkpoint alone.** After restarting the server, the finished run's
   page shows the same tree and numbers. Checked in Phase 5 task 3.
5. **Comparable splits.** The Claude session's second turn shows cache read
   greater than zero and `input` smaller than the first turn's; the Codex
   turns show `input` and cache read; the Antigravity turn shows `input`
   below its raw `input_tokens` by exactly `cache_read_tokens`, and cache
   write unavailable. Recorded in `result.md` with the raw numbers.
6. **Fake-adapter tests.** `go test ./...` and `cd web && pnpm test` pass,
   including the new fold, rollup, availability, and reconciliation tests.
7. **Build.** `just build` succeeds; `pnpm run check` reports zero errors.

Exit at 90 to 95 percent: if a presentation quirk remains (column widths,
bar rendering on very short intervals), file it as an issue and merge.

## Risks & Mitigations

- **Provider semantics assumed wrong.** The Gemini "input includes cache"
  reading rests on one run with zero cache reads. Phase 0 runs a warm-cache
  turn on each harness before any normalization changes, and the adapter
  edits are conditional on what it shows.
- **Claude and Codex may be unavailable-by-construction today.** If Phase 0
  confirms that live Claude lacks `output_tokens_details` and live Codex
  lacks a cache write field, then every message from both is currently
  `tokensAvailable: false`, and the intent's criterion 5 depends on Phase 3
  as much as on the agy fix. Phase 3 covers all three adapters for this
  reason.
- **Rollup cost per frame.** Recomputing every turn per frame is wasteful on
  long runs. The per-turn revision cache limits recomputation to the turn
  that changed. Measured on the proof run, not on a claim.
- **A checkpoint predating this sprint.** It has no scopes or turns. The
  page shows an empty Usage tree and the transcript still renders. No shim;
  such runs are attestation history, not a supported input.
- **Task JSON in `ScopeInfo`.** `polytype.Optional[Task]` marshals as
  `""` when absent, which is how `run.jsonl` looks today. The store takes
  a `json.RawMessage` filled only when present, so the snapshot does not
  inherit that quirk.
- **Scope of the page work.** A tree with a time axis is the largest piece.
  Phase 4 lands rows and cells first and the bar second; totals without a
  bar already satisfy criteria 1, 2, 4 and 5.
- **Timestamps from two clocks.** Scope times come from `LifecycleRecord.
  Time` (the run's clock); message times from `AgentEvent.Created` (stamped
  in `stampAgentEvent`, the same process clock). Both are `time.Now()` in
  one process; no cross-host skew.
- **Svelte 5 conventions.** Use runes throughout, keep the recursive row a
  self-import, and run `svelte-autofixer` on each component per
  `CLAUDE.md` before calling it done.

## Dependencies

- Nothing external. The live proof needs the three harness CLIs installed
  and logged in (`claude`, `codex`, `agy` 1.2.1 or later) and the cheap
  tier models: `claude-haiku-4-5-20251001`, `gpt-5.6-luna`,
  `gemini-3.8-flash-low`.
- `just build` before `go test ./...` in a fresh worktree, so the embedded
  web build exists (recorded friction in the Antigravity worklog).
- `web/` tests run with `bun` via `pnpm test`.
- The sealed `LifecycleEvent` union and `jsonschema/LifecycleRecord.json`
  are unchanged, so `go generate ./...` output does not move.

## Open Questions

1. **Row density.** Five cells plus a bar plus a models column is wide.
   Should the default hide cache write and reasoning behind the by-model
   details, showing input, cache read, output only, with a toggle for all
   five? The draft shows all five; Tyler's call at the interview.
2. **Turn rows for sessions used across scopes.** A session created in
   `loop.1` whose turns run in `loop.1/task.3` places those turns under
   `task.3`, which is what `API.md` says. Should the session's card also
   appear under its creating scope with a rolled-up total across all its
   turns? Not in this sprint unless asked; it is a second grouping axis.
3. **Duration cell.** `TurnInfo.Duration` and scope intervals allow a
   duration column and tokens-per-second. Cheap to add; is it wanted now or
   is it Sprint 4 material?
4. **Cost.** Bounded out here. If a price table is wanted later, the
   natural home is a map from `${providerID}/${modelID}` to per-field
   prices in the browser, multiplying the same `Usage` value; nothing in
   this sprint blocks it.
5. **Antigravity cache write.** If Phase 0 shows agy reporting a cache
   write field on some path, wire it; otherwise it stays unavailable and the
   page says so.
