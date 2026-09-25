# Milestone 1: the run interface in Storybook

The promise: every part of the run interface design exists as a Svelte 5
component, looks like its specimen, and can be seen working in Storybook in
both themes before any of it is wired into the application. Nothing here
touches the running app: no page, no remote function, no server. The
application migration is milestone 2.

Paths are relative to the repository root. The design is `docs/design/`:
`README.md` holds the rules and the vocabulary, `specimens/*.html` are the
exact markup and CSS of every screen at 1440px, and `specimens/tokens.css`
is the design system's palette. Open a specimen in a browser to see it;
render it with Playwright to compare pixels.

## Claims

Each claim is something the validator can see in the running Storybook or
in a command's output. The milestone is done when the software runs and the
claims hold at 90 to 95 percent; small gaps are listed, not chased.

### Ground

1. Storybook shows only Gimbal stories. The generated demo under
   `web/src/stories/` is gone.
2. `web/src/app.css` carries the design's palette from
   `docs/design/specimens/tokens.css` for light and dark: the neutral scale,
   the blue-600 `--chart-3` and blue-700 `--chart-4`, the radius. The font
   stays Inter Variable, which the app already loads; the tokens file names
   Geist, and the specimens were drawn in Inter.
3. Dark mode uses the app's own convention, the `.dark` class that
   `app.css` declares as the dark variant. The specimens' `data-theme`
   attribute is a rendering convenience, not the app's mechanism.
4. Storybook has a toolbar switch for light and dark, so every story can be
   looked at in both. The specimens' dark overrides are honoured.
5. `pnpm build-storybook`, `pnpm check`, `pnpm test`, and
   `pnpm exec vp fmt --check` all pass in `web/`. The pre-commit hook runs
   the web checks when web files are staged.

### Fixtures

6. `web/src/lib/run/fixtures/` holds typed fixtures built from the app's own
   types: `Graph` from `web/src/lib/workflow/types.ts`, and the row types
   from `web/src/lib/observation/index.ts` (`RunRow`, `ScopeRow`,
   `SessionRow`, `TurnRow`, `InterviewRow`, `ModelCallRow`, `RunSnapshot`).
   The TypeScript types lack the commands table the Go snapshot serves
   (`CommandRow` and `commands` in `internal/observation/snapshot.go`):
   add `CommandRow` and `commands` to the TypeScript `RunSnapshot`, matching
   the Go field names exactly.
7. The fixtures are the design's four scenarios, with the substantive text
   the specimens use, never lorem ipsum:
   - the implement-interview run at task 3 of 3 with the coding turn running
     and two watchers, as `Main.html` (its graph is the generated
     `internal/workflows/implementinterview/workflow_gen.go`, transcribed to
     the TypeScript `Graph` shape);
   - the plan-trip run with two interviews waiting, as `Interview.html`;
   - the failed recorded run whose graph no longer matches, as
     `History.html`;
   - the runs list with attention items, as `Runs.html`.

### The map

8. `Pip`: the five states of `States.html`, shape before colour. Filled:
   ended. Blue spinner: running. Red with a cross: failed. Blue ring with a
   dot: waiting for you. Dashed: not yet.
9. `Node`: agent call, command, and interview, each with its icon, a 15px
   name, a meta line, a selected state, and a small variant, in every pip
   state that kind can hold. Only an interview waits for a person; an agent
   call or a command is never "waiting for you", and an interrupted turn or
   command is ended, not waiting.
10. `Sheet`, the scope: nested sheets alternate between the two paper tints
    and inset by 8px; shadow only on the path to the selection; the selected
    sheet raised with a dark border; a label straddling the top border with
    the scope's icon; a context chip, "context 1 of 8", counting keys written
    so far out of the keys the scope writes.
11. A repeated scope's label carries a select, "task 3 of 3", defaulting to
    the latest instance. Picking an instance recolours the sheet's pips and
    reports the selection to the outside.
12. A sheet folds. Folded, it keeps its sheet and label and shows one glyph
    per step with that step's status, plus the scope's status and elapsed
    time, as `Collapse.html`. Opening a sheet folds its siblings. The root is
    the canvas and never folds.
13. `Group`: a fork bar, the child scopes side by side, a join bar.
14. `Loop`: the body returns to its planner by an arrow in the loop's own
    right margin.
15. `Watcher`: a supervisor is a card in the margin, level with the call it
    watches, joined by a dotted bracket, with one status pip.
16. `Map` takes a `Graph` and a `RunSnapshot` and draws the tree in source
    order on one centred spine, as `Main.html` and `Interview.html` draw the
    two fixtures. Layout is a recursive measure-and-place in a plain
    TypeScript module with vitest tests on both fixtures; no graph layout
    library. Conditions are not drawn; a step that did not run in the
    selected instance says so. Sessions are not drawn.
17. The map pans and zooms, with the fit, minus, and plus chrome and the
    dotted ground of the specimens. Clicking a node, a sheet, or a watcher
    selects it and reports the selection to the outside; the map itself
    holds no run data beyond its props.
18. Every text on the map is 13px or larger. Borders clear 3:1 and text
    clears 4.5:1 against their ground. Secondary text is `oklch(0.42 0 0)`.
    Text links use blue-700; rings and spinners use blue-600. Blue is
    liveness and selection, red is failure, nothing else has a hue.

### Around the map

19. `Topbar`: the breadcrumb (Runs, workflow, run id), the status badges
    (running, completed, failed, cancelled, live, disconnected), elapsed
    time, the "now coding · task 3" link, the search box, and the Stop turn
    and Cancel run buttons, as `Main.html`.
20. `DetailPane`, the placeholder design in `Main.html`: header, assignment,
    activity rows, the disclosures (Prompt sent, Watchers, Session, Usage,
    Definition), and the steer footer with its textarea and Steer button.
    Variants for a selected command (exit code, stdout and stderr tails), a
    selected scope (the values written, its sessions), a selected loop (the
    backlog and a wrap-up control), and a selected interview with the answer
    box and End interview from `Interview.html`. Its controls emit events; it
    calls nothing.
21. `CancelGuard`: the dialog of `Cancel.html`, stop this turn versus cancel
    the whole run, built on shadcn-svelte's alert dialog.
22. The runs list of `Runs.html`: the attention items for pending questions,
    the filters, and the table.
23. The lanes of `History.html` for a recorded run whose graph no longer
    matches.
24. The small states of `States.html`: empty, loading skeleton, disconnected,
    no graph.
25. One vocabulary story draws every item in the README's vocabulary the
    way `States.html` does, so the whole language can be checked on one
    screen.

### How it is built

26. Components live under `web/src/lib/run/`, one component per file, its
    story beside it. Every component has a story; every story renders in
    both themes without console errors.
27. Svelte 5 runes and TypeScript throughout: `$props`, `$state`,
    `$derived`; no legacy syntax, no stores for local state. Props are the
    app's own types; components never fetch, never import a remote
    function, and never reach for `window` data.
28. The shadcn-svelte components already under `web/src/lib/components/ui/`
    are used where the specimens use them (button, badge, input, textarea,
    tooltip, dialog, table, scroll-area, separator, skeleton, tabs), and
    `select` and `alert-dialog` are added with the shadcn-svelte CLI. Colours
    come from the tokens in `app.css`; no component carries its own palette.
29. Nothing under `internal/observation/`, `docs/`, or a generated skgo
    file changes, except the TypeScript type additions claim 6 names.
