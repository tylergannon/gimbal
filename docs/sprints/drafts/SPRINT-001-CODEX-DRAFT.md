# Sprint 001: Token usage by scope

Status: Codex draft, 2026-09-12. Prepared from `SPRINT-001-INTENT.md`.

## Pyramid Index

- L0: Make token spending inspectable from the whole run through scopes, turns, prompts, and messages, on a wall-clock timeline that works live and from the finished checkpoint.
- L1:
  - Persist scope and turn lifecycle metadata beside the existing transcript projections.
  - Normalize adapter accounting into comparable fields, preserving missing information.
  - Derive usage once in the browser observation layer; reuse it for hierarchy, timeline, and message detail.
  - Demonstrate reconciliation with deterministic fixtures and a real nested, concurrent workflow on cheap models.
- L2:
  - [Overview](#overview): scope, project context, and retrieved prior art.
  - [Architecture](#architecture): ownership, accounting rules, and interaction model.
  - [Implementation Plan](#implementation-plan): ordered work and phase evidence.
  - [Definition of Done](#definition-of-done): observable acceptance and required checks.
  - [Open Questions](#open-questions): proposed decisions and the provider-semantic blocker.

## Overview

The run page should answer: which scope spent the tokens, on which model, when, and for which prompt? A person can start with the whole run, select a scope, inspect its turns, and open an individual assistant message. The same five token fields appear at every level: input new, cache read, cache write, output, and reasoning. A timeline exposes concurrent work without suggesting that summed child durations equal elapsed run time.

The current foundation is sufficient. `internal/observation` owns each live run's projections, sends a replacement snapshot followed by event/lifecycle frames, and writes one final `observation.json`. Assistant rows already contain model, tokens, and timestamps. The missing pieces are durable scope/turn metadata, field-aware accounting, aggregation, and the usage interface.

This sprint delivers those pieces. It does not add a public workflow primitive, change `TurnEnded.Tokens`, alter the OpenCode projection ports, introduce prices, or implement the rest of the historical graph roadmap. Dollar cost remains unavailable. No log replay service, periodic checkpoints, GitHub automation, static graph analysis, or new workflow wrapper is needed.

### Project and planning context

- Read against the current `go doc -all .`, `AGENTS.md`, `ephemeral/research/api/API.md`, `ephemeral/research/api/SPRINTS.md`, and `docs/definition-of-done.md`. Godoc defines the API; the research documents explain design and historical delivery order.
- There are no existing `docs/sprints/SPRINT-*.md` plans. This uses the requested sprint template and the historical plans' pattern of concrete behavior, affected files, and live proof.
- No chapter is selected, and `docs/chapters/` does not exist. This advances the timeline/token portion of historical Sprint 3 without declaring that historical sprint complete. This draft does not change sprint or chapter status.
- `docs/SEMANTIC-INDEX.md` says the index is not yet built. There is no entrypoint to traverse. Prior art was retrieved directly from the named files and targeted `rg` searches in `ephemeral/`.
- `API.md`'s Observability section places usage at the model-call grain and derives containment from scope keys. `ephemeral/research/issue-130/execution/DELIVERY.md` establishes the snapshot-first path and its limits. `ephemeral/worklog/202609112050-issue-130-frontend-cost.md` establishes keyed message invalidation with the reducer remaining the only transcript model.

### Findings that affect this plan

All three adapters already emit `accounting.fieldAvailability`. The immediate display defect is that `accounting()` in `web/src/lib/observation/index.ts` instead requires `tokensAvailable`. Use the existing sidecar rather than inventing another availability protocol.

The saved Antigravity run named in the intent contains two usage-bearing assistant rows and a separate tool-only assistant row with placeholder zeros and no accounting sidecar. Its usage-bearing rows have known input/output/cache-read/reasoning fields, unknown cache write, and `tokensAvailable: false`. Treating every projected assistant row as a measured model call would misrepresent this run.

That recording has `cache_read_tokens: 0` throughout. It proves the availability problem but cannot distinguish cache-inclusive from cache-exclusive input. Verify that semantic question before choosing the Antigravity subtraction formula.

## Use Cases

1. **Find expensive work while it runs.** The run summary shows totals by actual provider/model and token field. Expanding a scope shows its child scopes and directly owned turns; totals update as accounting arrives.
2. **Explain one task's spend.** Select a task scope, read the recorded task description and definition of done, then select a turn to see its exact prompt, output type, duration, outcome, and constituent messages.
3. **Inspect an individual model call.** Select an assistant message and see its observed token fields, actual model, timestamps, and existing transcript content. Unknown fields say unavailable; an observed zero displays zero.
4. **Compare concurrent work.** Select a scope and view its child scopes, all descendant turns, or all descendant messages on a common run-relative time axis. Overlapping intervals occupy separate rows. Token weight and timing remain distinct encodings.
5. **Inspect a finished run after restart.** Open the same page from a project containing only the run's final checkpoint. Hierarchy, prompts, token details, and timeline agree with the live final view.
6. **Understand incomplete accounting.** If a model omits cache write or reasoning, other known values remain visible. A parent shows a known subtotal with an explicit partial label when some contributing calls lack that field.
7. **Account for work directly in a parent.** A root-level planning turn or a turn in a scope with children is visible alongside those children. It cannot disappear from the breakdown or be charged twice.

## Architecture

### 1. Extend observation metadata, not the public API

Keep the existing `RunSnapshot` and invocation projections. Add:

| Surface | Data | Source |
|---|---|---|
| Run info | Start and end timestamps | `run_started`, `run_ended`; cancellation remains an outcome, not a substitute completion time |
| `scopes`, keyed by exact scope key | Key, parent key, recorded name, optional task snapshot, began/ended timestamps, error | `scope_began`, `scope_ended` and lifecycle placement |
| `turns`, keyed by existing turn ID | Scope/session/turn placement, prompt, output type, started/ended timestamps, duration, error, interrupted | `turn_started`, `turn_ended` |
| Existing invocation | Transcript snapshot and per-message provenance | Existing native event path |

Separate turn metadata is useful: a turn exists as soon as `turn_started` arrives, including when it fails before emitting a native message. There remains exactly one transcript projection per invocation. Join metadata to it using the existing turn ID; do not copy its transcript into the turn record.

`internal/observation` cannot import the root package because the root imports it. Extend the existing `observation.Lifecycle` handoff with the necessary internal metadata. The switch in `run.observeLifecycle` still understands the sealed lifecycle variants. Preserve the optional task as detached JSON rather than duplicating the public `Task` definition in the observation package. The browser gives that JSON a display shape matching the existing task fields.

Use the timestamp already stamped into the lifecycle record. Either pass the stamped record time through the private writer handoff or extract its timestamp from the already supplied JSON; do not call `time.Now()` again in the fold. Keep the published record unchanged. Retain turn duration as the recorded Go duration, with one explicit nanoseconds-to-milliseconds conversion for display; lifecycle timestamps and native millisecond timestamps must land on the same axis.

The root scope is key `""`; its displayed name is the run name, not the root `ScopeBegan` name `"."`. Immediate parent is the path with its final segment removed. Membership means exact equality or prefix followed by `/`, with the empty root containing everything. A plain string prefix would incorrectly include `attempt.10` inside `attempt.1`.

Fold and detach metadata under the existing store mutex. Extend `snapshotLocked`, browser `replace`/`snapshot`, and checkpoint initialization together. Preserve a snapshot and its ordered suffix as one observation cut. The browser fold must match Go's cancellation precedence: a later `run_ended` must not change cancelled to failed/completed. Keep that correction within the lifecycle work.

Older checkpoints may lack the new metadata. Empty maps and an unavailable timing/prompt display are enough to avoid crashing; do not replay their logs, fabricate lifecycle facts, migrate files, or add a compatibility service.

### 2. Establish comparable token fields at adapter boundaries

The normalized fields are disjoint categories when known:

| Field | Meaning |
|---|---|
| Input new | Input tokens excluding cache read and cache write |
| Cache read | Input tokens served from cache |
| Cache write | Input tokens charged to cache creation/write |
| Output | Output excluding separately counted reasoning |
| Reasoning | Separately reported reasoning/thinking tokens |

Use each adapter's existing `fieldAvailability` map (`input`, `cacheRead`, `cacheWrite`, `output`, `reasoning`) plus the projected numeric value. A placeholder number without field availability is not a measurement. Derived fields are available only if their required inputs are known. Keep `costAvailable: false` and raw provider accounting in the existing sidecar.

For Antigravity, inspect the installed emitter's usage construction or an authoritative provider contract, and obtain a live sample with nonzero cached input. Record the command/model/version and the raw fields. Subtract cache read from input only if its inclusion is established; subtract cache write only if its inclusion is established too. An absent cache-write field stays unavailable, even if the verified emitter contract allows new-input to be calculated independently of it. Do not import Codex's formula by analogy or infer cache semantics from a zero-cache sample.

Audit the existing Codex and Claude normalizers against the same definitions. Their formulas already exclude cache and reasoning in the current implementation; change them only where the field dependencies or captured provider evidence require it. Missing reasoning means exclusive output is unknown, not that reasoning was zero. Do not clamp an inconsistent negative derived value into an apparently measured zero; preserve raw evidence and mark that derived field unavailable.

Recognize Antigravity's separate tool execution rows using its existing native provenance and emitter behavior. They remain visible messages but are not additional model calls: token accounting is not applicable to those rows. An actual model-call row containing tools still counts. An actual model call with no usage remains unavailable. Keep this distinction in the shared usage extractor and cover it with the saved three-row pattern; do not change either projection port or introduce another identity system.

### 3. One browser usage calculation

Add a small `web/src/lib/observation/usage.ts` module used by `RunObservation` and all usage renderers. Go persists canonical inputs; it does not persist a second set of arithmetic totals. The same TypeScript calculation runs for initial/SSR snapshots, live state, and finished checkpoints.

For each invocation, traverse its canonical projected assistant messages once. Identify a row by turn ID plus canonical message ID. Use the row's `model.providerID` and `model.id` for grouping; never replace an observed model with a configured session alias. Missing model identity has an explicitly unavailable model bucket, with configured session metadata usable only as context.

For each field, retain the known sum, number of contributing model calls, and number with that field observed. This gives an additive representation of availability:

- All contributing calls observed: display the sum, including a measured zero.
- Some observed: display the known sum with `partial` and the observed/total call count.
- Calls exist but none observed: display `unavailable`.
- No model calls: display `no model calls`; additive identity is zero without claiming a provider reported zero.
- A running call with no accounting is pending, never a zero-token completed call.

These sums and counts combine identically at message, turn, scope, and run level. A scalar total or token weight uses the sum of the five disjoint fields, marked partial when any component is unknown. The five columns remain primary so the scalar never hides missing categories.

Reconciliation for every model and field is:

```text
turn = its model-call messages
scope = its directly owned turns + its immediate child scopes
run = root scope
```

Apply the same equations to observation counts. Never add `TurnEnded.Tokens` to message totals: it contains the same usage a second time and lacks model/availability ownership. Use it only as a proof cross-check. Turns are charged to their invocation placement, not the session's creation scope. Fork ancestry does not cause inherited conversation history to be billed again.

Start with a pure aggregation pass over canonical state, cached until usage-relevant changes occur. Rebuild on snapshot replacement, model/step/accounting changes, and structural lifecycle changes. Text-only deltas continue through existing keyed row revisions and need not rescan all historical tokens. This avoids a second mutable ledger of incremental numeric deltas. Snapshot replacement discards derived state; reconnecting cannot add old totals a second time. Preserve accounting when a later unrelated message sidecar arrives if that case occurs in the emitted event stream; handle it consistently in Go and browser provenance folding.

### 4. Hierarchy and timeline share a selection

`RunViewer.svelte` owns selected scope, level, and selected turn/message. It keeps its existing connection ownership. Add two focused renderers:

- `UsageTree.svelte`: run/model summary and expandable scope rows, with direct turns alongside child scopes. Each row shows the five fields by model. Selecting a scope exposes its task, if present.
- `UsageTimeline.svelte`: selected scope's immediate child scopes plus direct turns in Scope mode, all its descendant turns in Turn mode, or all its descendant messages in Message mode. Use stable ordering by start time and key/ID, separate rows for concurrent intervals, and the same selections as the tree.

The horizontal axis is elapsed wall clock from run start. Bar position and width encode the recorded interval; an adjacent token-weight indicator and numeric label encode observed token count. Keep weight comparable within the visible view and label partial weights. Missing timestamps are unavailable; zero-duration events get a visible marker. Scope/turn durations are observed spans, not sums of children. Native message spans describe the observed event interval, not a claim about provider compute latency.

While the run is active, open spans may extend to now using a local display clock. After run completion, freeze the axis at the run end. An open message with missing completion metadata remains visibly incomplete rather than receiving a fabricated completion time. Turning the level selector never changes totals.

Selecting a turn shows its exact prompt, output type, duration, error/interruption, and the existing `SessionTimeline` transcript. Selecting a message reveals/focuses its existing `MessageRow`, where the five fields use the same formatter as the totals. Keep prompts and task text plain text. Provide keyboard-operable controls, explicit labels, and numeric values; color and hover alone are insufficient for inspection.

Keep these components within the existing observation feature. No chart dependency or general-purpose graph framework is necessary. The routes' transported snapshot already carries open JSON; no additional endpoint or transport type generation is needed for these fields.

## Implementation Plan

### Phase 1: Settle accounting semantics and fixtures

**Files:** `agy/events.go`, `agy/events_test.go`, `codex/events.go`, `codex/events_test.go`, `claude/events.go`, `claude/events_test.go`; evidence under `ephemeral/attest/token-usage-by-scope/`.

- [ ] Capture the Antigravity cache contract and a cheap live nonzero-cache example before changing its input formula. Preserve the raw observation and explain what the sample establishes.
- [ ] Correct Antigravity normalization and field dependencies according to that evidence. Keep unknown cache write unknown.
- [ ] Cover full usage, missing cache write, missing reasoning, measured zero, unavailable usage, and inconsistent subtraction inputs in focused adapter tests. Update Codex/Claude only as those cases require.
- [ ] Preserve the real Antigravity model/tool/model pattern as a compact accounting fixture. Identify tool execution rows separately from model calls using existing provenance.

**Phase evidence:** Named raw fields reconcile to the normalized five categories; missing fields cannot become known zero. If cache semantics remain unresolved, that accounting claim stays open while metadata and UI work proceed independently.

### Phase 2: Persist scope and turn observations

**Files:** `run.go`, optionally `event_persistence.go` for the stamped-time handoff; `internal/observation/snapshot.go`, `store.go`, `checkpoint.go`, `store_test.go`; `events_test.go` or `gimble_test.go`; browser `index.ts` and `index.test.ts`.

- [ ] Add scope/turn maps and run timestamps; extend the lifecycle handoff without a root import cycle.
- [ ] Capture optional task JSON, exact turn prompts, output types, durations, terminal errors, and interruption. Preserve root identity and segment-safe parent relationships.
- [ ] Create turn metadata at `turn_started`, before native events. Join projections later by existing ID.
- [ ] Deep-copy newly retained mutable fields in detached snapshots and include them in the final atomic checkpoint.
- [ ] Mirror the fold, snapshot replacement, serialization, and cancellation precedence in TypeScript.
- [ ] Exercise a real `gimble.Run` with a fake adapter: root work, nested scopes, a structured task scope, concurrent siblings, repeated ordinal names, and a turn that ends without a native message.
- [ ] Feed the resulting lifecycle/frame fixture through the browser fold and compare the final metadata to the Go checkpoint. Verify a late subscriber starts with the already folded scope/turn metadata.

**Phase evidence:** The checkpoint and snapshot-plus-suffix contain the same metadata, and the finished reader needs no lifecycle/session log. Existing subscriber isolation still holds.

### Phase 3: Derive usage with explicit availability

**Files:** new `web/src/lib/observation/usage.ts`; existing `index.ts`, `index.test.ts`; `internal/observation/identity.go` and its tests only if preserving accounting across later sidecars is required.

- [ ] Implement the field extractor, model grouping, sums/counts, and scope/turn/message selection functions.
- [ ] Use exact placement and canonical row identity, count each model call once, and exclude recognized tool execution rows from model-call denominators.
- [ ] Replace `accounting()`'s all-or-nothing token decision with the shared field-aware formatter. Keep cost separate and unavailable.
- [ ] Invalidate derived usage on relevant frames and fully replace it on a new snapshot; preserve existing per-message text invalidation.
- [ ] Add arithmetic tests to the existing `index.test.ts`, which `web/package.json` already runs. Avoid creating an unexecuted test file under the current explicit test list.

**Required examples:** Two models in one scope; one session used across scopes; root direct work; `attempt.1` versus `attempt.10`; multiple calls per turn; duplicate/replaced rows; same provider IDs in different turns; known-zero, partial, and unavailable fields; a running/interrupted call; a tool-only row; a reconnect replacement. Verify sums and availability counts at each level with independently specified expected values.

**Phase evidence:** Parent breakdowns reconcile, reconnects do not inflate totals, and known Antigravity fields remain visible despite missing cache write.

### Phase 4: Build the run usage interface

**Files:** `RunViewer.svelte`, new `UsageTree.svelte` and `UsageTimeline.svelte`, `SessionTimeline.svelte`, `MessageRow.svelte`, `web/observation_ssr_test.go`.

- [ ] Render run and scope usage by model, with direct turns explicitly included in the breakdown.
- [ ] Add shared scope/turn/message selection and the three timeline levels.
- [ ] Render recorded intervals, concurrent rows, visible token weights, partial labels, and live open spans.
- [ ] Show task and prompt detail and reuse transcript rows for individual-message inspection.
- [ ] Extend SSR coverage to confirm the transported checkpoint produces the same visible metadata and usage before hydration.
- [ ] Verify keyboard selection, numeric labels, and narrow-screen scrolling for the usage table/timeline.

**Phase evidence:** The production page can be inspected from run to message with unchanged totals at each selection. A text delta still updates its existing row, and later accounting updates its totals without reload.

### Phase 5: Demonstrate the whole path

**Files:** a bounded ordinary-Go proof program at `ephemeral/attest/token-usage-by-scope/main.go`; focused browser coverage in `e2e/features/token-usage.feature` and `e2e/steps/token-usage.ts`; evidence and exact invocation notes beside the proof program. Reuse the existing Playwright configuration and production `web.NewRuntime` pattern.

- [ ] Give the proof program a deterministic fake-adapter mode and a live mode, with a project directory/port chosen by the caller. Keep the workflow visible as `Scope`, `Group.Go`, `Wait`, `NewSession`, and `Generate`; no workflow library wrapper.
- [ ] Deterministic mode exercises exact totals, task metadata, partial fields, delayed accounting, errors/interruption, and reconnect. Browser assertions compare visible cells and timeline geometry against expected values, not merely page presence.
- [ ] Live mode runs an outer scope containing one direct turn and a group with two sibling branches, with a deeper scope in one branch. Use Codex `gpt-5.6-luna` and Antigravity `gemini-3.8-flash`; use separate workdirs for concurrent sessions. Add one short Claude Haiku turn to demonstrate all three adapters' field display. Record the exact actual model IDs, including resolved variants.
- [ ] Use small read-only prompts and a short local read tool where needed to produce multiple model calls. Confirm overlap from recorded intervals, not just the use of `Group`; repeat only the missing proof case if the run did not show it. Provider omissions remain unavailable.
- [ ] Inspect the live production page and capture the run summary, an expanded scope with concurrent timeline rows, a turn prompt, and one message's accounting. Wait for the asserted content/totals to render before capturing screenshots.
- [ ] Independently reconcile normalized `session.step.ended` records to the displayed per-model five-field sums. Compare each turn's `TurnEnded.Tokens` list with its step values, without counting that list again. Availability/model ownership comes from session records and provenance, not the opaque list.
- [ ] Copy only `runs/<id>/observation.json` into a separate proof project, restart the production server against it, and repeat browser assertions. Keep the original logs intact as evidence. Confirm no request or server read needs the source logs.
- [ ] Record which claims the real run demonstrated and which edge cases used deterministic evidence. Run the required checks below and have the validating agent assess the claim evidence against this sprint's requirements.

**Phase evidence:** Inspectable live and checkpoint-only screenshots, exact run/model IDs, raw logs, a reconciliation table, and browser assertions demonstrate the requested behavior. Passing builds alone do not finish the sprint.

## Files Summary

| Files | Change |
|---|---|
| `agy/events.go`, `agy/events_test.go` | Evidence-based cached/new normalization and availability cases |
| `codex/events.go`, `claude/events.go`, their `events_test.go` files | Verify common field meaning; make only accounting corrections demonstrated necessary |
| `run.go`; `event_persistence.go` if needed | Forward scope/turn metadata and original lifecycle time |
| `internal/observation/snapshot.go`, `store.go`, `checkpoint.go`, `store_test.go` | Fold, detach, persist, and restore scope/turn/run timing data |
| `internal/observation/identity.go` if needed | Retain accounting through later unrelated sidecar updates |
| `events_test.go` or `gimble_test.go` | Fake-adapter integration through actual lifecycle production |
| `web/src/lib/observation/index.ts`, `index.test.ts` | Matching lifecycle fold, replacement, invalidation, and usage tests |
| `web/src/lib/observation/usage.ts` | Single field/availability extraction and aggregation implementation |
| `web/src/lib/observation/RunViewer.svelte` | Usage selection and run-level integration |
| `web/src/lib/observation/UsageTree.svelte`, `UsageTimeline.svelte` | Hierarchy/totals and wall-clock timeline |
| `web/src/lib/observation/SessionTimeline.svelte`, `MessageRow.svelte` | Prompt/message drill-down and shared accounting display |
| `web/observation_ssr_test.go` | Production SSR proof for new snapshot fields and usage |
| `e2e/features/token-usage.feature`, `e2e/steps/token-usage.ts` | Production browser behavior and checkpoint-only inspection |
| `ephemeral/attest/token-usage-by-scope/` | Small proof workflow, recorded provider semantics, live evidence, reconciliation, and execution notes |

`events.go`, `internal/sessionstate`, and `web/src/lib/sessionstate` need no changes. The existing snapshot route, hooks, HTTP/SSE handlers, and registry remain the transport/read path. Generated files change only through the build's generator, never by hand.

## Definition of Done

### Observable acceptance

- [ ] **Whole run and every scope:** The production page shows usage grouped by observed model with all five fields. Direct turns plus immediate child scopes reconcile to their parent, including observation counts for partial fields.
- [ ] **Truthful accounting:** Known zeros are zero; omitted fields are unavailable; partial aggregates show their known subtotal and incomplete coverage. Separate tool execution rows do not create extra billable model calls.
- [ ] **Adapter comparison:** Antigravity's cached/new calculation has explicit source evidence and a nonzero-cache live observation. Codex, Claude, and Antigravity use the same meanings where fields are observable; missing fields are not fabricated to make them comparable.
- [ ] **Task and prompt inspection:** A scope's recorded task and each turn's exact prompt/output type/duration/outcome are inspectable, including a turn without native messages. Selection reaches the individual assistant row and its own usage.
- [ ] **Timeline:** Scope, turn, and message views share a wall-clock axis; actual concurrent intervals overlap visually; token weight is visible and distinct from duration. Live/open and unavailable timing are represented honestly.
- [ ] **Live correctness:** Incremental accounting updates visible totals; text streaming remains responsive; snapshot replacement/reconnect preserves exactly the final totals and metadata.
- [ ] **Checkpoint sufficiency:** After restarting against a project with only the checkpoint, all new-run usage views, prompts, task detail, and recorded intervals match the live final state. No source logs are needed to render the page.
- [ ] **Real proof:** A nested concurrent run on cheap models has named run/model IDs, inspected production-page screenshots, and per-model/per-field reconciliation to raw session accounting. Deterministic fixtures cover exact arithmetic and unavailable/interrupted cases separately.
- [ ] **Scope discipline:** No new root exported name, workflow wrapper, projection-port edit, cost table, log replay dependency, or GitHub automation. The validating agent assesses requested behavior and legitimate evidence; unrelated preferences do not become extra requirements.

### Required checks during implementation

Run `just build` first because Go web tests embed the built app. Then run the checks sequentially so generation/build output does not race tests:

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
cd web && pnpm test
cd web && pnpm run check
```

Run the new browser scenarios through the existing `just e2e` recipe with `BASE_URL` pointing at the proof server. Record the exact launch/test commands and artifact locations. The planning task itself does not run models, implement this work, commit, or mark any acceptance item complete.

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Antigravity input/cache semantics remain ambiguous | Resolve against emitter/provider evidence and a nonzero-cache run before selecting the formula; leave the affected acceptance claim open if evidence is unavailable |
| Missing cache/reasoning is silently treated as zero | Use field availability and additive observation counts; test explicit zero separately from omission |
| Tool-only assistant projections distort totals or coverage | Recognize the existing Antigravity provenance pattern; test a model call containing tools separately from a standalone tool execution row |
| Parent totals omit direct work or match the wrong ordinal | Use segment boundaries and explicit direct turns plus child scopes, with `attempt.1`/`attempt.10` tests |
| Snapshot/fold implementations drift | Feed actual Go-produced metadata into the browser fold and compare final snapshots; test cancellation precedence and immutable task snapshots |
| Reconnects or late sidecars lose or double-count accounting | Derive from current canonical rows; replace derived state on snapshots; test subsequent provenance updates |
| Every text delta triggers historical usage traversal | Cache derived usage until accounting/structure changes and retain keyed transcript revisions |
| Timeline implies compute time or additive elapsed durations | Label observed intervals; preserve wall-clock widths, separate weight encoding, and explicit missing times |
| Live caching, reasoning, or overlap does not appear | Record the limitation and use targeted follow-up for the missing live requirement; use deterministic tests for field/rendering edge cases without relabeling them live proof |
| A new proof harness expands into infrastructure | Keep one ordinary-Go workflow and focused scenarios on existing runtime/Playwright surfaces; no new proof service or framework |

## Dependencies

- Existing lifecycle emission, canonical message placement, native sidecars, observation store/registry, and snapshot transport.
- Existing Go/Polytype/skgo and Svelte build tools; no new production dependency is planned.
- Installed/authenticated Codex, Claude, and `agy` for implementation attestation. Use `gpt-5.6-luna`, Claude Haiku, and `gemini-3.8-flash`; record actual resolved IDs. This does not prescribe the models used to implement the sprint.
- Access to Antigravity emitter/provider accounting semantics and a cache-bearing live sample. This blocks final confirmation of the cached/new claim, not development of metadata and usage rendering.
- A production build and browser runner for the proof, with separate workdirs for concurrent sessions and a retained project directory for logs/checkpoint evidence.
- No chapter, semantic-index build, schema redesign, external pricing feed, or previously deferred graph feature is a prerequisite.

## Open Questions

1. **Where do totals live?** Proposed resolution: one TypeScript aggregation module over checkpointable canonical inputs. No Go arithmetic copy or persisted rollup. Revisit only if an actual non-browser consumer needs totals.
2. **How does timeline navigation work?** Proposed resolution: one scope selection shared with the tree, plus Scope/Turn/Message levels and detail selection. Bar time and token weight have separate encodings.
3. **Should `TurnEnded.Tokens` become typed?** Proposed resolution: leave `[]JSONText` unchanged. The projection plus sidecar already supplies the typed fields and ownership needed here; the opaque list is only a validation cross-check.
4. **Is pricing included?** Proposed resolution: no. Show tokens and unavailable cost. A price table would require an independent contract for rates, model variants, and billing semantics.
5. **What exactly does Antigravity include in input/output, and does absent cache write mean unreported or inapplicable?** Unresolved by the supplied recording. Phase 1 must establish this from the emitter/provider contract and live cached usage; absence alone proves neither zero nor exclusion.
6. **What is the smallest real proof?** Proposed resolution: a direct turn plus two concurrent nested branches using Codex and Antigravity, followed by a short Claude Haiku turn. Exact task metadata and difficult unavailable/error cases come from the fake-adapter integration; the live workflow demonstrates real accounting, hierarchy, overlap, and page inspection.

These are draft decisions for synthesis. Only the provider-semantic question requires new evidence before its dependent implementation can be finalized.
