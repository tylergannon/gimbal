# Gimble research graph index

This is a flat routing index over `/Users/tyler/.codex/worktrees/7b5c/gimble/ephemeral/research/graphs` (the source corpus is read-only). Start with the route matching the question, then open the named leaf/report and its cited source excerpt. Citations are local `file:line` anchors; `repo-*` files are historical repository snapshots and API design, while `recommendation.md` and `milestone-assessment.md` are proposals/assessment rather than implementation proof.

Routes:

- [index-milestones.md](index-milestones.md): Beta coverage, issue/task sequence, and #162 scope.
- [index-rules.md](index-rules.md): scope identity, static graph rules, and lint boundaries.
- [index-runtime.md](index-runtime.md): runtime graph rows, lifecycle edges, UI and command-observation limits.
- [index-go.md](index-go.md): Go analysis/SSA capabilities and the local feasibility probe.

Latest authority for immediate scope is `delivery-slices.md` and the opening direction in `milestone-assessment.md`; historical mandatory-constant rules remain evidence, not a release gate. The bounded Set-key study is `keys-recommendation.md` with evidence in `keys-evidence.md`; it recommends stable outer keys while keeping dynamic keys legal. Use `all-issues.json` only to locate a focused issue; it includes unrelated pre-restart issues and PRs. Do not infer that a design record, route, or probe means the graph generator, analyzer, or visualization is implemented. Current source/runtime evidence, historical API design, and proposed work are labeled in each route.

Housekeeping is in `index-state.json`; the eight-query retrieval check is in `index-evals.jsonl`.
