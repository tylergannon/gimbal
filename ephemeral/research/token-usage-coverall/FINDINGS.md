# Token usage coverall: what was found before filing

Written 2026-09-13 on branch `claude/token-usage-coverall-12e8d5`, which was
cut from `claude/token-usage-by-scope-50a907` at `3659f25`. Main has since
moved to `ad6f2f6` (#160, #161, #166, #167); none of those touch the
observation store, the adapters' usage paths, or the page, so the drafts'
file citations still hold, though some line numbers have shifted.

## 1. Branch `claude/token-usage-by-scope-50a907`

Three commits past #156 (`2c4585f`):

- `0ca5eef` Plan Sprint 001: token usage by scope
- `396726c` Rewrite Sprint 001 for what remains after the accounting port
- `3659f25` Add Sprint 001 review, amendments, and two coverall issue proposals

Diff against main: 17 files, 3733 insertions, all under `docs/sprints/`,
`docs/SEMANTIC-INDEX.md`, and `ephemeral/worklog/`. **Drafts only. No Go,
Svelte, or TypeScript changed.** The three named drafts:

- `docs/sprints/drafts/COVERALL-CLAUDE.md` (336 lines): coverall proposal
  that puts the turn record on the existing `Invocation`, forwards
  `Time` and a `TurnChange` through `Lifecycle` from `run.go`, deletes the
  Go scope query, adds two Svelte components, replays the saved issue-149
  logs as a shared Go and TS fixture, and ends with two open decisions.
- `docs/sprints/drafts/COVERALL-ASTRA.md` (156 lines): coverall proposal
  with a separate private `turnInfo` map in `RunSnapshot`, time and turn
  fields decoded from `Lifecycle.Record` inside the observation package,
  an axis driven by `run.updated` (max frame timestamp), a root bar that
  spans `run_started` to `run_ended`, a `Result` field on the turn, a
  recursive snippet instead of new components, and a proof program with a
  200-word preamble to force cache use and a real Loop dispatch. No open
  decisions.
- `docs/sprints/drafts/SPRINT-001-ASTRA-REVIEW.md` (106 lines): adversarial
  review of `SPRINT-001.md` plus amendments. Five findings (three P1, two
  P2), all against the earlier plan, all addressed by both coverall drafts.

## 2. Do the drafts agree on every decision?

The #157 status comment says they do. They agree on every *absorbed-issue*
decision and on the accounting rules. They differ on four points of the
design, two of which the comment's named merge already settles:

| Point | Claude | Astra | Settled by |
|---|---|---|---|
| Where the turn record lives | fields on `Invocation` | separate `Turns` map of private `turnInfo` | comment: Claude's |
| How the store learns times and turn fields | `run.go` fills `Lifecycle.Time` and a new `TurnChange` | decoded from `Lifecycle.Record` in the observation package | comment: Astra's; also the only choice consistent with #169 |
| Live axis end | `Date.now()` inside `$derived` on revision | `run.updated`, the max timestamp of every accepted frame including text deltas | comment: Astra's |
| Root bar | root scope `began`/`ended` | `run_started`/`run_ended` (cleanup included) | comment: Astra's |
| `Result` on the turn | absent | present | comment: Astra's |
| Proof program | five scopes, discards results, Loop dispatch | same topology, results fed to the judge, preamble to force cache, `dispatched` guard | comment: Astra's |

Not settled by the comment (decided in the coverall as written, stated there):

| Point | Claude | Astra | Coverall takes |
|---|---|---|---|
| Message-row cost cell | keep it, showing the step's stated cost (zero today) | remove the dollar label; tokens and model only | Astra's. Tyler's rule "never $0" decides it; #152 stays open anyway |
| Test fixture | copy the saved issue-149 logs under `testdata/` and replay them in one Go test and one TS test asserting the same literal numbers | small synthetic folds only; "no cross-language fixture runner" | After #169, reducing a run directory from its logs *is* the serve path, so one Go test that opens the saved run and asserts per-scope numbers tests production code. TS tests stay synthetic. No shared runner. |
| Page structure | `UsageTree.svelte` + recursive `UsageRow.svelte` | recursive snippet in `RunViewer.svelte`, no new component | implementer's choice; the coverall names the rows and cells, not the files |
| Expand state | `SvelteSet` | "reactive replacement" | implementer's choice; both are reactive |
| Old checkpoints | rows draw no bar | label "not recorded" | moot: #169 deletes checkpoints |

Everything else agrees: turn placement by executing scope; `turn_started`
creates the record; `turn_ended` replaces the usage map wholesale,
including empty; live cells from `session.step.ended` and from
`session.step.failed` only with both cost and tokens, under the session's
configured model; `RunInfo.Usage` stays as the session-total contract;
`RunInfo.ScopeUsage` and the `scopeUsage` remote query are deleted with
their tests and the header line; containment is equal-or-`key/`-prefix
with `""` as root; Codex subtracts cache write (PR #158); a task scope
shows its task once and keeps the `task` value in the runtime; #155 closes
by choosing the observation store as the contract without seeding reducer
info; #13 is superseded by #156 with the quota half declined; ports under
`internal/sessionstate/` and `web/src/lib/sessionstate/` are not edited.

## 3. Consistency with #169

#169 deletes `checkpoint.go`, `loadCheckpoint`, the write at `Store.Close`,
and `observation.json`; a finished run is reduced on request through the
same `Store.Lifecycle` and `Store.Event` the live path uses, from
`run.jsonl` and `sessions/*/*.jsonl`.

- Astra's record decoding (time and turn fields read from
  `Lifecycle.Record`) is consistent: the log record is the input on both
  paths. Claude's `TurnChange` filled by `run.go` would have to be
  reconstructed by the replay reader as well, a second decoder. The
  comment already picked Astra's, so no conflict remains.
- Both drafts carry checkpoint residue that #169 removes: Claude's step 2
  (`loadCheckpoint` defaults) and proof step 8 ("checkpoint alone");
  Astra's edit 2 (`checkpoint.go` in the file list), edit 4 (legacy
  checkpoint labelling), and proof step 6 (copy `observation.json`). The
  coverall drops all of it and replaces the proof step with #169's own:
  open a finished run whose logs predate the change and see the current
  fold.
