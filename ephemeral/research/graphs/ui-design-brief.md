# Gimble workflow and run viewer — design brief

2026-09-14. Recommendation for the design agent, informed by three independent gpt-5.6-luna research passes. This proposes a UI direction, not a new workflow API or a requirement to finish whole-program analysis for Beta.

## The central design decision

Make the workflow map the main surface. Keep the hierarchy as an optional navigator. Give runs a linked timeline, and let selection open details without replacing the map or losing the selected instance.

The screenshot is useful for locating a session, but the displayed Contents view does not explain the work around it. It shows a failed lap inside a running workflow without showing why work continued, what comes next, what ran concurrently, or who supervised whom. There is a Shape tab; this critique is of the visible frame, not a claim about its unseen contents. See [the supplied frame](ui-input-hierarchy.png) and [assessment](ui-input-assessment.md).

The person opening a run should immediately be able to answer: **Where are we? What led here? What can happen next? What failed, and did it stop the run?** The map and timeline answer different parts of those questions.

## The workspace

Keep the run name, overall state, and breadcrumb at the top. Under that, give the main surface most of the space. A compact Outline control opens the familiar hierarchy when needed; it need not permanently consume a quarter of the window.

The conceptual contexts are **Program** and **Run**. Program shows possible structure, without timing or invented instance counts. Run shows actual instances and outcomes. Within Run, **Map** and **Timeline** preserve the selected operation and instance. If a compatible manifest is absent, show an **Observed run** map and omit Program; do not pretend the observed path is the complete program.

On a wide screen, selection opens a right inspector while the map stays visible. On narrower screens it opens below. A click inspects; an explicit Focus action changes the scope being viewed. Close returns space to the map. Switching lenses or opening a transcript preserves scope, instance, selection, and the user's viewport. Use a breadcrumb and compact ancestor context when focused; a minimap becomes useful only once the canvas needs panning.

Do not recreate Agents / Shape / Metrics / Contents tabs independently inside every scope. Details can have sensible local sections, but the user should not have to abandon structure to see a result.

## Visual grammar

| Thing | How to draw it | What it means |
| --- | --- | --- |
| Scope | Quiet labeled enclosing region | Lifetime / ownership boundary, not an executed step by itself |
| Group | Region with parallel child lanes and explicit join | Children may overlap; continuation follows the join |
| Loop | One repeated region, a labeled return path, and an exit | Template repeats; number of instances may be unknown |
| Agent turn | Compact operation card, agent/session label | A call in an execution scope; session identity is available in details |
| Command | Terminal-marked operation card | Command execution belongs in the same workflow as agent work |
| Supervisor | Session shown in a supervisory role near the watched turn | Attachment is not an ordinary control-flow dependency |
| Supervisor look | Small observed operation attached to its target | Actual work, with interval and result where recorded |
| Sequence / branch | Solid directed connectors, branch labels where needed | Supported possible control flow; position alone is not a dependency |
| Supervision | Dotted attachment with a short “watches” label when selected | Which turn is watched; nested supervision targets a look turn |

Use containment instead of drawing a contains edge to every descendant. Keep the primary flow directional within a scope, but route supervision along a separate adjacent band. Show in-focus supervision attachments by default: this is core workflow meaning, not an obscure advanced overlay. Cross-region supervision, fork provenance, steering delivery, and caller/callee links can be revealed on selection or by a relation control. Do not show all cross-links at once.

Cards should be compact and consistent: name, operation kind, state, and one relevant fact. Use quiet surfaces and borders, strong selected focus, a restrained active accent, and red for actual failure. Pair state with text/icon, not color alone. Commands should not inherit meaningless model/token fields. Keep cost and token detail in the inspector unless it helps answer the current question.

Use human names on the canvas and the full qualified path in the inspector. Names may repeat across scopes. The layout should establish clear entry, exit, branch, and return paths, with connectors routed outside cards and labels.

## Drill levels and continuity

| Level | What the person sees | What selection reveals |
| --- | --- | --- |
| Whole workflow | Main phases, collapsed regions, current location, exceptional instances | Region summary, activity, observed counts |
| Focused scope | Its operations, branches, parallel work, joins, and supervisors | One operation's source, context, outcome, relationships |
| Repeated scope | One template plus an instance strip / list | A particular task/lap/retry and its local state |
| Turn or command | The operation in its surrounding flow | Prompt/result or command/output, actual interval, errors, relevant values |
| Session / transcript | Conversation with the selected turn anchored | Earlier turns, cross-scope uses, tool activity, supervisor messages |

At every level there is a cheap way back to the surrounding shape. Focus and inspect are different actions. A session conversation can span multiple execution scopes while remaining owned by an ancestor; do not force the transcript into a false one-session/one-child-scope hierarchy.

## Repetition, failures, and live updates

Show a loop template once. Beside it show observed instances, e.g. `1 ✓  2 ✓  3 ×  4 ● · 4 observed`. Do not say `4 / N` unless N is actually known. Selecting 3 shows the failed instance while the header still says the run is running and 4 is current. A scope containing an earlier failure must not automatically look terminally failed if the workflow continued. Preserve both its lifecycle state and the fact that it contains errors.

