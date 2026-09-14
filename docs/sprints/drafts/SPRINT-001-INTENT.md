# Sprint 001 Intent: Token usage by scope

## Seed

Tyler, 2026-09-12: "that would be fucking SICK if we had token totals
being added up at the level of each scope, and a timeline that would
support post-run inspection at any aggregate level as well as at the
level of a specific task prompt or individual message. But being able to
always see where the tokens are being spent, per model, cached/new, etc,
would be awesome."

## Context

- Gimble restarted from an empty tree on 2026-09-10. The root package is
  the API; `web/` is a `tylergannon/skgo` app that shows every run live;
  `internal/observation` is the per-run store the page reads (live from
  memory, finished runs from `runs/<id>/observation.json`).
- Token data is already captured at the model-call grain for all three
  harnesses (Claude, Codex, Antigravity via `agy`). Each emits an
  OpenCode-shaped `session.step.ended` with `tokens {input, output,
  reasoning, cache {read, write}}`, and `session.step.started` names
  `model {providerID, id}`. The projection keeps both on every assistant
  message row, with `time.created` and `time.completed` in ms. Confirmed
  in the real run at
  `ephemeral/attest/antigravity-run-prompt/logs/runs/20260911-214914.run-prompt/observation.json`.
- Every message is placed by scope key, session id, and turn id. Scope
  keys are paths with ordinals (`lap.3/bakeoff.1/attempt.2`); containment
  is prefix; the scope tree is the prefix tree of keys. So "tokens in
  scope X" is a sum over messages whose scope key starts with X. No new
  identity is needed.
- The `turn_ended` lifecycle record already carries the per-step token
  list (`[]JSONText`) and the turn's duration in `run.jsonl`.
- The page applies event frames incrementally with a revision counter
  (`web/src/lib/observation/index.ts`), so a rollup can be maintained per
  frame.

What is missing, from the readiness assessment done before this sprint:

1. The observation snapshot has no scopes and no turns. `RunSnapshot` is
   run info, sessions, and invocations (turn projections). `scope_began`,
   `scope_ended`, `turn_started`, `turn_ended` are written to `run.jsonl`
   and streamed as lifecycle frames, but `Store.Lifecycle` folds only run
   status and sessions, and the browser's `foldLifecycle` mirrors that.
   The post-run checkpoint therefore never has scope begin/end times, task
   prompts, or turn prompts.
2. The run page (`RunViewer.svelte`) is a flat list of turns with message
   rows. No scope hierarchy, no timeline, no totals. Sprint 3 of
   `ephemeral/research/api/SPRINTS.md` already asked for "scopes from key
   prefixes ... siblings on a timeline with overlaps drawn as concurrent
   ... turns with tokens and duration". None of it is built.
3. Antigravity's `input` likely double counts cached tokens. Codex
   normalizes `input = inputTokens - cached - cacheWrite`; Claude's
   `input_tokens` already excludes cache. `agy/events.go` passes
   `input_tokens` through untouched and never reports cache write.
4. Token availability is all-or-nothing. The Antigravity run had real
   numbers but the sidecar marked `tokensAvailable: false` because cache
   write was missing, so `MessageRow.svelte` prints "unavailable". A rollup
   must use per-field availability or it silently drops whole models.
5. No adapter reports cost (`costAvailable: false`, `cost: 0`) and there
   is no price table.

## Pyramid Index

- L0: Fold scopes and turns into the run observation, fix the two token
  accounting inconsistencies in the adapters, and give the run page a
  usage view that rolls tokens up the scope tree per model with a
  cached/new split, drillable to a turn's prompt and a single message,
  live and after the run.
- L1:
  - Observation: `internal/observation` snapshot gains scopes (key, name,
    parent, task, began, ended, error) and turns (prompt, output type,
    started, ended, duration, error, interrupted), folded from the
    lifecycle records the run already writes; the browser fold mirrors it.
  - Adapters: Antigravity input excludes cached tokens like Codex and
    Claude; accounting availability is per field, not all-or-nothing.
  - Page: a usage view on the run page. Scope tree with totals per model
    and per field (input new, cache read, cache write, output, reasoning),
    drill to turn and message, and a timeline over run time at the chosen
    aggregate level. Same code for live and finished runs.
  - Proof: a fake-adapter test for the fold and the rollup; a live run on
    the cheap tier that produces nested scopes across two harnesses and a
    page screenshot showing totals that reconcile with the run log.
- L2:
  - "What is missing" items 1 and 2 expand the observation and page
    surfaces; items 3 and 4 expand the adapter fixes; item 5 bounds cost
    out unless a draft argues for a price table.
  - `ephemeral/research/api/API.md` § Observability (Keys, Events) is the
    contract for placement and the scope tree.
  - `ephemeral/research/api/SPRINTS.md` Sprint 3 is the page scope this
    sprint partly delivers.

## Semantic Index

