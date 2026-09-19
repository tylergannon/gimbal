# Issue #284 semantics and exclusions

Goal: pin down the static relationship that issue #284 must show, without
turning the graph into a runtime service/process model.

## Synthesis

The graph must show declaration ownership at the scope that contains the
`Service` call. The root is the entry graph's `Graph.Body`; a named child is a
`workflow.Scope` with its own `Body`; an iteration is a `workflow.Iterate` with
one static `Body` reused for each runtime item. The current service tests make
the intended lifetime distinction concrete: a service in an enclosing scope
survives two item scopes, while a service declared in an item ends before the
next item ([service_test.go.txt](sources/service_test.go.txt#L18-L75)). The static graph
must preserve those three locations, not flatten services into a global list or
attach them to individual runtime iterations.

Issue #284 requires every service to retain its constant call-site name and
source location, and requires ordinary `RunCommand` and `Check` operations to
remain distinguishable ([issue-284.md](sources/issue-284.md#L9-L35)). The
existing contract gives the naming rules: `Service` and `RunCommand` take a
constant `name`; `Check` uses its constant context key. Source is already
module-relative file plus line. The API records that names are call-site
constants and that `Check` keys are explicit rather than invented
([gimble-api.md](sources/gimble-api.md#L972-L986)); the service implementation
also calls the name the service's constant graph/run name
([service.go.txt](sources/service.go.txt#L23-L37)). A proposed representation must not
rename these fields or replace source anchors with runtime IDs.

The #276 contract is a boundary, not new graph data. `Service` starts through
zsh, returns after start rather than readiness, and belongs to the current
scope; readiness remains ordinary workflow code or `Check`. Enclosing-scope
services span iterations, item-local services do not. Unexpected exit,
SIGTERM/SIGKILL cleanup, output, and failure propagation are runtime behavior
([gimble-api.md](sources/gimble-api.md#L988-L1010)); the original issue also
explicitly defers restarts, readiness frameworks, dependency graphs,
persistent daemons, and launchd integration ([issue-276.md](sources/issue-276.md#L19-L39)).

Therefore the graph must not contain process instances, PIDs, readiness or
health, restart state, process trees, shutdown edges, or a general dependency
graph. It represents declared ownership only. A service may be visually
adjacent to the scope's ordered operations, but adjacency must not mean
execution order or lifecycle sequencing.

## Questions answered

- Root, named-child, and iteration ownership: the declaring body's static
  scope owns the service; iteration is one declaration body, not per-item
  service nodes.
- Constant names and locations: preserve `name`/`Check` key and `Source{File,
  Line}` from the call site.
- Exclusions: retain the #276 runtime contract and issue #284's explicit
  exclusions; do not add graph/runtime semantics.

## Evidence gap

The local contract does not choose the final schema spelling (for example,
`Scope.Services` versus a distinct non-step operation variant). That is the
implementation decision left for issue #284; the evidence does settle that it
must be scope-owned, source-anchored, and outside the ordered operation list.
