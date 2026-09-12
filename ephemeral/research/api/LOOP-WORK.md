# Finish the Loop contract — issue #125

Replace `Task{Lap, Text}` with structured adaptive dispatch. The design and
reasons are in [Promises, dispatch, and completion](LOOP.md). This is the
execution note; the checkboxes remain open until the work is demonstrated.

## What we're doing

- [x] Replace the task contract with a short name, outcome-focused description,
  definition of done, and command/query validation. Use `loop.Tasks`, retain
  the workflow-selected planner Session, and remove the lap counter. Carry
  the same structured assignment through planner output, yielded value, scoped
  data, backlog, and durable events. Godoc and compiling examples agree.
- [x] Give dispatch the goal, relevant scoped information, and recorded results
  of the previous assignment. It selects the greatest concrete gain toward the
  overall Definition of Done, sized for one coherent worker session including
  evidence. It works from prioritized promises alone or adapts a supplied
  sprint plan. An unfinished earlier phase does not automatically block later
  work; outstanding defects remain visible.
- [x] Adapt the existing sprint workflow. It executes validation visibly in Go
  and records the results for dispatch, replacing Loop's automatic command
  sweep. Failed validation informs the next assignment. Returning from a task
  does not declare success, and ending dispatch does not certify fulfillment.
- [x] Demonstrate the replacement through focused mechanical tests and a small
  live workflow, then have an independent validator assess the evidence.

## How we'll know we're done

The mechanical evidence shows that task fields survive every representation,
feedback reaches the next planner without promoting child keys into the
parent, malformed assignments fail explicitly, historical assignments remain
faithful, and early exit/cancellation close owned task scopes and sessions.
The existing consumer and public examples run against the replacement API.

The live evidence shows a real planner receiving a failed check and a relevant
parent constraint, selecting useful follow-up work, and reaching a working
result. It also shows dispatch without a sprint plan and adaptation of a plan,
including advancing past a nonblocking defect when later work offers greater
gain. Controlled fixture defects are disclosed; the planner is not handed the
expected task sequence.

Use Codex `gpt-5.6-luna` for the live worker/planner and Claude Haiku for
independent validation. Retain a compact account of the assignments, actual
validation results, decisions, final assessment, and supporting artifacts.
The validator judges whether the evidence establishes these behaviors and
whether assignments are coherent, appropriately sized, and free of pedantic
implementation instructions. Compilation and unit tests alone are insufficient.

Every requirement above must meet its validation standard. Agent judgment uses
"it works and it's 90–95% done" release latitude; deterministic checks must
pass. Remaining minor issues are recorded without turning them into an endless
finishing loop.

This work does not build the CEO, a promise registry, badges, scheduling, a
generic workflow framework, or a general scoped-context/event-system redesign.

## Evidence

The retained [live result](../../attest/issue125/result.txt) and
[event stream](../../attest/issue125/run.jsonl) record Codex
`gpt-5.6-luna` planning and work plus an independent Claude Haiku PASS. The
no-plan run carries the parent constraint and failed check into adaptive
follow-up, preserves a real probe exit 7 as failure, and later records exit 0.
The plan run demonstrates task selection only: it chooses the higher-priority
export ahead of a nonblocking earlier typo; it does not claim the export was
implemented.

Focused Loop tests cover the structured assignment, scoped feedback,
historical event snapshot, malformed planner data, backlog repair, and task
scope cleanup. The sprint consumer also has a regression test proving that a
task with no command or query still receives an agent assessment against its
Definition of Done before it can be committed. Mechanical, contract, and proof
validators independently passed the result.
