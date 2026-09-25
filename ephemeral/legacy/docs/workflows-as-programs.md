# Workflows as programs

Architectural direction stated by Tyler on 2026-09-08 and clarified on
2026-09-09. This is an edited synthesis of
[his original statement](../ephemeral/projects/gimbal/programmatic-workflows/KEY-CLAIM-VERBATIM.md)
and [his context-engineering clarification](../ephemeral/projects/gimbal/programmatic-workflows/CONTEXT-REFRAMING-VERBATIM.md),
each preserved separately with only the retired project name normalized.
It describes the direction of Gimbal's Go-program work; the context and
indexing behavior described here is not yet implemented.

The broader [Gimbal direction](direction.md) places this work in the
experience of authoring workflows, the legibility worth preserving from
graphs, and the future ability to examine context choices through run
telemetry.

**Gimbal should organize work and information together, so each agent has
a clear objective, a responsibility it can handle, and a direct path to the
knowledge it needs. The workflow carries responsibilities we should not ask
any one agent to carry in its head.**

The purpose is to help agents achieve the intended result more accurately,
faster, and at lower token and monetary cost. Ordinary Go programs give us
freedom to express how work proceeds. Context engineering guides what each
participant needs to understand and discover along the way.

## Why author workflows as programs?

DAG-style workflow definitions work well for very simple workflows. In
practice, authoring Gimbal's graph language proved genuinely difficult rather
than easy as intended. Modeling workflow in general requires authors to
reconstruct ordinary program structure through nodes, edges, routing fields,
and graph-specific scoping rules. The art of agentic orchestration is changing
rapidly, and its tactics and useful primitives need to evolve just as quickly.

**Postulate:** as a workflow shorthand grows to express more of what authors
need, it tends toward becoming a pseudo-programming language, perhaps
asymptotically.

Our response is to author workflows directly as programs in a programming
language. For Gimbal, that language is Go. Loops, conditions, functions,
composition, and concurrency can be expressed using the language itself.
New orchestration shapes become new programs or ordinary reusable functions.

That is a substantive reason for Go, not merely familiarity or nicer syntax.
Go already provides goroutines, channels, selection, cancellation patterns,
and ordinary lexical scope, making concurrent and asynchronous orchestration
natural to express directly.

This frees Gimbal from many of the constraints of a graph-definition
language. It also gives us a practical design metaphor: the functions and
types we build should make orchestration feel natural to program. The
workflow's algorithm should remain visible in its source, with input
validation, defaults, and prompt construction separated from control flow.

