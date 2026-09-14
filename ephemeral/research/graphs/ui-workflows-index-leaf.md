# Workflow UI research index leaf

## Question

What can Gimble borrow from workflow graph UIs while replacing the tree/scope inventory view with a full workflow shape and runtime drilldown?

## Answer

Borrow linked shape/run projections, persistent selection inspectors, collapse/qualification for repeated hierarchy, typed control relations, and status-plus-duration cues. Keep static extraction incomplete and honest; runtime events are the source of truth for actual instances. Do not copy editor semantics into Gimble’s ordinary-Go authoring model.

## Local evidence

- Airflow Graph/Grid behavior, task actions, run switching, and drilldown: [ui-workflows-evidence.md:9](ui-workflows-evidence.md#L9); screenshots [ui-workflows-airflow-graph.png](ui-workflows-airflow-graph.png) and [ui-workflows-airflow-grid.png](ui-workflows-airflow-grid.png).
- Airflow visual hierarchy (TaskGroups), mapped tasks, and dynamic-topology guidance: [ui-workflows-evidence.md:15](ui-workflows-evidence.md#L15).
- Dagster definition graph and job tabs plus run timeline/logs/re-execute: [ui-workflows-evidence.md:21](ui-workflows-evidence.md#L21); screenshots [ui-workflows-dagster-job.png](ui-workflows-dagster-job.png) and [ui-workflows-dagster-run.png](ui-workflows-dagster-run.png).
- Argo artifact node selection and side panel: [ui-workflows-evidence.md:27](ui-workflows-evidence.md#L27); screenshot [ui-workflows-argo-graph-report.png](ui-workflows-argo-graph-report.png).
- Argo nested-template naming collision and collapse/qualification remedy: [ui-workflows-evidence.md:33](ui-workflows-evidence.md#L33).
- n8n execution filters, retry choices, history distinction, and debug handoff: [ui-workflows-evidence.md:39](ui-workflows-evidence.md#L39).
- n8n canvas-position branch ordering (product-specific caution): [ui-workflows-evidence.md:45](ui-workflows-evidence.md#L45).

## Gimble mapping

The local product context already names the required runtime surfaces: scope containment, session ownership, turn transcripts, usage/cost, steer/interrupt/cancel, and a later static template. The reviewed products support making the graph the navigation spine while keeping details in an inspector and logs/transcripts below or beside it. The research does not establish that any product models Gimble’s supervisor role, Go static extraction, or dynamic `Set` key semantics; those remain Gimble-specific contracts.

## Source manifest

See [ui-workflows-sources.md](ui-workflows-sources.md) for every relied-on URL, capture date, and downloaded artifact.
