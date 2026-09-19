# Phase 1b, the rest: runs list, lanes, small states

The promise: three more components under `web/src/lib/run/` with a story
beside each, in both themes. Topbar, DetailPane, and CancelGuard already
exist on this branch and are not changed. Nothing here touches a page, a
remote function, or the server. Use the fixtures under
`web/src/lib/run/fixtures/` as they are. Milestone 1 claims 26 to 29 apply.

## Claims

1. `RunsList`, as `docs/design/specimens/Runs.html`: attention items,
   filters, table; opening a run is an event.
2. `Lanes`, as `docs/design/specimens/History.html`, for a recorded run
   with no matching graph.
3. `SmallStates`, the states of `docs/design/specimens/States.html`:
   empty, loading skeleton, disconnected, no graph.
4. `pnpm build-storybook`, `pnpm check`, `pnpm test`, and
   `pnpm exec vp fmt --check` pass.

Done is the definition in the workflow. One task.
