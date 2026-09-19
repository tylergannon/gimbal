# Current graph and web seams

Goal: identify the smallest current implementation seam for adding static
scope-owned service representation while preserving existing names, source
anchors, and ordered operations.

## Current data path

1. `workflow.Graph` has an entry `Body []Operation`; `Scope` and `Iterate`
   each carry a nested `Body`, while `Source` is embedded in every relevant
   node ([workflow-graph.go.txt](sources/workflow-graph.go.txt#L21-L40),
   [workflow-graph.go.txt](sources/workflow-graph.go.txt#L93-L136)). This already gives
   root, named-child, and iteration nesting.
2. AST extraction in `generator-expr.go` emits `workflow.Command` for
   `RunCommand`, `Service`, and `Check`, in source order, preserving the
   constant name/key and `e.at(call.Pos())` source anchor
   ([generator-expr.go.txt](sources/generator-expr.go.txt#L202-L227)). Scope extraction
   stores nested operations in `Scope.Body`; `Iterate` does the same for its
   static item body ([generator-expr.go.txt](sources/generator-expr.go.txt#L238-L245),
   [generator-stmt.go.txt](sources/generator-stmt.go.txt#L322-L368)).
3. Generated Go writes the graph literal and registers it; the generated
   Polytype codec and TypeScript projection serialize the same sealed union
   ([generator-source.go.txt](sources/generator-source.go.txt#L100-L145),
   [workflow-types.ts.txt](sources/workflow-types.ts.txt#L8-L70)). The run route
   marshals the registered graph beside the snapshot, and the Svelte page
   parses it and chooses `Map` when graph and snapshot match
   ([page-server.go.txt](sources/page-server.go.txt#L17-L64),
   [run-page.svelte.txt](sources/run-page.svelte.txt#L19-L27)).
4. The web layout currently treats only agent calls, commands, and interviews
   as visible nodes; `command` is measured and laid out in the ordered
   sequence ([web-layout.ts.txt](sources/web-layout.ts.txt#L209-L255)). `Node.svelte`
   gives every command the same terminal icon and small step treatment
   ([web-Node.svelte.txt](sources/web-Node.svelte.txt#L35-L59),
   [web-Node.svelte.txt](sources/web-Node.svelte.txt#L128-L146)). Scope sheets already
   provide the natural ownership container; map selection is keyed by scope
   plus operation kind/name/source location ([web-layout.ts.txt](sources/web-layout.ts.txt#L339-L451),
   [web-Map.svelte.txt](sources/web-Map.svelte.txt#L66-L115)).

## Smallest seam and coverage

The smallest backend seam is the graph model plus extractor: add a static
service collection or service-specific non-step variant while leaving
`Scope.Body`, `Iterate.Body`, `Source`, and constant extraction intact. The
smallest serialization seam is the generated graph/Polytype output; it must be
regenerated, never hand-edited. The smallest UI seam is layout measurement plus
the scope sheet/Node rendering and selection path. The route and snapshot
handoff already transport arbitrary registered graph JSON, so no runtime table
or service lifecycle change is indicated.

Existing tests cover extraction of nested scopes, commands, service-as-command,
and iteration bodies ([generator-graph_test.go.txt](sources/generator-graph_test.go.txt#L14-L82));
web tests cover scope instance selection, folding, layout, source-based node
selection, and runtime ordinal binding ([web-layout.test.ts.txt](sources/web-layout.test.ts.txt#L149-L248),
[web-Map.svelte.test.ts.txt](sources/web-Map.svelte.test.ts.txt#L1-L105)). They do not
yet assert that a service is structurally outside the ordered step sequence or
visually distinct.

## Implementation implication

Keep `Body` as the sequence for `RunCommand` and `Check`. Put services on the
declaring scope/iteration container (a `Services` field is the most direct
shape), each item retaining the existing `Name` and `Source`. Render that
collection as a compact labelled resource strip/list inside the scope sheet,
using a service-specific icon/style and no sequence connector. The root uses
the graph container; named scopes and iteration scopes use their existing
sheet boundaries. Selection can reuse the existing source-based key and detail
pane, but must not look up a service as an ordered command step.

## Risks / unresolved

- Generated Go/TypeScript codec output must stay synchronized; stale generated
  types would make the page silently reject the graph.
- `selection.ts`, layout flattening, and detail lookup currently assume every
  visible command is a runtime command row; services need a separate static
  branch without changing runtime command observation.
- A service inside `Iterate` must remain one declaration in the static body,
  even when the run has many item scopes.
- The current sources do not settle whether service selection should reuse
  `kind: command` with an ownership field or introduce a distinct sealed
  variant; preserve the evidence boundary and decide this in implementation.
