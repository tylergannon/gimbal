# Workflow graph UI research for Gimble

Captured 2026-09-14. This is a UI case study, not a feature commitment. Gimble workflows remain ordinary Go. A static manifest can describe a possible template; a runtime run supplies actual instances, intervals, outcomes, transcripts, costs, and steering events. The useful Beta view must still work with incomplete extraction and dynamic `Set` keys.

## Four observed products

### Airflow: graph for shape, grid for history

Airflow’s Graph View is a dependency canvas. Its current UI lets a person switch the selected DAG run, see run-specific task states, click a task for metadata/history, and use search/zoom/minimap controls. The companion Grid View puts tasks on rows and runs on columns, with duration bars above the matrix. The docs explicitly describe clicking cells to reach logs or to mark a task instance successful, failed, or cleared, and selecting a run range such as the last 25 runs ([evidence](ui-workflows-evidence.md#L7-L11); `ui-workflows-airflow-graph.png`; `ui-workflows-airflow-grid.png`).

TaskGroups are a deliberately visual hierarchy: the docs call them a UI grouping concept, with caret open/close, while tasks remain in the same DAG ([evidence](ui-workflows-evidence.md#L15-L17)). Mapped tasks are collapsed until a mapped-task panel is opened. Airflow recommends keeping topology relatively stable for dynamic DAGs, a useful warning for any view that tries to compare extracted templates over time.

Transferable idea: keep a shape view and a temporal instance view as linked projections. Airflow’s graph answers “what can depend on what?”; its grid answers “which repeated instance is slow, skipped, or failing?” The grid is especially suited to Gimble’s repeated loop tasks and re-asks. Its limitation for Gimble is that a task/run matrix does not express nested scopes, sessions, supervisor attachments, or a live transcript.

### Dagster: source shape plus run timeline and structured detail

Dagster’s job Overview shows the graph of ops/assets and includes an op-subset search and a highlight field in the captured screenshot (`ui-workflows-dagster-job.png`). The same screenshot keeps an Info/Types inspector beside the canvas, so selection yields stable context without navigating away. The docs describe a job’s Overview as its graph and a Launchpad as a configuration editor; those are authoring/launch surfaces, not evidence that Dagster edits source code ([evidence](ui-workflows-evidence.md#L21-L23)).

Dagster’s run detail view is explicitly observational: a Gantt-like upper pane shows how long each op/asset took, a right pane groups preparing/executing/errored/succeeded steps, and the lower pane lists filterable structured events or raw stdout/stderr. The docs also describe re-executing with the same configuration and grouping related runs ([evidence](ui-workflows-evidence.md#L21-L23); `ui-workflows-dagster-run.png`).

Transferable idea: selection should open an inspector while preserving the graph, and run details should combine duration bars, status buckets, and a filterable event stream. This maps cleanly to Gimble scope/session/turn detail and transcript panes. Dagster’s asset lineage and type views are useful optional overlays, but Gimble should avoid inferring data dependencies from shared values or temporal adjacency.

### Argo Workflows: explicit parallel DAG plus artifact side panel

Argo’s DAG documentation uses a diamond: A runs first, B and C can run in parallel, then D waits for both. This is a compact visual grammar for parallel launch and join, but Argo’s model is Kubernetes/container-oriented and acyclic. Its artifact visualization makes artifacts clickable elements in the workflow DAG; clicking opens a panel, and known images/text/HTML are shown inline in a sandboxed frame ([evidence](ui-workflows-evidence.md#L25-L27)). The captured screenshot shows graph controls (filter, pan/zoom, search), state icons, and a persistent right detail panel (`ui-workflows-argo-graph-report.png`).

Argo’s own first-party UI issue #11106 records a concrete scale failure: nested template refs create repeated names such as `task-1`, making it hard to tell which level a node belongs to. The proposed remedy is to collapse template refs initially and include the invoking template in the display name ([evidence](ui-workflows-evidence.md#L29-L31)). This is direct evidence for preserving qualified keys and explicit collapse boundaries in Gimble.

Transferable idea: represent parallel launch/join as typed relations even when the default renderer keeps them visually light; let selected nodes expose attached artifacts/details in a side panel. Anti-pattern: flatten nested templates into same-named nodes or draw every relation as an undifferentiated edge. Argo’s issue history also shows that faster graph rendering can still harm traceability on wide parent/child graphs; collapse and qualification are usability features, not cosmetic extras.

### n8n: execution list, retry choice, and debug-to-editor handoff

n8n separates the editor canvas from an Executions list. The all-executions view supports filters for workflow, status (Failed, Running, Success, Waiting), start time, and saved custom data. A failed execution can be retried against the currently saved workflow or the original workflow; a prior execution can be copied back into the current editor for debugging. The per-workflow docs distinguish executions (runs of the current workflow) from workflow history (previous workflow versions) ([evidence](ui-workflows-evidence.md#L33-L35)).

n8n’s docs state that multi-branch order depends on branch position on the canvas for workflows created from n8n 1.0, with topmost then leftmost ordering ([evidence](ui-workflows-evidence.md#L37-L39)). That is a useful caution for Gimble: lexical registration order or screen position is not a semantic happens-before relation. n8n has no documented supervision equivalent in these sources; “retry” and “debug in editor” are operator commands, not supervisory attachment semantics.

Transferable idea: make run selection and retry explicit, preserve original-versus-current definition identity, and provide a direct handoff from a failed run to the source/editor context. For Gimble that means linking a runtime node to its generated manifest/source location and labeling the mapping ambiguous when it is ambiguous.

## Patterns to carry into Gimble Beta

1. **Two linked projections.** Default to a shape projection (regions, control edges, containment) and a run projection (expanded observed instances on the wall clock). Airflow’s Graph/Grid split and Dagster’s definition/run split support this. Do not require complete static extraction before runtime observation is usable.

2. **Persistent inspector on selection.** Keep the graph visible while a right-side inspector shows qualified key, source mapping, status, timestamps, values, prompt/result, usage, and transcript links. Dagster and Argo both demonstrate the benefit of a side panel; Airflow demonstrates click-through task metadata/history.

3. **Typed control edges with visual restraint.** Render sequence, branch, repeated/back-edge, parallel launch/join, and containment differently in the model. The first canvas can visually emphasize containment/control flow and keep supervision, fork, steer, and call relations as toggled overlays. This preserves meaning without turning the view into a hairball.

4. **Qualified identity and collapse.** Use full scope keys and source labels in the inspector, and show a collapsed region summary (`3 instances`, `2 succeeded`, `1 running`) before expanding repeats. Argo’s repeated-name issue and Airflow TaskGroups justify this directly. Dynamic keys remain legal; show an expression/unknown marker in the template and the actual runtime key in the run.

5. **Status plus time, then detail.** Use state color/icon and interval bars in the graph/run view; keep exact logs/transcripts in the inspector. Airflow’s grid and Dagster’s timeline show why duration and state together find stuck work faster than either alone. Preserve accessible text labels so color is not the only state cue.

6. **Commands and supervision as real operations, not decoration.** A command node should be visible only where the runtime has an observation boundary that records its interval/outcome; static `exec.Command` discovery alone cannot light it up. A supervisor should remain a session in a supervisory role with a dotted attachment overlay and actual look/steer events shown separately from the planned attachment. None of the reviewed products proves Gimble’s supervision semantics; this is a Gimble-specific design constraint derived from its existing contract.

## Anti-patterns and boundaries

- Treating a static graph as a predicted run. Dynamic branches, loop counts, retries, supervisor looks, and command outcomes are runtime facts.
- Flattening all nested scopes/templates into one node list. It loses containment and creates name collisions; Argo documents this failure mode.
- Making screen/lexical order a dependency. n8n’s canvas-position execution rule is product-specific; Gimble’s Go registration order does not imply sibling execution order.
- Putting every edge into the default view. Call, fork, supervision, steer, and data-relations need inspectable overlays.
- Hiding large/repeated work behind a giant canvas. Keep a run list, focus/search, minimap/fit, collapse/expand, and “show only active/errors” filters. Airflow and Dagster provide concrete controls; Argo’s issue history warns that graph speed alone does not solve traceability.
- Borrowing editor affordances as if they were observation. Dagster Launchpad and n8n’s editor are authoring/debug surfaces; Gimble remains Go-authored and should provide source/manifest navigation, run controls, transcripts, and inspectors.

## Recommended first visual slice

Render the current runtime tree as a hierarchical, left-to-right or top-to-bottom graph of scopes and turns, with repeated instances collapsed by default and a wall-clock strip available for the selected scope. Use a persistent inspector and focus/search controls. Add the static manifest as a template layer when available: declared-but-unrun nodes use a neutral state, unresolved extraction sites are visibly marked, and runtime observations light up only on evidence. Treat explicit command scopes and supervision overlays as separate later slices; do not claim process telemetry or exact source-site identity without runtime proof.
