# Gimbal's direction

Product and architectural direction from Tyler's September 8–9, 2026
discussions. This is an edited synthesis, with proposed design consequences
identified below. [His broader direction statement](../ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CLARIFICATION-VERBATIM.md),
[his follow-up on diagrams, actions, and research/indexing](../ephemeral/projects/gimbal/programmatic-workflows/ACTIONS-AND-KNOWLEDGE-VERBATIM.md),
his [clarification of program shape versus input](../ephemeral/projects/gimbal/programmatic-workflows/INPUT-SEPARATION-VERBATIM.md),
and his [corrections to the first synthesis](../ephemeral/projects/gimbal/programmatic-workflows/DIRECTION-CORRECTIONS-VERBATIM.md)
are preserved separately, with only the retired project name normalized.
This document guides the work ahead. The graph engine it contrasts with was
retired on 2026-09-10; the Go-program runtime described here is being built in
`program/`.

## Learn from the experience of authoring workflows

Gimbal's next direction comes from Tyler's experience adapting and using
StrongDM's Attractor pattern: expressing a workflow is difficult, and coding
agents often lack the judgment needed to design a good one. A workflow can
be structurally valid and easy to draw while giving an agent an unclear
objective, too many responsibilities, or poor access to the information it
needs.

The response is to make Gimbal a tool for designing, understanding, and
improving how agents do work. Ordinary Go gives authors freedom to express
new orchestration tactics. Context engineering guides how those tactics
arrange objectives, information, responsibility, feedback, and attention.
Success means reaching the intended result accurately, quickly, and at
reasonable token and monetary cost.

The desired state and how to recognize success remain declarative. Each Go
program defines the plain JSON input appropriate to its task and publishes
that shape. Validation may be executable code, agent work, or a combination.
Go expresses the mechanism that advances toward that state.

**Humans and agents should be able to understand both how a workflow
progresses and what its shape asks each participant to know, discover,
decide, and accomplish.** That is the legibility we want before a run, during
it, and when learning from its outcome.

## Separate program shape from its input

**A well-designed workflow distinctly separates the shape of its program
from the input data supplied to start it.** Tyler's input framing includes
the available information and the structure and quality of its semantic
index, alongside the goal, requirements, constraints, and applicable
validation.

The program expresses the method: stages, loops, decisions, agent
responsibilities, and how findings or feedback change the next action. Its
input establishes the particular task and the information environment from
which that method begins. Source material and index files can be supplied
by reference; the initial JSON need not contain their full contents.

This separation persists while the work evolves. Available data can grow as
research, implementation, and validation produce findings and artifacts.
The index's organization and quality can change as routes are added or
improved. The program defines how to act on and develop that information;
the current information is data on which it operates. Data-dependent paths
through the program are expected.

This makes the method reusable across tasks and gives evaluation distinct
variables to examine. We can run the same program with different source
coverage or index arrangements, or compare different program shapes from
the same starting task, data, and index. Understanding a run therefore
requires both the method and the information conditions in which it ran.

## Curate dependable named workflows

Tyler's September 9 clarification allows an additive Go library while the
existing workflow language remains available. The [recovered Go POC](../ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md)
already supplies a narrow implementation to extend; this is not a fresh
start for the primitives or the ordinary-Go authoring shape.

The public library matters, but the primary investment is a small curated
set of named workflows that improve over time. Their calling conventions
should remain dependable enough for long-term scheduled tasks. Each workflow
declares its own arbitrary Polytype-supported argument shape; generated JSON
Schemas describe that argument, rather than the workflow's program. The CLI
should explain when to use the workflow, document its calling convention,
and validate the argument. Workflow-defined role names such as `sswe`,
`eng-mgr`, and `tester` express assignments instead of direct model arguments.

The Go workflow examples preserve this clarification as a compact API sketch.
See the [earlier concurrency sketches](../ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md).

## Preserve what good graphs give us

Tyler identifies a real strength of DAG-style workflow definitions:

> When you look at a good DAG workflow design, you know what the workflow is going to do and how it's going to progress through its stages.

That clarity is valuable. It does not establish that the workflow will be
successful or easy to design. A clear diagram of builder, reviewer, and
retry stages can still conceal a poorly framed assignment or an ineffective
review loop.

Moving authorship into Go should preserve this ability to understand the
shape. The author should gain ordinary loops, conditions, types, functions,
and concurrency while the reader retains a clear view of how the work
proceeds. A graph can remain a useful derived view.

## Workflow code should read like pseudocode

Tyler's standard is that a workflow definition should ideally fit into a page
or two of Go that resembles pseudocode and gives a reader an excellent sense
of the workflow. This is a comprehension target, not a line-count limit. At
the intended level of detail, the reader should see the major stages,
enclosing loops, continuation and exit conditions, validation, and any
supervision or parallel work that defines the method.

This is first an API-design ideal, not a claim about diagrams. Gimbal's API
should be developed by asking whether it lets authors state an orchestration
method this clearly. A derived diagram can help explain the result, but it is
secondary to source that already reads as an intelligible workflow.

