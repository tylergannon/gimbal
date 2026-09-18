# Gimble run interface — design handoff

Prepared against `main` at `0908812` (PR #262). This is a design brief,
not an implementation plan or a claim that the proposed interface exists.

## The assignment

Design a coherent interface for finding runs, understanding what happened,
and participating while agents work. Gimble workflows are ordinary Go;
this interface observes and interacts with them, it does not author them.

Tyler requested four capabilities:

1. List current and past runs.
2. Open a past run and review its history.
3. Open an active run and interact with it.
4. Provide at least one interactive map built from the generated workflow
   graph: click around its structure and zoom in/out to understand the
   shape and sequence of calls.

The map is part of this assignment, not an optional later enhancement.
The team is free to choose navigation, composition, visual style, and how
map and history complement one another. A graph does not have to contain
the entire transcript inside its nodes.

Companion: `graph-and-state-examples.md` includes a complete small graph,
a larger real workflow's shape, and explicitly fictional runtime content.
The research notes provide precedents, not additional requirements.

### Feature coverage at a glance

| Required area | Include in the design |
| --- | --- |
| Runs | Current/past distinction, identity and status, useful summaries, finding and opening an older run. |
| Historical review | Outcome, ordered activity, concurrency, repeated instances, prompts/results, tools, commands, errors, usage, decisions and interventions. |
| Live interaction | Current activity, pending interviews with correct placement, agent steer feedback, loop messages/wrap-up, stop-turn and cancel-run controls, live/disconnected states. |
| Generated map | Declared structure, call sequence, scope/group/loop hierarchy, click-to-inspect, pan/zoom, repeated execution selection, missing/incomplete graph treatment. |

Tyler explicitly included stop-turn and cancel-run controls in the design;
they are not existing web capabilities. Starting runs, workflow editing,
scheduling, and accounts have not been requested. Do not inherit them from
research examples.

## What the person needs to accomplish

### Find a run

Recognize active work and distinguish it from completed, failed, or
cancelled work. Identify a run by workflow name, run identity, and time.
Find an older run without reading every transcript. Show enough summary
information to choose what to open; propose the right search/filter and
density treatment rather than designing a general analytics dashboard.

### Review a past run

Understand the outcome, work sequence, parallel work, and repeated
attempts. Navigate from an overview to a particular scope, agent turn,
command, or interview exchange. Inspect the prompt actually sent, output
or error, transcript/tool activity, timing, and available usage.
Follow planner decisions and human/supervisor interventions in context.
Historical views must not offer live actions as if the work were running.
Reviewing history does not require a playback engine or time-scrubber.

### Participate in an active run

Find what is running and what needs a human answer. Read earlier activity
without new events continually moving the selection or viewport. Make the
difference between run status and connection status clear: a disconnected
browser does not prove that the workflow failed.

Existing interactions to preserve:

- Steer an active agent session and show whether the message landed or
  was dropped. Sending a steer does not guarantee delivery.
- Send a PromiseLoop planner a message or ask it to wrap up. These messages
  wait for a planning decision; this differs from steering a running turn.
- Answer an interview question in its correct workflow location. The same
  location holds the agent's activity/steering while it thinks, then the
  question control while it waits. Keep previous exchanges accessible.
  Concurrent interviews must remain clearly distinct. Empty answers
  currently end an interview normally; the design should make that intent
  understandable rather than depend on discovering an empty-submit trick.

Also required: stop a running turn and cancel an entire run. Make their
different targets and consequences explicit, show the action's progress and
outcome, and guard against accidentally cancelling all work when the person
means one turn. Stopping a turn does not itself cancel the entire run; the
workflow decides how to handle that interruption. The current page has no
such controls. Their web wiring is implementation work, not a reason to
omit them from the design.

### Explore the workflow map

At overview scale, understand sequence, containment, concurrent branches,
and repetition. On selecting or expanding a region, see its calls and
their details. Support pan, zoom, and returning to the overall shape.
Provide a way to navigate without relying exclusively on precise dragging
or color. Keep selection understandable when moving between map and history.

Distinguish these two things:

- **Workflow definition:** the operations declared in source, including
  possible branches and repeating bodies.
- **Run history:** the actual instances, order, outcomes, and timings.

One loop body may execute many times. Several calls can use the same agent
session. A session can be created outside a loop and do work inside each
task. Do not equate a session, a call site, and an execution instance.
Parallel siblings have no implied completion order just because one is
drawn to the left. Supervisor links are not sequential workflow steps.

Declared but unobserved work is not automatically queued, skipped, or
failed. The graph is a partial source description, not a complete model of
Go execution. Show extraction gaps honestly. When a graph is unavailable
or cannot be associated reliably with history, the recorded run must still
be useful without an invented mapping.

## Concrete walkthroughs for the design

Use these as design scenarios, not fabricated evidence of live runs:

- **Interview:** find the active run, inspect question-generation activity,
  answer a question, see the next turn in the same location, and review the
  exchanges after completion. Include two interviews waiting concurrently.
- **Development workflow:** two researchers work in parallel, then a
  planner dispatches a coding task watched by two supervisors, followed by
  command checks and independent validation. Show a supervisor's checks
  with no objections as distinct from a corrective steer or approval.
- **Repeated task:** expand a loop and select a particular iteration;
  distinguish its failed command from later work that succeeds. This
  scenario exercises the design beyond the last run's single assignment.
- **Large or incomplete history:** navigate hundreds of turns, a lost live
  connection, and an archived run with no matching compiled graph. Empty
  project and missing-run states should also have a clear treatment.

## What exists and what needs engineering

This inventory is based on source inspection, not a fresh browser test.

| Surface | Current situation |
| --- | --- |
| Project home | Informational landing page, not a run list. A project-wide listing/live-discovery surface is work to do. |
| Individual run | `/runs/[runID]` renders a snapshot and subscribes to updates. Mostly scope rows and turn/transcript sections, plus interview grouping. |
| History | Recorded run tables and session transcripts support opening a known run ID. This is not an existing time-travel UI. |
| Controls | Session steer, planner message/wrap-up, interview answer are wired to Go remote functions. |
| Generated graph | Typed, ordered operations compiled into the workflow binary. The current run page load supplies a snapshot, not a graph. Graph delivery and map rendering are work to do. |
| Historical graph | Not persisted with a run. A current binary's same-named workflow is not proof of the exact historical definition. No version-matching solution is prescribed here. |
| Server lifetime | Example CLI exits on completion; keeping it alive while web users remain connected is tracked in issue #261. Design completed/disconnected states separately. |

The source graph contains sessions/forks, agent calls and supervisors,
interviews, commands, scoped values, scopes, groups, PromiseLoops, finite
iteration, ordinary repeats, conditions, and extraction diagnostics.
Models, results, costs, timestamps, questions, and answers are runtime data,
not properties to invent in the static graph. Runtime-to-call-site mapping
needs engineering verification, especially for repeated calls on one session.

## Working constraints and freedom

- SvelteKit/skgo app with shadcn-svelte primitives already installed. Their
  default appearance is not a prescribed visual identity.
- One local project runtime is the existing product boundary. Accounts,
  hosted collaboration, workflow editing, starting/configuring workflows,
  and multi-project management are not additions to this brief.
- Prefer an interface usable beside a terminal for long sessions. Desktop
  is the natural first exploration; mobile scope and theme choices remain
  open, not implied commitments.
- Usage comes from recorded accounting. A missing provider cost reported
  as zero does not establish that the work was free. Do not invent precision.
- The existing `docs/web-app.md` mixes decisions, aspirations, and stale
  implementation status. Its graph-as-later option is superseded by this
  request. Use the current source inventory above for capability claims.

## Requested design return

First compare a small number of genuinely different interaction concepts
using the same graph and run scenarios. This is the proposed exploration
process, pending Tyler's choice of deliverable. Choose a direction before
spending effort polishing every screen. A concept should show navigation,
overview/detail zoom, live versus historical states, pending interviews,
action feedback, and no-graph/disconnected cases. Mark new data/control needs
separately from existing capabilities. Bring visual judgment, not a new
workflow abstraction or a backend implementation plan.

## Source packet

Paths below are relative to this repository's root. Start with this brief;
the implementation references are for checking concrete questions.

- Product background: `docs/web-app.md` (with the caveat above).
- Graph vocabulary: `workflow/graph.go`; TypeScript counterpart:
  `web/src/lib/workflow/types.ts`.
- Small example: `internal/workflows/interview/interview.go` and its
  `workflow_gen.go`; entrypoint `cmd/examples/interview/main.go`.
- Parallel/repeated/supervised example:
  `internal/workflows/implementinterview/implementinterview.go` and its
  `workflow_gen.go`.
- Current UI: `web/src/lib/observation/RunViewer.svelte`,
  `InterviewNode.svelte`, `SessionTimeline.svelte`, `SteerBox.svelte`,
  `LoopBox.svelte`, `InterviewAnswerBox.svelte`.
- Runtime data: `internal/observation/rows.go`, `events.go`;
  page loading: `web/src/routes/runs/[runID]/page.server.go`.
- UI primitives and tokens: `web/src/lib/components/ui/`, `web/src/app.css`.

No private session transcripts or run dumps are bundled. No production UI
changes are part of preparing this handoff.
