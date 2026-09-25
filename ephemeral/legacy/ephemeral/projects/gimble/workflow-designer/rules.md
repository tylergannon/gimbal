# Rules of planning, design, and execution for the workflow designer

Written by Claude at Tyler's request on 2026-09-03, from the Sep 1 to 3
design sessions, the tie-off notes and issues, and the diffusioninc skills.
Tyler has not yet reviewed this document as a whole.

Each rule is marked with who decided it:

- **Tyler**: decided by Tyler.
- **Agent, accepted**: proposed by an agent and accepted by Tyler.
- **Agent, unreviewed**: proposed by an agent and never put to Tyler. A
  proposal, not a rule.

Read this before planning, designing, or running an interview, loop, or
workflow for this repo.

## Planning

### Plan around promises, not a spec

**Tyler.** The interview determines what the work promises and how each
promise will be proven, and asks until the requirements and the validation
contract are sufficient. A seed that already contains them goes straight to
work.

**Agent, accepted.** A promise has a statement, what it must not imply, a
scope, a verifier, and evidence. A promise that cannot be falsified is
narrowed before it is accepted. In a checklist item the check is the promise,
the command and the inference are its gates, and done is the attestation.
Nothing in the plan exists that serves no promise.

### Elicit promises, then prune the questions

**Tyler.** The planner drafts the promises a user would expect, including
ones the seed never mentions, and asks whether each is promised and what it
must not imply, with a recommendation. Declined promises become exclusions.

**Agent, accepted.** Once the promise list holds still, a question survives
only if its answer changes a promise, its scope, or its verifier. Stop when a
full pass changes nothing. Nothing already answered is asked again.

### Batch related questions, each with a recommendation

**Tyler.** The interview accepts a group of questions at a time.

**Agent, accepted.** One question file per independent group, questions
numbered inside it, each with options and a recommendation; the answer file
mirrors the numbering. Unrelated questions are not batched, and one decision
is not split across files. The one-question-per-file convention in older docs
was a convention, not a rule.

### The human decides what touches a promise; the agent decides the rest

**Tyler.** Decisions not adjacent to a promise are deferred to the
implementer, who takes a reasonable guess and tests it by building the rest.
Moving an internal seam is ordinary work unless another project depends on
it. The human's gates are the interview, the mechanism of proof, and the go
before a run, build, or merge.

**Agent, accepted.** A seam is recorded only where a promise crosses it:
another project, a persisted format, a public API. The brief records
promise-adjacent decisions only.

**Agent, unreviewed.** Any gate list beyond the three above.

### Research feeds the interview and never edits the brief

**Tyler.** Agents do deep research before and after the brief and the initial
interview, in a loop with them: open source and private projects to copy or
port, prior art, lessons, and advice that minimize what has to be invented.
The results live in a research directory with a semantic index.

**Agent, accepted.** Intake writes the first research plan. Brief and research
alternate. Research adds plan entries only through findings the brief
accepts. A tool node halts the loop when findings are empty and no entry is
open, with at most five research branches. Hitting the ceiling is a question
to the human, not a failed run.

### Sizes help the planner write a workflow; they never route to one

**Tyler.** SIMPLE is a single node with an optional supervisor. MEDIUM is a
simple loop: more than one sprint, less than two chapters. LARGE is a chapter
loop containing a sprint loop containing a coding and validation loop. The
planner writes the pipeline, anything from trivial to nested loops. There is
no fixed list of graphs to choose from.

The three canned graphs, the recommendation contract, and validate-plan built
on Sep 2 were agent decisions, never Tyler's, and were cut in PR #38.

**Agent, unreviewed.** LARGE only for twelve or more sprints, following
diffusioninc's floor for a chapter.

## Design

### Chapters are durable vectors; sprints are the executable unit

**Tyler**, adopting diffusioninc's conventions. A chapter is a large,
pre-defined slice of technology enhancement: a long-term vector guiding a
dozen to a hundred sprints, not an executable sprint, under 3000 tokens, with
a pyramid index. Sprints do the work, the sprint ledger is the source of truth
for status, and a sprint's checklist is edited each lap. A plan that needs a
prompt telling agents not to read the chapter plans has too many chapters.
These conventions live under ephemeral, committed but not official
documentation.

**Agent, unreviewed.** A sprint is one agent turn, one fresh context, one
commit, one demonstration; a sprint that needs two turns is two sprints; the
sprint document sections (read first, the work, definition of done, not in
this sprint, ask the reviewer).

### One loop shape at every level, one run

**Tyler.** Any number of nested loops. A loop body is an arbitrary subgraph,
with branching and fan-out allowed and lint rules against jumping out of the
loop. One run, no child runs per chapter. Ledgers are markdown with YAML
frontmatter so the definition of done is prose beside the checklist.

**Agent, accepted.** A chapter is an item whose lap is the sprint loop; a
sprint is an item whose lap is the coding and validation loop; each level's
item may carry its own command.

### Chapter ledger durable; sprint ledger re-planned every lap

**Tyler.** Sprints may be appended one lap at a time or written as a backlog
and edited over time: the plan is a document a planner edits. The checklist
is edited each lap because backlog entries go stale.

