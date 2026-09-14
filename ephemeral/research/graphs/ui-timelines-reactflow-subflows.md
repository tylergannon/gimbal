# React Flow sub-flow source note

- Source: https://reactflow.dev/learn/layouting/sub-flows
- Example: https://reactflow.dev/examples/grouping/sub-flows
- Source type: official xyflow/React Flow documentation and example
- Retrieved: 2026-09-14

## Relevant observations

1. A sub-flow is a flow inside a node. Child nodes use `parentId` and positions
   relative to the parent; nested groups are supported.
2. A child can connect to nodes inside its group and to nodes outside the group.
   Thus visual containment and graph connectivity coexist but are not the same
   relation.
3. Parent nodes should appear before children in the node array. Moving a parent
   moves its children; `extent: 'parent'` can constrain child placement.
4. The example uses a group node with no handles as a visual container, showing
   that grouping need not itself create an executable operation.

## Transferable UI pattern

For the program projection, render explicit Scope/Group/Loop regions as
containers with local coordinates and allow typed edges to cross the boundary.
Keep container identity and edge identity independent. An expand/collapse action
should change visibility/layout only; it must not mutate the underlying graph or
erase cross-boundary edges.

## Limits for Gimble

React Flow positions represent an editor layout, not elapsed time or execution
order. A workflow helper call should not become a nested visual node simply
because it is a Go call. Only recognized Gimble scopes and operations belong in
the default program view; helper/callee relations can remain an opt-in overlay.
