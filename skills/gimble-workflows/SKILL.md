---
name: gimble-workflows
description: >
  Design and author Gimble workflows as ordinary Go. Covers workflow
  structure, agent roles, context and handoffs, adaptive planning, supervision,
  independent validation, completion, and CLI documentation. Use Godoc for API details.
---

# Author Gimble workflows

Write a program whose purpose, decisions, and evidence are visible in its
source. Aim for the clarity of a page of pseudocode. Research, bake-offs,
critique, retries, worktrees, and delivery are tactics written inline with
ordinary Go and Gimble's primitives.

Use Godoc for the current API. For an existing command, use
[Use Gimble](../gimble-runs/SKILL.md); for building and installing the application,
use [Build and release](../gimble-release/SKILL.md).

## Define the outcome and its proof

State what the workflow should accomplish, its scope, and what someone must
observe to know it worked. Give agents outcomes and useful verified facts;
leave their approach open unless a particular method is a requirement. A plan
can change as facts emerge. The original goal remains authoritative.

Separate three things in the design:

- **Program:** roles, branches, concurrency, feedback, and stopping decisions.
- **Inputs:** the goal, project, issue, plan, or candidates for this invocation.
- **Knowledge:** material agents can retrieve while working.

The program should remain understandable as its inputs and knowledge change.
Put task data in parameters, scoped context, and local files rather than
encoding each new assignment into the orchestration.

## Choose the smallest useful shape

Use a single agent or a sequence when that is enough. Add a role or stage for
useful context, independent judgment, or a handoff the work actually needs.

| Need | Shape |
| --- | --- |
| One well-scoped task | One agent turn. |
| Dependent stages | A sequence with explicit results passed onward. |
| Independent investigations or candidates | Concurrent branches followed by a join. |
| Several agents need the same initial research | Research once, then fork conversations. |
| Competing implementations | Separate worktrees, followed by a judge inspecting actual candidates. |
| Criticism and revision | Explicit critique/revise rounds with a stopping condition. |
| A known finite collection | Iterate over the supplied items. |
| The next useful assignment depends on new evidence | A planner-driven loop with worker and validation feedback. |

Keep each tactic visible in the workflow. A helper that hides the orchestration
can make a shorter file harder to understand. In the Gimble repository, new
exported API names require Tyler's explicit request.

Read [workflow patterns](references/patterns.md) when choosing how research,
parallel attempts, critique, or adaptive implementation should fit together.

## Give roles clear responsibilities

A researcher establishes facts and useful source locations. A planner chooses
the next assignment. A worker performs it. A validator inspects the result and
whether the evidence establishes acceptance. A supervisor coaches an active
agent. Use the roles the task warrants; naming every possible role does not
make a better workflow.

Preserve independence where it matters. Shared research can save repeated
reading, but a judge must inspect original evidence and actual candidates.
The implementer's summary can orient validation; it cannot establish that the
implementation works. Forking a worker's conversation into a judge carries
its assumptions along with the useful context.

## Make ownership and handoffs visible

Place sessions according to the lifetime of their work. A planner may need
continuity across assignments; a task worker may need isolation from previous
attempts. Reusing a session also reuses its conversation history. Ending a
scope does not erase what that agent already saw.

Every concurrent branch needs an owner and a join before its enclosing work
ends. Cancellation and cleanup are separate concerns. Give parallel editing
agents separate worktrees when they could interfere, and verify where they
actually wrote their changes.

Land crucial information locally before dispatch. Prompts name absolute paths
to the issue, plan, source, or research needed. For a large corpus, retrieve
through a compact index and pass selected evidence or its location. Keep
context relevant rather than copying the whole corpus into each turn.

A handoff gives the next role the exact assignment, relevant files, observed
results, unresolved findings, and decisions. Make feedback explicit; do not
rely on a child scope or provider conversation to become shared project state.
Use local artifacts for knowledge another role must inspect independently.

## Write prompts that leave room for judgment

Use plain English: what to read, what outcome to produce, meaningful
constraints, and the expected answer. Keep prompts inspectable in the source;
task-specific data belongs in context. Read the rendered prompt when diagnosing
bad behavior, since harness instructions and prior conversation also matter.

Use structured answers when the workflow needs to make decisions from fields.
Those field descriptions are instructions to the agent. Otherwise, prose or a
local artifact may be sufficient. Avoid elaborate output contracts that merely
restate what the model already understands.

## Adapt plans without moving the goal

An adaptive loop should choose work from the goal, current backlog, and
observed results. Assign a coherent outcome with a definition of done. An
investigation is valid work when uncertainty blocks a sound implementation.

Feed actual worker results and validation back to planning. Retain unresolved
findings and explain deferred work so replanning does not silently lose it.
A review finding is a claim to investigate against the goal, not an automatic
addition to the goal.

