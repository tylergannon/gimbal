# The run interface design

Agreed with Tyler on 2026-09-18. The design canvas is the source:
https://claude.ai/artifact/WPqTNhEPN8waB4GhhVhS4L (private; ask Tyler).
The files in `specimens/` are static renders of its seven boards, each a
self-contained HTML page at 1440px wide: exact markup and CSS for every
component, in the shadcn-svelte Vega look the app already uses. Open one in a
browser. `tokens.css` and `bundle.css` are copied from that design system
and are styling data, never edited here. Each page has `data-theme` on its
root element; set it to `dark` to see the dark theme.

| Specimen | What it shows |
| --- | --- |
| `Runs.html` | The runs list: attention items for pending questions, filters, the table. |
| `Main.html` | A live run: the map on the left, the detail pane on the right, a coding turn selected. |
| `Interview.html` | A live run with two interviews waiting, one selected, the answer box. |
| `Cancel.html` | The cancel-run guard over the live run, stop turn versus cancel run. |
| `History.html` | A failed recorded run whose graph no longer matches: lanes instead of a map. |
| `States.html` | Small states and the map vocabulary. |
| `Collapse.html` | A folded scope, what a fold keeps, and depth by alternating tint. |

## Building it

`milestone-1-storybook.md` and `milestone-2-app.md` are the claims a build
run implements and demonstrates: first every component in Storybook, then
the application on those components. `gimbal run build-frontend` takes one
milestone file and commits each task with the repository's hooks running.

## The rules

- **One home per fact.** The map is shape plus status. Names, prompts,
  context values, watchers and usage live in the detail pane. A link between
  things appears on selection, never as a standing line.
- **The map is a tree drawn in source order.** Steps stack on one centered
  spine. A group's children sit side by side between a fork and a join bar. A
  loop's body returns to its planner by an arrow in the loop's own right
  margin. Layout is a recursive measure-and-place; no graph layout engine.
- **Every scope is a sheet.** Nested sheets alternate between the two paper
  tints and inset by 8px. Shadow means one thing: the path to the selection.
  The selected scope is raised with a dark border; everything off the path is
  flat.
- **A scope folds.** Folded, it keeps its sheet and label and shows one glyph
  per step with that step's status, plus the scope's status and elapsed
  time. Opening a scope folds its siblings. The root is the canvas and never
  folds.
- **Repeated scopes carry a select** on their label, "task 3 of 3",
  defaulting to the latest instance. Picking one recolors the scope's pips
  and points the detail pane at it.
- **Context is a chip on the scope**, "context 1 of 8": keys written so far
  out of the keys the scope writes. The list and values are in the detail
  pane. Sessions are a property of the scope that declared them and are not
  drawn.
- **Watchers sit in the margin.** A supervisor is a card outside the scope,
  level with the call it watches, joined by a dotted bracket, with one status
  pip.
- **Conditions are not drawn.** A guarded step is a plain step; in an
  instance where it did not run it says so. The raw case text is in the
  detail pane.
- **Status is shape before colour.** Filled: ended. Blue spinner: running.
  Red with a cross: failed. Blue ring with a dot: waiting for you. Dashed:
  not yet. Blue is liveness and selection, red is failure, nothing else has a
  hue.
- **Contrast for tired eyes.** Borders clear 3:1, text clears 4.5:1, nothing
  on the map is under 13px, node names are 15px, including the small
  command variant, whose name is set in the mono face (the specimens draw
  that one at 13.5px; 15px is the rule). Text links use blue-700;
  rings and spinners use blue-600. Secondary text is `oklch(0.42 0 0)`, not
  the token's mid gray.
- **Overview and detail are separate designs.** The detail pane in the
  specimens is a placeholder to be designed on its own.

## Vocabulary

agent call (a turn on a session) · command · interview · scope (a sheet) ·
group · loop · task select · context chip · watcher · fold · the five pips.
`States.html` draws each one.