**Agent, accepted.** The chapter ledger is written at plan time and edited
only with a recorded reason. A replan turn after every implement lap, in its
own node with a cheap model and fresh context, edits open sprint items only
and never the chapter ledger. A sprint that finds the chapter wrong asks the
human.

**Agent, unreviewed.** Replan never touches the item selected this lap; a
failed verify makes replan append one item from the findings.

### Slice vertically; every slice serves a promise

**Tyler.** Break the work into seams, vertical slices, contracts, phases, and
chapters that evolve toward the full vision. Pass the decomposition through a
fan-out multi-model critique, with rigor scaled to the size of the work.

**Agent, unreviewed.** Every slice yields something exercisable; every promise
reaches a slice, recorded in the ledger; docs and cleanup fold into the slice
they belong to; stack-order horizontal plans are the default model mistake.

### One proof per promise that the coder cannot satisfy while it is false

**Tyler.** Agents write the validation and other agents check that it is
valid: neither trivial, such as a command that always succeeds, nor rigid.
Validation design involves research, some UI design, an interview with the
human on the mechanism of proof, user stories that drive the validation
entrypoints, and adversarial review of all of it, in a loop. A holdout set
applies only where the validation is universal over a set; a user story is
validated against screenshots or a recording instead. Holdouts stay simple
and obscure.

**Agent, accepted.** Two archetypes, universal and scenario. The reviewer runs
on another provider with fresh context and sees only the promise and the
design. It asks whether a coder could satisfy the check while the promise is
false, whether any check is trivially true, and whether the design is
stricter than the promise.

**Tyler.** Those three questions grade a finding; they are not the verdict.
The goal is a correct solution, not a passed check.

### No budgets; the guard against over-engineering is the promise list

**Tyler.** Supervisors curtail belt-and-suspenders work and any rigor not
needed for the proof of work. Code-volume budgets are one measure of several,
not settled, and a separate research project. Opinionated is good; least
engineering first, then prove and massage; replayability and mechanical
history are not requirements.

**Agent, accepted.** Declined promises become exclusions. Supervisors are
named by their question, such as whether a change serves a promise, and steer
with specific design principles and validation angles.

## Execution

### Checks never prove a promise; sprints are demonstrated, chapters proven

**Tyler.** The presence of checks never gives confidence. Someone verifies
materially that the software works: an adversarial agent reviews screenshots
and the other recorded proof and judges whether it is real proof or theater.
A validation may call back to an agent with a Playwright MCP; a cheap judge
model, configured independently and defaulting to Gemini 2.7 Flash (#37),
scores screenshots against a Gherkin scenario. No verdict files: review and
validation are an agent node with routing instructions.

**Agent, accepted.** At chapter exit a verifier with tools, fresh context, and
another provider operates the software itself and routes pass or fail; the
chapter is marked only through the pass edge. A sprint is demonstrated by its
own command and inference; a chapter is proven. The exit gate before on_done
is #33.

### The engine owns done; validation is never skipped

**Tyler.** Every validation in the checklist runs before routing onward. The
engine flips done in both directions; a hand-marked done never skips
validation (#32). On failure the loop re-enters the same item; max_visits is
the only ceiling, counted per activation in nested loops (#34); escalation is
the parent's job. Extra tokens spent on validation are the accepted price of
working software.

**Agent, accepted.** Command first, then inference, both must pass; the agent
never writes done.

### Review loops stop at good enough

**Tyler.** A reviewer's job is to make sure the plan has enough detail for a
frontier coding agent to reach a correct result. Ninety to ninety-five
percent is done. Prompts must not be destructive: reword before adding, do
not accept every critique, do not treat every inconsistency as a failure. Put
a ceiling on laps, and a stopped loop is a question to the human. Never run a
review loop unattended overnight.

**Agent, unreviewed.** Findings graded blocking or note, only blocking routes
fail; the author's moves are fix, reject with a one-line reason, or concede;
two laps, then a human question; a fix that grows a document by more than a
fifth is a redesign; reviewers fresh, on another provider, codex with
memories disabled.

## The record

### Record who made every decision

**Tyler.** Every recorded decision says whether a human or an agent made it.

**Agent, unreviewed.** Each decision, ledger item, or interview entry also
says whether the human has accepted it, and an amendment made with no human
in the loop says so. This document follows that form.

## Where this stands (2026-09-03)

- No design or plan for the designer itself exists. The Sep 2 build was cut
  and nothing has replaced it. What to build next is Tyler's call.
- Open: what survives the Sep 3 cut beyond the loop node, the checklist
  package, frames, the lint rules, and Gimbal ask and answer. Two of Tyler's
  earlier decisions were never withdrawn: authoring is itself a Gimbal run
  with a library of named workflows and an interview API; and a content
  library shipped inside the binary holds prompts, doctrine pages, and a
  models table, edited per release, with manual editing first. The Go package
  that implemented the library is gone. Claude's reading is that the content
  library and the doctrine survive and that the canned graphs, the
  recommendation contract, and validate-plan do not.
- Engine follow-ups from the tie-off: issues #32 to #37 and #39.
