# Milestone coverage, checked 2026-09-14

Latest user direction: both the bounded linter (#162) and bounded source graph extractor (#201) belong in Beta. Useful program shape is part of the Beta UI; exhaustive analysis is deferred. Tolerate incomplete linting, keep dynamic keys supported, and avoid hard analysis now. The design record's mandatory constant-key language is historical evidence; it is not the current shipping gate. The inventory below preserves the original research snapshot, before those milestone updates. See `delivery-slices.md` for the deliberately smaller immediate path and `keys-recommendation.md` for the separate authoring study.

Milestone 1 is Beta: 9 open and 20 closed issues in the saved API response. Its stated bar is Gimble building Gimble, with usable observation and steering. `milestone-1.json` and `milestone-1-issues.json` preserve the snapshot; `all-issues.json` preserves the repository-wide issue/PR inventory used to find work outside the milestone. Per-issue Markdown and milestone comment JSON files hold the readable evidence.

| Need | Existing coverage | Remaining work |
| --- | --- | --- |
| Build-time Set checks | Open [#162](https://github.com/tylergannon/gimble/issues/162), **no milestone** | Bring into Beta if this is a Beta promise; ship its useful subset with honest coverage and avoid false positives on unknown contexts. |
| Set runtime contract | Closed [#159](https://github.com/tylergannon/gimble/issues/159), Beta | Implemented runtime backstop; does not provide static checking. |
| Broader workflow lifetime rules | Design record Scope/Lints; [#126](https://github.com/tylergannon/gimble/issues/126) reviews joining contracts | No dedicated task covering constant names, session escape, and Group.Wait on every exit path. #162's raw-go Set check is narrower. |
| Static graph / generation | API design record Observability; web F13; SPRINTS after Sprint 4 | No dedicated current issue found. |
| Scope/time/usage visualization | Open [#173](https://github.com/tylergannon/gimble/issues/173), Beta | A runtime scope/turn tree and timeline. It does not build the static workflow graph. |
| Supervisor relationships | Open [#120](https://github.com/tylergannon/gimble/issues/120), Beta; web F10 | #120 demonstrates real nested supervision; the page relationship model needs explicit delivery scope. |
| Steer/kill visibility | Open [#197](https://github.com/tylergannon/gimble/issues/197), no milestone; closed #176 provides runtime control | Preserve source, target, and landed/dropped/killed records in snapshots and page. |
| Workflow command execution nodes | Ordinary os/exec in built-in sprint and examples | No current task or command lifecycle event family found. Legacy process-manager issue #76 is closed and predates the restart; it is not this work. |
| Public API and built-in workflows | Closed #177 documents shapes; built-in sprint exists under internal/workflows/sprint | The analyzer/generator must work on a consuming module, not just a hard-coded internal package. This research does not propose a new workflow catalogue. |

The roadmap already anticipated the direction. `repo-ephemeral-research-api-API.md` lines 275–293 lists lints, and lines 856–870 describes an SSA static pass. `repo-ephemeral-research-api-SPRINTS.md` lines 172–175 defers that work. `repo-docs-web-app.md` F10 covers supervisor/fork/steer edges, F13 the static template. These are design intentions, not completed features or issue-sized acceptance contracts.

## What #162 needs clarified

It is a useful first slice, not a complete ownership proof. It explicitly skips nonconstant keys and interprocedural writes, whereas the historical design calls for constant keys/names and ctx tracing through helpers. The current sprint actually uses a dynamic repository-check key. Tyler explicitly prefers keeping that working over enforcing the older rule. The omission is acceptable initial coverage; no workflow migration is required to ship the first linter.

The semantic unit is **scope identity**, not an SSA context value or identifier. `context.WithCancel(childCtx)` is still the child's scope. A helper may write on the caller's scope; a closure may capture the wrong ancestor ctx; both are analyzable in bounded cases. Duplicate checking needs instruction order and path reasoning, not block dominance alone. A write in an if-arm followed by an unconditional write can collide even though the first block does not dominate the second. An unconditional return/break can make a syntactic loop execute a write at most once.

Keep these distinctions in acceptance: correct derived contexts and exclusive branches pass; known parent writes and reserved `task` fail where proven. Cover aliases and repeated writes when simple enough, otherwise skip with documented coverage. Unsupported dynamic behavior is permitted and never silently certified. Broad group/session lifetime proof is later work.

## Proposed task sequence (drafts only)

1. **Catch straightforward Set misuse before running.** Ship #162's useful local subset. Preserve dynamic keys and correct derived contexts. Unknowns do not fail the build. Test positive/negative source examples and run against built-in sprint.
2. **Generate and preview the known workflow shape.** JSON with named regions, direct agent/command sites, visible supervisor options, branches and repetition. Follow simple local helpers needed by sprint; mark unsupported targets. No whole-program ownership analysis or exact run-to-callsite matching required.
3. **Show real command operations and existing supervisor edges.** Preserve ordinary os/exec; use an explicit command scope/result in one real workflow as the first observation experiment. Expose the supervisor relations already in the log. Build on #173's scope/turn view and #120's live proof. Treat exact process telemetry as later work unless this first view proves insufficient.
4. **Refine template/run linkage where it improves the view.** Expand repeats; distinguish owner from execution scope; aggregate ambiguous source sites. Optional helper-call overlays, interprocedural facts, and full lifetime checks wait for an actual need.

Do not make exact source-to-run identity or constant-key enforcement prerequisites. Tasks 1 and 2 can share recognized facts without waiting for a complete analysis engine. No issues were created, moved, or edited during this research.