- **Not configured** for retrieval; the token cache is local and small.
  Token cache: `docs/` and `ephemeral/` under the repository root. Index:
  not built (see `docs/SEMANTIC-INDEX.md`). Prior art retrieved with `rg`:
  `API.md` § Observability says "usage per model call (tokens, duration);
  the turn's ended event sums it"; `ephemeral/research/issue-130/execution/DELIVERY.md`
  records that the live UI "correctly shows unavailable cost, observed
  tokens"; `ephemeral/worklog/202609112050-issue-130-frontend-cost.md`
  fixes per-delta invalidation at the keyed message row and keeps the
  reducer as the only transcript model. Nothing in `ephemeral/legacy`
  aggregates tokens by scope.

Planning agents drafting this sprint should read the files named above and
run `rg` against `ephemeral/` for anything else they need. Do not scan
`ephemeral/legacy` wholesale; it is inspiration, not the API.

## Chapter Context

No chapter link selected. The repository has no `docs/chapters/`.

## Recent Sprint Context

First sprint in `docs/sprints/`. The historical sprints 0 through 2 in
`ephemeral/research/api/SPRINTS.md` (the generated app, the API, events
into files) are done; Sprint 3 (the page, first pass) is partly built:
the run page renders live transcripts from the observation store
(PR #138, "Adopt OpenCode session events and snapshot-first live run UI").

## Relevant Codebase Areas

- `events.go`: sealed `LifecycleEvent` union (`ScopeBegan`, `ScopeEnded`,
  `TurnStarted`, `TurnEnded` with `Tokens []JSONText`), `LifecycleRecord`.
- `scope.go`, `session.go`, `run.go`, `event_persistence.go`: where
  lifecycle records are produced and handed to the store
  (`run.observeLifecycle`).
- `internal/observation/`: `store.go` (`Lifecycle`, `Event`, snapshot),
  `snapshot.go` (`RunSnapshot`, `Placement`, `SessionInfo`),
  `checkpoint.go` (`observation.json`), `http.go` (`/api/runs/{id}`, SSE).
- `internal/sessionstate/`: the OpenCode projection; assistant rows carry
  `tokens`, `cost`, `model`, `time`. Do not change the port.
- `claude/events.go`, `codex/events.go`, `agy/events.go`: usage
  normalization and the `accounting` sidecar in `NativeRef`.
- `web/src/lib/observation/index.ts`, `RunViewer.svelte`,
  `SessionTimeline.svelte`, `MessageRow.svelte`: the page.
- `web/src/routes/runs/[runID]/page.server.go`, `web/src/hooks.go`: the
  snapshot crossing SSR as its own JSON (an open schema, so adding fields
  needs no transport change).
- `internal/modelalias`: CLI model names to provider ids, if per-model
  display wants friendly names.
- Tests: `internal/observation/store_test.go`, `web/src/lib/observation/index.test.ts`
  (run with `cd web && pnpm test`), `gimble_test.go` and `events_test.go`
  over the fake adapter.

## Constraints

- Must follow `AGENTS.md`: as simple as possible; no wrappers; a new
  exported name in package `gimble` only if Tyler asks for it by name; no
  reflection; `gimble` never imports `web`; `generated/` is written by
  `go generate ./...`; no backwards compatibility or shims (an
  `observation.json` written before this sprint may simply lack scopes).
- The observation store is the producer and never blocks on a subscriber;
  it never reads a log to answer a live connection; finished runs are
  served from one checkpoint file. Keep that.
- The OpenCode projection in `internal/sessionstate` is a port and is not
  to be edited for this feature.
- Live and post-run must be one code path in the browser.
- Proof is a live run on real models on the cheap tier (`gpt-5.6-luna`,
  Claude Haiku, `gemini-3.8-flash`); the fake adapter is for unit tests.
  Say which model a run used.
- Do not add GitHub automation; agents file issues themselves.

## Success Criteria

- Open a run page, live or finished, and see for the whole run and for
  every scope in the tree: tokens per model, split into input (new),
  cache read, cache write, output, and reasoning, with unavailable fields
  shown as unavailable rather than zero.
- Drill from a scope to its turns (each with its prompt or task) and from
  a turn to its individual assistant messages, with the same numbers at
  every level, and the levels reconcile (children sum to the parent).
- A timeline over the run's wall clock that shows the chosen level (scope
  bars, or turns, or messages) with their token weight, so where the
  budget went and when is visible at a glance.
- A finished run's checkpoint alone is enough to render all of the above.
- Antigravity, Codex, and Claude runs report comparable cached/new splits.

## Open Questions

- Should the rollup be computed in Go (in the store and checkpoint) or in
  the browser from message rows and placement? Both need the same
  definition; what is the one place it lives?
- What does the timeline draw at each level, and how does a person pick
  the level: a tree with collapsible rows and a time axis, or separate
  views?
- Should `TurnEnded.Tokens` stay opaque `[]JSONText` or become the typed
  per-step token record, given the sealed Polytype union and the JSON
  schema generation?
- Is a price table in scope for cost, or does this sprint stop at tokens?
- How are Antigravity's `cache_read_tokens` defined relative to
  `input_tokens`, and can that be verified against a live `agy` run before
  changing the normalization?
- What is the smallest live workflow that exercises nested scopes,
  concurrent siblings, and two harnesses, for the proof run?
