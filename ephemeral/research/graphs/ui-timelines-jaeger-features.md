# Jaeger UI source note

- Source: https://www.jaegertracing.io/docs/2.17/features/
- Source type: official Jaeger documentation
- Retrieved: 2026-09-14

## Relevant observations

1. Jaeger explicitly models traces as directed acyclic graphs, rather than only
   trees, using span references. This keeps non-parent links separate from
   ordinary nesting.
2. The UI supports service graphs in two modes: a broad System Architecture
   graph and a focal-service Deep Dependency Graph. The documentation warns
   that the broad graph's one-hop edges do not prove that one trace contains the
   whole apparent chain.
3. The documented UI is designed to handle very large traces (the feature note
   cites experiments with tens of thousands of spans), which motivates keeping
   overview density and detail interaction separate.

## Transferable UI pattern

Maintain different relation types in the data model and make the default view
show the relation useful for the current question. A graph edge that conveys a
reference or observed link must not be rendered as a parent/child scope edge.
Use a focused subgraph or lane when the full run is too dense, while retaining a
way to restore the surrounding trace context.

## Limits for Gimble

Jaeger spans have trace timestamps and durations. Gimble's generated workflow
shape may contain possible operations, unresolved branches, and dynamic Set keys;
those are not observed spans. Do not claim a dependency, critical path, or exact
duration from static edges or from the width of a visual box.
