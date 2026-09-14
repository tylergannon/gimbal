# Beta implementation handoff and sequence

Prepared 2026-09-14 from GitHub issue listings and this session's final decisions. Updated after the full open-backlog audit: see beta-bug-triage.md for ten small bug/cleanup/coverage issues added to Beta and their parallel owners. This handoff supersedes earlier permissive lint/extraction recommendations. It is a delivery sequence, not a request to implement every research suggestion.

## Decisions implementers must receive

1. **Make software easy to write.** Static comprehensibility is the chosen initial simplicity rule. For now a lint-clean workflow must have completely recoverable possible structure. Unknown workflow operations/structural relationships fail the gate. We can relax restrictions when a real workflow gives a reason. This does not require predicting runtime inputs, branch outcomes, iteration counts, prompts, or command arguments.
2. **Constant context keys are mandatory in lint.** Set and SetJSON require Go constant string expressions. Dynamic values are fine. The existing builtin sprint must fail at its formatted `repository check %d` key. First demonstrate that failure without an exemption; then make a separately visible workflow repair using explicit vet/test calls and constant result keys before the final Beta gate. Preserve the failing source shape as a regression fixture. Do not “repair” it by reusing one constant key repeatedly in the same scope.
3. **Dynamic worker dispatch is a hard lint error.** Replace a worker map/function lookup with explicit branches calling named workers. Direct helpers and recognized Gimble callbacks remain valid. Both authoring restrictions are lint/gate behavior; Go compilation and direct execution are unchanged.
4. **Both #162 and #201 belong in Beta.** The graph must meaningfully describe the builtin workflow, not leave its important structure unresolved. If that proves to require speculative analysis with uncertain success, raise that concrete blocker and reconsider extraction; do not turn Beta into a compiler-research project or quietly weaken acceptance.
5. **The graph is a hierarchy with typed relations.** Scope containment, control flow, session ownership, execution scope, and supervision are distinct. Parent-owned sessions can run turns in child scopes. Commands and nested supervision must be represented. A loop is one source template with runtime instances, not a predicted number of tasks.
6. **The UI's main surface is a map, with linked timeline and inspector.** Hierarchy remains useful navigation. Selecting a past failed task must preserve the current run's actual state. Preserve instance selection while changing views. Program structure is not observed execution; source discovery is not command telemetry.
7. **No new speculative bans.** Workflow recursion, broader concurrency checks, and other proposed lint rules are not all approved merely because they are in a research table. Keep the accepted rules separate from candidates.

Read these before the historical reports: [constant-key rule](constant-context-key-rule.md), [dynamic worker rule](dynamic-worker-lint-rule.md), [current linter issue](beta-issue-162.md), [current extraction issue](beta-issue-201.md), and [UI brief](ui-design-brief.md). The UI brief's current-policy notice supersedes older permissive lines retained inside it. The interactive design study is illustrative, not implemented extraction or a runtime fixture.

## Current milestone inventory

The first listing contained 11 open issues: #110, #117, #120, #126, #130, #135, #162, #170, #172, #173, #201. The subsequent full-backlog audit added #116, #154, #174, #192, #193, #194, #195, #196, #197, and #199, for 21 open Beta issues. The original REST counter briefly disagreed with the enumerated list; use refreshed issue records. No open PRs were returned during the initial handoff. These are backlog states, not a claim that every issue has no implementation yet.

Local snapshots: `beta-handoff-milestone.json`, `beta-handoff-issues.json`, `beta-handoff-open-prs.json`, and one `beta-issue-N.md` per open issue. Copy the relevant issue files and this handoff into an implementer's worktree and give absolute local paths. The research branch is `codex/workflow-graph-research`; it must be available to implementers instead of assuming these files already exist on main.

Closed prerequisites already in Beta include #169 (finished-run log serving), #159 (Set runtime contract), #175/#176 (cancellation and live registry), #177 (workflow-writing material), and #178 (loop practice). Do not dispatch them again merely because an older issue still lists them as future work.

## Start: a short coordination pass, not another planning project

- Each owner checks their issue against current main and existing evidence. Some issue bodies describe older code: #135 still assumes an app-server process per turn, #117 describes an older event taxonomy, and #173 carries historical prerequisites and an outdated Set example. Preserve requested behavior, use the current contracts, and avoid rebuilding something already present.
- The static-analysis and UI owners agree on one example graph's semantic fields: source site identity, region parent/kind, operation kind, typed edges, source version, and separate runtime identities. The runtime/observation owners agree on the existing event/snapshot identities. Use a small concrete example; do not freeze a grand schema or add a separate abstraction layer.
- Reconcile #173's presentation instructions with this session before dispatching UI implementation. Its timing/accounting/live-replay requirements still apply. Its prescribed tree rows are not the complete new UI design. Use them as the timeline/outline where useful, while the map and inspector provide workflow shape.

## Parallel lanes