Function and type names, indirection, composition, and DRYness should be
judged by their contribution to that understanding. The same standard should
guide Gimbal's library functions, examples, and workflow-authoring skills.
Derive APIs from clear orchestration pseudocode, then examine whether the
resulting Go preserves its meaning.

Practical design consequences of this standard:

- Keep decisions that define the method visible: what repeats, what waits,
  what result changes the next action, and what ends the work.
- Extract input validation, defaults, prompt assembly, and execution detail
  so they do not obscure those decisions.
- Use composition to name understandable sections. A reader should be able
  to expand a section easily when its internal shape matters.
- Prefer names that explain the work and its meaningful outcomes. A generic
  helper that hides a retry policy or review circle can make code shorter
  while making the workflow harder to understand.
- Let abstraction and reuse earn their place through clarity. A little
  repetition may be preferable to making readers chase orchestration across
  many files.

The [Go-program direction](workflows-as-programs.md) develops the language
and context choices. Its ordinary-code freedom matters independently of
whether an automatic visualizer recognizes a particular program.

## A small vocabulary of actions, with meaningful task roles

Ordinary Go supplies loops, branching, composition, and concurrency. These
need no corresponding inventory of executable graph-node types. Tyler's
initial action vocabulary is command execution, agent tasks, and
supervisor/coach agents. Supervision deserves a recognizable relationship
to the work it observes and steers, even when it uses the same underlying
agent machinery.

Research, indexing, coding, and validation should be recognizable purposes
in the authoring and visual vocabulary. Their names help a reader understand
why a participant is present and what responsibility it carries. They can
have ordinary Go entry points, task-appropriate inputs and results, and
different visual treatment while sharing lower-level execution mechanisms.
Validation, for example, may combine commands and agent judgment.

Tyler proposes five or seven common action types as a way to keep the
authoring vocabulary small. The exact set is open; these are design budgets,
not an established taxonomy. Planning and critique can be expressed as named
agent tasks or composed routines. **Agent proposal:** explicit human
ask/decision is another useful interaction to consider, because waiting for
a person's answer differs from coaching work already in progress.

The test for a distinction is whether it makes the workflow and its context
consequences easier to understand. Giving research or validation a visible
identity should not require inventing a new execution engine for that role.

## Make the context consequences visible

For each agent, goals, requirements, rules, available means, scoped state,
the semantic index, and on-disk material form one deliberately arranged
information environment. The first message is an entry point organized
around success: what this assignment serves, what this agent must do, what
matters now, and how to find the information it needs.

A workflow distributes that burden across agents and time. A builder works
on the change; a supervisor watches for a particular kind of drift; a
checker later judges the result. Understanding the shape should include
understanding those responsibilities and information boundaries. For
example, it matters whether a checker receives the builder's whole account
or an independently framed assignment with the requirement and the actual
work to inspect.

The source should make important context choices apparent through readable
calls and scopes. A detailed inspection view can reveal the composed
request, index entry point, and relevant files without expanding every
prompt into the page of orchestration code. Fluent authoring and inspectable
context should reinforce one another.

## Research and indexing prepare the work's information

Tyler proposes research and indexing as companions, like coding and
validation. A task substantial enough to warrant a workflow should have its
relevant broader context local, searchable, and connected to the assignment.
Preparing that information is part of the work's design. Useful prior art,
project rules, decisions, examples, and known constraints should be available
so subsequent agents can work from evidence and existing solutions.

A semantic index can use ordinary local source material and a compact routing tree. An entry
point routes by likely task questions; narrower routes lead to annotated
citations and the underlying files. Agents can inspect those files directly
and search them with ordinary tools. The source collection and index are
different artifacts within the same agent context. This method does not
require embeddings or a vector database.

Research gathers and assesses the information needed for upcoming decisions,
including explicit gaps where an answer is not yet known. Indexing connects
that information to the questions the next agent will ask. Together they
produce usable knowledge: local material that an agent can actually find
and apply from its assignment's entry point. Discoverability is part of the
research handoff.

An existing useful collection can satisfy this need. When material is
missing, research adds it; when routes are missing or misleading, indexing
repairs them. A newly discovered gap during implementation or validation can
lead back to targeted research and an update to the relevant routes. The
workflow need not collect everything before any useful work begins.

As execution proceeds, selected findings, decisions, validation feedback,
and work products become part of this information environment. Index what
will help a later decision or handoff, and keep the current entry point
connected to it. Known gaps should be visible enough to direct further
research instead of inviting an agent to fill them with assumptions.

**Proposed readiness check:** take a few questions that the intended builder,
checker, or coach will need to answer. Starting from its entry point, can it
reach useful source material with little searching? Failed retrieval points
to a concrete repair in collection or navigation. This provides a practical
way to assess the pair without making exhaustive research or a large
index-maintenance system a prerequisite for every task.

## Recovery from wayward execution remains an open problem