The program's shape should also remain distinct from the input supplied to
start it. That input establishes the task and its available information,
including the semantic index's structure and quality. Data can grow and the
index can evolve during execution while the program continues to express
the method that works with them. This
[separation of shape and input](direction.md#separate-program-shape-from-its-input)
is a high-level design principle for reuse, comprehension, and comparative
evaluation.

## Preserve the legibility of a good workflow diagram

A good graph makes a workflow's stages and progression understandable. That
is a strength to preserve as authorship moves into Go. A reader should get
an excellent sense of a workflow from roughly a page of code that reads like
pseudocode: stages, loops, conditions, checks, and the relationships between
participants.

This standard governs the design of functions, types, examples, and
authoring guidance. Names should explain meaningful work and results.
Composition should reveal understandable sections. Indirection and DRYness
are useful when they improve the reader's understanding; shortening a
workflow by hiding its important decisions in generic helpers works against
the objective. Input preparation and prompt assembly can live elsewhere
while the decisions that define the orchestration remain apparent.

Context choices are part of that shape. The reader should be able to see
which responsibility and context each participant receives, and then inspect
the assembled request and its indexed information when detail is needed.
All that detail need not be inline in the workflow source.

Automatic diagrams may complement readable source. The
[direction document](direction.md#source-views-and-runtime-views) describes
the prospective boundary: recognizable static structure, explicit opaque
regions, and runtime views of actual activity and context. No static
extractor has been implemented.

That explanation can combine static analysis with an agent's grouping,
layout, and conceptual links. The [broader direction](direction.md) records
this proposed authoring pass, a small action vocabulary with visible task
roles, and research/indexing as part of preparing the work's information.
The diagram serves understanding while Go defines execution.

## One context, organized around success

For a given agent, we design one context: the information environment in
which it pursues its objective. Goals, requirements, rules, available means,
current work, feedback, the semantic index, and the files it points to all
belong to that context. Some information is immediately in the model's
context window; some is available through retrieval. Where information is
presented is a choice within this environment.

On-disk information is a first-class part of this design from the beginning.
Its purpose is to help the agent find what it needs while preserving clarity
about its objective. A concise entry point with excellent paths to detail
can be useful even when all the detail would fit in a prompt. Token limits
matter, but reaching one is not the reason to organize information this way.

The semantic index can be the task's entry point itself: goals, requirements,
rules, current scope, and routes to supporting material brought together
around what the agent is trying to accomplish. As work advances, that entry
point evolves. A failing validation can lead directly to the relevant
requirement, observed behavior, and implementation area. The index helps the
agent decide where to look next, as well as locate a file.

This is the design hypothesis to pursue. It does not require one enormous
document, a universal task schema, or identical views for every agent. The
index and its contents remain ordinary files: plain-text index files,
documents, images, and other artifacts agents can read, search, and inspect.

## The first message should orient the agent toward success

Constructing an agent request begins with what the agent needs to succeed.
The current loop frame is one contribution to that judgment. The initial
message should make clear:

- What outcome this assignment serves, what this agent is responsible for,
  and how success will be recognized.
- Which constraints and available means matter for its immediate work.
- Where the work stands, including relevant feedback and reasons to change
  direction.
- Where to find the information it may need, what those paths contain, and
  when to consult them.

These belong in a coherent entry point. The objective and immediate
responsibility stay prominent; useful routes make the rest discoverable
without requiring the agent to read the whole collection first. An agent
should be able to connect a question about its work to the material that
will help answer it.

Scoped context remains useful for assembling this view. A goal encloses a
chapter, a chapter encloses a sprint, and a sprint may enclose an attempt.
The selected item, applicable requirements, validation, and feedback can
follow those scopes automatically, leaving step instructions as simple as
"execute the current step" or "validate the current work."

The stack metaphor helps explain scope and lifetime. It does not decide
what deserves the agent's attention: an outer goal may matter more than the
innermost loop detail, and a requirement on disk may govern the next action.
Scope depth and storage location do not determine importance. Ending a scope
removes its automatic contribution to later requests; useful findings and
work products can remain available through the index.

## Orchestration distributes attention across agents and time

Context engineering also includes deciding which agent should attend to
which information, and when. We deliberately divide the burden of knowing
what to do and judging whether it was done well.

A builder concentrates on making the requested change. A supervisor can
observe that work from a broader perspective and steer it away from
overengineering. A checker later examines the result against the intended
behavior and can send it back for correction. Each assignment foregrounds
the information relevant to that responsibility while preserving its
connection to the overall goal.

The workflow therefore shapes both the sequence of actions and the
distribution of knowledge and attention. An index directs an agent toward
information; orchestration places an agent with the right responsibility
where that information can change the work. Artifacts and feedback connect
those perspectives across time without requiring one agent to remember
every previous turn.

This serves the [five arts](five-arts.md), with their existing tensions:
dividing responsibility can reduce an individual agent's burden while
increasing handoff cost or obscuring the whole. Each agent still needs enough
of the larger objective to avoid succeeding locally while the project fails.

The program metaphor earns its place by making this arrangement easy to
express and change. Its completeness is not a design objective. The test
of a workflow and its information layout is whether their combination helps
agents achieve the intended result with less confusion, wasted work, time,
and cost. Comparative evals can vary both the orchestration and the context
arrangement against the same task and established validation.

## Implementation record

The Go-program runtime is not present on `main`, but an existing unmerged
POC is preserved. Start with the [recovery guide](../ephemeral/projects/gimbal/programmatic-workflows/POC-RECOVERY.md)
before designing a replacement. It locates the implemented `program` package:
typed `Codergen[T]` calls, command execution, validation, and
`Loop(...) iter.Seq2[Iteration, error]`. Its sprint, chapter, and delivery
programs use ordinary Go control flow, with inputs and prompts separated
from their algorithms.

The earlier [concurrency sketches](../ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md)
express critique circles and bake-offs through ordinary goroutines/errgroup.
That concurrency work, automatic scoped context, the task-oriented index
entry point, and unified assembly of agent requests remain proposed behavior.
The examples currently use canned callbacks and symbolic workspaces while the
argument shape and role direction remain illustrative.

Until that refactor lands, the root README should continue to describe the
graph engine that Gimbal actually ships. Rewrite it when the implementation
changes rather than presenting this direction as current behavior.