| Owner/lane | Work and internal order | Can proceed independently of | Must coordinate with |
| --- | --- | --- | --- |
| Static analysis | #162 and #201 together under one owner. Deliver constant-key/dynamic-worker diagnostics, prove the current sprint fails, then extract a real builtin shape through its direct helpers. Make the explicit constant-key sprint repair a visible follow-up, retaining the negative fixture. | Price table, adapter event completion, UI rendering | UI owner on manifest shape; avoid separate competing scope/call recognizers |
| Native adapters | #117 and #135 under one owner, or split by provider with explicit file ownership. Audit current coverage, fix missing native events and resumed/forked tool attribution, demonstrate the browser path. | Extractor and price table | Observation owner on normalized event identities and shapes; supervision owner where live proofs expose adapter defects |
| Runtime lifecycle and supervision | First #126's focused completion/join review; then #110's failed-look record if still missing; then #120's worker/supervisor/nested-supervisor live proof. Fix concrete defects found. | Static graph implementation and price table; much can run before new UI | Adapter lane for relevant live failures; observation owner for recorded look outcomes |
| Observation and UI | #130 snapshot/stream continuity first where incomplete; #173 fold/timing/accounting and UI on that foundation. Build map/inspector against the small agreed shape while the extractor develops. Integrate actual graph output early. | Full adapter coverage and finished extractor during scaffolding; prices can initially be absent | Own Go/browser reducers coherently; accept actual events and generated manifest before claiming delivery |
| Prices | #172 deterministic embedded table, lookup, refresh behavior and release automation | All implementation lanes above | #173 cost display; complete price evidence before final priced-cost acceptance |
| Product promises | #170 align the existing constitution with Group, current API, and final authoring/UI decisions | Can be drafted alongside implementation | Reconcile with demonstrated behavior at the end; existing issue still reserves final location for Tyler |

With a smaller team, combine the adapter and supervision lanes. Keep one owner over #130/#173 because they change the same observation and frontend state machinery. Likewise keep #117/#135 coordinated, and #162/#201 coordinated. More agents should not mean several unrelated edits to the same reducers or analyzer plumbing.

Add the small fixes to these same lanes: UI gets #193/#197/#199; adapters and usage coordinate #195; lifecycle/supervision gets #192/#194/#196 and the current regression coverage from #116. #154's proof-log preservation and #174's docs-site move can run independently. Finish isolated small repairs while the larger work proceeds. See beta-bug-triage.md for effort caveats and the reasons six other open items remain outside Beta.

## Integration sequence

1. **First useful analysis result:** show the named constant-key failure on the actual sprint and produce a real program graph for its workflow structure. The known key violation must not be hidden. Resolve scope/session arguments through the actual helpers; inspect the result against source. Repair the builtin keys explicitly as a follow-up so the final workflow passes the gate.
2. **First useful run view:** #130's snapshot-to-stream boundary and #173's scope/turn fold render actual live and saved-run evidence. Parent-owned/child-executed turns must be placed and counted correctly. Adapter improvements feed this continuously; do not wait for all providers before exercising one production path.
3. **Connect shape and observation:** use the real manifest in the map, connect identities supported by actual records, retain selection across map/timeline/inspector, and show both declared structure and observed instances truthfully. Exact correlation is an instrumentation question where records lack identity, not an excuse to guess. Do not infer execution from source discovery.
4. **Complete commands, supervision, prices, and operator actions in that view:** show an actual command result and an honestly labeled interval; show attachments, looks, failures, and steering outcomes from records; connect #172 prices; exercise the existing steer/cancel runtime facilities through the actual page.
5. **One integrated Beta run:** use a Gimble workflow to build a real Gimble feature while Tyler watches and steers from the production page. Show the program graph, useful live activity, command/supervisor evidence, correct usage/time/cost, and finished-run replay. Use cheap models for attestation and record configured/resolved model IDs. A separate validator checks the claimed behavior under `docs/definition-of-done.md`; presentation preferences must not create unrelated gates.

Do not enable a repository-wide mandatory lint gate and then suppress the known builtin error to get green. Sequence the demonstrated negative case, explicit builtin repair, and gate activation so the final integrated branch is clean. The original source shape remains the negative regression fixture.

## Backlog coordination still needed before dispatch

There is no explicit owner in the current issue bodies for the whole graph-powered UI integration: #201 excludes UI work, while #173 describes the earlier usage tree. The recommended home is a concise acceptance update to #173 covering the map/inspector integration, command representation/observation, and the already-implemented runtime controls' page path. This is an integration responsibility, not a request for a second UI implementation.

Command observation needs an explicit boundary where workflow code uses ordinary os/exec. Start with the concrete builtin command and a named scope/recorded result if sufficient. Do not create a high-level command wrapper just for the graph, and do not label enclosing-scope time as exact process time. If existing fields cannot express the promised behavior, assign that small runtime change to the lifecycle owner before the final UI proof.

The final watched self-hosting run is a milestone acceptance responsibility even though it lacks its own current open issue. Name its owner when dispatching. #172's automated tagging has a real release side effect: follow that issue's required baseline/tag decision and proof; do not let a routine data-refresh implementation accidentally become the overall Beta release decision.

## Stop conditions and cost discipline

Use the issue's actual behavioral proof, not a new validation framework. Close stale issues on verified evidence rather than rewriting already-shipped behavior. Raise a concrete unsupported workflow case if extraction crosses into uncertain analysis; do not extrapolate hypothetical worst cases into a large project. Prefer one working graph and one useful live view quickly, then integrate the remaining required paths. Extra candidate lint rules can wait until real authoring shows they save time.
