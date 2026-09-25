# Cross-precedent synthesis

## Answers to assigned questions

The four concrete precedents below are the complete comparison set. They cover
parent ownership (Pulumi and Kubernetes), scope-owned non-step resources
(GitHub Actions and Compose), stable names/references, and nested membership.
None is being imported as a lifecycle model: the transferable lesson is only
static containment and labeling. In particular, reject Pulumi inheritance,
Kubernetes garbage collection, GitHub teardown, Compose health checks and
`depends_on`, and any process/readiness/restart meaning; `depends_on` is cited
only as evidence that dependency edges are a separate concept.

The single minimal representation is a `Services []Service` collection on the
root graph and each named/iteration scope, with `{Source, Name}` entries. The
single web representation is a labelled, visually distinct Services strip/list
inside the owning scope container, while ordinary `RunCommand` and `Check`
remain the existing ordered body nodes and connectors. This answers ownership,
nesting, non-step distinction, identity/source location, and applicability
without a general graph.

The five numbered constraints below are the requested implementation risks,
each tied to a local current seam or precedent source.

## Compact comparison

| Precedent | Ownership/nesting and identity | Non-step distinction | Fit for Gimbal |
| --- | --- | --- | --- |
| Pulumi `parent` | Every resource has an explicit parent or implicit root stack; names and arbitrary nested parent/child levels are shown as a tree ([source](sources/pulumi-parent.md):7-18,106-118). | Parent hierarchy is separate from ordered update output. | Best ownership model; omit inherited lifecycle behavior. |
| Kubernetes `ownerReferences` | A dependent stores owner name plus UID in metadata; membership is distinct from labels ([source](sources/kubernetes-owner-references.md):9-31). | Ownership is metadata, not a step sequence. | Good stable-reference precedent; omit garbage collection/finalizers. |
| GitHub Actions service containers | Named `services` belong to one job and are destroyed with it; steps use the service label ([source](sources/github-actions-docker-services.md):16-24,70-96). | Services are a job property, beside—not among—the steps. | Best service-as-scope-property precedent. |
| Docker Compose `services` | A top-level named map contains application resources; `depends_on` is an optional separate relation ([source](sources/docker-compose-services.md):8-18,30-80,412-477). | Service definitions are not command order. | Good shape and explicit warning not to add edges. |

## Recommendation

Use one minimal static shape: add a scope-owned `Services []Service` collection
to each scope-bearing graph node (`Graph` root, named `Scope`, and `Iterate`),
where each service is `{Source, Name}`. Keep ordinary `RunCommand` and `Check`
in the existing ordered `Body`; do not add a service operation, edge, PID,
readiness, or dependency relation. The collection preserves declaration order
only for stable display, not execution order. An iteration owns the service in
its declared static body; it does not expand runtime item instances.

Render each scope as its existing container with a small labelled “Services”
resource strip/list, using a distinct non-step icon/style. Render the existing
command nodes and vertical sequence connections unchanged below/alongside it.
Pulumi's nested container/tree is the ownership cue; GitHub Actions' named
service list is the non-step cue. Keep `Name` and `Source.File:Line` selectable
in the detail view. This is a scope-owned list, not a general graph.

## Constraints and risks

1. Preserve the current constant name and module-relative source location:
   extraction already computes `Source` in [generate-graph.go.txt](../topic-008/sources/generate-graph.go.txt:130-138),
   and generated output writes it in [generate-source.go.txt](../topic-008/sources/generate-source.go.txt:79-99).
2. Do not put services in `Body`: `Command` currently conflates Service with
   RunCommand/Check ([workflow-graph.go.txt](../topic-008/sources/workflow-graph.go.txt):93-99), while
   `Body` is explicitly source order ([workflow-graph.go.txt](../topic-008/sources/workflow-graph.go.txt):21-30).
3. Do not infer runtime iteration instances: `Iterate` records only its static
   body ([workflow-graph.go.txt](../topic-008/sources/workflow-graph.go.txt):129-136). Repeating the
   service visually per item would suggest runtime/process semantics.
4. Update generated TypeScript and layout together: the current map only makes
   command/agent/interview nodes ([run-layout.ts.txt](../topic-008/sources/run-layout.ts.txt):209-245)
   and connects measured blocks in sequence ([run-layout.ts.txt](../topic-008/sources/run-layout.ts.txt):322-330).
5. Keep the new affordance accessible and source-addressable: reuse the
   existing labelled, focus-visible selectable-node pattern as appropriate
   ([run-node.svelte.txt](../topic-008/sources/run-node.svelte.txt):38-86), but do not hide identity
   in color or an icon alone.

Questions addressed explicitly: (1) the table compares four precedents on
ownership, nesting, non-step distinction, identity, and applicability; (2) the
preceding paragraph selects one static graph and web representation; (3) the
five numbered constraints state the implementation risks with local citations.
Longer evidence: [comparison clip](clips/comparison-evidence.md).
