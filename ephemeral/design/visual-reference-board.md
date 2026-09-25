# Visual references and concept directions

These are reference images, not proposed Gimbal screens. They are published
product screenshots inspected by the parent or Luna execution researcher;
we did not exercise authenticated product sessions. An older screenshot is
still a useful design precedent, not a guarantee of the product's latest UI.
Images remain at their public source URLs with attribution. This board is
not an offline image bundle; use the source-page link if an embed fails.

## A. Graph-first workspace

Lead with the workflow's shape and a selected-node inspector. Keep execution
instances accessible from the selected definition node, without putting all
transcript text on the canvas. Useful for orientation and explaining
parallel branches; the risk is losing the human conversation in tiny nodes.

### LangSmith Studio — graph beside activity and a human response

![Published Studio graph and interrupt response interface](https://canada1.discourse-cdn.com/flex007/uploads/langchain/original/2X/c/ce3ea4d43d1d4279b48e0a460d85714910a32e24.png)

**Visible:** map with nested region and zoom controls at left; execution
details and a specific agent's pending question/response editor at right.
**Borrow:** preserve graph context while interacting with an identified
agent. **Do not borrow:** state editing, rerunning checkpoints, or the
developer-oriented raw response editor as the default interview experience.
This is a community-posted screenshot in a bug report, not evidence of
correct resumption. [Source discussion](https://forum.langchain.com/t/langsmith-studio-issue-when-resuming-from-an-interrupt-inside-a-subgraph-it-doesnt-properly-resume-instead-restarts/3160).

### GitHub Actions — compact run graph and grouped fan-out

![GitHub Actions job graph](https://docs.github.com/assets/cb-63715/images/help/actions/workflow-graph.png)

**Visible:** connected job cards, status/duration, grouped matrix jobs, and
fit/zoom controls. **Borrow:** readable overview with a deliberate route to
individual instances. **Do not borrow:** treating Gimbal's source-order
relationships as identical to CI dependency edges.
[Source and documented graph-to-log interaction](https://docs.github.com/en/actions/how-tos/monitor-workflows/use-the-visualization-graph).

### Blender — spatial composition as a deliberate contrast

![Blender node workspace](https://docs.blender.org/manual/en/latest/_images/interface_controls_nodes_introduction_example.jpg)

**Visible:** interconnected cards, colored headers, sockets and substantial
in-node controls over an image backdrop. **Borrow:** orientation across a
spatial network. **Do not borrow:** editable wiring, dense control panels
inside every node, or the busy backdrop. This is a useful counterexample
as well as inspiration: Gimbal observes code; it is not a node editor.
[Source manual](https://docs.blender.org/manual/en/latest/interface/controls/nodes/introduction.html).

### Airflow — human input at the selected task

![Airflow task graph beside its Required Action form](https://airflow.apache.org/docs/apache-airflow/stable/_images/hitl_wait_for_input.png)

**Visible:** a selected `wait_for_input` node among parallel siblings,
graph overview controls, and that task's input form in the right-hand
detail panel. **Borrow:** retain workflow location while answering a
specific question. **Do not borrow:** generic approvals, task-state
editing, or treating one waiting task as a paused run. The screenshot's
older “Deferred” label is not a proposed Gimbal status.
[Source tutorial](https://airflow.apache.org/docs/apache-airflow/stable/tutorial/hitl.html).

## B. History-first workspace

Lead with an execution outline or time-aligned lanes plus stable detail.
Provide the required generated map as a linked view, not as a replacement
for history. Useful for debugging repeated/parallel work; the risk is an
intimidating profiler that obscures the workflow's overall purpose.

### Jaeger — hierarchy aligned with elapsed time

![Jaeger trace-detail waterfall](https://www.jaegertracing.io/img/trace-detail-ss.png)

**Visible:** indented operations alongside bars on a shared time axis, with
trace-level summary above. **Borrow:** overlap and duration stay readable
while inspecting one instance. **Do not borrow:** equating trace parentage
with all Gimbal relations; session ownership, turn scope, and supervision
are different relationships. [Source](https://www.jaegertracing.io/docs/1.76/).

### Chrome DevTools — an overview band above focused detail

![Chrome Performance panel overview and flame chart](https://developer.chrome.com/static/docs/devtools/performance/reference/image/flame-chart.png)

**Visible:** a compressed recording overview, narrow selected time range,
nested detail bars, and alternative detail tabs. **Borrow:** retaining
orientation while zoomed into a tiny portion of a long history. **Do not
borrow:** call-stack causality or profiling-specific metrics as workflow
semantics. [Source reference](https://developer.chrome.com/docs/devtools/performance/reference).

### Langfuse — alternative representations of the same agent work

![Langfuse agent graph](https://langfuse.com/_next/image?url=%2Fimages%2Fdocs%2Ffaq%2Fgood-trace-agent-graph.png&w=3840&q=75&dpl=dpl_3VAKTcMXLYzwNRec1VTkfa49ffGL)

**Visible on the source page:** an indented trace tree alongside a graph
with Aggregated/Expanded choices, branching, counts, and zoom controls.
**Borrow:** the user can change representation without changing the work
being inspected. **Do not borrow:** assume this runtime-derived graph is
the same thing as Gimbal's generated source graph.
[Source, including both images](https://langfuse.com/docs/observability/best-practices).
Direct image navigation was blocked in our browser; the embedded images on
the source page rendered and were inspected. This link may need that fallback.

### AWS Step Functions — distinguish each repeated execution

![AWS execution table with expanded map iterations and timelines](https://docs.aws.amazon.com/images/step-functions/latest/dg/images/sm-table-view-timeline-color-codes.png)

**Visible:** numbered, expandable iterations; one selected row; distinct
statuses and durations; compact aligned timelines. **Borrow:** make the
particular execution instance explicit. The documentation additionally
describes linked graph/table selection and an iteration picker; those
interactions are not demonstrated by this still image. **Do not borrow:**
AWS state-machine semantics or restart/redrive controls.
[Source execution-details guide](https://docs.aws.amazon.com/step-functions/latest/dg/concepts-view-execution-details.html).

## C. Attention-first run console

Lead with current runs and specific reasons to open them, especially a
pending question. A selected run has a stable summary, activity, controls,
and its generated-map view. Useful when several runs compete for attention;
the risk is reducing the product to a list of alerts and hiding structure.

### Sentry — scannable rows with a strong primary label

![Sentry issue-stream rows](https://cslswue7zohm4cat.public.blob.vercel-storage.com/tL4PFCI-image.png)

**Visible:** bold labels, contextual sublines, recency/age, compact trends,
and small row indicators. **Borrow:** finding the item needing attention
without opening every detail. **Do not borrow:** assignees, incident policy,
unread persistence, or analytics as automatic new Gimbal requirements.
[Source changelog](https://sentry.io/changelog/issue-stream-ui-enhancements/).

### Temporal — stable run identity above a detailed execution history

![Temporal execution details](https://learn.temporal.io/assets/images/web-ui-detail-page-ebc2624dd8c608be9a818eb1e13e7d9b.png)

**Visible:** run/status summary, input/result panels, tabs, an activity
timeline, and timestamped event rows. **Borrow:** a dependable place to
answer “which run is this, how did it end, and what happened?” **Do not
borrow:** every infrastructure metadata field or tab.
[Source tutorial](https://learn.temporal.io/getting_started/ruby/first_program_in_ruby/).

## Compare concepts before selecting a look

The three directions are proposals, not a decision for Fable to treat as
settled. Each must cover the same required features, including the map.

| Concept | First thing shown inside a run | How the map participates | Main design question |
| --- | --- | --- | --- |
| Graph-first | Structure and current selection | Primary workspace | Can I answer a question and read a long turn without fighting the canvas? |
| History-first | Execution outline/time lanes | Linked structural view | Can I understand the declared workflow, not just what already happened? |
| Attention-first | Current activity and pending human work | Persistent route from run/detail selection | Can I explore freely without the app continually dragging me to the latest event? |

Suggested small process:

1. Show one rough overview/detail pair for each direction using the same
   development-workflow example. Vary information architecture, not just
   palette or corner radius.
2. Walk each through finding a past failure, selecting task 2 of a loop,
   answering one of two concurrent interviews, and stopping a turn versus
   cancelling the run. Include a lost connection and missing graph.
3. Tyler chooses the strongest direction or an explicit combination.
   Then refine that into the desired prototype/screens. Do not combine every
   reference into one screen before making this choice.

My initial hypothesis is graph plus a stable inspector/history surface,
with a small attention entry point for interviews. It is a hypothesis to
compare, not the handoff's mandated layout. In particular, a graph-only UI
would be poor for reading the long conversations Gimbal produces.

The detailed research notes are optional appendices:
`research-agent-interfaces.md`, `research-execution-interfaces.md`, and
`research-adjacent-interfaces.md`. [Supplemental intake](research-intake.md)
assesses Tyler's supplied Gemini and Claude reports. Temporal appeared in
two lanes and is intentionally counted once here. The two additions answer
specific questions about repeated instances and task-scoped human input;
they do not add required features or dictate a layout.
