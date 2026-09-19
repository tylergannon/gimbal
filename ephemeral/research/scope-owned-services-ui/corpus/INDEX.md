# Issue #284 prior-art semantic index

## Entrypoint and scope

This is the author-facing routing entry point for the local corpus supporting a
narrow report on static, scope-owned services in Gimble. Read the synthesis
below first; follow the route links for evidence. Citations are local paths
relative to this file. The corpus is eight topic indexes (`topic-001` through
`topic-008`); those indexes route to copied primary documentation, source, and
longer evidence clips. It excludes runtime process instances, PIDs,
readiness/health, restarts, process trees, general dependency graphs, and
service-runtime changes.

## Sufficient synthesis for the document

The required fact is declaration ownership: the root graph body owns a root
`Service`; a named `Scope` owns services in its body; an `Iterate` owns one
static service declaration in its reusable body, not one node per runtime item.
Keep each constant call-site `Name` and `Source{File, Line}`. The current
contract and exclusions are in [issue #284](topic-001/sources/issue-284.md#L9),
the #276 boundary and API naming rules in [Gimble API evidence](topic-001/sources/gimble-api.md#L972),
and the lifetime distinction in [service tests](topic-001/sources/service_test.go.txt#L18).

The minimal static shape supported by the evidence is a `Services []Service`
collection on the root graph and each scope-bearing node, with `{Name, Source}`
entries. Keep `RunCommand` and `Check` in the existing ordered `Body`; service
membership is containment, not an operation or edge. The current graph,
extractor, serialization, and web seams are routed by
[topic-002](topic-002/INDEX.md), especially [recursive bodies](topic-002/sources/workflow-graph.go.txt#L21),
[constant extraction](topic-002/sources/generator-expr.go.txt#L202), and
[ordered layout](topic-002/sources/web-layout.ts.txt#L209).

Render one labelled **Services** strip/list inside each existing scope sheet,
above or beside the ordered operation body. Use a distinct non-step icon/style,
no sequence spine or connector, and preserve `Name` plus `file:line` in the
selectable detail surface. Keep root, named-child, and static iteration
containers distinct. The web evidence is summarized in
[topic-006](topic-006/INDEX.md); retain its longer [rendering clip](topic-006/clips/web-rendering-evidence.md)
when evaluating containment, folding, accessibility, or responsive layout.

## Compact precedent route

These four are the recommended comparison set; do not expand it into a catalog.

| Precedent | Ownership / nesting | Non-step signal and Gimble use |
|---|---|---|
| [Pulumi parent](topic-007/sources/pulumi-parent.md#L7) | Explicit parent plus implicit root; arbitrary nested tree. | Borrow containment and stable author-facing paths; omit lifecycle inheritance. |
| [Kubernetes owner references](topic-007/sources/kubernetes-owner-references.md#L9) | Explicit owner name/UID; dependents can nest or share an owner. | Ownership metadata is separate from steps; omit garbage collection/controllers. |
| [GitHub Actions services](topic-007/sources/github-actions-docker-services.md#L16) | Named `services` belong to one job beside its steps. | Closest resource-collection layout; preserve Gimble’s source identity and no teardown semantics. |
| [Docker Compose services](topic-007/sources/docker-compose-services.md#L8) | Named top-level service map. | Separate from commands; `depends_on` proves edges are optional and out of scope. |

For details on the comparison and its five implementation risks, use
[topic-007](topic-007/INDEX.md) and its [comparison clip](topic-007/clips/comparison-evidence.md).
For the alternate first-class-resource cue, see [topic-004](topic-004/INDEX.md)
and its [service precedent clip](topic-004/clips/service-precedents.md); Dagger
is useful only as a semantic contrast, not as a runtime model.

## Graph and UI routes

- [Static graph encodings](topic-005/INDEX.md): Graphviz clusters, Mermaid
  subgraphs, and BPMN sequence-flow versus containment; retain the longer
  [graph clip](topic-005/clips/graph-encoding-evidence.md).
- [Web rendering patterns](topic-006/INDEX.md): React Flow and Svelte Flow
  parent containment plus GitHub’s job/service distinction; retain the longer
  [web clip](topic-006/clips/web-rendering-evidence.md).
- [Current Gimble seams](topic-002/INDEX.md): generated Go/TypeScript parity,
  scope sheets, selection, and existing tests. Generated artifacts must be
  regenerated, not hand-edited.

## Acceptance route

Use [topic-008](topic-008/INDEX.md) as the evidence matrix. The decisive proof
is generated graph JSON plus the rendered browser map for: root
`Service("root-db")`; named child `Scope("backend")` with `Service("api")`;
one static `Iterate` body with `Service("fixture")`; and surrounding ordered
`RunCommand("build")` / `Check("tests")`. Verify declaring-scope placement,
unchanged name/source, one iteration declaration, unchanged command/check
order, a labelled accessible Services group, and no service sequence edge.
Keep the longer [acceptance clip](topic-008/clips/acceptance-evidence.md) open
when writing proof. A green test gate alone is insufficient.

## Retrieval queries and housekeeping

| Query | First route |
|---|---|
| Issue contract, exclusions, lifetime | topic-001 |
| Existing graph/extractor/web seam | topic-002 |
| Parent-owned nesting and identity | topic-003, then topic-007 |
| Services beside ordered operations | topic-004, then topic-007 |
| Static graph or web encoding | topic-005 or topic-006 |
| Acceptance observations and unresolved choices | topic-008 |

Build state: scratch build from the eight topic `INDEX.md` files; target is
this exact file; leaves are the topic indexes and their linked source/clip
files. The only evidence-level unresolved choices are local spelling of the
service collection/icon and whether folded scopes show service names or a
count; see [topic-008](topic-008/INDEX.md). The final report remains capped at
1,800 `o200k_base` tokens and two editorial rounds.
