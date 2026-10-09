# Source workflow layout experiment

A local, standalone Svelte Flow viewer of `gimbalgen`'s static source inspection.
It does not execute workflows or change the runtime console. The original
`inspector.html` remains available for comparison.

## Live development

From this directory:

```sh
npm ci
npm run dev
```

Open [validate-product](http://127.0.0.1:8767/validate-product),
[routing](http://127.0.0.1:8767/routing), or
[context-shape](http://127.0.0.1:8767/context-shape). This is a local SvelteKit
3 development shell around the same diagram renderer used by portable exports.
Do not open `src/app.html` directly: it is Kit's template, filled by the server.

Vite hot updates the Svelte UI. A dev-only plugin watches workflow Go sources,
local dependencies, embedded inputs and Go module/workspace configuration;
re-extraction updates the graph without executing a workflow or reloading the
page. A parse/type error keeps the last successful graph and displays the error.
The next successful edit clears it. `vite.dev.config.ts` declares the preview
workflows; add `{name, dir, entry}` entries to `workflowSource({workflows})`
to inspect another workflow. Restart the server after changing the Go extractor
implementation itself; the extractor binary is compiled once at startup.

### URL view state

The workflow is the route path. Query parameters describe the requested view:

| Query | Meaning |
| --- | --- |
| `v=1` | State format version |
| `selected=key` | Selected source object |
| repeated `open=key` | Independently expanded scopes/write bundles |
| repeated `detail=/path` | Object/source/value disclosures for the selected object |
| `types=1` | Show Go types |
| `x`, `y`, `z` | Camera translation and zoom, as a complete finite tuple |

Defaults are omitted. Unknown/removed/ambiguous source keys and unsupported
field paths are dropped; the URL is replaced with its canonical valid form.
Valid hidden child expansion choices survive parent collapse. Keys fingerprint
source sites/content, so moving or changing an operation may reset its state.
There is no identity migration or guessing. Selection adds a history entry;
disclosure/camera/default repair replaces the current entry. Camera commits at
movement end. Reload, history traversal and UI hot updates restore the view
from the URL. The state describes presentation; it contains no source values.

Kit owns navigation through `goto` with `shallow:true`. Explicit event handlers
commit state; there is no effect that mirrors local state into the URL. Kit 3.0.1
skips `afterNavigate` on shallow Back/Forward, so a same-route popstate handler
applies the browser URL directly. Kit/portable/test Vite caches are isolated.

## Portable HTML export

From this directory:

```sh
npm ci
npm run check
npm test
npm run test:dev
npm run build
go run ../gimbalgen -dir ../../workflows/validateproduct -entry ValidateProduct -name validate-product -o /tmp/validate-product.json
npm run page -- /tmp/validate-product.json /tmp/validate-product-flow.html
```

For the branching specimen, use `-dir ../testdata/inspection -entry Routing`.
Open the generated HTML directly or serve its directory locally. Each output
includes the viewer, ELK, and the source projection; there is no CDN dependency.
Portable exports share the renderer but do not include Kit URL navigation or
the development source watcher.

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
