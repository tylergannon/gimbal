# The Gimble Constitution

Gimble is a Go library and in-process web application for writing, running,
observing, and steering agent workflows. Its purpose is to help humans and
agents define work clearly, put the right information within reach, carry the
work through to demonstrated results, and learn from what actually happened.

This document defines the product promises. It is deliberately lower resolution
than the public API: it should remain legible while individual names and data
types improve. A release may claim a promise only when the promised behavior has
been demonstrated. `ephemeral/research/api/SPRINTS.md` records delivery status;
this document does not imply that every promise is already shipped.

## Promises

### 1. A workflow reads like a small Go program

An author can express a useful workflow in roughly a page or two of clear,
idiomatic Go that reads like pseudocode. Its stages, loops, branches, concurrent
work, checks, and exit conditions remain visible in the workflow source.

**Verified when:** representative workflows can be understood from their main
Go function without reconstructing a graph, following framework plumbing, or
opening generic tactic helpers to discover the decisions that define the work.

### 2. Ordinary Go defines control flow and parallelism

Workflows use Go functions, `for`, `if`, `select`, channels,
`context.Context`, and `errgroup.Group` directly. Gimble does not replace these
with a workflow language, futures, special edge types, or a parallelism DSL.

**Verified when:** pipelines, fan-out/fan-in, first-finisher races, critique
rounds, and nested loops can be written with ordinary Go primitives, including
`errgroup.WithContext`, and their policies remain explicit at the call site.

### 3. Gimble exposes primitives, not tactics

The public API names only irreducible runtime capabilities that real workflows
need. A bake-off, sprint, delivery loop, research campaign, or critique circle
is a program written from those capabilities, not a high-level Gimble wrapper.

**Verified when:** new workflow strategies can be added as ordinary Go without
adding exported framework names, and any new exported name is justified by an
actual workflow needing runtime support that ordinary Go does not provide.

### 4. The same small vocabulary composes into many workflow shapes

Straight pipelines, planner/builder/checker loops, parallel drafts, peer
critique and synthesis, bounded promise loops, nested chapter and sprint loops,
and supervisors alongside active work are all compositions of the same basic
agent, scope, context, and recording capabilities.

**Verified when:** published examples cover these shapes without introducing a
different execution model for each shape, while still leaving each workflow's
decisions visible in its own code.

### 5. Program shape is separate from program input

The workflow defines a reusable method. Typed input defines the particular
goal, constraints, source material, models, evidence contract, and information
environment for one run. The available data and its index may grow during the
run without changing the program's shape.

**Verified when:** the same compiled workflow can run against different typed
tasks and document collections, and the same task can be used to compare two
workflow or context arrangements.

### 6. Agent work is conversational, typed, and portable

Gimble presents one coherent session-and-turn model across supported coding
agent harnesses. A session preserves conversation across turns; a turn may
return text or a schema-validated Go value. Harness adapters preserve useful
native capabilities instead of forcing every provider down to a lowest common
denominator.

**Verified when:** the same workflow runs against each supported harness,
structured results fail closed when invalid, follow-up turns retain the
conversation, and supported steering, interruption, and forking behavior works
through the common API.

### 7. An agent receives one information environment organized around success

For each assignment, the goal, immediate responsibility, requirements, rules,
current state, feedback, tools, documents, and routes to further information
form one designed context. Some of it may be in the first message and some on
disk; storage location does not make it a second kind of context.

**Verified when:** an observer can inspect what the agent was asked to achieve,
what information was placed directly in front of it, what it was told it could
retrieve, and which version of that information was available for the turn.

### 8. Documents and indexes are first-class workflow data

A workflow can collect research, decisions, evidence, and work products into a
local, searchable document collection, then build and maintain a compact
semantic index that routes agents to cited source material. The index is a
legible routing structure, not a requirement for embeddings or a vector
database.

**Verified when:** an indexing workflow can ingest or update a real document
collection, preserve resolvable citations, expose known gaps and freshness,
and demonstrate that representative agent questions reach useful sources in a
small number of retrieval steps.

### 9. Work has explicit ownership and bounded lifetimes

Every operation obeys `context.Context`. A run owns its scopes, a scope owns the
sessions and child work created within it, and concurrent work is joined before
its owner returns. Cancellation stops the owned native agent work; a browser
disconnect stops observation, not the run it was observing.

