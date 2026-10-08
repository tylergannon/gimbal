# Source workflow layout experiment

A local, standalone Svelte Flow viewer of `gimbalgen`'s static source inspection.
It does not execute workflows or change the runtime console. The original
`inspector.html` remains available for comparison.

From this directory:

```sh
npm ci
npm run check
npm test
npm run build
go run ../gimbalgen -dir ../../workflows/validateproduct -entry ValidateProduct -name validate-product -o /tmp/validate-product.json
npm run page -- /tmp/validate-product.json /tmp/validate-product-flow.html
```

For the branching specimen, use `-dir ../testdata/inspection -entry Routing`.
Open the generated HTML directly or serve its directory locally. Each output
includes the viewer, ELK, and the source projection; there is no CDN dependency.

`layout.ts` maps visible source operations into ELK compound nodes, ports, and
edges. Expanded conditions become diamonds; an else-if remains another ordered
decision. `RoutedEdge.svelte` uses ELK's actual sections and inline label positions.
Collapsed containers retain their descendant expansion choices. Set/SetJSON
sequences use one node with compact selectable rows.

Plain immediately preceding comments name nodes (first line) and describe them
(remaining lines). An optional `When true: Command provided` line labels the
positive edge. The fallback edge is unlabeled. Switch-case comments name their
case edges. These labels remain authored source, never guessed interpretations.

This is a routing experiment, not a replacement release. Expansion reruns layout
while keeping the clicked header in place; the rest of the graph can move. Very
large expanded graphs and screen-reader graph traversal need further work.
