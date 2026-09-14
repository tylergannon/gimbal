# Extracted evidence with stable local line references

Captured 2026-09-14 UTC from the downloaded first-party pages listed in `ui-workflows-sources.md`. These are short extracts/paraphrases for citation; the complete source pages remain beside them.

## Airflow UI (`ui-workflows-airflow-ui.html`)

Source: https://airflow.apache.org/docs/apache-airflow/stable/ui.html

The Dag Details page has tabs for Overview, Grid View, Graph View, Runs, Tasks, Events, Code, and Details. Grid View is described as a primary interface for inspecting runs and task states; task cells can open logs or mark instances successful, failed, or cleared; task names can be filtered; run ranges such as the last 25 runs can be selected. Each row is a task and each column a run. Graph View shows task connections, order, branching/retries, and run-specific state; clicking a task opens metadata/history and a dropdown switches runs.

## Airflow DAG visualization (`ui-workflows-airflow-dags.html`)

Source: https://airflow.apache.org/docs/apache-airflow/2.6.2/core-concepts/dags.html

Airflow advises keeping dynamic DAG topology relatively stable. TaskGroups organize tasks hierarchically in Graph view and are a UI grouping concept; a caret opens/closes the group while tasks remain in the original DAG. Mapped tasks have a dedicated mapped-task detail panel.

## Dagster UI (`ui-workflows-dagster-ui.html`)

Source: https://master.dagster.dagster-docs.io/concepts/webserver/ui

The Runs page can filter by job, run ID, execution status, or tag. Run details show timing, errors, and logs; the upper-left pane is a Gantt chart for asset/op duration, and the lower pane has filterable events/logs. Users can view structured and raw compute logs, re-execute with the same configuration, and see related runs grouped together. A Job Overview shows the graph of assets/ops; Launchpad is a configuration editor; Runs opens run details.

## Argo artifact visualization (`ui-workflows-argo-artifacts.html`)

Source: https://argo-workflows.readthedocs.io/en/latest/artifact-visualization/

Artifacts appear as elements in the workflow DAG and are clickable. Clicking an artifact opens a panel. Known image, text, and HTML files render inline in a sandboxed iframe; JSON is syntax-highlighted.

## Argo UI issue #11106 (`ui-workflows-argo-ui-issue-fresh.html`)

Source: https://github.com/argoproj/argo-workflows/issues/11106

The issue reports that nested template references produce repeated names such as `task-1`, making it difficult to know which workflow/template level a node belongs to. The requested UI remedy is to collapse template references initially and include the invoking template in the displayed name.

## n8n execution docs (`ui-workflows-n8n-view-all.md`, `ui-workflows-n8n-view-single.md`, `ui-workflows-n8n-debug.md`)

Sources: https://docs.n8n.io/build/understand-workflows/understand-executions/view-all-executions.md; https://docs.n8n.io/build/understand-workflows/understand-executions/view-executions-for-a-single-workflow.md; https://docs.n8n.io/build/understand-workflows/understand-executions/debug-executions.md

The all-executions list filters by workflow, Failed/Running/Success/Waiting status, start time, and saved custom data. Failed executions offer retry with the currently saved workflow or the original workflow. Per-workflow docs distinguish executions (runs) from workflow history (previous versions). Debugging copies a prior execution into the current workflow and pins data in the first node.

## n8n branch ordering (`ui-workflows-n8n-order.md`)

Source: https://docs.n8n.io/build/flow-logic/understand-execution-order.md

For workflows created from n8n 1.0, branches execute one at a time in canvas order: topmost first, then leftmost when heights match. This is a product rule and should not be generalized to Gimble’s Go registration order.
