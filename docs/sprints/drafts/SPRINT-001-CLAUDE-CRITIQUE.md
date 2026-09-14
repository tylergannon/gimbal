# Sprint 001 Critique: Codex and Gemini drafts

Status: Claude critique, 2026-09-12. Reviews `SPRINT-001-CODEX-DRAFT.md` and
`SPRINT-001-GEMINI-DRAFT.md` against `SPRINT-001-INTENT.md`, `AGENTS.md`,
`docs/definition-of-done.md`, and the code as of `2a52971`. Every factual
claim below that a draft makes about the repository was checked against the
source; where a draft is wrong about the code, this says so.

## Pyramid Index

- L0: The Codex draft is the sounder plan and should be the skeleton of the
  merged sprint; the Gemini draft contributes the concrete UI shape, the
  clearest statement of the normalization table, and a leaner proof, but its
  two-implementation rollup, boolean availability, and unverified Antigravity
  formula should not survive the merge.
- L1:
  - Both drafts agree on the skeleton: adapters first, then fold scopes and
    turns into the store and checkpoint, then the browser fold, then the
    page, then a live proof. Neither changes `TurnEnded.Tokens`, prices
    anything, or edits the `sessionstate` port. That agreement is solid.
  - Codex wins on accounting semantics (additive sum plus contributing and
    observed counts), on evidence discipline for Antigravity, on the root
    scope naming bug, on time-axis alignment, on the checkpoint-only restart
    proof, and on keeping one rollup implementation.
  - Gemini wins on legibility (diagram, normalization table, ASCII mockup,
    concrete Go types), on a proof that matches the intent's size, and on a
    phase list an implementer can start from without further design.
  - Gemini's material defects: rollup computed in both Go and TypeScript
    with no drift mitigation; per-model availability as one boolean, which
    cannot express partial coverage; a `Total` that silently sums unknown
    fields; the Antigravity formula baked into the definition of done
    before it is verified; a fabricated `ended` fallback for crashed scopes;
    a `TaskInfo` that redefines `Task` with the wrong fields; no Pyramid
    Index; no checkpoint-only page test.
  - Codex's material defects: dense prose with no picture of the page; the
    proof phase is roughly twice the size the intent asked for; several
    decisions are left as "if required" for the implementer; a rule about
    negative derived values that contradicts the current Codex and Claude
    normalizers it says not to change.
