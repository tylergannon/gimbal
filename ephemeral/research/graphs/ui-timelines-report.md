# Workflow program shape and actual run shape: UI research

- Research date: 2026-09-14
- Scope: bounded review of four primary-source UI families for Gimble's
  workflow graph and live/past run page.
- Local source notes: `ui-timelines-perfetto-ui.md`,
  `ui-timelines-perfetto-plugins.md`, `ui-timelines-jaeger-features.md`,
  `ui-timelines-jaeger-images.md`, `ui-timelines-temporal-web-ui.md`,
  `ui-timelines-temporal-events.md`, `ui-timelines-reactflow-subflows.md`,
  and `ui-timelines-reactflow-expand-collapse.md`.

## Finding

Gimble needs one identity-preserving model with at least two projections: a
static program projection and an observed run projection. A persistent run
context should be able to open the corresponding program region, source site,
session, turn, command, or event without reducing the page to a sidebar tree and
a flat selected-folder table. The run projection needs a wall-clock timeline;
the program projection needs regions and typed relations, not fabricated time.

The useful common pattern across the sources is global context plus local detail:
keep the overview visible, narrow the viewport or expand a region in place, and
drive a detail drawer/transcript from one stable selection. Perfetto documents
shared time tracks, area selections, and a Current Selection drawer
([ui-timelines-perfetto-ui.md:10-25](ui-timelines-perfetto-ui.md)); Jaeger shows a
trace overview strip above an indented, overlapping waterfall with row-level
errors ([ui-timelines-jaeger-images.md:12-25](ui-timelines-jaeger-images.md));
Temporal explicitly offers Timeline, All, Compact, and JSON views over one
execution ([ui-timelines-temporal-web-ui.md:14-28](ui-timelines-temporal-web-ui.md));
React Flow keeps hidden descendants in the full graph while rendering a visible
slice ([ui-timelines-reactflow-expand-collapse.md:10-20](ui-timelines-reactflow-expand-collapse.md)).

## Proposed drilldown levels

| Level | Program projection | Run projection | Required interaction |
| --- | --- | --- | --- |
| 0. Context | Workflow revision, entrypoint, graph coverage, unresolved facts | Run status, elapsed time, active/ended/failed counts, cost summary | Switch program/run context while retaining the same selected identity |
| 1. Regions | Root, Scope, Group, Loop, repeated body, branch, parallel group/join | Region lanes on the wall clock; collapsed region summary with count/status/duration | Expand/collapse in place; preserve row keys and cross-region links |
| 2. Operations | Generate site, planner/supervisor look, command operation, control point | Turn/operation spans with overlap, status, model/cost, start/end, pending state | Select one operation; center it in timeline and show detail drawer |
| 3. Evidence | Source location, declared name, typed edges, unresolved alternatives | Transcript turns, steer, tool/event payload, command result, error, raw JSON/log link | Selection is bidirectional among source, row, transcript, and event |

The level boundary is a view affordance, not a new data model. A collapsed
region still carries visible child count, active/failed markers, and a path to
its exceptional descendants. Compact summaries must be reversible to the full
turn/event list, following Temporal's compact-versus-all distinction
([ui-timelines-temporal-web-ui.md:20-28](ui-timelines-temporal-web-ui.md)).

## Program projection

Use explicit regions as containers: root, named Scope, Group children, Loop
body/planner, branch alternatives, and parallel launch/join. Keep typed edges
separate: `contains`, sequential/conditional/repeated control, parallel
launch/join, `uses-session`, `forked-from`, `supervises`, and runtime `steers`.
React Flow demonstrates why containment and connectivity must coexist as distinct
relations: children can connect both within a group and across its boundary
([ui-timelines-reactflow-subflows.md:11-22](ui-timelines-reactflow-subflows.md)).

Default rendering should show regions and execution flow. Keep helper/callee
relations, fork links, session ownership, and supervision available as toggled
overlays. Perfetto's overlay and selection model supports this separation:
cross-track relationships can be drawn without changing the underlying track
rows, and selection can scroll/focus the selected entity
([ui-timelines-perfetto-plugins.md:10-24](ui-timelines-perfetto-plugins.md)).

For a source region, show coverage honestly: recognized sites, possible branches,
repeated templates, and unresolved alternatives. A plain helper call is not
automatically a scope. Keep source-site identity even if several runtime turns
may map ambiguously to the same declared name.

## Run projection

Use a horizontally shared wall-clock axis and vertically stable lanes/rows. A
lane can be a declared execution region or an operation family; parallel siblings
occupy overlapping intervals. A row shows status, duration when observed, model
and cost where available, and a compact exceptional marker. Put the transcript,
structured payload, and raw log/event references in a detail drawer rather than
forcing the operator to leave the overview.

