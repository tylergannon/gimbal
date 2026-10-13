# Adversarial review: promises-ledger plan, round 08

> Recovered from the session log. The original `ephemeral/reviews/202610131030-promises-ledger-plan-round-08.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Issue — a failed marked recording can launch paid Claude turns forever with no way for the user to stop it

Marking is deliberately one-way and un-marking is deferred
(`ephemeral/promises-ledger/plan.md:57,157-164`). Every failed adapter call,
invalid answer, timeout, or save leaves the cursor unchanged and schedules the
same recording again; the exponential backoff is capped at five minutes, but
the number of attempts has no cap (`:244-263`). A save failure is especially
concrete: the plan discards the completed result and reruns the entire batch,
including a fresh model call, even when the persistent fault is a full or
unwritable promises directory (`:244-252,446-448`). Each attempt starts a fresh
Claude Code process on the paid default model (`:265-269,479`).

After reaching the cap, one permanently failing recording therefore causes up
to 288 fresh Claude turns per day. The Promises page reports the error, but it
offers no stop, disable, skip, or bounded/manual retry action; restarting
Gimbal only resets the in-memory backoff and resumes faster retries. Multiple
failed recordings multiply the calls while the server remains otherwise
healthy. This is not hypothetical completeness machinery: a persistent disk
write failure is already a planned and tested path, and a consistently invalid
semantic operation is also explicitly possible.

The cited Clarity coordinator bounds provider failures to two attempts per
drain before stopping (`ClaimDeliberationCoordinator.swift:8-19,99-180`). This
plan needs an equally bounded automatic-failure state or another minimal user
escape before it can safely run as an always-on project worker. It need not add
Clarity's job/proof system; it only must not spend indefinitely on an unchanged
failed batch.

## Outcome

**material findings remain**
