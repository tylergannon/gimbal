# Scope-owned services: prior art for #284

Represent **declaration ownership**: the root owns root services, a named child owns its declarations, and an iteration owns one static declaration set in its reusable body. Keep constant names and source locations. [Issue #284](corpus/topic-001/sources/issue-284.md) requests this before execution; [#276](corpus/topic-001/sources/issue-276.md) and the [settled service contract](corpus/topic-001/sources/gimble-api.md#L988) establish the lifetime boundary without requiring lifetime data in the graph.

Explicitly exclude runtime process instances, PIDs, readiness, health, restarts, process trees, general dependency graphs, and changes to service runtime semantics.

Six precedents supply complementary pieces. The proposed Services UI below is a synthesis, not a claim that these products display that exact UI.

| Precedent | Ownership, nesting, identity | Non-step distinction and useful limit |
| --- | --- | --- |
| [Pulumi `parent`](corpus/topic-007/sources/pulumi-parent.md#L7) | Implicit root Stack; explicit parents allow multiple nested levels. CLI indentation shows named membership. | Best ownership cue: containment. Borrow neither inherited options nor lifecycle behavior. |
| [Kubernetes `ownerReferences`](corpus/topic-007/sources/kubernetes-owner-references.md#L9) | Dependents reference owner name/UID; ownership differs from labels. References can form chains, without a single mandatory root. | Ownership is metadata, not sequence. Useful distinction; UID/controller/deletion machinery is unnecessary here. |
| [GitHub Actions services](corpus/topic-007/sources/github-actions-docker-services.md#L16) | Named services belong to a job; this is one ownership level, not arbitrary nested scopes. | Closest structural precedent: `jobs.<job>.services` beside steps. Does not establish service declaration order or Gimble source identity. |
| [Compose services](corpus/topic-007/sources/docker-compose-services.md#L8) | Top-level map keys name application resources; no nested scope ownership in this model. | Resource definitions are separate from commands. Supports a named collection, not iteration or dependency edges. |
| [Graphviz clusters](corpus/topic-005/sources/graphviz-dot-language.html) | Nested cluster membership yields bounding boxes; IDs identify members. Clusters are a layout convention. | Membership and edge statements are separate. Borrow boxes, not service connectors. |
| [React Flow sub-flows](corpus/topic-006/sources/reactflow-sub-flows.html) | `parentId` gives relative child placement; `extent: 'parent'` can constrain it. Roots lack parents. | Contained items need no edges. This is positioning, not DOM nesting or semantic ownership; do not import editing/reparenting. |

None of these cited models supplies Gimble’s constant call-site name plus module-relative file/line contract. Retain that contract directly rather than adopting foreign IDs. Iteration semantics come from Gimble’s static body contract, not these products’ resource instances.

**Recommendation.** Add a `Services` collection to the existing static owner records: `Graph` for root, `Scope` for named children, and `Iterate` for its declared item body. Each entry contains `Name` and the existing embedded `Source{File, Line}`; it is not an `Operation`. Keep `Body []Operation` for ordered operations. Preserve service source order for stable display only: placement above operations does not mean the service starts before them. No new ownership edges or synthetic root operation are needed. The [graph model](corpus/topic-002/sources/workflow-graph.go.txt#L21) already defines these containers and a static iteration body.

Render a labelled **Services** list inside the owning scope boundary, above its ordered body, including a clearly labelled root area. Use a service badge/icon plus constant name, no sequence connector and no status pip. Selection exposes the unchanged `file:line`. Wrap or stack entries within the boundary on narrow layouts. This combines Pulumi containment with Actions’ sibling resource collection; it needs neither a parallel execution lane nor a new graph library. [UI evidence](corpus/topic-006/clips/web-rendering-evidence.md) supports containment, but does not prescribe this layout.

**Five implementation constraints and risks:**

1. **Extract ownership, not a renamed command.** `internal/generate/expr.go` currently emits `workflow.Command` for all three calls; retain constant extraction and `e.at(call.Pos())`, but collect `Service` on its actual owner. `Repeat` and `Condition` create no scope; their bodies must not become owners. A listed conditional declaration does not assert it starts. Existing `GroupChild` and `PromiseLoop` task bodies do represent scopes, so audit those same ownership boundaries. See [extractor](corpus/topic-002/sources/generator-expr.go.txt#L202) and [scope definitions](corpus/topic-002/sources/workflow-graph.go.txt#L106).
2. **Keep generated data consistent.** Update `workflow/graph.go` and `internal/generate/source.go`, then regenerate codecs and TypeScript. Preserve existing source field projection: embedded `Source` becomes `file`/`line`, not an invented nested service `source` object. See [Go emission](corpus/topic-002/sources/generator-source.go.txt#L100) and [generated types](corpus/topic-002/sources/workflow-types.ts.txt).
3. **Separate static selection from runtime lookup.** `web/src/lib/run/layout.ts` currently routes commands into sequence layout and runtime command lookup; `Map.svelte` selection depends on snapshot scopes. Services need selection by declaring scope/name/source even without a service command row. Do not multiply iteration declarations when changing the displayed runtime item. See [layout](corpus/topic-002/sources/web-layout.ts.txt#L209) and [selection](corpus/topic-002/sources/web-Map.svelte.txt#L66).
4. **Make the distinction accessible.** Retain button activation, accessible names, `aria-pressed`, and visible focus from [Node.svelte](corpus/topic-002/sources/web-Node.svelte.txt#L35). Label the group “Services”; color/icon alone is insufficient. Full names and source anchors must remain available when text truncates or scopes fold.
5. **Replace the old assertion, then observe the result.** [Extraction tests](corpus/topic-002/sources/generator-graph_test.go.txt#L14) explicitly expect a service command. Replace that expectation with ownership assertions; extend [layout tests](corpus/topic-002/sources/web-layout.test.ts.txt) and [Map tests](corpus/topic-002/sources/web-Map.svelte.test.ts.txt) to prove services never enter the operation sequence. Green tests alone do not show readable ownership.

Acceptance pairs generated JSON with the rendered map, using the [local evidence cases](corpus/topic-008/INDEX.md):

- Root `Service("root-db")`: only root `Services` contains it; the root area visibly owns it.
- `Scope("backend")` declaring `Service("api")`: only that child contains it; the browser places it inside `backend`.
- An iteration declaring `Service("fixture")`: one declaration in the static item body; no per-item service copies in graph data or resource presentation.
- Surrounding `RunCommand("build")` and `Check("tests")`: unchanged relative order and connectors; services absent from `Body` and its connectors.

For every case, compare the exact constant name and source file/line through extraction, serialization, selection, and folding; inspect declarations before their runtime command rows exist. The remaining product choice is whether a folded scope summarizes service names or a count. The [local UI evidence](corpus/topic-006/INDEX.md) cannot settle that; either must reveal full identity when opened.