**Verified when:** cancellation and failure tests show that child work and agent
processes do not leak past their owner, parallel siblings are joined, and a run
started by the application survives the request that started it.

### 10. Completion follows evidence, not an agent's assertion

Work is defined by falsifiable promises with explicit scope and evidence.
Commands provide facts; independent agent judgment checks whether those facts
actually demonstrate the requested behavior. Supervisors may steer quality and
scope while work is underway, but they do not manufacture or veto completion.

**Verified when:** each claimed result maps to behavioral evidence that a coder
could not satisfy while the promise remained false, invalid evidence routes
back to work, and neither a success message nor a hand-edited status can bypass
validation.

### 11. Every run leaves one inspectable record

The runtime records the run's structure and activity as it happens: scopes,
agent sessions and turns, prompts, context values, model and harness identity,
tool activity, results, errors, validation, steering, timing, and reported
usage. Live observation and later replay read the same durable record.

**Verified when:** an independent reader can follow a run while it is active,
stop at its recorded completion, replay it after the process exits, reconstruct
the relationships and concurrency that occurred, and investigate where time,
tokens, failures, or unwanted steering accumulated.

### 12. The in-process web application makes runs observable and controllable

The process serves a web application that shows live and past runs, their
workflow structure, scoped information, agents, turns, transcripts, outcomes,
time, and usage. A person can steer or interrupt an active agent and cancel an
active run, with every intervention attributed in the record.

**Verified when:** a browser can watch a real workflow update without polling a
separate system, replay the same run from disk, inspect what each agent saw and
did, and exercise steer, interrupt, and cancel against live work without those
controls altering historical runs.

## Boundaries

- Gimble is not a graph-definition language. A graph or timeline is a derived
  explanation of source and observed execution; it does not constrain which
  ordinary Go programs may run.
- Gimble does not ship a catalog of opaque orchestration tactics. Reusable
  domain workflows may exist, but their method remains readable Go.
- Gimble does not promise crash recovery, deterministic replay, distributed
  scheduling, or an immortal agent process. Those require a separate contract
  if a real workflow needs them.
- Gimble does not inject a universal prompt or require one giant context
  document. Workflows decide what each participant needs, and make the rest
  discoverable.
- An index is not evidence merely because files exist, and a check is not proof
  merely because it exits successfully. Both are judged by the behavior they
  enable or demonstrate.

## Document authority

- This constitution defines **what Gimble promises** at product level.
- [`ephemeral/research/api/API.md`](ephemeral/research/api/API.md) defines **the
  behavioral contract of the public API**. It should explain exact observable
  semantics and small examples, not carry roadmap status or preserve every
  research argument.
- [`ephemeral/research/api/SPRINTS.md`](ephemeral/research/api/SPRINTS.md)
  defines **what is planned and what has shipped**.
- [`docs/definition-of-done.md`](docs/definition-of-done.md) defines **how a
  claimed behavior is accepted**.
- Research, methodology, and rejected alternatives under `ephemeral/` explain
  **why** decisions were made, but do not silently expand the promises above.

When these documents disagree, the contradiction is a decision to resolve, not
permission for an implementer to improvise a weaker promise.

## Source trail

The current workflow definitions live in [`.agents/skills/`](.agents/skills/).
The most relevant methodology and prompting notes are:

- [`workflow-designer/rules.md`](ephemeral/legacy/ephemeral/projects/gimble/workflow-designer/rules.md)
  for promise-led planning, validation design, supervision, and decision
  authority;
- [`workflows-as-programs.md`](ephemeral/legacy/docs/workflows-as-programs.md)
  for readable Go workflows, the first-message discipline, and the unified view
  of context and indexed information;
- [`direction.md`](ephemeral/legacy/docs/direction.md) for program/input
  separation, research and indexing, telemetry, and the distinction between
  source and runtime views;
- [`programmatic-workflows/`](ephemeral/legacy/ephemeral/projects/gimble/programmatic-workflows/)
  for the recovered workflow source, concurrency shapes, and Tyler's preserved
  source statements; and
- [`sprint/sprints.go`](sprint/sprints.go) for the prompts used by the first
  workflow implemented on the new runtime.

These sources contain both accepted direction and explicitly unreviewed or
historical proposals. Their status labels and dates remain significant.
