# Sprint 001 adversarial review

Target: `SPRINT-001.md` plus the unapplied amendments. Outcome: **material findings remain**.

References below use `plan` for `docs/sprints/SPRINT-001.md`, `amendments` for `docs/sprints/drafts/SPRINT-001-AMENDMENTS.md`, and `saved` for `ephemeral/attest/issue-149/logs/runs/20260912-205306.issue-149/`. Upstream references are pinned to `c55ee2a8152603f04a409163bd3edf79c425fbd7`, as recorded in `internal/sessionstate/NOTICE.md:3-15` and `third_party/opencode/prepare.sh:4-17`.

1. **[P1] #155 does not define a working session-info owner or live seeding path.**

   **Claim under attack:** seed at `SessionCreated`; the browser needs no seed; the session card can read reducer info (`amendments:23-34`). **Verdict: broken.**

   `Store.Lifecycle` only records metadata (`internal/observation/store.go:138-140`). Projections are created later, **per turn**, in `invocationLocked(at)`, keyed by `at.Turn` (`store.go:224-234`). `SessionCreated` has no turn (`session.go:45`). Seeding a projection there neither identifies nor initializes the future turn projections.

   The proposed “id” is also ambiguous: lifecycle placement uses `planner.1`, while reducer events address `ses_` plus base64url of that ID (`session.go:283-290`). `Info["planner.1"]` will never receive those usage events; the reducer indexes `data.sessionID` (`internal/sessionstate/reduce.go:76-87`). Parent IDs need the same identity domain.

   Snapshot restoration is only half the browser path. `replace` restores supplied projections (`web/src/lib/observation/index.ts:47-57`), but the first event for a **new** turn constructs `emptySnapshot()` (`index.ts:70-75`). A read-only Bun probe against those classes, with one seeded old invocation and a usage event for a new invocation, produced:

   ```json
   {"restored":{"ses_x":{"id":"ses_x","title":"x"}},"newLive":{},"oldRunUsage":{"x":{"cost":1,"tokens":{"input":2,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}}}
   ```

   Deleting the last map removes the only working live total in that example. Existing browser tests conceal this gap by supplying seeded info initially (`web/src/lib/observation/index.test.ts:7-12,55-60`).

   Nor is the proposed five-field seed upstream's session record. [Upstream `session.ts:33-60`](https://github.com/anomalyco/opencode/blob/c55ee2a8152603f04a409163bd3edf79c425fbd7/packages/schema/src/session.ts#L33) requires `projectID`, `cost`, `tokens`, `time.created/updated`, and `location`; model is a `{id, providerID, variant?}` reference, not the lifecycle's string ([`model.ts:14-18`](https://github.com/anomalyco/opencode/blob/c55ee2a8152603f04a409163bd3edf79c425fbd7/packages/schema/src/model.ts#L14)). `run.go:155-158` currently drops the available workdir. Revert and permission updates tolerate absent initial fields (`reduce.go:117-120,283-294`); moving a thin seed loses the previous location/project (`reduce.go:361-384`). The probe's move produced `previous:{}`. `parentID` alone does not populate `Family`: upstream does that separately in [`data.ts:494-535,1316-1321`](https://github.com/anomalyco/opencode/blob/c55ee2a8152603f04a409163bd3edf79c425fbd7/packages/client/src/solid/data.ts#L494); neither port registers families on initialization.

   Finally, each invocation freezes a different session cumulative total. Saved Claude usage is `$0.0198747` in turn 1 and `$0.0249421` in turn 2 (`saved/sessions/claude.1/claude.1.jsonl:36,75`). Reading each card's own projection would give different “session totals”; today's cards all read the latest run-level value (`SessionTimeline.svelte:17-30`). A fresh turn also needs the prior total before its first usage event.

   **Smallest plan change:** specify the canonical key, the supported seed shape and its real field sources, when every invocation is seeded in **both** observation layers, and how cards select the latest session info across turns. Carry prior session info without sharing mutable turn transcripts. Explicitly decide whether family initialization belongs to #155; do not imply that `parentID` performs it. Test subscription before session creation, a second turn, and reconnect. Keep both reducer ports untouched.

2. **[P1] Moving scope totals to turns requires changing the supposedly unchanged query and tests.**

   **Claim under attack:** `ScopeUsage` becomes turn-based while `usage.remote.go` and the live-query tests remain unchanged (`plan:179-181,384-386`; `amendments:30-33,50-52`). **Verdict: broken.**

   The snapshot query's receiver is `RunInfo`, which has no access to the proposed sibling `RunSnapshot.Turns` (`internal/observation/usage.go:68-85`; `plan:146-150`). Initial, already-closed, and checkpoint responses all call `snapshot.Run.ScopeUsage(...)` (`web/src/routes/runs/[runID]/usage.remote.go:54,61,102`). Rewriting only the store method leaves those paths wrong; deleting `RunInfo.Usage` without moving the receiver fails compilation.

   `web/observation_live_test.go:92-100,140-151` emits only `session.usage.updated`, expecting `4321` then `8642`. There is no turn-start or step/report input to the proposed fold, so its expected scope usage becomes zero. `internal/observation/usage_test.go:45-65,98` has the same assumption. Browser run-map tests also need replacement (`index.test.ts:70-78`). The amendment identifies a risk but supplies no migration.

   Two sum implementations have an actual consumer **if the query stays**: `+page.svelte:4-12` still subscribes to it above the new tree. Repository call-site search found the remote query and tests consuming the Go methods, not a separate program. “Any program” is not justification for a new public API: the types are under `internal/observation`. The new header and existing header would duplicate the root display (`plan:222-226`).

   **Smallest plan change:** choose now. If retaining the query, move the snapshot calculation to the object holding `Turns`, update all three snapshot call sites and the store method, and migrate the stream test to turn/step/report inputs while preserving opening, update, close, and checkpoint behavior. Keep its aggregate wire shape; per-model maps need not become a new remote API. If deleting it, explicitly remove its page subscription, generated bindings through generation, and obsolete tests. Update `SessionTimeline` and its tests with finding 1; do not retain a second session-usage fold in the browser.

3. **[P1] The proposed accounting fold differs from the runtime, and its Codex fallback does not exist.**

   **Claim under attack:** the store follows `session.go`, replaces only non-empty reports, totals do not change, and a resumed Codex turn without steps gets usage from its report (`plan:159-162,417-432`). **Verdict: broken.**

   `session.go:182-185` replaces the fallback whenever `result.Usage != nil`, including an empty map. `modelUsage` then emits an empty array (`usage.go:59-64`). The proposed non-empty check would retain step usage against an authoritative `turn_ended.usage:[]`. The lifecycle already contains the **final** turn accounting, whether derived from steps or reported by the harness (`events.go:111-120`); the store need not infer its source again.

   Failed-step accounting requires both cost and tokens; a failed step with only one must not contribute (`session.go:226-251`). State that predicate explicitly in both folds. Add-with-zero-defaults is not equivalent for malformed/partial failed records.

   Codex returns `TurnResult` without `Usage` (`codex/codex.go:160-162`). `thread/tokenUsage/updated.last` fills an **already open step** (`codex/events.go:358-378`), which turn completion closes (`382-408`); it is not a turn report. If there truly are no steps, the runtime fallback is zero. The saved resumed turn does have a step: `saved/sessions/codex.1/codex.1.jsonl:121` carries input `4402`, output `59`, cache read `20224`; `saved/run.jsonl:24` repeats those numbers. Absence of `rawResponse/completed` must not be confused with absence of normalized steps.

   “Totals do not change” is false for cost: Claude's first step costs zero, then its report supplies `$0.0198747` (`saved/sessions/claude.1/claude.1.jsonl:34`; `saved/run.jsonl:12`). The session was configured as `haiku` (`run.jsonl:6`), while the report is keyed `claude-haiku-4-5-20251001`; replacement must remove the provisional key. The interface does not guarantee reported tokens equal step sums either. Session cumulative accounting adds step tokens plus report **cost only** (`session.go:168-178,258-265`), so its tokens are not generally the sum of authoritative turn reports.

   **Smallest plan change:** always set ended-turn usage from `TurnEnded.Usage`, including empty. Specify the failed-step predicate, replacement of model keys, and that final reports may change totals. Replace the Codex no-step fallback claim with its actual open-step fallback. Gate empty reports, partial/full failed-step usage, model renaming, and report/step differences with focused tests; validate session totals against `session.usage.updated`, not an assumed equality to turn-report tokens.

4. **[P2] The page specification has missing time, identity, and rendering behavior.**

   **Claim under attack:** every frame extends the axis; native attributes are transcript anchors; a plain set tracks expansion; old checkpoints can simply show what they have (`plan:163-170,222-239,429`). **Verdict: underspecified, with broken literal behaviors.**

   Text deltas advance `event.created` (`session.go:334-338`) but only mutate text in the projection (`web/src/lib/sessionstate/index.ts:77`). They do not update message `time.created/completed`, nor any proposed scope/turn time. Thus “latest timestamp the snapshot holds” can remain unchanged throughout streaming. Revision invalidation alone does not provide a new time. For the finished axis, root `scope_ended` precedes root-session cleanup and `run_ended` (`scope.go:79-96`; `run.go:87-94`); define whether the bar spans the root body or the run lifecycle.

   `MessageRow.svelte:17` has `data-message-id`, not `id`; an ordinary fragment link will not target it. The promised anchor needs an actual HTML ID or explicit scroll behavior. `SessionTimeline.svelte:32` supplies no turn identity to `MessageRow`, so account for the same canonical message occurring in multiple turn projections (`session.go:342-350`). The tree and retained transcript must not create duplicate HTML IDs.

   A native `Set` mutated with `.add/.delete` is not made reactive merely by `$state`; use replacement assignment or `SvelteSet` ([Svelte reactivity reference](https://svelte.dev/docs/svelte/svelte-reactivity#SvelteSet)). The existing viewer explicitly reads `revision` around mutable observation data (`RunViewer.svelte:9-23`). Carry that dependency through recursive rows. Namespace row IDs by kind and full placement; do not use display names or stripped ordinals. Root key is `""`, root name is `"."` (`scope.go:29,74`), ordinary scope names already include ordinals, but group names do not (`group.go:37`). Derive instance labels from keys.

   `loadCheckpoint` accepts missing fields and never replays logs (`checkpoint.go:58-79`; `registry.go:77-98`). The supplied `saved/observation.json:1` lacks even `scopes`, has no `turns`, and has empty `info` in every projection. Deleting `run.usage` loses its only session-level cost. Defaulting maps is structurally safe, but cannot produce a root interval or recover that cost. Missing data must not become an epoch-sized bar or a purported measured zero. Also, replacing the scope list removes its existing error/value/decision display (`RunViewer.svelte:59-64`); no replacement location is specified despite retaining those fields.

   **Smallest plan change:** define an axis updated from received frame timestamps, its initialization/terminal bounds, and missing/zero-duration behavior; define real transcript targets and reactive expansion. Preserve scope errors/values/decisions within the expanded scope, showing `task` only once. State explicitly what an old checkpoint cannot display; no reconstruction shim is required. Message intervals themselves are available (`internal/sessionstate/reduce.go:539,548,571`).

5. **[P2] Phase acceptance still depends on unavailable evidence and unnecessary machinery.**

   **Claim under attack:** the single proof demonstrates every DoD item and each phase can be independently validated under the amendments (`plan:261-297,301-357,392-410`; `amendments:9-19`). **Verdict: broken.**

   The proof sketch has no `Loop.Tasks` or task-bearing scope. Only the loop installs `taskKey` and the duplicate `task` value (`loop.go:113-125`; `scope.go:74-76`). Neither task drill-down nor #147's “task once” can be seen on this run. Adding `Scope(ctx,"task",...)` would still not create a task record. The haiku results are discarded and the judge receives literal `...` (`plan:325,334`); this demonstrates turn placement, not an actual comparison.

   DoD placement says usage counts in `research.1`, “not at the root” (`plan:392-394`), contradicting inclusive root rollup (`plan:108-112`). It must mean not a **direct root turn**. One screenshot cannot demonstrate cells rising or successful transcript navigation (`plan:341-344`); the validator must observe before/after values and click the target. DoD 6 is explicitly a fixture test, not live proof, and DoD 7 is a build gate. Keep those labels distinct.

   Phase 2's required real fixture comes from Phase 4 (`plan:275-286,354-355`), preventing sequential phase sign-off as written. The original persistent reconciliation script remains required in the file list, proof commands and DoD (`plan:302,345-351,381,395-398`), despite its removal by `amendments:15-19`. Apply that deletion throughout; copied logs and ordinary tests do not violate the amendment. Session logs must be found recursively: saved files are under `sessions/claude.1/claude.1.jsonl`, etc., not the literal `sessions/*.jsonl` glob. Sequence counters belong to separate writers (`event_persistence.go:38-69`); fixture replay must respect per-file order and cross-file turn boundaries, not globally sort by `seq`.

   Checkpoint-only serving requires `<project>/runs/<runID>/observation.json`, not a file at the project root (`registry.go:103-108`). Give the validator that layout, the ordinary server-start command, run URL, concrete query observation if retained, and required live observations in the plan. Those are missing handoff inputs, not reasons to build a proof framework. Include the missing runtime/workdir/preamble setup by referencing `ephemeral/attest/issue-149/main.go:38-60`; no new wrapper is needed.

   The prescribed turn cache (`plan:184-186,423-424`) has no demonstrated need or pass/fail performance criterion. Turn usage is already incrementally folded; a text delta does not change it. Start with the direct rollup. Likewise, record-time forwarding is correct but not necessary: `Lifecycle.Record` already contains the exact timestamp (`event_persistence.go:42-56,112-118`; `store.go:25-32`). Decoding its small time header avoids changing `writeLifecycle` and both callers. `TurnInfo` is also a new exported name (`plan:133`); under literal `AGENTS.md:27`, make the helper type private unless an actual cross-package caller needs to name it. The snapshot fields need no new root-package workflow API.

   **Smallest plan change:** add one real task dispatch to the live program, correct inclusive-root wording, move real-fixture assertions to after capture, and replace every script-based acceptance reference with the validator's explicit observations/arithmetic. Record validator evidence independently of implementer notes. Specify the checkpoint layout and setup, remove the unproven cache mandate, and leave optional time plumbing to the implementer. For #13, say its remaining context-window/quota/MCP criteria are declined, not implemented by #156; do not expand this sprint to build them.

Placement — **holds**: `ephemeral/attest/issue125/run.jsonl:6` creates `planner.1` at `scope:""`; lines 8-9 place `planner.1/turn.1` start/end at `scope:"without-plan.1"`; lines 23-26 and 40-41 repeat it. `session.go:124-136,195,203,215,221` uses the current turn context for lifecycle and native records; `run.go:141-143,185-191` preserves it. The native envelope carries no scope internally; its record/frame placement does (`events.go:173-180`).

#146 snapshot fields — **holds**: the proposed shape retains `ScopeInfo`'s existing fields/status semantics (`internal/observation/snapshot.go:67-86`; `store_test.go:330-357`); adding maps/times needs detachment and replacement defaults, not a checkpoint format migration.

Seed-capable constructor — **holds**: `sessionstate.New` ensures buckets and clones supplied `Info`; it does not discard it (`internal/sessionstate/state.go:71-76,121-128`).

Containment rule — **holds**: equality or slash-delimited prefix with a special empty root handles `attempt.1`/`attempt.10` and group children; compare full placement keys, not session/turn IDs or ordinal-stripped names (`ephemeral/research/api/API.md:773-802`; `internal/observation/usage.go:88-94`).

Proof API calls/topology — **holds**: `NewSession` and generic method `Generate[T]` match `session.go:41,67` and `gimble_test.go:233-250`; constructors exist at `codex/codex.go:47`, `claude/claude.go:45`, `agy/agy.go:78`; `Group.Go/Wait` creates concurrent child scopes (`group.go:30-66`), and the nested `review/verdict` scopes are two deep. The snippet still needs its declared setup before standalone compilation.

Named cheap tier — **holds** against saved evidence: `saved/run.jsonl:4,10,12,16,24` records Gemini flash-low, Luna, and resolved Claude Haiku; `ephemeral/research/issue-149/FINDINGS.md:7,20,43` identifies the same tier. This is not a fresh login/availability check.

#151 subtraction — **holds**: current `normalizeUsage` subtracts only cache read (`codex/events.go:560-577`), and both raw-response and notification paths use it (`344,377`). Apply the amended subtraction outside the ports; change the existing nonzero-cache-write expectation from `75` to `70` (`codex/events_test.go:24,51`) and its stale comment, rather than merely adding a new test. The saved step fixtures have cache write zero; reducers copy normalized numbers without subtracting again (`internal/sessionstate/reduce.go:552-553`; browser port `index.ts:141`).

#152 finest-grain cost — **holds**: saved Claude step costs are zero while its turn reports carry dollars (`saved/sessions/claude.1/claude.1.jsonl:34,73`; `saved/run.jsonl:12,21`). No proportional allocation is needed.

## What I could not verify

- Root `CLAUDE.md` is absent; only `ephemeral/legacy/CLAUDE.md` exists. I followed the supplied no-edit rule for both reducer ports.
- `go doc -all .` failed because its configured Go build-cache path was not writable. API signatures were checked in source and existing tests; the sketch was not compiled or run.
- No proof program, agent CLI, full test suite, or live browser/server run was executed. The Bun checks above exercised existing reducers only. Actual overlap, rising cells, task drill-down, checkpoint rendering, and current provider access remain unproven.
- The mandatory `svelte-autofixer` invocation is not defined in `web/package.json:8-12`; the plan names no provisioned tool or command. Its availability to implementers/validators is unverified.
