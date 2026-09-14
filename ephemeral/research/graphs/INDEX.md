# Gimble research graph index

This is a flat routing index over `/Users/tyler/.codex/worktrees/7b5c/gimble/ephemeral/research/graphs` (the source corpus is read-only). Start with the route matching the question, then open the named leaf/report and its cited source excerpt. Citations are local `file:line` anchors; `repo-*` files are historical repository snapshots and API design, while `recommendation.md` and `milestone-assessment.md` are proposals/assessment rather than implementation proof.

Routes:

- [beta-implementation-handoff.md](beta-implementation-handoff.md): current session decisions, fresh Beta inventory, delivery sequence, parallel ownership, and integration responsibilities. Start here for implementation.
- [beta-bug-triage.md](beta-bug-triage.md): full open-backlog audit, ten additional Beta assignments, parallel ownership for small fixes, and explicit reasons for remaining deferrals.
- [index-milestones.md](index-milestones.md): Beta coverage, issue/task sequence, and #162 scope.
- [index-rules.md](index-rules.md): scope identity, static graph rules, and lint boundaries.
- [index-runtime.md](index-runtime.md): runtime graph rows, lifecycle edges, UI and command-observation limits.
- [index-go.md](index-go.md): Go analysis/SSA capabilities and the local feasibility probe.
- [index-ui.md](index-ui.md): UI inspiration for overview/focus, program/run projections, timelines, loops, supervision, commands, and Tenacious boundaries.

Latest authority is `beta-implementation-handoff.md`, `constant-context-key-rule.md`, and the captured current issues #162/#201. Constant Set/SetJSON keys and statically recoverable workflow shape are now required for clean lint; earlier recommendations allowing dynamic keys or unresolved accepted structure are superseded. `keys-recommendation.md` and older delivery drafts preserve earlier reasoning, not current policy. Use `all-issues.json` only to locate historical evidence; the current Beta listing and individual task texts are `beta-handoff-issues.json` and `beta-issue-N.md`. Do not infer that a design record, route, or probe means the graph generator, analyzer, or visualization is implemented.

Housekeeping is in `index-state.json`; dated retrieval checks are in `index-evals.jsonl`. Older citation checks do not certify the current issue scope.
