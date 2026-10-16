# Application generation during SKGO development

Status: Gimbal implementation authorized on 2026-10-16 by the user's request to retry the whole Gimbal upgrade and proof. This copy carries the previously reviewed design: three Sol 6.1 reviewers reported no remaining findings; SKGO and Polytype implementation has since merged and released as v0.35.0 and v1.10.0. The remaining work is Gimbal adoption and actual application proof. Design review is not implementation or runtime proof.

## Outcome and boundaries

`skgo dev` owns the watcher, development coordinator and application lifecycle. An optional application Go tooling process retains generator workers and chooses targeted generation from change information. Its application callback returns only `(generated bool, error)`. It owns no watcher, public listener or application supervisor.

- False permits SKGO's normal update path once generation is current and no prior full-rebuild obligation remains.
- True requires recapturing generated files and a full compile/restart. Compile the replacement before retiring the old application, then wait for old exit before replacement initialization.
- An error blocks application admission and preserves the healthy application. Partial writes and lost helper replies must not erase an outstanding full-rebuild obligation.

Do not run whole-application `go generate ./...` per edit, hot-patch generated output, exchange output manifests or Go object graphs, introduce a general task engine, or put Gimbal-specific behavior in SKGO. New development coordination and retained-worker wiring must be absent from production application targets. Preserve the existing shipping `gimbal generate` command.

## One optional tooling integration

An explicit optional `--generator-config PATH` on `skgo dev` reads application-owned configuration before the helper is compiled. The configuration names a Go main package and optionally one pre-build command as argv plus working directory. Paths resolve against the application root; commands are executed directly without a shell. The configured Go executable is used for helper builds and `go` prerequisite commands. Configuration changes invalidate helper selection and wake the coordinator.

Gimbal's registration is structurally:

```json
{
  "package": "./internal/generate/devgen",
  "beforeBuild": {
    "dir": "workflow",
    "argv": ["go", "tool", "polytype", "--typescript", "../web/src/lib/workflow"]
  }
}
```

SKGO compiles and starts this private child once, retains it across application replacements, and closes it with the development session. A small tooling-only SKGO Go package provides serialized JSON request/reply framing over stdin/stdout and adapts a Go callback to it. Diagnostics use stderr. Stdout corruption, exit or a missing result is a helper failure. Applications without a generator configuration keep the existing update behavior.

The request contains a monotonic request ID/event epoch, current SKGO compiler revision, a full-selection flag, and cumulative canonical absolute changed paths. The reply associates the request and carries generated plus error text; it contains no output inventory or patch instructions. The helper owns no request-serving application instance and acquires no project locks.

## Changes, selection and convergence

Extend SKGO's existing recursive watcher to accumulate raw application file events before compiler/generated-output filtering. Generation cannot depend only on Project.Changes: non-Go generator inputs and removed outputs may not change its captured compiler revision. Keep a raw event epoch independent of that revision. The first consumer's inputs must be covered by the existing selected local module watch roots; verify coverage rather than promising arbitrary external paths.

Preserve paths for removals even when they no longer exist. Directory events reconcile both previous and current membership. Startup, watcher overflow, generator configuration changes and a newly built helper request full selection. Ordinary requests carry the union of dirty paths since the last successful current generation pass. Keep failed and obsolete selections pending; events arriving during an invocation merge into the next pass.

Gimbal retains each producer's last successful source/dependency membership and its known owned output locations. It selects affected producers using those inputs, controls and outputs. Root-only path matching is insufficient for imported types and shared dependencies. Deleted/modified output selects its owner for repair; removed source invalidates old dependents and membership. A pure Svelte edit outside generator-owned output selects none. A producer registration/input removal may remove only independently identifiable owned output; diagnose ambiguous ownership instead of deleting arbitrary source.

Requests are serialized. Ordinary newer edits remain pending while generation finishes; no concurrent generator requests or cross-process AST snapshots are needed. A result acknowledges generation only if it is successful and current against the observed event epoch. A newer authored/helper/configuration event requires another selected pass before preparing or admitting an application update. Generated writes may require one converging no-op pass; equality-suppressed publication makes that pass terminate. This guarantee concerns observed edits, matching the existing watcher contract, rather than synchronously excluding undiscovered OS events.

## Result meaning and failure handling

`generated` means an application-consumed owned output was created, changed in bytes, removed or repaired. Invoking a generator, doing analysis/backend work, populating a private cache, or writing an equal-byte output is not generation. Return `changedSoFar, err` when publication partially succeeds and then fails. Gimbal ORs producer results before checking errors and does not run downstream consumers after a prerequisite fails.

