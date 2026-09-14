# Sprint 001: Codex critique of Claude and Gemini drafts

Reviewed against [the intent](SPRINT-001-INTENT.md), the current public API (`go doc -all .`), the API design record and historical sprint sequence, and [the repository definition of done](../../definition-of-done.md). Code inspection covered lifecycle persistence, observation snapshots and reducers, adapter accounting, the current page, and the recorded Antigravity checkpoint named in the intent. This is a plan review; no implementation or new live attestation was performed.

## Recommendation

Use **Claude's draft as the base**, after correcting its accounting contract and proof. It identifies concrete integration points, keeps aggregation in one place, and gives the implementation a substantially clearer path to completion. Keep Gemini's explicit **child scopes plus direct turns** reconciliation formula, its handling of incomplete intervals, and its dependency-free visual treatment of token composition.

Neither draft is ready to execute unchanged. Claude contains a reproducible contradiction between its unknown-field rules and its proposed fixture assertion. Gemini duplicates aggregation without defining parity or even the persisted aggregate field, loses task metadata, and treats unverified provider semantics as settled.

Both make good scope decisions: retain the existing scope/turn identities, leave `TurnEnded.Tokens` opaque, preserve the projection port, and exclude monetary cost and price tables. Those boundaries should survive the merged plan.

## Claude draft

### Architectural soundness

**Strong foundation, with accounting corrections required.** Folding lifecycle structure in Go while defining usage once in TypeScript meets checkpoint-only rendering without introducing aggregate events or another Go accounting implementation. The proposed snapshot preserves prompts, task payloads, timestamps, and failure information. Using invocation placement rather than session ownership correctly handles a session created above the scope where its turns run.

The `(tokens, unknown)` value is the strongest accounting idea in either draft. It preserves a known subtotal when some calls omit a field and distinguishes that from a measured zero. The exact slash-boundary prefix rule also avoids confusing `attempt.1` with `attempt.10`.

Three contracts need correction:

1. **Output changes meaning between providers or calls.** “Provider accounting” defines output as excluding reasoning, then explicitly puts Claude's total output in that column when reasoning is absent. Summing that with visible-output-only values does not yield a comparable output total. Decide one meaning for the displayed field. If a visible/reasoning split is unknown, label the observed inclusive total separately or leave the split unavailable; do not silently substitute it. Apply the decision consistently to all three adapters.
2. **Completed assistant rows are not necessarily measured model calls.** The named Antigravity checkpoint contains three completed assistant rows: two with accounting and one tool row with no accounting. The draft's rule makes the latter unknown in every field. Consequently `cacheWrite.unknown` is **3**, whereas Phase 2 expects **2**. Specify whether a tool-only row is a non-accounting contribution or unknown usage, and how the existing provenance distinguishes it from a model call that omitted usage. Do not change the expected value merely to get the test green.
3. **The parent invariant needs direct usage.** The rendering design already includes directly placed turns, which is correct. The repeated claim that child scopes alone sum to their parent is incomplete. State `scope total = direct-turn total + immediate-child-scope totals`, per model and field, including unknown counts. A root turn costing 10 and a child scope costing 20 must yield 30, not 20 or 50.

### Completeness

The draft covers essentially the full requested vertical slice: Go fold, checkpoint, browser fold, one rollup, message footer, scope/turn/message drilldown, and intervals at every level. Keeping task JSON intact is preferable to inventing another assignment schema.

Remaining gaps are concrete:

- The shared `messageUsage` contract has no explicit case for a completed row with a sidecar but no accounting/tokens, or an interrupted turn whose last model call never receives `time.completed`. The UI must distinguish “no measured usage yet” from an established zero. Completed earlier calls must remain counted after cancellation.
- The per-turn cache must be owned by one observation and invalidated on snapshot replacement, not merely keyed by turn and a revision counter reset to zero. Existing `RunObservation.replace` deliberately invalidates message state; give the new cache an equally explicit replacement rule and a reconnect test.
- Scope task data is persisted but its display is not specified. The row design describes turn prompts; add an explicit way to inspect a task-bearing scope's assignment.
- A link to `data-message-id` is not an HTML fragment link by itself. Switching from Usage must mount the Transcript branch and then locate the intended message. Define the interaction and verify it in the browser.

### Phasing and ordering

The live accounting probe before adapter changes is the right order. Structural folding can proceed independently if a harness probe is unavailable; a provider uncertainty need only block that provider's normalization decision.

However, Phase 2 fixes expected fixture totals before Phase 3 settles the new normalization. Use stable synthetic accounting fixtures for the structural work, then derive the real-fixture expectations from the approved provider contract. The old checkpoint is useful evidence, but it contains pre-fix numbers.

Make the build prerequisite explicit where commands first depend on the embedded app. The Dependencies section says `just build` precedes whole-repository tests, while the phase sequence lists checks before that build. Resolve this in one ordered validation sequence.

