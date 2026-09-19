# Static parent-owned resource models

## Goal and answer

Issue #284 needs a declared ownership relation: a service belongs to the
scope whose body contains `Service`, including the root, a named child, or an
iteration child. It must remain a static shape, retain the call-site constant
and `Source{File,Line}`, and not become a process, readiness, restart, or
dependency model ([issue #284](../../issue-284.md)). The current seam still
models `Service` as `workflow.Command` alongside `RunCommand` and `Check`,
while recursive `Scope`/`Iterate` bodies already provide the containment
boundary ([workflow/graph.go](../../../../../workflow/graph.go),
[extractor](../../../../../internal/generate/expr.go)).

## Precedent comparison

| Precedent | Root, nesting, repetition | Identity and membership | Fit for Gimble |
|---|---|---|---|
| **Pulumi parent resources** | Every resource defaults under an implicit root `pulumi:pulumi:Stack`; `parent` creates named, arbitrarily deep nesting. The CLI renders the hierarchy as an indented tree. The cited page does not define an iteration scope, but repeated children remain siblings under the same parent. | Child membership is an explicit parent relation. Names and parent paths participate in resource identity; aliases preserve identity when a name/type/parent path changes. | Best shape precedent: ownership is visible as containment and does not require a sequence edge. Gimble should borrow only the static tree/property idea, not Pulumi lifecycle inheritance or deletion behavior. ([pulumi-parent](sources/pulumi-parent.md), [pulumi-aliases](sources/pulumi-aliases.md)) |
| **Kubernetes owner references** | Ownerless objects act as roots; an object's `metadata.ownerReferences` names its owner and carries the owner's UID. Chains can nest, and many dependents can share one owner. Repetition is multiple distinct dependents, not an iteration construct. | Membership is explicit and identity-stable through name + UID, with namespace constraints. | Strong proof that ownership is a relation separate from labels/selectors, but a poor UI/data model to copy: `blockOwnerDeletion`, garbage collection, controllers, and finalizers add runtime lifecycle semantics that issue #284 excludes. ([kubernetes-owner-dependents](sources/kubernetes-owner-dependents.md)) |
| **Terraform modules** | The root module contains named child `module` blocks; nested modules are addressed by a path. `count`/`for_each` create repeated module instances, the closest cited analogue to iteration scopes. | A module's resources are a self-contained group; addresses such as `module.example...` identify nested resources, and `moved` blocks preserve identity across address changes. | Useful for stable scope paths and repeated children, but Terraform also has provisioning order and `depends_on`; those are not part of Gimble's static graph. ([terraform-modules-configuration](sources/terraform-modules-configuration.mdx), [terraform-module-refactoring](sources/terraform-module-refactoring.mdx)) |

## Evidence-bound takeaway

Pulumi is the closest precedent for communicating ownership without implying
that the owned item is an ordered step: a parent container owns a collection
of children, and the tree itself communicates the relation. Terraform adds the
important repeated-scope lesson: identity should be path-like and stable when
the same declaration is shown inside a named/repeated child. Kubernetes shows
why the graph must not reuse a general `ownerReference` concept if it brings
deletion, controller, or readiness meaning with it. None of these precedents
provides Gimble's source location; preserve Gimble's existing `Source{File,Line}`
on both scope and service nodes.

For this topic, the minimal evidence-shaped model is therefore: a scope
container owns `Services []Service`, while its ordered `Body []Operation`
continues to contain ordinary `RunCommand` and `Check` operations. A service
has the existing constant `Name` and `Source`; its membership is the enclosing
scope, not an edge in an execution/dependency graph. An `Iterate` body remains
one static child scope whose service collection is rendered at that declaration
site; runtime item count is not represented.

## Constraints and risks

1. Do not infer owner identity from runtime IDs: Pulumi and Terraform show that
   declaration/path identity is distinct from runtime instances ([pulumi-aliases](sources/pulumi-aliases.md), [terraform-module-refactoring](sources/terraform-module-refactoring.mdx)).
2. Preserve declaration order for `Body`; ownership is a separate collection,
   not a reordered command list ([pulumi-parent](sources/pulumi-parent.md)).
3. Do not add Kubernetes-style lifecycle fields or Terraform-style dependency
   edges; they would violate issue #284's exclusions ([issue #284](../../issue-284.md)).
4. Iteration must remain static: one named iteration scope with its declared
   service property, not one node per runtime item ([terraform-modules-configuration](sources/terraform-modules-configuration.mdx)).

All questions assigned to this topic are resolved by the cited local corpus;
no unresolved question remains here.
