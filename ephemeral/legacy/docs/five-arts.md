# The Five Arts of Orchestration

**Decided by Tyler, 2026-09-07.** The dimensions are his; the notes under each
are working material and may be wrong. He said himself he is not sure the
dimensions are right, so treat this as a live document rather than a settled
one.

These are the things Gimbal is for. They are **competing** priorities, not a
checklist — progress on one can cost another, and noticing that trade is part
of the work.

The later [Gimbal direction](direction.md) applies these aims to ordinary
Go workflow authoring, legibility, context engineering, and learning from
actual runs. It preserves the tensions below.

## 1. Ensure the agent actually does the work

An agent will report success it did not earn. Everything that makes completion
follow evidence rather than assertion lives here: engine-owned `done`, checks
that run the software instead of reading it, validation on a provider that did
not write the code.

## 2. Split the work so no agent carries more than one can get right

Instructions, requirements, rules, checks and validations should be divided so
that no single agent is responsible for more than we should expect it to hold.
A prompt that asks for six things gets four. Decomposition is a craft in its
own right, not a consequence of the others.

## 3. Help the human define the work and author the workflow

Before anything runs, someone has to say what is wanted and shape the graph
that pursues it. Interviews, planning workflows, the editor, examples, and the
schema all serve this. Work that was never well defined cannot be well done.

## 4. Help humans and agents see how the work is going

Run logs, timelines, stage directories, status, the frames injected into
prompts. Both audiences matter: a human checking in from a phone, and an agent
that needs to know where it is in a larger piece of work.

## 5. Let observers steer or otherwise modify the work in flight

Steering, supervisors, the interview gate, stop and resume. A long run that
cannot be corrected is a long run that has to be thrown away.

## The tensions

They pull against each other, and that is the interesting part.

- **1 against 5.** Every steering channel is a way for an observer to talk an
  agent into a verdict it should not reach. Supervisors that cannot route exist
  because of this tension.
- **2 against 4.** Splitting work across more agents makes each one legible and
  the whole harder to see. More nodes, more logs, less understanding.
- **3 against 1.** The person who defines the work is often the one who defines
  its checks, and checks written by the author of a plan tend to accommodate
  the plan.
- **2 against 1.** Divide a job far enough and no agent holds enough context to
  know whether the thing works.

## Practice

This deserves a regular, scheduled look — not only inward at Gimbal, but
outward at how others are answering the same questions. Model providers,
agent frameworks, CI and release engineering, safety evaluation, and the
ordinary discipline of testing all have prior art we are not reading.

The purpose of the review is to keep the arts named and honest, to move claims
from "we think" to "we checked," and to notice when a change served one art at
another's expense.