### Risk coverage

Provider availability, scope of page work, and per-frame computation are sensibly identified. Two mitigations overclaim:

- **Cache evidence:** positive cache reads plus `total = input + output` are a useful consistency check, not sufficient proof of what the agy wrapper means by each field. Two similar prompts also do not guarantee a cache hit. Require supporting wrapper/provider evidence and observed nonzero-cache data; if the probe remains inconclusive, preserve the uncertainty rather than making subtraction mandatory.
- **Timing evidence:** common process timestamps avoid cross-host skew, but do not prove actual model execution duration. In the existing Antigravity checkpoint, substantial assistant usage appears on roughly 1 ms intervals. The plan should describe message bars as observed event intervals and keep such short spans inspectable. Do not equate their width with true inference time.

Add focused cases for failed/interrupted turns, missing end times, replacement snapshots, and a session generating inside a descendant scope. These test the changed behavior without broadening into recovery or telemetry redesign.

### Feasibility

This is a feasible bounded sprint once accounting semantics are fixed. The recursive usage row and plain interval bars keep the largest UI task contained. The per-turn cache may be useful, but its claimed `O(turns)` cost omits the changed turn's message scan and tree traversal; measure the chosen implementation before adding further caches.

The proposed tool-specific autofixer should not be the only route to completion when unavailable. The repository's actual build, Svelte checks, and browser inspection provide executable verification. The live example also needs ordinary runtime initialization/error handling when turned from a sketch into a program; it currently discards `NewRuntime`'s return and then references `runtime`.

### Definition of done

Claude's criterion-to-evidence mapping is better than Gemini's, especially its live/finished screenshots and restart exercise. It still allows false confidence:

- The reconciliation script checks only the root's numeric field sums. Moving usage to the wrong scope or model can preserve those sums. Check per-turn and per-model placement, each scope's direct-plus-children equation, and availability as well.
- `TurnEnded.Tokens` is assembled from the same normalized `session.step.ended` payloads in `session.go`; agreement proves recording/projection consistency, not provider normalization. It also lacks per-step model and availability metadata. Use raw session provenance for those claims, keeping the turn log as a supplementary check.
- Require observed cache-read values and verified normalization, not “second turn fresh input is smaller than first.” Conversation growth can change input independently of caching.
- Restarting against a directory that still contains every log does not by itself establish checkpoint sufficiency. Serve a copy containing only the completed checkpoint and inspect the same tree, prompts, values, and message details.
- SSR string presence proves transport, not that users can open the Usage tab, expand rows, or reach a message. Name the browser actions and visible outcomes.

The plain bars with adjacent numbers are a reasonable interpretation of token weight. Keep that explicit and demonstrate that a viewer can compare spend at the selected level; a new charting system is unnecessary.

**Keep from Claude:** one browser aggregation definition; known sums plus unknown counts; full task preservation; invocation-based placement; live accounting investigation; one expandable tree across all levels; explicit scope boundaries.

## Gemini draft

### Architectural soundness

**Useful UI and hierarchy ideas, but too much unresolved duplication.** The direct-plus-children equation is correct and clearer than Claude's prose. Retaining the existing projection and deriving placement from invocations also fits the repository.

The recommendation to compute authoritative Go totals and reactive TypeScript totals creates two implementations of accounting, justified by a new requirement that external programs read precomputed totals without JavaScript. The intent requires a checkpoint sufficient to render the page; it does not require this extra consumer contract. Prefer Claude's single definition. If both implementations remain, the plan must specify their synchronization and shared parity fixtures; calling them “identical” does not establish equivalence.

There is also a concrete persistence hole: the proposed `RunSnapshot` has no rollup field, and neither `ScopeInfo` nor `TurnInfo` contains usage. Defining `TokenTotals` and calculating `ScopeRollupLocked()` does not explain where the promised persisted totals go.

`TaskInfo{Summary, Goal}` does not match the actual `Task{Name, Description, DefinitionOfDone, Validation}` carried by `ScopeBegan`. No mapping is supplied, and the proposed shape loses acceptance and validation information. Preserve the existing task payload through the internal observation boundary without importing the root package back into its dependency.

### Completeness

All broad work areas are named, but several details necessary for correctness are missing:

