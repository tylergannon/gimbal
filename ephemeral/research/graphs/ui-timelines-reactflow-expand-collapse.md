# React Flow expand/collapse source note

- Source: https://reactflow.dev/examples/layout/expand-collapse
- Source type: official xyflow/React Flow Pro example description
- Retrieved: 2026-09-14

## Relevant observations

1. The example keeps the complete graph structure while rendering only the
   currently visible portions.
2. Clicking a node expands or collapses its descendants. Expansion state lives
   in node data, and layout is recalculated as visible nodes change.
3. The example describes dynamic node addition and automatic layout
   recalculation, so a changing graph can preserve a coherent current viewport.

## Transferable UI pattern

Keep hidden descendants in the canonical model and expose an explicit expansion
state in the view model. For live Gimble runs, preserve stable row/node IDs and
anchor newly revealed work to its existing region; do not reorder the whole page
when one child starts or completes. Show a collapsed group's count/status and a
visible exceptional child indicator so failures remain discoverable.

## Limits for Gimble

Automatic layout is useful for a static workflow template, but changing a live
waterfall's vertical order can break the operator's spatial memory. Use stable
wall-clock lanes/row keys for the runtime projection; reserve relayout for an
explicit view change or a new program revision.
