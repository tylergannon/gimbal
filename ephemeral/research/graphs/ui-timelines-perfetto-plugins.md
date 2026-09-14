# Perfetto plugin source note

- Source: https://perfetto.dev/docs/contributing/ui-plugins
- Source type: official Perfetto UI extension documentation
- Retrieved: 2026-09-14

## Relevant observations

1. Plugins can programmatically select a track event, an area, or an entire
   track. Selection options can switch to the selection tab and can scroll the
   timeline to the selected item.
2. A plugin can register tracks independently from the track nodes that place
   them in the workspace. This separates the data/renderer from row placement.
3. Timeline overlays draw across multiple tracks. The documented overlay API
   receives a time scale and the vertical bounds of visible tracks, allowing
   arrows or vertical annotations to align with rows without changing row data.
4. Selection tabs let a selected event or area expose custom contextual detail
   without replacing the overview.

## Transferable UI pattern

Use a canonical node/instance identity and let all views publish or consume that
selection. Render supervisory, steering, or other cross lane relations as an
overlay/side lane so they remain visible without pretending to be containment or
completion edges. Treat the selected drawer as a projection, not a second graph.

## Limits for Gimble

The overlay is meaningful only for observed runtime coordinates. A static
program view can show source relations and unresolved alternatives, but should
not place them on a time axis or claim that an overlay relation is a dependency.