- Boolean aggregate availability has no merge rule. With 100 known cache-write tokens from one call and another call missing the field, neither a bare 100 nor an unavailable dash communicates both facts. Adopt Claude's known subtotal plus unknown count, including across models.
- Changing `tokensAvailable` to `inputOK && outputOK` does not fix derived-field availability. Current Codex input requires a cache-write field, and current Claude output requires a reasoning field. The plan must decide and implement each field's semantics; changing the whole-message flag alone leaves these gaps intact.
- The unconditional five-field `total_tokens` equation is valid only when fields are disjoint and known. Missing reasoning/cache fields make the proposed `Total` and segmented bar potentially misleading. Define what total means under partial data before rendering it.
- Architecture mentions `message.model.id`, despite separately modeling provider identity. Use the provider/model pair consistently, with a defined missing-model bucket or clearly identified fallback.
- The timeline implementation specifies scope bars and turn details, but not actual turn and message intervals on the axis. Expanding a transcript is not the intent's message-level timeline. Include those row types explicitly.

### Phasing and ordering

Adapter → Go observation → browser → UI → proof is understandable. Its first step nevertheless implements Antigravity subtraction before resolving the intent's explicit open question about agy semantics. Add Claude's targeted probe ahead of that mutation, without blocking independent lifecycle work.

The two reducers need shared fixture comparisons as they are developed, not a final screenshot after divergent implementations have accumulated. Define timestamp serialization and `time.Duration` conversion before the browser fold: this draft uses RFC3339 Go times and nanosecond durations alongside message milliseconds without specifying the conversion boundary.

Move a small integration demonstration earlier: fold a nested scope and turn, persist it, then render it from a checkpoint. That resolves the producer-to-page boundary before four new UI components and three view modes are built.

### Risk coverage

Keep the incomplete-interval fallback and plain CSS/SVG approach. Unique message placement and high-frequency invalidation are also appropriate concerns.

The current mitigations lack enforcement. The browser increments its revision on every event, including text deltas; “coarse-grained revision bumps” is not a change described by the implementation tasks. Specify when usage recomputes and how a replacement snapshot resets it. Deduplicate within the existing invocation/message identity rather than assuming a global message ID is sufficient.

The adapter risk section focuses on future field changes while skipping the present uncertainty about cached input and output/reasoning semantics. It also lacks partial-data aggregation, cancelled turns, and reconnect parity. Those are more immediate risks than future CLI evolution.

### Feasibility

Go aggregation plus TypeScript aggregation, four UI components, and three top-level views enlarge the sprint without establishing additional requested outcomes. Collapse to one hierarchy/timeline and a model breakdown unless an actual interaction requires a separate view.

Two execution details need correction. `go.mod` currently requires **Go 1.27.1**, so “Go 1.24+” is not a usable prerequisite. `web/package.json` runs Bun reducer tests; `cd web && pnpm test` is not a Playwright/browser test. The repository has a separate `e2e/` path. Use the existing browser setup or explicit manual browser inspection and name the evidence it will produce.

The proposed simultaneous candidate that writes tests “for that snippet” depends on another concurrent candidate's output. Give both independent, already-available input so the proof actually exercises overlap rather than an unstated handoff.

### Definition of done

The durable structure and direct-plus-children invariant are good acceptance statements. The proof is materially thinner than the intent:

- A screenshot does not establish exact reconciliation; specify an executable comparison of observed values, placement, models, and unknown fields.
- Explicitly inspect the page during streaming, after completion, after reconnect, and from a checkpoint-only directory. The draft largely promises these modes without an exercise for each.
- A Codex/Antigravity run meets the two-harness workflow requirement, but cannot validate changed Claude semantics. Add a targeted cheap Haiku accounting sample or recorded evidence supporting that change; the main concurrent proof can remain two harnesses.
- Add observable turn/message timeline and task/prompt drilldown criteria, including terminal failure behavior.
- Include `just build` before tests that exercise the embedded application. “Zero warnings” is broader than the repository's requirements-based gate; unrelated pre-existing warnings should not become a reason to expand this sprint.

**Keep from Gemini:** the explicit direct-usage equation; interval fallback when ends are missing; dependency-free timeline rendering; token composition as a visual aid once partial-field semantics are sound; a small two-harness concurrent proof.

## Changes needed in the combined plan

1. Settle field meanings and the distinction between model-call usage, tool-only rows, and missing accounting. Preserve partial knowledge at every level. Record the unresolved provider facts before changing normalization.
2. Persist exact scope/turn lifecycle metadata and existing task content; define one timestamp representation at the observation boundary. Keep the rollup in one browser module.
3. Define the direct-turn-plus-child-scope invariant, provider/model identity, and replacement invalidation. Test real partial-data rows and a synthetic tree with direct turns at multiple levels.
4. Build one expandable scope/turn/message timeline with the five fields and model breakdown. Demonstrate task/prompt inspection and message navigation, including short and unfinished intervals.
5. Prove normalization against raw accounting separately from aggregation against normalized logs. Check visible per-model and per-scope values, not just the root sum. Demonstrate live updates and checkpoint-only replay with the named cheap models.

These are repairs to the requested accounting and inspection behavior. They do not require cost pricing, a public usage API, projection-port changes, recovery infrastructure, or additional workflow abstractions.
