# The run map uses the designer's drawing

The active-run page draws a run with the workflow designer's shapes, layout
and styling (`web/src/lib/workflow-design/`). Run state is layered on top.
The visual redesign comes later. This change is about delivering the right
data to the front end.

## Data delivery

- When a run begins, `beginRun` writes the workflow's designer source page to
  `runs/<id>/source.json`, next to `graph.json`.
- The daemon's artifact whitelist allows `source.json`, so remote runs serve
  it too.
- `runview.Data` has a `source` field. It is read locally or through the
  execution backend. A missing file reads as an empty string, and a backend
  404 now wraps `fs.ErrNotExist`.
- The generated remote function and page load carry `{snapshot, graph,
  source}`.

## Binding a run to the drawing

`web/src/lib/run/runSource.ts` binds every designer node to a run graph
operation by source position (`file:line`). That match is exact across all
eight current workflows. A node's scope path such as `root/a/b` resolves one
segment at a time. It uses the chosen instance where there is one and the
latest instance otherwise. Promise-loop children live under `<loop>/task`.

Each bound node gets a status, a meta line, its instances and its watchers.
Selections are the same `MapSelection` values as before, so the detail pane,
session page, search and current-step tracking are unchanged.

A run recorded before `source.json` existed falls back to
`sourceFromGraph`, which builds an equivalent page from `graph.json`.

## What changed on the page

- The map is the only view. The Watchboard tab and the separate Focus tab are
  gone. Watchers sit in the margin beside the step they watch. Focus still
  opens from a selected agent step.
- Clicking a step selects it in the detail pane. The designer inspector
  shows a static node when nothing in the run is selected.
- Dark graph colours are scoped to `.dark .run-map`, so designer pages keep
  their light look.

## Follow-ups

- `selection.ts` still uses the old `buildMapLayout` geometry for search and
  reveal. That could move onto `runSource`.
- The visual redesign.
