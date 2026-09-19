# Phase 1b: the surround, in Storybook

The promise: the components around the map exist under `web/src/lib/run/`
with a story beside each, in both themes, so the application pages can be
assembled from them. Nothing here touches a page, a remote function, or
the server; the map components already exist on this branch and are not
changed here.

The design is `docs/design/`: `README.md` for the rules, `specimens/*.html`
for exact markup and CSS. Use the fixtures under `web/src/lib/run/fixtures/`
as they are. Milestone 1 claims 26 to 29 apply: Svelte 5 runes, the app's
own types as props, no fetching, tokens only, shadcn primitives where the
specimens use them.

## Claims

1. `Topbar`, as `Main.html`: breadcrumb, the status badges (running,
   completed, failed, cancelled, live, disconnected), elapsed time, the
   "now coding" link, the search box, Stop turn and Cancel run. It emits
   events for the buttons and calls nothing.
2. `DetailPane`, as `Main.html` and `Interview.html`: header, assignment,
   activity rows, the disclosures, the steer footer; variants for a
   selected command, scope, loop, and interview. Controls emit events.
3. `CancelGuard`: the dialog of `Cancel.html` on shadcn-svelte's
   alert-dialog, added with the CLI.
4. `RunsList`, as `Runs.html`: attention items, filters, table; opening a
   run is an event.
5. `Lanes`, as `History.html`, for a recorded run with no matching graph.
6. The small states of `States.html`: empty, loading skeleton,
   disconnected, no graph.
7. `pnpm build-storybook`, `pnpm check`, `pnpm test`, and
   `pnpm exec vp fmt --check` pass.

Done is the definition in the workflow: runs, looks like the specimen,
90 to 95 percent, small gaps listed. As few tasks as the coder can carry.