SKGO retains a full-rebuild obligation after true, a failed generation attempt, a helper failure/lost reply, or an active-session pre-build/helper refresh that may have written. Conservatively forcing a rebuild after helper refresh avoids a second prerequisite-result protocol; warm ordinary requests do not run prerequisites. Keep the dirty selection and obligation through errors, recapture, helper replacement, obsolete attempts, failed compilation and failed replacement startup. A subsequent no-op success cannot undo it. Only successful admitted full replacement clears it.

An outstanding obligation bypasses native active.Prepare/Install, not merely its final publication. After a successful current generation pass, explicitly recapture/reconcile new and removed application files, run SKGO's own binding generation, settle Kit route metadata, and enter the existing complete replacement path. Do not wait for eventual generated-file events to make required Go available to compilation. Retain existing fresh revision checks, stop/wait-before-constructor behavior and last-admitted-input restoration after replacement startup failure. Do not invoke external tooling while holding Project's admission mutex.

## Bootstrap and fresh generator code

The helper's compiled dependencies can include generated code. Gimbal's extractor imports the workflow model, whose generated codecs reference concrete fields. A model shape edit can therefore prevent helper compilation until those codecs are regenerated.

Run the configured targeted pre-build command before the initial helper build and whenever its compiled inputs need refresh. Gimbal uses the existing pinned workflow Polytype command shown above. The standalone tool analyzes the `jsonschema` source selection, excluding stale `!jsonschema` codecs; it does not need the broken helper to compile first. Then rediscover the helper build closure, build it, and record the identity of the inputs actually compiled. No whole-application generation is used for bootstrap.

Reuse the removed runner's Go-selected compiled-input discovery: local package source membership, imports, module/workspace controls, selected embedded resources and build configuration. Retain the last known closure for repair detection when discovery fails. Never treat failed discovery as an empty dependency set. Compare against the identity from the successful helper build; generation must not update that identity merely because it wrote a compiled dependency. Rebuild again before the next callback if a compiled input changed. Full selection follows every helper replacement.

Source-target changes that are only analyzed can reuse workers. Helper implementation, compiled configuration, actual compiled model/codecs and build controls require fresh helper code. A failed pre-build, discovery or helper build must never fall back to invoking stale generator logic. During an active session preserve the healthy application and retry after relevant repair events. A genuine cold-start failure has no healthy app to preserve: report a clear startup error. No waiting-page policy or generic bootstrap transaction system is added.

## Gimbal producer coverage and ordering

Do not call appgen.run from the callback: it recursively invokes SKGO generation and omits other repository producers. Gimbal owns an explicit registry matching its current active generation declarations; additions must become registered or produce a visible unsupported-generation diagnostic. Exclude frozen legacy/reference material. Preserve each directive's actual flags, entrypoint and workflow name.

1. Run selected source-first schemas/codecs before consumers, using existing Polytype source selection and generation APIs. This includes the root `supervise.go` validation generation and the declared workflow package schema/validation producers. The workflow model wire codec/TypeScript command is also the helper pre-build prerequisite when required.
2. Generate selected authored workflows' Go graphs through Gimbal's existing Source/GenerateGraph path. This includes interview/implement-interview workflows outside the built-in stock list. Preserve the existing overlay that hides stale generated graphs during extraction.
3. Update affected stock invocation adapters and reassemble registry/catalog/design products before SKGO captures route inputs. Stock does not generate workflow_gen.go. Reuse cached per-workflow products so an affected workflow does not require extracting every unaffected built-in workflow; Stock's current batch cache is not sufficient for that claim.
4. Keep separate retained Polytype Workers for observation TypeScript and workflow TypeScript. Use package/name declarations to avoid compiling observed application packages merely to name roots. Use Worker-owned bounded sessions; keep non-retained cross-producer sessions invocation-scoped. Do not persist today's unbounded invocation-scoped Session unchanged. No new eviction policy is needed.
5. Include the existing opencode OpenAPI producer when its `types.json`, `types.cfg.yaml`, declaration/tool inputs or owned generated output change. Invoke only its pinned targeted tool. Gimbal's fixed-output adapter must use the same changed/equal-byte publication semantics; no generic directive interpreter or output manifest is required.

Some families are selected independently; the list establishes prerequisite ordering, not a requirement to run every family for every edit. Source and dependency selection is followed by each producer's own cache decisions. The retained Polytype fast path is TypeScript-only; schemas/codecs/devalue retain their existing generation paths and caches. Do not claim retained backend caching for those modes merely because a Worker or Session is passed.