Collapsed regions retain active/error counts and boundary connections. A failed child cannot disappear behind a green summary. Large loops use a compact instance list with current and exceptional instances easy to reach, rather than unrolling hundreds of copies across the canvas.

Do not continuously re-fit or reorder the map as events arrive. Update state in place; append new instances to a stable region. Keep inspected history stable while newer work runs. Auto-follow, if added, should be an explicit user choice.

## The timeline

Use one shared wall-clock axis. Rows follow scopes, then operations/turns; nested regions can collapse. Overlap must be visible. A supervisor look occupies real time alongside the watched turn. A command shows its own interval only if that interval was recorded.

Selecting a bar selects the same instance in Map and the inspector. Distinguish run start/end, scope duration, agent duration, and process duration. Do not derive a critical path or “blocked by” relation from temporal adjacency. No critical-path analysis is needed for the first useful version.

## Commands and supervision must survive the first design

Reserve their visual treatment now, even if some telemetry arrives later. A statically recognized command belongs in Program without a runtime result. An observed command can show executable/arguments, working directory, exit status, and stdout/stderr only to the extent recorded. `exec.Command(...)` constructs a command; it is not evidence the process ran. A named enclosing scope with recorded results is a useful first observation boundary, but its duration is scope time, not automatically process time.

For supervision, distinguish attached/no look yet; look executing; look completed; steer delivered; and steer dropped. A supervisor is still a session. A second supervisor can watch the first supervisor's look turn. The inspector must make the exact target legible. A decorative “supervised” badge alone cannot do this.

## A realistic Beta cut

1. Render observed scope/turn structure as a map, preserve hierarchy navigation, and open useful details beside it. Show active and exceptional work without losing global run state.
2. Expose observed timing as a simple expandable timeline using the same selection. Give recorded command scopes/results and supervisor events appropriate representations as those records support them.
3. Add compatible generated program manifests when extraction is available. Then show possible branches, repeated templates, and declared-but-not-observed sites. Clearly distinguish unknown coverage from a path known not to have executed.

These are independently useful increments within the Beta direction. The bounded source extractor (#201) and a useful program-shape view belong in Beta alongside the bounded linter (#162). An observed-only view is an intermediate delivery, not the final Beta shape goal. Exhaustive extraction, arbitrary helper expansion, exact dynamic-instance correlation, and elaborate graph layout can follow. Dynamic Set keys remain legal while we learn from workflow authors. No drag-to-author workflow editor is proposed.

## Design exercises to bring back

Please render the same realistic workflow in these states, rather than polishing another generic happy-path agent tree:

- Whole run: parallel research, adaptive task loop, verification, a command, a supervisor, and nested supervision all remain legible.
- Historical failure: task 3's command failed while task 4 is running; inspector explains the failure and continuation without changing the run to failed.
- Focused parallel scope: one child failed, another is still running/canceling, and the join has not completed.
- Supervisor inspection: select a look and see both its watched turn and the landed/dropped steering outcome.
- Large loop: 40 observed instances, three exceptional ones, one active; find an exception without expanding 40 full subgraphs.
- Missing manifest and narrow window: truthful observed structure, accessible names, usable details, no invented future path.

Judge the design by whether a person can answer the four opening questions and reach the relevant output in a few deliberate actions. The interactive study is illustrative, not a product implementation or a generated graph. It exercises Program/Run, Map/Timeline, instance selection, operation details, and transcript drilling; it does not implement full focus/expand behavior.

## Research and what to borrow

| Reference | Useful idea | Limit for Gimble |
| --- | --- | --- |
| [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/ui.html) | Graph and instance/history views; collapsible groups | A DAG/task matrix does not explain sessions or supervision |
| [Dagster](https://master.dagster.dagster-docs.io/concepts/webserver/ui) | Definition graph, run timeline, persistent inspector | Asset lineage is not Go control flow |
| [Argo](https://argo-workflows.readthedocs.io/en/latest/artifact-visualization/) | Parallel flow and clickable artifacts in a side panel | Flattened repeated templates create ambiguous names |
| [Perfetto](https://perfetto.dev/docs/visualization/perfetto-ui) | Shared time axis and linked selection/details | Trace tracks do not supply static workflow semantics |
| [Jaeger](https://www.jaegertracing.io/docs/2.17/features/) | Nested intervals, overlap, drill to evidence | Trace parentage is not session ownership |
| [Temporal](https://github.com/temporalio/documentation/blob/main/docs/web-ui.mdx) | Multiple projections of the same execution history | Its workflow commands/history are different runtime contracts |
| [n8n](https://docs.n8n.io/build/understand-workflows/understand-executions/view-executions-for-a-single-workflow/) | Definition/run distinction and failed-execution investigation | Canvas position must not determine Gimble semantics |
| [Tenacious](https://github.com/diffusioninc/Tenacious/tree/d3a4d7ff9f5445a42a7516eda845f8c58a531a45) | Run → step → transcript navigation; declarations versus visits | Pinned viewer is a terminal file browser, not a graph/timeline renderer |

All relied-on research is cached locally: [workflow tools](ui-workflows-report.md), [timeline tools](ui-timelines-report.md), [Tenacious source analysis](ui-tenacious-report.md), and [UI index](index-ui.md). Tenacious's full archive, pinned commit, and relevant extracted flat source files are included. These are inspiration, not commitments to their libraries or feature inventories.
