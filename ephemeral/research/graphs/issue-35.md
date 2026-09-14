# Loop node: rebuild frames from checklists on resume instead of rewinding to the outermost loop

URL: https://github.com/tylergannon/gimble/issues/35
State: closed
Milestone: None
Updated: 2026-09-03T23:32:00Z

Branch `worktree-goal-gates`, `engine/runner.go`, `lint.OutermostLoop`.

## Current behavior

The loop frame stack is in-memory only. On resume inside a loop body, the engine rewinds to the outermost enclosing loop. That loop arrives frameless, selects its first open item, and dispatches its body from the top. In a nested shape (outer loop → plan node → inner loop → implement), the plan node runs again before the inner loop is reached.

This is not incorrect: the plan node sees the ledger it already wrote, and restart-by-re-derivation from the workspace is the memo's stated doctrine. It costs one extra turn of the outer body per resume.

## Enhancement

Rebuild the frame stack from the checklist files instead of rewinding past them:

1. For each loop whose body contains the checkpoint node, outermost first, load its checklist and push a frame for its first open item.
2. Resume at the innermost such loop. It arrives frameless, validates nothing, selects its first open item, and continues.

Nodes ahead of the inner loop inside the outer lap are not executed again.

## Changes

- `engine/runner.go`: replace the `lint.OutermostLoop` rewind with a walk over the enclosing loops (`lint` already computes body node-sets); push frames from the files; resume at the innermost loop.
- Timeline event `ResumeRewound` records the rebuilt stack.
- Test: `TestResumeAtNestedLoopRewindsToOuterLoop` becomes resume-at-inner-loop with the outer frame rebuilt; the outer body's first node does not run again.
- Docs: `docs/spec.md` loop resume paragraph; `loop-node.md` §4.

