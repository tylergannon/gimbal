# Loop node: max_visits on a nested loop resets per enclosing item

URL: https://github.com/tylergannon/gimble/issues/34
State: closed
Milestone: None
Updated: 2026-09-03T23:31:58Z

Branch `worktree-goal-gates`, `engine/loop.go`, `engine/runner.go`.

## Bug

`max_visits` is counted per node per run (spec §3.4) and nothing resets it. A loop node nested inside another loop's body therefore gets one budget for the whole run, not one per outer item. With an inner loop at `max_visits: 40`, the first outer item can spend every arrival; the second outer item then fails on its first arrival with "loop body has exhausted its visit budget". The same applies to the inner loop's body nodes.

## Required behavior

A loop node's `max_visits` counts arrivals per activation: the arrivals since the enclosing loop selected its current item. When the enclosing loop pushes a new item, the nested loop's visit counter and the counters of every node in its body node-set reset to zero.

A top-level loop has one activation per run; its budget is unchanged.

## Changes

- `engine/loop.go` / `engine/runner.go`: on frame push for a new item, reset visit counts for the nodes in the body node-set of the loop that pushed (`lint` already computes body node-sets; expose the lookup). The reset is engine-owned state, never written to the checklist.
- Checkpoints carry the reset counts as they carry counts today.
- Test: an outer checklist with two items, an inner loop at `max_visits: 3` whose first activation uses all three arrivals; the second outer item's inner loop runs without a budget error.
- Docs: `docs/spec.md` loop `max_visits` row and §3.4; `src/content/docs/loops.md`.

