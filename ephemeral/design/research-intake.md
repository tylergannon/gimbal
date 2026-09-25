# Supplemental research: what is worth carrying forward

These two agent reports are idea sources, not requirements or authoritative
descriptions of Gimbal. They do not change the feature brief or select a
winning concept.

## Sources and assessment

- [Gemini shared report](https://share.gemini.google/sOiIq8xW1nOr),
  “Architectural Interface Concepts for Agent Workflow Observability:
  Designing Gimbal.” Read in the rendered shared page; raw HTML downloaded
  locally to `/private/tmp/gimbal-design-gemini-share.html`. A direct
  HTML-to-Markdown conversion captured only the page shell, not the report.
  Its strongest additional lead is AWS Step Functions' execution browser.
  Its generated illustrations are concepts, not screenshots of real products.
- `~/src/inbox/Claude Research.md`, read in full and left unchanged.
  Broader and more explicit about some evidence gaps; useful leads include
  Perfetto navigation and Airflow's task-local human-input panel. Much of
  its three-concept recommendation repeats the existing reference board.

The following subset was checked against primary sources. Everything below
is a design option or a correction, not an added acceptance criterion.

## Worth exploring

### Make the selected execution instance unmistakable

AWS documents linked graph/table selection, nested iteration browsing,
and a stable selected-step detail panel. Its screenshot shows individually
expandable iterations with distinct outcomes and durations. This is a
concrete answer to an existing Gimbal design problem: selecting the second
execution of a repeated call without mistaking it for the definition node.
Borrow the selection affordance, not AWS's state-machine semantics.
[AWS execution details](https://docs.aws.amazon.com/step-functions/latest/dg/concepts-view-execution-details.html).

### Keep the answer beside the work that asked for it

Airflow's published screenshot places a selected task's Required Action
form beside the graph. Its documentation also describes links directly to
the response UI. That reinforces Gimbal's already-required task-local
interview interaction; it does not justify adding approvals, assigned
responders, paging, or a new global inbox.
[Airflow HITL tutorial](https://airflow.apache.org/docs/apache-airflow/stable/tutorial/hitl.html).

### Offer a readable summary with access to underlying events

Temporal groups related lifecycle events into one duration row while
retaining event markers. This is a useful precedent for reading an agent
turn or command without giving every low-level event equal visual weight.
The report's suggested multiple history lenses are an option, not a
requirement to add more tabs.
[Temporal timeline design](https://temporal.io/blog/lets-visualize-a-workflow).

### Preserve orientation in long histories

Perfetto documents pinned tracks, fit-to-selection, next/previous event
navigation, and time/track area selection. These offer small, concrete
interaction ideas for inspecting concurrent sessions. Try them only where
they improve the supplied scenarios; there is no mandate for a profiler,
aggregate flamegraphs, or a trace-query engine.
[Perfetto UI documentation](https://perfetto.dev/docs/visualization/perfetto-ui).

## Corrections and ideas not adopted

- Gemini incorrectly describes Gimbal as merely a visual layer that does
  not execute workflows. Gimbal runs the ordinary Go workflow; the web
  interface observes and interacts with that run.
- A pending interview does not mean the whole run is paused. Other agents
  can continue. Neither freeze the timeline nor relabel all work as waiting
  because one question needs an answer.
- A source-derived workflow graph and a runtime-aggregated graph are not
  interchangeable. Grouping or zooming a display does not require execution
  checkpoints; replaying or changing execution is a different capability.
- Checkpoint replay, editing arbitrary runtime state, bypassing nodes,
  goroutine/deadlock diagnostics, provenance tracing, cohort analytics,
  assignees, and notification systems remain outside this assignment.
- Claims that Gimbal needs WebSockets, a different backend, a particular
  canvas library, or a prescribed rendering implementation are not design
  requirements. The reports did not establish those engineering needs.
- “Start with the inbox,” fixed split panes, and usage thresholds such as
  30% graph usage or 50 concurrent runs are proposals or invented heuristics,
  not evidence that decides our layout or scale targets.
- Broad claims about which products alone support intervention, deprecated
  HumanLayer implementation details, and historical pricing were not
  established by this review. Do not repeat them as product facts.

## Effect on the handoff

Added two inspected product screenshots to the reference board: AWS for
repeated-instance navigation and Airflow for input at the selected task.
Kept the existing three concept directions and all required features.
Use the checked ideas to enrich those concepts, not to combine every
suggestion into one screen. Send the curated packet; these unfiltered
reports need not become part of Fable's instructions.