- L2:
  - [Verified facts](#verified-facts): what each draft got right or wrong about the code.
  - [Codex draft](#codex-draft): six criteria, strongest ideas, weaknesses.
  - [Gemini draft](#gemini-draft): six criteria, strongest ideas, weaknesses.
  - [Open questions, head to head](#open-questions-head-to-head): the intent's six questions.
  - [Merge recommendation](#merge-recommendation): what to take from each.

## Verified facts

| Claim | Source | Verdict |
|---|---|---|
| All three adapters already emit `accounting.fieldAvailability` (Codex) | `agy/events.go:301`, `codex/events.go:560`, `claude/events.go:348` | True. The five keys are `input`, `output`, `reasoning`, `cacheRead`, `cacheWrite`. |
| The browser's `accounting()` gates on `tokensAvailable` and prints raw JSON (Codex) | `web/src/lib/observation/index.ts:109` | True. `JSON.stringify(message.tokens)` or the word `unavailable`. |
| `available = inputOK && outputOK && reasoningOK && cacheReadOK && cacheWriteOK` (Gemini) | `agy/events.go:339`, `claude/events.go:437` | True, and Claude has the same all-or-nothing rule. Codex's normalizer folds all five field flags too (`codex/events.go:564`). The defect is in all three adapters, not only Antigravity. Neither draft says that plainly. |
| Antigravity passes `input_tokens` through untouched (both) | `agy/events.go:337` | True. |
| The saved Antigravity run has `cache_read_tokens: 0` on every usage row (Codex) | `ephemeral/attest/antigravity-run-prompt/.../observation.json` | True. Two rows with accounting (14980 and 15232 input, both cache read 0, cache write unavailable, `tokensAvailable: false`), and two provenance entries with no accounting at all, one of which is the tool-only assistant row Codex describes. |
| The root scope's recorded `ScopeBegan.Name` is `"."` (Codex) | `scope.go:74` uses `path.Base(s.key)` and the root key is `""` | True. `path.Base("")` is `"."`. Gemini does not notice; its tree would label the root `.`. |
| `LifecycleRecord` already carries `Time` (Codex says use it, do not call `time.Now()` again) | `events.go:162` | True. `observation.Lifecycle` does not forward it today (`store.go:30`). |
| `observation.Lifecycle` has only `Placement`, `Record`, `Name`, `Status`, `Error`, `Session` (both) | `store.go:30` | True. Both drafts extend this struct; neither creates an import cycle. |
| `web/src/hooks.ts` needs changing (Gemini Phase 3) | `web/src/hooks.ts` | False. It parses the transported JSON as an open `RunSnapshot`; new fields cross without edits. Codex says so correctly. |
| `Task` has `Summary` and `Goal` (Gemini's `TaskInfo`) | `go doc Task` | False. `Task` is `Name`, `Description`, `DefinitionOfDone`, `Validation{Command, Query}`. |
| Codex's `input` field availability depends on both cache fields | `codex/events.go:561` | True. Under Codex's own rule that derived fields need all inputs known, an Antigravity `input new` computed as `input - cacheRead` is available when those two are known, regardless of cache write. Codex's draft is consistent with this; Gemini's table is too. |
| The test script is an explicit file list (Codex, Phase 3) | `web/package.json:12` | True. A new `usage.test.ts` would not run unless added there. Good catch. |
| `just build`, `just e2e`, an existing Playwright feature (Codex) | `justfile:4,17`, `e2e/features/generated-app.feature` | True. |
| A sprint needs a Pyramid Index (`df-sprint-plan`) | `.agents/skills/df-sprint-plan/SKILL.md:14` | Codex has one. Gemini has none. |

## Codex draft

### Architectural soundness

Strong. The four architecture sections make the right calls and, more
importantly, name the traps an implementer would fall into:

- Root key `""` displays as the run name, not `"."`. Parent is the key
  minus its last segment. Containment is equality or prefix followed by
  `/`, so `attempt.1` does not contain `attempt.10`.
- Lifecycle timestamps come from the record, not a second `time.Now()`.
  Turn duration stays a Go duration with one explicit conversion so
  lifecycle times and the projection's millisecond `time.created` land on
  one axis. Gemini's `time.Time` fields serialize as RFC 3339 strings next
  to millisecond integers and the draft never says how the page reconciles
  them.
- The task rides as detached JSON rather than a second `Task` type in the
  observation package. That is the only way to avoid either an import
  cycle or a duplicate definition, and it is what Gemini's `TaskInfo` gets
  wrong.
- Turn metadata exists from `turn_started`, so a turn that fails before
  its first native message is still visible. Gemini's fold would create it
  at `turn_started` too, but never states the failure case as a
  requirement.
- One rollup implementation, in the browser, over checkpointable inputs.
  Go persists facts, not arithmetic. This is the `AGENTS.md` answer.
- Availability as sum plus contributing count plus observed count. This is
  the single best idea in either draft: it is additive, it distinguishes a
  measured zero from an omission from a partial parent, and it composes
  identically at message, turn, scope, and run level. Gemini's per-model
  boolean cannot say "three of five calls reported cache write."
- `TurnEnded.Tokens` is a cross-check, never an addend. Sessions are
  charged where the turn ran, not where the session was created, so a
  forked session's inherited history is not billed twice.

One internal contradiction: section 2 says "do not clamp an inconsistent
negative derived value into an apparently measured zero," but the current
Codex and Claude normalizers do exactly that with `max(0, ...)`, and the
same section says to change them only where evidence requires. The merged
plan has to pick one: either the clamp stays and the rule is dropped, or
the rule stands and Phase 1 changes both normalizers to mark the derived
field unavailable when the subtraction goes negative. The second is more
honest and is a small change; say so explicitly.

A second soft spot: "Preserve accounting when a later unrelated message
sidecar arrives if that case occurs" and "`identity.go` only if
preserving accounting across later sidecars is required" leave a real
question for the implementer to discover mid-sprint. `foldProvenanceLocked`
keeps the latest sidecar per message, so a later sidecar without
`accounting` (a delta after the step ended, say) would replace the one with
it. Whether the adapters ever emit that ordering is checkable now with
`rg` over the saved session logs; the plan should check and decide.

### Completeness

Covers every item in the intent's "what is missing" list and every success
criterion, including the two the intent states but Gemini's phases never
exercise: the checkpoint-only render (Phase 5 copies `observation.json`
into a fresh project and restarts the server against it) and the
three-harness comparison (a short Haiku turn in the live run).

It also covers cases the intent did not spell out but which a real run
produces: tool-only assistant rows, root-level direct turns, one session
used across scopes, reconnect replacement, a turn with no native message,
cancellation precedence in the browser fold. The "Required examples" list
in Phase 3 is a usable test plan on its own.

Missing: any picture of the page. Section 4 describes two components and a
level selector in prose; a reader cannot tell whether the tree and the
timeline are side by side, stacked, or tabbed, or what a row looks like.
Gemini's mockup answers that in twenty lines.

### Phasing and ordering

The order is right and the dependencies are honest. Phase 1's Antigravity
evidence is the only external blocker and the draft correctly decouples it:
metadata, rollup, and page work proceed on the existing sidecar. Two
adjustments would help:

- Split Phase 1 into a spike (capture one nonzero-cache `agy` sample,
  record the emitter's field construction) that runs in parallel with
  Phase 2, and the normalization change that lands once the spike answers.
  As written, an implementer reads Phase 1 as a gate.
- Phase 3's `usage.ts` and Phase 4's components depend on the Phase 2
  TypeScript types, but Phase 2 lists the browser fold under Go files.
  Move the `index.ts` fold into Phase 3 so each phase is one language
  boundary, or say why the split is where it is.

### Risk coverage

The best of the two. Every risk has a concrete mitigation that appears as a
task somewhere in the plan, and the table includes the two risks the Gemini
draft does not see: the proof harness growing into infrastructure, and
snapshot/fold drift between Go and TypeScript (mitigated by feeding real Go
output through the browser fold). The "live caching or overlap does not
appear" row is the honest one: a cheap live run may simply not show a cache
hit, and the draft says what to do then instead of pretending.

### Feasibility

Feasible but heavy. Phases 1 through 4 are proportionate. Phase 5 asks for
a proof program with a deterministic mode and a live mode, a new Playwright
feature with geometry assertions, an SSR test extension, a checkpoint-only
restart, an independent reconciliation table, and a record of which claims
were live versus deterministic. The intent asked for a fake-adapter test,
one live run, and a screenshot whose totals reconcile with the run log.
Codex's Phase 5 is roughly twice that. The checkpoint-only restart and the
reconciliation table earn their place because they map to success criteria.
The Playwright geometry assertions and the SSR extension do not map to any
criterion and are the first things to cut.

Phase 4's keyboard operability and narrow-screen scrolling are good
engineering and not asked for. `AGENTS.md` says build what was asked; leave
them as notes, not checklist items.

### Definition of done

Nine observable items, each phrased as something a validator can look at,
plus a scope-discipline item that restates `AGENTS.md`. The "Adapter
comparison" item is the right shape: it requires evidence and a nonzero
cache observation rather than asserting a formula. The required-checks
block is specific (build first because Go web tests embed the app, then
sequential checks). The closing sentence about the planning task not
running models is boilerplate and can go.

### Strongest ideas to keep

1. Sum plus contributing count plus observed count as the availability
   representation, with the four display states (all observed, partial,
   unavailable, no model calls) and "pending" for a running call.
2. One rollup, in `usage.ts`, over canonical rows; Go persists facts only.
3. Root scope naming, segment-safe containment, and the `attempt.1` versus
   `attempt.10` test.
4. Record time forwarded from the lifecycle record; one explicit
   nanosecond-to-millisecond conversion; one axis.
5. Task as detached JSON.
6. Turn metadata created at `turn_started`; a turn without native messages
   is a required test case.
7. Tool-only assistant rows excluded from the model-call denominator,
   using existing provenance.
8. `TurnEnded.Tokens` as cross-check only.
9. The checkpoint-only restart proof.
10. Adding tests to `index.test.ts` because the test script is an explicit
    list.
11. The Phase 3 "Required examples" list.

### Weaknesses and gaps

- No page mockup or diagram; section 4 is hard to implement from.
- Phase 5 is oversized relative to the intent.
- Three "if required" decisions (`event_persistence.go`, `identity.go`,
  later sidecars) that can be settled now.
- The negative-derived-value rule contradicts the current clamps.
- Does not say plainly that all three adapters share the all-or-nothing
  defect, so an implementer might fix only `agy`.
- Says nothing about the `tokensAvailable` key itself. After this sprint it
  is either redefined or dead. Leaving a key that means "all five fields
  known" in the sidecar while the page ignores it is a trap for the next
  reader. The merged plan should delete it or redefine it, per `AGENTS.md`
  ("delete what is replaced").

## Gemini draft

### Architectural soundness

Mixed. The structural decisions are mostly right and the presentation is
much clearer, but three choices are wrong for this repository.

Right:

- The containment rule in section 1 is stated correctly, including the
  `+ "/"` boundary and the empty root.
- The normalization table in section 2 is the clearest statement in either
  draft of what the five fields mean per adapter, and it matches the
  current Codex and Claude code.
- Per-field availability with `—` for unknown fields.
- Rollup derived from message rows placed by invocation, with the
  reconciliation invariant written down.
- Vanilla Svelte plus CSS for the timeline; no chart library. The SSR risk
  row explains why, which Codex only implies.

Wrong:

- Rollup in both Go and TypeScript, "identical logic." `AGENTS.md` says as
  simple as possible. Two implementations of the same arithmetic in two
  languages with no shared test vector is the classic drift setup, and the
  draft's own risk table does not list drift. The stated rationale (a CI
  script reads totals from `observation.json`) names a consumer that does
  not exist, and the checkpoint already contains the inputs any script
  would need. The intent's open question asked for the one place the
  definition lives; "both" is not an answer to that question.
- `ModelUsage.FieldAvailability map[string]bool` per model per scope. A
  boolean cannot express that some of a scope's calls reported cache write
  and some did not. The draft has no partial state at all, so a parent must
  either show the field as available (silently dropping the calls that
  lacked it) or unavailable (hiding known subtotals). Either way the
  success criterion "unavailable fields shown as unavailable rather than
  zero" is met at the message but broken at the scope.
- `TokenTotals.Total` alongside fields that may be unavailable. A total
  that includes an unknown addend is not a total. Codex marks the scalar
  partial; Gemini does not address it.

Also wrong in detail:

- `TaskInfo{Summary, Goal}` does not match `Task`, which is `Name`,
  `Description`, `DefinitionOfDone`, `Validation`. Redefining `Task` in the
  observation package is also a second definition of a root type; detached
  JSON avoids both problems.
- `Duration time.Duration` marshals as nanoseconds; `Began time.Time`
  marshals as an RFC 3339 string; message rows carry millisecond integers.
  The draft never puts these on one axis.
- The root scope will be named `"."` unless the fold special-cases it. Not
  mentioned.
- Four typed pointer fields on `observation.Lifecycle`
  (`ScopeBegan *ScopeBeganInfo` and so on) mirror the sealed union in a
  second package. Flat optional fields on the existing struct do the same
  job with less surface.
- "Antigravity double-counts cached tokens" is asserted as fact in the
  overview. The intent says "likely," and the only recording has cache
  read zero throughout, which cannot distinguish the two readings. The
  Gemini API's `promptTokenCount` does include cached tokens, so the
  hypothesis is probably right, but whether `agy`'s `input_tokens` is that
  number is exactly what needs one live sample to settle.

### Completeness

Covers the intent's five missing items and most success criteria. Gaps:

- No task or test demonstrates the checkpoint-only page. DoD item 2 claims
  it; nothing in Phase 5 shows it.
- The live proof uses Codex and Antigravity only. The success criterion
  names three harnesses reporting comparable splits, and Claude's
  normalizer has the same all-or-nothing defect, so it should appear in
  the run.
- No handling of tool-only assistant rows, which the only saved
  Antigravity run contains.
- No handling of direct turns in a scope that also has child scopes,
  beyond the `+ Direct` term in the invariant. The UI mockup shows turns
  only under leaf attempts.
- No reconnect or snapshot-replacement case, though the browser class
  already does replacement and a derived rollup must survive it.
- No cancellation-precedence mirror in the browser fold (the Go store
  keeps `cancelled` over a later `run_ended`; `foldLifecycle` does not).
  Codex catches this.
- `web/src/hooks.ts` is listed for modification and needs none.
- No Pyramid Index, which the sprint template requires.

### Phasing and ordering

The same five phases as Codex in the same order, and easier to read: each
phase is files then numbered tasks. Phase 1 is not gated on evidence, which
makes it faster to start and easier to get wrong. Phase 2 puts the Go
rollup engine (`ScopeRollupLocked`) before Phase 3 builds the same thing in
TypeScript, so the drift risk is designed in from the ordering.

Phase 4's three view toggles ("Hierarchy & Timeline", "By Model", "Flat
Transcript") are scope growth. The intent asked for one usage view; "By
Model" is a regrouping of the same numbers and "Flat Transcript" is the
current page. Keep the flat transcript reachable (it is the existing
drill target) and drop the toggle bar.

### Risk coverage

Five risks. Two are good (SSR engine compatibility, coarse-grained
invalidation). One is wrong: the "unanchored scope duration" mitigation
falls back to the last observed event or the run's end time, which
fabricates an `ended` the run never recorded. The intent says show
unavailable rather than zero; the same principle applies to time. Draw the
open bar to the run end if you must, but label it open and leave `ended`
null. Missing: drift between the two rollups, Antigravity semantics being
wrong, reconnect double counting, and the proof run failing to show
caching or overlap.

### Feasibility

The most feasible of the two as written, because it is smaller: four new
components, one proof run, existing test commands. It is also the one more
likely to ship a subtle wrong number, because nothing in it forces the
implementer to confront partial availability, tool-only rows, or the root
name. The Gemini plan would be done sooner and corrected later.

The `Dependencies` section (Go version, Node version, pnpm version,
Svelte) is filler; nothing in it is a decision.

### Definition of done

Five numbered groups, each concrete, and it cites `definition-of-done.md`.
Two problems. Item 1 bakes `max(0, input_tokens - cache_read_tokens)` into
the DoD before anyone has verified that `input_tokens` includes cache
reads; if the sample says otherwise the DoD is wrong, not the code. Item 5
requires a screenshot but no reconciliation method; "verified against a
captured browser screenshot" does not say what the screenshot is compared
to. Codex's "per-model, per-field reconciliation to raw session
accounting" is the missing sentence.

### Strongest ideas to keep

1. The ASCII page mockup: tree on the left, wall-clock bars on the right,
   token weight at the row's end, a detail pane below for the selected
   turn or message. This answers the intent's second open question
   concretely and should be the merged plan's picture.
2. The normalization table (five fields by three adapters with the exact
   source key for each).
3. `TokenBar.svelte`, a segmented bar for new, cache read, cache write,
   output, reasoning. It is the fastest way to see cached versus new at a
   glance, provided numeric labels stay beside it.
4. Live duration timers and active badges for open scopes.
5. The proof workflow's size: one outer scope, a group with two concurrent
   attempts on two harnesses, multiple turns per session. Add Codex's
   direct root turn and Haiku turn and it is the right run.
6. The `Store.Lifecycle` fold description (four bullets, one per record
   kind) is the clearest statement of what the fold does.
7. The SSR risk row and its mitigation.

### Weaknesses and gaps

- Two rollup implementations.
- Boolean availability; no partial state; a `Total` over unknown fields.
- Antigravity formula asserted and baked into DoD without verification.
- `TaskInfo` fields wrong; duplicate `Task` definition.
- Time axis (nanoseconds, RFC 3339, milliseconds) unaddressed.
- Root scope named `"."`.
- Fabricated `ended` fallback.
- No checkpoint-only page proof; no three-harness proof; no tool-only row
  handling; no reconnect case; no cancellation precedence in the browser.
- View toggles and four components where two would do.
- No Pyramid Index.

## Open questions, head to head

| Intent question | Codex | Gemini | Assessment |
|---|---|---|---|
| Where does the rollup live? | Browser only, `usage.ts`; Go persists inputs | Both, "identical logic" | Codex. One definition, no drift, no phantom consumer. If a headless reader ever appears, it can run the same TypeScript or compute from the same rows. |
| What does the timeline draw, and how is the level chosen? | Tree and timeline as two components sharing a selection; Scope/Turn/Message levels | One unified tree with bars beside each row; expand inline | Gemini's picture, Codex's levels. A unified tree-with-bars is the at-a-glance view; Codex's level selector is how you get "all descendant messages on one axis" without expanding every node. Both fit in Gemini's layout. |
| `TurnEnded.Tokens` typed? | Keep opaque; cross-check only | Keep opaque; typed data already arrives via `session.step.*` | Agreed. Both give the same reason. |
| Price table? | No | No | Agreed. Both bound it out. Gemini's quote of the seed is a nice touch. |
| Antigravity cache semantics? | Unresolved; capture a nonzero-cache sample first | `input - cache_read`, by analogy to the Gemini API | Gemini's formula is the likely answer; Codex's gate is the right process. Merge: the hypothesis is Gemini's, the evidence requirement is Codex's, and the DoD names the evidence, not the formula. |
| Smallest live proof? | Direct turn plus two concurrent nested branches (Codex, Antigravity), then a Haiku turn | Group with two concurrent attempts (Codex, Antigravity) | Codex, because the direct turn tests "direct work in a parent" and the Haiku turn covers the third adapter. Gemini's is the same run minus those two. |

## Merge recommendation

Use the Codex draft as the skeleton: its architecture sections, its
availability model, its Phase 3 example list, its DoD, and its risk table.
Into it, bring from Gemini: the ASCII mockup as the page's design, the
normalization table as the accounting contract, `TokenBar` as one of the
renderers, and the proof run's size (then add Codex's direct turn and Haiku
turn). Then make these changes to the result:

1. State that all three adapters share the all-or-nothing `available`
   rule, and decide what happens to the `tokensAvailable` key: delete it
   or redefine it. Do not leave it meaning "all five known" while the page
   ignores it.
2. Turn Phase 1's Antigravity evidence into a spike that runs beside
   Phase 2, with Gemini's formula as the stated hypothesis and Codex's
   sample as the gate.
3. Resolve the three "if required" decisions now: check the saved session
   logs for a sidecar arriving after the accounting one; forward the
   record time through `observeLifecycle` (it is one field); and say
   whether the negative-derived-value rule changes the Codex and Claude
   clamps.
4. Cut from Phase 5: Playwright geometry assertions and the SSR test
   extension. Keep the checkpoint-only restart, the reconciliation table,
   and the record of which claims were live.
5. Cut from Phase 4: keyboard and narrow-screen items become notes. Cut
   Gemini's "By Model" and "Flat Transcript" toggles; the existing
   transcript is the message drill target.
6. Add a Pyramid Index, which Gemini lacks and the template requires.
7. Keep the shape of the observation types close to Gemini's `ScopeInfo`
   and `TurnInfo` (they are the right fields) but with the task as
   `json.RawMessage`, times as one unit, and no `TokenTotals`,
   `ModelUsage`, or `ScopeUsageRollup` in Go.