Jaeger's inspected official screenshots demonstrate the visual grammar: a small
overview strip sits above a large waterfall; indented rows overlap on one time
axis; each row retains a duration and errors stay attached to the failing row
([ui-timelines-jaeger-images.md:12-25](ui-timelines-jaeger-images.md)). The
pattern transfers to Gimble's actual run shape, but its span nesting must not be
copied blindly: a supervisor is a supervisory relation/side lane, not a
completion dependency.

For live runs, append events and update existing rows in place. Do not globally
relayout the page when a sibling starts or finishes. React Flow's expand/collapse
example supports retaining the complete graph while rendering only the visible
portion ([ui-timelines-reactflow-expand-collapse.md:10-20](ui-timelines-reactflow-expand-collapse.md));
the runtime equivalent is retaining stable row IDs, with an explicit view state
for collapsed regions. New rows should be anchored to their region and ordinal
slot; a user-selected row should not jump because another branch emitted an
event.

Keep planned and observed states distinct. Temporal's event model separates
commands from later persisted events and notes that a running/retrying Activity
may have only a scheduled event until a terminal event exists
([ui-timelines-temporal-events.md:10-24](ui-timelines-temporal-events.md)). For
Gimble, a static command site or planned supervisor attachment is not proof that
the process started, the look happened, or the worker was steered. Show pending,
observed, ended, failed, cancelled, and dropped as evidence-backed states.

## Selection and focus

Every projection should use one stable identity tuple, for example graph
revision + source/region ID + runtime instance ID when present. Selection should
support these paths:

- click a program region -> highlight matching observed instances;
- click a run row -> reveal source site/region and center the wall-clock interval;
- click a transcript/event -> select the owning turn/operation and preserve run
  position;
- click a cross-region relation -> show its endpoints and relation type without
  changing either endpoint's containment.

Perfetto's documented selection options are a direct precedent for selection
that switches to contextual details and optionally scrolls to the entity
([ui-timelines-perfetto-plugins.md:10-18](ui-timelines-perfetto-plugins.md)).
Jaeger's top overview strip and detailed waterfall likewise give a selected run
both global and local time context ([ui-timelines-jaeger-images.md:17-25](ui-timelines-jaeger-images.md)).

## Exceptional paths and collapsed groups

Collapsed regions should preserve: total child count, active count, terminal
status summary, observed duration range, and an exception/steer badge if any
descendant has one. Provide "show exceptional descendants" as a local expansion
action. Do not hide a failed child only because the parent completed or because
the default compact view groups low-value events.

The source evidence supports this reversible compression: Temporal keeps a full
event view and a compact logical grouping ([ui-timelines-temporal-web-ui.md:20-28](ui-timelines-temporal-web-ui.md)); React Flow keeps hidden nodes in the
underlying graph ([ui-timelines-reactflow-expand-collapse.md:10-14](ui-timelines-reactflow-expand-collapse.md)); Jaeger keeps error markers on
individual waterfall rows ([ui-timelines-jaeger-images.md:19-23](ui-timelines-jaeger-images.md)).

## Boundaries and anti-patterns

- A trace parent is not automatically a lexical Go scope. Show `contains` only
  for recognized Gimble regions; show observed span/session/turn links as their
  own relation.
- A box's width means elapsed time only in the runtime projection when start/end
  were observed. Static graph node width is layout, not duration.
- A supervisor is not a dependency or approval gate. Render planned attachment,
  actual look, and steer as distinct facts.
- A lexical source edge is not a must-happen-before edge. Branch, loop, and
  parallel relations need explicit edge types.
- A generated command node does not prove process telemetry. Exact start/end,
  exit, stdout, and stderr require an explicit observation boundary.
- A critical path is safe to mark only when derived from observed intervals and
  declared dependency/control semantics. None of the reviewed sources licenses
  inferring it from indentation, adjacency, or row width.
- A clean static extraction or complete event count does not prove all runtime
  behavior. Keep unresolved source mappings and unsupported analysis visible.

## Decision-ready slice

For the first page, build the run projection around a stable timeline tree with
region headers and operation rows, plus a small overview strip and a selection
drawer. Add a program/run toggle that retains the selected graph identity, and
make typed overlays opt-in. Implement compact/expanded region state with
exception badges and reversible child counts. Defer a full free-form node editor,
automatic critical-path claims, and hard static ownership analysis until runtime
evidence and the supported graph manifest are sufficient.
