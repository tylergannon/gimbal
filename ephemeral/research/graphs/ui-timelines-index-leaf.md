# UI timelines research index leaf

- Date: 2026-09-14
- Parent context: workflow graph/UI research in `ephemeral/research/graphs`.
- Main report: [ui-timelines-report.md](ui-timelines-report.md)

## Primary sources captured

| Family | Local source note | Evidence |
| --- | --- | --- |
| Perfetto | [ui-timelines-perfetto-ui.md](ui-timelines-perfetto-ui.md) | shared timeline, zoom/pan, area selection, selection drawer: lines 10-25 |
| Perfetto plugins | [ui-timelines-perfetto-plugins.md](ui-timelines-perfetto-plugins.md) | linked selection, separate tracks/placement, cross-track overlays: lines 10-24 |
| Jaeger | [ui-timelines-jaeger-features.md](ui-timelines-jaeger-features.md) | span references as DAG links, service graph caveat, scale: lines 10-22 |
| Jaeger media | [ui-timelines-jaeger-images.md](ui-timelines-jaeger-images.md) plus local PNGs | overview plot/list and nested overlapping waterfall: lines 12-25 |
| Temporal Web UI | [ui-timelines-temporal-web-ui.md](ui-timelines-temporal-web-ui.md) | run projections and Timeline/All/Compact/JSON modes: lines 10-28 |
| Temporal event history | [ui-timelines-temporal-events.md](ui-timelines-temporal-events.md) | commands/events, pending lifecycle, durable history: lines 10-24 |
| React Flow subflows | [ui-timelines-reactflow-subflows.md](ui-timelines-reactflow-subflows.md) | nested containers and cross-boundary edges: lines 10-22 |
| React Flow expand/collapse | [ui-timelines-reactflow-expand-collapse.md](ui-timelines-reactflow-expand-collapse.md) | full hidden graph, visible slice, stable expansion state: lines 10-20 |

## Result

The report recommends a shared identity-preserving model with a static program
projection and an observed wall-clock run projection. It defines four drilldown
levels, global context plus local details, stable live row placement, explicit
overlap, linked selection, reversible collapsed groups, and visible exceptional
paths. See [ui-timelines-report.md:11-32](ui-timelines-report.md) and
[ui-timelines-report.md:118-147](ui-timelines-report.md).

The report also records the hard boundaries: source containment, observed trace
links, session ownership, supervision, steering, and control-flow edges must stay
typed; static layout is not elapsed time; and critical paths require observed
intervals plus real dependency semantics. See
[ui-timelines-report.md:136-153](ui-timelines-report.md).

## Source URLs and retrieval date

- Perfetto UI: https://perfetto.dev/docs/visualization/perfetto-ui (retrieved 2026-09-14)
- Perfetto UI plugins: https://perfetto.dev/docs/contributing/ui-plugins (retrieved 2026-09-14)
- Jaeger features: https://www.jaegertracing.io/docs/2.17/features/ (retrieved 2026-09-14)
- Jaeger docs/screenshots: https://www.jaegertracing.io/docs/2.20/, https://www.jaegertracing.io/img/traces-ss.png, https://www.jaegertracing.io/img/trace-detail-ss.png (retrieved 2026-09-14)
- Temporal Web UI: https://github.com/temporalio/documentation/blob/main/docs/web-ui.mdx (retrieved 2026-09-14)
- Temporal events: https://github.com/temporalio/documentation/blob/main/docs/encyclopedia/workflow/workflow-execution/event.mdx (retrieved 2026-09-14)
- React Flow subflows: https://reactflow.dev/learn/layouting/sub-flows and https://reactflow.dev/examples/grouping/sub-flows (retrieved 2026-09-14)
- React Flow expand/collapse: https://reactflow.dev/examples/layout/expand-collapse (retrieved 2026-09-14)
