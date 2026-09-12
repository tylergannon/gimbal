# Promises, dispatch, and completion

Design direction from the discussion of [issue #125](https://github.com/tylergannon/gimble/issues/125),
2026-09-11. This records the direction Tyler endorsed and the questions still
open. It is not a shipped API contract or an implementation plan. Godoc and
compiling examples continue to define the current public API.

The short [execution note](LOOP-WORK.md) records what will change and the
evidence needed to finish #125.

## The distinction

Promises describe what must be true. Dispatch decides what is worth doing
next. Overall assessment decides whether the requested outcome has been
achieved sufficiently to stop.

| Responsibility | Question |
| --- | --- |
| Promises | What conditions define success? |
| Dispatch planner | Given results, failures, remaining gaps, and priorities, what work should happen next? |
| Overall assessment | Have we achieved enough of the requested outcome to stop? |

The loop iterator belongs to dispatch. Its backlog is a revisable working
plan. Promises give the planner direction, but they are not its queue.

A promise that a person can reliably cancel a running agent might require
investigating a harness, fixing ownership, reproducing an intermittent
failure, or demonstrating that existing behavior already works. One task can
serve several promises; several tasks can serve one promise. A failed attempt
can reveal that a different assignment should come next.

There is not necessarily a sensible next promise. There is a sensible next
task, given what we now know. Making a promise list the iteration list would
confuse the definition of success with the decomposition of work.

## Task descriptions trust the worker

Explain the desired end state, why it matters where useful, and necessary
information the worker would be unlikely to discover independently. Let the
worker choose the approach. Procedural detail can signal that the task needs
decomposition. Necessary constraints remain valuable; obvious advice,
pedantic command checklists, speculative implementation prescriptions, and
untested or unverified code do not belong in the assignment.

The example Tyler endorsed:

> **Before:** Run these seven commands, inspect these functions, introduce a
> cancellation manager, wire it into the handler, and add tests.
>
> **After:** Runs started from the page must survive browser disconnection and
> remain explicitly cancellable. The agent harness runs in a separate process,
> so ending the HTTP request does not establish that agent work stopped.

The additional fact belongs only when verified and useful to that worker.
Shared knowledge belongs in scoped context rather than being repeated in
every task.

A quick editing pass in the planning workflow is a candidate for enforcing
this discipline. It would subtract unnecessary instructions, preserve useful
facts and constraints, and flag ambiguity rather than invent requirements.
This is a workflow idea, not a new Loop primitive.

## What a dispatched task carries

The current recommendation is a small structured assignment:

| Information | Meaning |
| --- | --- |
| Name | A short label for recognizing the work. |
| Description | The desired result and necessary, non-obvious information. |
| Definition of done | How to recognize that this assignment succeeded. |
| Validation | A command, an agent question, or both, where useful. |

These are conceptual fields; their exact Go types are not settled. In
particular, this does not adopt a list of promises as every task's definition
of done. A task may have several acceptance conditions without becoming a
Promise Loop.

For example, an investigation can be defined as:

**Name:** Harness cancellation  
**Description:** Determine what cancellation actually stops in each supported
harness.  
**Definition of done:** Each harness has an evidence-supported account of its
cancellation behavior, with unresolved cases identified.  
**Validation:** Assess whether observations support the conclusions and whether
unresolved cases are stated accurately.

Discovering why cancellation fails can successfully complete this task while
the wider cancellation promise remains unfulfilled. Dispatch responds by
selecting the repair. Research promises an evidence-supported answer, not a
predetermined favorable finding.

A validation request is distinct from its result and evidence. A known,
verified command may supply evidence; the planner need not invent an
executable check before the capability exists. Tasks carry no iteration
counter. Counts and limits are ordinary Go in the workflow.

## Promises belong to the scope whose success they define

Adopt the useful core of [df-promise](../../../.agents/skills/df-promise/SKILL.md):
a precise, scoped claim supported by evidence. Recurring execution, badges,
budgets, fingerprints, and a full freshness system are separate machinery;
this discussion does not adopt them into Loop.

Promises should describe observable outcomes at a useful scale. They must
not become microscopic implementation instructions under a different name.
Evidence should identify which claim it supports. Passing every listed claim
does not by itself establish that the list covers the requested goal.

The original goal remains authoritative when a planner revises its working
plan. If stopping a harness process is difficult, the planner may dispatch
an investigation. Replacing that obligation with "the page displays
cancelled" cannot establish success. Changing the requested outcome requires
the authority that owns that requirement.

Fulfillment concerns the state examined. Later changes can invalidate earlier
evidence. Changing run ownership may require reassessing cancellation;
an unrelated typography change ordinarily would not. The exact mechanism for
tracking and reassessing evidence remains open.

Scoped context should supply the wider promises, relevant evidence, previous
failures, priorities, and supporting information to each call site. The task
does not need to repeat that entire information environment. How completed
task feedback reaches the next planner call must be explicit; do not assume
that child-scope writes automatically become parent values.

## Completion authority and the CEO idea

Tyler proposed a "CEO" agent, supervising the supervisors, that periodically
rechecks the promises and tells the workers to stop when the job is done.
This fits an overall assessment role that can redirect effort and recognize
completion independently of individual assignments.

Giving that agent authority to end work goes beyond the advisory role of
ordinary supervisors. That authority should be explicit in the workflow.
The assessment cadence, communication mechanism, and treatment of in-flight
work remain open; no CEO API or automatic termination behavior is adopted.

Every promise must meet its validation standard before the work is ready.
For agent judgment, that standard is "it works and it's 90–95% done," applied
to each promise. This is subjective release latitude, not a numerical score
or permission to leave some promises unmet. It deliberately gives validators
room to release working software with minor remaining issues rather than
keep pursuing perfection, consistent with the repository's
[definition of done](../../../docs/definition-of-done.md).

Deterministic validation is binary: its check passes or fails. The percentage
does not apply to a command's result or permit a failing required check to
count as passing. Where a promise uses both deterministic checks and agent
judgment, the checks must pass and the agent must judge that the working
result meets the readiness standard. Remaining issues are documented as the
definition of done requires.

The iterator yields work. The workflow decides when the job is done.

## Choosing the greatest concrete gain

Loop must work with an existing sprint plan or with only a goal and
prioritized promises. A sprint plan supplies candidate work, dependencies,
and prior reasoning; it is not a prerequisite for dispatch. The planner can
adapt that plan as the work changes, while preserving the requested outcomes
and binding constraints. Without a plan, it can start with an empty backlog
and select the next assignment directly from the promises and current evidence.

Tyler refined the earlier "next doable task" framing to:

> you want to be picking the next thing that will concretely gain the most
> points towards the Definition of Done.

Proposed planner language:

> Choose the next assignment that offers the greatest concrete gain toward
> the overall Definition of Done, sized for the intended worker to complete
> and demonstrate in one working session. Judge that gain from the priorities,
> current evidence, and actual dependencies. Advance to later work when it
> offers more progress than repairing or finishing the current phase;
> unfinished phases do not themselves block dispatch.

"One working session" is a sizing judgment about a bounded assignment, not a
one-turn limit or a requirement to create a fresh native Session each time.
It includes understanding the relevant context, doing the work, and gathering
evidence of the result. Independent validation may use another session. The
intended worker's capabilities and available resources matter; there is no
universal file, line, token, or minute count that defines the right size.

A useful assignment has a coherent result, dependencies that are available
or explicitly within scope, and a credible way to establish what happened.
Keep coupled changes together when they are necessary to demonstrate the
selected outcome. Split unrelated outcomes or work whose uncertainty makes
completion implausible. Avoid tiny bookkeeping assignments that consume a
new round of orientation without delivering meaningful progress.

Priority applies to the contribution, not just the immediate task label. A
prerequisite or investigation can be the best next step toward the most
important promise. Repeated failure should change the next assignment or
expose a blocker, rather than automatically dispatch the same oversized task.
Dependencies and architectural constraints inform which assignment offers
the greatest gain. Phase headings do not impose additional completion gates.

For example, a phase may be about 87% complete with a remaining defect, while
the next phase can already proceed. If advancing provides more concrete gain
toward the overall definition of done than fixing that defect now, dispatch
the later work and retain the defect in the remaining-work record. Repair
comes first when the defect actually blocks or would undermine that progress.
This is greedy selection toward the goal, not phase-by-phase perfection.
"Points" expresses comparative judgment; it does not introduce a numerical
scoring system or a completion-percentage field on Task.

Deferring a repair changes work order, not validation truth. A failed
deterministic check remains failed, and every promise must still meet its
validation standard before overall completion.

This synthesis draws on the named skills as design sources, without adopting
their orchestration machinery:

- [df-easy-loop-simple](../../../.agents/skills/df-easy-loop-simple/SKILL.md)
  asks the coder to choose a coherent subset for one visit and supports
  continued sessions. Work boundaries and conversation lifetimes are distinct.
- [df-sprint-plan](../../../.agents/skills/df-sprint-plan/SKILL.md)
  supplies grounded intent, dependencies, feasibility, and definitions of done.
  It does not define a numeric session-size rule.
- `orchestrate-attractor-loops` recommends coherent feature boundaries and
  avoiding bookkeeping-sized tasks. `write-prompts` favors concise intent,
  useful context, and leaving deterministic enforcement to tools and code.

This is proposed planner language. Its usefulness must be judged from actual
assignments and results, not established by a prompt snapshot alone.

## Proposed Loop contract

The following makes the discussion concrete for #125. It is the recommended
contract for review, not a description of implemented behavior.

### Public shape

Keep `Loop(ctx, name, goal, planner)` and `Err()`. Replace `Laps` with `Tasks`,
yielding the task's `context.Context` and a structured `Task`. Use a fixed task
type for this revision; no concrete workflow currently requires a type
parameter. Genericity can be reconsidered against an actual typed dispatch
requirement independently of the promise design.

The task has `Name`, `Description`, and `DefinitionOfDone` string fields, plus
`Validation` containing `Command` and `Query` strings. Its name, description,
and definition of done must be nonblank. Its definition of done describes
successful completion of this assignment, which can include discovering a
failure or establishing uncertainty. It does not duplicate the enclosing
scope's promise list.

`Validation.Command` identifies a known executable check and
`Validation.Query` describes an assessment for a validator agent. Either or
both may be supplied. Without an explicit validation recipe, the workflow
still assesses the task against its definition of done; an absent recipe is
not evidence of success. The type carries neither results nor completion
flags. A planner-authored task name is a label, not a runtime scope identity.

### Dispatch and feedback

Loop opens a named scope. Before each dispatch, the planner receives the
original goal, current backlog, relevant scoped information, and the previous
assignment's recorded results. Scoped information can contain prioritized
promises and an optional sprint plan. The backlog may initially be empty;
Loop does not require the caller to precompute an iteration list. The planner
chooses the next useful assignment, sized as described above, based on what
happened and what remains. It can repeat, split, replace, or reorder work.
Priority is reflected in selection and backlog order; a priority enum or
counter is not required on every task.

Each assignment opens a child scope containing the structured task. The body
receives the same task value. It performs the work and uses the ordinary
scope-data API to record its result and validation evidence before returning.
Loop explicitly includes those completed task-scope values in the next
planner prompt as the previous assignment's record. This is a feedback
projection, not promotion of child values into the parent scope. The
planner's next call also receives the currently visible parent information.

Loop does not infer success from the body returning. Failed validation is
information for replanning; the workflow can record it and let dispatch
continue. A fatal error can still leave the surrounding workflow through
ordinary Go control flow. Each task's scope and sessions end when its body
ends, including when the range is broken or the workflow returns.

### Validation stays visible in the workflow

The recommended contract treats the task's validation fields as data for the
workflow. The workflow runs commands and calls its chosen validator session,
then records the actual results. Loop does not silently choose a validator
model, interpret a query as a command, or turn a failed check into a pass.
When both forms are required, a passing agent judgment cannot override a
failed deterministic check.

This deliberately replaces the current automatic sweep of backlog commands.
It keeps execution, validator selection, and retry policy visible in Go while
Loop supplies the feedback to dispatch. The description-editing pass is
likewise a workflow concern, not an additional automatic Loop agent.

### Ending dispatch

When the planner selects no further assignment, `Tasks` returns normally.
That records the planner's decision to end dispatch; it is not an independent
attestation that the enclosing promises are fulfilled. The workflow applies
its completion assessment and can initiate further dispatch with the unmet
outcomes. It may also stop dispatch with an ordinary range break.

Cancellation follows `context.Context`; `Err()` reports failures encountered
by Loop, such as persistence, planner, schema, or cancellation errors. A
validator finding an unmet condition is not itself a Loop infrastructure
error. Counters, budgets, and decisions to interrupt or finish in-flight work
remain in the enclosing workflow.

### Durable agreement

The planner output, yielded value, task-scope data, and durable task record
must preserve the same structured assignment. The backlog records tasks with
this same shape rather than reducing them to strings. The durable record must
also retain the dispatch decision and the recorded feedback that informed
the next decision. Later backlog edits must not rewrite the historical task
that was actually dispatched.

Whether the implementation validates an agent-edited backlog or persists a
structured planner revision is an implementation choice to resolve before
coding. Either way, an inconsistent or malformed assignment must not be
silently yielded. No compatibility form for `Task{Lap, Text}` is proposed.

### Evidence for the replacement

Demonstrate a dispatch in which the first assignment produces an unsuccessful
result, the planner receives that result and relevant parent context, and the
next assignment addresses what was learned. Demonstrate selection directly
from prioritized promises without a sprint plan, and adaptation of candidate
work when a supplied plan no longer fits the observed state. Include a case
where a nonblocking defect remains in an earlier phase and the planner
selects later work that offers greater progress while retaining that defect.
Assess whether
the assignments have coherent, useful outcomes and are reasonably sized for
the intended worker, including the work needed to demonstrate their results.
Verify that the yielded task,
scope data, backlog, and events agree, and that normal exhaustion is distinct
from recorded validation. Exercise early break and cancellation to verify
scope lifetime. A live run must show outcome-focused assignments and adaptive
planning; fake-adapter checks alone do not demonstrate that authoring behavior.

The CEO workflow, promise storage and revalidation, and overall-completion
signaling remain subsequent design work. They do not need to be built to
replace the current task contract.

## Original sources

- [Legacy checklist item](../../legacy/checklist/checklist.go): name,
  acceptance check, command, inference question and evidence paths, and
  document/checklist references.
- [Legacy iterator](../../legacy/program/loop.go): structured item and feedback,
  with mechanical validation and selection.
- [Compact workflow sketches](../../legacy/program/workflows/workflows.go):
  sprint, chapter, and delivery workflows with ordinary Go control flow.
- [Earlier Loop design](API.md#loop): the deliberate move from mechanical
  first-open selection to planner-directed work. Recovering meaningful task
  structure does not require restoring the old selection algorithm.

## Tyler's words

These excerpts preserve the framing that led to this direction:

> We focus on explaining the end state, providing any necessary information
> that the agent might not look for on its own, and trust it to get the job
> done right.

> Getting to a point where you need to provide instructions is a good sign
> that the task should be decomposed.

> We DEFINITELY want these to be part of our workflow, somewhere.

> But I'm not sure it's appropriate for an iteration list.

> The loop iterator is like a dispatch.  And it needs to respond to what work
> got done, what work got done badly, and what work remains (and its priority),
> and whether we've reached that "the functionality works and the requirements
> are 90-95% met" criterion.

Tyler clarified how that threshold applies:

> true, we should expect that each of the promises needs to be met to the
> extent of "it works and it's 90-95% done".
>
> That means all the promises need to meet that level.  That's a subjective
> thing for when you're asking a validator agent to judge whether software is
> ready to go.  We give them that little bit of extra rope otherwise they NEVER
> release software.  Meanwhile, if the validation is deterministic then it's
> binary, not a percent.

On planning only as far ahead as the work supports:

> I think there should be the ability to do it without a sprint plan per se,
> just a prioritized list of promises, and then ask the planner to come up
> with a task that's got the right size.

> maybe "what's the next doable task that meaningfully moves us towards the
> goal" is better.

Tyler then sharpened the selection rule:

> you want to be picking the next thing that will concretely gain the most
> points towards the Definition of Done.  That means if the last phase is
> like 87% done but a little broken, but we can still start the next phase,
> we should do it.

> Greedy algorithm towards the goal.