Keep retries, limits, and escalation visible and proportionate to the task.
A supplied plan can seed the backlog without dictating every later assignment.
Ending dispatch and fulfilling the goal are separate decisions.

Spend capability where uncertainty lives. A frontier planner can be worth one
expensive turn when it leaves a strong, bounded plan whose assignments are
reversible and independently checkable. Low-risk background work can then use
cheaper Terra or Sonnet workers when elapsed time matters less than token cost
and the workflow can tolerate a few completed attempts being rejected by
validation before one succeeds. Keep the validator strong enough for the risk,
feed each rejection back to planning, and cap the attempts.

This is a workflow property, not a model-selection slogan. Do not use the
pattern where work is irreversible, validation is weak, or a bad attempt can
corrupt shared state. A provider, harness, or execution error is not a rejected
implementation attempt and does not retry unless the workflow explicitly
handles it.

## Coach against scope drift

For implementation workflows, give the planner, workers, and validators scope
coaches. Their instruction should oppose over-engineering, unrequested
features, gold-plating, and hypothetical edge-case fixes without a reasonable
actual failing unit test.

Coaching is advisory and never gates completion. Put essential constraints in
the initial assignment too: a short turn can finish before its coach looks.
Choose a review cadence suited to the work; excessive coaching can cost more
than the task. Observe whether an objection changed behavior instead of
treating attachment or message delivery as proof of useful supervision.

## Validate behavior and assess completion

Choose meaningful acceptance evidence before implementation when practical.
Do not let a worker weaken that condition to obtain a pass. Record the actual
check result; an agent saying it passed cannot override a failed command.

Use `Check` when an agent should interpret a command result: it records the
command, working directory, exit code, both output streams, and any execution
error in the current scope for the next turn. Its key follows the same rules as
`Set`: it is a compile-time constant written once in that scope. Name repeated
observations explicitly at their call sites, such as `tests.1` and `tests.2`.
A nonzero exit is evidence rather than a `Check` error; failure to execute,
capture, or record is an error. Use `RunCommand` when ordinary Go needs the
returned values itself. `Check` gathers evidence and never certifies success.

Use `Service(ctx, name, workdir, command)` for a foreground dependency that
must live for one scope, such as a development server. Gimble starts the
string through `zsh -c` and continues immediately; successful start is not a
readiness check. Keep readiness in ordinary workflow code with `Check` or the
protocol the service exposes. Any exit before scope shutdown, including exit
zero, fails and cancels the owning scope.

The declaring scope is the lifetime boundary. A service outside an `Iterate`
stays alive across its item scopes; one declared inside an item ends before
the next item. Normal close and cancellation send SIGTERM to the service's
process group, followed by bounded SIGKILL escalation. The command must remain
in the foreground and its descendants must remain in that process group.
Gimble records output and status but does not add restarts, health checks, or
management of resources owned externally by Docker, Overmind, or similar
tools.

A validator examines the work and the legitimacy of its validation. A green
build or test gate establishes only what it exercised. Claims about live
workflows, external steering, or browser interactions require observing those
behaviors. Preserve the distinction between a task result, an execution error,
ended dispatch, and a fulfilled goal.

Return an honest outcome when proof is missing or a real blocker remains.
Style preferences and unrelated improvements do not become acceptance gates.

## Make the workflow callable and understandable

The current built-in authoring path is inside the Gimble checkout. Follow the
small existing review workflow and the repository's generation and command
registration conventions. Generated command support currently depends on
Gimble internals and its web build; do not assume it is an independent generator
for arbitrary external Go modules.

CLI documentation is part of authoring. A caller should understand purpose,
required inputs, meaningful defaults, outputs or changes, completion and proof
expectations, limits, and a useful invocation without reading implementation.
Use enough detail for that workflow, not a fixed word count or boilerplate.

The current generator takes the entry function's doc synopsis for short help,
package documentation for long help, and parameter field comments for flag
help. Put the detailed explanation and examples in package documentation;
extra paragraphs only in the function comment do not become long help.

The run UI requires the generated graph that matches the recorded workflow.
Keep the workflow's `go:generate` directive, run generation after changing its
shape, and compile the generated file into the binary that serves the run. The
generated `init` registers the graph under the workflow's run name. A missing
or structurally stale graph is an authoring/build error: regenerate, rebuild,
and restart the serving binary rather than relying on a graphless run view.

Inspect the generated workflow list and detailed help after changing the
comments. A graph or output schema does not replace caller documentation.
If the desired help requires a generator change, identify that concrete gap.

## Finish with observation

Run the relevant checks and exercise the real workflow with cheap models such
as Luna or Haiku. Inspect actual prompts, handoffs, decisions, validation, and
results. Check that a caller can discover and understand it through CLI help.
Report what you observed, which model ran, and anything still unproved.