- Both drafts' "old checkpoints show nothing" caveats disappear: every
  past run gets scope times, turn records, and the tree.

## 4. Per-call cost

No harness reports per-call cost. `claude/events.go` writes `"cost": 0`
on every step and says so in its comment ("Claude Code states no per-step
cost; the turn report carries the cost"); Claude's cost arrives once per
turn from the `result` message's `modelUsage`. `codex/events.go` and
`agy/events.go` write `"cost": 0` on every step, and `codex/codex.go` and
`agy/agy.go` say the harness states no cost at all. So **#152 is left out
of the coverall and stays open.** The price table gives every level a
priced proxy from tokens, which is the comparison Tyler wants, but that is
not a harness-stated per-message cost and does not close #152.

## 5. Duration and price (Tyler's additions)

- Duration: `turn_ended` already carries `duration` in ns; `scope_began`
  and `scope_ended` and `run_started`/`run_ended` carry `time`. A
  duration cell at run, scope, and turn level costs one subtraction per
  row. Scope wall time is `ended − began`; children that overlap (a
  `Group`) sum to more than the parent, which is the fact the page should
  show rather than hide.
- Price: models.dev `api.json` (4.6 MB, 213 providers) is what OpenCode
  loads at the pinned revision `c55ee2a` (`packages/core/src/models-dev.ts`
  embeds `models-dev/snapshot.txt`, 3.6 MB, and refreshes it from
  models.dev). Its `cost` block is `input`, `output`, `cache_read`,
  `cache_write` in USD per million tokens, plus `release_date` and
  `last_updated`. There is **no reasoning price**: providers bill reasoning
  as output. Rows exist for every model Gimble has run:
  `claude-haiku-4-5-20251001` (1 / 5 / 0.1 / 1.25), `claude-fable-5-1`
  (10 / 50 / 0.25 / 12.5), `gpt-5.6-luna` (0.2 / 1.2 / 0.02 / 0.25, with a
  tier above 272k context), `gpt-5.6-sol` (4 / 20 / 0.4 / 5),
  `gemini-3.8-flash` (0.75 / 3.75 / 0.075, no cache write). Antigravity
  reports `gemini-3.8-flash-low`: the effort suffix is Gimble's, the table
  key is `gemini-3.8-flash`, so the lookup has to strip it.
- Release process: there is none. No tags on origin, no goreleaser, no
  release workflow; `.github/workflows/docs.yml` deploys docs only. "Deploy
  a new patch version" means the pipeline issue also has to say what a
  version is (a `vX.Y.Z` tag on main is enough for a Go module).

## 6. Not retrieved

The Gemini share `https://share.gemini.google/R7BJOGLehroE` renders only in
a browser and no browser was reachable. See
`ephemeral/research/quota-telemetry/README.md`.
