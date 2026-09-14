# UI workflow research sources

Captured 2026-09-14 (UTC) from official documentation or first-party project repositories. Files are flat and prefixed `ui-workflows-` so the report can be reviewed offline.

| Local file | Source URL | What was used |
| --- | --- | --- |
| `ui-workflows-airflow-ui.html` | https://airflow.apache.org/docs/apache-airflow/stable/ui.html | Airflow 3 UI overview: Graph/Grid behavior, selection, task actions, run switching, tabs. |
| `ui-workflows-airflow-graph.png` | https://airflow.apache.org/docs/apache-airflow/stable/_images/dag_overview_graph.png | Actual Airflow Graph screenshot: zoom/pan canvas, search, trigger, minimap, branching layout. |
| `ui-workflows-airflow-grid.png` | https://airflow.apache.org/docs/apache-airflow/stable/_images/dag_overview_grid.png | Actual Airflow Grid screenshot: task-by-run matrix, duration bars, state colors. |
| `ui-workflows-airflow-dags.html` | https://airflow.apache.org/docs/apache-airflow/2.6.2/core-concepts/dags.html | TaskGroups as UI-only hierarchy, mapped tasks, dynamic-DAG topology guidance. |
| `ui-workflows-dagster-ui.html` | https://master.dagster.dagster-docs.io/concepts/webserver/ui | Dagster job/run UI, graph subset search/highlight, Gantt, logs, re-execute, related runs. |
| `ui-workflows-dagster-job.png` | https://master.dagster.dagster-docs.io/images/concepts/webserver/job-definition-with-ops.png | Actual Dagster job definition screenshot: graph canvas plus Info/Types inspector. |
| `ui-workflows-dagster-run.png` | https://master.dagster.dagster-docs.io/images/concepts/webserver/run-details.png | Actual Dagster run screenshot: timeline, state buckets, step filter, event/stdout/stderr panes. |
| `ui-workflows-dagster-graphs.html` | https://master.dagster.dagster-docs.io/concepts/ops-jobs-graphs/graphs | Graphs contain ops/subgraphs; supports conditional branching, fixed fan-in, dynamic outputs. |
| `ui-workflows-argo-dag.html` | https://argoproj.github.io/argo-workflows/walk-through/dag/ | Argo DAG contract: dependency graph, diamond parallelism and join. |
| `ui-workflows-argo-artifacts.html` | https://argo-workflows.readthedocs.io/en/latest/artifact-visualization/ | Actual Argo artifact-in-DAG interaction and sandboxed inline panel behavior. |
| `ui-workflows-argo-graph-report.png` | https://argo-workflows.readthedocs.io/en/latest/assets/graph-report.png | Actual Argo UI screenshot: toolbar, search, status icons, graph selection and right detail panel. |
| `ui-workflows-argo-test-report.png` | https://argo-workflows.readthedocs.io/en/latest/assets/test-report.png | Actual Argo UI screenshot: second artifact/report state for comparison. |
| `ui-workflows-argo-ui-issue-fresh.html` | https://github.com/argoproj/argo-workflows/issues/11106 | First-party UI issue documenting name collisions in nested template graphs and a collapse/display-name remedy. |
| `ui-workflows-n8n-view-all.md` | https://docs.n8n.io/build/understand-workflows/understand-executions/view-all-executions.md | n8n all-execution filters, statuses, retry choices, and history deletion behavior. |
| `ui-workflows-n8n-view-single.md` | https://docs.n8n.io/build/understand-workflows/understand-executions/view-executions-for-a-single-workflow.md | Per-workflow executions, retry, and separation from workflow version history. |
| `ui-workflows-n8n-debug.md` | https://docs.n8n.io/build/understand-workflows/understand-executions/debug-executions.md | Debug failed runs by copying execution data into the editor; pins first-node data. |
| `ui-workflows-n8n-order.md` | https://docs.n8n.io/build/flow-logic/understand-execution-order.md | Multi-branch execution order and canvas-position rule. |
| `ui-workflows-n8n-sitemap.md` | https://docs.n8n.io/sitemap.md | Current canonical n8n paths; old `/workflows/executions/...` URL redirected to a missing page. |

The n8n pages currently document behavior primarily as text; no useful full workflow screenshot was available on the relied-on execution pages. The report therefore makes no screenshot-based n8n claims.
