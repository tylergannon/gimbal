# Portable workflow viewer

This export uses the application's shared renderer in
`web/src/lib/workflow-design/`. It reflects static workflow source and does not
execute workflows. The original `inspector.html` remains available for comparison.

## Live development

Use `just dev` from the repository root, then open
[workflow pages](http://127.0.0.1:8080/workflows). The application's generated
built-in registry owns the page inventory. Each page has Workflow, Guide and Run
views. Source edits update the graph, guide and parameter descriptions together;
invalid edits keep the last successful version and show the error. Restart after
extractor changes; regenerate and restart for callable Go signature changes.

### URL view state

The workflow is the route path. Query parameters describe the requested view:

| Query | Meaning |
| --- | --- |
| `v=1` | State format version |
| `selected=key` | Selected source object |
| repeated `open=key` | Independently expanded scopes/write bundles |
| repeated `detail=/path` | Object/source/value disclosures for the selected object |
| `types=1` | Show Go types |
| `panel=guide` or `panel=run` | Active view; diagram is the default |
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
commit state; there is no effect that mirrors local state into the URL. The current Kit version
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

The shared `layout.ts` maps visible source operations into ELK compound nodes, ports, and
edges. Expanded conditions become diamonds; an else-if remains another ordered
decision. `RoutedEdge.svelte` uses ELK's actual sections and inline label positions.
Collapsed containers retain their descendant expansion choices. Set/SetJSON
sequences use one node with compact selectable rows.

Plain immediately preceding comments name nodes (first line) and describe them
(remaining lines). An optional `When true: Command provided` line labels the
positive edge. The fallback edge is unlabeled. Switch-case comments name their
case edges. These labels remain authored source, never guessed interpretations.

Portable export is a source-review artifact. Expansion reruns layout
while keeping the clicked header in place; the rest of the graph can move. Very
large expanded graphs and screen-reader graph traversal need further work.