Gimbal graph/component publication must adopt its existing equal-byte writer pattern and report real writes/removals; Stock's writer needs result reporting too. Polytype needs an additive result-bearing generation path while preserving existing error-only callers. Changed reporting and equality suppression must cover retained replay, ordinary TypeScript/devalue, Go/schema/codecs and owned removal/repair, not only the TypeScript Worker fast path. Statistics cannot substitute for this result.

Current SKGO pruning already preserves standalone Polytype outputs outside reserved src/lib/skgo. Correct the stale Gimbal comment rather than adding a post-SKGO callback phase.

## Production boundary

Place the helper main and new Gimbal selection/worker-retention wiring in tooling-only packages, importing the tooling-only SKGO protocol adapter. Ordinary production CLI/server/native entrypoints do not import these new packages, so no application-wide development build tag is needed. SKGO's native compiler currently rejects -tags; relaxing that profile is outside this integration.

The shipped Gimbal CLI already imports internal/generate and polytype/goload to implement gimbal generate. Preserve that feature and reuse its ordinary generator/cache routines where appropriate. The exclusion claim names the new development coordination/retained-worker wiring, not every existing package with generation in its name.

Check the actual production CLI, server and native-host dependency selections and built binaries for the new packages/symbols, and run their ordinary smoke behavior including the existing generate command. Verify separation by reachability and file selection, not a runtime development flag or an unexecuted build-tag assumption.

## Implementation order and proof

Implement the result/no-op publication semantics in Polytype and Gimbal writers first. Add the optional SKGO tooling integration with literal generic fixtures for raw changes, prerequisite/bootstrap ordering, request epochs, helper freshness, recapture, failure/retry and full-rebuild latching. Then connect Gimbal's explicit producer registry and update its dev recipe to select the configuration. Reuse the current application replacement path and watcher rather than writing a second supervisor.

Focused tests and the real Gimbal edit loop must show:

- An observation field edit and a body-only authored workflow edit produce the expected literal generated content and running behavior.
- A workflow schema/input edit, missing graph/TypeScript repair and deleted/recreated non-Go generator input select the correct producers; unrelated edits skip their backends.
- Cold-equal, warm-equal and analysis-with-equal-output runs preserve output bytes/mtimes and avoid unnecessary application replacement. A successful cache replay that repairs missing output reports true.
- A producer that writes and then fails, and a helper that dies after writes, keep the old app healthy; repair leads to a full replacement even if retry generation is now a no-op.
- A compiled workflow model/codec change and a helper implementation/configuration edit use fresh helper code through targeted bootstrap. Invalid helper source blocks stale invocation and recovers after correction.
- Rapid edits, generated-file events, route metadata retries and failed compilation do not admit stale replacements or lose the full-rebuild obligation. A cached generation result never substitutes for necessary Go compilation.
- Workers survive application replacement and are replaced with the helper when its compiled inputs change. Successful replacement restores the saved project with exclusive locking intact.
- Actual production targets exclude the new development integration while preserving intended existing commands.

These are application observations and ordinary tests, not a new proof runner, benchmark program or status ledger. No timing claim is made before measuring the running edit loop.

## Source and review anchors

- SKGO `/Users/tyler/src/skgo`, HEAD `5614666c53cab098e16d805b5b0b6dd5773816cb` (same tree as released v0.34.7).
- Gimbal `/Users/tyler/.codex/worktrees/gimbal-skgo-dev/gimbal`, HEAD `143ccd784ae852d05ec34564483e1c0e519ccd46`; preserve the existing dirty NewSession.svelte edit.
- Polytype `/Users/tyler/src/polytype`, HEAD `0fc3aadbbbce15689aebbd97287bc6640de298b1` (v1.9.0 matching Gimbal's pin).
- Initial proposal and first Sol 6.1 reviews are checkpointed at `041a79e` in this worktree. Review artifacts are under ephemeral/reviews/20261016-dev-generation-sol61-*-round-01.md.
- The same three reviewers rechecked the full consolidated design in round 2, with no remaining findings: [SKGO](../reviews/20261016-dev-generation-sol61-skgo-round-02.md), [Gimbal](../reviews/20261016-dev-generation-sol61-gimbal-round-02.md), [Polytype](../reviews/20261016-dev-generation-sol61-polytype-round-02.md). These reviews establish design consistency against the inspected source, not working implementation or production isolation.