Gimbal should eventually help a workflow recognize that it has lost the point
and steer back toward its declared goal. That is a desired capability, not a
designed mechanism. There is no established "compass" abstraction or general
self-correction method today.

Good supervisor steering is the only concrete approach currently identified:
an observer with the right perspective notices drift and corrects the active
work. Future designs may discover other mechanisms, but documentation and APIs
must not imply that one has already been selected.

## Learn from bad runs through useful telemetry

Tyler's longer-term priority is diagnostic: investigate why a workflow
wandered, failed, cost too much, or needed unwanted steering. Analysis of
successful runs may also help, but it is not the center of this requirement.
The following is a proposed information set for examining the context
engineering of those bad runs, not a first-version event schema:

| Information to inspect | What it helps investigate |
| --- | --- |
| Goal, requirements, validation, and the agent's assigned responsibility | Whether the work and the meaning of success were clear |
| Authored step instruction, assembled initial request, and subsequent delivered messages | What the agent was actually asked, including scoped context and index navigation |
| Semantic index and relevant indexed material as they existed for that invocation | Whether useful, accurate information was available and discoverable |
| Observed searches, reads, and tool results, where the harness exposes them | How the agent sought information and what it encountered |
| Steering messages, their source, target, timing, content, and delivery status | How often intervention occurred, what kind it was, and how work proceeded afterward |
| Agent/model, invocation, enclosing loop and attempt, outcome, elapsed time, and reported usage | Where work, waiting, retries, and spending accumulated |

For example, a slow review loop might reflect a vague acceptance condition,
a requirement buried behind an unhelpful index entry, repeated rediscovery,
or steering that asks for extra scope. Examining the actual requests,
information paths, and intervening messages makes those hypotheses
investigable.

Index quality includes the quality of the material it routes to and whether
the routes help answer the agent's real questions. Message counts and token
totals alone cannot explain that. Frequent steering may be necessary,
unhelpful, or a sign that the initial assignment needs improvement; its
content and the surrounding work matter.

Historical inspection needs the relevant request and information from the
time of the call. A link to a file that has since changed can mislead. Retain
the useful text or a retrievable version where needed for diagnosis; exact
workflow replay and a complete provenance system are not implied.

The eval tier can compare orchestration shapes and context arrangements
against the same task, starting project, and established validation. Actual
success, wall time, tokens, and interventions inform which changes to keep.
Telemetry should help improve the method and its information, rather than
becoming an independent scoring exercise.

## Source views and runtime views

**Technical assessment, still to be demonstrated:** a useful automatic
structural view of Go workflows appears feasible. Go's
[syntax tree](https://pkg.go.dev/go/ast) exposes loops, branches, calls, and
source locations. A focused analysis could retain those structures around
recognized agent and validation calls, expand resolvable helpers, and leave
unresolved behavior explicitly opaque. Showing a loop does not require
predicting its iteration count or future agent replies.

A raw compiler control-flow graph is not the intended user interface. For
example, Go's [CFG utility](https://pkg.go.dev/golang.org/x/tools/go/cfg)
works within a function and omits branch conditions from its edges. A
reader-oriented view needs to preserve the source's meaningful structure.
Dynamic call targets and concurrent execution require further handling;
diagram support should not limit which ordinary Go programs may run.

**Tyler's proposed authoring approach:** combine static extraction with an
agent session that builds a helpful visual explanation. Static analysis can
identify relevant operations, their source locations, enclosing structures,
and flow relationships it can establish. The agent can arrange those
elements spatially, name phases, group meaningful sections, and add
conceptual relationships that help explain the work. The purpose is human
understanding.

For example, research may inform both implementation and validation even
when those relationships are not direct execution-order edges. A coach can
be placed alongside the work it observes. A shared body of indexed knowledge
can appear as an information resource used by several participants; it need
not become another executable action type.

**Proposed presentation rule:** distinguish program-derived flow,
runtime-observed activity, and agent-interpreted conceptual relationships
through clear labels or visual treatment. Conceptual links explain such
things as information use, responsibility, or steering; they do not silently
assert an execution dependency. Source links let readers inspect the basis
of the explanation. The Go program continues to define execution, and the
diagram can be regenerated and edited for clarity.

Runtime visibility remains essential regardless of static rendering. It
should show current work, prior attempts, waits, outcomes, time, and token
use, with actual invocations linked to their enclosing work. Context
inspection and steering relationships belong alongside that history: an
execution-order arrow cannot explain what an agent received or why a supervisor
intervened.

## Scope of this direction

The [five arts](five-arts.md) remain the underlying aims and tensions.
Readable source aids authoring; context inspection aids understanding and
steering; distributing responsibility can help individual agents while
making the whole harder to follow. These are tradeoffs to examine in use.

The [narrow Go POC](../ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md)
establishes a starting point. This direction does not add
static rendering, context diagnostics, comprehensive telemetry, or an eval
platform to its first version. It gives the primitives, examples, and future
tools a common test: do they help people understand and improve the agent's
path to successful work?
