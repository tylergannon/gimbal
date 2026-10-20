# Durable Interactive execution

Status: authoring draft. Only the proposed overview and goals are drafted;
this is not yet an approved or spanning nlspec and does not govern implementation.

## Overview and goals

A workflow has a durable identity independent of its current execution. It can
remain logically unfinished at an explicit wait while releasing active
execution, then become runnable when input arrives. Conversation tabs are
ordinary workflows containing one interactive step, hosted alongside other
workflows in shared Go execution capacity.

Replace `gimbal.Interview` with `gimbal.Interactive`. The agent responds through
a Polytype tagged union with these exact alternatives:

```json
{"type":"chat","message":"…"}
```

```json
{"type":"end"}
```

`chat` finishes the agent turn, publishes its reply, and establishes a durable
wait as one durable transition. The workflow's call to `Interactive` remains
logically pending. `end` completes the interactive step and permits the
workflow to continue. Conversation workflows remain interactive until the
human archives them.

Human messages enter a durable inbox addressed to the interaction regardless
of whether execution is active, suspended, or restarting. An incoming message
signals that the interaction can run again. Resume preserves run identity,
session binding, scoped context, and completed results. Execution may be
reconstructed by replay under the same code; serializing Go stacks is not
required.

Suspension is distinct from completion and cancellation and does not perform
normal scope teardown. A waiting branch does not stop runnable siblings. A
run may become dormant when none of its branches can progress. Suspension
follows an explicit wait, never an inactivity threshold.

## Explicit exclusion: SERVICE lifecycle

Suspension must not close, release, or otherwise alter any service or SERVICE
claim. Existing service behavior and claims are left alone. Rechecking or
restarting SERVICE claims on resume and service-restoration guarantees are
deferred. This spec must not acquire those requirements indirectly through
cleanup, replay, recovery, or validation claims.

## Authoring work remaining

The [source map](SOURCE-MAP.md) records current code facts and cached prior art.
Goals are proposed above. Next: align the goals and recovery boundary, propose
cells with file ownership and code-volume budgets, harden contracts external
seams first, then define routed behavioral claims and exclusions. Run
spec-review before this draft governs work.

Contract decisions requiring alignment include replay-safe ordinary Go work,
active-turn message delivery, archive's workflow outcome, the return contract
of `Interactive`, and the unit of shared execution capacity. These are open
authoring decisions, not implementation permission to choose silently.
