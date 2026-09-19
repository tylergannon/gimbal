# Execution-interface research

Three references were selected for contrast: execution detail/history, CI dependency graph, and distributed-trace waterfall. Image URLs below are public assets inspected directly in Chrome on 2026-09-17; they are not local screenshots.

## 1. Temporal Web UI — execution detail plus event history

Source: [Temporal Ruby tutorial, “View the Workflow Execution”](https://learn.temporal.io/getting_started/ruby/first_program_in_ruby/) (the page explicitly describes current/past execution details, input/return values, an activity timeline, and a detailed event table).

![Temporal execution detail](https://learn.temporal.io/assets/images/web-ui-detail-page-ebc2624dd8c608be9a818eb1e13e7d9b.png)

Image: [public Temporal detail asset](https://learn.temporal.io/assets/images/web-ui-detail-page-ebc2624dd8c608be9a818eb1e13e7d9b.png).

Observed visually: a completed-status badge and run title dominate the header; start/end/duration, run ID, workflow type, task queue, and history count are compact metadata fields. Tabs expose History, Relationships, Workers, Pending Activities, Call Stack, Queries, and Metadata. Input and Result are side-by-side panels. Event History combines a horizontal time ruler with colored activity bars and event markers, followed by a table whose rows are individually timestamped.

Documented interaction: the source says users can open current or past executions, inspect timeline/input/return values, and use the event-history table to inspect every event plus activity inputs/results. The tutorial also documents failure/retry scenarios. These are product claims from the source, not inferred from the pixels.

Gimble borrow: make run status and identity persistent above every detail view; pair a compact overview timeline with a precise event list; retain stable tabs/locations for history, pending work, and metadata. This model is strong for historical review and repeated attempts. It does not itself provide a source-definition graph or a safe distinction between workflow topology and runtime instances, so Gimble should not copy the timeline as its map.

Tradeoff: the dense metadata-and-tabs approach preserves context while drilling down, but the activity timeline is still a single execution’s chronological projection; parallel branches and repeated calls need explicit grouping/iteration labels rather than relying on the horizontal order.

## 2. GitHub Actions — dependency graph as live run overview

Source: [GitHub Docs: Using the visualization graph](https://docs.github.com/en/actions/how-tos/monitor-workflows/use-the-visualization-graph). GitHub states that every workflow run generates a real-time graph; jobs are nodes, status icons sit beside job names, dependency lines connect jobs, and clicking a job opens its log.

![GitHub Actions workflow graph](https://docs.github.com/assets/cb-63715/images/help/actions/workflow-graph.png)

Image: [public GitHub Actions graph asset](https://docs.github.com/assets/cb-63715/images/help/actions/workflow-graph.png).

Observed visually: the canvas lays out job cards left-to-right with curved dependency connectors. Cards show a status icon, job name, and elapsed duration. A matrix job is visually grouped into one larger card with “3/3 jobs are completed” and “Show all jobs.” The canvas has explicit fit-to-view, minus, and plus controls. The screenshot shows completed, in-progress, and not-yet-completed states at once.

Documented interaction: the source says the graph is real-time, lines mean dependencies, and selecting a job opens its log. The matrix grouping and control affordances are visible evidence; the source does not claim that the graph is a source-code map.

Gimble borrow: use a fit-to-view graph as the run’s orientation layer; show status and duration directly on nodes; represent fan-out/matrix-like repetition as an expandable aggregate rather than flooding overview scale; connect graph selection to the corresponding detail/log location. Keep zoom controls and a “return to whole run” action obvious.

Tradeoff: a dependency graph makes concurrency legible at a glance, but it can imply a false total order if edge routing or left-to-right placement is read as chronology. Gimble should label edges as dependencies/containment and use runtime timestamps in the detail pane for actual order. Matrix aggregation is useful for many repeated instances, but selecting a specific iteration must remain possible.

## 3. Jaeger — distributed trace waterfall and tree drilldown

Source: [Jaeger documentation introduction](https://www.jaegertracing.io/docs/1.76/) and [Jaeger features](https://www.jaegertracing.io/docs/2.21/features/). The docs identify the trace-detail screenshots and describe trace troubleshooting, topology graphs, and support for large traces. The screenshot is an official Jaeger documentation asset.

![Jaeger trace detail waterfall](https://www.jaegertracing.io/img/trace-detail-ss.png)

Image: [public Jaeger trace-detail asset](https://www.jaegertracing.io/img/trace-detail-ss.png).

Observed visually: the view has a service/operation tree at left and aligned horizontal span bars at right over a shared millisecond ruler. Nested indentation shows parent/child structure; colors distinguish service/span groups. The top strip summarizes the trace title, duration, service count, depth, and span count. Long and short spans are simultaneously visible, and many repeated Redis calls appear as individual bars.

Documented interaction: Jaeger describes trace detail as a way to find bottlenecks/root causes; its current feature docs say traces are DAGs rather than only trees and mention large-trace rendering, topology graphs, and a critical-path option. The exact collapse/zoom behavior is not asserted here unless visible in the asset or separately documented.

Gimble borrow: use a synchronized hierarchy + time-axis detail for concurrency, latency, and repeated calls; allow a selected row/span to open a side/detail surface while preserving the global ruler; show counts/depth/total span-like summaries before drilling into hundreds of events. This is a useful history lens alongside, not instead of, a definition graph.

Tradeoff: waterfall density makes timing and overlap clear but becomes hard to navigate for very large histories; Jaeger’s own docs and issue history acknowledge scale concerns. Gimble should collapse/aggregate repeated iterations at overview scale, support keyboard or semantic navigation in addition to dragging, and avoid moving the user’s selection/viewport merely because live events arrive. A trace tree also encodes parentage, which is not automatically the same as Gimble’s scopes, sessions, supervisor links, or source call sites.

## Three design directions for Gimble

1. **Execution dossier (Temporal-inspired):** run list → stable run header → tabs for history/pending/metadata → event rows and transcript detail. Best for historical review, interventions, and “what exactly happened?”
2. **Topology canvas (GitHub Actions-inspired):** fit-to-view workflow/run map with status-bearing nodes, explicit dependency/containment edges, aggregate repeated branches, and graph-to-history selection. Best for finding parallel work and orienting in a large run.
3. **Time-and-concurrency lens (Jaeger-inspired):** hierarchy on the left, shared time axis on the right, collapsible repeated instances, and a persistent selected detail panel. Best for overlap, latency, retries, and long histories.

Recommended composition: make the topology canvas the overview requested by the handoff, then open the execution dossier or time-and-concurrency lens for the selected region. Do not merge the three metaphors into one overloaded visualization: workflow definition, run instances, and transcript/event evidence should remain visibly distinct.

## Access limits and evidence boundary

The official Temporal and Jaeger documentation pages were readable, and their image assets were directly inspected. GitHub’s official documentation and graph asset were readable and directly inspected. No authenticated product account, private run, or local production instance was used. The observations above are limited to these public images and the linked documentation; interaction details not visible or documented are intentionally left as inference or omitted. No remote media was downloaded into the repository.
