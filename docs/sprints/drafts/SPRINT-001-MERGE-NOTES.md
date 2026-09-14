# Sprint 001 Merge Notes

Synthesis of the three drafts, the three critiques, and Tyler's interview
answers on 2026-09-12.

## Claude Draft Strengths
- Usage as an additive value: each field a cell of tokens plus an unknown
  count, so any grouping sums to the same value and parents equal children
  by construction. Adopted.
- One rollup, in the browser, over checkpointable rows. Go persists scopes
  and turns, never arithmetic. Adopted.
- Concrete Go and TypeScript types for `ScopeInfo`, `TurnInfo`, the usage
  module; times as Unix milliseconds, the unit message rows already use.
  Adopted with the root name and segment-safe containment fixes.
- Per-turn revision cache so a text delta recomputes one turn. Adopted.
- A live probe on all three harnesses before any normalization change, and
  the finding that Claude and Codex are probably unavailable-by-construction
  today (Claude requires a reasoning split, Codex a cache-write field).
  Adopted.
- Lean proof: inline Go workflow, a reconcile script, screenshots, restart.
  Adopted, with the checkpoint-only directory from Codex.

## Codex Draft Strengths
- Root scope is recorded as `"."`; the fold names it after the run. Adopted.
- Containment is equality or prefix followed by `/`, so `attempt.1` does
  not contain `attempt.10`. Adopted.
- Tool-only assistant rows (the Antigravity tool step) are not model calls
  and must not count in any denominator. Adopted, using the absence of an
  `accounting` sidecar as the test.
- Parent invariant stated explicitly: scope = direct turns + immediate child
  scopes, per model and per field, unknown counts included. Adopted.
- Task rides as detached JSON; record time is forwarded, never re-stamped;
  turn metadata exists from `turn_started`; `TurnEnded.Tokens` is a
  cross-check, never an addend; sessions are charged where the turn ran.
  Adopted.
- Restart against a directory holding only `observation.json`. Adopted.
- The Phase 3 "required examples" list. Adopted as the test list.

## Gemini Draft Strengths
- The page picture: a collapsible tree, name on the left, wall-clock bars
  beside, token cells at the row's end, per-model detail under the row.
  Adopted as the layout (Tyler chose it).
- The normalization table, five fields by three adapters with the source
  key for each. Adopted as the accounting contract.
- The four-bullet description of what `Store.Lifecycle` folds. Adopted.
- Plain Svelte and CSS for the timeline; no chart library. Adopted.
- Proof workflow size: one outer scope, a group of two concurrent attempts
  on two harnesses. Adopted, plus Codex's direct root turn and a Haiku
  session so all three harnesses appear.

## Consensus Critiques (multiple agents agreed)
- Do not compute the rollup in both Go and TypeScript. (Claude, Codex)
- A boolean availability per model cannot express partial coverage; a
  `Total` over unknown fields is not a total. (Claude, Codex)
- Antigravity's cache formula is a hypothesis; the definition of done names
  the evidence, not the formula. (Claude, Codex)
- `TaskInfo{Summary, Goal}` does not match `Task`; keep the task payload as
  raw JSON. (Claude, Codex)
- Codex's Phase 5 is twice the intent's size; cut the Playwright feature
  and the SSR extension. (Claude, Gemini)
- Claude's fixture expectation of `cacheWrite.unknown === 2` contradicts
  its own rule that every completed row without accounting counts as
  unknown (three rows). Resolved by excluding rows without an accounting
  sidecar from model calls, which makes 2 correct. (Codex, Gemini)
- All three adapters share the all-or-nothing rule, and the
  `tokensAvailable` key must be deleted or redefined. (Claude critique)

## Valid Critiques Accepted
- Root name, segment-safe prefix, tool-only rows, direct-plus-children
  invariant, record time, one time unit (all above).
- Decide the three "if required" items now: `writeLifecycle` returns the
  record time; the sidecar-ordering question was checked on the saved
  Antigravity log (the step-ended sidecar with accounting is the last one
  for each message, and Claude clears its message id after the step ends),
  so `identity.go` is unchanged; a negative derived value marks that field
  unavailable rather than clamping to zero, in Codex and Claude as well.
- Delete `tokensAvailable` from all three adapters and delete the browser's
  `accounting()`; the page reads `fieldAvailability` only.
- Rollup cost claim measured on the proof run, not asserted.
- The reconcile script checks per-turn and per-model placement, not only
  the root sum.
- Task display: an expanded task scope shows its task name and description.

## Critiques Rejected (with reasoning)
- Gemini's "compute in Go too for headless readers": no such reader exists;
  the checkpoint holds the inputs, and a program that needs totals sums by
  prefix when the workflow that needs it exists (`AGENTS.md`).
- Gemini's objection to the negative-value rule as "rigid": a negative
  derived count is a provider inconsistency, and the raw numbers stay in
  `rawProviderAccounting`; hiding it behind zero is the thing the sprint is
  removing.
- Codex's separate `UsageTree` and `UsageTimeline` with a level switch:
  Tyler chose the single tree with bars in the row. Expanding is the level
  gesture.
- Codex's keyboard operability and narrow-screen work, Gemini's view
  toggles ("By Model", "Flat Transcript"), Gemini's fabricated `ended`
  fallback, Claude's Usage tab: not asked for, or replaced by the chosen
  layout.
- Codex's Playwright feature and SSR test extension: no success criterion
  maps to them.

## Interview Refinements Applied
- Layout: one tree with split columns (name, bar, five cells), per-model
  table under an expanded row.
- Proof: lean.
- Extras: none. No segmented bar, no duration column, no ticking clock.
  While a run is live the axis ends at the latest timestamp the snapshot
  holds and open bars extend to it; every frame moves it.
- Adapters: probe first on the cheap tier; change all three per the probe;
  delete `tokensAvailable`; keep `fieldAvailability` and raw accounting.

## Final Decisions
- Go: fold `scope_began`, `scope_ended`, `turn_started`, `turn_ended` into
  `Scopes` and `Turns` on `RunSnapshot`; checkpoint carries them.
- Browser: mirror fold; `usage.ts` is the one usage definition; the page
  renders one tree from the root scope with bars on the run's wall clock.
- Adapters: Phase 0 probe, then normalization per the probe; delete
  `tokensAvailable`; negative derived values are unavailable.
- Proof: one inline workflow on Haiku, `gpt-5.6-luna`, `gemini-3.8-flash-low`
  with a direct root turn, a warm-cache scope, a concurrent group, and a
  nested scope; `reconcile-usage.ts`; live and finished screenshots;
  checkpoint-only restart.
- Out: cost, `TurnEnded.Tokens` typing, the projection port, tabs, extras.
