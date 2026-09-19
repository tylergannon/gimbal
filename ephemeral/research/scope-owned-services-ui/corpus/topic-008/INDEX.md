# Acceptance and unresolved questions

## Answers to assigned questions

The evidence matrix is the required root/named-child/iteration coverage and
also includes ordinary `RunCommand`/`Check` ordering. The generated-data and
browser observations in its two columns are the proof criteria: they establish
declaring-scope ownership, unchanged constant/source identity, absence from the
ordered step sequence, and visual/accessibility distinction. A passing test
gate alone is explicitly insufficient.

The only unresolved items are the exact field spelling/icon token and the
collapsed-scope summary choice, because the local contract and current UI
sources do not decide them. The scope is otherwise complete: citations are
local copies, this topic has more than the required three sources, and the
final report must remain within the stated 1,800 o200k_base-token budget and
two editorial rounds.

## Evidence matrix

| Fixture case | Generated-data observation | Web-rendering observation |
| --- | --- | --- |
| Root `Service("root-db")` | Root graph/scope has `services:[{name:"root-db",source:{file,line}}]`; it is absent from ordered `body`. | Root container has a labelled Services strip/list; the resource is not on the operation connector. |
| Named child `Scope("backend")` with `Service("api")` | The child scope owns `api`; its source file/line and constant name survive; parent does not claim it. | `api` appears inside `backend`'s container, not at root, with the same source identity in detail/selection. |
| `Iterate("item")` with `Service("fixture")` | The static iterate body/declared item scope owns `fixture` once; no runtime item/PID instances are serialized. | The iteration scope shows one owned resource collection; folding/selection does not turn it into repeated steps. |
| Ordinary `RunCommand("build")`, `Check("tests")` around services | `build` and `tests` remain ordered `body` operations with their source locations; service membership is a separate field. | Their vertical sequence/connector and blocking-operation treatment remain unchanged and visibly distinct from Services. |

The proof must inspect the generated graph JSON produced by the real extractor
and the rendered browser map for all four cases. Existing tests cover nested
scope/iteration and command extraction ([generate-graph-test.go.txt](sources/generate-graph-test.go.txt):14-80),
but a green gate alone does not demonstrate ownership. The decisive observations
are: service appears under exactly its declaring scope; its name and file/line
are unchanged; no service is counted as a step; and command/check order is
unchanged. The current seam explains why: `Source` is module-relative file plus
line ([generate-graph.go.txt](sources/generate-graph.go.txt):130-138), while `Body` is
source order ([workflow-graph.go.txt](sources/workflow-graph.go.txt):21-30).

## Minimal acceptance constraints

1. Static extraction, generated serialization, generated TypeScript, and the
   web layout must agree on one ownership field; no runtime service state may
   leak into the graph ([issue-284](sources/issue-284.md):14-27).
2. Root, named child, and iteration declarations must retain constant names and
   source locations, including when the scope is folded or selected. `Iterate`
   records a static body, not runtime values ([workflow-graph.go.txt](sources/workflow-graph.go.txt):129-136).
3. The service collection must not feed existing ordered-node flattening or
   connector code ([run-layout.ts.txt](sources/run-layout.ts.txt):209-245,479-495).
4. Browser proof must show a visible label/icon and accessible selection/source
   detail; existing nodes use `aria-label`, `aria-pressed`, hidden decorative
   icons, and `:focus-visible` ([run-node.svelte.txt](sources/run-node.svelte.txt):38-86).

## Unresolved only where evidence cannot settle it

- The local contract does not choose the exact field spelling (`Services` vs a
  differently named collection) or the final icon/color token. Pick one during
  implementation, then assert the semantic label and non-step structure rather
  than a fragile visual pixel.
- The local sources do not settle whether a collapsed scope should show a
  service count, names, or both. The minimum safe acceptance is a labelled
  affordance with name and source available on selection/detail.

Questions addressed explicitly: (1) the matrix covers root, named child,
iteration, and ordinary command/check ordering; (2) the generated-data and
web-rendering observations define what must be seen to prove ownership rather
than merely show green tests; (3) unresolved questions are limited to the two
evidence gaps above, with local-corpus citations throughout, and the budget/
editorial-round constraint is recorded above. Longer evidence: [acceptance clip](clips/acceptance-evidence.md).
