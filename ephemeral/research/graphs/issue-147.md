# Run page shows a task scope's task twice

URL: https://github.com/tylergannon/gimble/issues/147
State: closed
Updated: 2026-09-13T17:35:32Z

Follow-up from #144 / #146.

A Loop task scope records its task two ways: `scope_began` carries it as `task`, and `loop.go` also stores it as the scope value `task` (`store(ctx, "task", raw)`) so `ScopeText` can hand it to the worker. The run page renders both: a `task · <name>` line from the scope's task, then a `task · {...full JSON...}` line from the values. Seen live in `ephemeral/attest/issue144/midrun.png`.

Not a correctness defect; the snapshot is telling the truth. It is just noise on every task scope.

Options, cheapest first:
- The viewer skips a value whose key is `task` when the scope already has a task.
- Or the runtime stops recording the task as a value and `ScopeText` reads it from the scope instead.

Acceptance: a task scope on the run page shows its task once.
